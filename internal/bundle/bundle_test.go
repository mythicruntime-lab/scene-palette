package bundle

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"scene-palette/internal/palette"
	"scene-palette/internal/vault"
)

func fixture(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "vault", "night-bay", "palette.json"))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	return raw
}

func encodeFixture(t *testing.T, raw map[string]json.RawMessage) []byte {
	t.Helper()
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func zipFiles(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	files := make(map[string][]byte, len(archive.File))
	for _, file := range archive.File {
		if strings.Contains(file.Name, "..") || strings.Contains(file.Name, `\`) || strings.HasPrefix(file.Name, "/") {
			t.Fatalf("unsafe entry %q", file.Name)
		}
		if _, duplicate := files[file.Name]; duplicate {
			t.Fatalf("duplicate entry %q", file.Name)
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		files[file.Name] = content
	}
	return files
}

func TestBuildPreservesDocumentAndSealWithoutSourceBytes(t *testing.T) {
	raw := fixture(t)
	raw["extension"] = json.RawMessage(`{"owner":"human","revision":9007199254740993}`)
	doc, err := palette.Parse(encodeFixture(t, raw))
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, err := doc.ReviewFingerprint(palette.ReviewScopePaletteAndSourceIdentities)
	if err != nil {
		t.Fatal(err)
	}
	raw["review"], _ = json.Marshal(map[string]any{
		"version": 1, "status": "sealed", "agentEligible": true, "sealedAt": "2026-08-30T12:34:56.000Z", "sealedBy": "human",
		"scope": palette.ReviewScopePaletteAndSourceIdentities, "fingerprint": fingerprint,
		"proposal": map[string]any{"version": 1, "extra": "retained"}, "history": []any{map[string]any{"record": "not dropped"}},
	})
	input := encodeFixture(t, raw)
	packed, err := Build(input, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	files := zipFiles(t, packed)
	if len(files) != 6 {
		t.Fatalf("file count = %d; want 6", len(files))
	}
	if _, exists := files["index.json"]; exists {
		t.Fatal("single bundle included a vault index")
	}
	for _, name := range []string{"palette.json", "tokens.css", "plate.gd", "palette.gpl", "notes.md", "preview.png"} {
		if len(files["night-bay/"+name]) == 0 {
			t.Fatalf("missing %s", name)
		}
	}
	var output map[string]json.RawMessage
	if err := json.Unmarshal(files["night-bay/palette.json"], &output); err != nil {
		t.Fatal(err)
	}
	for key, value := range raw {
		if key == "sources" {
			continue
		}
		var before, after bytes.Buffer
		_ = json.Compact(&before, value)
		_ = json.Compact(&after, output[key])
		if !bytes.Equal(before.Bytes(), after.Bytes()) {
			t.Fatalf("field %s changed\nbefore %s\nafter %s", key, value, output[key])
		}
	}
	var sources []struct {
		File     string
		Path     *string
		Retained *bool
	}
	_ = json.Unmarshal(output["sources"], &sources)
	if len(sources) != 1 || sources[0].Path != nil || sources[0].Retained == nil || *sources[0].Retained {
		t.Fatalf("missing source not marked honestly: %s", output["sources"])
	}
	doc, err = palette.Parse(files["night-bay/palette.json"])
	if err != nil {
		t.Fatal(err)
	}
	review, err := doc.VerifyReview()
	if err != nil || !review.AgentEligible {
		t.Fatalf("bundle invalidated human seal: %#v (%v)", review, err)
	}
	if !strings.Contains(string(files["night-bay/tokens.css"]), "human-sealed · agent-eligible") {
		t.Fatal("CSS omitted verified seal")
	}
	again, err := Build(input, nil, nil)
	if err != nil || !bytes.Equal(again, packed) {
		t.Fatal("identical inputs produced different ZIP bytes")
	}
	preview, err := png.Decode(bytes.NewReader(files["night-bay/preview.png"]))
	if err != nil {
		t.Fatal(err)
	}
	if preview.Bounds().Dx() != 960 || preview.Bounds().Dy() != 250 {
		t.Fatal("wrong preview dimensions")
	}
	if got := color.NRGBAModel.Convert(preview.At(4*120+20, 100)).(color.NRGBA); got != (color.NRGBA{R: 0x38, G: 0xc4, B: 0xd8, A: 255}) {
		t.Fatalf("preview observed color = %#v", got)
	}
}

func TestBuildMatchesHashesAndNormalizesSourcePaths(t *testing.T) {
	first, second := []byte("first original bytes"), []byte("second original bytes")
	sha := sha256.Sum256(first)
	raw := fixture(t)
	raw["sources"], _ = json.Marshal([]map[string]any{
		{"file": "unsafe / still.png", "path": "../../escape.png", "hash": palette.Hash{Algo: "sha256", Value: fmt.Sprintf("%x", sha[:])}, "bytes": len(first), "custom": "kept"},
		{"file": "second.png", "path": "sources/CON.png", "hash": palette.Hash{Algo: "fnv1a64", Value: browserFNV(second)}, "bytes": len(second)},
		{"file": "again.png", "path": "sources/01-unsafe-still.png", "hash": palette.Hash{Algo: "sha256", Value: fmt.Sprintf("%x", sha[:])}},
	})
	packed, err := Build(encodeFixture(t, raw), [][]byte{second, first}, nil)
	if err != nil {
		t.Fatal(err)
	}
	files := zipFiles(t, packed)
	for name, want := range map[string][]byte{"night-bay/sources/01-unsafe-still.png": first, "night-bay/sources/02-second.png": second, "night-bay/sources/03-again.png": first} {
		if !bytes.Equal(files[name], want) {
			t.Fatalf("source missing or altered: %s", name)
		}
	}
	var output struct{ Sources []map[string]any }
	_ = json.Unmarshal(files["night-bay/palette.json"], &output)
	if output.Sources[0]["custom"] != "kept" {
		t.Fatal("source extension dropped")
	}
	if _, exists := output.Sources[0]["retained"]; exists {
		t.Fatal("retained false survived successful source reattachment")
	}
	if _, err := Build(encodeFixture(t, raw), [][]byte{[]byte("wrong")}, nil); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("unverified source accepted: %v", err)
	}
	if _, err := Build(encodeFixture(t, raw), nil, []byte("not png")); err == nil {
		t.Fatal("invalid preview accepted")
	}
	var sources []map[string]any
	_ = json.Unmarshal(raw["sources"], &sources)
	sources[0]["bytes"] = 999
	raw["sources"], _ = json.Marshal(sources)
	if _, err := Build(encodeFixture(t, raw), [][]byte{first}, nil); err == nil || !strings.Contains(err.Error(), "byte count") {
		t.Fatalf("false byte count accepted: %v", err)
	}
}

func TestBuildVaultUsesDistinctFoldersAndRebuildableIndex(t *testing.T) {
	raw := fixture(t)
	first := encodeFixture(t, raw)
	raw["name"] = json.RawMessage(`"A plate"`)
	packed, err := BuildVault([]Input{{Palette: first}, {Palette: first}, {Palette: encodeFixture(t, raw)}})
	if err != nil {
		t.Fatal(err)
	}
	files := zipFiles(t, packed)
	root := t.TempDir()
	for name, data := range files {
		out := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(out, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var index vault.Index
	if err := json.Unmarshal(files["index.json"], &index); err != nil {
		t.Fatal(err)
	}
	if index.Count != 3 {
		t.Fatalf("index count=%d", index.Count)
	}
	paths := []string{}
	for _, item := range index.Entries {
		paths = append(paths, item.Path)
	}
	sort.Strings(paths)
	if !reflect.DeepEqual(paths, []string{"a-plate/palette.json", "night-bay-2/palette.json", "night-bay/palette.json"}) {
		t.Fatalf("wrong indexed folders: %#v", paths)
	}
	rebuilt, skipped, err := vault.Rebuild(root, "")
	if err != nil || len(skipped) != 0 {
		t.Fatalf("vault did not rebuild: %v (%v)", err, skipped)
	}
	rebuiltJSON, err := json.MarshalIndent(rebuilt, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.TrimSpace(files["index.json"]), rebuiltJSON) {
		t.Fatalf("bundled index differs from rebuilt index\nbundled: %s\nrebuilt: %s", files["index.json"], rebuiltJSON)
	}
}

func TestFormatExportsCanonicalRolesAndSafeText(t *testing.T) {
	raw := fixture(t)
	raw["name"] = json.RawMessage(`"Unsafe */\nname"`)
	data := encodeFixture(t, raw)
	for _, format := range []string{"css", "godot", "gpl", "hex", "json", "notes"} {
		text, err := Format(data, format)
		if err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		if format != "json" && strings.Contains(text, "*/\nname") {
			t.Fatalf("unsafe comment in %s", format)
		}
		if format != "json" && !strings.Contains(text, "not agent-eligible") {
			t.Fatalf("review state absent in %s", format)
		}
		if format == "godot" && !strings.Contains(text, `const OBSERVED := Color("#38c4d8")`) {
			t.Fatal("wrong Godot color")
		}
		if format == "css" && !strings.Contains(text, "--observed: #38C4D8;") {
			t.Fatal("wrong CSS color")
		}
	}
	raw["name"] = json.RawMessage(`"CON"`)
	filename, err := Filename(encodeFixture(t, raw))
	if err != nil || filename != "plate-con.zip" {
		t.Fatalf("unsafe Windows filename: %q (%v)", filename, err)
	}
}

func TestArchiveBudgetCountsEveryCopyBeforeAllocation(t *testing.T) {
	budget := newArchiveBudget(MaxArchiveBytes)
	// Simulate repeated references to one 64 MiB attachment with only integer
	// sizes. Three copies fit; the fourth plus ZIP overhead must be rejected.
	for index := 0; index < 3; index++ {
		if err := budget.reserve(fmt.Sprintf("plate/sources/%d.png", index), MaxSourceBytes); err != nil {
			t.Fatal(err)
		}
	}
	used := budget.used
	if err := budget.reserve("plate/sources/fourth.png", MaxSourceBytes); err == nil {
		t.Fatal("archive budget accepted four 64 MiB entries plus ZIP overhead")
	}
	if budget.used != used {
		t.Fatal("failed reservation changed the archive budget")
	}
	if err := budget.reserve("overflow.bin", int64(^uint64(0)>>1)); err == nil {
		t.Fatal("huge declared size overflowed the limit arithmetic")
	}
	if err := validateSourceSize(MaxSourceBytes); err != nil {
		t.Fatal(err)
	}
	if err := validateSourceSize(MaxSourceBytes + 1); err == nil {
		t.Fatal("oversized source attachment was accepted")
	}
	writer := limitedArchiveWriter{limit: 8}
	if n, err := writer.Write([]byte("12345678")); n != 8 || err != nil {
		t.Fatalf("write at limit failed: %d, %v", n, err)
	}
	if n, err := writer.Write([]byte("9")); n != 0 || err == nil || writer.Len() != 8 {
		t.Fatalf("writer exceeded final byte limit: n=%d err=%v size=%d", n, err, writer.Len())
	}
}

func TestBuildAllowsExtendedProvenanceButCapsSourceRecords(t *testing.T) {
	raw := fixture(t)
	sources := make([]map[string]any, MaxSourceRecords+1)
	for index := range sources {
		sources[index] = map[string]any{"file": fmt.Sprintf("historical-%d.png", index), "path": nil, "retained": false}
	}
	raw["sources"], _ = json.Marshal(sources[:MaxSourceRecords])
	packed, err := Build(encodeFixture(t, raw), nil, nil)
	if err != nil {
		t.Fatalf("valid historical provenance beyond eight open stills was rejected: %v", err)
	}
	files := zipFiles(t, packed)
	document, err := palette.Parse(files["night-bay/palette.json"])
	if err != nil || len(document.Sources) != MaxSourceRecords {
		t.Fatalf("historical source records were lost: count=%d err=%v", len(document.Sources), err)
	}
	raw["sources"], _ = json.Marshal(sources)
	if _, err := Build(encodeFixture(t, raw), nil, nil); err == nil || !strings.Contains(err.Error(), "source records") {
		t.Fatalf("excessive source count accepted: %v", err)
	}
}

func TestEntryPlanningRejectsAmplificationAndSharesVaultBudget(t *testing.T) {
	raw := fixture(t)
	source := []byte("one shared attachment")
	hash := sha256.Sum256(source)
	sources := make([]map[string]any, 20)
	for index := range sources {
		sources[index] = map[string]any{
			"file": fmt.Sprintf("copy-%d.png", index),
			"hash": palette.Hash{Algo: "sha256", Value: fmt.Sprintf("%x", hash[:])},
		}
	}
	raw["sources"], _ = json.Marshal(sources)
	if _, err := bundleEntries(encodeFixture(t, raw), [][]byte{source}, nil, "", newArchiveBudget(2048)); err == nil || !strings.Contains(err.Error(), "output limit") {
		t.Fatalf("reused source content bypassed entry-planning limit: %v", err)
	}
	// A vault must keep one running budget instead of starting over per plate.
	data := encodeFixture(t, fixture(t))
	budget := newArchiveBudget(MaxArchiveBytes)
	if _, err := bundleEntries(data, nil, nil, "first", budget); err != nil {
		t.Fatal(err)
	}
	budget.limit = budget.used + 512
	if _, err := bundleEntries(data, nil, nil, "second", budget); err == nil || !strings.Contains(err.Error(), "output limit") {
		t.Fatalf("subsequent vault plate bypassed accumulated budget: %v", err)
	}
	if _, err := BuildVault(make([]Input, MaxVaultPlates+1)); err == nil {
		t.Fatal("excessive vault plate count accepted")
	}
}
