package effects

import (
	"go/constant"
	"go/token"
	"go/types"

	"github.com/fanmi/go-tla/internal/frontend"
	"golang.org/x/tools/go/ssa"
)

// ProveConstantDataCall proves one literal-input invocation, not every invocation
// of its callee. Evaluation never invokes host/application code. Unsupported
// operations, executed panics, external memory and exhausted budgets refuse.
func (a *Analyzer) ProveConstantDataCall(site *ssa.Call) (proved bool) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(dataRefusal); !ok {
				panic(r)
			}
			proved = false
		}
	}()
	if len(a.program.Packages) == 0 || a.program.Packages[0].TypesSizes == nil {
		return false
	}
	e := &dataEval{program: a.program, sizes: a.program.Packages[0].TypesSizes, steps: 100000, cells: 65536, owned: map[*dataCell]bool{}, stack: map[*ssa.Function]bool{}}
	args := []dataValue{}
	for _, arg := range site.Common().Args {
		c, ok := arg.(*ssa.Const)
		if !ok {
			return false
		}
		args = append(args, e.literal(c))
	}
	f := a.program.CallTarget(site)
	if f == nil || site.Common().IsInvoke() {
		return false
	}
	e.function(f, args)
	return true
}

type dataRefusal struct{}

func refuseData() { panic(dataRefusal{}) }

type dataValue struct {
	typ      types.Type
	scalar   constant.Value
	pointer  *dataCell
	elements []*dataCell
	view     *dataView
}
type dataCell struct{ value dataValue }
type dataView struct {
	elements              []*dataCell
	low, length, capacity int
}
type dataEval struct {
	program      *frontend.Program
	sizes        types.Sizes
	steps, cells int
	owned        map[*dataCell]bool
	stack        map[*ssa.Function]bool
}

func (e *dataEval) step() {
	e.steps--
	if e.steps < 0 {
		refuseData()
	}
}
func (e *dataEval) cell(v dataValue) *dataCell {
	e.cells--
	if e.cells < 0 {
		refuseData()
	}
	c := &dataCell{v}
	e.owned[c] = true
	return c
}
func (e *dataEval) clone(v dataValue) dataValue {
	e.step()
	if v.elements != nil {
		out := make([]*dataCell, len(v.elements))
		for i, c := range v.elements {
			out[i] = e.cell(e.clone(c.value))
		}
		v.elements = out
	}
	return v
}
func (e *dataEval) zero(t types.Type, depth int) dataValue {
	e.step()
	if depth > 64 {
		refuseData()
	}
	v := dataValue{typ: t}
	switch t := t.Underlying().(type) {
	case *types.Basic:
		switch {
		case t.Info()&types.IsBoolean != 0:
			v.scalar = constant.MakeBool(false)
		case t.Info()&types.IsInteger != 0:
			v.scalar = constant.MakeInt64(0)
		case t.Info()&types.IsString != 0:
			v.scalar = constant.MakeString("")
		default:
			refuseData()
		}
	case *types.Array:
		if t.Len() > 4096 {
			refuseData()
		}
		for range t.Len() {
			v.elements = append(v.elements, e.cell(e.zero(t.Elem(), depth+1)))
		}
	case *types.Struct:
		for i := range t.NumFields() {
			v.elements = append(v.elements, e.cell(e.zero(t.Field(i).Type(), depth+1)))
		}
	default:
		refuseData()
	}
	return v
}
func (e *dataEval) literal(c *ssa.Const) dataValue {
	if !finiteDataType(c.Type(), 0) {
		refuseData()
	}
	v := dataValue{typ: c.Type(), scalar: c.Value}
	if c.Value != nil {
		e.checkScalar(v)
	}
	return v
}
func (e *dataEval) checkScalar(v dataValue) {
	if v.scalar == nil {
		refuseData()
	}
	b, ok := v.typ.Underlying().(*types.Basic)
	if !ok {
		refuseData()
	}
	switch v.scalar.Kind() {
	case constant.Bool:
		if b.Info()&types.IsBoolean == 0 {
			refuseData()
		}
	case constant.String:
		if b.Info()&types.IsString == 0 || len(constant.StringVal(v.scalar)) > 256*1024 {
			refuseData()
		}
	case constant.Int:
		if b.Info()&types.IsInteger == 0 {
			refuseData()
		}
		bits := e.sizes.Sizeof(v.typ) * 8
		if bits < 8 || bits > 64 {
			refuseData()
		}
		one := constant.MakeInt64(1)
		min := constant.MakeInt64(0)
		max := constant.BinaryOp(constant.Shift(one, token.SHL, uint(bits)), token.SUB, one)
		if b.Info()&types.IsUnsigned == 0 {
			max = constant.BinaryOp(constant.Shift(one, token.SHL, uint(bits-1)), token.SUB, one)
			min = constant.UnaryOp(token.SUB, constant.Shift(one, token.SHL, uint(bits-1)), 0)
		}
		// Overflow/conversion wrap is not approximated; refuse instead.
		if constant.Compare(v.scalar, token.LSS, min) || constant.Compare(v.scalar, token.GTR, max) {
			refuseData()
		}
	default:
		refuseData()
	}
}
func (e *dataEval) integer(v dataValue) int {
	if v.scalar == nil || v.scalar.Kind() != constant.Int {
		refuseData()
	}
	n, ok := constant.Int64Val(v.scalar)
	if !ok || n < 0 || n > 256*1024 {
		refuseData()
	}
	return int(n)
}

