package extract

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"scene-palette/internal/colorx"
)

type ColorPolicy string

const (
	ReadableUI ColorPolicy = "readable-ui"
	SourceHues ColorPolicy = "source-hues"
)

func NormalizeColorPolicy(policy ColorPolicy) ColorPolicy {
	if policy == SourceHues {
		return SourceHues
	}
	return ReadableUI
}

func ValidColorPolicy(policy ColorPolicy) bool {
	return policy == "" || policy == ReadableUI || policy == SourceHues
}

func ParseHex(value string) (colorx.RGB, bool) {
	if len(value) != 7 || value[0] != '#' {
		return colorx.RGB{}, false
	}
	number, err := strconv.ParseUint(value[1:], 16, 24)
	if err != nil {
		return colorx.RGB{}, false
	}
	return colorx.RGB{R: int(number >> 16), G: int(number >> 8 & 255), B: int(number & 255)}, true
}

// AccentInput is the final paint after contrast repair and lock application.
// It deliberately contains no shelf, naming, clock, or review state.
type AccentInput struct {
	RGB     colorx.RGB
	Raw     colorx.RGB
	HasRaw  bool
	Locked  bool
	Derived bool
}

type AccentRefinement struct {
	Colors       []colorx.RGB
	Repairs      [][]Repair
	Replacements map[int]AccentColor
	Notes        []string
}

// FinalizeAccents checks the final RGB colours together. Human locks are never
// changed. A bounded search reports unresolved pairs instead of claiming that
// no solution exists anywhere in the colour space.
func FinalizeAccents(input []AccentInput, observed []AccentColor, ground, panel colorx.RGB, policy ColorPolicy) AccentRefinement {
	result := AccentRefinement{
		Colors: make([]colorx.RGB, len(input)), Repairs: make([][]Repair, len(input)),
		Replacements: make(map[int]AccentColor), Notes: []string{},
	}
	for i, token := range input {
		result.Colors[i] = token.RGB
		result.Repairs[i] = []Repair{}
	}
	readable := func(color colorx.RGB) bool {
		return colorx.Contrast(color, ground) >= 3 && colorx.Contrast(color, panel) >= 3
	}
	others := func(index int) []colorx.RGB {
		out := make([]colorx.RGB, 0, len(input)-1)
		for i, color := range result.Colors {
			if i != index {
				out = append(out, color)
			}
		}
		return out
	}
	change := func(index int, next colorx.RGB, operation string) {
		before := result.Colors[index]
		if before != next {
			result.Repairs[index] = append(result.Repairs[index], Repair{
				Operation: operation, Against: "ground+panel", Target: 3,
				From: colorx.Hex(before), To: colorx.Hex(next),
			})
			result.Colors[index] = next
		}
	}
	for i, token := range input {
		if token.Locked || !token.HasRaw || token.Derived {
			continue
		}
		minimum := 0.07
		if NormalizeColorPolicy(policy) == SourceHues {
			minimum = 0
		}
		next := colorx.CloserContrastPair(token.Raw, result.Colors[i], ground, panel, 3, others(i), minimum)
		change(i, next, "source-distance")
	}
	closeToAnother := func(index int) bool {
		for i, color := range result.Colors {
			if i != index && colorx.DeltaE(result.Colors[index], color) < 0.07 {
				return true
			}
		}
		return false
	}
	if NormalizeColorPolicy(policy) == ReadableUI {
		order := []int{}
		for _, derived := range []bool{true, false} {
			for i, token := range input {
				if !token.Locked && token.Derived == derived {
					order = append(order, i)
				}
			}
		}
		for _, index := range order {
			if !closeToAnother(index) {
				continue
			}
			current := result.Colors[index]
			valid := func(candidate colorx.RGB) bool {
				if !readable(candidate) {
					return false
				}
				for _, other := range others(index) {
					if colorx.DeltaE(candidate, other) < 0.07 {
						return false
					}
				}
				return true
			}
			best, bestDistance, found := current, math.Inf(1), false
			var replacement *AccentColor
			// An unused observed paint is preferable to inventing another hue.
			for _, candidate := range observed {
				color, ok := colorx.ForceContrastPair(candidate.RGB, ground, panel, 3)
				if !ok || !valid(color) {
					continue
				}
				distance := colorx.DeltaE(current, color)
				if distance < bestDistance {
					copy := candidate
					copy.Repairs = append([]Repair{}, candidate.Repairs...)
					if color != candidate.RGB {
						copy.Repairs = append(copy.Repairs, Repair{Operation: "contrast", Against: "ground+panel", Target: 3, From: colorx.Hex(candidate.RGB), To: colorx.Hex(color)})
					}
					copy.RGB = color
					lch := colorx.RGBToLCH(color)
					copy.L, copy.C, copy.H = lch.L, lch.C, lch.H
					best, bestDistance, found, replacement = color, distance, true, &copy
				}
			}
			if found {
				result.Colors[index] = best
				result.Replacements[index] = *replacement
				result.Repairs[index] = []Repair{}
				continue
			}
			lch := colorx.RGBToLCH(current)
			// Native paint keeps its hue. Derived paint may explore nearby hues.
			offsets := []float64{0}
			if input[index].Derived {
				offsets = []float64{0, 15, -15, 30, -30, 60, -60, 90, -90, 120, -120, 180}
			}
			for _, offset := range offsets {
				for _, chroma := range []float64{lch.C, lch.C * 0.75, lch.C * 0.5, 0.08, 0.16, 0.24} {
					for lightness := 5; lightness <= 95; lightness += 2 {
						candidate := colorx.LCHToRGB(colorx.LCH{L: float64(lightness) / 100, C: chroma, H: lch.H + offset})
						if !valid(candidate) {
							continue
						}
						distance := colorx.DeltaE(current, candidate)
						if distance < bestDistance {
							best, bestDistance, found = candidate, distance, true
						}
					}
				}
			}
			if found {
				change(index, best, "distinctness")
			}
		}
	}
	pairs := []string{}
	for i, first := range result.Colors {
		for j := i + 1; j < len(result.Colors); j++ {
			if colorx.DeltaE(first, result.Colors[j]) < 0.07 {
				pairs = append(pairs, accentRoles[i].ID+"/"+accentRoles[j].ID)
			}
		}
	}
	if len(pairs) > 0 {
		prefix := "accent distinction remains limited after bounded search"
		if NormalizeColorPolicy(policy) == SourceHues {
			prefix = "source hues share similar accents"
		}
		result.Notes = append(result.Notes, fmt.Sprintf("%s — %s", prefix, strings.Join(pairs, ", ")))
	}
	return result
}

