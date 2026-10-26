package extract

import (
	"fmt"
	"strings"

	"scene-palette/internal/colorx"
)

type TokenOrigin struct {
	Kind      string
	Method    string
	Quantile  [2]float64
	Raw       string
	ChromaCap float64
	Capped    bool
	HueBin    int
	Band      int
	Area      float64
	Salience  float64
	Stills    []int
	IdealHue  float64
	Role      string
	MedianC   float64
	MedianL   float64
	Reason    string
	Repairs   []Repair
}

type PlateToken struct {
	ID      string
	Name    string
	Job     string
	RGB     colorx.RGB
	Hex     string
	Locked  bool
	Derived bool
	Origin  TokenOrigin
}

type PlateVariant struct {
	Index int
	Core  []PlateToken
	Found []PlateToken
	Notes []string
}

type PlateAssembly struct {
	FoundLimit int
	Variants   []PlateVariant
}

type plateRole struct {
	ID         string
	Job        string
	Structural bool
}

var plateRoles = []plateRole{
	{ID: "ground", Job: "ground", Structural: true},
	{ID: "panel", Job: "panels", Structural: true},
	{ID: "ink", Job: "ink · voices", Structural: true},
	{ID: "muted", Job: "meta · quiet", Structural: true},
	{ID: "observed", Job: "observed fact"},
	{ID: "attention", Job: "attention"},
	{ID: "rupture", Job: "rupture only"},
	{ID: "gold", Job: "human acts only"},
}

// BuildPlateAssembly maps every accent set into the eight semantic tokens and
// selects the raw candidates left over as found inks. This is the fresh,
// unlocked extraction path; browser lock and pin state remains a UI concern.
func BuildPlateAssembly(structure StructureResult, accents AccentStage, foundLimit int, skipFinal ...bool) PlateAssembly {
	assembly := PlateAssembly{
		FoundLimit: foundLimit,
		Variants:   make([]PlateVariant, 0, len(accents.Sets)),
	}
	for index, set := range accents.Sets {
		assembly.Variants = append(assembly.Variants, assembleVariant(
			index, set, structure, accents.Filtered, accents.RoleCandidates, len(accents.Sets), foundLimit, accents.Policy, len(skipFinal) == 0 || !skipFinal[0],
		))
	}
	return assembly
}

func assembleVariant(index int, set []AccentColor, structure StructureResult, candidates []Candidate, observed []AccentColor, variantCount, foundLimit int, policy ColorPolicy, finalize bool) PlateVariant {
	orderedIndices := AssignRoleIndices(set)
	usedNames := make(map[string]bool, len(plateRoles)+max(0, foundLimit))
	core := make([]PlateToken, 0, len(plateRoles))
	accentIndex := 0
	for _, role := range plateRoles {
		if role.Structural {
			color := structure.Colors[role.ID]
			core = append(core, PlateToken{
				ID:     role.ID,
				Name:   nameColor(color, usedNames),
				Job:    role.Job,
				RGB:    color,
				Hex:    colorx.Hex(color),
				Origin: structurePlateOrigin(structure.Origins[role.ID]),
			})
			continue
		}

		color := colorx.RGB{R: 120, G: 110, B: 96}
		derived := false
		origin := TokenOrigin{Kind: "fallback", Reason: "no candidate for this slot"}
		if accentIndex < len(orderedIndices) && orderedIndices[accentIndex] < len(set) {
			accent := set[orderedIndices[accentIndex]]
			color = accent.RGB
			derived = accent.Derived
			before := color
			color, _ = colorx.ForceContrastPair(color, structure.Colors["ground"], structure.Colors["panel"], 3.0)
			lateRepairs := []Repair{}
			if colorx.Hex(before) != colorx.Hex(color) {
				lateRepairs = append(lateRepairs, Repair{
					Operation: "contrast",
					Against:   "ground+panel",
					Target:    3.0,
					From:      colorx.Hex(before),
					To:        colorx.Hex(color),
				})
			}
			origin = AccentPlateOrigin(accent, lateRepairs)
		}
		core = append(core, PlateToken{
			ID:      role.ID,
			Name:    nameColor(color, usedNames),
			Job:     role.Job,
			RGB:     color,
			Hex:     colorx.Hex(color),
			Derived: derived,
			Origin:  origin,
		})
		accentIndex++
	}

	refinementNotes := []string{}
	if finalize {
		inputs := make([]AccentInput, 4)
		for i, token := range core[4:] {
			raw, ok := ParseHex(token.Origin.Raw)
			inputs[i] = AccentInput{RGB: token.RGB, Raw: raw, HasRaw: ok, Derived: token.Derived}
		}
		final := FinalizeAccents(inputs, observed, structure.Colors["ground"], structure.Colors["panel"], policy)
		for i, color := range final.Colors {
			token := &core[i+4]
			if replacement, ok := final.Replacements[i]; ok {
				token.Origin = AccentPlateOrigin(replacement, nil)
				token.Derived = replacement.Derived
			}
			token.RGB, token.Hex = color, colorx.Hex(color)
			token.Origin.Repairs = append(token.Origin.Repairs, final.Repairs[i]...)
		}
		refinementNotes = final.Notes
		usedNames = make(map[string]bool)
		for i := range core {
			core[i].Name = nameColor(core[i].RGB, usedNames)
		}
	}
	found := assembleFoundInks(candidates, core, usedNames, foundLimit)
	notes := append([]string(nil), structure.Notes...)
	notes = append(notes, refinementNotes...)
	if finalize && NormalizeColorPolicy(policy) == SourceHues {
		departures := []string{}
		for _, token := range core[2:] {
			if SourceHueDeparture(token.RGB, token.Origin.Raw, token.Origin.Method, token.Origin.IdealHue, token.Origin.MedianC) {
				departures = append(departures, token.ID)
			}
		}
		if len(departures) > 0 {
			notes = append(notes, "source hues shifted for readability — "+strings.Join(departures, ", "))
		}
	}
	derivedCount := 0
	for _, token := range core {
		if token.Derived {
			derivedCount++
		}
	}
	if derivedCount > 0 {
		plural := ""
		if derivedCount > 1 {
			plural = "s"
		}
		notes = append(notes, fmt.Sprintf(
			"%d signal slot%s derived — the still ran out of distinct paint", derivedCount, plural,
		))
	}
	if variantCount > 1 {
		notes = append(notes, fmt.Sprintf(
			"plate %d of %d — remix for the others", index+1, variantCount,
		))
	}
	if foundLimit > 0 && len(found) == 0 {
		notes = append(notes, "no found inks left over — drag the signal floor lower")
	}

	return PlateVariant{Index: index, Core: core, Found: found, Notes: notes}
}

