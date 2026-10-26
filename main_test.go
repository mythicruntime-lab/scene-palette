package main

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"scene-palette/internal/palette"
)

func TestFrontendHandlerServesReferenceApp(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	frontendHandler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("GET / returned %d", response.Code)
	}
	body := response.Body.String()
	if !strings.Contains(body, `id="extractBtn"`) || !strings.Contains(body, "Forge plate") {
		t.Fatal("GET / did not serve the existing Scene Palette frontend")
	}
	for _, marker := range []string{
		`id="startBtn"`,
		`id="stageCanvas" tabindex="0"`,
		`id="toastMsg" role="status"`,
		`id="reviewBar"`,
		`id="reviewFocus"`,
		`id="reviewDiff"`,
		`id="reviewHistory"`,
		`id="whyPanel"`,
		"function onStageKeydown",
		"function sealPlate",
		"function reviewDecision",
		"function reviewAssessment",
		"palette-and-source-identities",
		"function reviewSourceRecords",
		"function renderReviewDiff",
		"function renderReviewFocus",
		"function normalizeReviewHistory",
		"function renderReviewHistory",
		"activeShelfId",
		"function shelfRevisionValue",
		"function reserveShelfId",
		"function mergeShelfImport",
		"function mergeShelfRevision",
		"No shelf changes to save",
		"held back at the 40-plate limit",
		"plateMeta:{createdAt:null",
		"function plateMetadata",
		"createdAt: meta.createdAt",
		"generator: meta.generator ||",
		"meta.generator = null",
		"tags: meta.tags, family: meta.family",
		"id:sanitizeName(src.id) || null",
		"function currentPlateSources",
		"function sourceBytesFor",
		"stills: currentPlateSources().length",
		"state.plateSources = clonePlain",
		"function renderWhy",
	} {
		if !strings.Contains(body, marker) {
			t.Fatalf("GET / is missing polished frontend marker %q", marker)
		}
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q; want no-store", got)
	}
}

func TestFrontendHandlerHealth(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	frontendHandler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != "ok\n" {
		t.Fatalf("health response = (%d, %q)", response.Code, response.Body.String())
	}
}

func TestListenFrontendFallsBackOnlyForTheDefaultAddress(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()

	var stderr strings.Builder
	fallback, err := listenFrontend(occupied.Addr().String(), true, &stderr)
	if err != nil {
		t.Fatalf("fallback listen: %v", err)
	}
	defer fallback.Close()
	if fallback.Addr().String() == occupied.Addr().String() {
		t.Fatalf("fallback reused occupied address %s", occupied.Addr())
	}
	if !strings.Contains(stderr.String(), "already in use") {
		t.Fatalf("fallback message = %q", stderr.String())
	}

	if strict, err := listenFrontend(occupied.Addr().String(), false, &stderr); err == nil {
		strict.Close()
		t.Fatal("explicit address unexpectedly fell back")
	}
}

func TestBrowserCommand(t *testing.T) {
	name, args, err := browserCommand("windows", "http://127.0.0.1:8765/")
	if err != nil {
		t.Fatal(err)
	}
	if name != "rundll32.exe" || strings.Join(args, " ") != "url.dll,FileProtocolHandler http://127.0.0.1:8765/" {
		t.Fatalf("browser command = %q %q", name, args)
	}
	if _, _, err := browserCommand("plan9", "http://127.0.0.1:8765/"); err == nil {
		t.Fatal("browserCommand accepted an unsupported platform")
	}
}

func TestWindowsLauncherPrefersExecutableWithStandaloneFallback(t *testing.T) {
	data, err := os.ReadFile("Run Scene Palette.cmd")
	if err != nil {
		t.Fatal(err)
	}
	launcher := string(data)
	for _, required := range []string{
		`"%~dp0scene-palette.exe" serve -open`,
		`if not exist "%~dp0go.mod" goto standalone`,
		`if not exist "%~dp0main.go" goto standalone`,
		"run . serve -open",
		`%ProgramFiles%\Go\bin\go.exe`,
		`start "" "%~dp0index.html"`,
	} {
		if !strings.Contains(launcher, required) {
			t.Fatalf("launcher is missing %q", required)
		}
	}
	if exe, goLookup := strings.Index(launcher, `if exist "%~dp0scene-palette.exe" goto executable`), strings.Index(launcher, "where go"); exe < 0 || exe >= goLookup {
		t.Fatal("launcher must select the adjacent executable before looking for Go")
	}
}