// Aggregate assignment changes contents, not subobject addresses. Existing field
// and element pointers must keep referring to the same storage after *p = value.
func (e *dataEval) store(c *dataCell, v dataValue) {
	e.step()
	if c == nil || !e.owned[c] || !types.Identical(c.value.typ, v.typ) {
		refuseData()
	}
	switch c.value.typ.Underlying().(type) {
	case *types.Array, *types.Struct:
		if len(c.value.elements) != len(v.elements) {
			refuseData()
		}
		for i, cell := range c.value.elements {
			e.store(cell, v.elements[i].value)
		}
	default:
		c.value = e.clone(v)
	}
}

func (e *dataEval) load(c *dataCell) dataValue {
	if c == nil || !e.owned[c] {
		refuseData()
	}
	return e.clone(c.value)
}
func (e *dataEval) length(v dataValue, capacity bool) int {
	switch t := v.typ.Underlying().(type) {
	case *types.Basic:
		if !capacity && t.Info()&types.IsString != 0 && v.scalar != nil {
			return len(constant.StringVal(v.scalar))
		}
	case *types.Array:
		return int(t.Len())
	case *types.Pointer:
		if a, ok := t.Elem().Underlying().(*types.Array); ok && a.Len() <= 4096 {
			return int(a.Len())
		}
	case *types.Slice:
		if v.view == nil {
			return 0
		}
		if capacity {
			return v.view.capacity
		}
		return v.view.length
	}
	refuseData()
	return 0
}
func (e *dataEval) indexed(v dataValue, i int) *dataCell {
	if v.pointer != nil {
		v = e.load(v.pointer)
	}
	if v.view != nil {
		if i >= v.view.length {
			refuseData()
		}
		return v.view.elements[v.view.low+i]
	}
	if i >= len(v.elements) {
		refuseData()
	}
	return v.elements[i]
}

