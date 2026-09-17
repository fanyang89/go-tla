package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
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
	// Even this refused whole model must consume the real dependency's closed
	// global proof, rather than silently dropping initialization or trusting it.
	data, err := os.ReadFile(filepath.Join(out, "model.json"))
	if err != nil {
		t.Fatal(err)
	}
	var model behavior.Model
	if err := json.Unmarshal(data, &model); err != nil {
		t.Fatal(err)
	}
	closed := false
	for _, channel := range model.Channels {
		if channel.InitiallyClosed && channel.Source.Package == "context" && channel.Source.File == "context/context.go" && channel.Source.Line > 0 {
			closed = true
		}
	}
	proofs := 0
	for _, d := range r.Diagnostics {
		if d.Code == "closed-global-init" && strings.Contains(d.Message, "context.closedchan") && d.File == "context/context.go" && d.Line > 0 {
			proofs++
		}
	}
	if !closed || proofs != 2 {
		t.Fatal("actual context.closedchan initialization proof/state missing")
	}
	constructors, floatConstructors := 0, 0
	for _, d := range r.Diagnostics {
		if d.Code == "constant-data-call" && strings.Contains(d.Message, "go/constant.newFloat") && d.File == "go/constant/value.go" && d.Line > 0 {
			floatConstructors++
		}
		if d.Code == "constant-data-call" && strings.Contains(d.Message, "encoding/base64.NewEncoding") && d.File == "encoding/base64/base64.go" && d.Line > 0 {
			constructors++
		}
	}
	if floatConstructors != 1 {
		t.Fatal("actual zero Float constructor proof missing")
	}
	if constructors != 2 {
		t.Fatal("actual base64 literal-input constructor proofs missing")
	}
	for file, capacity := range map[string]int{
		"golang.org/x/tools/go/packages/packages.go":     20,
		"golang.org/x/tools/go/buildutil/allpackages.go": 20,
		"golang.org/x/tools/go/loader/util.go":           10,
	} {
		foundState, foundProof := false, false
		for _, channel := range model.Channels {
			if channel.Source.File == file && channel.Source.Line > 0 && channel.Capacity == capacity && !channel.InitiallyClosed {
				foundState = true
			}
		}
		for _, d := range r.Diagnostics {
			if d.Code == "open-global-init" && d.File == file && d.Line > 0 {
				foundProof = true
			}
		}
		if !foundState || !foundProof {
			t.Fatalf("actual global I/O semaphore proof/state missing: %s", file)
		}
	}
	arrayProofs := 0
	for _, d := range r.Diagnostics {
		if d.Code == "proved-loop" && d.File == "internal/artifact/directory.go" && d.Line > 0 && strings.Contains(d.Message, "scalar array range bound: 5 iterations") {
			arrayProofs++
		}
	}
	if arrayProofs != 2 {
		t.Fatal("actual artifact file-table loop proofs missing")
	}
	statisticsProof := false
	for _, d := range r.Diagnostics {
		if d.Code == "finite-data-loop" && d.File == "internal/behavior/statistics.go" && d.Line > 0 && strings.Contains(d.Message, ").Statistics;") {
			statisticsProof = true
		}
	}
	if !statisticsProof {
		t.Fatal("actual Statistics value-copy proof missing")
	}
	// Require a diagnostic in our own entry point, not merely a dependency load error.
	for _, d := range r.Diagnostics {
		if d.Severity == "error" && strings.HasPrefix(d.File, "cmd/gotla/") && d.Line > 0 {
			return
		}
	}
	t.Fatal("missing source-located diagnostic in gotla's own CLI")
}
