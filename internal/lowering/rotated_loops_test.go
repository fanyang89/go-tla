package lowering

import (
	"go/constant"
	"go/token"
	"go/types"
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/effects"
	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
)

func TestRotatedRangeConsumedProof(t *testing.T) {
	p := testutil.Load(t, `package main;func scan(s string)int{n:=0;for i:=range len(s){n+=int(s[i])};return n};func main(){_=scan("abc")}`)
	main, _ := p.Main()
	b := &builder{p: p, m: &behavior.Model{Outcome: "precisely-modeled"}, effects: effects.New(p, nil), plans: map[*ssa.Function]*functionPlan{}}
	plan := b.plan(main)
	var site *ssa.Call
	for call, summary := range plan.Calls {
		if summary.Kind == effects.FiniteData {
			site = call.(*ssa.Call)
		}
	}
	if site == nil {
		t.Fatal("finite proof missing")
	}
	callee := plan.Calls[site].Callee
	if !b.consumeFiniteData(&frame{f: main, plan: plan}, site, callee) {
		t.Fatal("baseline proof refused")
	}
	changed := false
	for _, bb := range callee.Blocks {
		for _, i := range bb.Instrs {
			if op, ok := i.(*ssa.BinOp); ok && op.Op == token.ADD {
				if phi, ok := op.X.(*ssa.Phi); ok && phi.Comment == "rangeint.iter" {
					op.Y = ssa.NewConst(constant.MakeInt64(2), types.Typ[types.Int])
					changed = true
				}
			}
		}
	}
	if !changed {
		t.Fatal("mutation missing")
	}
	if b.consumeFiniteData(&frame{f: main, plan: plan}, site, callee) || !b.m.HasErrors() {
		t.Fatal("stale rotated proof accepted")
	}
}
