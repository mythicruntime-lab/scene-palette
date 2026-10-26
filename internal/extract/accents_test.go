package extract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type accentFixture struct {
	Schema int
	Source struct {
		Extraction int
	}
	Params struct {
		Budget    int
		ChromaMin float64
	}
	Images []accentImageFixture
}

type accentImageFixture struct {
	Label      string
	File       string
	Structures map[string]accentModeFixture
}

type accentModeFixture struct {
	Accents expectedAccentStage
}

type expectedAccentStage struct {
	FilteredBins   []int
	RoleCandidates []expectedAccentColor
	Sets           []expectedAccentSet
}

type expectedAccentSet struct {
	Colors          []expectedAccentColor
	AssignedIndices []int
}

type expectedAccentColor struct {
	RGB         []int
	L           float64
	C           float64
	H           float64
	Area        float64
	Salience    float64
	Derived     bool
	Source      *expectedSignalSource `json:"src"`
	Repairs     []expectedRepair
	DerivedFrom *expectedDerivedSource
}

type expectedSignalSource struct {
	Kind     string
	Method   string
	Raw      string
	HueBin   int
	Band     int
	Area     float64
	Salience float64
	Stills   []int
}

type expectedDerivedSource struct {
	Method   string
	IdealHue float64
	Role     string
	MedianC  float64
	MedianL  float64
}

func TestAccentStageMatchesJavaScriptForEveryPolarity(t *testing.T) {
	fixture := loadAccentFixture(t)
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
					t.Fatalf("fixture is missing %s accents", mode)
				}
				structure := BuildStructure(pooled, mode)
				got := BuildAccentStage(rawCandidates, structure, 6)
				compareAccentStage(t, got, expectedMode.Accents)
			})
		}
	}
}

func loadAccentFixture(t *testing.T) accentFixture {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "histogram-signal.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture accentFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func compareAccentStage(t *testing.T, got AccentStage, expected expectedAccentStage) {
	t.Helper()
	filteredBins := make([]int, len(got.Filtered))
	for index, candidate := range got.Filtered {
		filteredBins[index] = candidate.Bin
	}
	if !reflect.DeepEqual(filteredBins, expected.FilteredBins) {
		t.Errorf("filtered bins = %v; want %v", filteredBins, expected.FilteredBins)
	}
	compareAccentColors(t, "role candidates", got.RoleCandidates, expected.RoleCandidates)
	if len(got.Sets) != len(expected.Sets) {
		t.Fatalf("accent set count = %d; want %d", len(got.Sets), len(expected.Sets))
	}
	for index, expectedSet := range expected.Sets {
		compareAccentColors(t, "set", got.Sets[index], expectedSet.Colors)
		assigned := AssignRoleIndices(got.Sets[index])
		if !reflect.DeepEqual(assigned, expectedSet.AssignedIndices) {
			t.Errorf("set %d assigned indices = %v; want %v", index, assigned, expectedSet.AssignedIndices)
		}
	}
}

func compareAccentColors(t *testing.T, label string, got []AccentColor, expected []expectedAccentColor) {
	t.Helper()
	if len(got) != len(expected) {
		t.Fatalf("%s count = %d; want %d", label, len(got), len(expected))
	}
	for index, want := range expected {
		actual := got[index]
		if len(want.RGB) != 3 || actual.RGB.R != want.RGB[0] || actual.RGB.G != want.RGB[1] || actual.RGB.B != want.RGB[2] {
			t.Errorf("%s %d rgb = [%d %d %d]; want %v", label, index, actual.RGB.R, actual.RGB.G, actual.RGB.B, want.RGB)
		}
		assertNear(t, label+" lightness", actual.L, want.L)
		assertNear(t, label+" chroma", actual.C, want.C)
		assertNear(t, label+" hue", actual.H, want.H)
		assertNear(t, label+" area", actual.Area, want.Area)
		assertNear(t, label+" salience", actual.Salience, want.Salience)
		if actual.Derived != want.Derived {
			t.Errorf("%s %d derived = %t; want %t", label, index, actual.Derived, want.Derived)
		}
		compareSignalSource(t, label, actual.Source, want.Source)
		compareAccentRepairs(t, label, actual.Repairs, want.Repairs)
		compareDerivedSource(t, label, actual.DerivedFrom, want.DerivedFrom)
	}
}

func compareSignalSource(t *testing.T, label string, got *SignalSource, expected *expectedSignalSource) {
	t.Helper()
	if got == nil || expected == nil {
		if got != nil || expected != nil {
			t.Errorf("%s signal source presence differs", label)
		}
		return
	}
	if got.Kind != expected.Kind || got.Method != expected.Method || got.Raw != expected.Raw ||
		got.HueBin != expected.HueBin || got.Band != expected.Band || !reflect.DeepEqual(got.Stills, expected.Stills) {
		t.Errorf("%s signal source = %+v; want %+v", label, got, expected)
	}
	assertNear(t, label+" source area", got.Area, expected.Area)
	assertNear(t, label+" source salience", got.Salience, expected.Salience)
}

func compareAccentRepairs(t *testing.T, label string, got []Repair, expected []expectedRepair) {
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

func compareDerivedSource(t *testing.T, label string, got *DerivedSource, expected *expectedDerivedSource) {
	t.Helper()
	if got == nil || expected == nil {
		if got != nil || expected != nil {
			t.Errorf("%s derived source presence differs", label)
		}
		return
	}
	if got.Method != expected.Method || got.Role != expected.Role {
		t.Errorf("%s derived source = %s/%s; want %s/%s", label, got.Method, got.Role, expected.Method, expected.Role)
	}
	assertNear(t, label+" ideal hue", got.IdealHue, expected.IdealHue)
	assertNear(t, label+" median chroma", got.MedianC, expected.MedianC)
	assertNear(t, label+" median lightness", got.MedianL, expected.MedianL)
}
