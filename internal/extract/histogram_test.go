package extract

import (
	"encoding/json"
	"image"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type stageFixture struct {
	Schema int
	Source struct {
		Extraction int
	}
	Params struct {
		Budget        int
		ChromaMin     float64
		LuminanceBins int
		HueBins       int
		Bands         int
	}
	Images []stageImage
}

type stageImage struct {
	Label         string
	File          string
	Width         int
	Height        int
	Step          int
	Total         int
	WeightedTotal float64
	LuminanceBins []fixtureBin
	SignalBins    []fixtureSignalBin
	StrideProbe   *fixtureHistogram
	Candidates    []fixtureCandidate
	RankedBins    []int
}

type fixtureHistogram struct {
	Budget        int
	Step          int
	Total         int
	WeightedTotal float64
	LuminanceBins []fixtureBin
	SignalBins    []fixtureSignalBin
}

type fixtureBin struct {
	Index int
	N     float64
	L     float64
	A     float64
	B     float64
}

type fixtureSignalBin struct {
	fixtureBin
	WeightSum float64 `json:"wsum"`
}

type fixtureCandidate struct {
	RGB      []int
	Area     float64
	C        float64
	H        float64
	Salience float64
	Bin      int
	Band     int
	HueBin   int
	Stills   []int
}

func TestHistogramAndSignalRankingMatchJavaScript(t *testing.T) {
	fixture := loadStageFixture(t)
	if fixture.Schema != 1 || fixture.Source.Extraction != 4 {
		t.Fatalf("unexpected fixture contract: schema %d extraction %d", fixture.Schema, fixture.Source.Extraction)
	}
	if fixture.Params.LuminanceBins != LuminanceBinCount ||
		fixture.Params.HueBins != HueBinCount || fixture.Params.Bands != LightnessBands {
		t.Fatalf("fixture bin layout does not match Go constants")
	}

	for _, expected := range fixture.Images {
		expected := expected
		t.Run(expected.Label, func(t *testing.T) {
			input := openFixtureImage(t, expected.File)
			bounds := input.Bounds()
			if bounds.Dx() != expected.Width || bounds.Dy() != expected.Height {
				t.Fatalf("decoded size = %dx%d; want %dx%d", bounds.Dx(), bounds.Dy(), expected.Width, expected.Height)
			}

			histogram := BuildHistogram(input, fixture.Params.Budget, fixture.Params.ChromaMin)
			if histogram.Step != expected.Step {
				t.Errorf("step = %d; want %d", histogram.Step, expected.Step)
			}
			if histogram.Total != expected.Total {
				t.Errorf("sample total = %d; want %d", histogram.Total, expected.Total)
			}
			compareLuminanceBins(t, histogram, expected.LuminanceBins)
			compareSignalBins(t, histogram, expected.SignalBins)

			weightedTotal := 0.0
			for _, bin := range histogram.Luminance {
				weightedTotal += bin.N
			}
			assertNear(t, "weighted total", weightedTotal, expected.WeightedTotal)

			if expected.StrideProbe != nil {
				probe := BuildHistogram(input, expected.StrideProbe.Budget, fixture.Params.ChromaMin)
				if probe.Step != expected.StrideProbe.Step {
					t.Errorf("stride probe step = %d; want %d", probe.Step, expected.StrideProbe.Step)
				}
				if probe.Total != expected.StrideProbe.Total {
					t.Errorf("stride probe total = %d; want %d", probe.Total, expected.StrideProbe.Total)
				}
				compareLuminanceBins(t, probe, expected.StrideProbe.LuminanceBins)
				compareSignalBins(t, probe, expected.StrideProbe.SignalBins)
				probeWeightedTotal := 0.0
				for _, bin := range probe.Luminance {
					probeWeightedTotal += bin.N
				}
				assertNear(t, "stride probe weighted total", probeWeightedTotal, expected.StrideProbe.WeightedTotal)
			}

			candidates := SignalCandidates(Pool([]Histogram{histogram}))
			if len(candidates) != len(expected.Candidates) {
				t.Fatalf("candidate count = %d; want %d", len(candidates), len(expected.Candidates))
			}
			for index, want := range expected.Candidates {
				got := candidates[index]
				if len(want.RGB) != 3 || got.RGB.R != want.RGB[0] || got.RGB.G != want.RGB[1] || got.RGB.B != want.RGB[2] {
					t.Errorf("candidate %d rgb = [%d %d %d]; want %v", index, got.RGB.R, got.RGB.G, got.RGB.B, want.RGB)
				}
				if got.Bin != want.Bin || got.Band != want.Band || got.HueBin != want.HueBin {
					t.Errorf("candidate %d location = bin %d band %d hue %d; want bin %d band %d hue %d",
						index, got.Bin, got.Band, got.HueBin, want.Bin, want.Band, want.HueBin)
				}
				if !reflect.DeepEqual(got.Stills, want.Stills) {
					t.Errorf("candidate %d stills = %v; want %v", index, got.Stills, want.Stills)
				}
				assertNear(t, "candidate area", got.Area, want.Area)
				assertNear(t, "candidate chroma", got.C, want.C)
				assertNear(t, "candidate hue", got.H, want.H)
				assertNear(t, "candidate salience", got.Salience, want.Salience)
			}

			ranked := RankedPool(candidates, 8)
			bins := make([]int, len(ranked))
			for index, candidate := range ranked {
				bins[index] = candidate.Bin
			}
			if !reflect.DeepEqual(bins, expected.RankedBins) {
				t.Errorf("ranked bins = %v; want %v", bins, expected.RankedBins)
			}
		})
	}
}

func TestPoolWeightsStillsEqually(t *testing.T) {
	first := Histogram{Total: 100}
	first.Luminance[3].N = 80
	first.Signal[4] = SignalBin{N: 10, L: 4, A: 3, B: 2, WeightSum: 5}
	second := Histogram{Total: 10}
	second.Luminance[3].N = 5
	second.Signal[4] = SignalBin{N: 2, L: 1, A: 0.5, B: 0.25, WeightSum: 1.5}
	second.Signal[5] = SignalBin{N: 1, WeightSum: 1}

	pooled := Pool([]Histogram{first, second})
	assertNear(t, "pooled total", pooled.TotalN, 1300)
	assertNear(t, "pooled shared signal count", pooled.Signal[4].N, 300)
	assertNear(t, "pooled second signal count", pooled.Signal[5].N, 100)
	if !reflect.DeepEqual(pooled.BinStills[4], []int{0, 1}) {
		t.Fatalf("shared bin stills = %v; want [0 1]", pooled.BinStills[4])
	}
	if !reflect.DeepEqual(pooled.BinStills[5], []int{1}) {
		t.Fatalf("second bin stills = %v; want [1]", pooled.BinStills[5])
	}
}

func loadStageFixture(t *testing.T) stageFixture {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "histogram-signal.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture stageFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func openFixtureImage(t *testing.T, name string) image.Image {
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

func compareLuminanceBins(t *testing.T, histogram Histogram, expected []fixtureBin) {
	t.Helper()
	want := make(map[int]fixtureBin, len(expected))
	for _, bin := range expected {
		want[bin.Index] = bin
	}
	for index, got := range histogram.Luminance {
		expectedBin, populated := want[index]
		if !populated {
			if got.N != 0 {
				t.Errorf("unexpected populated luminance bin %d", index)
			}
			continue
		}
		assertNear(t, "luminance n", got.N, expectedBin.N)
		assertNear(t, "luminance L", got.L, expectedBin.L)
		assertNear(t, "luminance a", got.A, expectedBin.A)
		assertNear(t, "luminance b", got.B, expectedBin.B)
	}
}

func compareSignalBins(t *testing.T, histogram Histogram, expected []fixtureSignalBin) {
	t.Helper()
	want := make(map[int]fixtureSignalBin, len(expected))
	for _, bin := range expected {
		want[bin.Index] = bin
	}
	for index, got := range histogram.Signal {
		expectedBin, populated := want[index]
		if !populated {
			if got.N != 0 {
				t.Errorf("unexpected populated signal bin %d", index)
			}
			continue
		}
		assertNear(t, "signal n", got.N, expectedBin.N)
		assertNear(t, "signal L", got.L, expectedBin.L)
		assertNear(t, "signal a", got.A, expectedBin.A)
		assertNear(t, "signal b", got.B, expectedBin.B)
		assertNear(t, "signal weight", got.WeightSum, expectedBin.WeightSum)
	}
}

func assertNear(t *testing.T, name string, got, want float64) {
	t.Helper()
	tolerance := 1e-9 + math.Abs(want)*2e-12
	if math.Abs(got-want) > tolerance {
		t.Errorf("%s = %.16g; want %.16g (difference %.3g, tolerance %.3g)",
			name, got, want, math.Abs(got-want), tolerance)
	}
}
