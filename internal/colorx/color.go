// Package colorx is the incremental Go port of Scene Palette's stateless
// sRGB, OKLab, OKLCh, luminance, and contrast functions.
package colorx

import (
	"fmt"
	"math"
	"sort"
)

type RGB struct {
	R int
	G int
	B int
}

type OKLab struct {
	L float64
	A float64
	B float64
}

type LCH struct {
	L float64
	C float64
	H float64
}

func clamp(value, low, high float64) float64 {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func toLinear(value int) float64 {
	if uint(value) < uint(len(browserLinear8Bit)) {
		return math.Float64frombits(browserLinear8Bit[value])
	}
	channel := float64(value) / 255
	if channel <= 0.04045 {
		return channel / 12.92
	}
	return math.Pow((channel+0.055)/1.055, 2.4)
}

func toSRGB(value float64) float64 {
	if value <= 0.0031308 {
		return value * 12.92
	}
	return 1.055*math.Pow(value, 1.0/2.4) - 0.055
}

// The browser math helpers mirror the fdlibm implementations used by the V8
// version that generated Scene Palette's JavaScript parity fixtures. Go's
// standard-library functions are equally accurate, but can land on neighboring
// floats and change an otherwise tied semantic-role assignment.
//
// The algorithms are adapted from fdlibm:
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
// Permission to use, copy, modify, and distribute this software is freely
// granted, provided that this notice is preserved.
func browserCbrt(value float64) float64 {
	const (
		b1 = uint32(715094163)
		b2 = uint32(696219795)
		p0 = 1.87595182427177009643
		p1 = -1.88497979543377169875
		p2 = 1.621429720105354466140
		p3 = -0.758397934778766047437
		p4 = 0.145996192886612446982
	)

	bits := math.Float64bits(value)
	sign := bits & (uint64(1) << 63)
	absBits := bits &^ (uint64(1) << 63)
	high := uint32(absBits >> 32)
	low := uint32(absBits)
	if high >= 0x7ff00000 {
		return value + value
	}
	if high|low == 0 {
		return value
	}

	var estimate float64
	if high < 0x00100000 {
		scaled := math.Float64frombits(0x4350000000000000) * value
		high = uint32(math.Float64bits(scaled)>>32) & 0x7fffffff
		estimate = math.Float64frombits(sign | uint64(high/3+b2)<<32)
	} else {
		estimate = math.Float64frombits(sign | uint64(high/3+b1)<<32)
	}

	ratio := estimate * estimate * (estimate / value)
	estimate *= (p0 + ratio*(p1+ratio*p2)) + ratio*ratio*ratio*(p3+ratio*p4)
	estimate = math.Float64frombits((math.Float64bits(estimate) + 0x80000000) & 0xffffffffc0000000)
	square := estimate * estimate
	ratio = value / square
	double := estimate + estimate
	ratio = (ratio - estimate) / (double + ratio)
	return estimate + estimate*ratio
}

func browserAtan(value float64) float64 {
	atanHigh := [...]float64{
		4.63647609000806093515e-01,
		7.85398163397448278999e-01,
		9.82793723247329054082e-01,
		1.57079632679489655800e+00,
	}
	atanLow := [...]float64{
		2.26987774529616870924e-17,
		3.06161699786838301793e-17,
		1.39033110312309984516e-17,
		6.12323399573676603587e-17,
	}
	coefficients := [...]float64{
		3.33333333333329318027e-01,
		-1.99999999998764832476e-01,
		1.42857142725034663711e-01,
		-1.11111104054623557880e-01,
		9.09088713343650656196e-02,
		-7.69187620504482999495e-02,
		6.66107313738753120669e-02,
		-5.83357013379057348645e-02,
		4.97687799461593236017e-02,
		-3.65315727442169155270e-02,
		1.62858201153657823623e-02,
	}

	high := int32(uint32(math.Float64bits(value) >> 32))
	magnitudeHigh := uint32(high) & 0x7fffffff
	if magnitudeHigh >= 0x44100000 {
		low := uint32(math.Float64bits(value))
		if magnitudeHigh > 0x7ff00000 || (magnitudeHigh == 0x7ff00000 && low != 0) {
			return value + value
		}
		if high > 0 {
			return atanHigh[3] + atanLow[3]
		}
		return -atanHigh[3] - atanLow[3]
	}

	id := -1
	if magnitudeHigh < 0x3fdc0000 {
		if magnitudeHigh < 0x3e400000 && 1.0e300+value > 1 {
			return value
		}
	} else {
		value = math.Abs(value)
		switch {
		case magnitudeHigh < 0x3fe60000:
			id = 0
			value = (2*value - 1) / (2 + value)
		case magnitudeHigh < 0x3ff30000:
			id = 1
			value = (value - 1) / (value + 1)
		case magnitudeHigh < 0x40038000:
			id = 2
			value = (value - 1.5) / (1 + 1.5*value)
		default:
			id = 3
			value = -1 / value
		}
	}

	square := value * value
	fourth := square * square
	odd := square * (coefficients[0] + fourth*(coefficients[2]+fourth*(coefficients[4]+fourth*(coefficients[6]+fourth*(coefficients[8]+fourth*coefficients[10])))))
	even := fourth * (coefficients[1] + fourth*(coefficients[3]+fourth*(coefficients[5]+fourth*(coefficients[7]+fourth*coefficients[9]))))
	if id < 0 {
		return value - value*(odd+even)
	}
	result := atanHigh[id] - ((value*(odd+even) - atanLow[id]) - value)
	if high < 0 {
		return -result
	}
	return result
}

func browserAtan2(y, x float64) float64 {
	const (
		quarterPi = 7.8539816339744827900e-01
		halfPi    = 1.5707963267948965580e+00
		pi        = 3.1415926535897931160e+00
		piLow     = 1.2246467991473531772e-16
		tiny      = 1.0e-300
	)

	if math.IsNaN(x) || math.IsNaN(y) {
		return x + y
	}
	if x == 1 {
		return browserAtan(y)
	}

	quadrant := 0
	if math.Signbit(y) {
		quadrant |= 1
	}
	if math.Signbit(x) {
		quadrant |= 2
	}
	if y == 0 {
		switch quadrant {
		case 0, 1:
			return y
		case 2:
			return pi + tiny
		default:
			return -pi - tiny
		}
	}
	if x == 0 {
		if math.Signbit(y) {
			return -halfPi - tiny
		}
		return halfPi + tiny
	}
	if math.IsInf(x, 0) {
		if math.IsInf(y, 0) {
			switch quadrant {
			case 0:
				return quarterPi + tiny
			case 1:
				return -quarterPi - tiny
			case 2:
				return 3*quarterPi + tiny
			default:
				return -3*quarterPi - tiny
			}
		}
		switch quadrant {
		case 0:
			return 0
		case 1:
			return math.Copysign(0, -1)
		case 2:
			return pi + tiny
		default:
			return -pi - tiny
		}
	}
	if math.IsInf(y, 0) {
		if math.Signbit(y) {
			return -halfPi - tiny
		}
		return halfPi + tiny
	}

	xHigh := int32(uint32(math.Float64bits(x)>>32)) & 0x7fffffff
	yHigh := int32(uint32(math.Float64bits(y)>>32)) & 0x7fffffff
	exponentGap := (yHigh - xHigh) >> 20
	var angle float64
	if exponentGap > 60 {
		angle = halfPi + 0.5*piLow
		quadrant &= 1
	} else if math.Signbit(x) && exponentGap < -60 {
		angle = 0
	} else {
		angle = browserAtan(math.Abs(y / x))
	}
	switch quadrant {
	case 0:
		return angle
	case 1:
		return -angle
	case 2:
		return pi - (angle - piLow)
	default:
		return (angle - piLow) - pi
	}
}

func RGBToOKLab(color RGB) OKLab {
	r := toLinear(color.R)
	g := toLinear(color.G)
	b := toLinear(color.B)
	l := browserCbrt(0.4122214708*r + 0.5363325363*g + 0.0514459929*b)
	m := browserCbrt(0.2119034982*r + 0.6806995451*g + 0.1073969566*b)
	s := browserCbrt(0.0883024619*r + 0.2817188376*g + 0.6299787005*b)
	return OKLab{
		L: 0.2104542553*l + 0.7936177850*m - 0.0040720468*s,
		A: 1.9779984951*l - 2.4285922050*m + 0.4505937099*s,
		B: 0.0259040371*l + 0.7827717662*m - 0.8086757660*s,
	}
}

func oklabToRaw(color OKLab) [3]float64 {
	l0 := color.L + 0.3963377774*color.A + 0.2158037573*color.B
	m0 := color.L - 0.1055613458*color.A - 0.0638541728*color.B
	s0 := color.L - 0.0894841775*color.A - 1.2914855480*color.B
	l := l0 * l0 * l0
	m := m0 * m0 * m0
	s := s0 * s0 * s0
	return [3]float64{
		toSRGB(4.0767416621*l - 3.3077115913*m + 0.2309699292*s),
		toSRGB(-1.2684380046*l + 2.6097574011*m - 0.3413193965*s),
		toSRGB(-0.0041960863*l - 0.7034186147*m + 1.7076147010*s),
	}
}

func inGamut(raw [3]float64) bool {
	for _, channel := range raw {
		if channel < -0.001 || channel > 1.001 {
			return false
		}
	}
	return true
}

func rawToRGB(raw [3]float64) RGB {
	return RGB{
		R: int(math.Round(clamp(raw[0], 0, 1) * 255)),
		G: int(math.Round(clamp(raw[1], 0, 1) * 255)),
		B: int(math.Round(clamp(raw[2], 0, 1) * 255)),
	}
}

// OKLabToRGB converts directly to sRGB and clamps out-of-gamut channels. This
// is the path the histogram candidate averaging uses in the browser.
func OKLabToRGB(color OKLab) RGB {
	return rawToRGB(oklabToRaw(color))
}

// LCHToRGB reduces chroma until the requested color fits sRGB, matching the
// browser implementation's 22-step binary search.
func LCHToRGB(color LCH) RGB {
	lightness := clamp(color.L, 0, 1)
	radians := color.H * math.Pi / 180
	cosine := math.Cos(radians)
	sine := math.Sin(radians)
	raw := oklabToRaw(OKLab{L: lightness, A: color.C * cosine, B: color.C * sine})
	if !inGamut(raw) {
		low, high := 0.0, color.C
		for range 22 {
			middle := (low + high) / 2
			candidate := oklabToRaw(OKLab{L: lightness, A: middle * cosine, B: middle * sine})
			if inGamut(candidate) {
				low = middle
				raw = candidate
			} else {
				high = middle
			}
		}
	}
	return rawToRGB(raw)
}

func OKLabToLCH(color OKLab) LCH {
	chroma := math.Hypot(color.A, color.B)
	hue := browserAtan2(color.B, color.A) * 180 / math.Pi
	if hue < 0 {
		hue += 360
	}
	return LCH{L: color.L, C: chroma, H: hue}
}

func RGBToLCH(color RGB) LCH {
	return OKLabToLCH(RGBToOKLab(color))
}

func HueGap(a, b float64) float64 {
	difference := math.Mod(math.Abs(a-b), 360)
	if difference > 180 {
		return 360 - difference
	}
	return difference
}

func DeltaE(a, b RGB) float64 {
	first := RGBToOKLab(a)
	second := RGBToOKLab(b)
	return math.Sqrt(
		math.Pow(first.L-second.L, 2) +
			math.Pow(first.A-second.A, 2) +
			math.Pow(first.B-second.B, 2),
	)
}

func RelativeLuminance(color RGB) float64 {
	return 0.2126*toLinear(color.R) + 0.7152*toLinear(color.G) + 0.0722*toLinear(color.B)
}

func Contrast(a, b RGB) float64 {
	first := RelativeLuminance(a) + 0.05
	second := RelativeLuminance(b) + 0.05
	if first > second {
		return first / second
	}
	return second / first
}

func ForceContrast(color, background RGB, target float64) RGB {
	if Contrast(color, background) >= target {
		return color
	}
	lch := RGBToLCH(color)
	up := RelativeLuminance(background) < 0.18
	best := color
	bestContrast := Contrast(color, background)
	for step := 1; step <= 100; step++ {
		direction := 1.0
		if !up {
			direction = -1
		}
		lightness := clamp(lch.L+direction*float64(step)*0.01, 0, 1)
		candidate := LCHToRGB(LCH{L: lightness, C: lch.C, H: lch.H})
		ratio := Contrast(candidate, background)
		if ratio > bestContrast {
			bestContrast = ratio
			best = candidate
		}
		if ratio >= target {
			return candidate
		}
		if lightness <= 0 || lightness >= 1 {
			break
		}
	}
	return best
}

// ForceContrastPair preserves the established sequential repair whenever it
// reads on both surfaces. Its fallback searches both lightness directions,
// then neutral colors and the full 8-bit RGB lattice, without accepting a
// one-surface result.
// False means the returned legacy result is best effort, not a readability pass.
func ForceContrastPair(color, ground, panel RGB, target float64) (RGB, bool) {
	passes := func(candidate RGB) bool {
		return Contrast(candidate, ground) >= target && Contrast(candidate, panel) >= target
	}
	legacy := ForceContrast(ForceContrast(color, ground, target), panel, target)
	if passes(legacy) {
		return legacy, true
	}
	lch := RGBToLCH(color)
	preferred := 1.0
	if RelativeLuminance(panel) >= 0.18 {
		preferred = -1
	}
	for step := 1; step <= 100; step++ {
		for _, direction := range []float64{preferred, -preferred} {
			candidate := LCHToRGB(LCH{
				L: clamp(lch.L+direction*float64(step)*0.01, 0, 1),
				C: lch.C, H: lch.H,
			})
			if passes(candidate) {
				return candidate, true
			}
		}
	}
	best, distance, found := legacy, 0, false
	for gray := 0; gray <= 255; gray++ {
		candidate := RGB{R: gray, G: gray, B: gray}
		if !passes(candidate) {
			continue
		}
		r, g, b := gray-color.R, gray-color.G, gray-color.B
		nextDistance := r*r + g*g + b*b
		if !found || nextDistance < distance {
			best, distance, found = candidate, nextDistance, true
		}
	}
	if found {
		return best, true
	}
	if exact, ok := contrastPairRGB(color, ground, panel, target); ok {
		return exact, true
	}
	return legacy, false
}

type luminanceInterval struct {
	low, high float64
}

// contrastPairRGB covers narrow feasible bands that contain an 8-bit color
// but no neutral gray or hue-preserving lightness step. Relative luminance is
// monotonic in each channel, so enumerating red/green and bounding blue gives
// the nearest representable RGB without enumerating all 16 million colors.
func contrastPairRGB(color, ground, panel RGB, target float64) (RGB, bool) {
	const boundaryTolerance = 1e-15
	intervals := []luminanceInterval{{0, 1}}
	for _, surface := range []RGB{ground, panel} {
		y := RelativeLuminance(surface)
		allowed := []luminanceInterval{
			{0, math.Min(1, (y+0.05)/target-0.05)},
			{math.Max(0, target*(y+0.05)-0.05), 1},
		}
		next := []luminanceInterval{}
		for _, interval := range intervals {
			for _, region := range allowed {
				low, high := math.Max(interval.low, region.low), math.Min(interval.high, region.high)
				if low <= high+boundaryTolerance {
					next = append(next, luminanceInterval{low, high})
				}
			}
		}
		intervals = next
	}
	if len(intervals) == 0 {
		return RGB{}, false
	}
	var linear [256]float64
	for value := range linear {
		linear[value] = toLinear(value)
	}
	best, bestDistance, found := RGB{}, 0, false
	for r := 0; r <= 255; r++ {
		for g := 0; g <= 255; g++ {
			base := 0.2126*linear[r] + 0.7152*linear[g]
			var candidates [20]int
			count := 0
			for _, interval := range intervals {
				// An inclusive lower and upper blue bound, with tolerance used
				// only to include candidate endpoints, never to grant a pass.
				low, high := 0, 256
				for low < high {
					mid := (low + high) / 2
					if base+0.0722*linear[mid] < interval.low-boundaryTolerance {
						low = mid + 1
					} else {
						high = mid
					}
				}
				first := low
				low, high = 0, 256
				for low < high {
					mid := (low + high) / 2
					if base+0.0722*linear[mid] <= interval.high+boundaryTolerance {
						low = mid + 1
					} else {
						high = mid
					}
				}
				last := low - 1
				if first > last {
					continue
				}
				closest := min(max(color.B, first), last)
				for _, b := range []int{closest, closest - 1, closest + 1, first, last} {
					if b >= first && b <= last {
						candidates[count] = b
						count++
					}
				}
			}
			sort.Ints(candidates[:count])
			for index, b := range candidates[:count] {
				if index > 0 && b == candidates[index-1] {
					continue
				}
				candidate := RGB{R: r, G: g, B: b}
				if Contrast(candidate, ground) < target || Contrast(candidate, panel) < target {
					continue
				}
				dr, dg, db := r-color.R, g-color.G, b-color.B
				distance := dr*dr + dg*dg + db*db
				if !found || distance < bestDistance {
					best, bestDistance, found = candidate, distance, true
				}
			}
		}
	}
	return best, found
}

func CapChroma(color RGB, maximum float64) RGB {
	lch := RGBToLCH(color)
	if lch.C <= maximum {
		return color
	}
	return LCHToRGB(LCH{L: lch.L, C: maximum, H: lch.H})
}

func Hex(color RGB) string {
	channel := func(value int) int {
		if value < 0 {
			return 0
		}
		if value > 255 {
			return 255
		}
		return value
	}
	return fmt.Sprintf("#%02X%02X%02X", channel(color.R), channel(color.G), channel(color.B))
}
