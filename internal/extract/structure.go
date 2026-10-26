package extract

import (
	"fmt"
	"math"

	"scene-palette/internal/colorx"
)

type Polarity string

const (
	PolarityAuto  Polarity = "auto"
	PolarityDark  Polarity = "dark"
	PolarityLight Polarity = "light"
)

type Repair struct {
	Operation string
	Against   string
	Target    float64
	MinDeltaL float64
	From      string
	To        string
}

type StructureOrigin struct {
	Kind      string
	Method    string
	Quantile  [2]float64
	Raw       string
	ChromaCap float64
	Capped    bool
	Repairs   []Repair
}

type StructureResult struct {
	Mode       Polarity
	MedianL    float64
	AutoLight  bool
	LightScene bool
	Colors     map[string]colorx.RGB
	Origins    map[string]StructureOrigin
	Notes      []string
}

type structureRole struct {
	ID  string
	Cap float64
}

var structureRoles = []structureRole{
	{ID: "ground", Cap: 0.055},
	{ID: "panel", Cap: 0.055},
	{ID: "ink", Cap: 0.045},
	{ID: "muted", Cap: 0.070},
}

var structureQuantiles = map[Polarity]map[string][2]float64{
	PolarityDark: {
		"ground": {0.00, 0.10},
		"panel":  {0.13, 0.32},
		"ink":    {0.93, 1.00},
		"muted":  {0.44, 0.70},
	},
	PolarityLight: {
		"ground": {0.90, 1.00},
		"panel":  {0.70, 0.88},
		"ink":    {0.00, 0.06},
		"muted":  {0.34, 0.60},
	},
}

// QuantileLightness returns the midpoint of the luminance bin that crosses q.
func QuantileLightness(histogram PooledHistogram, q float64) float64 {
	seen := 0.0
	target := histogram.TotalN * q
	for index, bin := range histogram.Luminance {
		seen += bin.N
		if seen >= target {
			return (float64(index) + 0.5) / LuminanceBinCount
		}
	}
	return 1
}

// QuantileColor averages the OKLab contributions overlapping a luminance
// quantile range, then converts that mean directly back to clamped sRGB.
func QuantileColor(histogram PooledHistogram, q0, q1 float64) (colorx.RGB, bool) {
	low := histogram.TotalN * q0
	high := histogram.TotalN * q1
	seen, count := 0.0, 0.0
	lightness, a, b := 0.0, 0.0, 0.0
	for _, bin := range histogram.Luminance {
		if bin.N == 0 {
			continue
		}
		start := seen
		end := seen + bin.N
		seen = end
		overlap := math.Min(end, high) - math.Max(start, low)
		if overlap <= 0 {
			continue
		}
		fraction := overlap / bin.N
		count += bin.N * fraction
		lightness += bin.L * fraction
		a += bin.A * fraction
		b += bin.B * fraction
	}
	if count == 0 {
		return colorx.RGB{}, false
	}
	return colorx.OKLabToRGB(colorx.OKLab{
		L: lightness / count,
		A: a / count,
		B: b / count,
	}), true
}

