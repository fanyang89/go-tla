package main

import (
	"bytes"
	"crypto/sha256"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/checker"
)

// Remove the real production method's cleanup, not a hand-written lock skeleton.
// Preserve newlines and offsets so source evidence remains directly comparable.
func withoutWriterUnlock(t *testing.T, source []byte) []byte {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "writer.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	mutant := bytes.Clone(source)
	matches := 0
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "Write" || fn.Recv == nil {
			continue
		}
		for _, stmt := range fn.Body.List {
			d, ok := stmt.(*ast.DeferStmt)
			if !ok {
				continue
			}
			call, ok := d.Call.Fun.(*ast.SelectorExpr)
			if !ok || call.Sel.Name != "Unlock" {
				continue
			}
			receiver, ok := call.X.(*ast.SelectorExpr)
			if !ok || receiver.Sel.Name != "mu" {
				continue
			}
			matches++
			for i := fset.Position(d.Pos()).Offset; i < fset.Position(d.End()).Offset; i++ {
				if mutant[i] != '\n' && mutant[i] != '\r' {
					mutant[i] = ' '
				}
			}
		}
	}
	if matches != 1 {
		t.Fatalf("expected exactly one production unlock defer, got %d", matches)
	}
	return mutant
}

func TestCheckProductionBoundedLog(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		if os.Getenv("GOTLA_REQUIRE_TLC") == "1" {
			t.Fatal("required TLC_JAR missing")
		}
		t.Skip("set TLC_JAR to check production bootstrap")
	}
	jar, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	const sourcePath = "internal/boundedlog/writer.go"
	const harnessPath = "examples/bootstrap/boundedlog/main.go"
	source, err := os.ReadFile(filepath.Join(root, sourcePath))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		want checker.Status
	}{{"production", checker.Passed}, {"missing-unlock", checker.Deadlock}, {"blocking-cancel", checker.Deadlock}} {
		t.Run(tc.name, func(t *testing.T) {
			dir := root
			mutated := tc.name == "missing-unlock"
			pattern := "./examples/bootstrap/boundedlog"
			if tc.name == "blocking-cancel" {
				pattern += "/blocked"
			}
			if mutated {
				dir = t.TempDir()
				for _, name := range []string{"go.mod", "go.sum", harnessPath, sourcePath} {
					data, err := os.ReadFile(filepath.Join(root, name))
					if err != nil {
						t.Fatal(err)
					}
					if name == sourcePath {
						data = withoutWriterUnlock(t, data)
						t.Logf("production SHA256=%x; mutant SHA256=%x", sha256.Sum256(source), sha256.Sum256(data))
					}
					path := filepath.Join(dir, name)
					if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, data, 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			t.Chdir(dir)
			out := t.TempDir()
			var stdout, stderr bytes.Buffer
			code := runContext(t.Context(), []string{"check", "-out", out, "-tlc-jar", jar, "-timeout=30s", pattern}, &stdout, &stderr)
			r := readResult(t, out)
			if code != tc.want.ExitCode() || r.Status != tc.want {
				t.Fatalf("exit=%d result=%+v stderr=%s", code, r, &stderr)
			}
			if r.StateStats == "" || r.TLCVersion == "" || len(r.Command) == 0 || len(r.TrustedCalls) != 0 {
				t.Fatalf("missing TLC evidence or hidden trust: %+v", r)
			}
			data, err := os.ReadFile(filepath.Join(out, "model.json"))
			if err != nil {
				t.Fatal(err)
			}
			m, err := behavior.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			stats := m.Statistics()
			if stats.Processes != 3 || stats.Mutexes != 1 || stats.WaitGroups != 1 || stats.Channels != 1 || stats.AbstractedPredicates == 0 {
				t.Fatalf("empty/wrong model or hidden data abstraction: %+v", stats)
			}
			locks, unlocks := 0, 0
			for _, tr := range m.Transitions {
				if tr.SourcePosition.File != sourcePath {
					continue
				}
				for _, effect := range tr.Effects {
					switch effect.Kind {
					case behavior.Lock:
						locks++
					case behavior.Unlock:
						unlocks++
					}
				}
			}
			if locks != 2 || !mutated && unlocks == 0 || mutated && unlocks != 0 {
				t.Fatalf("missing production lock effects: locks=%d unlocks=%d", locks, unlocks)
			}
			fieldCalls := 0
			for _, d := range r.Diagnostics {
				if d.Code == "resolved-field-call" && d.File == sourcePath {
					fieldCalls++
				}
			}
			if fieldCalls < 4 {
				t.Fatalf("production writer/cancel dispatch not proved for both invocations: %d", fieldCalls)
			}
			if tc.want != checker.Passed && len(r.SourceCandidates) == 0 {
				t.Fatal("missing counterexample source candidates")
			}
			t.Logf("model=%+v; %s; checker=%d ms", stats, r.StateStats, r.DurationMS)
		})
	}
	after, err := os.ReadFile(filepath.Join(root, sourcePath))
	if err != nil || !bytes.Equal(source, after) {
		t.Fatal("mutation changed production source", err)
	}
}
