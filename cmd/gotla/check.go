package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/fanmi/go-tla/internal/analysis"
	"github.com/fanmi/go-tla/internal/artifact"
	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/checker"
	"github.com/fanmi/go-tla/internal/diagnostic"
	"github.com/fanmi/go-tla/internal/lowering"
)

type fileEvidence struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type sourceCandidate struct {
	Process    string            `json:"process"`
	Location   string            `json:"location"`
	Transition string            `json:"transition"`
	TraceState string            `json:"traceState"`
	Source     behavior.Position `json:"source"`
}

type checkResult struct {
	checker.Report
	CLIExitCode      *int                    `json:"exitCode,omitempty"`
	SchemaVersion    int                     `json:"schemaVersion"`
	RunID            string                  `json:"runId"`
	StartedAt        time.Time               `json:"startedAt"`
	FinishedAt       time.Time               `json:"finishedAt,omitzero"`
	SourceDirectory  string                  `json:"sourceDirectory"`
	Patterns         []string                `json:"patterns"`
	TrustedCalls     []string                `json:"trustedCalls,omitempty"`
	Config           checker.Config          `json:"config"`
	GoVersion        string                  `json:"goVersion"`
	ToolVersion      string                  `json:"toolVersion"`
	ToolRevision     string                  `json:"toolRevision,omitempty"`
	ToolModified     string                  `json:"toolModified,omitempty"`
	AnalysisOutcome  diagnostic.Outcome      `json:"analysisOutcome,omitempty"`
	Assumptions      []string                `json:"assumptions,omitempty"`
	Diagnostics      []diagnostic.Diagnostic `json:"diagnostics,omitempty"`
	Statistics       *behavior.Statistics    `json:"modelStatistics,omitempty"`
	Artifacts        map[string]fileEvidence `json:"artifacts,omitempty"`
	SourceCandidates []sourceCandidate       `json:"sourceCandidates,omitempty"`
}

