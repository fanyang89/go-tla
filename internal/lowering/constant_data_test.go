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

func TestConstantDataConsumesProof(t *testing.T) {
	for _, broken := range []string{"none", "argument", "slice", "outer-graph", "inner-graph", "body"} {
		t.Run(broken, func(t *testing.T) {
			p := testutil.Load(t, `package main;func validate(n int)int{if n!=2{panic("bad")};return n};func build(n int)int{return validate(n)};func main(){_=build(2)}`)
			main, _ := p.Main()
			b := &builder{p: p, m: &behavior.Model{Outcome: "precisely-modeled"}, effects: effects.New(p, nil), plans: map[*ssa.Function]*functionPlan{}}
			plan := b.plan(main)
			var site *ssa.Call
			for call, summary := range plan.Calls {
				if summary.Kind == effects.ConstantData {
					site = call.(*ssa.Call)
				}
			}
			if site == nil || !plan.Discovery.Roots[site] {
				t.Fatal("missing literal-input root")
			}
			switch broken {
			case "argument":
				site.Call.Args[0] = ssa.NewConst(constant.MakeInt64(1), types.Typ[types.Int])
			case "slice":
				delete(plan.Slice.Data, site.Call.Args[0])
			case "outer-graph":
				p.Calls.Nodes[main].Out = nil
			case "inner-graph":
				p.Calls.Nodes[p.Roots[0].Func("build")].Out = nil
			case "body":
				for _, bb := range p.Roots[0].Func("validate").Blocks {
					for _, i := range bb.Instrs {
						if cmp, ok := i.(*ssa.BinOp); ok && cmp.Op == token.NEQ {
							cmp.Y = ssa.NewConst(constant.MakeInt64(3), types.Typ[types.Int])
						}
					}
				}
			}
			ok := b.consumeConstantData(&frame{f: main, plan: plan}, site)
			if broken == "none" {
				if !ok || b.m.HasErrors() {
					t.Fatalf("valid proof rejected: %+v", b.m.Diagnostics)
				}
				return
			}
			if ok || !b.m.HasErrors() {
				t.Fatal("stale literal-input proof accepted")
			}
		})
	}
}
