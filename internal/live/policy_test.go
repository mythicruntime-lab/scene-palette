package live

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"scene-palette/internal/colorx"
	"scene-palette/internal/extract"
	"scene-palette/internal/palette"
)

func TestSourceHuesMatchesActualBrowser(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node is unavailable; install it to verify standalone source-hue parity")
	}
	for _, pattern := range []string{"grayscale", "near-white-pastel", "two-hue", "six-hue", "locked-neutral"} {
		t.Run(pattern, func(t *testing.T) {
			img := image.NewNRGBA(image.Rect(0, 0, 96, 24))
			pixels := []byte{}
			for y := 0; y < 24; y++ {
				for x := 0; x < 96; x++ {
					level := uint8(x * 255 / 95)
					pixel := color.NRGBA{R: level, G: level, B: level, A: 255}
					if pattern == "near-white-pastel" {
						pixel = color.NRGBA{R: 245, G: 238, B: 220, A: 255}
						if x < 20 {
							pixel = color.NRGBA{R: 235, G: 248, B: 230, A: 255}
						}
					}
					if pattern == "two-hue" {
						pixel = color.NRGBA{R: 30, G: 45, B: 80, A: 255}
						if x < 40 {
							pixel = color.NRGBA{R: 205, G: 165, B: 110, A: 255}
						}
					}
					if pattern == "six-hue" {
						rgb := colorx.LCHToRGB(colorx.LCH{L: .7, C: .16, H: float64(x/16) * 60})
						pixel = color.NRGBA{R: uint8(rgb.R), G: uint8(rgb.G), B: uint8(rgb.B), A: 255}
					}
					img.SetNRGBA(x, y, pixel)
					pixels = append(pixels, pixel.R, pixel.G, pixel.B, pixel.A)
				}
			}
			request := defaultRequest()
			request.Params.ColorPolicy = extract.SourceHues
			if pattern == "locked-neutral" {
				request.Core = []palette.SnapshotToken{
					{ID: "ground", Name: "fixed ground", Job: "surface", R: 0, G: 0, B: 0, Hex: "#000000", Locked: true, Origin: json.RawMessage(`{"kind":"picked","repairs":[]}`)},
					{ID: "panel", Name: "fixed panel", Job: "surface", R: 251, G: 251, B: 251, Hex: "#FBFBFB", Locked: true, Origin: json.RawMessage(`{"kind":"picked","repairs":[]}`)},
				}
			}
			input, err := json.Marshal(map[string]any{"request": request, "width": 96, "height": 24, "pixels": pixels})
			if err != nil {
				t.Fatal(err)
			}
			command := exec.Command(node, "-e", browserOracle, filepath.Join("..", "..", "index.html"))
			command.Stdin = bytes.NewReader(input)
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("actual browser source-hue oracle: %v\n%s", err, output)
			}
			var expected map[extract.Polarity][]map[string]any
			if err := json.Unmarshal(output, &expected); err != nil {
				t.Fatalf("oracle JSON: %v\n%s", err, output)
			}
			for _, mode := range []extract.Polarity{extract.PolarityAuto, extract.PolarityDark, extract.PolarityLight} {
				request.Polarity = mode
				result, err := Build([]image.Image{img}, request)
				if err != nil {
					t.Fatal(err)
				}
				if pattern == "six-hue" && len(result.Variants) < 2 {
					t.Fatalf("rich source lost Remix choices in %s", mode)
				}
				actual := []map[string]any{}
				for _, plate := range result.Variants {
					if plate.Params.ColorPolicy != extract.SourceHues {
						t.Fatal("snapshot lost colour policy")
					}
					actual = append(actual, map[string]any{"core": browserTokens(plate.Core, false), "found": browserTokens(plate.Found, true), "notes": plate.Notes})
				}
				t.Run(string(mode), func(t *testing.T) { assertJSONEqual(t, actual, expected[mode]) })
				request.Histogram = &result.Histogram
				cached, err := Build(nil, request)
				if err != nil {
					t.Fatal(err)
				}
				assertJSONEqual(t, cached, result)
				request.Histogram = nil
			}
		})
	}
}

func TestSourceHuesWarnsWhenLockedSurfacesRequireChromaticInk(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 1))
	for x := 0; x < 32; x++ {
		gray := uint8(x * 8)
		img.SetNRGBA(x, 0, color.NRGBA{R: gray, G: gray, B: gray, A: 255})
	}
	request := defaultRequest()
	request.Params.ColorPolicy = extract.SourceHues
	request.Core = []palette.SnapshotToken{
		{ID: "ground", R: 0, G: 0, B: 0, Hex: "#000000", Locked: true, Origin: json.RawMessage(`{"kind":"picked","repairs":[]}`)},
		{ID: "panel", R: 251, G: 251, B: 251, Hex: "#FBFBFB", Locked: true, Origin: json.RawMessage(`{"kind":"picked","repairs":[]}`)},
	}
	result, err := Build([]image.Image{img}, request)
	if err != nil {
		t.Fatal(err)
	}
	for _, plate := range result.Variants {
		if plate.Core[0].Hex != "#000000" || plate.Core[1].Hex != "#FBFBFB" {
			t.Fatal("source policy changed locked surfaces")
		}
		ink, ground, panel := rgb(plate.Core[2]), rgb(plate.Core[0]), rgb(plate.Core[1])
		if colorx.Contrast(ink, ground) < 4.5 || colorx.Contrast(ink, panel) < 4.5 {
			t.Fatal("source policy lost feasible readability")
		}
		if ink.R == ink.G && ink.R == ink.B {
			t.Fatal("narrow fixture unexpectedly has a representable passing gray")
		}
		if !strings.Contains(strings.Join(plate.Notes, "\n"), "source hues shifted for readability — ink") {
			t.Fatalf("chromatic fallback was not disclosed: %v", plate.Notes)
		}
	}
}