func TestRunCheckEnforcesHumanSeal(t *testing.T) {
	fixture := filepath.Join("testdata", "vault", "night-bay", "palette.json")
	var stdout, stderr strings.Builder
	if err := run([]string{"check", fixture}, &stdout, &stderr); err == nil ||
		!strings.Contains(err.Error(), "not agent-eligible") ||
		!strings.Contains(stdout.String(), "BLOCKED") ||
		!strings.Contains(stdout.String(), "Review focus:") {
		t.Fatalf("unsealed check = %v, stdout %q", err, stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	if err := run([]string{"check", "-allow-unsealed", fixture}, &stdout, &stderr); err != nil {
		t.Fatalf("allow unsealed: %v", err)
	}

	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	document, err := palette.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, err := document.ReviewFingerprint(palette.ReviewScopePaletteAndSourceIdentities)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	raw["review"] = map[string]any{
		"version": 1, "status": "sealed", "agentEligible": true,
		"sealedAt": "2026-08-30T12:34:56.000Z", "sealedBy": "human",
		"scope": palette.ReviewScopePaletteAndSourceIdentities, "fingerprint": fingerprint,
		"decision": map[string]any{
			"version": 1, "baseline": true, "changeCount": 1, "fieldCount": 2,
		},
		"history": []any{
			map[string]any{
				"version": 1, "sealedAt": "2026-08-29T12:34:56.000Z",
				"sealedBy": "human", "scope": "palette-content",
				"fingerprint": map[string]any{"algo": "fnv1a64", "value": "0123456789abcdef"},
				"decision": map[string]any{
					"version": 1, "baseline": true, "changeCount": 0, "fieldCount": 0,
				},
				"assessment": map[string]any{
					"version": 1, "status": "clear", "attentionCount": 0,
				},
				"invalidatedAt": "2026-08-30T09:00:00.000Z",
				"invalidatedBy": "observed renamed by human",
			},
		},
	}
	write := func(name string) string {
		t.Helper()
		encoded, err := json.MarshalIndent(raw, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(path, append(encoded, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}

	sealedPath := write("sealed.json")
	stdout.Reset()
	stderr.Reset()
	if err := run([]string{"check", "-json", sealedPath}, &stdout, &stderr); err != nil {
		t.Fatalf("sealed check: %v\nstdout: %s", err, stdout.String())
	}
	var report checkReport
	if err := json.Unmarshal([]byte(stdout.String()), &report); err != nil {
		t.Fatal(err)
	}
	if report.Status != "ok" || !report.ContractValid || !report.AgentEligible ||
		report.Review == nil || !report.Review.FingerprintMatches ||
		report.Review.Scope != palette.ReviewScopePaletteAndSourceIdentities ||
		report.Review.RecordedDecision == nil || report.Review.RecordedDecision.ChangeCount != 1 ||
		len(report.Review.RecordedHistory) != 1 ||
		report.Review.RecordedHistory[0].InvalidatedBy == nil ||
		*report.Review.RecordedHistory[0].InvalidatedBy != "observed renamed by human" ||
		report.Review.Assessment.Version != 1 || len(report.Review.Assessment.Checks) != 4 {
		t.Fatalf("sealed report = %#v", report)
	}
	stdout.Reset()
	stderr.Reset()
	if err := run([]string{"check", sealedPath}, &stdout, &stderr); err != nil {
		t.Fatalf("plain sealed check: %v\nstdout: %s", err, stdout.String())
	}
	if !strings.Contains(stdout.String(), "Prior seals: 1 seal recorded, not reverified.") {
		t.Fatalf("plain check omitted history caveat: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "scope palette + source identities") {
		t.Fatalf("plain check omitted source-bound scope: %q", stdout.String())
	}

	raw["core"].([]any)[4].(map[string]any)["hex"] = "#ABCDEF"
	tamperedPath := write("tampered.json")
	stdout.Reset()
	stderr.Reset()
	if err := run([]string{"check", "-json", tamperedPath}, &stdout, &stderr); err == nil {
		t.Fatal("tampered plate was accepted")
	}
	report = checkReport{}
	if err := json.Unmarshal([]byte(stdout.String()), &report); err != nil {
		t.Fatal(err)
	}
	if report.AgentEligible || report.Review == nil || report.Review.FingerprintMatches ||
		!strings.Contains(report.Review.Reason, "match the human seal") {
		t.Fatalf("tampered report = %#v", report)
	}
}

func TestRunForgeWritesDurablePalette(t *testing.T) {
	output := filepath.Join(t.TempDir(), "palette.json")
	input := filepath.Join("testdata", "reference-images", "bauhaus-primary.png")
	var stdout, stderr strings.Builder
	if err := run([]string{
		"forge", "-out", output, "-name", "Bauhaus Primary", input,
	}, &stdout, &stderr); err != nil {
		t.Fatalf("run forge: %v\nstderr: %s", err, stderr.String())
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	document, err := palette.Parse(data)
	if err != nil {
		t.Fatalf("parse forged palette: %v", err)
	}
	if document.Name != "Bauhaus Primary" || len(document.Core) != 8 {
		t.Fatalf("forged document = %q with %d roles", document.Name, len(document.Core))
	}
	if !strings.Contains(stdout.String(), "variant 1/") {
		t.Fatalf("forge output = %q", stdout.String())
	}
}

func TestForgeDoesNotOverwriteAnInputAlias(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "reference-images", "bauhaus-primary.png"))
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	input, output := filepath.Join(directory, "original.png"), filepath.Join(directory, "alias.json")
	if err := os.WriteFile(input, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(input, output); err != nil {
		t.Skipf("filesystem cannot create hard links: %v", err)
	}
	var stdout, stderr strings.Builder
	if err := run([]string{"forge", "-out", output, input}, &stdout, &stderr); err == nil || !strings.Contains(err.Error(), "overwrite input") {
		t.Fatalf("forge input alias error = %v", err)
	}
	after, err := os.ReadFile(input)
	if err != nil || string(after) != string(data) {
		t.Fatalf("input changed: %v", err)
	}
}
