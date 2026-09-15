package lowering

import (
	"fmt"
	"go/types"

	"github.com/fanmi/go-tla/internal/discovery"
	"golang.org/x/tools/go/ssa"
)

// inlineSync follows value containment, never pointers. Copying a struct/array
// containing synchronization state is rejected even if that state is not used.
func inlineSync(t types.Type) bool {
	switch u := t.Underlying().(type) {
	case *types.Pointer:
		return false
	case *types.Array:
		return inlineSync(u.Elem())
	case *types.Struct:
		if discovery.SyncType(types.Unalias(t)) != "" {
			return true
		}
		for i := range u.NumFields() {
			if inlineSync(u.Field(i).Type()) {
				return true
			}
		}
	}
	return false
}
func aggregatePointer(t types.Type) bool {
	p, ok := t.Underlying().(*types.Pointer)
	if !ok {
		return false
	}
	_, ok = p.Elem().Underlying().(*types.Struct)
	return ok && discovery.SyncType(types.Unalias(p.Elem())) == "" && inlineSync(p.Elem())
}

// Only addressable inline fields inherit a concrete object's identity. Pointer,
// channel, container and loaded aggregate identities are not inferred here.
func (b *builder) fieldIdentity(fr *frame, x *ssa.FieldAddr, seen map[ssa.Value]bool) string {
	if !aggregatePointer(x.X.Type()) {
		return ""
	}
	p := x.X.Type().Underlying().(*types.Pointer)
	field := p.Elem().Underlying().(*types.Struct).Field(x.Field)
	if !inlineSync(field.Type()) {
		return ""
	}
	base := b.identity(fr, x.X, seen)
	if base == "nil" || base == "invalid" {
		b.diag("error", "sync-identity", "synchronization field requires a non-nil static object", x.Pos())
		return "invalid"
	}
	key := fmt.Sprintf("%s/%d", base, x.Field)
	id := b.fields[key]
	if id == "" {
		id = b.fresh(fmt.Sprintf("%s_field_%d", base, x.Field))
		b.fields[key] = id
		if typ := discovery.SyncType(types.Unalias(field.Type())); typ != "" {
			if typ != "Mutex" && typ != "WaitGroup" {
				b.diag("error", "sync-type", "unsupported sync field type "+typ, x.Pos())
			} else {
				b.addSync(id, typ)
			}
		}
	}
	return id
}
