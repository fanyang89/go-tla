package lowering

import "golang.org/x/tools/go/ssa"

// Inspect current operands rather than trusting cached SSA Referrers. Exhaustion
// refuses the identity proof; it never truncates the use inventory.
func channelObjectUses(v ssa.Value) ([]ssa.Instruction, bool) {
	f := v.Parent()
	if f == nil {
		return nil, false
	}
	var uses []ssa.Instruction
	steps, found := 0, false
	for _, bb := range f.Blocks {
		for _, i := range bb.Instrs {
			steps++
			if steps > 4096 || i.Parent() != f {
				return nil, false
			}
			if value, ok := i.(ssa.Value); ok && value == v {
				found = true
			}
			used := false
			for _, operand := range i.Operands(nil) {
				steps++
				if steps > 4096 {
					return nil, false
				}
				if operand != nil && *operand == v {
					used = true
				}
			}
			if used {
				uses = append(uses, i)
			}
		}
	}
	return uses, found
}

// A box is an escape boundary: only exact receiver calls are admitted here.
// Arguments, stores, returns, phis and closure captures require separate proofs.
func (b *builder) channelReceiverBox(box *ssa.MakeInterface) bool {
	uses, ok := channelObjectUses(box)
	if !ok {
		return false
	}
	for _, use := range uses {
		if _, ok := use.(*ssa.DebugRef); ok {
			continue
		}
		site, ok := use.(ssa.CallInstruction)
		if !ok || !site.Common().IsInvoke() || site.Common().Value != box || b.p.CallTarget(site) == nil || b.p.InvokeReceiver(site) != box.X {
			return false
		}
		for _, arg := range site.Common().Args {
			if arg == box {
				return false
			}
		}
	}
	return true
}
