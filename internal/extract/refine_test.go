package extract

import (
	"image"
	"image/color"
	"path/filepath"
	"testing"

	"scene-palette/internal/colorx"
)

func TestFinalAccentDistinctionAfterRepair(t *testing.T) {
	ground, panel := colorx.RGB{R: 18, G: 18, B: 18}, colorx.RGB{R: 34, G: 34, B: 34}
	inputs := make([]AccentInput, 4)
	for i := range inputs {
		inputs[i] = AccentInput{RGB: colorx.RGB{R: 120, G: 180, B: 210}, Derived: true}
	}
	result := FinalizeAccents(inputs, nil, ground, panel, ReadableUI)
	if len(result.Notes) != 0 {
		t.Fatalf("ordinary generated colours remained unresolved: %v", result.Notes)
	}
	assertFinalAccents(t, result.Colors, ground, panel, true)
	if result.Colors[3] != inputs[3].RGB {
		t.Fatal("the last valid paint changed unnecessarily")
	}
	second := make([]AccentInput, len(inputs))
	for i := range inputs {
		second[i] = AccentInput{RGB: result.Colors[i], Derived: true}
	}
	again := FinalizeAccents(second, nil, ground, panel, ReadableUI)
	for i := range inputs {
		if again.Colors[i] != result.Colors[i] {
			t.Fatal("final distinction was not idempotent")
		}
	}
}

func TestFinalAccentDistinctionPreservesConflictingLocks(t *testing.T) {
	ground, panel := colorx.RGB{}, colorx.RGB{R: 20, G: 20, B: 20}
	paint := colorx.RGB{R: 150, G: 170, B: 200}
	inputs := []AccentInput{{RGB: paint, Locked: true}, {RGB: paint, Locked: true}, {RGB: paint, Derived: true}, {RGB: paint, Derived: true}}
	result := FinalizeAccents(inputs, nil, ground, panel, ReadableUI)
	if result.Colors[0] != paint || result.Colors[1] != paint {
		t.Fatal("locked paint changed")
	}
	if len(result.Notes) == 0 {
		t.Fatal("conflicting locks were not reported")
	}
	assertFinalAccents(t, result.Colors, ground, panel, false)
	for i := 2; i < 4; i++ {
		for j := 0; j < i; j++ {
			if colorx.DeltaE(result.Colors[i], result.Colors[j]) < .07 {
				t.Fatal("unlocked collision was not repaired")
			}
		}
	}
}

func TestSourceHuesGrayscaleAndPastel(t *testing.T) {
	for _, fixture := range []struct {
		name  string
		gray  bool
		paint color.NRGBA
	}{
		{"grayscale", true, color.NRGBA{}},
		{"near-white-pastel", false, color.NRGBA{R: 247, G: 235, B: 218, A: 255}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			img := image.NewNRGBA(image.Rect(0, 0, 128, 16))
			for y := 0; y < 16; y++ {
				for x := 0; x < 128; x++ {
					pixel := fixture.paint
					if fixture.gray {
						level := uint8(x * 2)
						pixel = color.NRGBA{R: level, G: level, B: level, A: 255}
					}
					img.SetNRGBA(x, y, pixel)
				}
			}
			pooled := Pool([]Histogram{BuildHistogram(img, 140000, .085)})
			for _, mode := range []Polarity{PolarityAuto, PolarityDark, PolarityLight} {
				structure := BuildStructure(pooled, mode)
				accents := BuildAccentStage(SignalCandidates(pooled), structure, 6, SourceHues)
				assembly := BuildPlateAssembly(structure, accents, 6)
				if len(assembly.Variants) != 1 {
					t.Fatal("sparse source should make one source-hue plate")
				}
				core := assembly.Variants[0].Core
				if len(core) != 8 {
					t.Fatal("source policy changed the durable role count")
				}
				for i, token := range core {
					if token.ID != plateRoles[i].ID {
						t.Fatal("source policy changed role order")
					}
					if fixture.gray && (token.RGB.R != token.RGB.G || token.RGB.R != token.RGB.B) {
						t.Fatalf("grayscale invented colour in %s/%s: %s", mode, token.ID, token.Hex)
					}
					if i >= 2 {
						target := 3.0
						if i == 2 {
							target = 4.5
						}
						if colorx.Contrast(token.RGB, core[0].RGB) < target || colorx.Contrast(token.RGB, core[1].RGB) < target {
							t.Fatalf("%s/%s is unreadable", mode, token.ID)
						}
					}
				}
				if !fixture.gray {
					hue := colorx.RGBToLCH(colorx.RGB{R: 247, G: 235, B: 218}).H
					for _, token := range core[4:] {
						lch := colorx.RGBToLCH(token.RGB)
						if lch.C > .01 && colorx.HueGap(lch.H, hue) > 10 {
							t.Fatalf("source mode invented a distant hue: %s", token.Hex)
						}
					}
				}
			}
		})
	}
}

