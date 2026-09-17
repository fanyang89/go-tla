package effects

import (
	"go/constant"
	"go/types"
	"path/filepath"
	"runtime"
	"slices"

	"golang.org/x/tools/go/ssa"
)

const ReflectionQueryModel = "Standard Go reflect.Type Elem and direct FieldByName queries read immutable type metadata; ordinary-executable reflection correctness is assumed, not proved. Promoted-field search and reflective value execution are not modeled."

func (e *dataEval) reflectionQuery(site *ssa.Call, receiver dataValue, args []dataValue) (dataValue, bool) {
	c := site.Common()
	if !c.IsInvoke() || c.Method == nil || !reflectionType(receiver.typ) || receiver.reflected == nil || e.program.CallTarget(site) != nil {
		return dataValue{}, false
	}
	if receiver.scalar != nil || receiver.pointer != nil || receiver.elements != nil || receiver.view != nil {
		return dataValue{}, false
	}
	n := types.Unalias(receiver.typ).(*types.Named)
	if n.Obj().Pkg().Scope().Lookup("Type") != n.Obj() {
		return dataValue{}, false
	}
	owner := n.Underlying().(*types.Interface)
	found := false
	for i := range owner.NumMethods() {
		if owner.Method(i) == c.Method {
			found = true
			break
		}
	}
	file := filepath.Clean(e.program.Fset.Position(c.Method.Pos()).Filename)
	if !found || filepath.Dir(file) != filepath.Join(runtime.GOROOT(), "src", "reflect") || e.program.Sources[file].Package != "reflect" {
		return dataValue{}, false
	}
	signature, ok := c.Method.Type().(*types.Signature)
	if !ok || signature.Params().Len() != len(args) {
		return dataValue{}, false
	}
	for i, arg := range args {
		if !types.Identical(arg.typ, signature.Params().At(i).Type()) {
			return dataValue{}, false
		}
	}
	if signature.Results().Len() == 1 {
		if !types.Identical(site.Type(), signature.Results().At(0).Type()) {
			return dataValue{}, false
		}
	} else if !types.Identical(site.Type(), signature.Results()) {
		return dataValue{}, false
	}
	var result dataValue
	switch c.Method.Name() {
	case "Elem":
		if len(args) != 0 || !reflectionType(site.Type()) {
			return dataValue{}, false
		}
		var elem types.Type
		switch t := receiver.reflected.Underlying().(type) {
		case *types.Pointer:
			elem = t.Elem()
		case *types.Array:
			elem = t.Elem()
		case *types.Slice:
			elem = t.Elem()
		case *types.Chan:
			elem = t.Elem()
		case *types.Map:
			elem = t.Elem()
		default:
			return dataValue{}, false
		}
		result = dataValue{typ: site.Type(), reflected: elem}
	case "FieldByName":
		if len(args) != 1 || args[0].scalar == nil || args[0].scalar.Kind() != constant.String {
			return dataValue{}, false
		}
		e.checkScalar(args[0])
		structure, ok := receiver.reflected.Underlying().(*types.Struct)
		if !ok || structure.NumFields() > 4096 {
			return dataValue{}, false
		}
		tuple, ok := site.Type().Underlying().(*types.Tuple)
		if !ok || tuple.Len() != 2 || !types.Identical(tuple.At(1).Type(), types.Typ[types.Bool]) {
			return dataValue{}, false
		}
		fieldType, ok := types.Unalias(tuple.At(0).Type()).(*types.Named)
		if !ok || fieldType.Obj().Pkg() == nil || fieldType.Obj().Pkg() != n.Obj().Pkg() || fieldType.Obj().Name() != "StructField" || n.Obj().Pkg().Scope().Lookup("StructField") != fieldType.Obj() {
			return dataValue{}, false
		}
		field := e.zero(fieldType, 0)
		index := -1
		embedded := false
		name := constant.StringVal(args[0].scalar)
		for i := range structure.NumFields() {
			e.step()
			f := structure.Field(i)
			if f.Name() == name {
				index = i
				break
			}
			embedded = embedded || f.Embedded()
		}
		if index < 0 && embedded {
			return dataValue{}, false
		}
		if index >= 0 {
			fields := make([]*types.Var, structure.NumFields())
			for i := range structure.NumFields() {
				e.step()
				fields[i] = structure.Field(i)
			}
			offset := e.sizes.Offsetsof(fields)[index]
			if offset < 0 {
				return dataValue{}, false
			}
			f := fields[index]
			pkg := ""
			if !f.Exported() && f.Pkg() != nil {
				pkg = f.Pkg().Path()
				// The Go executable compiler records the main package as "main".
				if f.Pkg().Name() == "main" {
					pkg = "main"
				}
			}
			shape := fieldType.Underlying().(*types.Struct)
			if shape.NumFields() != 7 {
				return dataValue{}, false
			}
			seen := map[string]bool{}
			for i := range shape.NumFields() {
				v := &field.elements[i].value
				key := shape.Field(i).Name()
				if seen[key] {
					return dataValue{}, false
				}
				seen[key] = true
				switch key {
				case "Name":
					v.scalar = constant.MakeString(f.Name())
				case "PkgPath":
					v.scalar = constant.MakeString(pkg)
				case "Type":
					if !reflectionType(v.typ) {
						return dataValue{}, false
					}
					v.reflected = f.Type()
					continue
				case "Tag":
					v.scalar = constant.MakeString(structure.Tag(index))
				case "Offset":
					v.scalar = constant.MakeInt64(offset)
				case "Anonymous":
					v.scalar = constant.MakeBool(f.Embedded())
				case "Index":
					s, ok := v.typ.Underlying().(*types.Slice)
					if !ok || !types.Identical(s.Elem(), types.Typ[types.Int]) {
						return dataValue{}, false
					}
					v.view = &dataView{elements: []*dataCell{e.cell(dataValue{typ: s.Elem(), scalar: constant.MakeInt64(int64(index))})}, length: 1, capacity: 1}
					continue
				default:
					return dataValue{}, false
				}
				e.checkScalar(*v)
			}
		}
		result = dataValue{typ: site.Type(), elements: []*dataCell{e.cell(field), e.cell(dataValue{typ: types.Typ[types.Bool], scalar: constant.MakeBool(index >= 0)})}}
	default:
		return dataValue{}, false
	}
	if !slices.Contains(e.modeledOperations, ReflectionQueryModel) {
		e.modeledOperations = append(e.modeledOperations, ReflectionQueryModel)
	}
	return result, true
}
