package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/checker"
)

func TestCheckWorkerPoolComponents(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		if os.Getenv("GOTLA_REQUIRE_TLC") == "1" {
			t.Fatal("required TLC_JAR missing")
		}
		t.Skip("set TLC_JAR to check component harnesses")
	}
	for _, c := range []struct {
		variant                string
		want                   checker.Status
		locations, transitions int
	}{{"good", checker.Passed, 22, 21}, {"bad", checker.Deadlock, 18, 15}} {
		t.Run(c.variant, func(t *testing.T) {
			out := t.TempDir()
			var stdout, stderr bytes.Buffer
			code := run([]string{"check", "-out", out, "-tlc-jar", jar, "-timeout=30s",
				"../../examples/components/workerpool/" + c.variant + "/harness"}, &stdout, &stderr)
			r := readResult(t, out)
			if code != c.want.ExitCode() || r.Status != c.want {
				t.Fatalf("exit=%d result=%+v stderr=%s", code, r, &stderr)
			}
			if r.StateStats == "" || r.TLCVersion == "" || len(r.TrustedCalls) != 0 {
				t.Fatalf("missing evidence or hidden trusted contract: %+v", r)
			}
			data, err := os.ReadFile(filepath.Join(out, "model.json"))
			if err != nil {
				t.Fatal(err)
			}
			m, err := behavior.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if len(m.Processes) != 3 || len(m.Channels) != 2 || len(m.WaitGroups) != 1 || len(m.AbstractedPredicates) != 0 {
				t.Fatalf("component topology/predicates changed: %+v", m.Statistics())
			}
			stats := m.Statistics()
			if stats.Locations != c.locations || stats.Transitions != c.transitions || stats.Mutexes != 0 {
				t.Fatalf("component size baseline changed: %+v", stats)
			}
			if c.want == checker.Deadlock && len(r.SourceCandidates) == 0 {
				t.Fatal("counterexample lacks source candidates")
			}
		})
	}
}
