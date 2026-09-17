package lowering

import (
	"github.com/fanmi/go-tla/internal/frontend"
	"golang.org/x/tools/go/ssa"
)

type callableFieldKey struct {
	object string
	field  int
}
type callableFieldBinding struct {
	initializer *ssa.Store
	receiverID  string
	captures    map[ssa.Value]string
}

// Bind values in the allocation frame, not in the later reader's frame. A shared
// SSA constructor invoked twice must not merge its two receivers or captures.
func (b *builder) prepareCallableFields(fr *frame) {
	if b.callableFields == nil {
		b.callableFields = map[callableFieldKey]callableFieldBinding{}
	}
	for _, bb := range fr.f.Blocks {
		for _, i := range bb.Instrs {
			store, ok := i.(*ssa.Store)
			if !ok {
				continue
			}
			value, root := b.p.CallableInitializer(store)
			if root == nil || fr.ids[root] == "" {
				continue
			}
			if !b.requireData(fr, store.Val) {
				continue
			}
			field := store.Addr.(*ssa.FieldAddr)
			binding := callableFieldBinding{initializer: store, captures: map[ssa.Value]string{}}
			switch x := value.(type) {
			case *ssa.MakeInterface:
				if relevantType(x.X.Type()) {
					if !b.requireData(fr, x.X) {
						continue
					}
					binding.receiverID = b.identity(fr, x.X, map[ssa.Value]bool{})
				}
			case *ssa.MakeClosure:
				f, ok := x.Fn.(*ssa.Function)
				if !ok {
					continue
				}
				for j, v := range f.FreeVars {
					if relevantType(v.Type()) || capturedChannel(v.Type()) || capturedSyncObject(v.Type()) {
						if !b.requireData(fr, x.Bindings[j]) {
							continue
						}
						binding.captures[v] = b.identity(fr, x.Bindings[j], map[ssa.Value]bool{})
					}
				}
			}
			b.callableFields[callableFieldKey{fr.ids[root], field.Field}] = binding
		}
	}
}

func (b *builder) callableBinding(fr *frame, site ssa.CallInstruction, proof *frontend.FieldCallProof) (callableFieldBinding, bool) {
	if !b.requireData(fr, site.Common().Value) || !b.requireData(fr, proof.Object) {
		return callableFieldBinding{}, false
	}
	object := b.identity(fr, proof.Object, map[ssa.Value]bool{})
	binding, ok := b.callableFields[callableFieldKey{object, proof.Field}]
	if !ok || binding.initializer != proof.Initializer || object == "nil" || object == "invalid" {
		b.diag("error", "callable-field-binding", "callable field lacks its proved allocation-frame binding", site.Pos())
		return callableFieldBinding{}, false
	}
	return binding, true
}
