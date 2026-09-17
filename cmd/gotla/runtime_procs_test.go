package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/checker"
)

func TestCheckRuntimeProcsTLC(t *testing.T) {
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
	for _, n := range []int{0, 1, 2} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			dir := t.TempDir()
			source := `package main;import("runtime";"sync");var _ sync.Mutex;var c=make(chan int,runtime.GOMAXPROCS(0));func main(){c<-1;c<-2;<-c;<-c}`
			for name, text := range map[string]string{"go.mod": "module fixture\n\ngo 1.26\n", "main.go": source} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0600); err != nil {
					t.Fatal(err)
				}
			}
			t.Chdir(dir)
			out := filepath.Join(dir, "out")
			var stdout, stderr bytes.Buffer
			code := run([]string{"check", "-runtime-procs", strconv.Itoa(n), "-out", out, "-tlc-jar", jar, "."}, &stdout, &stderr)
			r := readResult(t, out)
			want := checker.Passed
			if n == 0 {
				want = checker.Unsupported
			} else if n == 1 {
				want = checker.Deadlock
			}
			if code != want.ExitCode() || r.Status != want || r.RuntimeProcs != n {
				t.Fatalf("profile=%d code=%d status=%s: %s", n, code, r.Status, &stderr)
			}
			data, err := os.ReadFile(filepath.Join(out, "model.json"))
			if err != nil {
				t.Fatal(err)
			}
			var m behavior.Model
			if err := json.Unmarshal(data, &m); err != nil {
				t.Fatal(err)
			}
			if n != 0 {
				if len(m.Metadata.Options["startup.GOMAXPROCS"]) != 1 || m.Metadata.Options["startup.GOMAXPROCS"][0] != strconv.Itoa(n) || r.JARSHA256 == "" || r.TLCVersion == "" || len(r.Command) == 0 {
					t.Fatal("profile/checker evidence missing")
				}
			} else if len(r.Command) != 0 {
				t.Fatal("unbound profile ran TLC")
			}
		})
	}
}

func TestCheckSelfRuntimeProcsBoundary(t *testing.T) {
	out := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := runContext(t.Context(), []string{"check", "-runtime-procs", "2", "-out", out, "-tlc-jar", filepath.Join(out, "unused.jar"), "."}, &stdout, &stderr)
	r := readResult(t, out)
	if code != 5 || r.Status != checker.Unsupported || r.RuntimeProcs != 2 || len(r.Command) != 0 || len(r.TrustedCalls) != 0 || len(r.Artifacts) != 1 {
		t.Fatalf("self profile boundary: code=%d status=%s %s", code, r.Status, &stderr)
	}
	data, err := os.ReadFile(filepath.Join(out, "model.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m behavior.Model
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, c := range m.Channels {
		if strings.Contains(c.ID, "cpuLimit") && (c.Source.Package == "golang.org/x/tools/go/ssa" || c.Source.Package == "golang.org/x/tools/go/packages") {
			if c.Capacity != 2 {
				t.Fatal("wrong runtime capacity")
			}
			count++
		}
	}
	if count != 2 {
		t.Fatalf("actual CPU semaphores missing: %d", count)
	}
	queries := 0
	for _, d := range r.Diagnostics {
		if d.Code == "runtime-procs-query" && d.Line > 0 {
			queries++
		}
	}
	if queries != 2 {
		t.Fatal("actual runtime query proofs missing")
	}
}

func TestRuntimeProcsBadOptions(t *testing.T) {
	for _, command := range []string{"inspect", "analyze", "check"} {
		for _, flags := range [][]string{{"-runtime-procs=-1"}, {"-runtime-procs=1025"}, {"-runtime-procs=2", "-trust-call=runtime.GOMAXPROCS"}} {
			var stdout, stderr bytes.Buffer
			args := append([]string{command}, flags...)
			args = append(args, ".")
			if code := run(args, &stdout, &stderr); code != 2 {
				t.Fatalf("bad profile options accepted: %v: %d", args, code)
			}
		}
	}
}
