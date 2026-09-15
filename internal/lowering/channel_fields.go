package lowering

import (
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"
)

func inlineChannel(t types.Type) bool {
	switch u := t.Underlying().(type) {
	case *types.Chan:
		return true
	case *types.Array:
		return inlineChannel(u.Elem())
	case *types.Struct:
		for i := range u.NumFields() {
			if inlineChannel(u.Field(i).Type()) {
				return true
			}
		}
	}
	return false
}

func channelAggregate(t types.Type) bool {
	switch t.Underlying().(type) {
	case *types.Struct, *types.Array:
		return inlineChannel(t)
	}
	return false
}

type channelSlot struct {
	stores []*ssa.Store
	reads  []ssa.Instruction
}

// Channel fields are immutable after construction in their allocation's frame.
// Every syntactic field address and object escape is inspected, including uses
// outside the backward slice. Callees may read these bindings, never initialize
// or change them. Missing stores mean the Go zero-value nil channel.
func (b *builder) prepareChannelFields(fr *frame) {
	for _, bb := range fr.f.Blocks {
		for _, i := range bb.Instrs {
			alloc, ok := i.(*ssa.Alloc)
			if !ok || !aggregatePointer(alloc.Type()) || !inlineChannel(alloc.Type().Underlying().(*types.Pointer).Elem()) {
				continue
			}
			slots := map[string]*channelSlot{}
			var order []string
			var declare func(types.Type, string)
			declare = func(t types.Type, base string) {
				if !inlineChannel(t) {
					return
				}
				if s, ok := t.Underlying().(*types.Struct); ok {
					for index := range s.NumFields() {
						field := s.Field(index)
						id := b.fieldID(base, index)
						if _, ok := field.Type().Underlying().(*types.Chan); ok {
							slots[id] = &channelSlot{}
							order = append(order, id)
						} else {
							declare(field.Type(), id)
						}
					}
				}
			}
			declare(alloc.Type().Underlying().(*types.Pointer).Elem(), fr.ids[alloc])
			var escapes []ssa.Instruction
			seen := map[ssa.Value]bool{}
			var visit func(ssa.Value, string)
			visit = func(v ssa.Value, base string) {
				if seen[v] {
					return
				}
				seen[v] = true
				refs := v.Referrers()
				if refs == nil {
					return
				}
				for _, ref := range *refs {
					switch x := ref.(type) {
					case *ssa.FieldAddr:
						id := b.fieldID(base, x.Field)
						if slot := slots[id]; slot != nil {
							if uses := x.Referrers(); uses != nil {
								for _, use := range *uses {
									switch u := use.(type) {
									case *ssa.Store:
										if u.Addr == x {
											slot.stores = append(slot.stores, u)
										} else {
											b.diag("error", "channel-field-alias", "channel field address escapes", u.Pos())
										}
									case *ssa.UnOp:
										if u.Op != token.MUL {
											b.diag("error", "channel-field-alias", "unsupported channel field use", u.Pos())
										}
										slot.reads = append(slot.reads, u)
									case *ssa.DebugRef:
									default:
										b.diag("error", "channel-field-alias", "channel field address escapes", use.Pos())
									}
								}
							}
						} else if aggregatePointer(x.Type()) {
							visit(x, id)
						}
					case *ssa.Call, *ssa.Go, *ssa.MakeClosure:
						escapes = append(escapes, ref)
					case *ssa.DebugRef:
					default:
						b.diag("error", "channel-field-alias", "channel-bearing object requires direct allocation/field/receiver uses; copies, returned objects and pointer-cell aliases unsupported", ref.Pos())
					}
				}
			}
			visit(alloc, fr.ids[alloc])
			for _, id := range order {
				slot := slots[id]
				if len(slot.stores) == 0 {
					b.channelFields[id] = "nil"
					continue
				}
				if len(slot.stores) != 1 {
					b.diag("error", "channel-field-initialization", "channel field requires at most one immutable initialization", alloc.Pos())
					continue
				}
				store := slot.stores[0]
				valid := true
				for _, use := range append(slot.reads, escapes...) {
					if !instructionDominates(store, use) {
						b.diag("error", "channel-field-initialization", "channel field initialization must dominate every read and object call/capture/spawn", use.Pos())
						valid = false
					}
				}
				if !directChannelValue(fr, store.Val) {
					b.diag("error", "channel-field-initialization", "channel field initializer requires a direct channel allocation, parameter, capture or nil", store.Pos())
					valid = false
				}
				if !valid || !b.requireData(fr, store.Val) {
					continue
				}
				b.channelFields[id] = b.identity(fr, store.Val, map[ssa.Value]bool{})
				fr.fieldStores[store] = true
			}
		}
	}
}

func directChannelValue(fr *frame, v ssa.Value) bool {
	switch x := v.(type) {
	case *ssa.MakeChan, *ssa.Parameter, *ssa.FreeVar:
		return true
	case *ssa.Const:
		return x.IsNil()
	case *ssa.UnOp:
		// Captured channel cells were already proved immutable/dominating when
		// bound into this frame. Do not infer arbitrary memory/field loads.
		return x.Op == token.MUL && fr.ids[x.X] != "" && fr.ids[x.X] != "invalid"
	case *ssa.ChangeType:
		return directChannelValue(fr, x.X)
	case *ssa.Convert:
		return directChannelValue(fr, x.X)
	}
	return false
}
