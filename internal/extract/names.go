package extract

import "scene-palette/internal/colorx"

type neutralName struct {
	Max  float64
	Name string
}

var neutralNames = []neutralName{
	{Max: 0.16, Name: "pitch"},
	{Max: 0.27, Name: "bitumen"},
	{Max: 0.38, Name: "lacquer"},
	{Max: 0.50, Name: "graphite"},
	{Max: 0.62, Name: "ash"},
	{Max: 0.74, Name: "umber"},
	{Max: 0.86, Name: "linen"},
	{Max: 1.01, Name: "bone"},
}

type hueName struct {
	Max   float64
	Names [4]string
}

var hueNames = []hueName{
	{Max: 20, Names: [4]string{"", "oxblood", "crimson", "coral"}},
	{Max: 42, Names: [4]string{"", "rust", "ember", "salmon"}},
	{Max: 62, Names: [4]string{"", "umber", "ochre", "sand"}},
	{Max: 82, Names: [4]string{"", "bronze", "amber", "wheat"}},
	{Max: 102, Names: [4]string{"", "olive", "gold", "straw"}},
	{Max: 126, Names: [4]string{"", "moss", "lime", "acid"}},
	{Max: 152, Names: [4]string{"", "pine", "verdigris", "mint"}},
	{Max: 178, Names: [4]string{"", "jade", "emerald", "seafoam"}},
	{Max: 205, Names: [4]string{"", "petrol", "teal", "ice"}},
	{Max: 232, Names: [4]string{"", "abyss", "cyan", "vapor"}},
	{Max: 262, Names: [4]string{"", "cobalt", "signal", "sky"}},
	{Max: 292, Names: [4]string{"", "indigo", "night", "periwinkle"}},
	{Max: 320, Names: [4]string{"", "aubergine", "violet", "orchid"}},
	{Max: 345, Names: [4]string{"", "plum", "magenta", "lilac"}},
	{Max: 361, Names: [4]string{"", "oxblood", "neon", "rose"}},
}

// NameColor returns the first project-vocabulary name for a color.
func NameColor(color colorx.RGB) string {
	return nameColor(color, nil)
}

// NameColorUnique chooses and reserves a name using the same vocabulary order
// as fresh extraction. Live rebuilds seed used with names kept by human locks.
func NameColorUnique(color colorx.RGB, used map[string]bool) string {
	return nameColor(color, used)
}

func nameColor(color colorx.RGB, used map[string]bool) string {
	lch := colorx.RGBToLCH(color)
	tier := 2
	if lch.L < 0.42 {
		tier = 1
	} else if lch.L > 0.74 {
		tier = 3
	}

	var options []string
	if lch.C < 0.042 {
		index := 0
		for index < len(neutralNames) && lch.L >= neutralNames[index].Max {
			index++
		}
		if index >= len(neutralNames) {
			index = len(neutralNames) - 1
		}
		at := func(candidate int) string {
			candidate = clampInt(candidate, 0, len(neutralNames)-1)
			return neutralNames[candidate].Name
		}
		options = []string{
			neutralFor(lch.L), at(index - 1), at(index + 1), at(index - 2), at(index + 2),
		}
	} else {
		index := 0
		for index < len(hueNames) && lch.H >= hueNames[index].Max {
			index++
		}
		if index >= len(hueNames) {
			index = len(hueNames) - 1
		}
		secondTier := 3
		if tier == 3 {
			secondTier = 2
		}
		thirdTier := 1
		if tier == 1 {
			thirdTier = 2
		}
		previous := (index - 1 + len(hueNames)) % len(hueNames)
		next := (index + 1) % len(hueNames)
		options = []string{
			hueNames[index].Names[tier],
			hueNames[index].Names[secondTier],
			hueNames[index].Names[thirdTier],
			hueNames[previous].Names[tier],
			hueNames[next].Names[tier],
			neutralFor(lch.L),
		}
	}

	if used == nil {
		return options[0]
	}
	for _, option := range options {
		if option != "" && !used[option] {
			used[option] = true
			return option
		}
	}
	used[options[0]] = true
	return options[0]
}

func neutralFor(lightness float64) string {
	for _, neutral := range neutralNames {
		if lightness < neutral.Max {
			return neutral.Name
		}
	}
	return "bone"
}
