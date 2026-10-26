package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"scene-palette/internal/bundle"
	"scene-palette/internal/extract"
	"scene-palette/internal/forge"
	"scene-palette/internal/palette"
	"scene-palette/internal/vault"
)

// The frontend remains the reference implementation. Embedding it lets the Go
// bridge serve the exact same files without translating or replacing its JS.
//
//go:embed index.html index.legacy.html probe-file-access.html README.md vendor/*.js
var frontendFiles embed.FS

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "scene-palette:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return runServe([]string{"-open"}, stdout, stderr)
	}
	switch args[0] {
	case "serve":
		return runServe(args[1:], stdout, stderr)
	case "rebuild-index":
		return runRebuildIndex(args[1:], stdout, stderr)
	case "forge":
		return runForge(args[1:], stdout, stderr)
	case "bundle":
		return runBundle(args[1:], stdout, stderr)
	case "check":
		return runCheck(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		writeUsage(stdout)
		return nil
	default:
		writeUsage(stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func writeUsage(w io.Writer) {
	fmt.Fprintln(w, "Scene Palette Go bridge")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  scene-palette  (start the local frontend and open the browser)")
	fmt.Fprintln(w, "  scene-palette serve [-addr 127.0.0.1:8765] [-open]")
	fmt.Fprintln(w, "  scene-palette rebuild-index [-out index.json] <vault-folder>")
	fmt.Fprintln(w, "  scene-palette forge [-out palette.json | -bundle plate.zip] [options] <image> [image ...]")
	fmt.Fprintln(w, "  scene-palette bundle [-out plate.zip] <palette.json> [source-image ...]")
	fmt.Fprintln(w, "  scene-palette check [-json] [-allow-unsealed] <palette.json>")
}

func runServe(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	address := flags.String("addr", "127.0.0.1:8765", "address for the local frontend")
	open := flags.Bool("open", false, "open the frontend in the default browser")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("serve accepts flags only")
	}

	addressWasSet := false
	flags.Visit(func(value *flag.Flag) {
		if value.Name == "addr" {
			addressWasSet = true
		}
	})
	listener, err := listenFrontend(*address, !addressWasSet, stderr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", *address, err)
	}
	server := &http.Server{
		Handler:           frontendHandler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	url := "http://" + listener.Addr().String() + "/"
	fmt.Fprintln(stdout, "Scene Palette frontend:", url)
	fmt.Fprintln(stdout, "Press Ctrl+C to stop.")

	serverErr := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		serverErr <- err
	}()
	if *open {
		if err := openBrowser(url); err != nil {
			fmt.Fprintln(stderr, "Could not open the browser:", err)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shut down server: %w", err)
		}
		return <-serverErr
	}
}

// listenFrontend preserves the documented default port when it is available,
// but a desktop launcher should not fail merely because another local tool is
// already using it. An explicitly supplied -addr remains strict.
func listenFrontend(address string, allowFallback bool, stderr io.Writer) (net.Listener, error) {
	listener, err := net.Listen("tcp", address)
	if err == nil || !allowFallback || !isAddressInUse(err) {
		return listener, err
	}

	host, _, splitErr := net.SplitHostPort(address)
	if splitErr != nil {
		return nil, err
	}
	fallbackAddress := net.JoinHostPort(host, "0")
	listener, fallbackErr := net.Listen("tcp", fallbackAddress)
	if fallbackErr != nil {
		return nil, fmt.Errorf("default address unavailable and fallback failed: %w", fallbackErr)
	}
	fmt.Fprintf(stderr, "%s is already in use; using %s instead.\n", address, listener.Addr().String())
	return listener, nil
}

func isAddressInUse(err error) bool {
	if errors.Is(err, syscall.EADDRINUSE) {
		return true
	}
	// Windows exposes WSAEADDRINUSE (10048) through syscall.Errno, but it does
	// not consistently compare equal to syscall.EADDRINUSE across Go builds.
	var errno syscall.Errno
	return errors.As(err, &errno) && errno == syscall.Errno(10048)
}

func openBrowser(url string) error {
	name, args, err := browserCommand(runtime.GOOS, url)
	if err != nil {
		return err
	}
	if err := exec.Command(name, args...).Start(); err != nil {
		return fmt.Errorf("launch %s: %w", name, err)
	}
	return nil
}

func browserCommand(goos, url string) (string, []string, error) {
	switch goos {
	case "windows":
		return "rundll32.exe", []string{"url.dll,FileProtocolHandler", url}, nil
	case "darwin":
		return "open", []string{url}, nil
	case "linux":
		return "xdg-open", []string{url}, nil
	default:
		return "", nil, fmt.Errorf("opening a browser is unsupported on %s", goos)
	}
}

func frontendHandler() http.Handler {
	files, err := fs.Sub(frontendFiles, ".")
	if err != nil {
		panic(err)
	}
	static := http.FileServer(http.FS(files))
	api := newAPIHandler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			api.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/healthz" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "ok\n")
			return
		}
		static.ServeHTTP(w, r)
	})
}

