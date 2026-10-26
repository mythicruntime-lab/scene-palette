// Package vault rebuilds index.json exclusively from palette.json documents.
// It deliberately does not know about browser state or extraction internals.
package vault

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"scene-palette/internal/palette"
)

const indexNote = "Derived from each folder's palette.json. Safe to delete; rebuild from the folders."

type LocatedPalette struct {
	Document palette.Document
	Folder   string
}

type Index struct {
	Schema    int            `json:"schema"`
	Kind      string         `json:"kind"`
	Note      string         `json:"note"`
	Generator IndexGenerator `json:"generator"`
	Count     int            `json:"count"`
	Entries   []IndexEntry   `json:"entries"`
}

type IndexGenerator struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type IndexSource struct {
	File string        `json:"file"`
	Hash *palette.Hash `json:"hash"`
}

type IndexEntry struct {
	Name       string            `json:"name"`
	Path       string            `json:"path"`
	Family     *string           `json:"family"`
	Tags       []string          `json:"tags"`
	Polarity   string            `json:"polarity"`
	CreatedAt  *string           `json:"createdAt"`
	Extraction *int              `json:"extraction"`
	Sources    []IndexSource     `json:"sources"`
	Roles      map[string]string `json:"roles"`
	Facets     json.RawMessage   `json:"facets"`
	Signature  json.RawMessage   `json:"signature"`
}

func Build(palettes []LocatedPalette) Index {
	entries := make([]IndexEntry, 0, len(palettes))
	for _, located := range palettes {
		document := located.Document
		folder := located.Folder
		if folder == "" || folder == "." {
			folder = Slug(document.Name)
		}

		tags := append([]string(nil), document.Tags...)
		if tags == nil {
			tags = []string{}
		}
		sources := make([]IndexSource, 0, len(document.Sources))
		for _, source := range document.Sources {
			sources = append(sources, IndexSource{File: source.File, Hash: source.Hash})
		}
		roles := make(map[string]string, len(document.Core))
		for _, token := range document.Core {
			roles[token.Role()] = token.Hex
		}

		entries = append(entries, IndexEntry{
			Name:       document.Name,
			Path:       filepath.ToSlash(filepath.Join(folder, "palette.json")),
			Family:     document.Family,
			Tags:       tags,
			Polarity:   document.Polarity,
			CreatedAt:  document.CreatedAt,
			Extraction: document.Generator.Extraction,
			Sources:    sources,
			Roles:      roles,
			Facets:     append(json.RawMessage(nil), document.Facets...),
			Signature:  append(json.RawMessage(nil), document.Signature...),
		})
	}

	return Index{
		Schema: palette.SchemaVersion,
		Kind:   "palette-index",
		Note:   indexNote,
		Generator: IndexGenerator{
			Name:    "scene-palette",
			Version: palette.ToolVersion,
		},
		Count:   len(entries),
		Entries: entries,
	}
}

// Load reads every palette.json below root. Invalid palettes are returned as
// skipped errors so one damaged folder does not hide the valid remainder.
func Load(root string) ([]LocatedPalette, []error, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, nil, fmt.Errorf("open vault: %w", err)
	}
	if !info.IsDir() {
		return nil, nil, fmt.Errorf("vault root is not a directory: %s", root)
	}

	var paths []string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.EqualFold(entry.Name(), "palette.json") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("scan vault: %w", err)
	}
	if len(paths) == 0 {
		return nil, nil, fmt.Errorf("no palette.json files found below %s", root)
	}
	sort.Strings(paths)

	loaded := make([]LocatedPalette, 0, len(paths))
	var skipped []error
	for _, path := range paths {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			skipped = append(skipped, fmt.Errorf("%s: %w", path, readErr))
			continue
		}
		document, parseErr := palette.Parse(data)
		if parseErr != nil {
			skipped = append(skipped, fmt.Errorf("%s: %w", path, parseErr))
			continue
		}
		folder, relErr := filepath.Rel(root, filepath.Dir(path))
		if relErr != nil {
			skipped = append(skipped, fmt.Errorf("%s: resolve folder: %w", path, relErr))
			continue
		}
		if folder == "." {
			folder = Slug(document.Name)
		}
		loaded = append(loaded, LocatedPalette{Document: document, Folder: filepath.ToSlash(folder)})
	}
	if len(loaded) == 0 {
		return nil, skipped, fmt.Errorf("no valid palette.json files found below %s", root)
	}
	return loaded, skipped, nil
}

func Rebuild(root, output string) (Index, []error, error) {
	loaded, skipped, err := Load(root)
	if err != nil {
		return Index{}, skipped, err
	}
	sort.SliceStable(loaded, func(i, j int) bool {
		if loaded[i].Document.Name == loaded[j].Document.Name {
			return loaded[i].Folder < loaded[j].Folder
		}
		return loaded[i].Document.Name < loaded[j].Document.Name
	})
	index := Build(loaded)
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return Index{}, skipped, fmt.Errorf("encode index: %w", err)
	}
	if output == "" {
		output = filepath.Join(root, "index.json")
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return Index{}, skipped, fmt.Errorf("create index directory: %w", err)
	}
	if err := os.WriteFile(output, data, 0o644); err != nil {
		return Index{}, skipped, fmt.Errorf("write index: %w", err)
	}
	return index, skipped, nil
}

// Slug mirrors the browser's kebab() naming for ASCII plate names.
func Slug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	dash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			dash = false
			continue
		}
		if builder.Len() > 0 && !dash {
			builder.WriteByte('-')
			dash = true
		}
	}
	result := strings.Trim(builder.String(), "-")
	if len(result) > 60 {
		result = strings.TrimRight(result[:60], "-")
	}
	if result == "" {
		return "plate"
	}
	return result
}
