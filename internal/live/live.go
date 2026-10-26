// Package live applies the deterministic Go extraction pipeline to browser
// working pixels and preserves human-owned locks during subsequent rebuilds.
package live

import (
	"encoding/json"
	"fmt"
	"image"
	"math"
	"strings"
	"time"

	"scene-palette/internal/colorx"
	"scene-palette/internal/extract"
	"scene-palette/internal/palette"
)

const (
	MaxImages     = 8
	MaxDimension  = 4096
	MaxPixels     = 16000000
	maxPinnedInks = 100
)

type Params struct {
	ColorPolicy extract.ColorPolicy `json:"colorPolicy"`
	Budget      int                 `json:"budget"`
	ChromaMin   float64             `json:"chromaMin"`
	FoundInks   int                 `json:"foundInks"`
}

// Request carries all state needed to reproduce a build. A first build supplies
// working images; a rebuild supplies its previously returned histogram instead.
// Only locked current tokens are carried into newly generated variants.
type Request struct {
	Histogram *extract.PooledHistogram `json:"histogram,omitempty"`
	Params    Params                   `json:"params"`
	Polarity  extract.Polarity         `json:"polarity"`
	Name      string                   `json:"name"`
	CreatedAt string                   `json:"createdAt"`
	Sources   []palette.SnapshotSource `json:"sources"`
	Core      []palette.SnapshotToken  `json:"core"`
	Found     []palette.SnapshotToken  `json:"found"`
}

type Response struct {
	Histogram extract.PooledHistogram `json:"histogram"`
	Variants  []palette.Snapshot      `json:"variants"`
	AutoLight bool                    `json:"autoLight"`
	MedianL   float64                 `json:"medianL"`
}

// Build returns every variant for one polarity, with the same locks applied to
// each. No human-review metadata is manufactured; a new result is a proposal.
func Build(images []image.Image, request Request) (Response, error) {
	if err := validate(images, request); err != nil {
		return Response{}, err
	}
	var pooled extract.PooledHistogram
	if request.Histogram != nil {
		pooled = *request.Histogram
		for i, stills := range pooled.BinStills {
			pooled.BinStills[i] = append([]int(nil), stills...)
		}
	} else {
		histograms := make([]extract.Histogram, len(images))
		for i, source := range images {
			histograms[i] = extract.BuildHistogram(source, request.Params.Budget, request.Params.ChromaMin)
		}
		pooled = extract.Pool(histograms)
	}
	structure := extract.BuildStructure(pooled, request.Polarity)
	accents := extract.BuildAccentStage(extract.SignalCandidates(pooled), structure, 6, request.Params.ColorPolicy)
	assembly := extract.BuildPlateAssembly(structure, accents, request.Params.FoundInks, true)
	result := Response{
		Histogram: pooled,
		Variants:  make([]palette.Snapshot, 0, len(assembly.Variants)),
		AutoLight: structure.AutoLight,
		MedianL:   structure.MedianL,
	}
	for _, variant := range assembly.Variants {
		plate, err := palette.BuildSnapshot(palette.SnapshotInput{
			Name:         request.Name,
			CreatedAt:    request.CreatedAt,
			Structure:    structure,
			Variant:      variant,
			VariantCount: len(assembly.Variants),
			Params: palette.SnapshotParamsInput{
				ColorPolicy: request.Params.ColorPolicy,
				Budget:      request.Params.Budget, ChromaMin: request.Params.ChromaMin,
				FoundInks: request.Params.FoundInks,
			},
			Sources: request.Sources,
		})
		if err != nil {
			return Response{}, err
		}
		applyHumanState(&plate, request, accents.Filtered, accents.RoleCandidates, structure.Notes)
		result.Variants = append(result.Variants, plate)
	}
	return result, nil
}

