package effects

import (
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"runtime"
	"slices"

	"golang.org/x/tools/go/ssa"
)

const ReflectionTypeModel = "Standard Go reflect.TypeFor returns immutable metadata for its concrete type argument; runtime ABI/type metadata correctness is assumed, not proved. Reflective values and application method dispatch are not admitted; type queries require separate explicit models."

func reflectionType(t types.Type) bool {
	n, ok := types.Unalias(t).(*types.Named)
	if !ok || n.Obj().Pkg() == nil || n.Obj().Pkg().Path() != "reflect" || n.Obj().Name() != "Type" {
		return false
	}
	_, ok = n.Underlying().(*types.Interface)
	return ok
}

func (e *dataEval) standardFunction(f *ssa.Function, pkg, name string) bool {
	if f == nil || f.Signature == nil || f.Signature.Recv() != nil || f.Signature.Variadic() {
		return false
	}
	o := f.Origin()
	if o == nil {
		o = f
	}
	if o.Pkg == nil || o.Pkg.Pkg.Path() != pkg || o.Name() != name || o.Object() == nil || o.Pkg.Pkg.Scope().Lookup(name) != o.Object() {
		return false
	}
	decl, ok := o.Syntax().(*ast.FuncDecl)
	if !ok || decl.Body == nil || decl.Name.Name != name || decl.Name.Pos() != o.Pos() || !types.Identical(o.Signature, o.Object().Type()) {
		return false
	}
	file := filepath.Clean(e.program.Fset.Position(o.Pos()).Filename)
	return filepath.Dir(file) == filepath.Join(runtime.GOROOT(), "src", filepath.FromSlash(pkg)) && e.program.Sources[file].Package == pkg
}

func exactBlock(f *ssa.Function, n int) []ssa.Instruction {
	if f == nil || len(f.FreeVars) != 0 || len(f.Blocks) != 1 || len(f.Blocks[0].Preds) != 0 || len(f.Blocks[0].Succs) != 0 || len(f.Blocks[0].Instrs) != n {
		return nil
	}
	if f.Signature.Params().Len() != len(f.Params) {
		return nil
	}
	for n, p := range f.Params {
		if p.Parent() != f || !types.Identical(p.Type(), f.Signature.Params().At(n).Type()) {
			return nil
		}
	}
	for _, i := range f.Blocks[0].Instrs {
		if i.Parent() != f {
			return nil
		}
		if r, ok := i.(*ssa.Return); ok {
			if len(r.Results) != f.Signature.Results().Len() {
				return nil
			}
			for n, v := range r.Results {
				if !types.Identical(v.Type(), f.Signature.Results().At(n).Type()) {
					return nil
				}
			}
		}
	}
	return f.Blocks[0].Instrs
}
func returns(i ssa.Instruction, v ssa.Value) bool {
	r, ok := i.(*ssa.Return)
	return ok && len(r.Results) == 1 && r.Results[0] == v
}
func converted(i ssa.Instruction, v ssa.Value) *ssa.Convert {
	c, ok := i.(*ssa.Convert)
	if !ok || c.X != v {
		return nil
	}
	return c
}
func loaded(i ssa.Instruction, v ssa.Value) *ssa.UnOp {
	u, ok := i.(*ssa.UnOp)
	if !ok || u.Op != token.MUL || u.X != v {
		return nil
	}
	return u
}
func pointerNamed(t types.Type, pkg, name string) bool {
	p, ok := t.Underlying().(*types.Pointer)
	if !ok {
		return false
	}
	n, ok := types.Unalias(p.Elem()).(*types.Named)
	return ok && n.Obj().Pkg() != nil && n.Obj().Pkg().Path() == pkg && n.Obj().Name() == name
}
func fieldAddress(i ssa.Instruction, v ssa.Value, index int, name string) *ssa.FieldAddr {
	a, ok := i.(*ssa.FieldAddr)
	if !ok || a.X != v || a.Field != index {
		return nil
	}
	p, ok := v.Type().Underlying().(*types.Pointer)
	if !ok {
		return nil
	}
	s, ok := p.Elem().Underlying().(*types.Struct)
	if !ok || index >= s.NumFields() || s.Field(index).Name() != name {
		return nil
	}
	return a
}
func (e *dataEval) standardCall(i ssa.Instruction, pkg, name string, args ...ssa.Value) *ssa.Call {
	c, ok := i.(*ssa.Call)
	if !ok || c.Common().IsInvoke() || len(c.Common().Args) != len(args) {
		return nil
	}
	for n, v := range args {
		if c.Common().Args[n] != v {
			return nil
		}
	}
	f := e.program.CallTarget(c)
	if f == nil || f != c.Common().StaticCallee() || !e.standardFunction(f, pkg, name) {
		return nil
	}
	return c
}