// BuildStructure reproduces the browser's polarity choice, structural
// quantiles, chroma caps, surface separation, and contrast repairs.
func BuildStructure(histogram PooledHistogram, mode Polarity) StructureResult {
	if mode != PolarityDark && mode != PolarityLight {
		mode = PolarityAuto
	}
	median := QuantileLightness(histogram, 0.5)
	autoLight := median > 0.62
	lightScene := autoLight
	if mode != PolarityAuto {
		lightScene = mode == PolarityLight
	}
	quantileMode := PolarityDark
	if lightScene {
		quantileMode = PolarityLight
	}

	result := StructureResult{
		Mode:       mode,
		MedianL:    median,
		AutoLight:  autoLight,
		LightScene: lightScene,
		Colors:     make(map[string]colorx.RGB, len(structureRoles)),
		Origins:    make(map[string]StructureOrigin, len(structureRoles)),
		Notes:      []string{},
	}
	read := "dark"
	if autoLight {
		read = "bright"
	}
	built := "dark"
	if lightScene {
		built = "light"
	}
	if mode != PolarityAuto && lightScene != autoLight {
		result.Notes = append(result.Notes, fmt.Sprintf(
			"forced %s — the still reads %s (median L %.2f)", built, read, median,
		))
	} else {
		result.Notes = append(result.Notes, fmt.Sprintf(
			"%s still (median L %.2f) — built a %s plate", read, median, built,
		))
	}
	for _, role := range structureRoles {
		quantiles := structureQuantiles[quantileMode][role.ID]
		raw, ok := QuantileColor(histogram, quantiles[0], quantiles[1])
		if !ok {
			raw = colorx.RGB{R: 24, G: 22, B: 18}
		}
		capped := colorx.CapChroma(raw, role.Cap)
		result.Colors[role.ID] = capped
		result.Origins[role.ID] = StructureOrigin{
			Kind:      "structure",
			Method:    "luminance-quantile",
			Quantile:  quantiles,
			Raw:       colorx.Hex(raw),
			ChromaCap: role.Cap,
			Capped:    colorx.Hex(capped) != colorx.Hex(raw),
			Repairs:   []Repair{},
		}
	}

	direction := 1.0
	if lightScene {
		direction = -1
	}
	groundLCH := colorx.RGBToLCH(result.Colors["ground"])
	panelLCH := colorx.RGBToLCH(result.Colors["panel"])
	if direction*(panelLCH.L-groundLCH.L) < 0.035 {
		panel := colorx.LCHToRGB(colorx.LCH{
			L: clampFloat(groundLCH.L+direction*0.055, 0, 1),
			C: math.Min(groundLCH.C, 0.05),
			H: groundLCH.H,
		})
		applyRepair(&result, "panel", panel, Repair{
			Operation: "separate",
			Against:   "ground",
			MinDeltaL: 0.035,
		})
		result.Notes = append(result.Notes, "panel stepped off ground — the still had no separate surface tone")
	}

	// Opposite-polarity quantiles can straddle an unreadable luminance gap.
	// Adjust generated surfaces only when no shared body-ink color is found.
	if _, readable := colorx.ForceContrastPair(result.Colors["ink"], result.Colors["ground"], result.Colors["panel"], 4.5); !readable {
		endpoint := colorx.RGB{R: 255, G: 255, B: 255}
		note := "surfaces adjusted for a shared light ink"
		if lightScene {
			endpoint = colorx.RGB{}
			note = "surfaces adjusted for a shared dark ink"
		}
		for _, role := range []string{"ground", "panel"} {
			next := colorx.ForceContrast(result.Colors[role], endpoint, 4.5)
			applyRepair(&result, role, next, Repair{Operation: "contrast", Against: colorx.Hex(endpoint), Target: 4.5})
		}
		groundLCH = colorx.RGBToLCH(result.Colors["ground"])
		panelLCH = colorx.RGBToLCH(result.Colors["panel"])
		if direction*(panelLCH.L-groundLCH.L) < 0.035 {
			ground := colorx.LCHToRGB(colorx.LCH{
				L: clampFloat(panelLCH.L-direction*0.055, 0, 1),
				C: groundLCH.C, H: groundLCH.H,
			})
			ground = colorx.ForceContrast(ground, endpoint, 4.5)
			applyRepair(&result, "ground", ground, Repair{Operation: "separate", Against: "panel", MinDeltaL: 0.035})
		}
		result.Notes = append(result.Notes, note)
	}

	inkInput := result.Colors["ink"]
	inkBefore := colorx.Contrast(result.Colors["ink"], result.Colors["ground"])
	ink := colorx.ForceContrast(result.Colors["ink"], result.Colors["ground"], 4.5)
	applyRepair(&result, "ink", ink, Repair{Operation: "contrast", Against: "ground", Target: 4.5})
	ink = colorx.ForceContrast(result.Colors["ink"], result.Colors["panel"], 4.5)
	applyRepair(&result, "ink", ink, Repair{Operation: "contrast", Against: "panel", Target: 4.5})
	if colorx.Contrast(ink, result.Colors["ground"]) < 4.5 || colorx.Contrast(ink, result.Colors["panel"]) < 4.5 {
		ink, _ = colorx.ForceContrastPair(inkInput, result.Colors["ground"], result.Colors["panel"], 4.5)
		applyRepair(&result, "ink", ink, Repair{Operation: "contrast", Against: "ground+panel", Target: 4.5})
	}
	if inkBefore < 4.5 {
		result.Notes = append(result.Notes, fmt.Sprintf(
			"ink lifted to %.1f:1 for body copy",
			colorx.Contrast(result.Colors["ink"], result.Colors["ground"]),
		))
	}

	mutedInput := result.Colors["muted"]
	muted := colorx.ForceContrast(mutedInput, result.Colors["ground"], 3.0)
	applyRepair(&result, "muted", muted, Repair{Operation: "contrast", Against: "ground", Target: 3.0})
	muted = colorx.ForceContrast(result.Colors["muted"], result.Colors["panel"], 3.0)
	applyRepair(&result, "muted", muted, Repair{Operation: "contrast", Against: "panel", Target: 3.0})
	if colorx.Contrast(muted, result.Colors["ground"]) < 3 || colorx.Contrast(muted, result.Colors["panel"]) < 3 {
		muted, _ = colorx.ForceContrastPair(mutedInput, result.Colors["ground"], result.Colors["panel"], 3)
		applyRepair(&result, "muted", muted, Repair{Operation: "contrast", Against: "ground+panel", Target: 3})
	}

	return result
}

func applyRepair(result *StructureResult, role string, next colorx.RGB, repair Repair) {
	current := result.Colors[role]
	if colorx.Hex(current) == colorx.Hex(next) {
		return
	}
	repair.From = colorx.Hex(current)
	repair.To = colorx.Hex(next)
	origin := result.Origins[role]
	origin.Repairs = append(origin.Repairs, repair)
	result.Origins[role] = origin
	result.Colors[role] = next
}

func clampFloat(value, low, high float64) float64 {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
