// Package extract contains the headless, deterministic stages of Scene
// Palette's image extraction pipeline. Browser ingestion and UI stay in
// JavaScript; this package starts at decoded pixels.
package extract

import (
	"image"
	"image/color"
	"math"
	"sort"

	"scene-palette/internal/colorx"
)

const (
	LuminanceBinCount = 64
	HueBinCount       = 36
	LightnessBands    = 3
	SignalBinCount    = HueBinCount * LightnessBands
	areaFloor         = 0.00012
)

type LuminanceBin struct {
	N float64
	L float64
	A float64
	B float64
}

type SignalBin struct {
	N         float64
	L         float64
	A         float64
	B         float64
	WeightSum float64
}

type Histogram struct {
	Luminance [LuminanceBinCount]LuminanceBin
	Signal    [SignalBinCount]SignalBin
	Step      int
	Total     int
}

type PooledHistogram struct {
	Luminance [LuminanceBinCount]LuminanceBin
	Signal    [SignalBinCount]SignalBin
	BinStills [SignalBinCount][]int
	TotalN    float64
}

type Candidate struct {
	RGB      colorx.RGB
	Area     float64
	C        float64
	H        float64
	Salience float64
	Bin      int
	Band     int
	HueBin   int
	Stills   []int
}

// BuildHistogram samples decoded pixels on the same full-resolution stride as
// index.html. The budget selects a stride; it does not resize the image.
func BuildHistogram(source image.Image, budget int, chromaMin float64) Histogram {
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	step := 1
	if budget > 0 {
		step = max(1, int(math.Floor(math.Sqrt(float64(width*height)/float64(budget)))))
	}

	histogram := Histogram{Step: step}
	for y := 0; y < height; y += step {
		for x := 0; x < width; x += step {
			pixel := color.NRGBAModel.Convert(source.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.NRGBA)
			if pixel.A < 200 {
				continue
			}

			lab := colorx.RGBToOKLab(colorx.RGB{R: int(pixel.R), G: int(pixel.G), B: int(pixel.B)})
			chroma := math.Hypot(lab.A, lab.B)
			histogram.Total++

			structureWeight := 1.0
			if chroma > 0.14 {
				structureWeight = 0.2
			}
			luminanceIndex := clampInt(int(math.Floor(lab.L*LuminanceBinCount)), 0, LuminanceBinCount-1)
			luminance := &histogram.Luminance[luminanceIndex]
			luminance.N += structureWeight
			luminance.L += lab.L * structureWeight
			luminance.A += lab.A * structureWeight
			luminance.B += lab.B * structureWeight

			if chroma < chromaMin {
				continue
			}
			hue := math.Atan2(lab.B, lab.A) * 180 / math.Pi
			if hue < 0 {
				hue += 360
			}
			hueIndex := clampInt(int(math.Floor(hue/(360/HueBinCount))), 0, HueBinCount-1)
			band := 2
			if lab.L < 0.38 {
				band = 0
			} else if lab.L < 0.70 {
				band = 1
			}
			signal := &histogram.Signal[band*HueBinCount+hueIndex]
			chromaWeight := math.Pow(chroma/0.12, 2)
			signal.N++
			signal.WeightSum += chromaWeight
			signal.L += lab.L * chromaWeight
			signal.A += lab.A * chromaWeight
			signal.B += lab.B * chromaWeight
		}
	}
	if histogram.Total == 0 {
		histogram.Total = 1
	}
	return histogram
}

// Pool gives each still equal weight regardless of its resolution, matching
// the browser's 1000-sample normalization and source-bin attribution.
func Pool(histograms []Histogram) PooledHistogram {
	var pooled PooledHistogram
	for stillIndex, histogram := range histograms {
		total := histogram.Total
		if total == 0 {
			total = 1
		}
		normalization := 1000 / float64(total)
		for index, bin := range histogram.Luminance {
			out := &pooled.Luminance[index]
			out.N += bin.N * normalization
			out.L += bin.L * normalization
			out.A += bin.A * normalization
			out.B += bin.B * normalization
		}
		for index, bin := range histogram.Signal {
			if bin.N > 0 {
				pooled.BinStills[index] = append(pooled.BinStills[index], stillIndex)
			}
			out := &pooled.Signal[index]
			out.N += bin.N * normalization
			out.L += bin.L * normalization
			out.A += bin.A * normalization
			out.B += bin.B * normalization
			out.WeightSum += bin.WeightSum * normalization
		}
	}
	for _, bin := range pooled.Luminance {
		pooled.TotalN += bin.N
	}
	if pooled.TotalN == 0 {
		pooled.TotalN = 1
	}
	return pooled
}

// SignalCandidates averages each populated hue/lightness bin, ranks it by
// sqrt(area)*chroma^2, then merges neighboring colors that read as one.
func SignalCandidates(histogram PooledHistogram) []Candidate {
	candidates := make([]Candidate, 0, SignalBinCount)
	for index, bin := range histogram.Signal {
		if bin.N == 0 || bin.WeightSum == 0 || bin.N/histogram.TotalN < areaFloor {
			continue
		}
		lab := colorx.OKLab{
			L: bin.L / bin.WeightSum,
			A: bin.A / bin.WeightSum,
			B: bin.B / bin.WeightSum,
		}
		rgb := colorx.OKLabToRGB(lab)
		lch := colorx.RGBToLCH(rgb)
		area := bin.N / histogram.TotalN
		stills := append([]int(nil), histogram.BinStills[index]...)
		sort.Ints(stills)
		candidates = append(candidates, Candidate{
			RGB:      rgb,
			Area:     area,
			C:        lch.C,
			H:        lch.H,
			Salience: math.Sqrt(area) * math.Pow(lch.C/0.10, 2),
			Bin:      index,
			Band:     index / HueBinCount,
			HueBin:   index % HueBinCount,
			Stills:   stills,
		})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].Salience > candidates[j].Salience
	})
	merged := make([]Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		duplicate := false
		for _, existing := range merged {
			if colorx.HueGap(existing.H, candidate.H) < 16 &&
				math.Abs(colorx.RGBToLCH(existing.RGB).L-colorx.RGBToLCH(candidate.RGB).L) < 0.16 {
				duplicate = true
				break
			}
		}
		if !duplicate {
			merged = append(merged, candidate)
		}
	}
	return merged
}

// RankedPool keeps the most salient candidates that are visibly distinct.
func RankedPool(candidates []Candidate, limit int) []Candidate {
	pool := make([]Candidate, 0, min(limit, len(candidates)))
	for _, candidate := range candidates {
		distinct := true
		for _, existing := range pool {
			if colorx.DeltaE(existing.RGB, candidate.RGB) < 0.07 {
				distinct = false
				break
			}
		}
		if distinct {
			pool = append(pool, candidate)
		}
		if len(pool) == limit {
			break
		}
	}
	return pool
}

func clampInt(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
