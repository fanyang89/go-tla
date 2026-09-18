package tests

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/tla"
)

func TestRestrictedDeferRefusals(t *testing.T) {
	for name, source := range map[string]string{
		"lock":        `package main;import "sync";func main(){var mu sync.Mutex;defer mu.Lock()}`,
		"wait":        `package main;import "sync";func main(){var wg sync.WaitGroup;defer wg.Wait()}`,
		"add":         `package main;import "sync";func main(){var wg sync.WaitGroup;defer wg.Add(-1)}`,
		"close":       `package main;func main(){ch:=make(chan int);defer close(ch)}`,
		"trusted":     `package main;func cleanup();func main(){defer cleanup()}`,
		"nil":         `package main;import "sync";func main(){var mu *sync.Mutex;defer mu.Unlock()}`,
		"panic":       `package main;import "sync";func main(){var mu sync.Mutex;defer mu.Unlock();panic("bad")}`,
		"recover":     `package main;import "sync";func main(){var mu sync.Mutex;defer mu.Unlock();recover()}`,
		"loop":        `package main;import "sync";func main(){var mu sync.Mutex;for i:=0;i<2;i++{defer mu.Unlock()}}`,
		"initializer": `package main;import "sync";func init(){var mu sync.Mutex;defer mu.Unlock()};func main(){}`,
		"site-limit":  `package main;import "sync";func main(){var mu sync.Mutex;` + strings.Repeat("defer mu.Unlock();", 65) + `}`,
	} {
		t.Run(name, func(t *testing.T) {
			m := fromSource(t, source, "fixture.cleanup")
			if !m.HasErrors() {
				t.Fatal("unsupported defer accepted")
			}
			if _, _, err := tla.Generate(m); err == nil {
				t.Fatal("unsupported defer executable")
			}
			if name == "site-limit" {
				found := false
				for _, d := range m.Diagnostics {
					if d.Code == "defer-site-limit" {
						found = true
					}
				}
				if !found {
					t.Fatal("missing explicit expansion limit diagnostic")
				}
			}
		})
	}
}

func TestTLCRestrictedDefers(t *testing.T) {
	for _, c := range []struct{ name, source, want string }{
		{"unlock", `package main;import "sync";func release(mu *sync.Mutex){defer mu.Unlock()};func main(){var mu sync.Mutex;mu.Lock();release(&mu);mu.Lock();mu.Unlock()}`, "No error has been found"},
		{"done", `package main;import "sync";func main(){var wg sync.WaitGroup;wg.Add(1);go func(){defer wg.Done()}();wg.Wait()}`, "No error has been found"},
		{"invalid-unlock", `package main;import "sync";func main(){var mu sync.Mutex;defer mu.Unlock()}`, "Error: Invariant NoSynchronizationErrors is violated"},
		{"negative-done", `package main;import "sync";func main(){var wg sync.WaitGroup;defer wg.Done()}`, "Error: Invariant NoSynchronizationErrors is violated"},
		{"blocked-return-expression", `package main;import "sync";func wait(ch chan int)int{<-ch;return 0};func f(mu *sync.Mutex,ch chan int)int{defer mu.Unlock();return wait(ch)};func main(){var mu sync.Mutex;mu.Lock();ch:=make(chan int);go f(&mu,ch);mu.Lock()}`, "Error: Deadlock reached"},
		{"conditional-registration", `package main;import "sync";func unknown()bool;func main(){var wg sync.WaitGroup;wg.Add(1);go func(){if unknown(){defer wg.Done()}}();wg.Wait()}`, "Error: Deadlock reached"},
		{"multiple-returns", `package main;import "sync";func unknown()bool;func f(wg *sync.WaitGroup){defer wg.Done();if unknown(){return}};func main(){var wg sync.WaitGroup;wg.Add(1);go f(&wg);wg.Wait()}`, "No error has been found"},
		{"unselected-case", `package main;import "sync";func main(){var wg sync.WaitGroup;wg.Add(1);go func(){var ch chan int;select{case <-ch:defer wg.Done();default:}}();wg.Wait()}`, "Error: Deadlock reached"},
		{"receiver-capture", `package main;import "sync";func f(a,b *sync.Mutex){p:=a;defer p.Unlock();p=b;_ = p};func main(){var a,b sync.Mutex;a.Lock();f(&a,&b);a.Lock();a.Unlock()}`, "No error has been found"},
		{"nested-frames", `package main;import "sync";func g(wg *sync.WaitGroup){defer wg.Done()};func f(wg *sync.WaitGroup){defer wg.Done();g(wg)};func main(){var wg sync.WaitGroup;wg.Add(4);f(&wg);f(&wg);wg.Wait()}`, "No error has been found"},
		{"fields", `package main;import "sync";type T struct{mu sync.Mutex;wg sync.WaitGroup;ch chan int};func(t *T)work(){defer t.wg.Done();t.mu.Lock();defer t.mu.Unlock();t.ch<-1};func main(){t:=&T{ch:make(chan int)};t.wg.Add(1);go t.work();<-t.ch;t.wg.Wait();t.mu.Lock();t.mu.Unlock()}`, "No error has been found"},
	} {
		t.Run(c.name, func(t *testing.T) { checkTLC(t, fromSource(t, c.source, "fixture.unknown"), c.want) })
	}
}

