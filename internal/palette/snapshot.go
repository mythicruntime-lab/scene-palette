package palette

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"

	"scene-palette/internal/colorx"
	"scene-palette/internal/extract"
)

var (
	snapshotExtensionPattern = regexp.MustCompile(`(?i)\.([a-z0-9]{1,5})$`)
	snapshotSeparatorPattern = regexp.MustCompile(`[^a-z0-9]+`)
	snapshotNewlinePattern   = regexp.MustCompile(`[\r\n]+`)
)

// Snapshot is the complete palette.json value written by the browser's
// snapshot() function. Document remains the narrower strict-reader shape used
// by the vault indexer; this type owns serialization of fresh Go extraction.
type Snapshot struct {
	Schema     int               `json:"schema"`
	Kind       string            `json:"kind"`
	Name       string            `json:"name"`
	CreatedAt  string            `json:"createdAt"`
	Generator  Generator         `json:"generator"`
	Polarity   string            `json:"polarity"`
	LightScene bool              `json:"lightScene"`
	MedianL    *float64          `json:"medianL"`
	Params     SnapshotParams    `json:"params"`
	Sources    []SnapshotSource  `json:"sources"`
	Stills     int               `json:"stills"`
	Tags       []string          `json:"tags"`
	Family     *string           `json:"family"`
	Core       []SnapshotToken   `json:"core"`
	Found      []SnapshotToken   `json:"found"`
	Notes      []string          `json:"notes"`
	Facets     SnapshotFacets    `json:"facets"`
	Signature  SnapshotSignature `json:"signature"`
}

type SnapshotParams struct {
	Budget       int     `json:"budget"`
	ChromaMin    float64 `json:"chromaMin"`
	FoundInks    int     `json:"foundInks"`
	PolarityMode string  `json:"polarityMode"`
	Variant      int     `json:"variant"`
	VariantCount int     `json:"variantCount"`
}

// SnapshotSource is source metadata captured during ingestion. BuildSnapshot
// preserves these explicit facts and derives only the bundle-relative path.
type SnapshotSource struct {
	File       string           `json:"file"`
	MIME       *string          `json:"mime"`
	Bytes      *int64           `json:"bytes"`
	Hash       *Hash            `json:"hash"`
	Natural    [2]int           `json:"natural"`
	Working    [2]int           `json:"working"`
	Transforms []map[string]any `json:"transforms"`
	Path       string           `json:"path"`
}

type SnapshotToken struct {
	Key      string           `json:"key"`
	ID       string           `json:"id,omitempty"`
	Name     string           `json:"name"`
	Job      string           `json:"job"`
	Hex      string           `json:"hex"`
	RGB      [3]int           `json:"rgb"`
	R        int              `json:"r"`
	G        int              `json:"g"`
	B        int              `json:"b"`
	OKLCH    [3]float64       `json:"oklch"`
	Locked   bool             `json:"locked"`
	Derived  bool             `json:"derived"`
	Contrast SnapshotContrast `json:"contrast"`
	Origin   json.RawMessage  `json:"origin"`
}

type SnapshotContrast struct {
	OnGround float64 `json:"onGround"`
	OnPanel  float64 `json:"onPanel"`
}

type SnapshotFacets struct {
	Polarity            string         `json:"polarity"`
	GroundL             float64        `json:"groundL"`
	MedianL             *float64       `json:"medianL"`
	DominantHue         *int           `json:"dominantHue"`
	AccentHues          []int          `json:"accentHues"`
	HueSpread           int            `json:"hueSpread"`
	Chroma              SnapshotChroma `json:"chroma"`
	Temperature         string         `json:"temperature"`
	WarmAccents         int            `json:"warmAccents"`
	CoolAccents         int            `json:"coolAccents"`
	DerivedSlots        int            `json:"derivedSlots"`
	PickedSlots         int            `json:"pickedSlots"`
	FoundInks           int            `json:"foundInks"`
	MinContrastOnGround float64        `json:"minContrastOnGround"`
	PanelSeparationL    *float64       `json:"panelSeparationL"`
}

