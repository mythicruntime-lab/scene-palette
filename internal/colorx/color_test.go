package colorx

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

type mathFixture struct {
	Schema int
	Source struct {
		Extraction int
	}
	Colors []struct {
		Input             string
		Lab               []float64
		LCH               []float64
		RelativeLuminance float64
		RoundTrip         string
		Cap055            string
	}
	Contrasts []struct {
		A      string
		B      string
		Ratio  float64
		DeltaE float64
		HueGap float64
	}
	Repairs []struct {
		Color      string
		Background string
		Target     float64
		Result     string
		Ratio      float64
	}
	LCHToRGB []struct {
		Input  []float64
		Result string
	}
}

func TestMatchesJavaScriptColorMath(t *testing.T) {
	fixture := loadMathFixture(t)
	if fixture.Schema != 1 || fixture.Source.Extraction != 3 {
		t.Fatalf("unexpected fixture contract: schema %d extraction %d", fixture.Schema, fixture.Source.Extraction)
	}

	for _, test := range fixture.Colors {
		color := parseHex(t, test.Input)
		lab := RGBToOKLab(color)
		lch := RGBToLCH(color)
		assertNear(t, test.Input+" lab L", lab.L, test.Lab[0])
		assertNear(t, test.Input+" lab a", lab.A, test.Lab[1])
		assertNear(t, test.Input+" lab b", lab.B, test.Lab[2])
		assertNear(t, test.Input+" lch L", lch.L, test.LCH[0])
		assertNear(t, test.Input+" lch C", lch.C, test.LCH[1])
		hueTolerance := 1e-12
		if lch.C < 1e-6 {
			// Hue is undefined for neutral colors, so tiny platform-level cube-root
			// differences can rotate the near-zero chroma vector without changing RGB.
			hueTolerance = 1e-6
		}
		assertNearTolerance(t, test.Input+" lch H", lch.H, test.LCH[2], hueTolerance)
		assertNear(t, test.Input+" luminance", RelativeLuminance(color), test.RelativeLuminance)
		if got := Hex(LCHToRGB(lch)); got != test.RoundTrip {
			t.Errorf("%s round trip = %s; want %s", test.Input, got, test.RoundTrip)
		}
		if got := Hex(CapChroma(color, 0.055)); got != test.Cap055 {
			t.Errorf("%s chroma cap = %s; want %s", test.Input, got, test.Cap055)
		}
	}

	for _, test := range fixture.Contrasts {
		a, b := parseHex(t, test.A), parseHex(t, test.B)
		assertNear(t, test.A+"/"+test.B+" contrast", Contrast(a, b), test.Ratio)
		assertNear(t, test.A+"/"+test.B+" deltaE", DeltaE(a, b), test.DeltaE)
		assertNear(t, test.A+"/"+test.B+" hue gap", HueGap(RGBToLCH(a).H, RGBToLCH(b).H), test.HueGap)
	}

	for _, test := range fixture.Repairs {
		background := parseHex(t, test.Background)
		result := ForceContrast(parseHex(t, test.Color), background, test.Target)
		if got := Hex(result); got != test.Result {
			t.Errorf("ForceContrast(%s, %s, %.1f) = %s; want %s", test.Color, test.Background, test.Target, got, test.Result)
		}
		assertNear(t, test.Color+"/"+test.Background+" repaired contrast", Contrast(result, background), test.Ratio)
	}

	for _, test := range fixture.LCHToRGB {
		color := LCHToRGB(LCH{L: test.Input[0], C: test.Input[1], H: test.Input[2]})
		if got := Hex(color); got != test.Result {
			t.Errorf("LCHToRGB(%v) = %s; want %s", test.Input, got, test.Result)
		}
	}
}

func loadMathFixture(t *testing.T) mathFixture {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "color-math.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture mathFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func parseHex(t *testing.T, value string) RGB {
	t.Helper()
	if len(value) != 7 || value[0] != '#' {
		t.Fatalf("invalid fixture hex %q", value)
	}
	number, err := strconv.ParseUint(value[1:], 16, 32)
	if err != nil {
		t.Fatal(err)
	}
	return RGB{R: int(number >> 16), G: int(number>>8) & 255, B: int(number) & 255}
}

func assertNear(t *testing.T, name string, got, want float64) {
	t.Helper()
	assertNearTolerance(t, name, got, want, 1e-12)
}

func assertNearTolerance(t *testing.T, name string, got, want, tolerance float64) {
	t.Helper()
	if math.Abs(got-want) > tolerance {
		t.Errorf("%s = %.16g; want %.16g (difference %.3g)", name, got, want, math.Abs(got-want))
	}
}