// Explore exact flag states on every acyclic single-process IR path. This tests
// LIFO, conditional registration and complete draining, not just emitted syntax.
func TestDeferIRPreservesConditionalLIFO(t *testing.T) {
	m := fromSource(t, `package main
import "sync"
func unknown()bool
func main(){
 var a,b,c sync.Mutex
 a.Lock();b.Lock();c.Lock()
 defer a.Unlock()
 if unknown(){defer b.Unlock();if unknown(){return}}
 defer c.Unlock()
}`, "fixture.unknown")
	if m.HasErrors() {
		t.Fatalf("unsupported: %+v", m.Diagnostics)
	}
	if _, _, err := tla.Generate(m); err != nil {
		t.Fatal(err)
	}
	cleanup := map[string]behavior.Effect{}
	for _, tr := range m.Transitions {
		if len(tr.Effects) == 2 && tr.Effects[1].Kind == behavior.AssignAbstractState && tr.Effects[1].Value == 0 {
			cleanup[tr.Effects[1].Variable] = tr.Effects[0]
		}
	}
	if len(cleanup) != 3 {
		t.Fatalf("missing cleanup sites: %v", cleanup)
	}
	p := m.Processes[0]
	initial := map[string]int{}
	for _, v := range p.Locals {
		initial[v.Name] = v.Initial
	}
	var guard func(behavior.Guard, map[string]int) bool
	guard = func(g behavior.Guard, vars map[string]int) bool {
		value := true
		switch g.Kind {
		case behavior.True, behavior.Choice:
		case behavior.Equal:
			value = vars[g.Variable] == g.Value
		case behavior.All:
			for _, term := range g.Terms {
				value = value && guard(term, vars)
			}
		default:
			t.Fatalf("unexpected guard %s", g.Kind)
		}
		return value != g.Negated
	}
	paths := 0
	var walk func(string, map[string]int, []behavior.Effect, int)
	walk = func(pc string, vars map[string]int, stack []behavior.Effect, depth int) {
		if depth > len(m.Transitions)+1 {
			t.Fatal("unexpected cycle")
		}
		if pc == p.Terminal {
			if len(stack) != 0 {
				t.Fatal("return skipped registered cleanup")
			}
			paths++
			return
		}
		enabled := false
		for _, tr := range m.Transitions {
			if tr.Source != pc || !guard(tr.Guard, vars) {
				continue
			}
			enabled = true
			state := maps.Clone(vars)
			pending := slices.Clone(stack)
			for _, e := range tr.Effects {
				if e.Kind == behavior.AssignAbstractState {
					if effect, ok := cleanup[e.Variable]; ok && e.Value == 1 {
						pending = append(pending, effect)
					}
					state[e.Variable] = e.Value
				} else if e.Kind == behavior.Unlock {
					if len(pending) == 0 || pending[len(pending)-1] != e {
						t.Fatalf("non-LIFO cleanup %v; pending=%v", e, pending)
					}
					pending = pending[:len(pending)-1]
				}
			}
			walk(tr.Destination, state, pending, depth+1)
		}
		if !enabled {
			t.Fatal("cleanup control got stuck")
		}
	}
	walk(p.Entry, initial, nil, 0)
	if paths < 3 {
		t.Fatalf("conditional return paths lost: %d", paths)
	}
}