func applyHumanState(plate *palette.Snapshot, request Request, candidates []extract.Candidate, observed []extract.AccentColor, baseNotes []string) {
	locked := make(map[string]palette.SnapshotToken)
	used := make(map[string]bool)
	for _, token := range request.Core {
		if token.Locked {
			role := token.ID
			if role == "" {
				role = token.Key
			}
			locked[role] = token
			used[token.Name] = true
		}
	}
	ground, panel := rgb(plate.Core[0]), rgb(plate.Core[1])
	if token, ok := locked["ground"]; ok {
		ground = rgb(token)
	}
	if token, ok := locked["panel"]; ok {
		panel = rgb(token)
	}
	unreadableLockedSurfaces := false
	derived := 0
	for i, token := range plate.Core {
		if previous, ok := locked[token.ID]; ok {
			previous.ID = token.ID
			previous.Key = token.ID
			previous.Origin = append(json.RawMessage(nil), previous.Origin...)
			plate.Core[i] = previous
		} else {
			if token.ID != "ground" && token.ID != "panel" {
				target := 3.0
				if token.ID == "ink" {
					target = 4.5
				}
				if colorx.Contrast(rgb(token), ground) < target || colorx.Contrast(rgb(token), panel) < target {
					joint, readable := colorx.ForceContrastPair(rgb(token), ground, panel, target)
					if !readable {
						unreadableLockedSurfaces = true
					} else if colorx.Hex(joint) != token.Hex {
						appendPairRepair(&plate.Core[i], joint, target)
					}
				}
			}
			plate.Core[i].Name = extract.NameColorUnique(rgb(plate.Core[i]), used)
		}
		if plate.Core[i].Derived {
			derived++
		}
	}
	inputs := make([]extract.AccentInput, 4)
	for i, token := range plate.Core[4:] {
		var origin struct {
			Raw string `json:"raw"`
		}
		_ = json.Unmarshal(token.Origin, &origin)
		raw, ok := extract.ParseHex(origin.Raw)
		inputs[i] = extract.AccentInput{RGB: rgb(token), Raw: raw, HasRaw: ok, Derived: token.Derived, Locked: token.Locked}
	}
	final := extract.FinalizeAccents(inputs, observed, ground, panel, request.Params.ColorPolicy)
	for i, color := range final.Colors {
		token := &plate.Core[i+4]
		if token.Locked {
			continue
		}
		if replacement, ok := final.Replacements[i]; ok {
			token.Origin, _ = palette.MarshalSnapshotOrigin(extract.AccentPlateOrigin(replacement, nil))
			token.Derived = replacement.Derived
		}
		for _, repair := range final.Repairs[i] {
			appendAccentRepair(token, repair)
		}
		token.R, token.G, token.B, token.Hex = color.R, color.G, color.B, colorx.Hex(color)
	}
	used = make(map[string]bool)
	for _, token := range plate.Core {
		if token.Locked {
			used[token.Name] = true
		}
	}
	derived = 0
	for i := range plate.Core {
		if !plate.Core[i].Locked {
			plate.Core[i].Name = extract.NameColorUnique(rgb(plate.Core[i]), used)
		}
		if plate.Core[i].Derived {
			derived++
		}
	}
	plate.Notes = append([]string{}, baseNotes...)
	plate.Notes = append(plate.Notes, final.Notes...)
	if extract.NormalizeColorPolicy(request.Params.ColorPolicy) == extract.SourceHues {
		departures := []string{}
		for _, token := range plate.Core[2:] {
			if token.Locked {
				continue
			}
			var origin struct {
				Raw      string  `json:"raw"`
				Method   string  `json:"method"`
				IdealHue float64 `json:"idealHue"`
				MedianC  float64 `json:"medianC"`
			}
			_ = json.Unmarshal(token.Origin, &origin)
			if extract.SourceHueDeparture(rgb(token), origin.Raw, origin.Method, origin.IdealHue, origin.MedianC) {
				departures = append(departures, token.ID)
			}
		}
		if len(departures) > 0 {
			plate.Notes = append(plate.Notes, "source hues shifted for readability — "+strings.Join(departures, ", "))
		}
	}
	if unreadableLockedSurfaces {
		plate.Notes = append(plate.Notes, "locked surfaces cannot share readable ink — unlock or adjust a surface")
	}
	if derived > 0 {
		plural := ""
		if derived > 1 {
			plural = "s"
		}
		plate.Notes = append(plate.Notes, fmt.Sprintf("%d signal slot%s derived — the still ran out of distinct paint", derived, plural))
	}
	if plate.Params.VariantCount > 1 {
		plate.Notes = append(plate.Notes, fmt.Sprintf("plate %d of %d — remix for the others", plate.Params.Variant+1, plate.Params.VariantCount))
	}

	found := make([]palette.SnapshotToken, 0, request.Params.FoundInks)
	for _, token := range request.Found {
		if token.Locked {
			token.Origin = append(json.RawMessage(nil), token.Origin...)
			found = append(found, token)
			used[token.Name] = true
		}
	}
	pins := append([]palette.SnapshotToken(nil), found...)
	for _, candidate := range candidates {
		if len(found) >= request.Params.FoundInks {
			break
		}
		if !distinctFrom(candidate.RGB, plate.Core) || !distinctFrom(candidate.RGB, pins) {
			continue
		}
		origin, _ := json.Marshal(map[string]any{
			"kind": "signal", "method": "leftover — not taken by a role",
			"raw": colorx.Hex(candidate.RGB), "hueBin": candidate.HueBin, "band": candidate.Band,
			"area": round(candidate.Area, 6), "salience": round(candidate.Salience, 6),
			"stills": append([]int{}, candidate.Stills...), "repairs": []any{},
		})
		found = append(found, palette.SnapshotToken{
			Name: extract.NameColorUnique(candidate.RGB, used), Job: "found ink",
			R: candidate.RGB.R, G: candidate.RGB.G, B: candidate.RGB.B,
			Hex: colorx.Hex(candidate.RGB), Origin: origin,
		})
	}
	plate.Found = found
	if request.Params.FoundInks > 0 && len(found) == 0 {
		plate.Notes = append(plate.Notes, "no found inks left over — drag the signal floor lower")
	}
	for i := range plate.Core {
		repack(&plate.Core[i], plate.Core[i].ID, ground, panel)
	}
	for i := range plate.Found {
		repack(&plate.Found[i], fmt.Sprintf("found_%d", i+1), ground, panel)
	}
	plate.Facets = palette.PlateFacets(*plate)
	plate.Signature = palette.PlateSignature(*plate)
}

