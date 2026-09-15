package frontend

import (
	"go/types"

	"golang.org/x/tools/go/ssa"
)

// UsesUnsafePointer is shared by call-effect analysis and reachable lowering.
func UsesUnsafePointer(i ssa.Instruction) bool {
	if v, ok := i.(ssa.Value); ok && unsafePointerType(v.Type()) {
		return true
	}
	for _, operand := range i.Operands(nil) {
		if operand != nil && *operand != nil && unsafePointerType((*operand).Type()) {
			return true
		}
	}
	return false
}
func unsafePointerType(t types.Type) bool {
	seen := map[types.Type]bool{}
	for t != nil && !seen[t] {
		seen[t] = true
		switch u := t.Underlying().(type) {
		case *types.Basic:
			return u.Kind() == types.UnsafePointer
		case *types.Pointer:
			t = u.Elem()
		default:
			return false
		}
	}
	return false
}