func runCheck(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("out", ".", "output directory (one writer per directory)")
	trust := fs.String("trust-call", "", "comma-separated total, side-effect-free call contracts")
	cfg := checker.DefaultConfig()
	fs.StringVar(&cfg.JAR, "tlc-jar", "", "TLC JAR; overrides TLC_JAR and ./tla2tools.jar")
	fs.StringVar(&cfg.Java, "java", cfg.Java, "Java executable path or name")
	fs.DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "checker deadline including Java startup (not Go analysis)")
	fs.IntVar(&cfg.MemoryMiB, "memory-mib", cfg.MemoryMiB, "maximum JVM heap in MiB (not total process memory)")
	fs.IntVar(&cfg.Workers, "workers", cfg.Workers, "TLC worker count")
	fs.IntVar(&cfg.MaxLogMiB, "max-log-mib", cfg.MaxLogMiB, "combined Java/TLC log limit in MiB")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "a Go package pattern is required")
		return 2
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 2
	}
	cfg.JAR = checker.FindJAR(cfg.JAR)
	if cfg.JAR != "" {
		path, err := filepath.Abs(cfg.JAR)
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		cfg.JAR = path
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	dir, err := artifact.Begin(*out)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	defer dir.Close()
	result := checkResult{
		Report:        checker.Report{Status: checker.Running, Reason: "Analysis/check in progress; this is not a verification result"},
		SchemaVersion: 1, RunID: rand.Text(), StartedAt: time.Now().UTC(),
		SourceDirectory: cwd, Patterns: fs.Args(), Config: cfg, GoVersion: runtime.Version(), ToolVersion: "development",
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" {
			result.ToolVersion = info.Main.Version
		}
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				result.ToolRevision = setting.Value
			case "vcs.modified":
				result.ToolModified = setting.Value
			}
		}
	}
	opts := lowering.Options{}
	if *trust != "" {
		opts.TrustedCalls = strings.Split(*trust, ",")
	}
	result.TrustedCalls = opts.TrustedCalls
	if err := writeJSON(dir, "result.json", result); err != nil {
		fmt.Fprintln(stderr, "error: cannot record current run:", err)
		return 1
	}
	finish := func(status checker.Status, reason string) int {
		result.Status, result.Reason = status, reason
		result.FinishedAt = time.Now().UTC()
		result.Artifacts = map[string]fileEvidence{}
		for _, name := range []string{"model.json", "model.tla", "model.cfg", "tlc.log"} {
			path := filepath.Join(dir.Path, name)
			sum, err := hashArtifact(path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				result.Status, result.Reason = checker.ToolError, "cannot fingerprint output: "+err.Error()
				continue
			}
			result.Artifacts[name] = fileEvidence{Path: path, SHA256: sum}
		}
		result.CLIExitCode = new(result.Status.ExitCode())
		if err := writeJSON(dir, "result.json", result); err != nil {
			fmt.Fprintln(stderr, "error: cannot save final result (any remaining running result is incomplete):", err)
			return 1
		}
		fmt.Fprintf(stdout, "Check: %s\n%s\nResult: %s\n", result.Status, result.Reason, filepath.Join(dir.Path, "result.json"))
		if log, ok := result.Artifacts["tlc.log"]; ok {
			fmt.Fprintln(stdout, "TLC log:", log.Path)
		}
		if result.AnalysisOutcome != "" {
			fmt.Fprintf(stdout, "Analysis: %s; see result.json/model.json for assumptions and contracts.\n", result.AnalysisOutcome)
		}
		if len(result.SourceCandidates) > 0 {
			fmt.Fprintln(stdout, "Source candidates (not a decoded counterexample):")
			for i, c := range result.SourceCandidates {
				if i == 12 {
					fmt.Fprintln(stdout, "  ... see result.json for more candidates")
					break
				}
				fmt.Fprintf(stdout, "  %s: %s at %s:%d:%d (%s)\n", c.Process, c.Transition, c.Source.File, c.Source.Line, c.Source.Column, c.TraceState)
			}
		}
		return result.Status.ExitCode()
	}
	m, err := analysis.AnalyzeContext(ctx, "", fs.Args(), opts)
	if ctx.Err() != nil {
		return finish(checker.Incomplete, "Analysis cancelled; verification did not complete")
	}
	if err != nil {
		return finish(checker.AnalysisError, err.Error())
	}
	printDiagnostics(stderr, m)
	result.AnalysisOutcome, result.Assumptions, result.Diagnostics = m.Outcome, m.Assumptions, m.Diagnostics
	result.Statistics = new(m.Statistics())
	analysis.PrintStatistics(stdout, m)
	spec, configText, err := emitModel(dir, m)
	if err != nil {
		return finish(checker.ToolError, "Model output failed: "+err.Error())
	}
	if m.HasErrors() {
		return finish(checker.Unsupported, "Unsupported analysis; no executable TLA+ emitted and TLC not run")
	}
	result.Report = checker.Run(ctx, cfg, spec, configText, filepath.Join(dir.Path, "tlc.log"))
	result.SourceCandidates = candidates(m, result.Report)
	return finish(result.Status, result.Reason)
}

func hashArtifact(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func candidates(m *behavior.Model, report checker.Report) []sourceCandidate {
	if report.Status != checker.Deadlock && report.Status != checker.SynchronizationError {
		return nil
	}
	var out []sourceCandidate
	states := []struct {
		name string
		pc   map[string]string
	}{{"last trace state", report.LastPC}}
	if report.Status == checker.SynchronizationError {
		states = append(states, struct {
			name string
			pc   map[string]string
		}{"previous trace state", report.PreviousPC})
	}
	for _, state := range states {
		for _, tr := range m.Transitions {
			if state.pc[tr.Process] == tr.Source {
				out = append(out, sourceCandidate{Process: tr.Process, Location: tr.Source, Transition: tr.ID, TraceState: state.name, Source: tr.SourcePosition})
			}
		}
	}
	return out
}
