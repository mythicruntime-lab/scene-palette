package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunBundleWritesZIPAndProtectsInput(t *testing.T) {
	input := filepath.Join("testdata", "vault", "night-bay", "palette.json")
	root := t.TempDir()
	output := filepath.Join(root, "plate.zip")
	var stdout, stderr bytes.Buffer
	if err := runBundle([]string{"-out", output, input}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil || len(archive.File) != 6 {
		t.Fatalf("invalid CLI bundle: %v", err)
	}
	if !strings.Contains(stdout.String(), "Wrote ") {
		t.Fatal("missing output receipt")
	}
	stdout.Reset()
	if err := runBundle([]string{"-out", "-", input}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stdout.Bytes(), data) {
		t.Fatal("stdout output differs from file bundle")
	}
	copyInput := filepath.Join(root, "palette.json")
	original, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(copyInput, original, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runBundle([]string{"-out", copyInput, copyInput}, &stdout, &stderr); err == nil || !strings.Contains(err.Error(), "overwrite input") {
		t.Fatalf("overwriting palette permitted: %v", err)
	}
	unchanged, _ := os.ReadFile(copyInput)
	if !bytes.Equal(unchanged, original) {
		t.Fatal("input was modified")
	}
	badSource := filepath.Join(root, "unrelated.png")
	if err := os.WriteFile(badSource, []byte("unrelated"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runBundle([]string{"-out", output, copyInput, badSource}, &stdout, &stderr); err == nil {
		t.Fatal("unmatched source accepted")
	}
}
