package tests

import (
	"testing"

	"github.com/fanmi/go-tla/internal/tla"
)

func TestTLCDeferredHelpers(t *testing.T) {
	for _, tc := range []struct{ name, source, want string }{
		{"pure-closure", `package main;func main(){c:=make(chan int,1);c<-1;defer func(){}()}`, "No error has been found"},
		{"unlock-helper", `package main;import "sync";func unlock(m *sync.Mutex){m.Unlock()};func f(m *sync.Mutex){defer unlock(m)};func main(){var m sync.Mutex;m.Lock();f(&m);m.Lock();m.Unlock()}`, "No error has been found"},
		{"invalid-closure-unlock", `package main;import "sync";func main(){var m sync.Mutex;defer func(){m.Unlock()}()}`, "Error: Invariant NoSynchronizationErrors is violated"},
		{"done-helper", `package main;import "sync";func done(w *sync.WaitGroup){w.Done()};func f(w *sync.WaitGroup){defer done(w)};func main(){var w sync.WaitGroup;w.Add(1);go f(&w);w.Wait()}`, "No error has been found"},
		{"blocked-cleanup", `package main;import "sync";func wait(c chan int){<-c};func f(c chan int,w *sync.WaitGroup){defer w.Done();defer wait(c)};func main(){var w sync.WaitGroup;w.Add(1);c:=make(chan int);go f(c,&w);w.Wait()}`, "Error: Deadlock reached"},
		{"lifo-send-close", `package main;func send(c chan int){c<-1};func shut(c chan int){close(c)};func f(c chan int){defer shut(c);defer send(c)};func main(){c:=make(chan int,1);f(c);<-c}`, "No error has been found"},
		{"lifo-close-send", `package main;func send(c chan int){c<-1};func shut(c chan int){close(c)};func f(c chan int){defer send(c);defer shut(c)};func main(){c:=make(chan int,1);f(c)}`, "Error: Invariant NoSynchronizationErrors is violated"},
		{"nested-defer", `package main;import "sync";func done(w *sync.WaitGroup){defer w.Done()};func f(w *sync.WaitGroup){defer done(w)};func main(){var w sync.WaitGroup;w.Add(2);f(&w);f(&w);w.Wait()}`, "No error has been found"},
		{"argument-snapshot", `package main;import "sync";func unlock(m *sync.Mutex){m.Unlock()};func f(a,b *sync.Mutex){p:=a;defer unlock(p);p=b;_=p};func main(){var a,b sync.Mutex;a.Lock();f(&a,&b);a.Lock();a.Unlock()}`, "No error has been found"},
		{"conditional-registration", `package main;import "sync";func unknown()bool;func done(w *sync.WaitGroup){w.Done()};func f(w *sync.WaitGroup){if unknown(){defer done(w)}};func main(){var w sync.WaitGroup;w.Add(1);go f(&w);w.Wait()}`, "Error: Deadlock reached"},
		{"multiple-returns", `package main;import "sync";func unknown()bool;func done(w *sync.WaitGroup){w.Done()};func f(w *sync.WaitGroup){defer done(w);if unknown(){return}};func main(){var w sync.WaitGroup;w.Add(1);go f(&w);w.Wait()}`, "No error has been found"},
		{"method", `package main;import "sync";type T struct{mu sync.Mutex};func(t *T)cleanup(){t.mu.Unlock()};func f(t *T){defer t.cleanup()};func main(){t:=&T{};t.mu.Lock();f(t);t.mu.Lock();t.mu.Unlock()}`, "No error has been found"},
	} {
		t.Run(tc.name, func(t *testing.T) { checkTLC(t, fromSource(t, tc.source, "fixture.unknown"), tc.want) })
	}
}

func TestDeferredHelperRefusals(t *testing.T) {
	for name, source := range map[string]string{
		"recursive":          `package main;func cleanup(){defer cleanup()};func main(){defer cleanup()}`,
		"unknown-body":       `package main;func opaque();func cleanup(){opaque()};func main(){defer cleanup()}`,
		"panic":              `package main;func cleanup(){panic("bad")};func main(){defer cleanup()}`,
		"recover":            `package main;func cleanup(){recover()};func main(){defer cleanup()}`,
		"dynamic":            `package main;func f(g func()){defer g()};func main(){f(func(){})}`,
		"future-capture":     `package main;func main(){var c chan int;defer func(){close(c)}();c=make(chan int)}`,
		"reassigned-capture": `package main;func main(){c:=make(chan int);defer func(){close(c)}();c=make(chan int)}`,
		"loop-registration":  `package main;func cleanup(){};func main(){for i:=0;i<2;i++{defer cleanup()}}`,
		"initializer":        `package main;func cleanup(){};func init(){defer cleanup()};func main(){}`,
	} {
		t.Run(name, func(t *testing.T) {
			// Trusting cleanup itself must not erase its unavailable/unsafe body.
			m := fromSource(t, source, "fixture.cleanup")
			if !m.HasErrors() {
				t.Fatal("unproved deferred helper accepted")
			}
			if _, _, err := tla.Generate(m); err == nil {
				t.Fatal("unsupported cleanup executable")
			}
		})
	}
}
