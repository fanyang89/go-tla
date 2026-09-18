package effects

import (
	"go/constant"
	"go/types"

	cslice "github.com/fanmi/go-tla/internal/slice"
	"golang.org/x/tools/go/ssa"
)

// ProveInitializerData is NOT a purity proof. It admits finite sequential data
// writes only for a consumer already proving startup before application execution.
// All transitive operations/types must exclude communication, callbacks, unsafe
// pointers and interfaces. Runtime shared writes must never consume this proof.
func (a *Analyzer) ProveInitializerData(f *ssa.Function) bool {
	if f == nil || !cslice.HasCycle(f) {
		return false
	}
	p := finiteDataProof{program: a.program, remaining: maxFiniteDataProofSteps, visiting: map[*ssa.Function]bool{}, proved: map[*ssa.Function]bool{}, initializing: true}
	return p.function(f)
}

func portableIntType(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Kind() == types.Int
}

func nonnegativeIntConstant(v ssa.Value) bool {
	c, ok := v.(*ssa.Const)
	if !ok || !portableIntType(c.Type()) || c.Value == nil || c.Value.Kind() != constant.Int {
		return false
	}
	n, ok := constant.Int64Val(c.Value)
	return ok && n >= 0 && n <= 1<<31-1
}
