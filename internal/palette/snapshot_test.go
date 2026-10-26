package palette

import (
	"encoding/json"
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"scene-palette/internal/extract"
)

func TestBuildSnapshotMatchesBrowserReferenceDocuments(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "reference-palettes", "*", "palette.json"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	if len(paths) != 10 {
		t.Fatalf("reference palette count = %d; want 10", len(paths))
	}

	for _, path := range paths {
		path := path
		label := filepath.Base(filepath.Dir(path))
		t.Run(label, func(t *testing.T) {
			expectedData, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var expected Snapshot
			if err := json.Unmarshal(expectedData, &expected); err != nil {
				t.Fatal(err)
			}
			if len(expected.Sources) != 1 {
				t.Fatalf("reference source count = %d; want 1", len(expected.Sources))
			}

			input := openSnapshotFixture(t, expected.Sources[0].File)
			histogram := extract.BuildHistogram(input, expected.Params.Budget, expected.Params.ChromaMin)
			pooled := extract.Pool([]extract.Histogram{histogram})
			structure := extract.BuildStructure(pooled, extract.Polarity(expected.Params.PolarityMode))
			accents := extract.BuildAccentStage(extract.SignalCandidates(pooled), structure, 6)
			assembly := extract.BuildPlateAssembly(structure, accents, expected.Params.FoundInks)
			if len(assembly.Variants) != expected.Params.VariantCount {
				t.Fatalf("variant count = %d; want %d", len(assembly.Variants), expected.Params.VariantCount)
			}

			sources := append([]SnapshotSource{}, expected.Sources...)
			for index := range sources {
				sources[index].Path = ""
			}
			actual, err := BuildSnapshot(SnapshotInput{
				Name:         expected.Name,
				CreatedAt:    expected.CreatedAt,
				Structure:    structure,
				Variant:      assembly.Variants[expected.Params.Variant],
				VariantCount: len(assembly.Variants),
				Params: SnapshotParamsInput{
					Budget:    expected.Params.Budget,
					ChromaMin: expected.Params.ChromaMin,
					FoundInks: expected.Params.FoundInks,
				},
				Sources: sources,
				Tags:    expected.Tags,
				Family:  expected.Family,
			})
			if err != nil {
				t.Fatal(err)
			}
			actualData, err := json.Marshal(actual)
			if err != nil {
				t.Fatal(err)
			}
			assertJSONEqual(t, actualData, expectedData)
			if _, err := Parse(actualData); err != nil {
				t.Fatalf("generated snapshot fails durable validation: %v", err)
			}
		})
	}
}

func TestBuildSnapshotRequiresCanonicalCoreOrder(t *testing.T) {
	variant := extract.PlateVariant{Index: 0, Core: make([]extract.PlateToken, len(coreRoles))}
	for index, role := range coreRoles {
		variant.Core[index].ID = role
	}
	variant.Core[0], variant.Core[1] = variant.Core[1], variant.Core[0]
	_, err := BuildSnapshot(SnapshotInput{
		CreatedAt:    "2026-08-28T00:00:00.000Z",
		Structure:    extract.StructureResult{Mode: extract.PolarityAuto},
		Variant:      variant,
		VariantCount: 1,
	})
	if err == nil {
		t.Fatal("BuildSnapshot accepted reordered core roles")
	}
}

func openSnapshotFixture(t *testing.T, name string) image.Image {
	t.Helper()
	file, err := os.Open(filepath.Join("..", "..", "testdata", "reference-images", name))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	decoded, _, err := image.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func assertJSONEqual(t *testing.T, actual, expected []byte) {
	t.Helper()
	var got, want any
	if err := json.Unmarshal(actual, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(expected, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("snapshot drifted from browser baseline\ngot:  %s\nwant: %s", actual, expected)
	}
}
