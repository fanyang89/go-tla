package lowering

import (
	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/effects"
	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
	"testing"
)

func TestDeferredBoundMethodConsumesProof(t *testing.T) {
	for _, broken := range []string{"none", "graph", "capture-slice", "capture-arity", "creation-order", "budget"} {
		t.Run(broken, func(t *testing.T) {
			p := testutil.Load(t, `package main;type C chan int;func(c C)Close(){close(c)};func main(){c:=make(C);f:=c.Close;defer f()}`)
			main, _ := p.Main()
			b := &builder{p: p, m: &behavior.Model{}, effects: effects.New(p, nil), plans: map[*ssa.Function]*functionPlan{}, names: map[string]int{}, resources: map[string]bool{}, channelFields: map[string]string{}}
			plan := b.plan(main)
			var site *ssa.Defer
			for c := range plan.Calls {
				if d, ok := c.(*ssa.Defer); ok {
					site = d
				}
			}
			if site == nil {
				t.Fatal("defer missing")
			}
			closure := site.Common().Value.(*ssa.MakeClosure)
			wrapper := closure.Fn.(*ssa.Function)
			fr := &frame{f: main, plan: plan, ids: map[ssa.Value]string{}, defers: map[*ssa.Defer]deferredCall{}, cleanup: map[ssa.Instruction][]deferredCall{}}
			b.allocateResources(fr)
			switch broken {
			case "graph":
				p.Calls.Nodes[wrapper].Out = nil
			case "capture-slice":
				delete(plan.Slice.Data, closure.Bindings[0])
			case "capture-arity":
				closure.Bindings = nil
			case "creation-order":
				ci, si := -1, -1
				for i, ins := range site.Block().Instrs {
					if ins == closure {
						ci = i
					}
					if ins == site {
						si = i
					}
				}
				if ci < 0 || si < 0 {
					t.Fatal("fixture not local")
				}
				site.Block().Instrs[ci], site.Block().Instrs[si] = site.Block().Instrs[si], site.Block().Instrs[ci]
			case "budget":
				last := main.Blocks[0].Instrs[len(main.Blocks[0].Instrs)-1]
				for range 4097 {
					main.Blocks[0].Instrs = append(main.Blocks[0].Instrs, last)
				}
			}
			ok := b.prepareDefers(fr)
			if broken == "none" {
				if !ok || b.m.HasErrors() {
					t.Fatalf("valid wrapper refused: %+v", b.m.Diagnostics)
				}
			} else if ok || !b.m.HasErrors() {
				t.Fatal("stale wrapper accepted")
			}
		})
	}
}
