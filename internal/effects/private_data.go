package effects

import (
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"
)

// Discarding a sequential builtin's runtime data operations during communication
// lowering is not a proof that a containing helper has no external effects.
func readOnlyBuiltin(c *ssa.CallCommon) bool {
	b, ok := c.Value.(*ssa.Builtin)
	if !ok {
		return false
	}
	switch b.Name() {
	case "len", "cap", "min", "max", "complex", "real", "imag":
		return true
	default:
		return false
	}
}

// privateDataStore proves that a store only initializes or updates a fresh data
// allocation owned by this invocation. Heap allocation is not itself a shared
// effect: Go's escape analysis also marks pointers returned to callers as heap.
// Publication through anything other than a return is conservatively rejected.
func privateDataStore(store *ssa.Store, cache map[*ssa.Alloc]bool) bool {
	return privateTypedStore(store, cache, false)
}

// privateFiniteDataStore permits local value copies of reference-bearing data.
// Walking the destination stops at a load: writing a copied pointer's pointee or
// a copied slice's backing array cannot inherit ownership of its local header.
// Its caller additionally checks every operand and transitive operation using
// the finite-data whitelist; this is not a general constructor purity rule.
func privateFiniteDataStore(store *ssa.Store, cache map[*ssa.Alloc]bool) bool {
	return privateTypedStore(store, cache, true)
}

func privateTypedStore(store *ssa.Store, cache map[*ssa.Alloc]bool, referenceData bool) bool {
	addr := store.Addr
walk:
	for {
		switch x := addr.(type) {
		case *ssa.FieldAddr:
			addr = x.X
		case *ssa.IndexAddr:
			addr = x.X
		default:
			break walk
		}
	}
	root, ok := addr.(*ssa.Alloc)
	if !ok || root.Parent() != store.Parent() {
		return false
	}
	if proved, ok := cache[root]; ok {
		return proved
	}
	p, ok := root.Type().Underlying().(*types.Pointer)
	data := false
	if ok {
		if referenceData {
			data = finiteDataType(p.Elem(), 0)
		} else {
			data = plainData(p.Elem(), 0)
		}
	}
	proved := data && privateAddressUses(root, map[ssa.Value]bool{})
	cache[root] = proved
	return proved
}

// Only scalars, value structs and fixed arrays qualify. Reference-bearing fields, opaque
// containers and synchronization state cannot acquire purity through this proof.
func plainData(t types.Type, depth int) bool {
	if depth > 64 {
		return false
	}
	if n, ok := types.Unalias(t).(*types.Named); ok && n.Obj().Pkg() != nil {
		switch n.Obj().Pkg().Path() {
		case "sync", "sync/atomic", "internal/sync":
			return false
		}
	}
	switch t := t.Underlying().(type) {
	case *types.Basic:
		return t.Info()&(types.IsBoolean|types.IsInteger|types.IsFloat|types.IsComplex|types.IsString) != 0
	case *types.Array:
		return plainData(t.Elem(), depth+1)
	case *types.Struct:
		for i := range t.NumFields() {
			if !plainData(t.Field(i).Type(), depth+1) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func privateAddressUses(v ssa.Value, seen map[ssa.Value]bool) bool {
	if seen[v] {
		return true
	}
	seen[v] = true
	refs := v.Referrers()
	if refs == nil {
		return false
	}
	for _, use := range *refs {
		switch x := use.(type) {
		case *ssa.FieldAddr:
			if x.X != v || !privateAddressUses(x, seen) {
				return false
			}
		case *ssa.IndexAddr:
			if x.X != v || !privateAddressUses(x, seen) {
				return false
			}
		case *ssa.Store:
			if x.Addr != v || x.Val == v {
				return false
			}
		case *ssa.UnOp:
			if x.Op != token.MUL || x.X != v {
				return false
			}
		case *ssa.MakeInterface:
			// Boxing does not copy a pointer's pointee. Only returning that box is
			// admitted, never calling through it or publishing it before return.
			if !returnedBox(x, map[ssa.Value]bool{}) {
				return false
			}
		case *ssa.Return, *ssa.DebugRef:
		default:
			return false
		}
	}
	return true
}

func returnedBox(v ssa.Value, seen map[ssa.Value]bool) bool {
	if seen[v] {
		return true
	}
	seen[v] = true
	refs := v.Referrers()
	if refs == nil {
		return false
	}
	for _, use := range *refs {
		switch x := use.(type) {
		case *ssa.Return, *ssa.DebugRef:
		case *ssa.ChangeInterface:
			if !returnedBox(x, seen) {
				return false
			}
		default:
			return false
		}
	}
	return true
}
