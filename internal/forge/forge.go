// Package forge composes Scene Palette's headless ingestion, extraction, plate
// assembly, and durable snapshot stages.
package forge

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"scene-palette/internal/extract"
	"scene-palette/internal/palette"
)

const (
	maxStills   = 8
	maxVariants = 6
)

var nameSeparatorPattern = regexp.MustCompile(`[^a-zA-Z0-9]+`)

type Options struct {
	Name      string
	CreatedAt string
	Budget    int
	ChromaMin float64
	FoundInks int
	Polarity  extract.Polarity
	Variant   int // zero-based; the CLI presents this as one-based
}

func DefaultOptions() Options {
	return Options{
		Budget:    140000,
		ChromaMin: 0.085,
		FoundInks: 6,
		Polarity:  extract.PolarityAuto,
		Variant:   0,
	}
}

// Build reads original image bytes, records their hashes and decode metadata,
// pools the decoded pixels, and returns one complete durable palette. CreatedAt
// remains explicit so this deterministic pipeline does not own the clock.
func Build(paths []string, options Options) (palette.Snapshot, error) {
	if err := validate(paths, options); err != nil {
		return palette.Snapshot{}, err
	}

	histograms := make([]extract.Histogram, 0, len(paths))
	sources := make([]palette.SnapshotSource, 0, len(paths))
	for _, path := range paths {
		decoded, source, err := loadImage(path)
		if err != nil {
			return palette.Snapshot{}, err
		}
		histograms = append(histograms, extract.BuildHistogram(decoded, options.Budget, options.ChromaMin))
		sources = append(sources, source)
	}

	pooled := extract.Pool(histograms)
	structure := extract.BuildStructure(pooled, options.Polarity)
	accents := extract.BuildAccentStage(extract.SignalCandidates(pooled), structure, maxVariants)
	assembly := extract.BuildPlateAssembly(structure, accents, options.FoundInks)
	if options.Variant >= len(assembly.Variants) {
		return palette.Snapshot{}, fmt.Errorf(
			"requested variant %d, but this plate has %d", options.Variant+1, len(assembly.Variants),
		)
	}

	name := options.Name
	if strings.TrimSpace(name) == "" {
		name = defaultPlateName(paths[0])
	}
	return palette.BuildSnapshot(palette.SnapshotInput{
		Name:         name,
		CreatedAt:    options.CreatedAt,
		Structure:    structure,
		Variant:      assembly.Variants[options.Variant],
		VariantCount: len(assembly.Variants),
		Params: palette.SnapshotParamsInput{
			Budget:    options.Budget,
			ChromaMin: options.ChromaMin,
			FoundInks: options.FoundInks,
		},
		Sources: sources,
	})
}

func validate(paths []string, options Options) error {
	if len(paths) == 0 {
		return fmt.Errorf("forge requires at least one image")
	}
	if len(paths) > maxStills {
		return fmt.Errorf("forge accepts at most %d images", maxStills)
	}
	if strings.TrimSpace(options.CreatedAt) == "" {
		return fmt.Errorf("forge createdAt is required")
	}
	if _, err := time.Parse(time.RFC3339Nano, options.CreatedAt); err != nil {
		return fmt.Errorf("forge createdAt must be RFC3339: %w", err)
	}
	if options.Budget <= 0 {
		return fmt.Errorf("forge budget must be positive")
	}
	if options.ChromaMin < 0 || options.ChromaMin > 1 {
		return fmt.Errorf("forge chroma minimum must be between 0 and 1")
	}
	if options.FoundInks < 0 || options.FoundInks > 10 {
		return fmt.Errorf("forge found inks must be between 0 and 10")
	}
	if options.Polarity != extract.PolarityAuto && options.Polarity != extract.PolarityDark && options.Polarity != extract.PolarityLight {
		return fmt.Errorf("forge polarity must be auto, dark, or light")
	}
	if options.Variant < 0 {
		return fmt.Errorf("forge variant must be positive")
	}
	return nil
}

func loadImage(path string) (image.Image, palette.SnapshotSource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, palette.SnapshotSource{}, fmt.Errorf("read image %s: %w", path, err)
	}
	decoded, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, palette.SnapshotSource{}, fmt.Errorf("decode image %s: %w", path, err)
	}
	bounds := decoded.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width < 1 || height < 1 {
		return nil, palette.SnapshotSource{}, fmt.Errorf("decode image %s: empty dimensions", path)
	}
	mime := map[string]string{
		"gif":  "image/gif",
		"jpeg": "image/jpeg",
		"png":  "image/png",
	}[format]
	if mime == "" {
		mime = "application/octet-stream"
	}
	size := int64(len(data))
	hash := sha256.Sum256(data)
	return decoded, palette.SnapshotSource{
		File:    filepath.Base(path),
		MIME:    &mime,
		Bytes:   &size,
		Hash:    &palette.Hash{Algo: "sha256", Value: fmt.Sprintf("%x", hash[:])},
		Natural: [2]int{width, height},
		Working: [2]int{width, height},
		Transforms: []map[string]any{{
			"op":  "decode",
			"via": "go-image/" + format,
		}},
	}, nil
}

func defaultPlateName(path string) string {
	base := filepath.Base(path)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	words := strings.Fields(nameSeparatorPattern.ReplaceAllString(stem, " "))
	for index, word := range words {
		runes := []rune(word)
		if len(runes) > 0 {
			runes[0] = unicode.ToUpper(runes[0])
			words[index] = string(runes)
		}
	}
	if len(words) == 0 {
		return "Plate"
	}
	return strings.Join(words, " ")
}
