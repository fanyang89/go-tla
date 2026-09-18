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
			data = constructorValueType(p.Elem(), 0)
		}
	}
	proved := data && privateAddressUses(root, map[ssa.Value]bool{})
	if !proved && !referenceData {
		proved = privateDormantAllocation(root)
	}
	cache[root] = proved
	return proved
}

// Copying an opaque reference/header into fresh private storage does not grant
// ownership of its pointee, backing array, map entries or callback body. The
// address proof never follows loads; calls and updates need independent proofs.
// Synchronization state itself still cannot acquire purity through value copies.
func constructorValueType(t types.Type, depth int) bool {
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
	case *types.Pointer, *types.Slice, *types.Map, *types.Chan, *types.Signature, *types.Interface:
		return true
	case *types.Array:
		return constructorValueType(t.Elem(), depth+1)
	case *types.Struct:
		for i := range t.NumFields() {
			if !constructorValueType(t.Field(i).Type(), depth+1) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// This is only a proof-selection hint, never an ownership or effect proof.
func hasReferenceStore(f *ssa.Function) bool {
	var reference func(types.Type, int) bool
	reference = func(t types.Type, depth int) bool {
		if depth > 64 {
			return true
		}
		switch x := t.Underlying().(type) {
		case *types.Pointer, *types.Interface, *types.Slice, *types.Map, *types.Signature, *types.Chan:
			return true
		case *types.Struct:
			for i := range x.NumFields() {
				if reference(x.Field(i).Type(), depth+1) {
					return true
				}
			}
		case *types.Array:
			return reference(x.Elem(), depth+1)
		}
		return false
	}
	for _, bb := range f.Blocks {
		for _, i := range bb.Instrs {
			s, ok := i.(*ssa.Store)
			if !ok {
				continue
			}
			v := s.Addr
		walk:
			for {
				switch x := v.(type) {
				case *ssa.FieldAddr:
					v = x.X
				case *ssa.IndexAddr:
					v = x.X
				default:
					break walk
				}
			}
			if a, ok := v.(*ssa.Alloc); ok {
				if p, ok := a.Type().Underlying().(*types.Pointer); ok && reference(p.Elem(), 0) {
					return true
				}
			}
		}
	}
	return false
}

func privateAddressUses(v ssa.Value, seen map[ssa.Value]bool) bool {
	if seen[v] {
		return true
	}
	seen[v] = true
	refs, complete := privateCurrentUses(v)
	if !complete {
		return false
	}
	for _, use := range refs {
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
	refs, complete := privateCurrentUses(v)
	if !complete {
		return false
	}
	for _, use := range refs {
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
