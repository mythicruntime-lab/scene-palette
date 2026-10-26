package extract

import (
	"math"
	"sort"
	"strconv"
	"strings"

	"scene-palette/internal/colorx"
)

type AccentRole struct {
	ID       string
	IdealHue float64
}

var accentRoles = []AccentRole{
	{ID: "observed", IdealHue: 205},
	{ID: "attention", IdealHue: 100},
	{ID: "rupture", IdealHue: 27},
	{ID: "gold", IdealHue: 65},
}

type SignalSource struct {
	Kind     string
	Method   string
	Raw      string
	HueBin   int
	Band     int
	Area     float64
	Salience float64
	Stills   []int
}

type DerivedSource struct {
	Method   string
	IdealHue float64
	Role     string
	MedianC  float64
	MedianL  float64
}

type AccentColor struct {
	RGB         colorx.RGB
	L           float64
	C           float64
	H           float64
	Area        float64
	Salience    float64
	Derived     bool
	Source      *SignalSource
	Repairs     []Repair
	DerivedFrom *DerivedSource
	sourceIndex int
}

type AccentStage struct {
	Filtered       []Candidate
	RoleCandidates []AccentColor
	Sets           [][]AccentColor
}

// BuildAccentStage filters raw paint against the structural colors, repairs it
// for both surfaces, generates the best four-color sets, and derives only the
// colors the image cannot supply distinctly.
func BuildAccentStage(rawCandidates []Candidate, structure StructureResult, maxSets int) AccentStage {
	filtered := make([]Candidate, 0, len(rawCandidates))
	for _, candidate := range rawCandidates {
		if colorx.DeltaE(candidate.RGB, structure.Colors["ground"]) > 0.10 &&
			colorx.DeltaE(candidate.RGB, structure.Colors["ink"]) > 0.10 {
			filtered = append(filtered, candidate)
		}
	}

	roleCandidates := make([]AccentColor, 0, len(filtered))
	for index, candidate := range filtered {
		onGround := colorx.ForceContrast(candidate.RGB, structure.Colors["ground"], 3.0)
		final := colorx.ForceContrast(onGround, structure.Colors["panel"], 3.0)
		repairs := make([]Repair, 0, 2)
		if colorx.Hex(onGround) != colorx.Hex(candidate.RGB) {
			repairs = append(repairs, Repair{
				Operation: "contrast",
				Against:   "ground",
				Target:    3.0,
				From:      colorx.Hex(candidate.RGB),
				To:        colorx.Hex(onGround),
			})
		}
		if colorx.Hex(final) != colorx.Hex(onGround) {
			repairs = append(repairs, Repair{
				Operation: "contrast",
				Against:   "panel",
				Target:    3.0,
				From:      colorx.Hex(onGround),
				To:        colorx.Hex(final),
			})
		}
		if colorx.Contrast(final, structure.Colors["ground"]) < 3 || colorx.Contrast(final, structure.Colors["panel"]) < 3 {
			joint, _ := colorx.ForceContrastPair(candidate.RGB, structure.Colors["ground"], structure.Colors["panel"], 3)
			if colorx.Hex(joint) != colorx.Hex(final) {
				repairs = append(repairs, Repair{Operation: "contrast", Against: "ground+panel", Target: 3, From: colorx.Hex(final), To: colorx.Hex(joint)})
			}
			final = joint
		}
		lch := colorx.RGBToLCH(final)
		accent := AccentColor{
			RGB:      final,
			L:        lch.L,
			C:        lch.C,
			H:        lch.H,
			Area:     candidate.Area,
			Salience: candidate.Salience,
			Source: &SignalSource{
				Kind:     "signal",
				Method:   "hue-bin · sqrt(area)·chroma²",
				Raw:      colorx.Hex(candidate.RGB),
				HueBin:   candidate.HueBin,
				Band:     candidate.Band,
				Area:     roundDecimal(candidate.Area, 6),
				Salience: roundDecimal(candidate.Salience, 6),
				Stills:   append([]int(nil), candidate.Stills...),
			},
			Repairs:     repairs,
			sourceIndex: index,
		}
		duplicate := false
		for _, existing := range roleCandidates {
			if colorx.DeltaE(existing.RGB, accent.RGB) < 0.02 {
				duplicate = true
				break
			}
		}
		if !duplicate {
			roleCandidates = append(roleCandidates, accent)
		}
	}

	return AccentStage{
		Filtered:       filtered,
		RoleCandidates: roleCandidates,
		Sets:           accentSets(roleCandidates, maxSets),
	}
}

// AssignRoleIndices returns set indices in observed, attention, rupture, gold
// order after checking all 24 permutations.
func AssignRoleIndices(accents []AccentColor) []int {
	permutations := indexPermutations([]int{0, 1, 2, 3})
	var best []int
	bestCost := math.Inf(1)
	for _, permutation := range permutations {
		cost := 0.0
		for roleIndex, role := range accentRoles {
			accentIndex := permutation[roleIndex]
			if accentIndex >= len(accents) {
				cost += 400
				continue
			}
			accent := accents[accentIndex]
			cost += colorx.HueGap(accent.H, role.IdealHue)
			if role.ID == "gold" {
				cost -= math.Min(accent.C, 0.25) * 12
			}
		}
		if cost < bestCost {
			bestCost = cost
			best = append([]int(nil), permutation...)
		}
	}
	return best
}

