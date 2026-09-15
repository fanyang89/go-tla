// Package slice computes pass 5's conservative backward control/data slice.
package slice

import (
	"github.com/fanmi/go-tla/internal/discovery"
	"golang.org/x/tools/go/ssa"
)

type Result struct {
	Roots   map[ssa.Instruction]bool
	Control map[*ssa.If]bool
	Data    map[ssa.Value]bool
}

// Compute deliberately retains all predecessor control on a path to a behavioral
// root, rather than relying on optimistic postdominator or alias assumptions.
func Compute(f *ssa.Function, relevantCall func(*ssa.Call) bool) Result {
	roots := discovery.Scan(f).Roots
	for _, bb := range f.Blocks {
		for _, i := range bb.Instrs {
			if c, ok := i.(*ssa.Call); ok && relevantCall(c) {
				roots[i] = true
			}
		}
	}
	return FromRoots(f, roots)
}

// FromRoots consumes discovery and call-effect roots instead of rediscovering them.
func FromRoots(f *ssa.Function, roots map[ssa.Instruction]bool) Result {
	r := Result{map[ssa.Instruction]bool{}, map[*ssa.If]bool{}, map[ssa.Value]bool{}}
	blocks := map[*ssa.BasicBlock]bool{}
	var markBlock func(*ssa.BasicBlock)
	markBlock = func(b *ssa.BasicBlock) {
		if blocks[b] {
			return
		}
		blocks[b] = true
		for _, p := range b.Preds {
			markBlock(p)
		}
	}
	var markValue func(ssa.Value)
	markValue = func(v ssa.Value) {
		if v == nil || r.Data[v] {
			return
		}
		r.Data[v] = true
		if i, ok := v.(ssa.Instruction); ok {
			for _, p := range i.Operands(nil) {
				if p != nil {
					markValue(*p)
				}
			}
		}
	}
	for _, b := range f.Blocks {
		for _, i := range b.Instrs {
			if roots[i] {
				r.Roots[i] = true
				markBlock(b)
				for _, p := range i.Operands(nil) {
					if p != nil {
						markValue(*p)
					}
				}
			}
		}
	}
	for b := range blocks {
		for _, i := range b.Instrs {
			if x, ok := i.(*ssa.If); ok {
				r.Control[x] = true
				markValue(x.Cond)
			}
		}
	}
	return r
}

// HasCycle rejects unbounded allocation/spawn/counters and silent sequential
// divergence. Supporting bounded loops requires a separate proved bound pass.
func HasCycle(f *ssa.Function) bool {
	color := map[*ssa.BasicBlock]int{}
	var visit func(*ssa.BasicBlock) bool
	visit = func(b *ssa.BasicBlock) bool {
		if color[b] == 1 {
			return true
		}
		if color[b] == 2 {
			return false
		}
		color[b] = 1
		for _, s := range b.Succs {
			if visit(s) {
				return true
			}
		}
		color[b] = 2
		return false
	}
	return len(f.Blocks) > 0 && visit(f.Blocks[0])
}
