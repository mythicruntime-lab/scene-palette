package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"scene-palette/internal/extract"
	"scene-palette/internal/live"
	"scene-palette/internal/palette"
	"scene-palette/internal/vault"
)

func TestAPIInfoAndLocalBoundary(t *testing.T) {
	handler := frontendHandler()
	for _, tc := range []struct {
		name, host, origin, site string
		status                   int
	}{
		{"loopback", "127.0.0.1:8765", "", "", 200},
		{"same origin", "127.0.0.1:8765", "http://127.0.0.1:8765", "same-origin", 200},
		{"localhost", "localhost:8765", "http://localhost:8765", "", 200},
		{"IPv6", "[::1]:8765", "http://[::1]:8765", "", 200},
		{"foreign host", "attacker.example:8765", "", "", 403},
		{"foreign origin", "127.0.0.1:8765", "https://attacker.example", "", 403},
		{"opaque origin", "127.0.0.1:8765", "null", "", 403},
		{"different port", "127.0.0.1:8765", "http://127.0.0.1:9000", "", 403},
		{"wrong scheme", "127.0.0.1:8765", "https://127.0.0.1:8765", "", 403},
		{"userinfo", "127.0.0.1:8765", "http://someone@127.0.0.1:8765", "", 403},
		{"cross site", "127.0.0.1:8765", "", "cross-site", 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/api/info", nil)
			request.Host = tc.host
			request.Header.Set("Origin", tc.origin)
			request.Header.Set("Sec-Fetch-Site", tc.site)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			apiWantStatus(t, response, tc.status)
			if response.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatal("missing nosniff header")
			}
			if tc.status == http.StatusOK {
				var info struct {
					Engine              string
					Version, Extraction int
				}
				apiDecode(t, response.Body.Bytes(), &info)
				if info.Engine != "go" || info.Version != 1 || info.Extraction != palette.ExtractionVersion {
					t.Fatalf("API info = %+v", info)
				}
			}
		})
	}
	response := apiRequest(t, handler, http.MethodGet, "/api/extract", "", nil)
	apiWantStatus(t, response, http.StatusMethodNotAllowed)
	if response.Header().Get("Allow") != "POST" {
		t.Fatal("missing method guidance")
	}
	apiWantStatus(t, apiRequest(t, handler, http.MethodPost, "/api/unknown", "application/json", []byte(`{}`)), http.StatusNotFound)
}

func TestAPILiveExtractionRoundTrip(t *testing.T) {
	handler := frontendHandler()
	data := apiFixture(t, "bauhaus-primary")
	var expected palette.Snapshot
	apiDecode(t, data, &expected)
	request := apiLiveRequest(expected)
	imageBytes, err := os.ReadFile(filepath.Join("testdata", "reference-images", expected.Sources[0].File))
	if err != nil {
		t.Fatal(err)
	}
	response := apiMultipart(t, handler, "/api/extract", []apiPart{
		{"request", "", apiJSONBytes(t, request)}, {"image", "working.png", imageBytes},
	})
	apiWantStatus(t, response, http.StatusOK)
	var fresh live.Response
	apiDecode(t, response.Body.Bytes(), &fresh)
	if len(fresh.Variants) != expected.Params.VariantCount {
		t.Fatalf("variant count = %d", len(fresh.Variants))
	}
	apiAssertJSON(t, fresh.Variants[expected.Params.Variant], expected)
	request.Histogram = &fresh.Histogram
	response = apiRequest(t, handler, http.MethodPost, "/api/extract", "application/json", apiJSONBytes(t, request))
	apiWantStatus(t, response, http.StatusOK)
	var cached live.Response
	apiDecode(t, response.Body.Bytes(), &cached)
	apiAssertJSON(t, cached, fresh)

	request.Core = fresh.Variants[0].Core
	request.Core[4] = palette.SnapshotToken{ID: "observed", Name: "human pigment", Job: "keep me", R: 12, G: 34, B: 56, Hex: "#0C2238", Locked: true, Origin: json.RawMessage(`{"kind":"picked","working":{"x":3,"y":7},"natural":{"x":6,"y":14},"still":{"file":"artist.png"},"repairs":[]}`)}
	request.Found = []palette.SnapshotToken{{ID: "personal_ink", Name: "pinned ink", Job: "annotation", R: 200, G: 42, B: 140, Hex: "#C82A8C", Locked: true, Origin: json.RawMessage(`{"kind":"picked","repairs":[]}`)}}
	request.Params.FoundInks = 0
	request.Polarity = extract.PolarityDark
	response = apiRequest(t, handler, http.MethodPost, "/api/extract", "application/json", apiJSONBytes(t, request))
	apiWantStatus(t, response, http.StatusOK)
	var locked live.Response
	apiDecode(t, response.Body.Bytes(), &locked)
	for _, variant := range locked.Variants {
		if variant.Polarity != "dark" || variant.Params.FoundInks != 0 {
			t.Fatal("requested polarity/count not applied")
		}
		core := variant.Core[4]
		if core.Hex != request.Core[4].Hex || core.Name != request.Core[4].Name || core.Job != request.Core[4].Job || !core.Locked {
			t.Fatalf("locked role changed: %+v", core)
		}
		apiAssertJSON(t, core.Origin, request.Core[4].Origin)
		if len(variant.Found) != 1 || variant.Found[0].ID != request.Found[0].ID || variant.Found[0].Hex != request.Found[0].Hex {
			t.Fatalf("pinned ink changed: %+v", variant.Found)
		}
		if _, err := palette.Parse(apiJSONBytes(t, variant)); err != nil {
			t.Fatalf("live output fails contract: %v", err)
		}
	}
}

