// Package palette defines the durable palette.json contract shared by the
// JavaScript prototype and the incremental Go implementation.
package palette

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"scene-palette/internal/colorx"
)

const (
	SchemaVersion                         = 1
	ToolVersion                           = "2.1.0"
	ExtractionVersion                     = 5
	Kind                                  = "color-plate"
	ReviewScopePaletteContent             = "palette-content"
	ReviewScopePaletteAndSourceIdentities = "palette-and-source-identities"
)

var (
	hexPattern               = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	reviewFingerprintPattern = regexp.MustCompile(`^[0-9a-f]{16}$`)
	reviewNewlinePattern     = regexp.MustCompile(`[\r\n]+`)
	coreRoles                = [...]string{"ground", "panel", "ink", "muted", "observed", "attention", "rupture", "gold"}
)

// Generator identifies the application and extraction algorithm that wrote a
// plate. Extraction is a pointer because index.json represents a missing value
// as JSON null instead of silently inventing a version.
type Generator struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	Extraction *int   `json:"extraction"`
}

type Hash struct {
	Algo  string `json:"algo"`
	Value string `json:"value"`
}

// ReviewDecision is recorded provenance about how a human changed the browser
// proposal. VerifyReview exposes this summary but does not treat it as proof;
// only the independently recomputed fingerprint for the declared scope grants
// agent eligibility.
type ReviewDecision struct {
	Version     int  `json:"version"`
	Baseline    bool `json:"baseline"`
	ChangeCount int  `json:"changeCount"`
	FieldCount  int  `json:"fieldCount"`
}

// ReviewAssessment is advisory, deterministic review guidance. It never
// overrides a human seal; VerifyReview recomputes it so agents can see current
// attention items without trusting browser-recorded metadata.
type ReviewAssessment struct {
	Version        int                     `json:"version"`
	Status         string                  `json:"status"`
	AttentionCount int                     `json:"attentionCount"`
	Checks         []ReviewAssessmentCheck `json:"checks"`
}

type ReviewAssessmentCheck struct {
	ID     string   `json:"id"`
	Label  string   `json:"label"`
	Status string   `json:"status"`
	Roles  []string `json:"roles,omitempty"`
	Detail string   `json:"detail"`
}

// ReviewAssessmentSummary is the compact advisory result recorded when a
// prior seal was made. Unlike ReviewAssessment, it is not recomputed because
// the historical palette content is no longer present.
type ReviewAssessmentSummary struct {
	Version        int    `json:"version"`
	Status         string `json:"status"`
	AttentionCount int    `json:"attentionCount"`
}

// ReviewHistoryEntry records a superseded human seal and why it ceased to be
// current. It is audit provenance only: VerifyReview exposes but never treats
// it as independently verified.
type ReviewHistoryEntry struct {
	Version       int                      `json:"version"`
	SealedAt      *string                  `json:"sealedAt,omitempty"`
	SealedBy      *string                  `json:"sealedBy,omitempty"`
	Scope         string                   `json:"scope"`
	Fingerprint   *Hash                    `json:"fingerprint,omitempty"`
	Decision      *ReviewDecision          `json:"decision,omitempty"`
	Assessment    *ReviewAssessmentSummary `json:"assessment,omitempty"`
	InvalidatedAt *string                  `json:"invalidatedAt,omitempty"`
	InvalidatedBy *string                  `json:"invalidatedBy,omitempty"`
}

type Source struct {
	File string `json:"file"`
	Hash *Hash  `json:"hash"`
}

// Review is optional browser-owned human approval metadata. Absence is
// deliberately equivalent to an unsealed plate. VerifyReview checks the claim
// against the complete semantic palette content before an agent may use it.
type Review struct {
	Version       int                  `json:"version"`
	Status        string               `json:"status"`
	AgentEligible bool                 `json:"agentEligible"`
	ChangedBy     *string              `json:"changedBy"`
	SealedAt      *string              `json:"sealedAt"`
	SealedBy      *string              `json:"sealedBy"`
	Fingerprint   *Hash                `json:"fingerprint"`
	Scope         string               `json:"scope"`
	Decision      *ReviewDecision      `json:"decision,omitempty"`
	History       []ReviewHistoryEntry `json:"history,omitempty"`
}

