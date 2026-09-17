package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/checker"
)

func readResult(t *testing.T, out string) checkResult {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(out, "result.json"))
	if err != nil {
		t.Fatal(err)
	}
	var r checkResult
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	if r.SchemaVersion != 1 || r.RunID == "" || r.FinishedAt.IsZero() || r.StartedAt.IsZero() ||
		r.CLIExitCode == nil || *r.CLIExitCode != r.Status.ExitCode() {
		t.Fatalf("incomplete result metadata: %+v", r)
	}
	for _, file := range r.Artifacts {
		sum, err := hashArtifact(file.Path)
		if err != nil || sum != file.SHA256 {
			t.Fatalf("artifact evidence does not match: %+v; %v", file, err)
		}
	}
	if file, ok := r.Artifacts["model.json"]; ok {
		data, err := os.ReadFile(file.Path)
		if err != nil {
			t.Fatal(err)
		}
		var m behavior.Model
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatal(err)
		}
		if r.ModelSchemaVersion != m.SchemaVersion || r.ModelSemantics != m.Semantics || r.TerminationPolicy != m.Termination ||
			r.ToolVersion != m.Metadata.Version || r.GoVersion != m.Metadata.Toolchain || r.ToolRevision != m.Metadata.Revision || r.ToolModified != m.Metadata.Modified {
			t.Fatal("checker envelope and IR contract/provenance disagree")
		}
	}
	return r
}

func seedOldArtifacts(t *testing.T, out string) {
	t.Helper()
	for _, name := range []string{"result.json", "tlc.log", "model.json", "model.tla", "model.cfg", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(out, name), []byte(`{"status":"passed","runId":"old"}`), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCheckFailureResultsAndStaleArtifacts(t *testing.T) {
	for _, c := range []struct {
		name, pattern string
		want          checker.Status
	}{
		{"load-failure", "../../examples/does-not-exist", checker.AnalysisError},
		{"unsupported", "../../examples/unknown", checker.Unsupported},
		{"missing-tool", "../../examples/unbuffered", checker.ToolError},
	} {
		t.Run(c.name, func(t *testing.T) {
			out := t.TempDir()
			seedOldArtifacts(t, out)
			var stdout, stderr bytes.Buffer
			code := run([]string{"check", "-out", out, "-tlc-jar", filepath.Join(out, "missing.jar"), c.pattern}, &stdout, &stderr)
			r := readResult(t, out)
			if r.Status != c.want || code != c.want.ExitCode() || r.RunID == "old" {
				t.Fatalf("exit=%d result=%+v stderr=%s", code, r, &stderr)
			}
			if _, err := os.Stat(filepath.Join(out, "notes.txt")); err != nil {
				t.Fatal("unrelated file removed")
			}
			if c.want == checker.Unsupported || c.want == checker.AnalysisError {
				for _, name := range []string{"model.tla", "model.cfg", "tlc.log"} {
					if _, err := os.Stat(filepath.Join(out, name)); !os.IsNotExist(err) {
						t.Fatalf("stale file survives failed analysis: %s", name)
					}
				}
			}
			if _, err := os.Stat(filepath.Join(out, ".gotla.lock")); !os.IsNotExist(err) {
				t.Fatal("completed command leaked output lock")
			}
		})
	}
}

func TestAnalyzeLoadFailureInvalidatesCheckResult(t *testing.T) {
	out := t.TempDir()
	seedOldArtifacts(t, out)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"analyze", "-out", out, "../../examples/does-not-exist"}, &stdout, &stderr); code != 1 {
		t.Fatal("load failure exit", code)
	}
	for _, name := range []string{"result.json", "model.json", "model.tla", "model.cfg", "tlc.log"} {
		if _, err := os.Stat(filepath.Join(out, name)); !os.IsNotExist(err) {
			t.Fatalf("old check artifacts survived analyze failure: %s", name)
		}
	}
}

