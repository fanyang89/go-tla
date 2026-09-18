package lowering

import (
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"
)

// Resolve only the single-assignment local result slot synthesized around
// deferred cleanup. Rebuild uses rather than trusting cached referrers.
func (b *builder) returnOrigin(fr *frame, v ssa.Value) (ssa.Value, bool) {
	load, ok := v.(*ssa.UnOp)
	if !ok || load.Op != token.MUL {
		return v, true
	}
	a, ok := load.X.(*ssa.Alloc)
	if !ok {
		return v, true
	}
	fail := func() (ssa.Value, bool) {
		b.diag("error", "return-contract", "channel result slot lacks a current private single-store/dominance/slice proof", v.Pos())
		return nil, false
	}
	if a.Parent() != fr.f || a.Heap {
		return fail()
	}
	var store *ssa.Store
	var loads []*ssa.UnOp
	found := false
	steps := 0
	for _, bb := range fr.f.Blocks {
		for _, i := range bb.Instrs {
			steps++
			if steps > 4096 || i.Parent() != fr.f || i.Block() != bb {
				return fail()
			}
			if i == a {
				found = true
			}
			used := false
			for _, p := range i.Operands(nil) {
				steps++
				if steps > 4096 {
					return fail()
				}
				if p != nil && *p == a {
					used = true
				}
			}
			if !used {
				continue
			}
			switch x := i.(type) {
			case *ssa.Store:
				if x.Addr != a || store != nil {
					return fail()
				}
				store = x
			case *ssa.UnOp:
				if x.Op != token.MUL || x.X != a {
					return fail()
				}
				loads = append(loads, x)
			case *ssa.DebugRef:
			default:
				return fail()
			}
		}
	}
	if !found || store == nil || store.Val == nil || !types.Identical(store.Val.Type(), load.Type()) || !fr.plan.Slice.Roots[store] || !b.requireData(fr, store.Val) {
		return fail()
	}
	for _, u := range loads {
		if !recoveryReturnBlock(fr.f, u.Block()) && !instructionDominates(store, u) {
			return fail()
		}
	}
	return store.Val, true
}
