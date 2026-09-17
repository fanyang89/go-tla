package lowering

import (
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/effects"
	"github.com/fanmi/go-tla/internal/testutil"
	"github.com/fanmi/go-tla/internal/tla"
	"golang.org/x/tools/go/ssa"
)

func TestInitializerConsumesCachedGraph(t *testing.T) {
	for _, finite := range []bool{false, true} {
		for _, broken := range []string{"none", "outer", "inner"} {
			name := "acyclic/" + broken
			body := "return add(1)"
			if finite {
				name = "finite/" + broken
				body = "n:=0;for _,v:=range s{n+=add(v)};return n"
			}
			t.Run(name, func(t *testing.T) {
				p := testutil.Load(t, `package main;func add(n int)int{return n+1};func value(s []int)int{`+body+`};var initial=value(nil);func main(){}`)
				init := p.Roots[0].Func("init")
				value := p.Roots[0].Func("value")
				a := effects.New(p, nil)
				var outer *ssa.Call
				for _, bb := range init.Blocks {
					for _, i := range bb.Instrs {
						if call, ok := i.(*ssa.Call); ok && call.Common().StaticCallee() == value {
							outer = call
						}
					}
				}
				if outer == nil || !a.Call(outer).IsPure() {
					t.Fatal("missing accepted cached initializer summary")
				}
				switch broken {
				case "outer":
					p.Calls.Nodes[init].Out = nil
				case "inner":
					p.Calls.Nodes[value].Out = nil
				}
				b := &builder{p: p, m: &behavior.Model{Outcome: "precisely-modeled"}, effects: a, loopReported: map[*ssa.Function]bool{}}
				b.initializers()
				if broken == "none" {
					if b.m.HasErrors() {
						t.Fatalf("valid initializer refused: %+v", b.m.Diagnostics)
					}
					return
				}
				found := false
				for _, d := range b.m.Diagnostics {
					if d.Code == "call-contract" || d.Code == "finite-data-contract" {
						found = true
					}
				}
				if !found || !b.m.HasErrors() {
					t.Fatalf("stale initializer summary accepted: %+v", b.m.Diagnostics)
				}
				if _, _, err := tla.Generate(b.m); err == nil {
					t.Fatal("stale proof emitted executable TLA+")
				}
			})
		}
	}
}