func TestSourceHuesReportsSimilarRolesWithoutInventingHue(t *testing.T) {
	paint := colorx.RGB{R: 160, G: 160, B: 160}
	inputs := make([]AccentInput, 4)
	for i := range inputs {
		inputs[i] = AccentInput{RGB: paint, Derived: true}
	}
	result := FinalizeAccents(inputs, nil, colorx.RGB{}, colorx.RGB{R: 20, G: 20, B: 20}, SourceHues)
	for _, got := range result.Colors {
		if got != paint {
			t.Fatal("source mode invented distinction")
		}
	}
	if len(result.Notes) == 0 {
		t.Fatal("shared source hues were not disclosed")
	}
}

func assertFinalAccents(t *testing.T, colours []colorx.RGB, ground, panel colorx.RGB, distinct bool) {
	t.Helper()
	for i, current := range colours {
		if colorx.Contrast(current, ground) < 3 || colorx.Contrast(current, panel) < 3 {
			t.Fatalf("accent %d lost readability", i)
		}
		if distinct {
			for j := 0; j < i; j++ {
				if colorx.DeltaE(current, colours[j]) < .07 {
					t.Fatalf("accents %d/%d still collide", i, j)
				}
			}
		}
	}
}

func TestReadablePolicyFixtureDistinctness(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "reference-images", "*.png"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) < 10 {
		t.Fatal("missing public regression images")
	}
	for _, path := range paths {
		name := filepath.Base(path)
		pooled := Pool([]Histogram{BuildHistogram(openFixtureImage(t, name), 140000, .085)})
		for _, mode := range []Polarity{PolarityAuto, PolarityDark, PolarityLight} {
			t.Run(name+"/"+string(mode), func(t *testing.T) {
				structure := BuildStructure(pooled, mode)
				accents := BuildAccentStage(SignalCandidates(pooled), structure, 6, ReadableUI)
				for _, variant := range BuildPlateAssembly(structure, accents, 6).Variants {
					colors := make([]colorx.RGB, 4)
					for i, token := range variant.Core[4:] {
						colors[i] = token.RGB
					}
					assertFinalAccents(t, colors, variant.Core[0].RGB, variant.Core[1].RGB, true)
				}
			})
		}
	}
}

func TestSourceHuesKeepsObservedRemixVariants(t *testing.T) {
	candidates := []AccentColor{}
	for i, hue := range []float64{0, 60, 120, 180, 240, 300} {
		rgb := colorx.LCHToRGB(colorx.LCH{L: .7, C: .16, H: hue})
		lch := colorx.RGBToLCH(rgb)
		candidates = append(candidates, AccentColor{RGB: rgb, L: lch.L, C: lch.C, H: lch.H, Salience: 1, sourceIndex: i})
	}
	sets := sourceHueSets(candidates, StructureResult{}, 6)
	if len(sets) != 6 {
		t.Fatalf("rich source lost real-paint Remix choices: got %d; want 6", len(sets))
	}
	seen := map[string]bool{}
	for _, set := range sets {
		signature := accentSetSignature(set)
		if seen[signature] {
			t.Fatal("duplicate Remix choice")
		}
		seen[signature] = true
		for _, token := range set {
			if token.Derived {
				t.Fatal("rich source invented accent paint")
			}
			if token.RGB != candidates[token.sourceIndex].RGB {
				t.Fatal("native paint changed during set selection")
			}
		}
	}
}
