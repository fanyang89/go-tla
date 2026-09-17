package lowering

import (
	"slices"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/effects"
	"golang.org/x/tools/go/ssa"
)

func (b *builder) onceCall(fr *frame, site *ssa.Call, next string) string {
	proof := b.effects.ProveOnceCall(site)
	if proof == nil || fr.plan.Calls[site].Callee != b.p.CallTarget(site) {
		b.diag("error", "once-contract", "Once.Do lacks a current standard body/graph/callback proof", site.Pos())
		return next
	}
	for _, v := range proof.Operands {
		if !b.requireData(fr, v) {
			return next
		}
	}
	id := b.identity(fr, proof.Receiver, map[ssa.Value]bool{})
	if id == "nil" || id == "invalid" || !b.resources[id] {
		b.diag("error", "once-identity", "Once.Do requires a static non-nil Once identity", site.Pos())
		return next
	}
	bind := map[ssa.Value]string{}
	for i, v := range proof.Callback.FreeVars {
		if relevantType(v.Type()) || capturedChannel(v.Type()) || capturedSyncObject(v.Type()) {
			bind[v] = b.identity(fr, proof.Captures[i], map[ssa.Value]bool{})
		}
	}
	if !slices.Contains(b.m.Assumptions, effects.OnceModel) {
		b.m.Assumptions = append(b.m.Assumptions, effects.OnceModel)
	}
	pos := b.position(site.Pos(), fr.f)
	unlock := b.fresh(fr.process + "_once_unlock")
	done := b.fresh(fr.process + "_once_complete")
	check := b.fresh(fr.process + "_once_recheck")
	lock := b.fresh(fr.process + "_once_lock")
	entry := b.fresh(fr.process + "_once_fast")
	b.nodes[unlock] = &node{edges: []edge{{to: next, guard: behavior.Guard{Kind: behavior.True}, effects: []behavior.Effect{{Kind: behavior.Unlock, Resource: id + "_once_lock"}}, pos: pos}}}
	b.nodes[done] = &node{edges: []edge{{to: unlock, guard: behavior.Guard{Kind: behavior.True}, effects: []behavior.Effect{{Kind: behavior.AssignAbstractState, Variable: id + "_once_done", Value: 1}}, pos: pos}}}
	callback := b.function(proof.Callback, bind, fr.process, done)
	guards := func(value int) behavior.Guard {
		return behavior.Guard{Kind: behavior.Equal, Variable: id + "_once_done", Value: value}
	}
	b.nodes[check] = &node{edges: []edge{{to: unlock, guard: guards(1), pos: pos}, {to: callback, guard: guards(0), pos: pos}}}
	b.nodes[lock] = &node{edges: []edge{{to: check, guard: behavior.Guard{Kind: behavior.True}, effects: []behavior.Effect{{Kind: behavior.Lock, Resource: id + "_once_lock"}}, pos: pos}}}
	// Commit the atomic flag read before attempting the possibly blocking lock.
	// A later completion must not turn a previously selected slow path into a fast one.
	b.nodes[entry] = &node{edges: []edge{{to: next, guard: guards(1), pos: pos, boundary: true}, {to: lock, guard: guards(0), pos: pos, boundary: true}}}
	b.diag("info", "once-call", "modeled source-checked Once.Do with an analyzed callback and completion-before-return ordering", site.Pos())
	return entry
}
