package lowering

import (
	"go/constant"
	"go/token"

	"github.com/fanmi/go-tla/internal/behavior"
	"golang.org/x/tools/go/ssa"
)

// Only the status component is retained; received payloads remain abstract.
func (b *builder) prepareReceiveStatus(fr *frame, i ssa.Instruction) {
	var tuple ssa.Value
	switch x := i.(type) {
	case *ssa.UnOp:
		if x.Op == token.ARROW && x.CommaOk {
			tuple = x
		}
	case *ssa.Select:
		tuple = x
	}
	if tuple == nil || tuple.Referrers() == nil {
		return
	}
	used := false
	for _, ref := range *tuple.Referrers() {
		if extract, ok := ref.(*ssa.Extract); ok && extract.Index == 1 {
			used = true
		}
	}
	if !used {
		return
	}
	name := b.fresh(fr.process + "_receive_ok")
	fr.receiveStatus[tuple] = name
	for j := range b.m.Processes {
		if b.m.Processes[j].ID == fr.process {
			b.m.Processes[j].Locals = append(b.m.Processes[j].Locals, behavior.Variable{Name: name, Domain: []int{0, 1}, Initial: 0})
		}
	}
}

// This deliberately does not infer through Phi, shared memory or call results.
// Unhandled Boolean computations keep the existing conservative abstraction.
func receiveStatusGuard(v ssa.Value, flags map[ssa.Value]string, truth bool) (behavior.Guard, bool) {
	if x, ok := v.(*ssa.Extract); ok && x.Index == 1 {
		if flag := flags[x.Tuple]; flag != "" {
			return behavior.Guard{Kind: behavior.Equal, Variable: flag, Value: 1, Negated: !truth}, true
		}
	}
	if x, ok := v.(*ssa.UnOp); ok && x.Op == token.NOT {
		return receiveStatusGuard(x.X, flags, !truth)
	}
	if x, ok := v.(*ssa.BinOp); ok && (x.Op == token.EQL || x.Op == token.NEQ) {
		for j := range 2 {
			a, z := x.X, x.Y
			if j == 1 {
				a, z = z, a
			}
			if c, ok := z.(*ssa.Const); ok && c.Value != nil && c.Value.Kind() == constant.Bool {
				want := truth == constant.BoolVal(c.Value)
				if x.Op == token.NEQ {
					want = !want
				}
				return receiveStatusGuard(a, flags, want)
			}
		}
	}
	return behavior.Guard{}, false
}