func (e *dataEval) function(f *ssa.Function, args []dataValue) []dataValue {
	e.step()
	if f == nil || len(f.Blocks) == 0 || len(f.FreeVars) != 0 || len(args) != len(f.Params) || e.stack[f] || len(e.stack) >= 32 {
		refuseData()
	}
	if !finiteDataType(f.Signature.Results(), 0) {
		refuseData()
	}
	e.stack[f] = true
	defer delete(e.stack, f)
	values := map[ssa.Value]dataValue{}
	for i, p := range f.Params {
		if !finiteDataType(p.Type(), 0) || !types.Identical(p.Type(), args[i].typ) {
			refuseData()
		}
		values[p] = e.clone(args[i])
	}
	get := func(v ssa.Value) dataValue {
		if c, ok := v.(*ssa.Const); ok {
			return e.literal(c)
		}
		if result, ok := values[v]; ok {
			return result
		}
		refuseData()
		return dataValue{}
	}
	var previous *ssa.BasicBlock
	block := f.Blocks[0]
	for {
		// Phi operands all read the preceding iteration before any phi is updated.
		phis := map[ssa.Value]dataValue{}
		for _, i := range block.Instrs {
			if phi, ok := i.(*ssa.Phi); ok {
				e.step()
				found := false
				for n, pred := range block.Preds {
					if pred == previous {
						phis[phi] = e.clone(get(phi.Edges[n]))
						found = true
						break
					}
				}
				if !found {
					refuseData()
				}
			}
		}
		for v, value := range phis {
			values[v] = value
		}
		var next *ssa.BasicBlock
		for _, instruction := range block.Instrs {
			e.step()
			if frontend.UsesUnsafePointer(instruction) {
				refuseData()
			}
			var out dataValue
			switch x := instruction.(type) {
			case *ssa.Phi, *ssa.DebugRef:
				continue
			case *ssa.Alloc:
				t := x.Type().Underlying().(*types.Pointer).Elem()
				if !plainData(t, 0) {
					refuseData()
				}
				out = dataValue{typ: x.Type(), pointer: e.cell(e.zero(t, 0))}
			case *ssa.Store:
				addr := get(x.Addr)
				if addr.pointer == nil || !e.owned[addr.pointer] {
					refuseData()
				}
				e.store(addr.pointer, get(x.Val))
				continue
			case *ssa.UnOp:
				v := get(x.X)
				switch x.Op {
				case token.MUL:
					out = e.load(v.pointer)
					out.typ = x.Type()
				case token.NOT:
					if v.scalar == nil || v.scalar.Kind() != constant.Bool {
						refuseData()
					}
					out = dataValue{typ: x.Type(), scalar: constant.MakeBool(!constant.BoolVal(v.scalar))}
				case token.SUB:
					if v.scalar == nil || v.scalar.Kind() != constant.Int {
						refuseData()
					}
					out = dataValue{typ: x.Type(), scalar: constant.UnaryOp(token.SUB, v.scalar, 0)}
					e.checkScalar(out)
				default:
					refuseData()
				}
			case *ssa.BinOp:
				out = e.binary(x, get(x.X), get(x.Y))
			case *ssa.Convert, *ssa.ChangeType:
				var src ssa.Value
				var typ types.Type
				if c, ok := x.(*ssa.Convert); ok {
					src, typ = c.X, c.Type()
				} else {
					c := x.(*ssa.ChangeType)
					src, typ = c.X, c.Type()
				}
				out = e.clone(get(src))
				out.typ = typ
				if out.scalar != nil {
					e.checkScalar(out)
				} else if !types.Identical(src.Type().Underlying(), typ.Underlying()) {
					refuseData()
				}
			case *ssa.FieldAddr:
				v := get(x.X)
				if v.pointer == nil || !e.owned[v.pointer] || x.Field >= len(v.pointer.value.elements) {
					refuseData()
				}
				out = dataValue{typ: x.Type(), pointer: v.pointer.value.elements[x.Field]}
			case *ssa.Field:
				v := get(x.X)
				if x.Field >= len(v.elements) {
					refuseData()
				}
				out = e.load(v.elements[x.Field])
				out.typ = x.Type()
			case *ssa.IndexAddr:
				v := get(x.X)
				n := e.integer(get(x.Index))
				// Addressing must not clone an array: stores affect its owned cell.
				if v.pointer != nil {
					if !e.owned[v.pointer] {
						refuseData()
					}
					v = v.pointer.value
				}
				out = dataValue{typ: x.Type(), pointer: e.indexed(v, n)}
			case *ssa.Index:
				v := get(x.X)
				n := e.integer(get(x.Index))
				if v.scalar != nil && v.scalar.Kind() == constant.String {
					s := constant.StringVal(v.scalar)
					if n >= len(s) {
						refuseData()
					}
					out = dataValue{typ: x.Type(), scalar: constant.MakeInt64(int64(s[n]))}
				} else {
					out = e.load(e.indexed(v, n))
					out.typ = x.Type()
				}
			case *ssa.Slice:
				v := get(x.X)
				if _, pointer := v.typ.Underlying().(*types.Pointer); pointer && (v.pointer == nil || !e.owned[v.pointer]) {
					refuseData()
				}
				low, high, maxIndex := 0, e.length(v, false), e.length(v, false)
				if _, ok := v.typ.Underlying().(*types.Slice); ok {
					maxIndex = e.length(v, true)
				}
				if x.Low != nil {
					low = e.integer(get(x.Low))
				}
				if x.High != nil {
					high = e.integer(get(x.High))
				}
				if x.Max != nil {
					maxIndex = e.integer(get(x.Max))
				}
				if low > high || high > maxIndex || maxIndex > e.length(v, v.scalar == nil) {
					refuseData()
				}
				out = dataValue{typ: x.Type()}
				if v.scalar != nil && v.scalar.Kind() == constant.String {
					out.scalar = constant.MakeString(constant.StringVal(v.scalar)[low:high])
				} else {
					if v.pointer != nil {
						if !e.owned[v.pointer] {
							refuseData()
						}
						v = v.pointer.value
					}
					elements, offset := v.elements, 0
					if v.view != nil {
						elements, offset = v.view.elements, v.view.low
					}
					out.view = &dataView{elements, offset + low, high - low, maxIndex - low}
				}
			case *ssa.Call:
				args := []dataValue{}
				for _, arg := range x.Common().Args {
					args = append(args, get(arg))
				}
				if builtin, ok := x.Common().Value.(*ssa.Builtin); ok {
					out = e.builtin(builtin.Name(), x.Type(), args)
				} else {
					if x.Common().IsInvoke() {
						refuseData()
					}
					results := e.function(e.program.CallTarget(x), args)
					if len(results) == 1 {
						out = results[0]
					} else {
						out.typ = x.Type()
						for _, v := range results {
							out.elements = append(out.elements, e.cell(v))
						}
					}
				}
			case *ssa.Extract:
				v := get(x.Tuple)
				if x.Index >= len(v.elements) {
					refuseData()
				}
				out = e.load(v.elements[x.Index])
				out.typ = x.Type()
			case *ssa.If:
				v := get(x.Cond)
				if v.scalar == nil || v.scalar.Kind() != constant.Bool {
					refuseData()
				}
				if constant.BoolVal(v.scalar) {
					next = block.Succs[0]
				} else {
					next = block.Succs[1]
				}
				continue
			case *ssa.Jump:
				next = block.Succs[0]
				continue
			case *ssa.Return:
				out := []dataValue{}
				for _, v := range x.Results {
					out = append(out, e.clone(get(v)))
				}
				return out
			default:
				refuseData()
			}
			if v, ok := instruction.(ssa.Value); ok {
				values[v] = out
			}
		}
		if next == nil {
			refuseData()
		}
		previous, block = block, next
	}
}

