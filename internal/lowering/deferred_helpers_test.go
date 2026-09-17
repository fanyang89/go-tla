package lowering

import (
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/effects"
	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
)

func TestDeferredHelperConsumesGraphAndSlice(t *testing.T) {
	for _, broken := range []string{"none", "slice", "graph"} {
		t.Run(broken, func(t *testing.T) {
			p := testutil.Load(t, `package main;func cleanup(n int){};func main(){defer cleanup(1)}`)
			main, _ := p.Main()
			b := &builder{p: p, m: &behavior.Model{Outcome: "precisely-modeled"}, effects: effects.New(p, nil), plans: map[*ssa.Function]*functionPlan{}, names: map[string]int{}}
			plan := b.plan(main)
			var site *ssa.Defer
			for call := range plan.Calls {
				if d, ok := call.(*ssa.Defer); ok {
					site = d
				}
			}
			if site == nil || plan.Calls[site].Callee == nil || !plan.Slice.Data[site.Common().Args[0]] {
				t.Fatal("missing deferred call proof")
			}
			if broken == "slice" {
				delete(plan.Slice.Data, site.Common().Args[0])
			}
			if broken == "graph" {
				p.Calls.Nodes[main].Out = nil
			}
			fr := &frame{f: main, plan: plan, process: "main", defers: map[*ssa.Defer]deferredCall{}, cleanup: map[ssa.Instruction][]deferredCall{}}
			ok := b.prepareDefers(fr)
			if broken == "none" {
				if !ok || b.m.HasErrors() {
					t.Fatalf("valid helper rejected: %+v", b.m.Diagnostics)
				}
				return
			}
			if ok || !b.m.HasErrors() {
				t.Fatal("missing proof accepted")
			}
			want := "slice-contract"
			if broken == "graph" {
				want = "call-contract"
			}
			if got := b.m.Diagnostics[len(b.m.Diagnostics)-1].Code; got != want {
				t.Fatalf("got %s want %s", got, want)
			}
		})
	}
}