// ReviewVerification is the machine-readable result of checking the optional
// human seal. AgentEligible is true only for a complete, matching version-1
// seal; callers should use this result, never Review.AgentEligible directly.
type ReviewVerification struct {
	Status             string               `json:"status"`
	Scope              string               `json:"scope,omitempty"`
	AgentEligible      bool                 `json:"agentEligible"`
	FingerprintMatches bool                 `json:"fingerprintMatches"`
	Reason             string               `json:"reason"`
	Computed           Hash                 `json:"computedFingerprint"`
	Recorded           *Hash                `json:"recordedFingerprint,omitempty"`
	RecordedDecision   *ReviewDecision      `json:"recordedDecision,omitempty"`
	RecordedHistory    []ReviewHistoryEntry `json:"recordedHistory,omitempty"`
	Assessment         ReviewAssessment     `json:"assessment"`
}

// Token contains only the fields needed to validate and index a role. The JSON
// decoder intentionally ignores richer provenance and colour metadata it does
// not yet need to interpret.
type Token struct {
	Key     string          `json:"key"`
	ID      string          `json:"id"`
	Name    string          `json:"name"`
	Job     string          `json:"job"`
	Hex     string          `json:"hex"`
	Derived bool            `json:"derived"`
	Origin  json.RawMessage `json:"origin"`
}

func (t Token) Role() string {
	if t.ID != "" {
		return t.ID
	}
	return t.Key
}

// Document is the strict output contract written by Scene Palette. The browser
// importer remains deliberately more tolerant; this type is for durable vault
// files, where accepting an incomplete document would make an index unreliable.
type Document struct {
	Schema     int             `json:"schema"`
	Kind       string          `json:"kind"`
	Name       string          `json:"name"`
	CreatedAt  *string         `json:"createdAt"`
	Generator  Generator       `json:"generator"`
	Polarity   string          `json:"polarity"`
	LightScene bool            `json:"lightScene"`
	Family     *string         `json:"family"`
	Tags       []string        `json:"tags"`
	Sources    []Source        `json:"sources"`
	Core       []Token         `json:"core"`
	Found      []Token         `json:"found"`
	Facets     json.RawMessage `json:"facets"`
	Signature  json.RawMessage `json:"signature"`
	Review     *Review         `json:"review,omitempty"`
}

func Decode(r io.Reader) (Document, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Document{}, fmt.Errorf("read palette: %w", err)
	}
	return Parse(data)
}

func Parse(data []byte) (Document, error) {
	var document Document
	if err := json.Unmarshal(data, &document); err != nil {
		return Document{}, fmt.Errorf("decode palette: %w", err)
	}
	if err := document.Validate(); err != nil {
		return Document{}, err
	}
	return document, nil
}

func (document Document) Validate() error {
	if document.Schema != SchemaVersion {
		return fmt.Errorf("palette schema %d is unsupported; expected %d", document.Schema, SchemaVersion)
	}
	if document.Kind != Kind {
		return fmt.Errorf("palette kind %q is unsupported; expected %q", document.Kind, Kind)
	}
	if strings.TrimSpace(document.Name) == "" {
		return fmt.Errorf("palette name is required")
	}
	if document.CreatedAt == nil || strings.TrimSpace(*document.CreatedAt) == "" {
		return fmt.Errorf("palette createdAt is required")
	}
	if document.Generator.Name == "" || document.Generator.Version == "" || document.Generator.Extraction == nil {
		return fmt.Errorf("palette generator name, version, and extraction are required")
	}
	if document.Polarity != "dark" && document.Polarity != "light" {
		return fmt.Errorf("palette polarity must be dark or light")
	}
	if len(document.Core) != len(coreRoles) {
		return fmt.Errorf("palette core has %d roles; expected %d", len(document.Core), len(coreRoles))
	}
	for i, expected := range coreRoles {
		token := document.Core[i]
		if token.Role() != expected {
			return fmt.Errorf("palette core role %d is %q; expected %q", i, token.Role(), expected)
		}
		if !hexPattern.MatchString(token.Hex) {
			return fmt.Errorf("palette role %q has invalid hex %q", expected, token.Hex)
		}
		if !isJSONObject(token.Origin) {
			return fmt.Errorf("palette role %q has no origin object", expected)
		}
	}
	if !isJSONObject(document.Facets) {
		return fmt.Errorf("palette facets must be an object")
	}
	if !isJSONObject(document.Signature) {
		return fmt.Errorf("palette signature must be an object")
	}
	return nil
}

