package main

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/checker"
)

func TestCheckProductionSourceCapture(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		if os.Getenv("GOTLA_REQUIRE_TLC") == "1" {
			t.Fatal("required TLC_JAR missing")
		}
		t.Skip("set TLC_JAR to check source capture bootstrap")
	}
	jar, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	const sourcePath = "internal/sourcecapture/capture.go"
	source, err := os.ReadFile(filepath.Join(root, sourcePath))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		want checker.Status
	}{
		{"production", checker.Passed}, {"missing-unlock", checker.Deadlock},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := root
			if tc.want == checker.Deadlock {
				dir = t.TempDir()
				for _, name := range []string{"go.mod", "go.sum", sourcePath, "examples/bootstrap/sourcecapture/main.go"} {
					data, err := os.ReadFile(filepath.Join(root, name))
					if err != nil {
						t.Fatal(err)
					}
					if name == sourcePath {
						needle := []byte("c.mu.Unlock()")
						if bytes.Count(data, needle) != 1 {
							t.Fatal("production Unlock mutation is not unique")
						}
						data = bytes.Replace(data, needle, bytes.Repeat([]byte(" "), len(needle)), 1)
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
			code := runContext(t.Context(), []string{"check", "-out", out, "-tlc-jar", jar,
				"-timeout=30s", "./examples/bootstrap/sourcecapture"}, &stdout, &stderr)
			r := readResult(t, out)
			if code != tc.want.ExitCode() || r.Status != tc.want {
				t.Fatalf("exit=%d result=%+v stderr=%s", code, r, &stderr)
			}
			if len(r.TrustedCalls) != 0 || len(r.Command) == 0 || r.TLCVersion == "" || r.StateStats == "" {
				t.Fatal("missing actual checker evidence or hidden trust")
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
			if stats.Processes != 3 || stats.Mutexes != 1 || stats.WaitGroups != 1 || stats.Channels != 0 || stats.AbstractedPredicates != 0 {
				t.Fatalf("unexpected capture synchronization model: %+v", stats)
			}
			locks, unlocks := 0, 0
			for _, tr := range m.Transitions {
				if tr.SourcePosition.File != sourcePath {
					continue
				}
				for _, e := range tr.Effects {
					switch e.Kind {
					case behavior.Lock:
						locks++
					case behavior.Unlock:
						unlocks++
					}
				}
			}
			// Direct Unlock has normal and synchronization-error transitions per caller.
			wantUnlocks := 4
			if tc.want == checker.Deadlock {
				wantUnlocks = 0
			}
			if locks != 2 || unlocks != wantUnlocks {
				t.Fatalf("missing production effects: locks=%d unlocks=%d", locks, unlocks)
			}
			if tc.want == checker.Deadlock && len(r.SourceCandidates) == 0 {
				t.Fatal("missing source candidates")
			}
			t.Logf("model=%+v; %s; checker=%d ms", stats, r.StateStats, r.DurationMS)
		})
	}
	after, err := os.ReadFile(filepath.Join(root, sourcePath))
	if err != nil || !bytes.Equal(source, after) {
		t.Fatal("mutation changed working source", err)
	}
}
