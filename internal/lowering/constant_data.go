package lowering

import (
	"golang.org/x/tools/go/ssa"
	"slices"
)

func (b *builder) consumeConstantData(fr *frame, site *ssa.Call) bool {
	summary := b.effects.Call(site)
	proof := b.effects.ProveConstantData(site)
	valid := summary.Callee != nil && b.p.CallTarget(site) == summary.Callee && proof != nil
	if fr != nil {
		for _, operand := range site.Operands(nil) {
			if operand != nil && !b.requireData(fr, *operand) {
				valid = false
			}
		}
	}
	if !valid {
		b.diag("error", "constant-data-contract", "literal-input computation lacks a current argument/body/graph/slice proof", site.Pos())
		return false
	}
	for _, operation := range proof.ModeledOperations {
		if !slices.Contains(b.m.Assumptions, operation) {
			b.m.Assumptions = append(b.m.Assumptions, operation)
		}
		b.diag("info", "modeled-data-operation", operation, site.Pos())
	}
	b.diag("info", "constant-data-call", "bounded SSA evaluation proved this literal-input data call: "+summary.Callee.String()+"; result remains abstract", site.Pos())
	return true
}