func runRebuildIndex(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("rebuild-index", flag.ContinueOnError)
	flags.SetOutput(stderr)
	output := flags.String("out", "", "index file to write (default: <vault-folder>/index.json)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return fmt.Errorf("rebuild-index requires exactly one vault folder")
	}
	root := filepath.Clean(flags.Arg(0))
	index, skipped, err := vault.Rebuild(root, *output)
	for _, skip := range skipped {
		fmt.Fprintln(stderr, "skipped:", skip)
	}
	if err != nil {
		return err
	}
	written := *output
	if written == "" {
		written = filepath.Join(root, "index.json")
	}
	fmt.Fprintf(stdout, "Wrote %s with %d plate(s).\n", written, index.Count)
	return nil
}

type checkPlate struct {
	Name     string            `json:"name"`
	Polarity string            `json:"polarity"`
	Roles    map[string]string `json:"roles"`
	Found    int               `json:"found"`
}

type checkReport struct {
	Kind          string                      `json:"kind"`
	File          string                      `json:"file"`
	Status        string                      `json:"status"`
	ContractValid bool                        `json:"contractValid"`
	AgentEligible bool                        `json:"agentEligible"`
	Plate         *checkPlate                 `json:"plate,omitempty"`
	Review        *palette.ReviewVerification `json:"review,omitempty"`
	Error         string                      `json:"error,omitempty"`
}

func runCheck(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	jsonOutput := flags.Bool("json", false, "write a machine-readable result")
	allowUnsealed := flags.Bool("allow-unsealed", false, "return success for a valid but unsealed plate")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return fmt.Errorf("check requires exactly one palette.json file")
	}

	path := filepath.Clean(flags.Arg(0))
	report := checkReport{
		Kind: "scene-palette-check", File: path, Status: "invalid",
	}
	data, err := os.ReadFile(path)
	if err != nil {
		report.Error = err.Error()
		if *jsonOutput {
			_ = writeCheckReport(stdout, report)
		}
		return fmt.Errorf("read palette %s: %w", path, err)
	}
	document, err := palette.Parse(data)
	if err != nil {
		report.Error = err.Error()
		if *jsonOutput {
			_ = writeCheckReport(stdout, report)
		}
		return fmt.Errorf("invalid palette %s: %w", path, err)
	}
	verification, err := document.VerifyReview()
	if err != nil {
		return fmt.Errorf("verify palette %s: %w", path, err)
	}

	roles := make(map[string]string, len(document.Core))
	for _, token := range document.Core {
		roles[token.Role()] = token.Hex
	}
	report.ContractValid = true
	report.AgentEligible = verification.AgentEligible
	report.Status = "blocked"
	if verification.AgentEligible {
		report.Status = "ok"
	}
	report.Plate = &checkPlate{
		Name: document.Name, Polarity: document.Polarity,
		Roles: roles, Found: len(document.Found),
	}
	report.Review = &verification

	if *jsonOutput {
		if err := writeCheckReport(stdout, report); err != nil {
			return err
		}
	} else {
		focus := "all review checks clear"
		if count := verification.Assessment.AttentionCount; count > 0 {
			noun, verb := "checks", "need"
			if count == 1 {
				noun, verb = "check", "needs"
			}
			focus = fmt.Sprintf("%d review %s %s attention", count, noun, verb)
		}
		coverage := "not recorded"
		switch verification.Scope {
		case palette.ReviewScopePaletteAndSourceIdentities:
			coverage = "palette + source identities"
		case palette.ReviewScopePaletteContent:
			coverage = "legacy palette content only"
		}
		history := ""
		if count := len(verification.RecordedHistory); count > 0 {
			noun := "seals"
			if count == 1 {
				noun = "seal"
			}
			history = fmt.Sprintf(" Prior seals: %d %s recorded, not reverified.", count, noun)
		}
		if verification.AgentEligible {
			fmt.Fprintf(stdout, "OK %s — %q is human-sealed and agent-eligible (%s:%s; scope %s; %s).%s\n",
				path, document.Name, verification.Computed.Algo, verification.Computed.Value, coverage, focus, history)
		} else {
			fmt.Fprintf(stdout, "BLOCKED %s — %q is not agent-eligible: %s. Review focus: %s. Seal scope: %s.%s\n",
				path, document.Name, verification.Reason, focus, coverage, history)
		}
	}

	if !verification.AgentEligible && !*allowUnsealed {
		return fmt.Errorf("palette is not agent-eligible: %s", verification.Reason)
	}
	return nil
}

