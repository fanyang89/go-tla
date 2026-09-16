package lowering

import (
	"fmt"
	"go/token"

	"github.com/fanmi/go-tla/internal/behavior"
	cslice "github.com/fanmi/go-tla/internal/slice"
	"golang.org/x/tools/go/ssa"
)

// Receive cycles do not need a guessed trip count. They need stable identities,
// finite modeled state, an exact closed-empty exit and no hidden pure subcycle.
// Repeated allocation/spawn/defer/calls/counter changes are not admitted here.
func (b *builder) proveReceiveLoops(f *ssa.Function, plan *functionPlan) bool {
	for _, component := range cslice.CyclicComponents(f) {
		members := map[*ssa.BasicBlock]bool{}
		for _, bb := range component {
			members[bb] = true
		}
		var receive *ssa.UnOp
		fail := func(message string) bool { b.diag("error", "receive-loop", message, f.Pos()); return false }
		for _, bb := range component {
			for _, i := range bb.Instrs {
				switch x := i.(type) {
				case *ssa.UnOp:
					if x.Op == token.ARROW {
						if !x.CommaOk || receive != nil {
							return fail("receive cycle requires exactly one comma-ok receive")
						}
						receive = x
					} else if x.Op != token.NOT && x.Op != token.MUL {
						return fail("unsupported unary operation in receive cycle")
					}
				case *ssa.Phi:
					if relevantType(x.Type()) {
						return fail("loop-carried synchronization identity unsupported")
					}
				case *ssa.Call:
					primitive, ok := plan.Discovery.Primitives[x]
					if !ok || (primitive.Kind != behavior.CloseChannel && primitive.Kind != behavior.Lock && primitive.Kind != behavior.Unlock) {
						return fail("receive cycles permit only direct close/Lock/Unlock calls; helper calls and counter changes are unsupported")
					}
				case *ssa.Send, *ssa.If, *ssa.Jump, *ssa.Extract, *ssa.BinOp, *ssa.Convert, *ssa.ChangeType, *ssa.FieldAddr, *ssa.DebugRef:
				default:
					return fail(fmt.Sprintf("%T cannot repeat in a receive cycle (allocation, spawn, defer and stores must remain acyclic)", i))
				}
			}
		}
		if receive == nil {
			return fail("cyclic computation lacks a close-driven receive")
		}
		if operand, ok := receive.X.(ssa.Instruction); ok && members[operand.Block()] {
			return fail("range channel must be evaluated outside the receive cycle")
		}
		header := receive.Block()
		branch, ok := header.Instrs[len(header.Instrs)-1].(*ssa.If)
		if !ok {
			return fail("receive loop header must branch directly on its completion status")
		}
		guard, ok := receiveStatusGuard(branch.Cond, map[ssa.Value]string{receive: "status"}, true)
		if !ok {
			return fail("receive loop exit must use exact comma-ok status")
		}
		inside := 0
		if guard.Negated {
			inside = 1
		}
		if !members[header.Succs[inside]] || members[header.Succs[1-inside]] {
			return fail("closed-empty status must exit the receive cycle")
		}
		for _, bb := range component {
			if !header.Dominates(bb) {
				return fail("receive cycle must have one dominating receive header")
			}
		}
		// Removing the header must break every cycle, not merely one back edge.
		color := map[*ssa.BasicBlock]int{}
		var cycle func(*ssa.BasicBlock) bool
		cycle = func(bb *ssa.BasicBlock) bool {
			if bb == header || !members[bb] {
				return false
			}
			if color[bb] == 1 {
				return true
			}
			if color[bb] == 2 {
				return false
			}
			color[bb] = 1
			for _, succ := range bb.Succs {
				if cycle(succ) {
					return true
				}
			}
			color[bb] = 2
			return false
		}
		for _, bb := range component {
			if cycle(bb) {
				return fail("receive cycle contains a subcycle that can bypass reception")
			}
		}
		b.diag("info", "finite-receive-loop", "Close-driven receive cycle preserves exact status and finite modeled state; no trip count or termination assumption", receive.Pos())
	}
	b.m.Metadata.Options["profile"] = []string{"finite-state-static-identity"}
	return true
}
