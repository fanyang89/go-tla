package lowering

import (
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/effects"
	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
)

func TestFiniteDataConsumesGraphAndSlice(t *testing.T) {
	for _, broken := range []string{"none", "slice", "outer-graph", "inner-graph"} {
		t.Run(broken, func(t *testing.T) {
			p := testutil.Load(t, `package main;func value(n int)int{return n+1};func scan(s []int)int{n:=0;for _,v:=range s{n+=value(v)};return n};func main(){scan(nil)}`)
			main, _ := p.Main()
			b := &builder{p: p, m: &behavior.Model{Outcome: "precisely-modeled"}, effects: effects.New(p, nil), plans: map[*ssa.Function]*functionPlan{}}
			plan := b.plan(main)
			var site *ssa.Call
			for call, summary := range plan.Calls {
				if summary.Kind == effects.FiniteData {
					site = call.(*ssa.Call)
				}
			}
			if site == nil || !plan.Discovery.Roots[site] || !plan.Slice.Data[site.Common().Args[0]] {
				t.Fatal("missing finite computation proof/root")
			}
			callee := plan.Calls[site].Callee
			switch broken {
			case "slice":
				delete(plan.Slice.Data, site.Common().Args[0])
			case "outer-graph":
				p.Calls.Nodes[main].Out = nil
			case "inner-graph":
				p.Calls.Nodes[callee].Out = nil
			}
			ok := b.consumeFiniteData(&frame{f: main, plan: plan}, site, callee)
			if broken == "none" {
				if !ok || b.m.HasErrors() {
					t.Fatalf("valid proof rejected: %+v", b.m.Diagnostics)
				}
				return
			}
			if ok || !b.m.HasErrors() {
				t.Fatal("missing proof accepted")
			}
		})
	}
}