// AssessReview identifies deterministic facts a person should judge before
// sealing. Attention is advisory: it does not change AgentEligible.
func (document Document) AssessReview() ReviewAssessment {
	checks := make([]ReviewAssessmentCheck, 0, 4)
	addCheck := func(id, label string, attention bool, roles []string, detail string) {
		status := "pass"
		if attention {
			status = "attention"
		}
		checks = append(checks, ReviewAssessmentCheck{
			ID: id, Label: label, Status: status,
			Roles: uniqueReviewRoles(roles), Detail: detail,
		})
	}

	var readabilityDetails, readabilityRoles []string
	panelStep := 0.0
	if len(document.Core) != len(coreRoles) {
		readabilityDetails = append(readabilityDetails,
			fmt.Sprintf("core has %d of %d required roles", len(document.Core), len(coreRoles)))
	} else {
		ground, groundOK := reviewRGB(document.Core[0].Hex)
		panel, panelOK := reviewRGB(document.Core[1].Hex)
		if !groundOK || !panelOK {
			readabilityDetails = append(readabilityDetails, "ground or panel has no readable color")
			readabilityRoles = append(readabilityRoles, "ground", "panel")
		} else {
			for index, token := range document.Core[2:] {
				role := reviewAssessmentRole(token, index+2)
				color, ok := reviewRGB(token.Hex)
				target := 3.0
				if role == "ink" {
					target = 4.5
				}
				if !ok {
					readabilityDetails = append(readabilityDetails, role+" has no readable color")
					readabilityRoles = append(readabilityRoles, role)
					continue
				}
				for _, surface := range []struct {
					name  string
					color colorx.RGB
				}{{"ground", ground}, {"panel", panel}} {
					ratio := colorx.Contrast(color, surface.color)
					if ratio < target {
						readabilityDetails = append(readabilityDetails,
							fmt.Sprintf("%s on %s %.2f:1 < %.1f:1", role, surface.name, ratio, target))
						readabilityRoles = append(readabilityRoles, role)
					}
				}
			}
			panelStep = math.Abs(colorx.RGBToLCH(panel).L - colorx.RGBToLCH(ground).L)
			if panelStep < 0.03 {
				readabilityDetails = append(readabilityDetails,
					fmt.Sprintf("panel separation ΔL %.2f < 0.03", panelStep))
				readabilityRoles = append(readabilityRoles, "ground", "panel")
			}
		}
	}
	readabilityDetail := strings.Join(readabilityDetails, "; ")
	if readabilityDetail == "" {
		readabilityDetail = fmt.Sprintf(
			"All six readable roles clear ground and panel; panel separation ΔL %.2f.", panelStep)
	}
	addCheck("readability", "Readability", len(readabilityDetails) > 0,
		readabilityRoles, readabilityDetail)

	allTokens := make([]Token, 0, len(document.Core)+len(document.Found))
	allTokens = append(allTokens, document.Core...)
	allTokens = append(allTokens, document.Found...)
	var originRoles []string
	for index, token := range allTokens {
		kind := reviewOriginKind(token.Origin)
		if kind == "unknown" || kind == "imported" {
			originRoles = append(originRoles, reviewAssessmentRole(token, index))
		}
	}
	originDetail := fmt.Sprintf("Every token has an explicit origin (%d total).", len(allTokens))
	if len(originRoles) > 0 {
		originDetail = "Missing native origin for " + strings.Join(uniqueReviewRoles(originRoles), ", ") + "."
	}
	addCheck("origins", "Origins", len(originRoles) > 0, originRoles, originDetail)

	missingHashes := 0
	algorithms := make([]string, 0, len(document.Sources))
	algorithmSeen := map[string]bool{}
	for _, source := range document.Sources {
		if source.Hash == nil || strings.TrimSpace(source.Hash.Algo) == "" ||
			strings.TrimSpace(source.Hash.Value) == "" {
			missingHashes++
			continue
		}
		algorithm := reviewText(source.Hash.Algo)
		if !algorithmSeen[algorithm] {
			algorithmSeen[algorithm] = true
			algorithms = append(algorithms, algorithm)
		}
	}
	var sourceDetail string
	switch {
	case len(document.Sources) == 0:
		sourceDetail = "No source records are attached."
	case missingHashes > 0:
		verb := "have"
		if missingHashes == 1 {
			verb = "has"
		}
		sourceDetail = fmt.Sprintf("%d of %d source records %s no hash.",
			missingHashes, len(document.Sources), verb)
	default:
		sourceDetail = fmt.Sprintf("%d source records hashed", len(document.Sources))
		if len(algorithms) > 0 {
			sourceDetail += " (" + strings.Join(algorithms, ", ") + ")"
		}
		sourceDetail += "."
	}
	addCheck("sources", "Source records", len(document.Sources) == 0 || missingHashes > 0,
		nil, sourceDetail)

	var inventedRoles []string
	for index, token := range document.Core {
		kind := reviewOriginKind(token.Origin)
		if token.Derived || kind == "derived" || kind == "fallback" {
			inventedRoles = append(inventedRoles, reviewAssessmentRole(token, index))
		}
	}
	inventionDetail := "No core role was derived or filled."
	if len(inventedRoles) > 0 {
		pronoun := "it was"
		if len(uniqueReviewRoles(inventedRoles)) > 1 {
			pronoun = "they were"
		}
		inventionDetail = fmt.Sprintf("Inspect %s; %s derived or filled because the sources lacked a distinct role color.",
			strings.Join(uniqueReviewRoles(inventedRoles), ", "), pronoun)
	}
	addCheck("invention", "Machine invention", len(inventedRoles) > 0,
		inventedRoles, inventionDetail)

	assessment := ReviewAssessment{Version: 1, Status: "clear", Checks: checks}
	for _, check := range checks {
		if check.Status == "attention" {
			assessment.AttentionCount++
		}
	}
	if assessment.AttentionCount > 0 {
		assessment.Status = "attention"
	}
	return assessment
}