func appendAccentRepair(token *palette.SnapshotToken, item extract.Repair) {
	var origin map[string]json.RawMessage
	_ = json.Unmarshal(token.Origin, &origin)
	if origin == nil {
		origin = map[string]json.RawMessage{"kind": json.RawMessage(`"unknown"`)}
	}
	var repairs []json.RawMessage
	_ = json.Unmarshal(origin["repairs"], &repairs)
	repair, _ := json.Marshal(map[string]any{"op": item.Operation, "against": item.Against, "target": item.Target, "from": item.From, "to": item.To})
	origin["repairs"], _ = json.Marshal(append(repairs, repair))
	token.Origin, _ = json.Marshal(origin)
}

func appendPairRepair(token *palette.SnapshotToken, next colorx.RGB, target float64) {
	var origin map[string]json.RawMessage
	_ = json.Unmarshal(token.Origin, &origin)
	if origin == nil {
		origin = map[string]json.RawMessage{"kind": json.RawMessage(`"unknown"`)}
	}
	var repairs []json.RawMessage
	_ = json.Unmarshal(origin["repairs"], &repairs)
	repair, _ := json.Marshal(map[string]any{
		"op": "contrast", "against": "ground+panel", "target": target,
		"from": token.Hex, "to": colorx.Hex(next),
	})
	origin["repairs"], _ = json.Marshal(append(repairs, repair))
	token.Origin, _ = json.Marshal(origin)
	token.R, token.G, token.B = next.R, next.G, next.B
	token.Hex = colorx.Hex(next)
}