type SnapshotChroma struct {
	Min  float64 `json:"min"`
	Max  float64 `json:"max"`
	Mean float64 `json:"mean"`
}

type SnapshotSignature struct {
	Version   int                     `json:"version"`
	Roles     []SnapshotSignatureRole `json:"roles"`
	Hue12     [12]float64             `json:"hue12"`
	Lightness []float64               `json:"lightness"`
}

type SnapshotSignatureRole struct {
	ID  string     `json:"id"`
	Lab [3]float64 `json:"lab"`
}

type SnapshotParamsInput struct {
	Budget    int
	ChromaMin float64
	FoundInks int
}

// SnapshotInput keeps environment-owned values explicit. Extraction owns the
// colours; callers own clocks, source bytes and ingestion metadata.
type SnapshotInput struct {
	Name         string
	CreatedAt    string
	Structure    extract.StructureResult
	Variant      extract.PlateVariant
	VariantCount int
	Params       SnapshotParamsInput
	Sources      []SnapshotSource
	Tags         []string
	Family       *string
}

type snapshotRepair struct {
	Operation string   `json:"op"`
	Against   string   `json:"against"`
	Target    *float64 `json:"target,omitempty"`
	MinDeltaL *float64 `json:"minDeltaL,omitempty"`
	From      string   `json:"from"`
	To        string   `json:"to"`
}

// BuildSnapshot turns one fresh, unlocked Go plate variant into the browser's
// durable palette.json shape. Locks and picked origins remain browser state and
// are intentionally outside this boundary.
func BuildSnapshot(input SnapshotInput) (Snapshot, error) {
	if strings.TrimSpace(input.CreatedAt) == "" {
		return Snapshot{}, fmt.Errorf("snapshot createdAt is required")
	}
	if input.VariantCount < 1 {
		return Snapshot{}, fmt.Errorf("snapshot variant count must be positive")
	}
	if input.Variant.Index < 0 || input.Variant.Index >= input.VariantCount {
		return Snapshot{}, fmt.Errorf("snapshot variant %d is outside count %d", input.Variant.Index, input.VariantCount)
	}
	if len(input.Variant.Core) != len(coreRoles) {
		return Snapshot{}, fmt.Errorf("snapshot core has %d roles; expected %d", len(input.Variant.Core), len(coreRoles))
	}
	for index, role := range coreRoles {
		if input.Variant.Core[index].ID != role {
			return Snapshot{}, fmt.Errorf("snapshot core role %d is %q; expected %q", index, input.Variant.Core[index].ID, role)
		}
	}

	extractionVersion := ExtractionVersion
	medianL := roundSnapshot(input.Structure.MedianL, 4)
	tags := append([]string{}, input.Tags...)
	notes := append([]string{}, input.Variant.Notes...)
	sources := make([]SnapshotSource, len(input.Sources))
	for index, source := range input.Sources {
		sources[index] = source
		sources[index].Transforms = append([]map[string]any{}, source.Transforms...)
		sources[index].Path = snapshotSourcePath(index, source.File, source.MIME)
	}

	ground := input.Variant.Core[0].RGB
	panel := input.Variant.Core[1].RGB
	core := make([]SnapshotToken, 0, len(input.Variant.Core))
	for _, token := range input.Variant.Core {
		packed, err := packSnapshotToken(token, token.ID, ground, panel)
		if err != nil {
			return Snapshot{}, err
		}
		core = append(core, packed)
	}
	found := make([]SnapshotToken, 0, len(input.Variant.Found))
	for index, token := range input.Variant.Found {
		packed, err := packSnapshotToken(token, fmt.Sprintf("found_%d", index+1), ground, panel)
		if err != nil {
			return Snapshot{}, err
		}
		found = append(found, packed)
	}

	polarity := "dark"
	if input.Structure.LightScene {
		polarity = "light"
	}
	family := input.Family
	if family != nil {
		value := *family
		family = &value
	}
	snapshot := Snapshot{
		Schema:     SchemaVersion,
		Kind:       Kind,
		Name:       sanitizeSnapshotName(input.Name),
		CreatedAt:  input.CreatedAt,
		Generator:  Generator{Name: "scene-palette", Version: ToolVersion, Extraction: &extractionVersion},
		Polarity:   polarity,
		LightScene: input.Structure.LightScene,
		MedianL:    &medianL,
		Params: SnapshotParams{
			Budget:       input.Params.Budget,
			ChromaMin:    input.Params.ChromaMin,
			FoundInks:    input.Params.FoundInks,
			PolarityMode: string(input.Structure.Mode),
			Variant:      input.Variant.Index,
			VariantCount: input.VariantCount,
		},
		Sources: sources,
		Stills:  len(sources),
		Tags:    tags,
		Family:  family,
		Core:    core,
		Found:   found,
		Notes:   notes,
	}
	snapshot.Facets = PlateFacets(snapshot)
	snapshot.Signature = PlateSignature(snapshot)
	return snapshot, nil
}

