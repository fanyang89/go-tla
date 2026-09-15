package tests

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/tla"
)

// TLC_JAR must be an explicitly provisioned official tla2tools.jar. Tests do not download executables.
func checkTLC(t *testing.T, m *behavior.Model, want string) {
	t.Helper()
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("set TLC_JAR to an official tla2tools.jar to run model checking")
	}
	jar, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	spec, cfg, err := tla.Generate(m)
	if err != nil {
		t.Fatalf("generation failed: %v; diagnostics: %+v", err, m.Diagnostics)
	}
	dir := t.TempDir()
	for name, text := range map[string]string{"model.tla": spec, "model.cfg": cfg} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", "-Xmx512m", "-XX:+UseParallelGC", "-cp", jar, "tlc2.TLC", "-workers", "1", "model.tla")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("TLC timeout: %v", ctx.Err())
	}
	if !strings.Contains(string(output), want) {
		t.Fatalf("TLC missing %q (exit %v):\n%s", want, err, output)
	}
	if want == "No error has been found" && err != nil {
		t.Fatalf("TLC failed: %v\n%s", err, output)
	}
	if want != "No error has been found" && err == nil {
		t.Fatal("expected TLC counterexample exit")
	}
	for _, line := range strings.Split(string(output), "\n") {
		if strings.Contains(line, "distinct states found") || strings.Contains(line, "Error:") {
			t.Log(line)
		}
	}
}
func TestTLCExamples(t *testing.T) {
	if os.Getenv("TLC_JAR") == "" {
		t.Skip("TLC_JAR not configured")
	}
	for _, name := range []string{"unbuffered", "buffered", "deadlock", "select", "mutex", "waitgroup", "sequential", "unknown"} {
		t.Run(name, func(t *testing.T) {
			want := "No error has been found"
			if name == "deadlock" || name == "unknown" {
				want = "Error: Deadlock reached"
			}
			checkTLC(t, example(t, name), want)
		})
	}
}
func TestTLCBlockingAndErrors(t *testing.T) {
	if os.Getenv("TLC_JAR") == "" {
		t.Skip("TLC_JAR not configured")
	}
	cases := []struct{ name, source, want string }{
		{"nil-receive", `package main;func main(){var ch chan int;<-ch}`, "Error: Deadlock reached"},
		{"nil-send", `package main;func main(){var ch chan int;ch<-1}`, "Error: Deadlock reached"},
		{"nil-close", `package main;func main(){var ch chan int;close(ch)}`, "Invariant NoSynchronizationErrors is violated"},
		{"closed-send", `package main;func main(){ch:=make(chan int);close(ch);ch<-1}`, "Invariant NoSynchronizationErrors is violated"},
		{"double-close", `package main;func main(){ch:=make(chan int);close(ch);close(ch)}`, "Invariant NoSynchronizationErrors is violated"},
		{"closed-drain", `package main;func main(){ch:=make(chan int,1);ch<-1;close(ch);<-ch;<-ch}`, "No error has been found"},
		{"buffer-full", `package main;func main(){ch:=make(chan int,1);ch<-1;ch<-2}`, "Error: Deadlock reached"},
		{"main-exit", `package main;func main(){ch:=make(chan int);go func(){<-ch}()}`, "No error has been found"},
		{"select-ready-no-default", `package main;func main(){ch:=make(chan int,1);ch<-1;select{case <-ch:default:var nilch chan int;<-nilch}}`, "No error has been found"},
		{"select-nil-default", `package main;func main(){var ch chan int;select{case <-ch:default:}}`, "No error has been found"},
		{"select-exact-dispatch", `package main;func main(){a:=make(chan int,1);b:=make(chan int);a<-1;select{case <-a:case <-b:var nilch chan int;<-nilch}}`, "No error has been found"},
		{"select-closed-send", `package main;func main(){ch:=make(chan int);close(ch);select{case ch<-1:default:}}`, "Invariant NoSynchronizationErrors is violated"},
		{"select-rendezvous", `package main;func main(){ch:=make(chan int);go func(){select{case ch<-1:}}();select{case <-ch:}}`, "No error has been found"},
		{"select-self-no-rendezvous", `package main;func main(){ch:=make(chan int);select{case ch<-1:case <-ch:}}`, "Error: Deadlock reached"},
		{"mutex-lock-twice", `package main;import "sync";func main(){var mu sync.Mutex;mu.Lock();mu.Lock()}`, "Error: Deadlock reached"},
		{"mutex-invalid-unlock", `package main;import "sync";func main(){var mu sync.Mutex;mu.Unlock()}`, "Invariant NoSynchronizationErrors is violated"},
		{"mutex-cross-process-unlock", `package main;import "sync";func main(){var mu sync.Mutex;ch:=make(chan int);mu.Lock();go func(){mu.Unlock();ch<-1}();<-ch}`, "No error has been found"},
		{"waitgroup-negative", `package main;import "sync";func main(){var wg sync.WaitGroup;wg.Done()}`, "Invariant NoSynchronizationErrors is violated"},
		{"waitgroup-blocked", `package main;import "sync";func main(){var wg sync.WaitGroup;wg.Add(1);wg.Wait()}`, "Error: Deadlock reached"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { checkTLC(t, fromSource(t, c.source), c.want) })
	}
}
