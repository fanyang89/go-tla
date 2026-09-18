package lowering

import (
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/effects"
	"github.com/fanmi/go-tla/internal/testutil"
	"github.com/fanmi/go-tla/internal/tla"
	"golang.org/x/tools/go/ssa"
)

func TestInitializerRechecksPrivateStore(t *testing.T) {
	for _, mutation := range []string{"none", "global-store", "transitive-global-store"} {
		t.Run(mutation, func(t *testing.T) {
			target := "ctor"
			if mutation == "transitive-global-store" {
				target = "outer"
			}
			p := testutil.Load(t, `package main;type B struct{p *int};var exposed *int;var n int;func ctor(p *int)*B{return &B{p}};func outer(p *int)*B{return ctor(p)};var value=`+target+`(&n);func main(){}`)
			a := effects.New(p, nil)
			init := p.Roots[0].Func("init")
			outer := p.Roots[0].Func(target)
			ctor := p.Roots[0].Func("ctor")
			var call *ssa.Call
			for _, bb := range init.Blocks {
				for _, i := range bb.Instrs {
					if c, ok := i.(*ssa.Call); ok && c.Common().StaticCallee() == outer {
						call = c
					}
				}
			}
			if call == nil || a.Call(call).Kind != effects.Pure {
				t.Fatal("cached pure constructor fixture missing")
			}
			if mutation != "none" {
				changed := false
				for _, bb := range ctor.Blocks {
					for _, i := range bb.Instrs {
						if s, ok := i.(*ssa.Store); ok {
							s.Addr = p.Roots[0].Members["exposed"].(*ssa.Global)
							changed = true
						}
					}
				}
				if !changed {
					t.Fatal("store missing")
				}
			}
			b := &builder{p: p, m: &behavior.Model{Outcome: "precisely-modeled"}, effects: a, loopReported: map[*ssa.Function]bool{}}
			b.initializers()
			if mutation == "none" {
				if b.m.HasErrors() {
					t.Fatalf("valid constructor refused: %+v", b.m.Diagnostics)
				}
				return
			}
			found := false
			for _, d := range b.m.Diagnostics {
				if d.Code == "pure-data-contract" {
					found = true
				}
			}
			if !found {
				t.Fatalf("stale private store accepted: %+v", b.m.Diagnostics)
			}
			if _, _, err := tla.Generate(b.m); err == nil {
				t.Fatal("stale purity emitted executable")
			}
		})
	}
}
