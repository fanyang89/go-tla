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
	return ok && discovery.SyncType(types.Unalias(p.Elem())) == "" && (inlineSync(p.Elem()) || inlineChannel(p.Elem()))
}

// Captured pointer cells are resolved by identity's unique dominating-store
// proof, not treated as fresh objects. Channel-bearing objects keep their stricter
// allocation-frame escape rules; this capability is for inline synchronization.
func capturedSyncObject(t types.Type) bool {
	cell, ok := t.Underlying().(*types.Pointer)
	if !ok || !aggregatePointer(cell.Elem()) {
		return false
	}
	object := cell.Elem().Underlying().(*types.Pointer).Elem()
	return inlineSync(object) && !inlineChannel(object)
}

// Only addressable inline fields inherit a concrete object's identity. Channel
// bindings require a separately proved immutable initialization.
func (b *builder) fieldIdentity(fr *frame, x *ssa.FieldAddr, seen map[ssa.Value]bool) string {
	if !aggregatePointer(x.X.Type()) {
		return ""
	}
	p := x.X.Type().Underlying().(*types.Pointer)
	field := p.Elem().Underlying().(*types.Struct).Field(x.Field)
	if !inlineSync(field.Type()) && !inlineChannel(field.Type()) {
		return ""
	}
	base := b.identity(fr, x.X, seen)
	if base == "nil" || base == "invalid" {
		b.diag("error", "sync-identity", "synchronization field requires a non-nil static object", x.Pos())
		return "invalid"
	}
	id := b.fieldID(base, x.Field)
	if _, ok := field.Type().Underlying().(*types.Chan); ok {
		if channel := b.channelFields[id]; channel != "" {
			return channel
		}
		b.diag("error", "channel-field-initialization", "channel field lacks a proved allocation-frame initialization", x.Pos())
		return "invalid"
	}
	if typ := discovery.SyncType(types.Unalias(field.Type())); typ != "" {
		if typ != "Mutex" && typ != "WaitGroup" && typ != "Once" {
			b.diag("error", "sync-type", "unsupported sync field type "+typ, x.Pos())
		} else {
			b.addSync(id, typ)
		}
	}
	return id
}

func (b *builder) fieldID(base string, index int) string {
	key := fmt.Sprintf("%s/%d", base, index)
	if b.fields[key] == "" {
		b.fields[key] = b.fresh(fmt.Sprintf("%s_field_%d", base, index))
	}
	return b.fields[key]
}