func distinctFrom(color colorx.RGB, tokens []palette.SnapshotToken) bool {
	for _, token := range tokens {
		if colorx.DeltaE(rgb(token), color) <= 0.09 {
			return false
		}
	}
	return true
}

func rgb(token palette.SnapshotToken) colorx.RGB {
	return colorx.RGB{R: token.R, G: token.G, B: token.B}
}

func round(value float64, places int) float64 {
	factor := math.Pow10(places)
	return math.Round(value*factor) / factor
}

func repack(token *palette.SnapshotToken, key string, ground, panel colorx.RGB) {
	color := rgb(*token)
	lch := colorx.RGBToLCH(color)
	token.Key = key
	token.Hex = colorx.Hex(color)
	token.RGB = [3]int{color.R, color.G, color.B}
	token.OKLCH = [3]float64{round(lch.L, 4), round(lch.C, 4), round(lch.H, 1)}
	token.Contrast = palette.SnapshotContrast{OnGround: round(colorx.Contrast(color, ground), 2), OnPanel: round(colorx.Contrast(color, panel), 2)}
	if len(token.Origin) == 0 {
		token.Origin = json.RawMessage(`{"kind":"unknown"}`)
	}
}

func validate(images []image.Image, request Request) error {
	if !extract.ValidColorPolicy(request.Params.ColorPolicy) {
		return fmt.Errorf("live color policy must be readable-ui or source-hues")
	}
	if request.Params.Budget < 1 || request.Params.Budget > 400000 {
		return fmt.Errorf("live budget must be between 1 and 400000")
	}
	if !finite(request.Params.ChromaMin) || request.Params.ChromaMin < 0 || request.Params.ChromaMin > 1 {
		return fmt.Errorf("live chroma minimum must be between 0 and 1")
	}
	if request.Params.FoundInks < 0 || request.Params.FoundInks > 10 {
		return fmt.Errorf("live found inks must be between 0 and 10")
	}
	if request.Polarity != extract.PolarityAuto && request.Polarity != extract.PolarityDark && request.Polarity != extract.PolarityLight {
		return fmt.Errorf("live polarity must be auto, dark, or light")
	}
	if _, err := time.Parse(time.RFC3339Nano, request.CreatedAt); err != nil {
		return fmt.Errorf("live createdAt must be RFC3339")
	}
	if len(request.Sources) > MaxImages {
		return fmt.Errorf("live accepts at most %d sources", MaxImages)
	}
	if len(request.Core) > 8 || len(request.Found) > maxPinnedInks {
		return fmt.Errorf("live current token count exceeds its limit")
	}
	roles := map[string]bool{"ground": true, "panel": true, "ink": true, "muted": true, "observed": true, "attention": true, "rupture": true, "gold": true}
	seen := make(map[string]bool)
	for _, token := range request.Core {
		role := token.ID
		if role == "" {
			role = token.Key
		}
		if !roles[role] || seen[role] {
			return fmt.Errorf("live core role %q is unknown or repeated", role)
		}
		seen[role] = true
		if err := validateToken(token); err != nil {
			return err
		}
	}
	for _, token := range request.Found {
		if err := validateToken(token); err != nil {
			return err
		}
	}
	if request.Histogram != nil {
		if len(images) != 0 {
			return fmt.Errorf("live build accepts images or a cached histogram, not both")
		}
		return validateHistogram(*request.Histogram, len(request.Sources))
	}
	if len(images) < 1 || len(images) > MaxImages {
		return fmt.Errorf("live requires one to eight working images")
	}
	if len(request.Sources) != 0 && len(images) != len(request.Sources) {
		return fmt.Errorf("live images and source records must have equal counts")
	}
	pixels := int64(0)
	for _, source := range images {
		if source == nil {
			return fmt.Errorf("live image must not be nil")
		}
		bounds := source.Bounds()
		width, height := bounds.Dx(), bounds.Dy()
		if width < 1 || height < 1 || width > MaxDimension || height > MaxDimension {
			return fmt.Errorf("live working image dimensions must be between 1 and %d", MaxDimension)
		}
		pixels += int64(width) * int64(height)
		if pixels > MaxPixels {
			return fmt.Errorf("live working images exceed %d total pixels", MaxPixels)
		}
	}
	return nil
}

