package lowering

import (
	"maps"
	"slices"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/discovery"
	"golang.org/x/tools/go/ssa"
)

const maxDeferSites = 64

type deferredCall struct {
	flag     string
	effect   behavior.Effect
	source   behavior.Position
	callee   *ssa.Function
	bindings map[ssa.Value]string
}

// Registration/drain sites must be acyclic, even when receive-only SCCs exist.
// Their reverse-postorder is a linear extension of executable registration order.
// Reversing the registered subset implements LIFO without assuming a stack bound.
// Each invocation owns fresh flags and statically captured resource identities.
func (b *builder) prepareDefers(fr *frame) bool {
	blocks := normalBlockOrder(fr.f)
	var order []*ssa.Defer
	for _, bb := range blocks {
		for _, i := range bb.Instrs {
			if d, ok := i.(*ssa.Defer); ok {
				order = append(order, d)
			}
		}
	}
	if len(order) > maxDeferSites {
		b.diag("error", "defer-site-limit", "more than 64 defer sites per invocation unsupported; no cleanup is truncated", fr.f.Pos())
		return false
	}
	for _, d := range order {
		call := deferredCall{source: b.position(d.Pos(), fr.f)}
		target := fr.plan.Calls[d].Callee
		if primitive, ok := discovery.Deferred(d); ok {
			if target == nil || b.p.CallTarget(d) != target {
				b.diag("error", "call-contract", "deferred primitive lacks a matching call-graph proof", d.Pos())
				return false
			}
			if !b.requireData(fr, primitive.Resource) {
				return false
			}
			resource := b.identity(fr, primitive.Resource, map[ssa.Value]bool{})
			if resource == "nil" || resource == "invalid" {
				b.diag("error", "sync-identity", "deferred cleanup requires a static non-nil synchronization identity", d.Pos())
				return false
			}
			call.effect = behavior.Effect{Kind: primitive.Kind, Resource: resource}
		} else {
			// The ordinary callee binder rechecks exact direct, boxed-interface
			// and immutable-field proofs and captures identities at registration.
			// Cleanup executes the source body, never a trusted-body shortcut.
			if d.DeferStack != nil || target == nil || (target.Synthetic != "" && b.p.BoundMethodClosure(d.Common().Value, d) == nil) ||
				discovery.SyncTypeReceiver(target) != "" {
				b.diag("error", "unsupported-defer", "defer requires direct Unlock/Done or an exactly bound source-defined helper on this invocation's stack", d.Pos())
				return false
			}
			for _, operand := range d.Operands(nil) {
				if operand != nil && !b.requireData(fr, *operand) {
					return false
				}
			}
			call.callee, call.bindings = b.callee(fr, d, target, d.Pos())
			if call.callee == nil {
				return false
			}
		}
		flag := b.fresh(fr.process + "_defer_registered")
		call.flag = flag
		fr.defers[d] = call
		for pi := range b.m.Processes {
			if b.m.Processes[pi].ID == fr.process {
				b.m.Processes[pi].Locals = append(b.m.Processes[pi].Locals, behavior.Variable{Name: flag, Domain: []int{0, 1}, Initial: 0})
			}
		}
	}
	// A may-pending analysis chooses which sites need tests at each drain. Runtime
	// flags retain exact conditional registration; joins must not assume all sites ran.
	out := map[*ssa.BasicBlock]map[*ssa.Defer]bool{}
	transfer := func(bb *ssa.BasicBlock, emit bool) map[*ssa.Defer]bool {
		pending := map[*ssa.Defer]bool{}
		for _, pred := range bb.Preds {
			maps.Copy(pending, out[pred])
		}
		for _, i := range bb.Instrs {
			switch x := i.(type) {
			case *ssa.Defer:
				pending[x] = true
			case *ssa.RunDefers, *ssa.Return:
				if emit {
					for _, d := range order {
						if pending[d] {
							fr.cleanup[i] = append(fr.cleanup[i], fr.defers[d])
						}
					}
				}
				clear(pending)
			}
		}
		return pending
	}
	// Monotone union/kill transfer reaches a fixed point across receive SCCs.
	for changed := true; changed; {
		changed = false
		for _, bb := range blocks {
			pending := transfer(bb, false)
			if !maps.Equal(pending, out[bb]) {
				out[bb] = pending
				changed = true
			}
		}
	}
	for _, bb := range blocks {
		transfer(bb, true)
	}
	return true
}

func normalBlockOrder(f *ssa.Function) []*ssa.BasicBlock {
	seen := map[*ssa.BasicBlock]bool{}
	var order []*ssa.BasicBlock
	var visit func(*ssa.BasicBlock)
	visit = func(bb *ssa.BasicBlock) {
		if seen[bb] {
			return
		}
		seen[bb] = true
		for _, next := range bb.Succs {
			visit(next)
		}
		order = append(order, bb)
	}
	visit(f.Blocks[0])
	slices.Reverse(order)
	return order
}

func (b *builder) cleanupChain(fr *frame, site ssa.Instruction, next string) string {
	// Prepending in registration order executes the latest pending registration first.
	for _, d := range fr.cleanup[site] {
		id := b.fresh(fr.process + "_defer_cleanup")
		target := next
		effects := []behavior.Effect{{Kind: behavior.AssignAbstractState, Variable: d.flag, Value: 0}}
		if d.callee != nil {
			// The caller cannot drain earlier defers until this body returns.
			// Arguments/captured identities were bound at registration, not here.
			target = b.function(d.callee, d.bindings, fr.process, next)
		} else {
			effects = append([]behavior.Effect{d.effect}, effects...)
		}
		b.nodes[id] = &node{edges: []edge{
			{to: target, guard: behavior.Guard{Kind: behavior.Equal, Variable: d.flag, Value: 1},
				effects: effects, pos: d.source},
			{to: next, guard: behavior.Guard{Kind: behavior.Equal, Variable: d.flag, Value: 0}, pos: d.source},
		}}
		next = id
	}
	return next
}