// ContentFingerprint reproduces the browser's legacy palette-content
// fingerprint. It remains stable so existing human seals can still be checked.
func (document Document) ContentFingerprint() (Hash, error) {
	return document.reviewFingerprint(false)
}

// ReviewFingerprint computes the payload declared by a supported review
// scope. New seals include normalized source file/hash identities; legacy seals
// continue to cover palette content only.
func (document Document) ReviewFingerprint(scope string) (Hash, error) {
	switch scope {
	case ReviewScopePaletteContent:
		return document.reviewFingerprint(false)
	case ReviewScopePaletteAndSourceIdentities:
		return document.reviewFingerprint(true)
	default:
		return Hash{}, fmt.Errorf("review scope %q is unsupported", scope)
	}
}

// The structs below deliberately declare fields in alphabetical JSON-key
// order, matching the browser's recursively sorted payload.
func (document Document) reviewFingerprint(includeSources bool) (Hash, error) {
	type fingerprintToken struct {
		Derived bool    `json:"derived"`
		Hex     string  `json:"hex"`
		ID      *string `json:"id"`
		Job     string  `json:"job"`
		Name    string  `json:"name"`
		Origin  any     `json:"origin"`
	}
	type fingerprintSource struct {
		File string `json:"file"`
		Hash *Hash  `json:"hash"`
	}
	type fingerprintPayload struct {
		Core     []fingerprintToken   `json:"core"`
		Found    []fingerprintToken   `json:"found"`
		Name     string               `json:"name"`
		Polarity string               `json:"polarity"`
		Sources  *[]fingerprintSource `json:"sources,omitempty"`
	}

	pack := func(tokens []Token) ([]fingerprintToken, error) {
		packed := make([]fingerprintToken, len(tokens))
		for index, token := range tokens {
			origin, err := fingerprintOrigin(token.Origin)
			if err != nil {
				return nil, fmt.Errorf("fingerprint %s origin: %w", token.Role(), err)
			}
			var id *string
			if token.ID != "" {
				value := token.ID
				id = &value
			}
			packed[index] = fingerprintToken{
				Derived: token.Derived,
				Hex:     strings.ToUpper(token.Hex),
				ID:      id,
				Job:     reviewText(token.Job),
				Name:    reviewText(token.Name),
				Origin:  origin,
			}
		}
		return packed, nil
	}

	core, err := pack(document.Core)
	if err != nil {
		return Hash{}, err
	}
	found, err := pack(document.Found)
	if err != nil {
		return Hash{}, err
	}
	polarity := "dark"
	if document.LightScene || document.Polarity == "light" {
		polarity = "light"
	}
	payload := fingerprintPayload{
		Core:     core,
		Found:    found,
		Name:     reviewText(document.Name),
		Polarity: polarity,
	}
	if includeSources {
		count := len(document.Sources)
		if count > 8 {
			count = 8
		}
		sources := make([]fingerprintSource, count)
		for index, source := range document.Sources[:count] {
			file := source.File
			if file == "" {
				file = "still"
			}
			var hash *Hash
			if source.Hash != nil && source.Hash.Algo != "" && source.Hash.Value != "" {
				hash = &Hash{
					Algo:  reviewText(source.Hash.Algo),
					Value: reviewHashValue(source.Hash.Value),
				}
			}
			sources[index] = fingerprintSource{File: reviewText(file), Hash: hash}
		}
		payload.Sources = &sources
	}
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(payload); err != nil {
		return Hash{}, fmt.Errorf("encode fingerprint payload: %w", err)
	}
	data := bytes.TrimSuffix(encoded.Bytes(), []byte{'\n'})
	return Hash{Algo: "fnv1a64", Value: reviewFNV1A64(data)}, nil
}