func writeCheckReport(w io.Writer, report checkReport) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return fmt.Errorf("write check report: %w", err)
	}
	return nil
}

func runForge(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("forge", flag.ContinueOnError)
	flags.SetOutput(stderr)
	output := flags.String("out", "-", "palette file to write (- for stdout)")
	bundleOutput := flags.String("bundle", "", "write a ZIP bundle with original source images instead of JSON")
	name := flags.String("name", "", "plate name (default: first image filename)")
	budget := flags.Int("budget", 140000, "sample budget per image")
	chromaMin := flags.Float64("chroma-min", 0.085, "minimum signal chroma")
	foundInks := flags.Int("found", 6, "number of found inks (0-10)")
	polarity := flags.String("polarity", "auto", "plate polarity: auto, dark, or light")
	variant := flags.Int("variant", 1, "one-based plate variant")
	colorPolicy := flags.String("color-policy", "readable-ui", "accent policy: readable-ui or source-hues")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() == 0 {
		return fmt.Errorf("forge requires at least one image")
	}
	if *variant < 1 {
		return fmt.Errorf("forge variant must be positive")
	}
	if *bundleOutput != "" && *output != "-" {
		return fmt.Errorf("forge accepts either -out or -bundle, not both")
	}

	createdAt := time.Now().UTC().Truncate(time.Millisecond).Format("2006-01-02T15:04:05.000Z")
	snapshot, err := forge.Build(flags.Args(), forge.Options{
		ColorPolicy: extract.ColorPolicy(*colorPolicy),
		Name:        *name,
		CreatedAt:   createdAt,
		Budget:      *budget,
		ChromaMin:   *chromaMin,
		FoundInks:   *foundInks,
		Polarity:    extract.Polarity(*polarity),
		Variant:     *variant - 1,
	})
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("encode forged palette: %w", err)
	}
	data = append(data, '\n')
	if *bundleOutput != "" {
		sources := make([][]byte, 0, flags.NArg())
		for _, input := range flags.Args() {
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
		if *bundleOutput == "-" {
			_, err = stdout.Write(archive)
			return err
		}
		if err := writeOutputFile(*bundleOutput, archive, flags.Args()); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Wrote %s (%s, variant %d/%d).\n", *bundleOutput, snapshot.Name, snapshot.Params.Variant+1, snapshot.Params.VariantCount)
		return nil
	}
	if *output == "-" {
		if _, err := stdout.Write(data); err != nil {
			return fmt.Errorf("write forged palette: %w", err)
		}
		return nil
	}

	outputPath := filepath.Clean(*output)
	if err := writeOutputFile(outputPath, data, flags.Args()); err != nil {
		return fmt.Errorf("write forged palette %s: %w", outputPath, err)
	}
	fmt.Fprintf(stdout, "Wrote %s (%s, variant %d/%d).\n", outputPath, snapshot.Name, snapshot.Params.Variant+1, snapshot.Params.VariantCount)
	return nil
}
