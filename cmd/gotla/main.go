// gotla extracts a finite concurrent behavioral model from a Go main package.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/fanmi/go-tla/internal/analysis"
	"github.com/fanmi/go-tla/internal/lowering"
	"github.com/fanmi/go-tla/internal/tla"
)

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || (args[0] != "analyze" && args[0] != "inspect") {
		fmt.Fprintln(stderr, "usage: gotla {analyze|inspect} [-out DIR] [-trust-call package.Function,...] ./package")
		return 2
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("out", ".", "output directory")
	trust := fs.String("trust-call", "", "comma-separated total, side-effect-free call contracts")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "a Go package pattern is required")
		return 2
	}
	opts := lowering.Options{}
	if *trust != "" {
		opts.TrustedCalls = strings.Split(*trust, ",")
	}
	m, err := analysis.Analyze("", fs.Args(), opts)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	for _, d := range m.Diagnostics {
		fmt.Fprintf(stderr, "%s: %s:%d [%s] %s\n", d.Severity, d.File, d.Line, d.Code, d.Message)
	}
	if args[0] == "inspect" {
		analysis.Inspect(stdout, m)
		if m.HasErrors() {
			return 1
		}
		return 0
	}
	if err = os.MkdirAll(*out, 0755); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	// Remove stale executable artifacts before writing a new diagnostic/IR result.
	for _, name := range []string{"model.tla", "model.cfg"} {
		if err = os.Remove(filepath.Join(*out, name)); err != nil && !os.IsNotExist(err) {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err == nil {
		err = os.WriteFile(filepath.Join(*out, "model.json"), append(data, '\n'), 0644)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if m.HasErrors() {
		fmt.Fprintln(stderr, "error: unsupported analysis; emitted diagnostic model.json only")
		return 1
	}
	spec, cfg, err := tla.Generate(m)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	for _, f := range []struct{ name, content string }{{"model.tla", spec}, {"model.cfg", cfg}} {
		if err = os.WriteFile(filepath.Join(*out, f.name), []byte(f.content), 0644); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	fmt.Fprintf(stdout, "wrote %s/{model.tla,model.cfg,model.json} (%s)\n", *out, m.Outcome)
	printTLCHint(stdout, *out)
	return 0
}

// printTLCHint emits a POSIX-shell command without changing the caller's directory.
func printTLCHint(w io.Writer, out string) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		if info, err := os.Stat("tla2tools.jar"); err == nil && info.Mode().IsRegular() {
			jar = "tla2tools.jar"
		}
	}
	dir, err := filepath.Abs(out)
	if err != nil {
		fmt.Fprintln(w, "Run TLC from the output directory: java -cp /absolute/path/tla2tools.jar tlc2.TLC -workers 1 model.tla")
		return
	}
	classpath := `"${TLC_JAR:?Set TLC_JAR to the absolute path of tla2tools.jar}"`
	if jar != "" {
		if path, err := filepath.Abs(jar); err == nil {
			classpath = shellQuote(path)
		}
	} else {
		fmt.Fprintln(w, "Set TLC_JAR to the absolute path of tla2tools.jar (https://github.com/tlaplus/tlaplus/releases).")
	}
	fmt.Fprintf(w, "Run TLC:\n  (cd %s && java -cp %s tlc2.TLC -workers 1 model.tla)\n", shellQuote(dir), classpath)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
