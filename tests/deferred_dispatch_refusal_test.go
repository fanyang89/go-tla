package tests

import (
	"github.com/fanmi/go-tla/internal/tla"
	"testing"
)

func TestDeferredDispatchRefusals(t *testing.T) {
	const iface = `package main;type I interface{Finish()};type C chan int;func(c C)Finish(){close(c)};`
	for name, source := range map[string]string{
		"nil-interface":                  iface + `func main(){var x I;defer x.Finish()}`,
		"ambiguous-interface":            iface + `func choice()bool;func main(){var x I;if choice(){x=C(make(chan int))}else{x=C(make(chan int))};defer x.Finish()}`,
		"returned-closure":               `package main;func factory()func(){return func(){}};func main(){defer factory()()}`,
		"mutable-field":                  `package main;import "sync";type H struct{mu sync.Mutex;f func()};func main(){h:=&H{f:func(){}};defer h.f();h.f=func(){}}`,
		"sync-interface":                 `package main;import "sync";func main(){var x sync.Locker=&sync.Mutex{};defer x.Unlock()}`,
		"output":                         `package main;type I interface{Finish()};type C int;func(C)Finish(){println("output")};func main(){var x I=C(1);defer x.Finish()}`,
		"panic":                          `package main;type I interface{Finish()};type C int;func(C)Finish(){panic("bad")};func main(){var x I=C(1);defer x.Finish()}`,
		"unmodeled-boxed-channel-object": `package main;type I interface{Finish()};type C struct{c chan int};func(c *C)Finish(){close(c.c)};func main(){var x I=&C{make(chan int)};defer x.Finish()}`,
		"unmodeled-callable-only-object": `package main;type H struct{f func()};func main(){h:=&H{f:func(){}};defer h.f()}`,
	} {
		t.Run(name, func(t *testing.T) {
			m := fromSource(t, source, "fixture.choice")
			if !m.HasErrors() {
				t.Fatal("unproved dispatch accepted")
			}
			if _, _, err := tla.Generate(m); err == nil {
				t.Fatal("unproved dispatch emitted")
			}
		})
	}
}
