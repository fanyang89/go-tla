package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fanmi/go-tla/internal/checker"
)

// This is a fail-closed self-analysis boundary test, not a proof of gotla.
// Revisiting this expectation requires a supported self-model and real TLC evidence.
func TestCheckSelfAnalysisBoundary(t *testing.T) {
	out := t.TempDir()
	seedOldArtifacts(t, out)
	var stdout, stderr bytes.Buffer
	code := runContext(t.Context(), []string{
		"check", "-out", out,
		"-tlc-jar", filepath.Join(out, "must-not-be-used.jar"),
		".", // Load the actual CLI and its dependencies, not a rewritten skeleton.
	}, &stdout, &stderr)
	r := readResult(t, out)
	if code != checker.Unsupported.ExitCode() || r.Status != checker.Unsupported || r.AnalysisOutcome != "unsupported" {
		t.Fatalf("self-analysis boundary changed: exit=%d status=%s outcome=%s; inspect support before updating this test", code, r.Status, r.AnalysisOutcome)
	}
	if r.RunID == "old" || len(r.TrustedCalls) != 0 {
		t.Fatal("self-analysis reused stale evidence or introduced trusted calls")
	}
	if len(r.Command) != 0 || r.ExitCode != nil || r.TLCVersion != "" {
		t.Fatal("unsupported self-analysis must not run TLC")
	}
	if len(r.Artifacts) != 1 || r.Artifacts["model.json"].SHA256 == "" {
		t.Fatal("unsupported self-analysis must preserve only the diagnostic model")
	}
	for _, name := range []string{"model.tla", "model.cfg", "tlc.log", ".gotla.lock"} {
		if _, err := os.Stat(filepath.Join(out, name)); !os.IsNotExist(err) {
			t.Fatalf("unsupported self-analysis left executable/stale artifacts or a lock: %s (%v)", name, err)
		}
	}
	// Require a diagnostic in our own entry point, not merely a dependency load error.
	for _, d := range r.Diagnostics {
		if d.Severity == "error" && strings.HasPrefix(d.File, "cmd/gotla/") && d.Line > 0 {
			return
		}
	}
	t.Fatal("missing source-located diagnostic in gotla's own CLI")
}
