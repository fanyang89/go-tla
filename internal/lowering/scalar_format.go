package lowering

import (
	"slices"

	"github.com/fanmi/go-tla/internal/effects"
	"golang.org/x/tools/go/ssa"
)

func (b *builder) consumeScalarFormat(fr *frame, site *ssa.Call) bool {
	proof := b.effects.ProveScalarFormat(site)
	summary := b.effects.Call(site)
	valid := proof != nil && summary.Kind == effects.ScalarFormat && summary.Callee != nil && b.p.CallTarget(site) == summary.Callee
	if proof != nil && fr != nil {
		if fr.plan.Calls[site].Callee != summary.Callee {
			valid = false
		}
		for _, v := range proof.Operands {
			if !b.requireData(fr, v) {
				valid = false
			}
		}
		for _, store := range proof.Stores {
			if !fr.plan.Slice.Roots[store] {
				valid = false
			}
		}
	}
	if !valid {
		b.diag("error", "scalar-format-contract", "scalar formatting lacks a current wrapper/graph/private-argument/slice proof", site.Pos())
		return false
	}
	if !slices.Contains(b.m.Assumptions, effects.ScalarFormatModel) {
		b.m.Assumptions = append(b.m.Assumptions, effects.ScalarFormatModel)
	}
	b.diag("info", "modeled-data-operation", effects.ScalarFormatModel, site.Pos())
	b.diag("info", "scalar-format-call", "modeled basic-scalar "+summary.Callee.String()+"; arguments evaluated and result remains abstract", site.Pos())
	return true
}
