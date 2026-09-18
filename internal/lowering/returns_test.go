package lowering

import (
	"context"
	"strings"
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/effects"
	"github.com/fanmi/go-tla/internal/frontend"
	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
)

func TestActualContextNilChannelReturns(t *testing.T) {
	loaded, err := frontend.LoadContext(t.Context(), "../checker", ".")
	if err != nil {
		t.Fatal(err)
	}
	p := frontend.BuildSSA(loaded)
	frontend.BuildCallGraph(p)
	found := map[string]bool{"emptyCtx": false, "withoutCancelCtx": false}
	b := &builder{p: p, m: &behavior.Model{}, effects: effects.New(p, nil), plans: map[*ssa.Function]*functionPlan{}}
	for f := range p.Calls.Nodes {
		if f == nil || f.Pkg == nil || f.Pkg.Pkg.Path() != "context" || f.Name() != "Done" || f.Synthetic != "" || f.Signature.Recv() == nil {
			continue
		}
		for name := range found {
			if !strings.HasSuffix(f.Signature.Recv().Type().String(), "."+name) {
				continue
			}
			found[name] = true
			fr := &frame{f: f, plan: b.plan(f), ids: map[ssa.Value]string{}}
			if id := b.returnedChannel(fr); id != "nil" || b.m.HasErrors() {
				t.Fatalf("actual %s Done refused: %s %+v", name, id, b.m.Diagnostics)
			}
		}
	}
	for name, ok := range found {
		if !ok {
			t.Errorf("actual %s method missing", name)
		}
	}
	if context.Background().Done() != nil || context.WithoutCancel(context.Background()).Done() != nil {
		t.Fatal("native context nil-channel oracle changed")
	}
}

func TestReturnProofConsumesCurrentOperands(t *testing.T) {
	for _, mutation := range []string{"none", "stale-referrers", "operand", "root", "store-root", "store-value", "duplicate-store", "recovery-effect", "recovery-edge", "budget"} {
		t.Run(mutation, func(t *testing.T) {
			p := testutil.Load(t, `package main;func shut(c chan int){close(c)};func f(a,b chan int)chan int{defer shut(a);return a};func main(){}`)
			f := p.Roots[0].Func("f")
			b := &builder{p: p, m: &behavior.Model{}, effects: effects.New(p, nil), plans: map[*ssa.Function]*functionPlan{}}
			plan := b.plan(f)
			fr := &frame{f: f, plan: plan, ids: map[ssa.Value]string{f.Params[0]: "first", f.Params[1]: "second"}}
			var ret *ssa.Return
			var store *ssa.Store
			var cell *ssa.Alloc
			for _, bb := range f.Blocks {
				for _, i := range bb.Instrs {
					switch x := i.(type) {
					case *ssa.Return:
						if bb != f.Recover {
							ret = x
						}
					case *ssa.Store:
						store = x
					case *ssa.Alloc:
						cell = x
					}
				}
			}
			if ret == nil || store == nil || cell == nil || f.Recover == nil {
				t.Fatal("return-slot fixture missing")
			}
			switch mutation {
			case "stale-referrers":
				*cell.Referrers() = nil
			case "operand":
				delete(plan.Slice.Data, ret.Results[0])
			case "root":
				delete(plan.Slice.Roots, ret)
			case "store-root":
				delete(plan.Slice.Roots, store)
			case "store-value":
				store.Val = f.Params[1]
			case "duplicate-store":
				bb := store.Block()
				bb.Instrs = append(bb.Instrs[:len(bb.Instrs)-1], store, bb.Instrs[len(bb.Instrs)-1])
			case "recovery-edge":
				f.Blocks[0].Succs = append(f.Blocks[0].Succs, f.Recover)
			case "recovery-effect":
				f.Recover.Instrs = append([]ssa.Instruction{store}, f.Recover.Instrs...)
			case "budget":
				bb := store.Block()
				for range 4097 {
					bb.Instrs = append(bb.Instrs, cell)
				}
			}
			id := b.returnedChannel(fr)
			valid := mutation == "none" || mutation == "stale-referrers"
			if valid {
				if id != "first" || b.m.HasErrors() {
					t.Fatalf("valid return refused: %s %+v", id, b.m.Diagnostics)
				}
			} else if id != "invalid" || !b.m.HasErrors() {
				t.Fatalf("stale return accepted: %s %+v", id, b.m.Diagnostics)
			}
		})
	}
}
