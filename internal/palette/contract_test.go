package palette

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseContractFixture(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "vault", "night-bay", "palette.json"))
	if err != nil {
		t.Fatal(err)
	}
	document, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if document.Name != "Night bay" || len(document.Core) != 8 {
		t.Fatalf("unexpected document: %q with %d roles", document.Name, len(document.Core))
	}
	if document.Core[4].Role() != "observed" {
		t.Fatalf("role 4 = %q; want observed", document.Core[4].Role())
	}
}

func TestParseExposesOptionalHumanReview(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "vault", "night-bay", "palette.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	raw["review"] = map[string]any{
		"version": 1, "status": "sealed", "agentEligible": true,
		"sealedBy": "human", "scope": "palette-content",
		"fingerprint": map[string]any{"algo": "fnv1a64", "value": "0123456789abcdef"},
		"decision": map[string]any{
			"version": 1, "baseline": true, "changeCount": 2, "fieldCount": 3,
			"changes": []any{
				map[string]any{"key": "observed", "fields": []any{"name", "hex"}},
			},
		},
		"history": []any{
			map[string]any{
				"version": 1, "sealedAt": "2026-08-29T12:34:56.000Z",
				"sealedBy": "human", "scope": "palette-content",
				"fingerprint": map[string]any{"algo": "fnv1a64", "value": "fedcba9876543210"},
				"decision": map[string]any{
					"version": 1, "baseline": true, "changeCount": 0, "fieldCount": 0,
				},
				"assessment": map[string]any{
					"version": 1, "status": "attention", "attentionCount": 1,
				},
				"invalidatedAt": "2026-08-30T09:00:00.000Z",
				"invalidatedBy": "source still removed",
			},
		},
	}
	data, err = json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	document, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if document.Review == nil || document.Review.Status != "sealed" || !document.Review.AgentEligible ||
		document.Review.Fingerprint == nil || document.Review.Fingerprint.Algo != "fnv1a64" ||
		document.Review.Decision == nil || document.Review.Decision.ChangeCount != 2 ||
		document.Review.Decision.FieldCount != 3 || len(document.Review.History) != 1 {
		t.Fatalf("unexpected review: %#v", document.Review)
	}
	verification, err := document.VerifyReview()
	if err != nil {
		t.Fatal(err)
	}
	if verification.RecordedDecision == nil || !verification.RecordedDecision.Baseline ||
		verification.RecordedDecision.ChangeCount != 2 {
		t.Fatalf("recorded decision not exposed: %#v", verification)
	}
	if len(verification.RecordedHistory) != 1 ||
		verification.RecordedHistory[0].InvalidatedBy == nil ||
		*verification.RecordedHistory[0].InvalidatedBy != "source still removed" ||
		verification.RecordedHistory[0].Assessment == nil ||
		verification.RecordedHistory[0].Assessment.AttentionCount != 1 {
		t.Fatalf("recorded history not exposed: %#v", verification)
	}
}