func structurePlateOrigin(source StructureOrigin) TokenOrigin {
	return TokenOrigin{
		Kind:      source.Kind,
		Method:    source.Method,
		Quantile:  source.Quantile,
		Raw:       source.Raw,
		ChromaCap: source.ChromaCap,
		Capped:    source.Capped,
		Repairs:   append([]Repair{}, source.Repairs...),
	}
}

func AccentPlateOrigin(accent AccentColor, lateRepairs []Repair) TokenOrigin {
	if accent.Derived {
		origin := TokenOrigin{
			Kind:    "derived",
			Reason:  "the still offered no distinct hue for this slot",
			Repairs: append([]Repair{}, lateRepairs...),
		}
		if accent.DerivedFrom != nil {
			origin.Method = accent.DerivedFrom.Method
			if origin.Method == "source-tonal-reuse" {
				origin.Reason = "source hue or neutral paint reused to keep the palette within its source"
			}
			origin.IdealHue = accent.DerivedFrom.IdealHue
			origin.Role = accent.DerivedFrom.Role
			origin.MedianC = accent.DerivedFrom.MedianC
			origin.MedianL = accent.DerivedFrom.MedianL
		}
		return origin
	}

	origin := TokenOrigin{Kind: "signal"}
	if accent.Source != nil {
		origin.Kind = accent.Source.Kind
		origin.Method = accent.Source.Method
		origin.Raw = accent.Source.Raw
		origin.HueBin = accent.Source.HueBin
		origin.Band = accent.Source.Band
		origin.Area = accent.Source.Area
		origin.Salience = accent.Source.Salience
		origin.Stills = append([]int(nil), accent.Source.Stills...)
	}
	origin.Repairs = make([]Repair, 0, len(accent.Repairs)+len(lateRepairs))
	origin.Repairs = append(origin.Repairs, accent.Repairs...)
	origin.Repairs = append(origin.Repairs, lateRepairs...)
	return origin
}

func assembleFoundInks(candidates []Candidate, core []PlateToken, usedNames map[string]bool, foundLimit int) []PlateToken {
	limit := max(0, foundLimit)
	found := make([]PlateToken, 0, min(limit, len(candidates)))
	for _, candidate := range candidates {
		if len(found) >= limit {
			break
		}
		distinct := true
		for _, token := range core {
			if colorx.DeltaE(token.RGB, candidate.RGB) <= 0.09 {
				distinct = false
				break
			}
		}
		if !distinct {
			continue
		}
		found = append(found, PlateToken{
			Name: nameColor(candidate.RGB, usedNames),
			Job:  "found ink",
			RGB:  candidate.RGB,
			Hex:  colorx.Hex(candidate.RGB),
			Origin: TokenOrigin{
				Kind:     "signal",
				Method:   "leftover — not taken by a role",
				Raw:      colorx.Hex(candidate.RGB),
				HueBin:   candidate.HueBin,
				Band:     candidate.Band,
				Area:     roundDecimal(candidate.Area, 6),
				Salience: roundDecimal(candidate.Salience, 6),
				Stills:   append([]int(nil), candidate.Stills...),
				Repairs:  []Repair{},
			},
		})
	}
	return found
}