func TestCheckCancelledAndInvalidOptions(t *testing.T) {
	out := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var stdout, stderr bytes.Buffer
	if code := runContext(ctx, []string{"check", "-out", out, "../../examples/unbuffered"}, &stdout, &stderr); code != 6 {
		t.Fatalf("cancellation exit=%d: %s", code, &stderr)
	}
	if r := readResult(t, out); r.Status != checker.Incomplete || len(r.Artifacts) != 0 {
		t.Fatalf("cancelled analysis produced a result: %+v", r)
	}
	before, _ := os.ReadFile(filepath.Join(out, "result.json"))
	for _, flag := range []string{"-timeout=0", "-workers=0", "-memory-mib=0", "-max-log-mib=0", "-java=", "-runtime-procs=-1", "-runtime-procs=1025"} {
		if code := run([]string{"check", "-out", out, flag, "../../examples/unbuffered"}, &stdout, &stderr); code != 2 {
			t.Fatalf("bad option accepted: %s, %d", flag, code)
		}
	}
	after, _ := os.ReadFile(filepath.Join(out, "result.json"))
	if !bytes.Equal(before, after) {
		t.Fatal("argument error changed output")
	}
}

func TestCheckTrustedContractMetadata(t *testing.T) {
	out := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := run([]string{"check", "-out", out, "-tlc-jar", filepath.Join(out, "missing.jar"),
		"-trust-call", "strings.HasPrefix", "../../examples/unknown"}, &stdout, &stderr)
	r := readResult(t, out)
	if code != 1 || r.Status != checker.ToolError || r.AnalysisOutcome != "conservatively-abstracted" ||
		len(r.TrustedCalls) != 1 || r.TrustedCalls[0] != "strings.HasPrefix" || len(r.Assumptions) == 0 || r.Statistics == nil {
		t.Fatalf("extraction and checker outcomes conflated or contracts lost: %+v; %s", r, &stderr)
	}
}

func TestCheckOutputLockPreservesCurrentWriter(t *testing.T) {
	out := t.TempDir()
	seedOldArtifacts(t, out)
	if err := os.Mkdir(filepath.Join(out, ".gotla.lock"), 0700); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"check", "-out", out, "../../examples/unbuffered"}, &stdout, &stderr); code != 1 {
		t.Fatal("concurrent check accepted")
	}
	data, _ := os.ReadFile(filepath.Join(out, "result.json"))
	if !strings.Contains(string(data), `"runId":"old"`) || !strings.Contains(stderr.String(), "output lock") {
		t.Fatal("blocked writer modified existing result or lacked lock diagnostic")
	}
}

func TestCheckTLCIntegration(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		if os.Getenv("GOTLA_REQUIRE_TLC") == "1" {
			t.Fatal("required TLC_JAR missing")
		}
		t.Skip("set TLC_JAR to run actual CLI/TLC integration")
	}
	jar, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, source string
		want         checker.Status
	}{
		{"pass", `package main;func main(){ch:=make(chan int);go func(){ch<-1}();<-ch}`, checker.Passed},
		{"deadlock", `package main;func main(){ch:=make(chan int);ch<-1}`, checker.Deadlock},
		{"synchronization-error", `package main;func main(){var ch chan int;close(ch)}`, checker.SynchronizationError},
		{"timeout", `package main;func main(){}`, checker.Incomplete},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, text := range map[string]string{"go.mod": "module cli-fixture\n\ngo 1.26.2\n", "main.go": c.source} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0600); err != nil {
					t.Fatal(err)
				}
			}
			t.Chdir(dir)
			out := filepath.Join(dir, "output with 'quotes'")
			args := []string{"check", "-out", out, "-tlc-jar", jar}
			if c.name == "timeout" {
				args = append(args, "-timeout=1ns")
			}
			args = append(args, ".")
			var stdout, stderr bytes.Buffer
			code := run(args, &stdout, &stderr)
			r := readResult(t, out)
			if r.Status != c.want || code != c.want.ExitCode() {
				t.Fatalf("code=%d result=%+v stderr=%s", code, r, &stderr)
			}
			if c.want != checker.Incomplete && (r.TLCVersion == "" || r.JARSHA256 == "" || r.ExitCode == nil) {
				t.Fatalf("missing checker provenance: %+v", r)
			}
			if c.want == checker.Deadlock || c.want == checker.SynchronizationError {
				if len(r.SourceCandidates) == 0 || !strings.Contains(stdout.String(), "Source candidates") {
					t.Fatalf("missing source summary: %s", &stdout)
				}
			}
		})
	}
}