// VerifyReview treats every incomplete, unsupported, or mismatched seal as
// non-eligible. A missing review is a normal proposal, not a contract error.
func (document Document) VerifyReview() (ReviewVerification, error) {
	computed, err := document.ContentFingerprint()
	if err != nil {
		return ReviewVerification{}, err
	}
	result := ReviewVerification{
		Status: "proposed", Reason: "no human review recorded", Computed: computed,
		Assessment: document.AssessReview(),
	}
	review := document.Review
	if review == nil {
		return result, nil
	}
	result.Status = review.Status
	result.Scope = review.Scope
	result.Recorded = review.Fingerprint
	result.RecordedDecision = review.Decision
	if review.Scope == ReviewScopePaletteAndSourceIdentities {
		computed, err = document.ReviewFingerprint(review.Scope)
		if err != nil {
			return ReviewVerification{}, err
		}
		result.Computed = computed
	}
	// Only the current fingerprint can be checked against current content.
	// Prior entries remain bounded, recorded provenance—not verified claims.
	if count := len(review.History); count > 0 {
		start := 0
		if count > 20 {
			start = count - 20
		}
		result.RecordedHistory = append([]ReviewHistoryEntry(nil), review.History[start:]...)
	}
	if review.Version != 1 {
		result.Reason = fmt.Sprintf("review version %d is unsupported", review.Version)
		return result, nil
	}
	if review.Status != "sealed" {
		result.Reason = fmt.Sprintf("review status is %q, not sealed", review.Status)
		return result, nil
	}
	if !review.AgentEligible {
		result.Reason = "seal is not marked agent-eligible"
		return result, nil
	}
	if review.SealedBy == nil || *review.SealedBy != "human" {
		result.Reason = "seal was not recorded by a human"
		return result, nil
	}
	if review.SealedAt == nil || strings.TrimSpace(*review.SealedAt) == "" {
		result.Reason = "seal has no timestamp"
		return result, nil
	}
	if _, err := time.Parse(time.RFC3339Nano, *review.SealedAt); err != nil {
		result.Reason = "seal timestamp is invalid"
		return result, nil
	}
	if review.Scope != ReviewScopePaletteContent &&
		review.Scope != ReviewScopePaletteAndSourceIdentities {
		result.Reason = fmt.Sprintf("review scope %q is unsupported", review.Scope)
		return result, nil
	}
	if review.Fingerprint == nil || review.Fingerprint.Algo != "fnv1a64" ||
		!reviewFingerprintPattern.MatchString(review.Fingerprint.Value) {
		result.Reason = "seal fingerprint is missing or unsupported"
		return result, nil
	}
	result.FingerprintMatches = review.Fingerprint.Value == computed.Value
	if !result.FingerprintMatches {
		if review.Scope == ReviewScopePaletteAndSourceIdentities {
			result.Reason = "palette content or source identities do not match the human seal"
		} else {
			result.Reason = "palette content does not match the human seal"
		}
		return result, nil
	}
	result.AgentEligible = true
	result.Reason = "human seal verified"
	return result, nil
}

