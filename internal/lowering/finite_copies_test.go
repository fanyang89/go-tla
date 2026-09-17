package lowering

import (
	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/effects"
	"github.com/fanmi/go-tla/internal/testutil"
	"go/types"
	"golang.org/x/tools/go/ssa"
	"testing"
)

func TestFiniteCopyRechecksStoreOwnership(t *testing.T) {
	p := testutil.Load(t, `package main;type E struct{S []int};var leak E;func scan(es []E)int{n:=0;for _,e:=range es{n+=len(e.S)};return n};func main(){scan(nil)}`)
	main, _ := p.Main()
	b := &builder{p: p, m: &behavior.Model{Outcome: "precisely-modeled"}, effects: effects.New(p, nil), plans: map[*ssa.Function]*functionPlan{}}
	plan := b.plan(main)
	var site *ssa.Call
	for call, s := range plan.Calls {
		if s.Kind == effects.FiniteData {
			site = call.(*ssa.Call)
		}
	}
	if site == nil {
		t.Fatal("missing finite-copy proof")
	}
	callee := plan.Calls[site].Callee
	if !b.consumeFiniteData(&frame{f: main, plan: plan}, site, callee) {
		t.Fatal("valid copy refused")
	}
	changed := false
	for _, bb := range callee.Blocks {
		for _, i := range bb.Instrs {
			if store, ok := i.(*ssa.Store); ok {
				if n, ok := store.Val.Type().(*types.Named); ok && n.Obj().Name() == "E" {
					store.Addr = p.Roots[0].Var("leak")
					changed = true
				}
			}
		}
	}
	if !changed {
		t.Fatal("private copy store missing")
	}
	if b.consumeFiniteData(&frame{f: main, plan: plan}, site, callee) || !b.m.HasErrors() {
		t.Fatal("cached copy proof hid publication")
	}
}
