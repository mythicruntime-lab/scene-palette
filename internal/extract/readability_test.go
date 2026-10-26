package extract

import (
	"image"
	"image/color"
	"path/filepath"
	"strings"
	"testing"

	"scene-palette/internal/colorx"
)

func TestJointContrastGeneratedReadability(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "reference-images", "*.png"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) < 10 {
		t.Fatalf("reference image count = %d", len(paths))
	}
	for _, path := range paths {
		name := filepath.Base(path)
		checkGeneratedReadability(t, name, openFixtureImage(t, name))
	}
	for _, value := range []color.NRGBA{{0, 0, 0, 255}, {255, 255, 255, 255}, {128, 128, 128, 255}, {255, 0, 0, 255}, {0, 0, 0, 0}} {
		input := image.NewUniform(value)
		working := image.NewNRGBA(image.Rect(0, 0, 16, 16))
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				working.Set(x, y, input.At(x, y))
			}
		}
		checkGeneratedReadability(t, colorx.Hex(colorx.RGB{R: int(value.R), G: int(value.G), B: int(value.B)}), working)
	}
}

func checkGeneratedReadability(t *testing.T, name string, input image.Image) {
	t.Helper()
	histogram := Pool([]Histogram{BuildHistogram(input, 140000, 0.085)})
	for _, mode := range []Polarity{PolarityAuto, PolarityDark, PolarityLight} {
		t.Run(name+"/"+string(mode), func(t *testing.T) {
			structure := BuildStructure(histogram, mode)
			accents := BuildAccentStage(SignalCandidates(histogram), structure, 6)
			assembly := BuildPlateAssembly(structure, accents, 6)
			for _, variant := range assembly.Variants {
				ground, panel := variant.Core[0].RGB, variant.Core[1].RGB
				for _, token := range variant.Core[2:] {
					target := 3.0
					if token.ID == "ink" {
						target = 4.5
					}
					if colorx.Contrast(token.RGB, ground) < target || colorx.Contrast(token.RGB, panel) < target {
						t.Fatalf("variant %d %s %s fails %v:1 on %s/%s (%.8f/%.8f)", variant.Index, token.ID, token.Hex, target, colorx.Hex(ground), colorx.Hex(panel), colorx.Contrast(token.RGB, ground), colorx.Contrast(token.RGB, panel))
					}
				}
			}
		})
	}
}

func TestGeneratedAutoMatchesVersionedFixtures(t *testing.T) {
	fixture := loadPlateFixture(t)
	for _, reference := range fixture.Images {
		t.Run(reference.Label, func(t *testing.T) {
			histogram := Pool([]Histogram{BuildHistogram(openFixtureImage(t, reference.File), fixture.Params.Budget, fixture.Params.ChromaMin)})
			structure := BuildStructure(histogram, PolarityAuto)
			accents := BuildAccentStage(SignalCandidates(histogram), structure, 6)
			expected := reference.Structures["auto"].Assembly
			comparePlateAssembly(t, BuildPlateAssembly(structure, accents, expected.FoundLimit), expected)
		})
	}
}

func TestJointContrastSurfaceRepairProvenance(t *testing.T) {
	for _, mode := range []Polarity{PolarityDark, PolarityLight} {
		t.Run(string(mode), func(t *testing.T) {
			input := image.NewNRGBA(image.Rect(0, 0, 100, 1))
			cutoff := 10
			if mode == PolarityLight {
				cutoff = 90
			}
			for x := 0; x < 100; x++ {
				gray := uint8(170)
				if x < cutoff {
					gray = 89
				}
				input.SetNRGBA(x, 0, color.NRGBA{R: gray, G: gray, B: gray, A: 255})
			}
			structure := BuildStructure(Pool([]Histogram{BuildHistogram(input, 140000, 0.085)}), mode)
			endpoint, note := "#FFFFFF", "surfaces adjusted for a shared light ink"
			if mode == PolarityLight {
				endpoint, note = "#000000", "surfaces adjusted for a shared dark ink"
			}
			if !strings.Contains(strings.Join(structure.Notes, "\n"), note) {
				t.Fatalf("missing surface feasibility note: %q", structure.Notes)
			}
			recorded := false
			for _, id := range []string{"ground", "panel"} {
				origin := structure.Origins[id]
				previous := origin.Raw
				for _, repair := range origin.Repairs {
					if repair.From == "" || repair.To == "" || repair.From == repair.To {
						t.Fatalf("invalid %s repair: %+v", id, repair)
					}
					if repair.From != previous {
						t.Fatalf("broken %s provenance chain: %s then %+v", id, previous, repair)
					}
					previous = repair.To
					if repair.Operation == "contrast" && repair.Against == endpoint && repair.Target == 4.5 {
						recorded = true
					}
				}
				if previous != colorx.Hex(structure.Colors[id]) {
					t.Fatalf("%s repair chain does not end at final color", id)
				}
			}
			if !recorded {
				t.Fatal("no endpoint surface repair was recorded")
			}
			direction := 1.0
			if mode == PolarityLight {
				direction = -1
			}
			if direction*(colorx.RGBToLCH(structure.Colors["panel"]).L-colorx.RGBToLCH(structure.Colors["ground"]).L) < 0.035 {
				t.Fatal("surface feasibility repair lost required separation")
			}
		})
	}
}
