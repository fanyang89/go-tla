package lowering

import "golang.org/x/tools/go/ssa"

func (b *builder) consumeConstantData(fr *frame, site *ssa.Call) bool {
	summary := b.effects.Call(site)
	valid := summary.Callee != nil && b.p.CallTarget(site) == summary.Callee && b.effects.ProveConstantDataCall(site)
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
	b.diag("info", "constant-data-call", "bounded SSA evaluation proved this literal-input data call: "+summary.Callee.String()+"; result remains abstract", site.Pos())
	return true
}