func TestAPIBundleAndReviewRoundTrip(t *testing.T) {
	handler := frontendHandler()
	data := apiFixture(t, "bauhaus-primary")
	var sourcePlate palette.Snapshot
	apiDecode(t, data, &sourcePlate)
	original, err := os.ReadFile(filepath.Join("testdata", "reference-images", sourcePlate.Sources[0].File))
	if err != nil {
		t.Fatal(err)
	}
	sealed := apiSealedFixture(t, data)
	response := apiMultipart(t, handler, "/api/bundle", []apiPart{{"palette", "", sealed}, {"source", "original.png", original}})
	apiWantStatus(t, response, http.StatusOK)
	if response.Header().Get("Content-Type") != "application/zip" || !strings.Contains(response.Header().Get("Content-Disposition"), "bauhaus-primary.zip") {
		t.Fatalf("bundle headers = %v", response.Header())
	}
	files := apiZipFiles(t, response.Body.Bytes())
	if _, ok := files["index.json"]; ok {
		t.Fatal("single bundle must not contain an index")
	}
	for _, name := range []string{"palette.json", "tokens.css", "plate.gd", "palette.gpl", "notes.md", "preview.png"} {
		if _, ok := files["bauhaus-primary/"+name]; !ok {
			t.Fatalf("missing bundle file %s", name)
		}
	}
	if !bytes.Equal(files["bauhaus-primary/"+sourcePlate.Sources[0].Path], original) {
		t.Fatal("source bytes changed")
	}
	preview, err := png.DecodeConfig(bytes.NewReader(files["bauhaus-primary/preview.png"]))
	if err != nil || preview.Width != 960 || preview.Height != 250 {
		t.Fatalf("generated preview = %+v, %v", preview, err)
	}
	var exported, initial map[string]any
	apiDecode(t, files["bauhaus-primary/palette.json"], &exported)
	apiDecode(t, sealed, &initial)
	for _, field := range []string{"review", "extension", "core", "found", "createdAt", "generator", "params"} {
		apiAssertJSON(t, exported[field], initial[field])
	}
	check := apiRequest(t, handler, http.MethodPost, "/api/check", "application/json", files["bauhaus-primary/palette.json"])
	apiWantStatus(t, check, http.StatusOK)
	var report struct {
		ContractValid, AgentEligible bool
		Review                       palette.ReviewVerification
	}
	apiDecode(t, check.Body.Bytes(), &report)
	if !report.ContractValid || !report.AgentEligible || !report.Review.FingerprintMatches {
		t.Fatalf("exported seal no longer verifies: %+v", report)
	}

	check = apiRequest(t, handler, http.MethodPost, "/api/check", "application/json", data)
	apiWantStatus(t, check, http.StatusOK)
	apiDecode(t, check.Body.Bytes(), &report)
	if !report.ContractValid || report.AgentEligible {
		t.Fatalf("unsealed plate became eligible: %+v", report)
	}

	exported["core"].([]any)[4].(map[string]any)["hex"] = "#ABCDEF"
	check = apiRequest(t, handler, http.MethodPost, "/api/check", "application/json", apiJSONBytes(t, exported))
	apiWantStatus(t, check, http.StatusOK)
	apiDecode(t, check.Body.Bytes(), &report)
	if report.AgentEligible || report.Review.FingerprintMatches {
		t.Fatal("tampered palette was agent-eligible")
	}

	missing := apiMultipart(t, handler, "/api/bundle", []apiPart{{"palette", "", data}})
	apiWantStatus(t, missing, http.StatusOK)
	missingFiles := apiZipFiles(t, missing.Body.Bytes())
	var missingPlate struct {
		Sources []struct {
			Path     *string
			Retained *bool
		}
	}
	apiDecode(t, missingFiles["bauhaus-primary/palette.json"], &missingPlate)
	if len(missingPlate.Sources) != 1 || missingPlate.Sources[0].Path != nil || missingPlate.Sources[0].Retained == nil || *missingPlate.Sources[0].Retained {
		t.Fatalf("missing source was not recorded accurately: %+v", missingPlate)
	}
}

