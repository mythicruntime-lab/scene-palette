package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"

	"scene-palette/internal/bundle"
	"scene-palette/internal/live"
	"scene-palette/internal/palette"
	"scene-palette/internal/vault"
)

// Requests contain their own pixels or histogram. No browser state, source
// bytes, or filesystem paths are retained by the server between requests.
func newAPIHandler() http.Handler {
	active := make(chan struct{}, 2)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if !localAPIRequest(r) {
			apiError(w, http.StatusForbidden, "the local API requires a same-origin loopback request")
			return
		}
		if r.URL.Path == "/api/info" && r.Method == http.MethodGet {
			apiJSON(w, map[string]any{"engine": "go", "version": 1, "extraction": palette.ExtractionVersion})
			return
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			apiError(w, http.StatusMethodNotAllowed, "POST required")
			return
		}
		select {
		case active <- struct{}{}:
			defer func() { <-active }()
		default:
			apiError(w, http.StatusTooManyRequests, "two operations are already running; try again shortly")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 256<<20)
		switch r.URL.Path {
		case "/api/extract":
			r.Body = http.MaxBytesReader(w, r.Body, 96<<20)
			serveExtract(w, r)
		case "/api/bundle", "/api/vault":
			serveBundle(w, r)
		case "/api/check":
			serveCheck(w, r)
		case "/api/index":
			serveIndex(w, r)
		default:
			apiError(w, http.StatusNotFound, "unknown API endpoint")
		}
	})
}

func localAPIRequest(r *http.Request) bool {
	host := r.Host
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	}
	host = strings.Trim(host, "[]")
	ip := net.ParseIP(host)
	if !strings.EqualFold(host, "localhost") && (ip == nil || !ip.IsLoopback()) {
		return false
	}
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		parsed, err := url.Parse(origin)
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		if err != nil || parsed.Scheme != scheme || !strings.EqualFold(parsed.Host, r.Host) || parsed.User != nil {
			return false
		}
	}
	return true
}

func apiJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(value)
}

func apiError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func readJSON(r *http.Request, value any) error {
	kind, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || kind != "application/json" {
		return fmt.Errorf("application/json required")
	}
	const limit = 8 << 20
	data, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
	if err != nil {
		return err
	}
	if len(data) > limit {
		return fmt.Errorf("JSON exceeds 8 MiB")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("expected a single JSON value")
	}
	return nil
}

// Read parts in memory with explicit limits, avoiding temp files and accepting
// no client-supplied disk paths. The outer request limit bounds their sum.
func readParts(r *http.Request) (map[string][][]byte, error) {
	reader, err := r.MultipartReader()
	if err != nil {
		return nil, fmt.Errorf("multipart/form-data required: %w", err)
	}
	parts := make(map[string][][]byte)
	for count := 0; ; count++ {
		part, err := reader.NextPart()
		if err == io.EOF {
			return parts, nil
		}
		if err != nil {
			return nil, err
		}
		if count >= 400 {
			return nil, fmt.Errorf("too many attachments")
		}
		limit := int64(64 << 20)
		if part.FileName() == "" {
			limit = 8 << 20
		}
		data, err := io.ReadAll(io.LimitReader(part, limit+1))
		if err != nil {
			return nil, err
		}
		if int64(len(data)) > limit {
			return nil, fmt.Errorf("attachment %q exceeds %d MiB", part.FormName(), limit>>20)
		}
		parts[part.FormName()] = append(parts[part.FormName()], data)
	}
}

func singlePart(parts map[string][][]byte, name string, required bool) ([]byte, error) {
	values := parts[name]
	if len(values) == 0 && !required {
		return nil, nil
	}
	if len(values) != 1 {
		return nil, fmt.Errorf("expected one %q part", name)
	}
	return values[0], nil
}

