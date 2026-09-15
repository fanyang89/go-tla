// gotla extracts a finite concurrent behavioral model from a Go main package.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/fanmi/go-tla/internal/analysis"
	"github.com/fanmi/go-tla/internal/artifact"
	"github.com/fanmi/go-tla/internal/checker"
	"github.com/fanmi/go-tla/internal/lowering"
)

func run(args []string, stdout, stderr io.Writer) int {
	return runContext(context.Background(), args, stdout, stderr)
}

func runContext(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "check" {
		return runCheck(ctx, args[1:], stdout, stderr)
	}
	if len(args) == 0 || (args[0] != "analyze" && args[0] != "inspect") {
		fmt.Fprintln(stderr, "usage: gotla {analyze|inspect|check} [-out DIR] [-trust-call package.Function,...] ./package")
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
	var dir *artifact.Directory
	if args[0] == "analyze" {
		var err error
		dir, err = artifact.Begin(*out)
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		defer dir.Close()
	}
	m, err := analysis.AnalyzeContext(ctx, "", fs.Args(), opts)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	printDiagnostics(stderr, m)
	if args[0] == "inspect" {
		analysis.Inspect(stdout, m)
		if m.HasErrors() {
			return 1
		}
		return 0
	}
	if _, _, err := emitModel(dir, m); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if m.HasErrors() {
		fmt.Fprintln(stderr, "error: unsupported analysis; emitted diagnostic model.json only")
		return 1
	}
	fmt.Fprintf(stdout, "wrote %s/{model.tla,model.cfg,model.json} (%s)\n", *out, m.Outcome)
	analysis.PrintStatistics(stdout, m)
	printTLCHint(stdout, *out)
	return 0
}

// printTLCHint emits a POSIX-shell command without changing the caller's directory.
func printTLCHint(w io.Writer, out string) {
	jar := checker.FindJAR("")
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

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := runContext(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}
