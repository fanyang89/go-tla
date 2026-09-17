package lowering

import (
	"github.com/fanmi/go-tla/internal/discovery"
	"github.com/fanmi/go-tla/internal/effects"
	cslice "github.com/fanmi/go-tla/internal/slice"
	"golang.org/x/tools/go/ssa"
)

// functionPlan is cached per function, independent of per-invocation identities.
// Its roots, calls, process entries and backward dependencies are consumed below.
type functionPlan struct {
	Discovery discovery.Function
	Calls     map[ssa.CallInstruction]effects.Summary
	Entries   map[*ssa.Go]effects.Summary
	Slice     cslice.Result
	Cyclic    bool
}

func (b *builder) plan(f *ssa.Function) *functionPlan {
	if plan := b.plans[f]; plan != nil {
		return plan
	}
	p := &functionPlan{Discovery: discovery.Scan(f), Calls: map[ssa.CallInstruction]effects.Summary{}, Entries: map[*ssa.Go]effects.Summary{}, Cyclic: cslice.HasCycle(f)}
	for _, bb := range f.Blocks {
		for _, i := range bb.Instrs {
			if call, ok := i.(ssa.CallInstruction); ok {
				summary := b.effects.Call(call)
				p.Calls[call] = summary
				if !summary.IsPure() || call.Common().IsInvoke() && summary.Callee != nil || b.p.CallableField(call) != nil {
					// A resolved invoke still needs its box/receiver in the retained
					// data slice, including when its body is local computation.
					p.Discovery.Roots[i] = true
				}
			}
		}
	}
	for _, goCall := range p.Discovery.Goroutines {
		p.Entries[goCall] = p.Calls[goCall]
	}
	p.Slice = cslice.FromRoots(f, p.Discovery.Roots)
	b.plans[f] = p
	return p
}

// requireData is a fail-closed pass boundary: identity/capacity/control extraction
// must never bypass the retained dependency slice for a behavioral root.
func (b *builder) requireData(fr *frame, v ssa.Value) bool {
	if v == nil || fr.plan.Slice.Data[v] {
		return true
	}
	b.diag("error", "slice-contract", "behavioral operand is missing from the retained data slice", v.Pos())
	return false
}
