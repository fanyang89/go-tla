package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/fanmi/go-tla/internal/checker"
)

// Re-entry through a captured writer cell has no current field-origin proof.
// Unsupported must remain distinct from a TLC pass or a proved deadlock.
func TestCheckReentrantLoggerBoundary(t *testing.T) {
	out := t.TempDir()
	seedOldArtifacts(t, out)
	var stdout, stderr bytes.Buffer
	code := runContext(t.Context(), []string{"check", "-out", out,
		"-tlc-jar", filepath.Join(out, "must-not-be-used.jar"),
		"../../examples/bootstrap/boundedlog/reentrant"}, &stdout, &stderr)
	r := readResult(t, out)
	if code != checker.Unsupported.ExitCode() || r.Status != checker.Unsupported || r.AnalysisOutcome != "unsupported" {
		t.Fatalf("unproved re-entry changed boundary: exit=%d result=%+v stderr=%s", code, r, &stderr)
	}
	if r.RunID == "old" || len(r.TrustedCalls) != 0 || len(r.Command) != 0 || r.ExitCode != nil || r.TLCVersion != "" {
		t.Fatal("stale evidence, trust or checker execution for unsupported re-entry")
	}
	if len(r.Artifacts) != 1 || r.Artifacts["model.json"].SHA256 == "" {
		t.Fatal("expected only a fresh diagnostic model")
	}
	for _, name := range []string{"model.tla", "model.cfg", "tlc.log", ".gotla.lock"} {
		if _, err := os.Stat(filepath.Join(out, name)); !os.IsNotExist(err) {
			t.Fatalf("stale/executable artifact after refusal: %s (%v)", name, err)
		}
	}
	initializationRefused, productionIdentityRefused := false, false
	for _, d := range r.Diagnostics {
		if d.Severity != "error" {
			continue
		}
		if d.Code == "sync-initialization-order" {
			initializationRefused = true
		}
		if d.Code == "sync-identity" && d.Line > 0 && d.File == "internal/boundedlog/writer.go" {
			productionIdentityRefused = true
		}
	}
	if !initializationRefused || !productionIdentityRefused {
		t.Fatal("missing capture-initialization proof refusal or production source location")
	}
}
