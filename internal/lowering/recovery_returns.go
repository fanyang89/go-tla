package lowering

import (
	"go/token"

	"golang.org/x/tools/go/ssa"
)

// SSA emits a predecessor-free recovery-only return block for functions with
// defers. It is not a normal execution path. Explicit panic/recover remains
// rejected by ordinary lowering; implicit sequential panics are outside the
// declared communication domain. Never exempt instructions with other effects.
func recoveryReturnBlock(f *ssa.Function, bb *ssa.BasicBlock) bool {
	if bb == nil || bb != f.Recover || len(f.Blocks) == 0 || bb == f.Blocks[0] || len(bb.Preds) != 0 || len(bb.Succs) != 0 || len(bb.Instrs) == 0 {
		return false
	}
	// A stale predecessor cache must not hide an ordinary CFG edge.
	steps := 0
	for _, block := range f.Blocks {
		steps++
		if steps > 4096 {
			return false
		}
		for _, succ := range block.Succs {
			steps++
			if steps > 4096 || succ == bb {
				return false
			}
		}
	}
	for n, i := range bb.Instrs {
		if i.Parent() != f || i.Block() != bb {
			return false
		}
		switch x := i.(type) {
		case *ssa.UnOp:
			a, ok := x.X.(*ssa.Alloc)
			if x.Op != token.MUL || !ok || a.Parent() != f || a.Heap {
				return false
			}
		case *ssa.Return:
			if n != len(bb.Instrs)-1 || len(x.Results) != f.Signature.Results().Len() {
				return false
			}
			for _, v := range x.Results {
				u, ok := v.(*ssa.UnOp)
				if !ok || u.Block() != bb {
					return false
				}
			}
		default:
			return false
		}
	}
	_, ok := bb.Instrs[len(bb.Instrs)-1].(*ssa.Return)
	return ok
}
