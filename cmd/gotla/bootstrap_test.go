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
	identProof := false
	for _, d := range r.Diagnostics {
		if d.Code == "constant-data-call" && d.File == "go/doc/exports.go" && d.Line > 0 && strings.Contains(d.Message, "go/ast.NewIdent;") {
			identProof = true
		}
	}
	if !identProof {
		t.Fatal("actual ast.NewIdent nil-interface storage proof missing")
	}
	versionProofs, operationModels := 0, 0
	for _, d := range r.Diagnostics {
		if d.File == "go/types/version.go" && d.Line > 0 {
			if d.Code == "constant-data-call" && strings.Contains(d.Message, "go/types.asGoVersion;") {
				versionProofs++
			}
			if d.Code == "modeled-data-operation" && strings.Contains(d.Message, "internal/bytealg.IndexByteString") {
				operationModels++
			}
		}
	}
	if versionProofs < 10 || operationModels != versionProofs {
		t.Fatal("version initializer proof/model evidence missing")
	}
	byteSearchAssumptions := 0
	for _, assumption := range model.Assumptions {
		if strings.HasPrefix(assumption, "Standard Go internal/bytealg.IndexByteString") {
			byteSearchAssumptions++
		}
	}
	if byteSearchAssumptions != 1 {
		t.Fatal("byte-search model assumption missing or duplicated")
	}
	for _, d := range r.Diagnostics {
		if d.Code == "initializer" && (strings.Contains(d.Message, "(in github.com/fanmi/go-tla/internal/checker.init)") || strings.Contains(d.Message, "(in github.com/fanmi/go-tla/internal/tla.init)")) {
			t.Fatalf("project lexical initialization regressed: %+v", d)
		}
	}
	edgeProofs, typeProofs, literalTypeProofs := 0, 0, 0
	for _, d := range r.Diagnostics {
		if d.Code != "constant-data-call" || d.Line == 0 {
			continue
		}
		if d.File == "golang.org/x/tools/go/ast/edge/edge.go" && strings.Contains(d.Message, "edge.info[") {
			edgeProofs++
		}
		if strings.Contains(d.Message, "reflect.TypeFor[") {
			typeProofs++
		}
		if strings.Contains(d.Message, "reflect.rtypeOf") && (d.File == "reflect/map.go" || d.File == "reflect/value.go") {
			literalTypeProofs++
		}
	}
	if edgeProofs != 104 || typeProofs != 5 {
		t.Fatalf("reflection metadata proofs: edge=%d type=%d", edgeProofs, typeProofs)
	}
	reflectionAssumptions := 0
	for _, assumption := range model.Assumptions {
		if strings.HasPrefix(assumption, "Standard Go reflect.") {
			reflectionAssumptions++
		}
	}
	if literalTypeProofs != 3 {
		t.Fatalf("actual reflect literal metadata proofs: %d", literalTypeProofs)
	}
	if reflectionAssumptions != 3 {
		t.Fatal("reflection model assumptions missing or duplicated")
	}
	versionFormat, ownFormat, formatAssumptions := false, false, 0
	for _, d := range r.Diagnostics {
		if d.Code == "scalar-format-call" && d.Line > 0 {
			versionFormat = versionFormat || d.File == "go/types/version.go"
			ownFormat = ownFormat || strings.HasPrefix(d.File, "internal/lowering/")
		}
	}
	for _, assumption := range model.Assumptions {
		if strings.HasPrefix(assumption, "Standard Go fmt.Sprintf") {
			formatAssumptions++
		}
	}
	if !versionFormat || !ownFormat || formatAssumptions != 1 {
		t.Fatal("actual scalar formatting proofs/assumption missing")
	}
	exit := false
	for _, transition := range model.Transitions {
		if transition.SourcePosition.File != "cmd/gotla/main.go" {
			continue
		}
		for _, effect := range transition.Effects {
			if effect.Kind == behavior.Exit {
				exit = true
			}
		}
	}
	if !exit {
		t.Fatal("actual CLI program-exit instruction missing")
	}
	// Require a diagnostic in our own entry point, not merely a dependency load error.
	for _, d := range r.Diagnostics {
		if d.Severity == "error" && strings.HasPrefix(d.File, "cmd/gotla/") && d.Line > 0 {
			return
		}
	}
	t.Fatal("missing source-located diagnostic in gotla's own CLI")
}
