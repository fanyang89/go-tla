package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/fanmi/go-tla/internal/analysis"
	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/diagnostic"
	"github.com/fanmi/go-tla/internal/lowering"
	"github.com/fanmi/go-tla/internal/testutil"
	"github.com/fanmi/go-tla/internal/tla"
)

func fromSource(t *testing.T, src string, trust ...string) *behavior.Model {
	t.Helper()
	m, err := lowering.Lower(testutil.Load(t, src), lowering.Options{TrustedCalls: trust})
	if err != nil {
		t.Fatal(err)
	}
	return m
}
func example(t *testing.T, name string) *behavior.Model {
	t.Helper()
	opts := lowering.Options{}
	if name == "unknown" {
		opts.TrustedCalls = []string{"strings.HasPrefix"}
	}
	m, err := analysis.Analyze("..", []string{"./examples/" + name}, opts)
	if err != nil {
		t.Fatal(err)
	}
	return m
}
func TestExamplesAndSnapshots(t *testing.T) {
	for _, name := range []string{"unbuffered", "buffered", "deadlock", "select", "mutex", "waitgroup", "sequential", "unknown"} {
		t.Run(name, func(t *testing.T) {
			m := example(t, name)
			if m.HasErrors() {
				t.Fatalf("unsupported: %+v", m.Diagnostics)
			}
			spec, _, err := tla.Generate(m)
			if err != nil {
				t.Fatal(err)
			}
			if name == "unknown" && (m.Outcome != diagnostic.Abstracted || len(m.AbstractedPredicates) == 0) {
				t.Fatal("unknown predicate was not abstracted")
			}
			if name == "unbuffered" || name == "sequential" {
				if len(m.Transitions) != 5 {
					t.Fatalf("sequential regions not collapsed: %d transitions", len(m.Transitions))
				}
			}
			if name == "unbuffered" || name == "select" {
				data, err := json.MarshalIndent(m, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				snapshot(t, name+".json", append(data, '\n'))
				snapshot(t, name+".tla", []byte(spec))
			}
		})
	}
}
func snapshot(t *testing.T, name string, data []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if os.Getenv("UPDATE_SNAPSHOTS") == "1" {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != string(data) {
		t.Fatalf("snapshot mismatch %s (UPDATE_SNAPSHOTS=1 go test ./tests -run TestExamplesAndSnapshots)", path)
	}
}

func TestUnsupportedIsNeverExecutable(t *testing.T) {
	cases := []struct{ name, source string }{
		{"dynamic-capacity", `package main;func size()int{return 2};func main(){ch:=make(chan int,size());ch<-1}`},
		{"loop", `package main;func main(){ch:=make(chan int);for {ch<-1}}`},
		{"recursion", `package main;func f(){f()};func main(){f()}`},
		{"external-effects", `package main;func external()bool;func main(){ch:=make(chan int);if external(){ch<-1}}`},
		{"unknown-unused-result", `package main;func external();func main(){external()}`},
		{"dynamic-call", `package main;func apply(f func()){f()};func main(){apply(func(){})}`},
		{"init-channel", `package main;var ch=make(chan int);func main(){}`},
		{"init-goroutine", `package main;func init(){go func(){}()};func main(){}`},
		{"init-unknown", `package main;func external();func init(){external()};func main(){}`},
		{"defer", `package main;func main(){defer func(){}()}`},
		{"panic", `package main;func main(){panic("bad")}`},
		{"unresolved-field", `package main;type box struct{ch chan int};func main(){b:=box{make(chan int)};b.ch<-1}`},
		{"changing-capture", `package main;func main(){ch:=make(chan int);go func(){ch=make(chan int);ch<-1}();<-ch}`},
		{"mutex-copy", `package main;import "sync";func f(mu sync.Mutex){mu.Lock()};func main(){var mu sync.Mutex;f(mu)}`},
		{"waitgroup-reuse", `package main;import "sync";func main(){var wg sync.WaitGroup;wg.Wait();wg.Add(1)}`},
		{"concurrent-add", `package main;import "sync";func main(){var wg sync.WaitGroup;go func(){wg.Add(1)}();wg.Wait()}`},
		{"rwmutex", `package main;import "sync";func main(){var mu sync.RWMutex;mu.RLock()}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := fromSource(t, c.source)
			if !m.HasErrors() {
				t.Fatalf("unsafe input accepted: %+v", m)
			}
			if _, _, err := tla.Generate(m); err == nil {
				t.Fatal("emitted unsupported model")
			}
		})
	}
}
func TestTrustedResultAndStaticClosure(t *testing.T) {
	m := fromSource(t, `package main;func external()bool;func main(){ch:=make(chan int);if external(){ch<-1}}`, "fixture.external")
	if m.HasErrors() || m.Outcome != diagnostic.Abstracted || len(m.AbstractedPredicates) != 1 {
		t.Fatalf("bad trusted abstraction: %+v", m)
	}
	kinds := []behavior.EffectKind{}
	for _, tr := range m.Transitions {
		for _, e := range tr.Effects {
			kinds = append(kinds, e.Kind)
		}
	}
	if !reflect.DeepEqual(kinds, []behavior.EffectKind{behavior.Send}) {
		t.Fatalf("send removed: %v", kinds)
	}
	closure := fromSource(t, `package main;func main(){ch:=make(chan int);go func(){ch<-1}();<-ch}`)
	if closure.HasErrors() {
		t.Fatalf("static closure rejected: %+v", closure.Diagnostics)
	}
}
func TestDiagnosticsAndIRDeterministic(t *testing.T) {
	a := example(t, "unbuffered")
	b := example(t, "unbuffered")
	if !reflect.DeepEqual(a, b) {
		t.Fatal("nondeterministic IR")
	}
	var out strings.Builder
	analysis.Inspect(&out, a)
	for _, s := range []string{"Processes:", "Channels:", "Transitions:", "Assumptions:"} {
		if !strings.Contains(out.String(), s) {
			t.Errorf("missing %s", s)
		}
	}
}

func TestSynchronizationIdentityRequiresDominatingInitialization(t *testing.T) {
	for name, source := range map[string]string{
		"future-store-send":   `package main;func main(){var ch chan int;func(){ch<-1}();ch=make(chan int,1);<-ch}`,
		"future-store-close":  `package main;func main(){var ch chan int;func(){close(ch)}();ch=make(chan int,1)}`,
		"store-after-spawn":   `package main;func main(){var ch chan int;go func(){ch<-1}();ch=make(chan int,1);<-ch}`,
		"store-after-capture": `package main;func main(){var ch chan int;f:=func(){ch<-1};ch=make(chan int,1);f();<-ch}`,
		"conditional-store":   `package main;func yes()bool{return true};func main(){var ch chan int;if yes(){ch=make(chan int,1)};func(){ch<-1}()}`,
	} {
		t.Run(name, func(t *testing.T) {
			m := fromSource(t, source)
			if !m.HasErrors() {
				t.Fatalf("future/non-dominating store accepted: %+v", m)
			}
			found := false
			for _, d := range m.Diagnostics {
				if d.Code == "sync-initialization-order" {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing initialization-order diagnostic: %+v", m.Diagnostics)
			}
			if _, _, err := tla.Generate(m); err == nil {
				t.Fatal("unsafe model executable")
			}
		})
	}
	t.Run("dominating-store", func(t *testing.T) {
		m := fromSource(t, `package main;func main(){ch:=make(chan int,1);func(){ch<-1}();<-ch}`)
		if m.HasErrors() {
			t.Fatalf("dominating store rejected: %+v", m.Diagnostics)
		}
	})
}
