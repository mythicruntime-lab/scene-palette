package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"scene-palette/internal/bundle"
)

func runBundle(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("bundle", flag.ContinueOnError)
	flags.SetOutput(stderr)
	output := flags.String("out", "", "ZIP file to write (default: <palette-name>.zip; - for stdout)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() < 1 {
		return fmt.Errorf("bundle requires a palette.json file and optional original source images")
	}
	data, err := os.ReadFile(flags.Arg(0))
	if err != nil {
		return fmt.Errorf("read bundle palette: %w", err)
	}
	name, err := bundle.Filename(data)
	if err != nil {
		return err
	}
	if *output == "" {
		*output = name
	}
	sources := make([][]byte, 0, flags.NArg()-1)
	for _, input := range flags.Args()[1:] {
		content, err := os.ReadFile(input)
		if err != nil {
			return fmt.Errorf("read bundle source %s: %w", input, err)
		}
		sources = append(sources, content)
	}
	archive, err := bundle.Build(data, sources, nil)
	if err != nil {
		return err
	}
	if *output == "-" {
		_, err := stdout.Write(archive)
		return err
	}
	if err := writeOutputFile(*output, archive, flags.Args()); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Wrote %s (%d bytes).\n", filepath.Clean(*output), len(archive))
	return nil
}

// writeOutputFile serves JSON and ZIP exports. Resolve aliases before replacing
// any output so an export cannot destroy its palette or original source.
func writeOutputFile(output string, data []byte, inputs []string) error {
	outputPath := filepath.Clean(output)
	outputAbsolute, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("resolve output: %w", err)
	}
	outputInfo, _ := os.Stat(outputPath)
	for _, input := range inputs {
		inputAbsolute, err := filepath.Abs(input)
		if err != nil {
			return fmt.Errorf("resolve input: %w", err)
		}
		inputInfo, _ := os.Stat(input)
		if strings.EqualFold(inputAbsolute, outputAbsolute) || (outputInfo != nil && inputInfo != nil && os.SameFile(outputInfo, inputInfo)) {
			return fmt.Errorf("output must not overwrite input %s", input)
		}
	}
	file, err := os.CreateTemp(filepath.Dir(outputPath), ".scene-palette-export-*.tmp")
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if _, err = file.Write(data); err != nil {
		_ = file.Close()
		return fmt.Errorf("write output: %w", err)
	}
	if err = file.Close(); err != nil {
		return fmt.Errorf("close output: %w", err)
	}
	if err = os.Rename(temporary, outputPath); err != nil {
		return fmt.Errorf("save output: %w", err)
	}
	return nil
}
