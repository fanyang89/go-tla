package effects

import (
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"
)

// Only untouched zero storage is admitted, not a synchronization operation or
// returned resource identity. Go allocation zeroes all fields before any store.
func dormantSyncType(t types.Type) bool {
	n, ok := types.Unalias(t).(*types.Named)
	return ok && n.Obj().Pkg() != nil && n.Obj().Pkg().Path() == "sync" && (n.Obj().Name() == "Mutex" || n.Obj().Name() == "WaitGroup" || n.Obj().Name() == "Once")
}

func dormantContainer(t types.Type, depth int, budget *int) bool {
	*budget--
	if depth > 64 || *budget < 0 {
		return false
	}
	if dormantSyncType(t) {
		return true
	}
	if n, ok := types.Unalias(t).(*types.Named); ok && n.Obj().Pkg() != nil {
		switch n.Obj().Pkg().Path() {
		case "sync", "sync/atomic", "internal/sync":
			return false
		}
	}
	switch x := t.Underlying().(type) {
	case *types.Struct:
		for i := range x.NumFields() {
			if !dormantContainer(x.Field(i).Type(), depth+1, budget) {
				return false
			}
		}
		return true
	case *types.Array:
		return dormantContainer(x.Elem(), depth+1, budget)
	default:
		return constructorValueType(t, depth)
	}
}

func privateDormantAllocation(root *ssa.Alloc) bool {
	p, ok := root.Type().Underlying().(*types.Pointer)
	budget := 256
	return ok && dormantContainer(p.Elem(), 0, &budget) && privateDormantUses(root, map[ssa.Value]bool{})
}

func privateDormantUses(v ssa.Value, seen map[ssa.Value]bool) bool {
	if seen[v] {
		return true
	}
	seen[v] = true
	p, ok := v.Type().Underlying().(*types.Pointer)
	// Even taking the address of a synchronization field is outside this proof.
	if !ok || dormantSyncType(p.Elem()) {
		return false
	}
	refs, complete := privateCurrentUses(v)
	if !complete {
		return false
	}
	for _, i := range refs {
		switch x := i.(type) {
		case *ssa.FieldAddr:
			st, ok := p.Elem().Underlying().(*types.Struct)
			if !ok || x.Field < 0 || x.Field >= st.NumFields() || !types.Identical(x.Type(), types.NewPointer(st.Field(x.Field).Type())) || x.X != v || !privateDormantUses(x, seen) {
				return false
			}
		case *ssa.IndexAddr:
			arr, ok := p.Elem().Underlying().(*types.Array)
			if !ok || !types.Identical(x.Type(), types.NewPointer(arr.Elem())) || x.X != v || !privateDormantUses(x, seen) {
				return false
			}
		case *ssa.Store:
			if x.Addr != v || x.Val == nil || x.Val == v || !constructorValueType(p.Elem(), 0) || !types.Identical(x.Val.Type(), p.Elem()) {
				return false
			}
		case *ssa.UnOp:
			if x.Op != token.MUL || x.X != v || !constructorValueType(x.Type(), 0) {
				return false
			}
		case *ssa.MakeInterface:
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