func serveExtract(w http.ResponseWriter, r *http.Request) {
	var request live.Request
	var images []image.Image
	kind, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	var err error
	if kind == "application/json" {
		err = readJSON(r, &request)
	} else {
		var parts map[string][][]byte
		parts, err = readParts(r)
		if err == nil {
			var data []byte
			data, err = singlePart(parts, "request", true)
			if err == nil {
				err = json.Unmarshal(data, &request)
			}
		}
		if err == nil && (len(parts["image"]) < 1 || len(parts["image"]) > 8) {
			err = fmt.Errorf("one to eight working PNG images are required")
		}
		if err == nil {
			for _, data := range parts["image"] {
				var config image.Config
				config, err = png.DecodeConfig(bytes.NewReader(data))
				if err != nil {
					break
				}
				if config.Width < 1 || config.Height < 1 || config.Width > 1400 || config.Height > 1400 {
					err = fmt.Errorf("working PNG dimensions must be between 1 and 1400 pixels")
					break
				}
				var decoded image.Image
				decoded, err = png.Decode(bytes.NewReader(data))
				if err != nil {
					break
				}
				images = append(images, decoded)
			}
		}
	}
	if err != nil {
		apiError(w, http.StatusBadRequest, "extract request: "+err.Error())
		return
	}
	response, err := live.Build(images, request)
	if err != nil {
		apiError(w, http.StatusBadRequest, err.Error())
		return
	}
	apiJSON(w, response)
}

func serveBundle(w http.ResponseWriter, r *http.Request) {
	parts, err := readParts(r)
	if err != nil {
		apiError(w, http.StatusBadRequest, err.Error())
		return
	}
	var archive []byte
	filename := "palette-vault.zip"
	if r.URL.Path == "/api/bundle" {
		var data, preview []byte
		data, err = singlePart(parts, "palette", true)
		if err == nil {
			preview, err = singlePart(parts, "preview", false)
		}
		if err == nil {
			archive, err = bundle.Build(data, parts["source"], preview)
		}
		if err == nil {
			filename, err = bundle.Filename(data)
		}
	} else {
		var data []byte
		data, err = singlePart(parts, "palettes", true)
		var palettes []json.RawMessage
		if err == nil {
			err = json.Unmarshal(data, &palettes)
		}
		if err == nil && (len(palettes) < 1 || len(palettes) > 41) {
			err = fmt.Errorf("a vault contains one to 41 plates")
		}
		if err == nil {
			items := make([]bundle.Input, len(palettes))
			for i, data := range palettes {
				preview, previewErr := singlePart(parts, fmt.Sprintf("preview-%d", i), false)
				if previewErr != nil {
					err = previewErr
					break
				}
				items[i] = bundle.Input{Palette: data, Sources: parts[fmt.Sprintf("source-%d", i)], Preview: preview}
			}
			if err == nil {
				archive, err = bundle.BuildVault(items)
			}
		}
	}
	if err != nil {
		apiError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	_, _ = w.Write(archive)
}

func serveCheck(w http.ResponseWriter, r *http.Request) {
	var data json.RawMessage
	if err := readJSON(r, &data); err != nil {
		apiError(w, http.StatusBadRequest, err.Error())
		return
	}
	document, err := palette.Parse(data)
	if err != nil {
		apiError(w, http.StatusBadRequest, err.Error())
		return
	}
	review, err := document.VerifyReview()
	if err != nil {
		apiError(w, http.StatusBadRequest, err.Error())
		return
	}
	apiJSON(w, map[string]any{"contractValid": true, "agentEligible": review.AgentEligible, "review": review})
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	var inputs []struct {
		Folder  string          `json:"folder"`
		Palette json.RawMessage `json:"palette"`
	}
	if err := readJSON(r, &inputs); err != nil {
		apiError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(inputs) < 1 || len(inputs) > 1000 {
		apiError(w, http.StatusBadRequest, "index requires one to 1000 plates")
		return
	}
	loaded := make([]vault.LocatedPalette, 0, len(inputs))
	skipped := []string{}
	for _, input := range inputs {
		document, err := palette.Parse(input.Palette)
		folder := input.Folder
		if err != nil || folder == "" || strings.Contains(folder, "\\") || strings.Contains(folder, ":") ||
			strings.HasPrefix(folder, "/") || path.Clean(folder) != folder || folder == ".." || strings.HasPrefix(folder, "../") {
			skipped = append(skipped, fmt.Sprintf("invalid palette or relative folder %q", folder))
			continue
		}
		loaded = append(loaded, vault.LocatedPalette{Document: document, Folder: folder})
	}
	if len(loaded) == 0 {
		apiError(w, http.StatusBadRequest, "no valid palette folders")
		return
	}
	sort.SliceStable(loaded, func(i, j int) bool {
		if loaded[i].Document.Name == loaded[j].Document.Name {
			return loaded[i].Folder < loaded[j].Folder
		}
		return loaded[i].Document.Name < loaded[j].Document.Name
	})
	apiJSON(w, map[string]any{"index": vault.Build(loaded), "skipped": skipped})
}
