package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/checker"
)

func checkComponent(t *testing.T, path string, want checker.Status) *behavior.Model {
	t.Helper()
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		if os.Getenv("GOTLA_REQUIRE_TLC") == "1" {
			t.Fatal("required TLC_JAR missing")
		}
		t.Skip("set TLC_JAR to check component harnesses")
	}
	out := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := run([]string{"check", "-out", out, "-tlc-jar", jar, "-timeout=30s",
		"../../examples/components/" + path + "/harness"}, &stdout, &stderr)
	r := readResult(t, out)
	if code != want.ExitCode() || r.Status != want {
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
	if want != checker.Passed && len(r.SourceCandidates) == 0 {
		t.Fatal("counterexample lacks source candidates")
	}
	t.Logf("model=%+v; %s; checker=%d ms", m.Statistics(), r.StateStats, r.DurationMS)
	return m
}

func TestCheckWorkerPoolComponents(t *testing.T) {
	for _, c := range []struct {
		variant                string
		want                   checker.Status
		locations, transitions int
	}{{"good", checker.Passed, 22, 21}, {"bad", checker.Deadlock, 18, 15}} {
		t.Run(c.variant, func(t *testing.T) {
			m := checkComponent(t, "workerpool/"+c.variant, c.want)
			stats := m.Statistics()
			if stats.Locations != c.locations || stats.Transitions != c.transitions || stats.Mutexes != 0 {
				t.Fatalf("component size baseline changed: %+v", stats)
			}
		})
	}
}

func TestCheckPipelineComponents(t *testing.T) {
	for _, c := range []struct {
		variant                string
		want                   checker.Status
		locations, transitions int
	}{
		{"good", checker.Passed, 21, 22}, {"missingclose", checker.Deadlock, 20, 21}, {"earlyclose", checker.SynchronizationError, 21, 22},
	} {
		t.Run(c.variant, func(t *testing.T) {
			m := checkComponent(t, "pipeline/"+c.variant, c.want)
			stats := m.Statistics()
			if stats.Locations != c.locations || stats.Transitions != c.transitions || stats.Mutexes != 0 {
				t.Fatalf("pipeline size baseline changed: %+v", stats)
			}
			if profile := m.Metadata.Options["profile"]; len(profile) != 1 || profile[0] != "finite-state-static-identity" {
				t.Fatal("receive cycle profile missing")
			}
		})
	}
}
