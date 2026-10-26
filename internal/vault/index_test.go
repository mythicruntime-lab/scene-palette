package vault

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRebuildMatchesGoldenIndex(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "vault")
	testRebuildMatchesGoldenIndex(t, root, 2)
}

func TestRebuildReferencePalettesMatchesGoldenIndex(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "reference-palettes")
	testRebuildMatchesGoldenIndex(t, root, 10)
}

func testRebuildMatchesGoldenIndex(t *testing.T, root string, wantCount int) {
	t.Helper()
	output := filepath.Join(t.TempDir(), "index.json")
	index, skipped, err := Rebuild(root, output)
	if err != nil {
		t.Fatal(err)
	}
	if len(skipped) != 0 {
		t.Fatalf("unexpected skipped palettes: %v", skipped)
	}
	if index.Count != wantCount {
		t.Fatalf("index count = %d; want %d", index.Count, wantCount)
	}

	got := decodeJSONFile(t, output)
	want := decodeJSONFile(t, filepath.Join(root, "index.golden.json"))
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rebuilt index does not match %s", filepath.Join(root, "index.golden.json"))
	}
}

func TestLoadSkipsInvalidPaletteWhenAValidOneRemains(t *testing.T) {
	root := t.TempDir()
	validDir := filepath.Join(root, "valid")
	invalidDir := filepath.Join(root, "invalid")
	if err := os.MkdirAll(validDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(invalidDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile(filepath.Join("..", "..", "testdata", "vault", "night-bay", "palette.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(validDir, "palette.json"), fixture, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(invalidDir, "palette.json"), []byte(`{"schema":1}`), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, skipped, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || len(skipped) != 1 {
		t.Fatalf("loaded %d, skipped %d; want 1 and 1", len(loaded), len(skipped))
	}
}

func TestSlugMatchesBrowserNaming(t *testing.T) {
	tests := map[string]string{
		" Night Bay ":      "night-bay",
		"Day / Night Pair": "day-night-pair",
		"***":              "plate",
	}
	for input, want := range tests {
		if got := Slug(input); got != want {
			t.Errorf("Slug(%q) = %q; want %q", input, got, want)
		}
	}
}

func decodeJSONFile(t *testing.T, path string) any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	return value
}
