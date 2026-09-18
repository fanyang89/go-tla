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
	proof := a.ProveInitializerDataEffects(f)
	// The boolean API cannot convey operation assumptions; never drop them.
	return proof != nil && len(proof.ModeledOperations) == 0
}

type InitializerDataProof struct{ ModeledOperations []string }

// ProveInitializerDataEffects additionally carries every assumed operation model
// needed by the transitive proof. Consumers must retain these assumptions. Only
// source-checked scalar formatting's private argument boxes gain an interface
// exception; argument evaluation and all other instructions remain checked.
func (a *Analyzer) ProveInitializerDataEffects(f *ssa.Function) *InitializerDataProof {
	if f == nil || !cslice.HasCycle(f) {
		return nil
	}
	p := finiteDataProof{program: a.program, remaining: maxFiniteDataProofSteps, visiting: map[*ssa.Function]bool{}, proved: map[*ssa.Function]bool{}, initializing: true}
	if !p.function(f) {
		return nil
	}
	proof := &InitializerDataProof{}
	if p.scalarFormatting {
		proof.ModeledOperations = append(proof.ModeledOperations, ScalarFormatModel)
	}
	return proof
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