func accentSets(candidates []AccentColor, maxSets int) [][]AccentColor {
	pool := rankedAccentPool(candidates, 8)
	if len(pool) <= 4 {
		return [][]AccentColor{padAccentSet(pool, candidates)}
	}

	n := min(len(pool), 7)
	type scoredSet struct {
		colors []AccentColor
		score  float64
	}
	combinations := make([]scoredSet, 0, 35)
	for a := 0; a < n; a++ {
		for b := a + 1; b < n; b++ {
			for c := b + 1; c < n; c++ {
				for d := c + 1; d < n; d++ {
					set := []AccentColor{pool[a], pool[b], pool[c], pool[d]}
					score := scoreAccentSet(set)
					if score > 0 {
						combinations = append(combinations, scoredSet{colors: set, score: score})
					}
				}
			}
		}
	}
	sort.SliceStable(combinations, func(i, j int) bool {
		return combinations[i].score > combinations[j].score
	})

	sets := make([][]AccentColor, 0, maxSets)
	seen := make(map[string]bool)
	for _, combination := range combinations {
		signature := accentSetSignature(combination.colors)
		if seen[signature] {
			continue
		}
		seen[signature] = true
		sets = append(sets, combination.colors)
		if len(sets) >= maxSets {
			break
		}
	}
	if len(sets) == 0 {
		return [][]AccentColor{padAccentSet(pool[:min(4, len(pool))], candidates)}
	}
	return sets
}

func rankedAccentPool(candidates []AccentColor, limit int) []AccentColor {
	pool := make([]AccentColor, 0, min(limit, len(candidates)))
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

func scoreAccentSet(set []AccentColor) float64 {
	minimumHueGap := 180.0
	minimumDeltaE := 9.0
	for first := range set {
		for second := first + 1; second < len(set); second++ {
			minimumHueGap = math.Min(minimumHueGap, colorx.HueGap(set[first].H, set[second].H))
			minimumDeltaE = math.Min(minimumDeltaE, colorx.DeltaE(set[first].RGB, set[second].RGB))
		}
	}
	if minimumDeltaE < 0.07 {
		return -1
	}
	score := 0.0
	for _, color := range set {
		score += color.Salience
	}
	return score + math.Min(minimumHueGap, 90)/90*0.35 + math.Min(minimumDeltaE, 0.4)*0.5
}

func padAccentSet(set, candidates []AccentColor) []AccentColor {
	result := append([]AccentColor(nil), set...)
	for _, candidate := range candidates {
		if len(result) >= 4 {
			break
		}
		if containsSource(result, candidate.sourceIndex) {
			continue
		}
		distinct := true
		for _, existing := range result {
			if colorx.DeltaE(existing.RGB, candidate.RGB) < 0.10 {
				distinct = false
				break
			}
		}
		if distinct {
			result = append(result, candidate)
		}
	}

	if len(result) >= 4 {
		return result
	}
	medianC, medianL := 0.14, 0.62
	if len(result) > 0 {
		chromas := make([]float64, len(result))
		lightnesses := make([]float64, len(result))
		for index, color := range result {
			chromas[index] = color.C
			lightnesses[index] = color.L
		}
		sort.Float64s(chromas)
		sort.Float64s(lightnesses)
		medianC = chromas[len(chromas)>>1]
		medianL = lightnesses[len(lightnesses)>>1]
	}

	missing := make([]AccentRole, 0, len(accentRoles))
	for _, role := range accentRoles {
		present := false
		for _, color := range result {
			if colorx.HueGap(color.H, role.IdealHue) < 30 {
				present = true
				break
			}
		}
		if !present {
			missing = append(missing, role)
		}
	}
	for _, role := range missing {
		if len(result) >= 4 {
			break
		}
		result = append(result, derivedAccent(medianL, math.Max(medianC, 0.10), role.IdealHue, &DerivedSource{
			Method:   "role-ideal-hue",
			IdealHue: role.IdealHue,
			Role:     role.ID,
			MedianC:  roundDecimal(medianC, 4),
			MedianL:  roundDecimal(medianL, 4),
		}))
	}
	for len(result) < 4 {
		hue := float64((len(result) * 97) % 360)
		result = append(result, derivedAccent(medianL, medianC, hue, &DerivedSource{
			Method:   "hue-spread",
			IdealHue: hue,
			MedianC:  roundDecimal(medianC, 4),
			MedianL:  roundDecimal(medianL, 4),
		}))
	}
	return result
}

func derivedAccent(lightness, chroma, hue float64, source *DerivedSource) AccentColor {
	rgb := colorx.LCHToRGB(colorx.LCH{L: lightness, C: chroma, H: hue})
	lch := colorx.RGBToLCH(rgb)
	return AccentColor{
		RGB:         rgb,
		L:           lch.L,
		C:           lch.C,
		H:           lch.H,
		Derived:     true,
		Repairs:     []Repair{},
		DerivedFrom: source,
		sourceIndex: -1,
	}
}

func containsSource(colors []AccentColor, sourceIndex int) bool {
	for _, color := range colors {
		if color.sourceIndex == sourceIndex {
			return true
		}
	}
	return false
}

func accentSetSignature(set []AccentColor) string {
	parts := make([]int, len(set))
	for index, color := range set {
		parts[index] = int(math.Round(color.H / 14))
	}
	sort.Ints(parts)
	stringsParts := make([]string, len(parts))
	for index, value := range parts {
		stringsParts[index] = strconv.Itoa(value)
	}
	return strings.Join(stringsParts, ",")
}

func indexPermutations(values []int) [][]int {
	if len(values) == 0 {
		return [][]int{{}}
	}
	result := make([][]int, 0, 24)
	for index, value := range values {
		rest := append([]int(nil), values[:index]...)
		rest = append(rest, values[index+1:]...)
		for _, tail := range indexPermutations(rest) {
			permutation := make([]int, 1, len(values))
			permutation[0] = value
			permutation = append(permutation, tail...)
			result = append(result, permutation)
		}
	}
	return result
}

func roundDecimal(value float64, places int) float64 {
	factor := math.Pow10(places)
	return math.Round(value*factor) / factor
}
