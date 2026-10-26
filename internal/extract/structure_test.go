package extract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type structureFixture struct {
	Schema int
	Source struct {
		Extraction int
	}
	Params struct {
		Budget    int
		ChromaMin float64
	}
	Images []structureImageFixture
}

type structureImageFixture struct {
	Label      string
	File       string
	Structures map[string]expectedStructure
}

type expectedStructure struct {
	Mode       string
	MedianL    float64
	AutoLight  bool
	LightScene bool
	Roles      []expectedStructureRole
}

type expectedStructureRole struct {
	ID     string
	RGB    []int
	Origin expectedStructureOrigin
}

type expectedStructureOrigin struct {
	Kind      string
	Method    string
	Quantile  [2]float64
	Raw       string
	ChromaCap float64
	Capped    bool
	Repairs   []expectedRepair
}

type expectedRepair struct {
	Operation string `json:"op"`
	Against   string
	Target    float64
	MinDeltaL float64
	From      string
	To        string
}

func TestStructureMatchesJavaScriptForEveryPolarity(t *testing.T) {
	fixture := loadStructureFixture(t)
	if fixture.Schema != 1 || fixture.Source.Extraction != 5 {
		t.Fatalf("unexpected fixture contract: schema %d extraction %d", fixture.Schema, fixture.Source.Extraction)
	}
	modes := []Polarity{PolarityAuto, PolarityDark, PolarityLight}

	for _, imageFixture := range fixture.Images {
		imageFixture := imageFixture
		input := openFixtureImage(t, imageFixture.File)
		histogram := BuildHistogram(input, fixture.Params.Budget, fixture.Params.ChromaMin)
		pooled := Pool([]Histogram{histogram})
		for _, mode := range modes {
			mode := mode
			t.Run(imageFixture.Label+"/"+string(mode), func(t *testing.T) {
				expected, ok := imageFixture.Structures[string(mode)]
				if !ok {
					t.Fatalf("fixture is missing %s structure", mode)
				}
				got := BuildStructure(pooled, mode)
				compareStructure(t, got, expected)
			})
		}
	}
}

func loadStructureFixture(t *testing.T) structureFixture {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "histogram-signal.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture structureFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func compareStructure(t *testing.T, got StructureResult, expected expectedStructure) {
	t.Helper()
	if string(got.Mode) != expected.Mode {
		t.Errorf("mode = %s; want %s", got.Mode, expected.Mode)
	}
	assertNear(t, "median lightness", got.MedianL, expected.MedianL)
	if got.AutoLight != expected.AutoLight {
		t.Errorf("autoLight = %t; want %t", got.AutoLight, expected.AutoLight)
	}
	if got.LightScene != expected.LightScene {
		t.Errorf("lightScene = %t; want %t", got.LightScene, expected.LightScene)
	}
	if len(got.Colors) != len(expected.Roles) || len(got.Origins) != len(expected.Roles) {
		t.Fatalf("structure role count = %d colors/%d origins; want %d", len(got.Colors), len(got.Origins), len(expected.Roles))
	}
	for _, role := range expected.Roles {
		color, ok := got.Colors[role.ID]
		if !ok {
			t.Errorf("missing color for %s", role.ID)
			continue
		}
		if len(role.RGB) != 3 || color.R != role.RGB[0] || color.G != role.RGB[1] || color.B != role.RGB[2] {
			t.Errorf("%s rgb = [%d %d %d]; want %v", role.ID, color.R, color.G, color.B, role.RGB)
		}
		origin, ok := got.Origins[role.ID]
		if !ok {
			t.Errorf("missing origin for %s", role.ID)
			continue
		}
		compareStructureOrigin(t, role.ID, origin, role.Origin)
	}
}

func compareStructureOrigin(t *testing.T, role string, got StructureOrigin, expected expectedStructureOrigin) {
	t.Helper()
	if got.Kind != expected.Kind || got.Method != expected.Method {
		t.Errorf("%s origin = %s/%s; want %s/%s", role, got.Kind, got.Method, expected.Kind, expected.Method)
	}
	assertNear(t, role+" quantile low", got.Quantile[0], expected.Quantile[0])
	assertNear(t, role+" quantile high", got.Quantile[1], expected.Quantile[1])
	if got.Raw != expected.Raw {
		t.Errorf("%s raw = %s; want %s", role, got.Raw, expected.Raw)
	}
	assertNear(t, role+" chroma cap", got.ChromaCap, expected.ChromaCap)
	if got.Capped != expected.Capped {
		t.Errorf("%s capped = %t; want %t", role, got.Capped, expected.Capped)
	}
	if len(got.Repairs) != len(expected.Repairs) {
		t.Fatalf("%s repair count = %d; want %d", role, len(got.Repairs), len(expected.Repairs))
	}
	for index, repair := range expected.Repairs {
		actual := got.Repairs[index]
		if actual.Operation != repair.Operation || actual.Against != repair.Against ||
			actual.From != repair.From || actual.To != repair.To {
			t.Errorf("%s repair %d = %s/%s %s→%s; want %s/%s %s→%s",
				role, index, actual.Operation, actual.Against, actual.From, actual.To,
				repair.Operation, repair.Against, repair.From, repair.To)
		}
		assertNear(t, role+" repair target", actual.Target, repair.Target)
		assertNear(t, role+" repair min delta L", actual.MinDeltaL, repair.MinDeltaL)
	}
}
