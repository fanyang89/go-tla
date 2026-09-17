package tests

import (
	"strings"
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/tla"
)

func TestProvedIntegerRange(t *testing.T) {
	for name, source := range map[string]string{
		"literal":            `package main;func main(){ch:=make(chan int,2);for range 2 {ch<-1};<-ch;<-ch}`,
		"imported-bound":     `package main;import "unicode/utf8";func main(){for range utf8.UTFMax {}}`,
		"helper":             `package main;func worker(ch chan int){for range 2 {ch<-1}};func main(){ch:=make(chan int,2);worker(ch);<-ch;<-ch}`,
		"closure":            `package main;func main(){ch:=make(chan int,2);func(){for range 2 {ch<-1}}();<-ch;<-ch}`,
		"negative-int64":     `package main;func main(){for range int64(-4294967295) {var ch chan int;ch<-1}}`,
		"initializer-helper": `package main;func helper(){for range 2 {_=1}};func init(){helper()};func main(){}`,
		"constant":           `package main;const workers=1+1;func main(){ch:=make(chan int,2);for range workers {ch<-1};<-ch;<-ch}`,
		"blank-index":        `package main;func main(){ch:=make(chan int,2);for _ = range 2 {ch<-1};<-ch;<-ch}`,
		"zero":               `package main;import "sync";func main(){for range 0 {var mu sync.Mutex;mu.Lock()}}`,
		"negative":           `package main;func main(){for range -2 {var ch chan int;ch<-1}}`,
		"return":             `package main;func main(){for range 2 {return}}`,
	} {
		t.Run(name, func(t *testing.T) {
			m := fromSource(t, source)
			if m.HasErrors() {
				t.Fatalf("unsupported: %+v", m.Diagnostics)
			}
			found := false
			for _, d := range m.Diagnostics {
				if d.Code == "proved-loop" {
					found = true
				}
			}
			if !found {
				t.Fatal("bound proof not recorded")
			}
			if _, _, err := tla.Generate(m); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestFiniteLoopRefusals(t *testing.T) {
	for name, body := range map[string]string{
		"index":                    "for i:=range 2 {_=i}",
		"dynamic":                  "n:=2;for range n {}",
		"over-budget":              "for range 17 {}",
		"nested":                   "for range 2 {for range 2 {}}",
		"continue":                 "for range 2 {continue}",
		"goto":                     "for range 2 {goto done};done:",
		"label":                    "for range 2 {again: goto again}",
		"line-directive":           "\n//line virtual.go:50\nfor range 2 {}",
		"unknown-body":             "for range 2 {unknown()}",
		"break":                    "for range 2 {break}",
		"classic":                  "var mu sync.Mutex;for i:=0;i<2;i++ {mu.Lock();mu.Unlock()}",
		"channel-range-allocation": "ch:=make(chan int);for range ch {next:=make(chan int);_=next}",
		"loop-defer-limit":         "var mu sync.Mutex;for range 16 {defer mu.Unlock();defer mu.Unlock();defer mu.Unlock();defer mu.Unlock();defer mu.Unlock()}",
	} {
		t.Run(name, func(t *testing.T) {
			m := fromSource(t, `package main;import "sync";var _ sync.Mutex;func unknown();func main(){`+body+`}`)
			if !m.HasErrors() {
				t.Fatal("unproved/over-budget loop accepted")
			}
			if _, _, err := tla.Generate(m); err == nil {
				t.Fatal("unsupported loop executable")
			}
		})
	}
}

func TestLoopInstancesAndSourcePositions(t *testing.T) {
	m := fromSource(t, `package main
func main(){
 for range 2 {
  ch:=make(chan int)
  go func(){ch<-1}()
  <-ch
 }
}`)
	if m.HasErrors() {
		t.Fatalf("unsupported: %+v", m.Diagnostics)
	}
	if len(m.Channels) != 2 || len(m.Processes) != 3 {
		t.Fatalf("loop identities collapsed: %+v", m.Statistics())
	}
	for _, ch := range m.Channels {
		if ch.Source.File != "main.go" || ch.Source.Line != 4 {
			t.Fatalf("lost source position: %+v", ch.Source)
		}
	}
	for _, tr := range m.Transitions {
		for _, e := range tr.Effects {
			if e.Kind == behavior.Receive && tr.SourcePosition.Line != 6 {
				t.Fatalf("lost receive source: %+v", tr.SourcePosition)
			}
		}
	}
}

func TestLoopByteBudgetAndUnusedFunction(t *testing.T) {
	source := `package main;func unused(){for range 17 {}};func main(){}`
	if m := fromSource(t, source); m.HasErrors() {
		t.Fatal("unused unsupported function rejected")
	}
	source = `package main;func main(){for range 16 {/*` + strings.Repeat("padding", 3000) + `*/}}`
	if m := fromSource(t, source); !m.HasErrors() {
		t.Fatal("expansion byte budget ignored")
	}
}

func TestTLCProvedIntegerRanges(t *testing.T) {
	for _, c := range []struct{ name, source, want string }{
		{"workers", `package main;import "sync";func main(){var wg sync.WaitGroup;wg.Add(2);for range 2 {go func(){defer wg.Done()}()};wg.Wait()}`, "No error has been found"},
		{"allocations", `package main;func main(){for range 2 {ch:=make(chan int);go func(){ch<-1}();<-ch}}`, "No error has been found"},
		{"double-close", `package main;func main(){ch:=make(chan int);for range 2 {close(ch)}}`, "Error: Invariant NoSynchronizationErrors is violated"},
		{"too-many-sends", `package main;func main(){ch:=make(chan int,1);for range 2 {ch<-1}}`, "Error: Deadlock reached"},
		{"defer-not-per-iteration", `package main;import "sync";func main(){var mu sync.Mutex;for range 2 {mu.Lock();defer mu.Unlock()}}`, "Error: Deadlock reached"},
		{"distinct-deferred-mutexes", `package main;import "sync";func main(){for range 2 {var mu sync.Mutex;mu.Lock();defer mu.Unlock()}}`, "No error has been found"},
		{"missing-done", `package main;import "sync";func main(){var wg sync.WaitGroup;wg.Add(2);for range 1 {go func(){defer wg.Done()}()};wg.Wait()}`, "Error: Deadlock reached"},
		{"return-stops-iterations", `package main;func main(){ch:=make(chan int,1);for range 2 {ch<-1;return}}`, "No error has been found"},
		{"zero-iterations", `package main;func main(){for range 0 {var ch chan int;ch<-1}}`, "No error has been found"},
	} {
		t.Run(c.name, func(t *testing.T) { checkTLC(t, fromSource(t, c.source), c.want) })
	}
}
