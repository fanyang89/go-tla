package lowering

import (
	"go/types"

	"golang.org/x/tools/go/ssa"
)

// Unsafe pointer operations can invalidate both synchronization identity and
// state. They are outside the supported subset, not sequential panic assumptions.
func usesUnsafePointer(i ssa.Instruction) bool {
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
func (b *builder) checkUnsafePointer(i ssa.Instruction) {
	if usesUnsafePointer(i) {
		b.diag("error", "unsafe-pointer", "unsafe pointer operations may mutate synchronization identity or state; unsupported", i.Pos())
	}
}