func TestContentFingerprintMatchesBrowserAndDetectsTampering(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "vault", "night-bay", "palette.json"))
	if err != nil {
		t.Fatal(err)
	}
	document, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, err := document.ContentFingerprint()
	if err != nil {
		t.Fatal(err)
	}
	if fingerprint.Algo != "fnv1a64" || fingerprint.Value != "1281aeeff2fc99f7" {
		t.Fatalf("fingerprint = %#v; want browser parity value", fingerprint)
	}
	sourceBound, err := document.ReviewFingerprint(ReviewScopePaletteAndSourceIdentities)
	if err != nil {
		t.Fatal(err)
	}
	if sourceBound.Algo != "fnv1a64" || sourceBound.Value != "11fa430c543fc1c5" {
		t.Fatalf("source-bound fingerprint = %#v; want browser parity value", sourceBound)
	}

	sealedAt, sealedBy := "2026-08-30T12:34:56.000Z", "human"
	document.Review = &Review{
		Version: 1, Status: "sealed", AgentEligible: true,
		SealedAt: &sealedAt, SealedBy: &sealedBy,
		Fingerprint: &sourceBound, Scope: ReviewScopePaletteAndSourceIdentities,
	}
	verification, err := document.VerifyReview()
	if err != nil {
		t.Fatal(err)
	}
	if !verification.AgentEligible || !verification.FingerprintMatches {
		t.Fatalf("valid source-bound seal rejected: %#v", verification)
	}
	if verification.Scope != ReviewScopePaletteAndSourceIdentities {
		t.Fatalf("verified scope = %q", verification.Scope)
	}

	originalSourceHash := document.Sources[0].Hash.Value
	document.Sources[0].Hash.Value = "changed-source-hash"
	verification, err = document.VerifyReview()
	if err != nil {
		t.Fatal(err)
	}
	if verification.AgentEligible || verification.FingerprintMatches ||
		!strings.Contains(verification.Reason, "source identities") {
		t.Fatalf("source-tampered plate accepted: %#v", verification)
	}
	document.Sources[0].Hash.Value = originalSourceHash

	document.Review.Fingerprint = &fingerprint
	document.Review.Scope = ReviewScopePaletteContent
	verification, err = document.VerifyReview()
	if err != nil {
		t.Fatal(err)
	}
	if !verification.AgentEligible || !verification.FingerprintMatches {
		t.Fatalf("valid legacy seal rejected: %#v", verification)
	}

	document.Core[4].Hex = "#ABCDEF"
	verification, err = document.VerifyReview()
	if err != nil {
		t.Fatal(err)
	}
	if verification.AgentEligible || verification.FingerprintMatches ||
		!strings.Contains(verification.Reason, "does not match") {
		t.Fatalf("tampered plate accepted: %#v", verification)
	}
}

func TestReviewAssessmentSurfacesDeterministicAttention(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "vault", "night-bay", "palette.json"))
	if err != nil {
		t.Fatal(err)
	}
	document, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	document.Core[2].Hex = document.Core[0].Hex
	document.Core[4].Derived = true
	document.Core[5].Origin = json.RawMessage(`{"kind":"imported"}`)
	document.Sources = nil

	assessment := document.AssessReview()
	if assessment.Version != 1 || assessment.Status != "attention" || assessment.AttentionCount != 4 ||
		len(assessment.Checks) != 4 {
		t.Fatalf("unexpected assessment: %#v", assessment)
	}
	checks := make(map[string]ReviewAssessmentCheck, len(assessment.Checks))
	for _, check := range assessment.Checks {
		checks[check.ID] = check
	}
	hasRole := func(check ReviewAssessmentCheck, role string) bool {
		for _, candidate := range check.Roles {
			if candidate == role {
				return true
			}
		}
		return false
	}
	if checks["readability"].Status != "attention" || !hasRole(checks["readability"], "ink") {
		t.Fatalf("readability check = %#v", checks["readability"])
	}
	if checks["origins"].Status != "attention" || !hasRole(checks["origins"], "attention") {
		t.Fatalf("origins check = %#v", checks["origins"])
	}
	if checks["sources"].Status != "attention" {
		t.Fatalf("sources check = %#v", checks["sources"])
	}
	if checks["invention"].Status != "attention" || !hasRole(checks["invention"], "observed") {
		t.Fatalf("invention check = %#v", checks["invention"])
	}

	verification, err := document.VerifyReview()
	if err != nil {
		t.Fatal(err)
	}
	if verification.Assessment.AttentionCount != assessment.AttentionCount {
		t.Fatalf("verification assessment = %#v; want %#v", verification.Assessment, assessment)
	}
}

func TestParseRejectsIncompleteDurableContract(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "vault", "night-bay", "palette.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	delete(raw, "signature")
	data, err = json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(data); err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("Parse error = %v; want missing signature error", err)
	}
}

func TestParseRejectsRoleReordering(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "vault", "day-facade", "palette.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	core := raw["core"].([]any)
	core[0], core[1] = core[1], core[0]
	data, err = json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(data); err == nil || !strings.Contains(err.Error(), "expected") {
		t.Fatalf("Parse error = %v; want role-order error", err)
	}
}
