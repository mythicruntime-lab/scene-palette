// Package bundle writes portable palette folders as deterministic ZIP files.
// It preserves the durable document, including human-review metadata, and
// attaches original source bytes only after verifying their recorded hashes.
package bundle

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image/png"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"scene-palette/internal/palette"
	"scene-palette/internal/vault"
)

var (
	safeSourcePath  = regexp.MustCompile(`(?i)^sources/[a-z0-9][a-z0-9._-]*$`)
	sourceExtension = regexp.MustCompile(`(?i)\.([a-z0-9]{1,5})$`)
)

const (
	// Provenance can include stills picked during earlier editing sessions, so
	// it is deliberately larger than the eight simultaneously open stills.
	MaxSourceRecords = 256
	MaxSourceBytes   = int64(64 << 20)
	MaxArchiveBytes  = int64(256 << 20)
	MaxVaultPlates   = 41
	zipEntryOverhead = int64(256)
	zipFooterBudget  = int64(256)
)

// archiveBudget counts every occurrence of an attachment, even when entries
// share the same backing bytes. ZIP stores a separate copy at every path.
type archiveBudget struct {
	limit int64
	used  int64
}

func newArchiveBudget(limit int64) *archiveBudget {
	return &archiveBudget{limit: limit, used: zipFooterBudget}
}

func (budget *archiveBudget) reserve(name string, size int64) error {
	if len(name) > 65535 {
		return fmt.Errorf("archive entry name exceeds the ZIP filename limit")
	}
	// Reserve both filename copies plus room for local/central headers,
	// timestamp extras, descriptors and ZIP64 overhead before allocating.
	overhead := 2*int64(len(name)) + zipEntryOverhead
	remaining := budget.limit - budget.used
	if size < 0 || remaining < overhead || size > remaining-overhead {
		return fmt.Errorf("archive exceeds the %d-byte output limit", budget.limit)
	}
	budget.used += size + overhead
	return nil
}

func validateSourceSize(size int64) error {
	if size < 0 || size > MaxSourceBytes {
		return fmt.Errorf("source attachment exceeds the 64 MiB limit")
	}
	return nil
}

// limitedArchiveWriter is a final guard on real bytes, independently of the
// conservative preflight estimate and any future changes to ZIP metadata.
type limitedArchiveWriter struct {
	bytes.Buffer
	limit int64
}

func (writer *limitedArchiveWriter) Write(data []byte) (int, error) {
	if int64(len(data)) > writer.limit-int64(writer.Len()) {
		return 0, fmt.Errorf("archive exceeds the %d-byte output limit", writer.limit)
	}
	return writer.Buffer.Write(data)
}

type entry struct {
	name    string
	content []byte
}

// Input is one durable palette and its available original attachments.
type Input struct {
	Palette []byte
	Sources [][]byte
	Preview []byte
}

// BuildVault creates one complete vault tree. Each plate has a unique folder;
// index.json is generated over those actual folders and is always rebuildable.
func BuildVault(items []Input) ([]byte, error) {
	if len(items) == 0 || len(items) > MaxVaultPlates {
		return nil, fmt.Errorf("vault requires one to %d palettes", MaxVaultPlates)
	}
	entries := []entry{}
	budget := newArchiveBudget(MaxArchiveBytes)
	located := make([]vault.LocatedPalette, 0, len(items))
	used := make(map[string]bool, len(items))
	var readme strings.Builder
	fmt.Fprintf(&readme, "# Palette vault\n\n%d plates. Each folder is the authority.\n\n`index.json` is a cache rebuilt from the `palette.json` in each folder.\n\n| plate | polarity | folder |\n| --- | --- | --- |\n", len(items))
	for index, item := range items {
		if int64(len(item.Palette)) > MaxArchiveBytes {
			return nil, fmt.Errorf("vault palette %d exceeds the archive size limit", index+1)
		}
		document, err := palette.Parse(item.Palette)
		if err != nil {
			return nil, fmt.Errorf("vault palette %d: %w", index+1, err)
		}
		base := bundleSlug(document.Name)
		directory := base
		for suffix := 2; used[directory]; suffix++ {
			directory = fmt.Sprintf("%s-%d", base, suffix)
		}
		used[directory] = true
		files, err := bundleEntries(item.Palette, item.Sources, item.Preview, directory, budget)
		if err != nil {
			return nil, fmt.Errorf("vault palette %d: %w", index+1, err)
		}
		document, err = palette.Parse(files[0].content)
		if err != nil {
			return nil, err
		}
		entries = append(entries, files...)
		located = append(located, vault.LocatedPalette{Document: document, Folder: directory})
		fmt.Fprintf(&readme, "| %s | %s | `%s/` |\n", safeMarkdown(document.Name), document.Polarity, directory)
	}
	sort.SliceStable(located, func(i, j int) bool {
		if located[i].Document.Name == located[j].Document.Name {
			return located[i].Folder < located[j].Folder
		}
		return located[i].Document.Name < located[j].Document.Name
	})
	index, err := json.MarshalIndent(vault.Build(located), "", "  ")
	if err != nil {
		return nil, err
	}
	if err := budget.reserve("index.json", int64(len(index))+1); err != nil {
		return nil, err
	}
	if err := budget.reserve("README.md", int64(readme.Len())); err != nil {
		return nil, err
	}
	entries = append(entries, entry{"index.json", append(index, '\n')}, entry{"README.md", []byte(readme.String())})
	return encodeEntries(entries)
}