// These guards bind the modeled operation to the current typed SSA wrapper and
// ABI extraction chain. They do not claim to prove unsafe ABI representation.
func (e *dataEval) noEscapeShape(f *ssa.Function) bool {
	is := exactBlock(f, 4)
	if is == nil || len(f.Params) != 1 {
		return false
	}
	a := converted(is[0], f.Params[0])
	if a == nil || !types.Identical(a.Type(), types.Typ[types.Uintptr]) {
		return false
	}
	b, ok := is[1].(*ssa.BinOp)
	if !ok || b.Op != token.XOR || b.X != a || !intConstant(b.Y, 0) || !types.Identical(b.Type(), a.Type()) || !types.Identical(b.Y.Type(), a.Type()) {
		return false
	}
	c := converted(is[2], b)
	return c != nil && types.Identical(c.Type(), types.Typ[types.UnsafePointer]) && returns(is[3], c)
}
func (e *dataEval) typeOfShape(f *ssa.Function) bool {
	is := exactBlock(f, 13)
	if is == nil || len(f.Params) != 1 {
		return false
	}
	a, ok := is[0].(*ssa.Alloc)
	if !ok {
		return false
	}
	s, ok := is[1].(*ssa.Store)
	if !ok || s.Addr != a || s.Val != f.Params[0] {
		return false
	}
	b, ok := is[2].(*ssa.Alloc)
	if !ok || !pointerNamed(b.Type(), "internal/abi", "EmptyInterface") {
		return false
	}
	c := converted(is[3], a)
	if c == nil || !types.Identical(c.Type(), types.Typ[types.UnsafePointer]) {
		return false
	}
	d := converted(is[4], c)
	if d == nil || !types.Identical(d.Type(), b.Type()) {
		return false
	}
	v := loaded(is[5], d)
	if v == nil {
		return false
	}
	s, ok = is[6].(*ssa.Store)
	if !ok || s.Addr != b || s.Val != v {
		return false
	}
	field := fieldAddress(is[7], b, 0, "Type")
	if field == nil {
		return false
	}
	typ := loaded(is[8], field)
	if typ == nil || !pointerNamed(typ.Type(), "internal/abi", "Type") {
		return false
	}
	ptr := converted(is[9], typ)
	if ptr == nil || !types.Identical(ptr.Type(), types.Typ[types.UnsafePointer]) {
		return false
	}
	call := e.standardCall(is[10], "internal/abi", "NoEscape", ptr)
	if call == nil || !e.noEscapeShape(call.Common().StaticCallee()) {
		return false
	}
	out := converted(is[11], call)
	return out != nil && types.Identical(out.Type(), typ.Type()) && returns(is[12], out)
}
func (e *dataEval) abiTypeForShape(f *ssa.Function, t types.Type) bool {
	is := exactBlock(f, 7)
	if is == nil || len(f.Params) != 0 || len(f.TypeArgs()) != 1 || !types.Identical(f.TypeArgs()[0], t) {
		return false
	}
	box, ok := is[0].(*ssa.MakeInterface)
	if !ok {
		return false
	}
	nilptr, ok := box.X.(*ssa.Const)
	if !ok || nilptr.Value != nil {
		return false
	}
	p, ok := nilptr.Type().Underlying().(*types.Pointer)
	if !ok || !types.Identical(p.Elem(), t) {
		return false
	}
	call := e.standardCall(is[1], "internal/abi", "TypeOf", box)
	if call == nil || !e.typeOfShape(call.Common().StaticCallee()) {
		return false
	}
	a := converted(is[2], call)
	if a == nil || !types.Identical(a.Type(), types.Typ[types.UnsafePointer]) {
		return false
	}
	b := converted(is[3], a)
	if b == nil || !pointerNamed(b.Type(), "internal/abi", "PtrType") {
		return false
	}
	field := fieldAddress(is[4], b, 1, "Elem")
	if field == nil {
		return false
	}
	out := loaded(is[5], field)
	return out != nil && pointerNamed(out.Type(), "internal/abi", "Type") && returns(is[6], out)
}
func (e *dataEval) toRTypeShape(f *ssa.Function) bool {
	is := exactBlock(f, 3)
	if is == nil || len(f.Params) != 1 || !pointerNamed(f.Params[0].Type(), "internal/abi", "Type") {
		return false
	}
	a := converted(is[0], f.Params[0])
	if a == nil || !types.Identical(a.Type(), types.Typ[types.UnsafePointer]) {
		return false
	}
	b := converted(is[1], a)
	return b != nil && pointerNamed(b.Type(), "reflect", "rtype") && returns(is[2], b)
}

