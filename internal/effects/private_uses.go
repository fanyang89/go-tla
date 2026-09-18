package effects

import "golang.org/x/tools/go/ssa"

// Rebuild ownership uses from current instructions. Referrers may be stale after
// another pass; an exhausted or incomplete inventory cannot establish privacy.
func privateCurrentUses(v ssa.Value) ([]ssa.Instruction, bool) {
	f := v.Parent()
	if f == nil {
		return nil, false
	}
	steps, found := 0, false
	var uses []ssa.Instruction
	for _, bb := range f.Blocks {
		for _, i := range bb.Instrs {
			steps++
			if steps > 4096 || i.Parent() != f || i.Block() != bb {
				return nil, false
			}
			if value, ok := i.(ssa.Value); ok && value == v {
				found = true
			}
			used := false
			for _, op := range i.Operands(nil) {
				steps++
				if steps > 4096 {
					return nil, false
				}
				if op != nil && *op == v {
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