func packSnapshotToken(token extract.PlateToken, key string, ground, panel colorx.RGB) (SnapshotToken, error) {
	lch := colorx.RGBToLCH(token.RGB)
	origin, err := marshalSnapshotOrigin(token.Origin)
	if err != nil {
		return SnapshotToken{}, fmt.Errorf("encode %s origin: %w", key, err)
	}
	return SnapshotToken{
		Key:     key,
		ID:      token.ID,
		Name:    token.Name,
		Job:     token.Job,
		Hex:     token.Hex,
		RGB:     [3]int{token.RGB.R, token.RGB.G, token.RGB.B},
		R:       token.RGB.R,
		G:       token.RGB.G,
		B:       token.RGB.B,
		OKLCH:   [3]float64{roundSnapshot(lch.L, 4), roundSnapshot(lch.C, 4), roundSnapshot(lch.H, 1)},
		Locked:  token.Locked,
		Derived: token.Derived,
		Contrast: SnapshotContrast{
			OnGround: roundSnapshot(colorx.Contrast(token.RGB, ground), 2),
			OnPanel:  roundSnapshot(colorx.Contrast(token.RGB, panel), 2),
		},
		Origin: origin,
	}, nil
}

func marshalSnapshotOrigin(origin extract.TokenOrigin) (json.RawMessage, error) {
	value := make(map[string]any)
	kind := origin.Kind
	if kind == "" {
		kind = "unknown"
	}
	value["kind"] = kind
	switch kind {
	case "structure":
		value["method"] = origin.Method
		value["quantile"] = origin.Quantile
		value["raw"] = origin.Raw
		value["chromaCap"] = origin.ChromaCap
		value["capped"] = origin.Capped
		value["repairs"] = snapshotRepairs(origin.Repairs)
	case "signal":
		if origin.Method != "" || origin.Raw != "" || origin.Stills != nil {
			value["method"] = origin.Method
			value["raw"] = origin.Raw
			value["hueBin"] = origin.HueBin
			value["band"] = origin.Band
			value["area"] = origin.Area
			value["salience"] = origin.Salience
			value["stills"] = append([]int{}, origin.Stills...)
		}
		value["repairs"] = snapshotRepairs(origin.Repairs)
	case "derived":
		if origin.Method != "" {
			value["method"] = origin.Method
			value["idealHue"] = origin.IdealHue
			if origin.Role != "" {
				value["role"] = origin.Role
			}
			value["medianC"] = origin.MedianC
			value["medianL"] = origin.MedianL
		}
		value["reason"] = origin.Reason
		value["repairs"] = snapshotRepairs(origin.Repairs)
	case "fallback":
		value["reason"] = origin.Reason
	}
	return json.Marshal(value)
}

