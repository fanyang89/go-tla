package effects

import (
	"go/types"
	"slices"

	"golang.org/x/tools/go/ssa"
)

const ReflectionLiteralTypeModel = "Standard Go reflect.rtypeOf reads immutable ABI type metadata from a boxed literal; runtime ABI/type metadata correctness is assumed, not proved. This model neither admits general interface boxing/dispatch nor reads application memory. Returned metadata remains abstract."

// literalRuntimeType admits only a source-bound standard wrapper around the
// already checked ABI extraction chain. It never creates an interpreter-level
// boxed application value or executable runtime pointer.
func (e *dataEval) literalRuntimeType(site *ssa.Call) (types.Type, bool) {
	if site == nil || site.Common().IsInvoke() || len(site.Common().Args) != 1 {
		return nil, false
	}
	f := e.program.CallTarget(site)
	if f != site.Common().StaticCallee() || !e.standardFunction(f, "reflect", "rtypeOf") || len(f.Params) != 1 || f.Signature.Results().Len() != 1 || !pointerNamed(f.Signature.Results().At(0).Type(), "internal/abi", "Type") || !types.Identical(site.Type(), f.Signature.Results().At(0).Type()) {
		return nil, false
	}
	param, ok := f.Params[0].Type().Underlying().(*types.Interface)
	if !ok || !param.Empty() {
		return nil, false
	}
	box, ok := site.Common().Args[0].(*ssa.MakeInterface)
	if !ok || box.Parent() != site.Parent() || box.Block() == nil || site.Block() == nil || !box.Block().Dominates(site.Block()) || !types.Identical(box.Type(), f.Params[0].Type()) {
		return nil, false
	}
	if !slices.Contains(box.Block().Instrs, ssa.Instruction(box)) || !slices.Contains(site.Block().Instrs, ssa.Instruction(site)) {
		return nil, false
	}
	if box.Block() == site.Block() {
		seen := false
		for _, i := range site.Block().Instrs {
			if i == box {
				seen = true
			}
			if i == site {
				if !seen {
					return nil, false
				}
				break
			}
		}
	}
	literal, ok := box.X.(*ssa.Const)
	if !ok {
		return nil, false
	}
	if _, nested := literal.Type().Underlying().(*types.Interface); nested {
		return nil, false
	}
	// Reuse the existing literal/type/value bounds, including refusal of nil
	// synchronization/callback references. No new general value domain is added.
	e.literal(literal)
	remaining := 256
	if !concreteType(literal.Type(), map[types.Type]bool{}, &remaining, 0) {
		return nil, false
	}
	is := exactBlock(f, 2)
	if is == nil {
		return nil, false
	}
	call := e.standardCall(is[0], "internal/abi", "TypeOf", f.Params[0])
	if call == nil || !e.typeOfShape(call.Common().StaticCallee()) || !types.Identical(call.Type(), site.Type()) || !returns(is[1], call) {
		return nil, false
	}
	return literal.Type(), true
}
