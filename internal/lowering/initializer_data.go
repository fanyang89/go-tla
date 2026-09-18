package lowering

import (
	"slices"

	"golang.org/x/tools/go/ssa"
)

func (b *builder) recordInitializerData(f *ssa.Function) {
	const assumption = "Finite data-only package initialization completes before application execution; ordinary data writes and map operations are abstract, not runtime purity or payload/input-bound proofs. Implicit sequential panics and resource exhaustion remain excluded."
	if !slices.Contains(b.m.Assumptions, assumption) {
		b.m.Assumptions = append(b.m.Assumptions, assumption)
	}
	b.diag("info", "initializer-data", "proved finite data-only initialization with current body/type/graph checks: "+f.String(), f.Pos())
}