// Filename returns the portable default ZIP filename for a valid document.
func Filename(data []byte) (string, error) {
	document, err := palette.Parse(data)
	if err != nil {
		return "", err
	}
	return bundleSlug(document.Name) + ".zip", nil
}

// Build packages one palette. Sources are an unordered collection of original
// bytes matched by the document's sha256 or browser fnv1a64 hashes. Missing
// sources remain as metadata with retained:false and path:null; an unmatched
// supplied attachment is an error. A nil preview generates a labeled swatch
// sheet; otherwise preview must contain a PNG. No vault index is included.
func Build(data []byte, sources [][]byte, preview []byte) ([]byte, error) {
	entries, err := bundleEntries(data, sources, preview, "", newArchiveBudget(MaxArchiveBytes))
	if err != nil {
		return nil, err
	}
	return encodeEntries(entries)
}

func bundleEntries(data []byte, sources [][]byte, preview []byte, directory string, budget *archiveBudget) ([]entry, error) {
	if int64(len(data)) > MaxArchiveBytes {
		return nil, fmt.Errorf("palette data exceeds the archive size limit")
	}
	if len(sources) > MaxSourceRecords {
		return nil, fmt.Errorf("bundle accepts at most %d source attachments", MaxSourceRecords)
	}
	inputBytes := int64(0)
	for _, source := range sources {
		if err := validateSourceSize(int64(len(source))); err != nil {
			return nil, err
		}
		if int64(len(source)) > MaxArchiveBytes-inputBytes {
			return nil, fmt.Errorf("source attachments exceed the archive size limit")
		}
		inputBytes += int64(len(source))
	}
	document, err := palette.Parse(data)
	if err != nil {
		return nil, err
	}
	if _, err := tokens(document); err != nil {
		return nil, err
	}
	if len(document.Sources) > MaxSourceRecords {
		return nil, fmt.Errorf("bundle accepts at most %d source records", MaxSourceRecords)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode bundle palette: %w", err)
	}
	if directory == "" {
		directory = bundleSlug(document.Name)
	}
	raw["folder"], _ = json.Marshal(directory)
	var records []map[string]json.RawMessage
	if value, exists := raw["sources"]; exists {
		if err := json.Unmarshal(value, &records); err != nil {
			return nil, fmt.Errorf("decode bundle sources: %w", err)
		}
	}
	if records == nil {
		records = []map[string]json.RawMessage{}
	}
	available := make(map[string][]byte, len(sources)*2)
	matched := make([]bool, len(sources))
	identities := make([][2]string, len(sources))
	for index, content := range sources {
		if content == nil {
			matched[index] = true // optional browser attachment omitted
			continue
		}
		digest := sha256.Sum256(content)
		identities[index] = [2]string{"sha256:" + fmt.Sprintf("%x", digest[:]), "fnv1a64:" + browserFNV(content)}
		for _, identity := range identities[index] {
			available[identity] = content
		}
	}
	attachments := make([]entry, 0, len(records))
	usedPaths := make(map[string]bool, len(records))
	for index, record := range records {
		if record == nil {
			return nil, fmt.Errorf("source %d must be an object", index+1)
		}
		source := document.Sources[index]
		identity := ""
		if source.Hash != nil {
			identity = strings.ToLower(source.Hash.Algo) + ":" + strings.ToLower(source.Hash.Value)
		}
		content, found := available[identity]
		if !found {
			record["path"] = json.RawMessage("null")
			record["retained"] = json.RawMessage("false")
			continue
		}
		var recordedSize *int64
		if value := record["bytes"]; len(value) > 0 {
			if err := json.Unmarshal(value, &recordedSize); err != nil {
				return nil, fmt.Errorf("source %q has invalid byte count", source.File)
			}
		}
		if recordedSize != nil && *recordedSize != int64(len(content)) {
			return nil, fmt.Errorf("source %q byte count does not match its verified bytes", source.File)
		}
		for candidate, ids := range identities {
			if identity == ids[0] || identity == ids[1] {
				matched[candidate] = true
			}
		}
		var requested, mime string
		_ = json.Unmarshal(record["path"], &requested)
		_ = json.Unmarshal(record["mime"], &mime)
		requested = strings.ReplaceAll(requested, `\`, "/")
		if !safeSourcePath.MatchString(requested) || unsafeWindowsName(path.Base(requested)) || usedPaths[strings.ToLower(requested)] {
			requested = generatedSourcePath(index, source.File, mime)
		}
		base := requested
		for suffix := 2; usedPaths[strings.ToLower(requested)]; suffix++ {
			ext := path.Ext(base)
			requested = strings.TrimSuffix(base, ext) + fmt.Sprintf("-%d", suffix) + ext
		}
		usedPaths[strings.ToLower(requested)] = true
		record["path"], _ = json.Marshal(requested)
		delete(record, "retained")
		name := directory + "/" + requested
		if err := budget.reserve(name, int64(len(content))); err != nil {
			return nil, err
		}
		attachments = append(attachments, entry{name, content})
	}
	for index, ok := range matched {
		if !ok {
			return nil, fmt.Errorf("source attachment %d does not match any recorded source hash", index+1)
		}
	}
	raw["sources"], err = json.Marshal(records)
	if err != nil {
		return nil, fmt.Errorf("encode bundle sources: %w", err)
	}
	packed, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode bundle palette: %w", err)
	}
	if err := budget.reserve(directory+"/palette.json", int64(len(packed))+1); err != nil {
		return nil, err
	}
	if len(preview) == 0 {
		preview, err = makePreview(document)
	} else {
		if int64(len(preview)) > budget.limit-budget.used {
			return nil, fmt.Errorf("preview exceeds the remaining archive size limit")
		}
		var configErr error
		config, configErr := png.DecodeConfig(bytes.NewReader(preview))
		if configErr != nil || config.Width < 1 || config.Height < 1 || config.Width > 4096 || config.Height > 4096 {
			return nil, fmt.Errorf("bundle preview must be a valid PNG no larger than 4096 pixels per side")
		}
		_, err = png.Decode(bytes.NewReader(preview))
	}
	if err != nil {
		return nil, fmt.Errorf("render bundle preview: %w", err)
	}
	if err := budget.reserve(directory+"/preview.png", int64(len(preview))); err != nil {
		return nil, err
	}
	entries := []entry{{directory + "/palette.json", append(packed, '\n')}}
	for _, format := range []struct{ name, kind string }{{"tokens.css", "css"}, {"plate.gd", "godot"}, {"palette.gpl", "gpl"}, {"notes.md", "notes"}} {
		formatted, formatErr := Format(packed, format.kind)
		if formatErr != nil {
			return nil, formatErr
		}
		name := directory + "/" + format.name
		if err := budget.reserve(name, int64(len(formatted))); err != nil {
			return nil, err
		}
		entries = append(entries, entry{name, []byte(formatted)})
	}
	entries = append(entries, entry{directory + "/preview.png", preview})
	entries = append(entries, attachments...)
	return entries, nil
}

func encodeEntries(entries []entry) ([]byte, error) {
	budget := newArchiveBudget(MaxArchiveBytes)
	for _, file := range entries {
		if err := budget.reserve(file.name, int64(len(file.content))); err != nil {
			return nil, err
		}
	}
	output := limitedArchiveWriter{limit: MaxArchiveBytes}
	// One allocation after validation avoids bytes.Buffer doubling its capacity
	// while writing a large, but permitted, vault.
	output.Grow(int(budget.used))
	archive := zip.NewWriter(&output)
	for _, file := range entries {
		header := &zip.FileHeader{Name: file.name, Method: zip.Store}
		header.SetModTime(time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC))
		header.SetMode(0o644)
		writer, createErr := archive.CreateHeader(header)
		if createErr != nil {
			return nil, fmt.Errorf("create bundle entry: %w", createErr)
		}
		if _, writeErr := writer.Write(file.content); writeErr != nil {
			return nil, fmt.Errorf("write bundle entry: %w", writeErr)
		}
	}
	if err := archive.Close(); err != nil {
		return nil, fmt.Errorf("finish bundle: %w", err)
	}
	return output.Bytes(), nil
}

func generatedSourcePath(index int, file, mime string) string {
	ext := "bin"
	stem := file
	if match := sourceExtension.FindStringSubmatch(file); len(match) == 2 {
		ext = strings.ToLower(match[1])
		stem = strings.TrimSuffix(file, match[0])
	} else if known := map[string]string{"image/png": "png", "image/jpeg": "jpg", "image/gif": "gif", "image/webp": "webp"}[mime]; known != "" {
		ext = known
	}
	return fmt.Sprintf("sources/%02d-%s.%s", index+1, vault.Slug(stem), ext)
}

func unsafeWindowsName(name string) bool {
	if strings.HasSuffix(name, ".") {
		return true
	}
	base := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" {
		return true
	}
	return len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9'
}

func bundleSlug(name string) string {
	slug := vault.Slug(name)
	if unsafeWindowsName(slug) {
		return "plate-" + slug
	}
	return slug
}

// The browser's labeled fnv1a64 fallback is two independent 32-bit streams,
// not the standard FNV-1a 64-bit algorithm. Keep the original bytes compatible.
func browserFNV(data []byte) string {
	h1, h2 := uint32(0x811c9dc5), uint32(0x01000193)
	for index, value := range data {
		h1 = (h1 ^ uint32(value)) * uint32(0x01000193)
		h2 = (h2 ^ (uint32(value) + uint32(index&0xff))) * uint32(0x85ebca6b)
	}
	return fmt.Sprintf("%08x%08x", h1, h2)
}
