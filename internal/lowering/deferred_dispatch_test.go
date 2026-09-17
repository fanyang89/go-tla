package lowering

import (
	"go/types"
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/effects"
	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
)

func TestDeferredDispatchConsumesProofs(t *testing.T) {
	for _, kind := range []string{"interface", "field"} {
		for _, broken := range []string{"none", "graph", "operand", "origin", "wrapper", "foreign-stack"} {
			t.Run(kind+"/"+broken, func(t *testing.T) {
				source := `package main;type I interface{Finish()};type T int;func(T)Finish(){};func main(){var x I=T(1);defer x.Finish()}`
				if kind == "field" {
					source = `package main;import "sync";type H struct{mu sync.Mutex;f func()};func main(){h:=&H{f:func(){}};defer h.f()}`
				}
				p := testutil.Load(t, source)
				main, _ := p.Main()
				b := &builder{p: p, m: &behavior.Model{Outcome: "precisely-modeled"}, effects: effects.New(p, nil), plans: map[*ssa.Function]*functionPlan{}, names: map[string]int{}, resources: map[string]bool{}, channelFields: map[string]string{}}
				plan := b.plan(main)
				var site *ssa.Defer
				for c := range plan.Calls {
					if d, ok := c.(*ssa.Defer); ok {
						site = d
					}
				}
				if site == nil || plan.Calls[site].Callee == nil {
					t.Fatal("target missing")
				}
				fr := &frame{f: main, plan: plan, process: "main", ids: map[ssa.Value]string{}, defers: map[*ssa.Defer]deferredCall{}, cleanup: map[ssa.Instruction][]deferredCall{}}
				b.allocateResources(fr)
				switch broken {
				case "graph":
					p.Calls.Nodes[main].Out = nil
				case "operand":
					delete(plan.Slice.Data, site.Common().Value)
				case "origin":
					if kind == "interface" {
						box := site.Common().Value.(*ssa.MakeInterface)
						delete(plan.Slice.Data, box.X)
					} else {
						proof := p.CallableField(site)
						if proof == nil {
							t.Fatal("field proof missing")
						}
						proof.Initializer.Val = ssa.NewConst(nil, proof.Initializer.Val.Type())
					}
				case "wrapper":
					plan.Calls[site].Callee.Synthetic = "unproved wrapper"
				case "foreign-stack":
					site.DeferStack = ssa.NewConst(nil, types.Typ[types.UnsafePointer])
				}
				ok := b.prepareDefers(fr)
				if broken == "none" {
					if !ok || b.m.HasErrors() {
						t.Fatalf("valid proof rejected: %+v", b.m.Diagnostics)
					}
				} else if ok || !b.m.HasErrors() {
					t.Fatalf("stale %s accepted", broken)
				}
			})
		}
	}
}
