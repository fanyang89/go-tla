package lowering

import (
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/effects"
	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
)

func TestNilInterfaceProofRechecksOwnership(t *testing.T) {
	p := testutil.Load(t, `package main;var escaped any;type T struct{V any};func build()*T{p:=new(T);p.V=nil;return p};func main(){_=build()}`)
	main, _ := p.Main()
	b := &builder{p: p, m: &behavior.Model{Outcome: "precisely-modeled"}, effects: effects.New(p, nil), plans: map[*ssa.Function]*functionPlan{}}
	plan := b.plan(main)
	var site *ssa.Call
	for c, s := range plan.Calls {
		if s.Kind == effects.ConstantData {
			site = c.(*ssa.Call)
		}
	}
	if site == nil {
		t.Fatal("missing nil-interface constructor proof")
	}
	if !b.consumeConstantData(&frame{f: main, plan: plan}, site) {
		t.Fatal("valid constructor refused")
	}
	changed := false
	for _, bb := range p.Roots[0].Func("build").Blocks {
		for _, i := range bb.Instrs {
			if store, ok := i.(*ssa.Store); ok {
				store.Addr = p.Roots[0].Members["escaped"].(*ssa.Global)
				changed = true
			}
		}
	}
	if !changed {
		t.Fatal("private store missing")
	}
	if b.consumeConstantData(&frame{f: main, plan: plan}, site) || !b.m.HasErrors() {
		t.Fatal("cached proof hides external interface publication")
	}
}