func snapshotRepairs(repairs []extract.Repair) []snapshotRepair {
	result := make([]snapshotRepair, 0, len(repairs))
	for _, repair := range repairs {
		item := snapshotRepair{
			Operation: repair.Operation,
			Against:   repair.Against,
			From:      repair.From,
			To:        repair.To,
		}
		if repair.Target != 0 {
			value := repair.Target
			item.Target = &value
		}
		if repair.MinDeltaL != 0 {
			value := repair.MinDeltaL
			item.MinDeltaL = &value
		}
		result = append(result, item)
	}
	return result
}

// PlateFacets mirrors the reconstructible browser facet function and reads
// only the packed plate value.
func PlateFacets(plate Snapshot) SnapshotFacets {
	all := append(append([]SnapshotToken{}, plate.Core...), plate.Found...)
	ground := snapshotTokenRGB(plate.Core[0])
	accents := plate.Core[4:]
	chromas := make([]float64, 0, len(all))
	for _, token := range all {
		chromas = append(chromas, colorx.RGBToLCH(snapshotTokenRGB(token)).C)
	}
	accentHues := make([]int, 0, len(accents))
	warm := 0
	for _, token := range accents {
		hue := colorx.RGBToLCH(snapshotTokenRGB(token)).H
		accentHues = append(accentHues, int(math.Floor(hue+0.5)))
		if hue < 70 || hue > 330 {
			warm++
		}
	}
	spread := 0.0
	for first := range accentHues {
		for second := first + 1; second < len(accentHues); second++ {
			spread = math.Max(spread, colorx.HueGap(float64(accentHues[first]), float64(accentHues[second])))
		}
	}

	var dominant *int
	bestArea := 0.0
	for _, token := range all {
		_, area := snapshotOriginMetadata(token.Origin)
		if area > bestArea {
			bestArea = area
			hue := int(math.Floor(colorx.RGBToLCH(snapshotTokenRGB(token)).H + 0.5))
			dominant = &hue
		}
	}
	minContrast := math.Inf(1)
	for _, token := range plate.Core {
		if token.ID == "ground" || token.ID == "panel" {
			continue
		}
		minContrast = math.Min(minContrast, colorx.Contrast(snapshotTokenRGB(token), ground))
	}
	minChroma, maxChroma, chromaTotal := math.Inf(1), math.Inf(-1), 0.0
	for _, chroma := range chromas {
		minChroma = math.Min(minChroma, chroma)
		maxChroma = math.Max(maxChroma, chroma)
		chromaTotal += chroma
	}
	temperature := "none"
	if len(accents) > 0 {
		ratio := float64(warm) / float64(len(accents))
		switch {
		case ratio >= 0.6:
			temperature = "warm"
		case ratio <= 0.25:
			temperature = "cool"
		default:
			temperature = "mixed"
		}
	}
	derivedSlots, pickedSlots := 0, 0
	for _, token := range plate.Core {
		if token.Derived {
			derivedSlots++
		}
	}
	for _, token := range all {
		kind, _ := snapshotOriginMetadata(token.Origin)
		if kind == "picked" {
			pickedSlots++
		}
	}
	panelSeparation := roundSnapshot(math.Abs(
		colorx.RGBToLCH(snapshotTokenRGB(plate.Core[1])).L-colorx.RGBToLCH(ground).L,
	), 4)
	medianL := copyFloatPointer(plate.MedianL)
	return SnapshotFacets{
		Polarity:            plate.Polarity,
		GroundL:             roundSnapshot(colorx.RGBToLCH(ground).L, 4),
		MedianL:             medianL,
		DominantHue:         dominant,
		AccentHues:          accentHues,
		HueSpread:           int(math.Floor(spread + 0.5)),
		Chroma:              SnapshotChroma{Min: roundSnapshot(minChroma, 4), Max: roundSnapshot(maxChroma, 4), Mean: roundSnapshot(chromaTotal/float64(len(chromas)), 4)},
		Temperature:         temperature,
		WarmAccents:         warm,
		CoolAccents:         len(accents) - warm,
		DerivedSlots:        derivedSlots,
		PickedSlots:         pickedSlots,
		FoundInks:           len(plate.Found),
		MinContrastOnGround: roundSnapshot(minContrast, 2),
		PanelSeparationL:    &panelSeparation,
	}
}