func sourceHueSets(candidates []AccentColor, structure StructureResult, maxSets int) [][]AccentColor {
	pool := rankedAccentPool(candidates, 4)
	if len(pool) >= 4 {
		return accentSets(candidates, maxSets)
	}
	seeds := append([]AccentColor{}, pool...)
	if len(seeds) == 0 {
		for _, role := range []string{"muted", "ink", "panel", "ground"} {
			raw, ok := ParseHex(structure.Origins[role].Raw)
			if !ok {
				raw = structure.Colors[role]
			}
			lch := colorx.RGBToLCH(raw)
			seeds = append(seeds, derivedAccent(lch.L, lch.C, lch.H, &DerivedSource{Method: "source-tonal-reuse", IdealHue: lch.H, MedianC: roundDecimal(lch.C, 4), MedianL: roundDecimal(lch.L, 4)}))
		}
	}
	for len(pool) < 4 {
		seed := seeds[len(pool)%len(seeds)]
		lch := colorx.RGBToLCH(seed.RGB)
		lightness := min(1, max(0, lch.L+[]float64{0, 0.08, -0.08, 0.16}[len(pool)]))
		pool = append(pool, derivedAccent(lightness, lch.C, lch.H, &DerivedSource{
			Method: "source-tonal-reuse", IdealHue: lch.H, MedianC: roundDecimal(lch.C, 4), MedianL: roundDecimal(lch.L, 4),
		}))
	}
	return [][]AccentColor{pool}
}

// SourceHueDeparture reports when readability repair leaves a recorded source
// hue family or introduces chroma into near-neutral paint. Locked manual paint
// is excluded by callers; this does not label its deliberate choice a failure.
func SourceHueDeparture(color colorx.RGB, raw, method string, hue, chroma float64) bool {
	if source, ok := ParseHex(raw); ok {
		lch := colorx.RGBToLCH(source)
		hue, chroma = lch.H, lch.C
	} else if method != "source-tonal-reuse" {
		return false
	}
	final := colorx.RGBToLCH(color)
	if chroma <= 0.000001 {
		return color.R != color.G || color.R != color.B
	}
	if chroma <= 0.01 {
		return final.C > 0.01
	}
	return final.C > 0.01 && colorx.HueGap(hue, final.H) > 15
}
