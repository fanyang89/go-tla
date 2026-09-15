// Package abstract defines pass 6's value abstraction without backend syntax.
package abstract

import (
	"github.com/fanmi/go-tla/internal/behavior"
	"go/constant"
	"go/token"
	"golang.org/x/tools/go/ssa"
)

func Integer(v ssa.Value) (int, bool) {
	c, ok := v.(*ssa.Const)
	if !ok || c.Value == nil {
		return 0, false
	}
	n, ok := constant.Int64Val(c.Value)
	return int(n), ok && int64(int(n)) == n
}

// Predicate keeps select dispatch exact; all other unknown predicates are explicit choices.
func Predicate(v ssa.Value, selects map[*ssa.Select]string, truth bool) behavior.Guard {
	if c, ok := v.(*ssa.Const); ok && c.Value != nil && c.Value.Kind() == constant.Bool {
		if constant.BoolVal(c.Value) == truth {
			return behavior.Guard{Kind: behavior.True}
		}
		return behavior.Guard{Kind: behavior.True, Negated: true}
	}
	if b, ok := v.(*ssa.BinOp); ok && (b.Op == token.EQL || b.Op == token.NEQ) {
		for j := range 2 {
			a, z := b.X, b.Y
			if j == 1 {
				a, z = z, a
			}
			if e, ok := a.(*ssa.Extract); ok && e.Index == 0 {
				if s, ok := e.Tuple.(*ssa.Select); ok {
					if n, ok := Integer(z); ok {
						return behavior.Guard{Kind: behavior.Equal, Variable: selects[s], Value: n, Negated: (b.Op == token.NEQ) == truth}
					}
				}
			}
		}
	}
	return behavior.Guard{Kind: behavior.Choice}
}
