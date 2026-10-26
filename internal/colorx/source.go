package colorx

// CloserContrastPair reduces substantial source changes over a fixed,
// deterministic set of lightness/chroma candidates. The current readable
// result is always retained as a candidate; this is a bounded search, not a
// claim of a globally closest colour. Existing distinctions may improve but
// cannot be reduced below the requested threshold by this refinement.
func CloserContrastPair(raw, current, ground, panel RGB, target float64, others []RGB, minimumDistance float64) RGB {
	if Contrast(current, ground) < target || Contrast(current, panel) < target || DeltaE(raw, current) <= 0.10 {
		return current
	}
	best, bestDistance := current, DeltaE(raw, current)
	accept := func(candidate RGB) {
		if Contrast(candidate, ground) < target || Contrast(candidate, panel) < target {
			return
		}
		for _, other := range others {
			minimum := min(minimumDistance, DeltaE(current, other))
			if DeltaE(candidate, other) < minimum {
				return
			}
		}
		distance := DeltaE(raw, candidate)
		if distance < bestDistance {
			best, bestDistance = candidate, distance
		}
	}
	accept(raw)
	lch := RGBToLCH(raw)
	for _, scale := range []float64{1, 0.9, 0.75, 0.5, 0.25, 0} {
		for lightness := 0; lightness <= 100; lightness++ {
			accept(LCHToRGB(LCH{L: float64(lightness) / 100, C: lch.C * scale, H: lch.H}))
		}
	}
	return best
}