func reviewText(value string) string {
	value = strings.ReplaceAll(value, "*/", "*")
	value = strings.TrimSpace(reviewNewlinePattern.ReplaceAllString(value, " "))
	runes := []rune(value)
	if len(runes) > 40 {
		value = string(runes[:40])
	}
	return value
}

func reviewHashValue(value string) string {
	runes := []rune(value)
	if len(runes) > 128 {
		return string(runes[:128])
	}
	return value
}

func reviewRGB(value string) (colorx.RGB, bool) {
	if !hexPattern.MatchString(value) {
		return colorx.RGB{}, false
	}
	parse := func(part string) int {
		channel, _ := strconv.ParseUint(part, 16, 8)
		return int(channel)
	}
	return colorx.RGB{
		R: parse(value[1:3]),
		G: parse(value[3:5]),
		B: parse(value[5:7]),
	}, true
}

func reviewAssessmentRole(token Token, index int) string {
	if role := reviewText(token.Role()); role != "" {
		return role
	}
	return fmt.Sprintf("token_%d", index+1)
}

func reviewOriginKind(raw json.RawMessage) string {
	var origin struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(raw, &origin); err != nil {
		return "unknown"
	}
	kind := strings.ToLower(strings.TrimSpace(origin.Kind))
	if kind == "" {
		return "unknown"
	}
	return kind
}

func uniqueReviewRoles(roles []string) []string {
	if len(roles) == 0 {
		return nil
	}
	unique := make([]string, 0, len(roles))
	seen := make(map[string]bool, len(roles))
	for _, role := range roles {
		if role != "" && !seen[role] {
			seen[role] = true
			unique = append(unique, role)
		}
	}
	return unique
}

func fingerprintOrigin(raw json.RawMessage) (any, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return map[string]any{"kind": "unknown"}, nil
	}
	var value any
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func reviewFNV1A64(data []byte) string {
	h1, h2 := uint32(0x811c9dc5), uint32(0x01000193)
	for index, value := range data {
		h1 = (h1 ^ uint32(value)) * uint32(0x01000193)
		h2 = (h2 ^ (uint32(value) + uint32(index&0xff))) * uint32(0x85ebca6b)
	}
	return fmt.Sprintf("%08x%08x", h1, h2)
}

func isJSONObject(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) >= 2 && trimmed[0] == '{' && trimmed[len(trimmed)-1] == '}' && json.Valid(trimmed)
}