func (e *dataEval) binary(x *ssa.BinOp, a, b dataValue) dataValue {
	out := dataValue{typ: x.Type()}
	if a.scalar == nil || b.scalar == nil {
		refuseData()
	}
	switch x.Op {
	case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
		out.scalar = constant.MakeBool(constant.Compare(a.scalar, x.Op, b.scalar))
	case token.ADD, token.SUB, token.MUL, token.QUO, token.REM:
		if a.scalar.Kind() != constant.Int || b.scalar.Kind() != constant.Int {
			refuseData()
		}
		if (x.Op == token.QUO || x.Op == token.REM) && constant.Sign(b.scalar) == 0 {
			refuseData()
		}
		op := x.Op
		if op == token.QUO {
			op = token.QUO_ASSIGN
		}
		out.scalar = constant.BinaryOp(a.scalar, op, b.scalar)
	default:
		refuseData()
	}
	e.checkScalar(out)
	return out
}
func (e *dataEval) builtin(name string, t types.Type, args []dataValue) dataValue {
	out := dataValue{typ: t}
	switch name {
	case "len", "cap":
		if len(args) != 1 {
			refuseData()
		}
		out.scalar = constant.MakeInt64(int64(e.length(args[0], name == "cap")))
	case "copy":
		if len(args) != 2 {
			refuseData()
		}
		dst, src := args[0], args[1]
		n := min(e.length(dst, false), e.length(src, false))
		// Snapshot before writing, preserving Go's overlapping-copy semantics.
		values := make([]dataValue, n)
		for i := range n {
			e.step()
			if src.scalar != nil && src.scalar.Kind() == constant.String {
				values[i] = dataValue{typ: types.Typ[types.Byte], scalar: constant.MakeInt64(int64(constant.StringVal(src.scalar)[i]))}
			} else {
				values[i] = e.load(e.indexed(src, i))
			}
		}
		for i, v := range values {
			c := e.indexed(dst, i)
			if !e.owned[c] {
				refuseData()
			}
			e.store(c, v)
		}
		out.scalar = constant.MakeInt64(int64(n))
	default:
		refuseData()
	}
	return out
}