func (e *dataEval) reflectedTypeFor(f *ssa.Function, args []dataValue) (dataValue, bool) {
	if !e.standardFunction(f, "reflect", "TypeFor") || len(f.TypeArgs()) != 1 || len(args) != 0 || len(f.Params) != 0 || f.Signature.Results().Len() != 1 || !reflectionType(f.Signature.Results().At(0).Type()) {
		return dataValue{}, false
	}
	t := f.TypeArgs()[0]
	remaining := 256
	if !concreteType(t, map[types.Type]bool{}, &remaining, 0) {
		return dataValue{}, false
	}
	is := exactBlock(f, 4)
	if is == nil {
		return dataValue{}, false
	}
	a := e.standardCall(is[0], "internal/abi", "TypeFor")
	if a == nil || !e.abiTypeForShape(a.Common().StaticCallee(), t) {
		return dataValue{}, false
	}
	b := e.standardCall(is[1], "reflect", "toRType", a)
	if b == nil || !e.toRTypeShape(b.Common().StaticCallee()) {
		return dataValue{}, false
	}
	box, ok := is[2].(*ssa.MakeInterface)
	if !ok || box.X != b || !types.Identical(box.Type(), f.Signature.Results().At(0).Type()) || !returns(is[3], box) {
		return dataValue{}, false
	}
	if !slices.Contains(e.modeledOperations, ReflectionTypeModel) {
		e.modeledOperations = append(e.modeledOperations, ReflectionTypeModel)
	}
	return dataValue{typ: box.Type(), reflected: t}, true
}

func concreteType(t types.Type, seen map[types.Type]bool, left *int, depth int) bool {
	if t == nil || depth > 64 {
		return false
	}
	if seen[t] {
		return true
	}
	*left--
	if *left < 0 {
		return false
	}
	seen[t] = true
	switch t := types.Unalias(t).(type) {
	case *types.TypeParam:
		return false
	case *types.Named:
		if t.TypeParams().Len() != t.TypeArgs().Len() {
			return false
		}
		for i := range t.TypeArgs().Len() {
			if !concreteType(t.TypeArgs().At(i), seen, left, depth+1) {
				return false
			}
		}
		return true
	case *types.Basic:
		return true
	case *types.Pointer:
		return concreteType(t.Elem(), seen, left, depth+1)
	case *types.Array:
		return concreteType(t.Elem(), seen, left, depth+1)
	case *types.Slice:
		return concreteType(t.Elem(), seen, left, depth+1)
	case *types.Chan:
		return concreteType(t.Elem(), seen, left, depth+1)
	case *types.Map:
		return concreteType(t.Key(), seen, left, depth+1) && concreteType(t.Elem(), seen, left, depth+1)
	case *types.Struct:
		for i := range t.NumFields() {
			if !concreteType(t.Field(i).Type(), seen, left, depth+1) {
				return false
			}
		}
		return true
	case *types.Signature:
		return t.TypeParams().Len() == 0 && concreteType(t.Params(), seen, left, depth+1) && concreteType(t.Results(), seen, left, depth+1)
	case *types.Tuple:
		for i := range t.Len() {
			if !concreteType(t.At(i).Type(), seen, left, depth+1) {
				return false
			}
		}
		return true
	case *types.Interface:
		for i := range t.NumMethods() {
			if !concreteType(t.Method(i).Type(), seen, left, depth+1) {
				return false
			}
		}
		return t.IsMethodSet()
	}
	return false
}
