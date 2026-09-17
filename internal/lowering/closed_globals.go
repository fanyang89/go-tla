package lowering

import (
	"go/ast"
	"go/token"

	"github.com/fanmi/go-tla/internal/abstract"
	"github.com/fanmi/go-tla/internal/behavior"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// This is deliberately a small initializer language, not a package allowlist:
// a direct global make, followed by an unconditional source init that only closes
// that global. All other initializer effects are still checked normally.
type closedGlobalProof struct {
	global   *ssa.Global
	make     *ssa.MakeChan
	store    *ssa.Store
	call     *ssa.Call
	close    *ssa.Call
	capacity int
}

func (b *builder) proveClosedGlobal(makeChan *ssa.MakeChan) *closedGlobalProof {
	root := makeChan.Parent()
	if root == nil || root.Pkg == nil || root != root.Pkg.Func("init") || root.Synthetic != "package initializer" {
		return nil
	}
	capacity, ok := abstract.Integer(makeChan.Size)
	if !ok || capacity < 0 || capacity > 1024 {
		return nil
	}
	refs := makeChan.Referrers()
	if refs == nil {
		return nil
	}
	var store *ssa.Store
	for _, i := range *refs {
		if _, ok := i.(*ssa.DebugRef); ok {
			continue
		}
		s, ok := i.(*ssa.Store)
		if !ok || s.Val != makeChan || store != nil {
			return nil
		}
		store = s
	}
	if store == nil || store.Parent() != root {
		return nil
	}
	global, ok := store.Addr.(*ssa.Global)
	if !ok || global.Pkg != root.Pkg || !instructionDominates(makeChan, store) {
		return nil
	}
	proof := &closedGlobalProof{global: global, make: makeChan, store: store, capacity: capacity}
	for _, bb := range root.Blocks {
		for _, i := range bb.Instrs {
			call, ok := i.(*ssa.Call)
			if !ok {
				continue
			}
			f := b.p.CallTarget(call)
			if f == nil || f.Pkg != root.Pkg {
				continue
			}
			decl, ok := f.Syntax().(*ast.FuncDecl)
			if !ok || decl.Name.Name != "init" || len(f.Blocks) != 1 {
				continue
			}
			closeCall := onlyGlobalClose(f, global)
			if closeCall == nil {
				continue
			}
			if proof.call != nil || !instructionDominates(store, call) || !mustReachBlock(store.Block(), call.Block(), map[*ssa.BasicBlock]int{}) {
				return nil
			}
			node := b.p.Calls.Nodes[f]
			if node == nil || len(node.In) != 1 || node.In[0].Site != call {
				return nil
			}
			proof.call, proof.close = call, closeCall
		}
	}
	if proof.call == nil {
		return nil
	}
	// Globals have no usable SSA Referrers list. Inspect all SSA functions,
	// including currently unreachable ones, to reject rebindings/address escape.
	// Exhaustion rejects instead of treating a partial inventory as complete.
	remaining := 1000000
	for f := range ssautil.AllFunctions(b.p.SSA) {
		if f == nil {
			continue
		}
		for _, bb := range f.Blocks {
			for _, i := range bb.Instrs {
				remaining--
				if remaining < 0 {
					return nil
				}
				for _, operand := range i.Operands(nil) {
					if operand == nil || *operand != global {
						continue
					}
					switch x := i.(type) {
					case *ssa.Store:
						if x != store || x.Addr != global {
							return nil
						}
					case *ssa.UnOp:
						if x.Op != token.MUL || x.X != global {
							return nil
						}
					case *ssa.DebugRef:
					default:
						return nil
					}
				}
			}
		}
	}
	return proof
}

// Every normal control path after the store must reach the close call. A cycle
// bypassing it is not a completed initializer, even if another path reaches it.
func mustReachBlock(from, target *ssa.BasicBlock, state map[*ssa.BasicBlock]int) bool {
	if from == target {
		return true
	}
	if state[from] != 0 {
		return state[from] == 2
	}
	state[from] = 1
	if len(from.Succs) == 0 {
		return false
	}
	for _, next := range from.Succs {
		if !mustReachBlock(next, target, state) {
			return false
		}
	}
	state[from] = 2
	return true
}

func onlyGlobalClose(f *ssa.Function, global *ssa.Global) *ssa.Call {
	var load *ssa.UnOp
	var closeCall *ssa.Call
	for _, i := range f.Blocks[0].Instrs {
		switch x := i.(type) {
		case *ssa.UnOp:
			if load != nil || x.Op != token.MUL || x.X != global {
				return nil
			}
			load = x
		case *ssa.Call:
			builtin, ok := x.Common().Value.(*ssa.Builtin)
			if !ok || builtin.Name() != "close" || closeCall != nil || len(x.Common().Args) != 1 || load == nil || x.Common().Args[0] != load {
				return nil
			}
			closeCall = x
		case *ssa.Return:
			if closeCall == nil || len(x.Results) != 0 {
				return nil
			}
		case *ssa.DebugRef:
		default:
			return nil
		}
	}
	return closeCall
}

func (b *builder) consumeClosedGlobal(i ssa.Instruction) bool {
	switch x := i.(type) {
	case *ssa.MakeChan:
		proof := b.proveClosedGlobal(x)
		if proof == nil {
			return false
		}
		if b.closedGlobals == nil {
			b.closedGlobals = map[*ssa.Global]*closedGlobalProof{}
		}
		if b.globals == nil {
			b.globals = map[*ssa.Global]string{}
		}
		id := b.fresh("global_" + proof.global.String())
		b.globals[proof.global] = id
		b.closedGlobals[proof.global] = proof
		b.m.Channels = append(b.m.Channels, behavior.Channel{ID: id, Capacity: proof.capacity, InitiallyClosed: true, Source: b.position(x.Pos(), x.Parent())})
		b.diag("info", "closed-global-init", "proved unique global channel creation and pre-main close: "+proof.global.String(), x.Pos())
		return true
	case *ssa.Call:
		for _, proof := range b.closedGlobals {
			if proof.call != x {
				continue
			}
			fresh := b.proveClosedGlobal(proof.make)
			if fresh == nil || fresh.global != proof.global || fresh.call != x || fresh.close != proof.close {
				return false
			}
			b.diag("info", "closed-global-init", "proved unconditional initialization close: "+proof.global.String(), proof.close.Pos())
			return true
		}
	}
	return false
}
