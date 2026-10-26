package forge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"scene-palette/internal/extract"
	"scene-palette/internal/palette"
)

func TestBuildMatchesBrowserReferencePalettes(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "reference-palettes", "*", "palette.json"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	if len(paths) != 10 {
		t.Fatalf("reference palette count = %d; want 10", len(paths))
	}

	for _, expectedPath := range paths {
		expectedPath := expectedPath
		label := filepath.Base(filepath.Dir(expectedPath))
		t.Run(label, func(t *testing.T) {
			expectedData, err := os.ReadFile(expectedPath)
			if err != nil {
				t.Fatal(err)
			}
			var expected palette.Snapshot
			if err := json.Unmarshal(expectedData, &expected); err != nil {
				t.Fatal(err)
			}
			options := DefaultOptions()
			options.Name = expected.Name
			options.CreatedAt = expected.CreatedAt
			options.Budget = expected.Params.Budget
			options.ChromaMin = expected.Params.ChromaMin
			options.FoundInks = expected.Params.FoundInks
			options.Polarity = extract.Polarity(expected.Params.PolarityMode)
			options.Variant = expected.Params.Variant
			imagePath := filepath.Join("..", "..", "testdata", "reference-images", expected.Sources[0].File)

			actual, err := Build([]string{imagePath}, options)
			if err != nil {
				t.Fatal(err)
			}
			expected.Sources[0].Transforms = []map[string]any{{"op": "decode", "via": "go-image/png"}}
			assertForgeJSONEqual(t, actual, expected)
			actualData, err := json.Marshal(actual)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := palette.Parse(actualData); err != nil {
				t.Fatalf("forged palette fails durable validation: %v", err)
			}
		})
	}
}

func TestBuildPoolsMultipleImages(t *testing.T) {
	options := DefaultOptions()
	options.Name = "Two still study"
	options.CreatedAt = "2026-08-29T12:34:56.000Z"
	paths := []string{
		filepath.Join("..", "..", "testdata", "reference-images", "bauhaus-primary.png"),
		filepath.Join("..", "..", "testdata", "reference-images", "winter-cabin.png"),
	}
	snapshot, err := Build(paths, options)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Stills != 2 || len(snapshot.Sources) != 2 {
		t.Fatalf("forged stills/sources = %d/%d; want 2/2", snapshot.Stills, len(snapshot.Sources))
	}
	if snapshot.Sources[0].Path != "sources/01-bauhaus-primary.png" || snapshot.Sources[1].Path != "sources/02-winter-cabin.png" {
		t.Fatalf("unexpected source paths: %q, %q", snapshot.Sources[0].Path, snapshot.Sources[1].Path)
	}
	if len(snapshot.Core) != 8 || snapshot.Params.VariantCount < 1 {
		t.Fatalf("forged core/variants = %d/%d", len(snapshot.Core), snapshot.Params.VariantCount)
	}
}

func assertForgeJSONEqual(t *testing.T, actual, expected palette.Snapshot) {
	t.Helper()
	actualData, err := json.Marshal(actual)
	if err != nil {
		t.Fatal(err)
	}
	expectedData, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}
	var got, want any
	if err := json.Unmarshal(actualData, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(expectedData, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("forged snapshot drifted from browser baseline\ngot:  %s\nwant: %s", actualData, expectedData)
	}
}
