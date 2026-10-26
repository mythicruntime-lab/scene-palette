package extract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type plateFixture struct {
	Schema int
	Source struct {
		Extraction int
	}
	Params struct {
		Budget    int
		ChromaMin float64
	}
	Images []plateImageFixture
}

type plateImageFixture struct {
	Label      string
	File       string
	Structures map[string]plateModeFixture
}

type plateModeFixture struct {
	Assembly expectedPlateAssembly
}

type expectedPlateAssembly struct {
	FoundLimit int
	Variants   []expectedPlateVariant
}

type expectedPlateVariant struct {
	Index int
	Core  []expectedPlateToken
	Found []expectedPlateToken
	Notes []string
}

type expectedPlateToken struct {
	ID      string
	Name    string
	Job     string
	RGB     []int
	Hex     string
	Locked  bool
	Derived bool
	Origin  expectedTokenOrigin
}

type expectedTokenOrigin struct {
	Kind      string
	Method    string
	Quantile  [2]float64
	Raw       string
	ChromaCap float64
	Capped    bool
	HueBin    int
	Band      int
	Area      float64
	Salience  float64
	Stills    []int
	IdealHue  float64
	Role      string
	MedianC   float64
	MedianL   float64
	Reason    string
	Repairs   []expectedRepair
}

func TestPlateAssemblyMatchesJavaScriptForEveryVariantAndPolarity(t *testing.T) {
	fixture := loadPlateFixture(t)
	if fixture.Schema != 1 || fixture.Source.Extraction != 5 {
		t.Fatalf("unexpected fixture contract: schema %d extraction %d", fixture.Schema, fixture.Source.Extraction)
	}
	modes := []Polarity{PolarityAuto, PolarityDark, PolarityLight}

	for _, imageFixture := range fixture.Images {
		imageFixture := imageFixture
		input := openFixtureImage(t, imageFixture.File)
		histogram := BuildHistogram(input, fixture.Params.Budget, fixture.Params.ChromaMin)
		pooled := Pool([]Histogram{histogram})
		rawCandidates := SignalCandidates(pooled)
		for _, mode := range modes {
			mode := mode
			t.Run(imageFixture.Label+"/"+string(mode), func(t *testing.T) {
				expectedMode, ok := imageFixture.Structures[string(mode)]
				if !ok {
					t.Fatalf("fixture is missing %s assembly", mode)
				}
				structure := BuildStructure(pooled, mode)
				accents := BuildAccentStage(rawCandidates, structure, 6)
				got := BuildPlateAssembly(structure, accents, expectedMode.Assembly.FoundLimit)
				comparePlateAssembly(t, got, expectedMode.Assembly)
			})
		}
	}
}

func loadPlateFixture(t *testing.T) plateFixture {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "histogram-signal.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture plateFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func comparePlateAssembly(t *testing.T, got PlateAssembly, expected expectedPlateAssembly) {
	t.Helper()
	if got.FoundLimit != expected.FoundLimit {
		t.Errorf("found limit = %d; want %d", got.FoundLimit, expected.FoundLimit)
	}
	if len(got.Variants) != len(expected.Variants) {
		t.Fatalf("variant count = %d; want %d", len(got.Variants), len(expected.Variants))
	}
	for index, expectedVariant := range expected.Variants {
		actual := got.Variants[index]
		if actual.Index != expectedVariant.Index {
			t.Errorf("variant %d index = %d; want %d", index, actual.Index, expectedVariant.Index)
		}
		comparePlateTokens(t, "variant core", actual.Core, expectedVariant.Core)
		comparePlateTokens(t, "variant found", actual.Found, expectedVariant.Found)
		if !reflect.DeepEqual(actual.Notes, expectedVariant.Notes) {
			t.Errorf("variant %d notes = %q; want %q", index, actual.Notes, expectedVariant.Notes)
		}
	}
}

func comparePlateTokens(t *testing.T, label string, got []PlateToken, expected []expectedPlateToken) {
	t.Helper()
	if len(got) != len(expected) {
		t.Fatalf("%s count = %d; want %d", label, len(got), len(expected))
	}
	for index, want := range expected {
		actual := got[index]
		if actual.ID != want.ID || actual.Name != want.Name || actual.Job != want.Job ||
			actual.Hex != want.Hex || actual.Locked != want.Locked || actual.Derived != want.Derived {
			t.Errorf("%s %d metadata = %s/%s/%s/%s locked=%t derived=%t; want %s/%s/%s/%s locked=%t derived=%t",
				label, index, actual.ID, actual.Name, actual.Job, actual.Hex, actual.Locked, actual.Derived,
				want.ID, want.Name, want.Job, want.Hex, want.Locked, want.Derived)
		}
		if len(want.RGB) != 3 || actual.RGB.R != want.RGB[0] || actual.RGB.G != want.RGB[1] || actual.RGB.B != want.RGB[2] {
			t.Errorf("%s %d rgb = [%d %d %d]; want %v", label, index, actual.RGB.R, actual.RGB.G, actual.RGB.B, want.RGB)
		}
		compareTokenOrigin(t, label, actual.Origin, want.Origin)
	}
}

func compareTokenOrigin(t *testing.T, label string, got TokenOrigin, expected expectedTokenOrigin) {
	t.Helper()
	if got.Kind != expected.Kind || got.Method != expected.Method || got.Raw != expected.Raw ||
		got.Capped != expected.Capped || got.HueBin != expected.HueBin || got.Band != expected.Band ||
		got.Role != expected.Role || got.Reason != expected.Reason || !reflect.DeepEqual(got.Stills, expected.Stills) {
		t.Errorf("%s origin = %+v; want %+v", label, got, expected)
	}
	assertNear(t, label+" origin quantile low", got.Quantile[0], expected.Quantile[0])
	assertNear(t, label+" origin quantile high", got.Quantile[1], expected.Quantile[1])
	assertNear(t, label+" origin chroma cap", got.ChromaCap, expected.ChromaCap)
	assertNear(t, label+" origin area", got.Area, expected.Area)
	assertNear(t, label+" origin salience", got.Salience, expected.Salience)
	assertNear(t, label+" origin ideal hue", got.IdealHue, expected.IdealHue)
	assertNear(t, label+" origin median chroma", got.MedianC, expected.MedianC)
	assertNear(t, label+" origin median lightness", got.MedianL, expected.MedianL)
	comparePlateRepairs(t, label, got.Repairs, expected.Repairs)
}

func comparePlateRepairs(t *testing.T, label string, got []Repair, expected []expectedRepair) {
	t.Helper()
	if len(got) != len(expected) {
		t.Fatalf("%s repair count = %d; want %d", label, len(got), len(expected))
	}
	for index, want := range expected {
		actual := got[index]
		if actual.Operation != want.Operation || actual.Against != want.Against ||
			actual.From != want.From || actual.To != want.To {
			t.Errorf("%s repair %d = %s/%s %s→%s; want %s/%s %s→%s",
				label, index, actual.Operation, actual.Against, actual.From, actual.To,
				want.Operation, want.Against, want.From, want.To)
		}
		assertNear(t, label+" repair target", actual.Target, want.Target)
		assertNear(t, label+" repair min delta L", actual.MinDeltaL, want.MinDeltaL)
	}
}