func validateToken(token palette.SnapshotToken) error {
	if token.R < 0 || token.R > 255 || token.G < 0 || token.G > 255 || token.B < 0 || token.B > 255 {
		return fmt.Errorf("live current token RGB channels must be between 0 and 255")
	}
	if token.Hex != "" && !strings.EqualFold(token.Hex, colorx.Hex(rgb(token))) {
		return fmt.Errorf("live current token hex does not match its RGB channels")
	}
	if len(token.Name) > 1024 || len(token.Job) > 1024 || len(token.ID) > 128 || len(token.Origin) > 65536 {
		return fmt.Errorf("live current token metadata exceeds its limit")
	}
	if len(token.Origin) > 0 {
		var origin map[string]json.RawMessage
		if err := json.Unmarshal(token.Origin, &origin); err != nil || origin == nil {
			return fmt.Errorf("live current token origin must be an object")
		}
		if raw, ok := origin["area"]; ok {
			var area float64
			if err := json.Unmarshal(raw, &area); err != nil || !finite(area) || area < 0 || area > 1000000 {
				return fmt.Errorf("live current token origin area is invalid")
			}
		}
	}
	return nil
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func validateHistogram(histogram extract.PooledHistogram, sourceCount int) error {
	if !finite(histogram.TotalN) || histogram.TotalN < 1 || histogram.TotalN > 8000.000001 {
		return fmt.Errorf("live cached histogram total is invalid")
	}
	limit := 8000.000001
	total := 0.0
	for _, bin := range histogram.Luminance {
		if !finite(bin.N) || bin.N < 0 || bin.N > limit || !validWeightedColor(bin.N, bin.L, bin.A, bin.B) {
			return fmt.Errorf("live cached luminance bin is invalid")
		}
		total += bin.N
	}
	if total == 0 {
		total = 1
	}
	if math.Abs(total-histogram.TotalN) > 0.000001 {
		return fmt.Errorf("live cached histogram total does not match its bins")
	}
	for i, bin := range histogram.Signal {
		if !finite(bin.N) || !finite(bin.WeightSum) || bin.N < 0 || bin.N > limit || bin.WeightSum < 0 || bin.WeightSum > 64*bin.N || !validWeightedColor(bin.WeightSum, bin.L, bin.A, bin.B) {
			return fmt.Errorf("live cached signal bin is invalid")
		}
		stills := histogram.BinStills[i]
		if len(stills) > MaxImages {
			return fmt.Errorf("live cached source-bin attribution exceeds its limit")
		}
		seen := make(map[int]bool)
		for _, index := range stills {
			if index < 0 || index >= MaxImages || (sourceCount > 0 && index >= sourceCount) || seen[index] {
				return fmt.Errorf("live cached source-bin attribution is invalid")
			}
			seen[index] = true
		}
	}
	return nil
}

func validWeightedColor(weight, lightness, a, b float64) bool {
	if !finite(lightness) || !finite(a) || !finite(b) || lightness < 0 {
		return false
	}
	if weight == 0 {
		return lightness == 0 && a == 0 && b == 0
	}
	// Check normalized values rather than allowing an absolute tolerance on
	// weighted sums, which would permit an enormous color at a tiny weight.
	return lightness/weight <= 1.000001 && math.Abs(a/weight) <= 1.000001 && math.Abs(b/weight) <= 1.000001
}