func TestAPIVaultIndexMatchesDiskRebuild(t *testing.T) {
	handler := frontendHandler()
	// Deliberately reverse alphabetical order: filesystem and browser file-list
	// order must not change a reconstructed index.
	plates := []json.RawMessage{apiFixture(t, "winter-cabin"), apiFixture(t, "bauhaus-primary")}
	parts := []apiPart{{"palettes", "", apiJSONBytes(t, plates)}}
	for i, data := range plates {
		var plate palette.Snapshot
		apiDecode(t, data, &plate)
		source, err := os.ReadFile(filepath.Join("testdata", "reference-images", plate.Sources[0].File))
		if err != nil {
			t.Fatal(err)
		}
		field := "source-0"
		if i == 1 {
			field = "source-1"
		}
		parts = append(parts, apiPart{field, plate.Sources[0].File, source})
	}
	response := apiMultipart(t, handler, "/api/vault", parts)
	apiWantStatus(t, response, http.StatusOK)
	files := apiZipFiles(t, response.Body.Bytes())
	if _, ok := files["README.md"]; !ok {
		t.Fatal("vault readme missing")
	}
	var exported vault.Index
	apiDecode(t, files["index.json"], &exported)
	if exported.Count != 2 {
		t.Fatalf("vault count = %d", exported.Count)
	}
	root := t.TempDir()
	apiWriteZipFiles(t, root, files)
	if err := os.Remove(filepath.Join(root, "index.json")); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := run([]string{"rebuild-index", root}, &stdout, &stderr); err != nil {
		t.Fatalf("disk rebuild: %v\n%s", err, stderr.String())
	}
	rebuiltBytes, err := os.ReadFile(filepath.Join(root, "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rebuilt vault.Index
	apiDecode(t, rebuiltBytes, &rebuilt)
	apiAssertJSON(t, rebuilt, exported)
	inputs := []map[string]any{
		{"folder": "winter-cabin", "palette": json.RawMessage(files["winter-cabin/palette.json"])},
		{"folder": "bauhaus-primary", "palette": json.RawMessage(files["bauhaus-primary/palette.json"])},
		{"folder": "../outside", "palette": json.RawMessage(files["bauhaus-primary/palette.json"])},
	}
	indexResponse := apiRequest(t, handler, http.MethodPost, "/api/index", "application/json", apiJSONBytes(t, inputs))
	apiWantStatus(t, indexResponse, http.StatusOK)
	var indexed struct {
		Index   vault.Index
		Skipped []string
	}
	apiDecode(t, indexResponse.Body.Bytes(), &indexed)
	if len(indexed.Skipped) != 1 {
		t.Fatalf("invalid folder skips = %v", indexed.Skipped)
	}
	apiAssertJSON(t, indexed.Index, exported)
}

func TestAPIRejectsMalformedRequests(t *testing.T) {
	handler := frontendHandler()
	data := apiFixture(t, "bauhaus-primary")
	var plate palette.Snapshot
	apiDecode(t, data, &plate)
	req := apiLiveRequest(plate)
	var oversized bytes.Buffer
	if err := png.Encode(&oversized, image.NewNRGBA(image.Rect(0, 0, 1401, 1))); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, endpoint, contentType string
		body                        []byte
	}{
		{"wrong JSON media", "/api/check", "text/plain", data},
		{"two JSON values", "/api/check", "application/json", append(append([]byte{}, data...), []byte(` {}`)...)},
		{"JSON beyond limit", "/api/check", "application/json", append(append(append([]byte{}, data...), bytes.Repeat([]byte(" "), 8<<20)...), []byte(`{}`)...)},
		{"invalid plate", "/api/check", "application/json", []byte(`{}`)},
		{"invalid extract shape", "/api/extract", "application/json", []byte(`[]`)},
		{"null extract", "/api/extract", "application/json", []byte(`null`)},
		{"missing pixels", "/api/extract", "application/json", apiJSONBytes(t, req)},
		{"wrong multipart media", "/api/bundle", "application/json", data},
		{"empty index", "/api/index", "application/json", []byte(`[]`)},
		{"unsafe only index", "/api/index", "application/json", apiJSONBytes(t, []map[string]any{{"folder": "../escape", "palette": json.RawMessage(data)}})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			apiWantStatus(t, apiRequest(t, handler, http.MethodPost, tc.endpoint, tc.contentType, tc.body), http.StatusBadRequest)
		})
	}
	for _, tc := range []struct {
		name, endpoint string
		parts          []apiPart
	}{
		{"corrupt PNG", "/api/extract", []apiPart{{"request", "", apiJSONBytes(t, req)}, {"image", "bad.png", []byte("invalid image")}}},
		{"oversized dimensions", "/api/extract", []apiPart{{"request", "", apiJSONBytes(t, req)}, {"image", "wide.png", oversized.Bytes()}}},
		{"duplicate request", "/api/extract", []apiPart{{"request", "", apiJSONBytes(t, req)}, {"request", "", apiJSONBytes(t, req)}}},
		{"missing palette", "/api/bundle", nil},
		{"unknown attachment hash", "/api/bundle", []apiPart{{"palette", "", data}, {"source", "wrong.png", []byte("unrelated bytes")}}},
		{"invalid preview", "/api/bundle", []apiPart{{"palette", "", data}, {"preview", "bad.png", []byte("invalid image")}}},
		{"empty vault", "/api/vault", []apiPart{{"palettes", "", []byte(`[]`)}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			apiWantStatus(t, apiMultipart(t, handler, tc.endpoint, tc.parts), http.StatusBadRequest)
		})
	}
}