// PlateSignature mirrors the browser's similarity vector and reads only the
// packed plate value.
func PlateSignature(plate Snapshot) SnapshotSignature {
	roles := make([]SnapshotSignatureRole, 0, len(plate.Core))
	lightness := make([]float64, 0, len(plate.Core))
	for _, token := range plate.Core {
		rgb := snapshotTokenRGB(token)
		lab := colorx.RGBToOKLab(rgb)
		roles = append(roles, SnapshotSignatureRole{
			ID:  firstNonempty(token.ID, token.Key),
			Lab: [3]float64{roundSnapshot(lab.L, 4), roundSnapshot(lab.A, 4), roundSnapshot(lab.B, 4)},
		})
		lightness = append(lightness, roundSnapshot(colorx.RGBToLCH(rgb).L, 4))
	}
	var hue12 [12]float64
	total := 0.0
	all := append(append([]SnapshotToken{}, plate.Core...), plate.Found...)
	for _, token := range all {
		lch := colorx.RGBToLCH(snapshotTokenRGB(token))
		if lch.C < 0.04 {
			continue
		}
		_, area := snapshotOriginMetadata(token.Origin)
		if area <= 0 {
			area = 0.01
		}
		weight := area + lch.C
		hue := math.Mod(math.Mod(lch.H, 360)+360, 360)
		index := int(math.Floor(hue/30)) % len(hue12)
		hue12[index] += weight
		total += weight
	}
	for index, value := range hue12 {
		if total > 0 {
			value /= total
		} else {
			value = 0
		}
		hue12[index] = roundSnapshot(value, 4)
	}
	return SnapshotSignature{Version: 1, Roles: roles, Hue12: hue12, Lightness: lightness}
}

func snapshotOriginMetadata(raw json.RawMessage) (string, float64) {
	var metadata struct {
		Kind string  `json:"kind"`
		Area float64 `json:"area"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return "", 0
	}
	return metadata.Kind, metadata.Area
}

func snapshotTokenRGB(token SnapshotToken) colorx.RGB {
	return colorx.RGB{R: token.R, G: token.G, B: token.B}
}

func snapshotSourcePath(index int, file string, mime *string) string {
	name := file
	if name == "" {
		name = "still"
	}
	base := snapshotExtensionPattern.ReplaceAllString(name, "")
	base = snapshotKebab(base)
	extension := "bin"
	if match := snapshotExtensionPattern.FindStringSubmatch(file); len(match) == 2 {
		extension = strings.ToLower(match[1])
	} else if mime != nil {
		extension = map[string]string{
			"image/png":  "png",
			"image/jpeg": "jpg",
			"image/webp": "webp",
			"image/gif":  "gif",
		}[*mime]
		if extension == "" {
			extension = "bin"
		}
	}
	return fmt.Sprintf("sources/%02d-%s.%s", index+1, base, extension)
}

func snapshotKebab(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = snapshotSeparatorPattern.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if len(value) > 60 {
		value = value[:60]
	}
	if value == "" {
		return "plate"
	}
	return value
}

func sanitizeSnapshotName(value string) string {
	value = strings.ReplaceAll(value, "*/", "*")
	value = snapshotNewlinePattern.ReplaceAllString(value, " ")
	value = strings.TrimSpace(value)
	if value == "" {
		return "Plate"
	}
	var builder strings.Builder
	units := 0
	for _, r := range value {
		next := 1
		if r > 0xffff {
			next = 2
		}
		if units+next > 40 {
			break
		}
		builder.WriteRune(r)
		units += next
	}
	return builder.String()
}

func roundSnapshot(value float64, places int) float64 {
	factor := math.Pow10(places)
	return math.Round(value*factor) / factor
}

func copyFloatPointer(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copy := roundSnapshot(*value, 4)
	return &copy
}

func firstNonempty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