func TestRunForgeBundlePipeline(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "forged.zip")
	input := filepath.Join("testdata", "reference-images", "bauhaus-primary.png")
	var stdout, stderr bytes.Buffer
	if err := run([]string{"forge", "-bundle", archivePath, "-name", "Pipeline plate", input}, &stdout, &stderr); err != nil {
		t.Fatalf("forge bundle: %v\n%s", err, stderr.String())
	}
	archive, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	files := apiZipFiles(t, archive)
	if _, ok := files["index.json"]; ok {
		t.Fatal("forge single bundle contains an index")
	}
	plateData := files["pipeline-plate/palette.json"]
	document, err := palette.Parse(plateData)
	if err != nil {
		t.Fatalf("forged bundled contract: %v", err)
	}
	if document.Name != "Pipeline plate" || len(document.Core) != 8 || len(document.Sources) != 1 {
		t.Fatalf("unexpected forged document: %+v", document)
	}
	original, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(files["pipeline-plate/sources/01-bauhaus-primary.png"], original) {
		t.Fatal("forge bundle changed original source bytes")
	}
	vaultRoot := filepath.Join(root, "vault")
	apiWriteZipFiles(t, vaultRoot, files)
	stdout.Reset()
	stderr.Reset()
	if err := run([]string{"rebuild-index", vaultRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("rebuild bundled vault: %v", err)
	}
	indexData, err := os.ReadFile(filepath.Join(vaultRoot, "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	var index vault.Index
	apiDecode(t, indexData, &index)
	if index.Count != 1 || index.Entries[0].Path != "pipeline-plate/palette.json" {
		t.Fatalf("rebuilt index = %+v", index)
	}
	stdout.Reset()
	stderr.Reset()
	if err := run([]string{"check", "-json", filepath.Join(vaultRoot, "pipeline-plate", "palette.json")}, &stdout, &stderr); err == nil {
		t.Fatal("fresh forged palette became agent-eligible without a human seal")
	}
	var report checkReport
	apiDecode(t, stdout.Bytes(), &report)
	if !report.ContractValid || report.AgentEligible || report.Status != "blocked" {
		t.Fatalf("unsealed forged check = %+v", report)
	}
	stdout.Reset()
	stderr.Reset()
	if err := run([]string{"forge", "-bundle", "-", input}, &stdout, &stderr); err != nil {
		t.Fatalf("ZIP stdout: %v", err)
	}
	apiZipFiles(t, stdout.Bytes())
	if err := run([]string{"forge", "-bundle", archivePath, "-out", filepath.Join(root, "other.json"), input}, io.Discard, io.Discard); err == nil {
		t.Fatal("forge accepted conflicting output flags")
	}
}

type apiPart struct {
	name, filename string
	data           []byte
}

func apiMultipart(t *testing.T, handler http.Handler, endpoint string, parts []apiPart) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, part := range parts {
		var field io.Writer
		var err error
		if part.filename == "" {
			field, err = writer.CreateFormField(part.name)
		} else {
			field, err = writer.CreateFormFile(part.name, part.filename)
		}
		if err != nil {
			t.Fatal(err)
		}
		if _, err = field.Write(part.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return apiRequest(t, handler, http.MethodPost, endpoint, writer.FormDataContentType(), body.Bytes())
}
func apiRequest(t *testing.T, handler http.Handler, method, endpoint, contentType string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, "http://127.0.0.1"+endpoint, bytes.NewReader(body))
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
func apiWantStatus(t *testing.T, response *httptest.ResponseRecorder, status int) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("HTTP status = %d, want %d: %s", response.Code, status, response.Body.String())
	}
}
func apiDecode(t *testing.T, data []byte, value any) {
	t.Helper()
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, data)
	}
}
func apiJSONBytes(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func apiAssertJSON(t *testing.T, actual, expected any) {
	t.Helper()
	var got, want any
	apiDecode(t, apiJSONBytes(t, actual), &got)
	apiDecode(t, apiJSONBytes(t, expected), &want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("JSON values differ\ngot: %s\nwant: %s", apiJSONBytes(t, actual), apiJSONBytes(t, expected))
	}
}
func apiFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "reference-palettes", name, "palette.json"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func apiLiveRequest(p palette.Snapshot) live.Request {
	return live.Request{Name: p.Name, CreatedAt: p.CreatedAt, Params: live.Params{Budget: p.Params.Budget, ChromaMin: p.Params.ChromaMin, FoundInks: p.Params.FoundInks}, Polarity: extract.Polarity(p.Params.PolarityMode), Sources: p.Sources}
}
func apiSealedFixture(t *testing.T, data []byte) []byte {
	t.Helper()
	document, err := palette.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, err := document.ReviewFingerprint(palette.ReviewScopePaletteAndSourceIdentities)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	apiDecode(t, data, &raw)
	raw["extension"] = map[string]any{"owner": "human", "kept": true}
	raw["review"] = map[string]any{"version": 1, "status": "sealed", "agentEligible": true, "sealedAt": "2026-08-30T12:34:56.000Z", "sealedBy": "human", "scope": palette.ReviewScopePaletteAndSourceIdentities, "fingerprint": fingerprint, "decision": map[string]any{"version": 1, "baseline": true, "changeCount": 0, "fieldCount": 0}, "history": []any{map[string]any{"version": 1, "sealedAt": "2026-08-29T12:34:56.000Z", "sealedBy": "human", "scope": "palette-content", "fingerprint": map[string]any{"algo": "fnv1a64", "value": "0123456789abcdef"}, "invalidatedAt": "2026-08-30T10:00:00.000Z", "invalidatedBy": "source changed"}}}
	return apiJSONBytes(t, raw)
}
func apiZipFiles(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("read ZIP: %v", err)
	}
	files := make(map[string][]byte, len(reader.File))
	for _, file := range reader.File {
		if file.Name == "" || path.IsAbs(file.Name) || path.Clean(file.Name) != file.Name || strings.HasPrefix(file.Name, "../") || strings.Contains(file.Name, "\\") {
			t.Fatalf("unsafe ZIP entry %q", file.Name)
		}
		if _, exists := files[file.Name]; exists {
			t.Fatalf("duplicate ZIP entry %q", file.Name)
		}
		stream, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(stream)
		stream.Close()
		if err != nil {
			t.Fatal(err)
		}
		files[file.Name] = content
	}
	return files
}
func apiWriteZipFiles(t *testing.T, root string, files map[string][]byte) {
	t.Helper()
	for name, data := range files {
		relative := filepath.FromSlash(name)
		if !filepath.IsLocal(relative) {
			t.Fatalf("unsafe extracted path %q", name)
		}
		target := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
