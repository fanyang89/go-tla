package effects

import (
	"go/constant"
	"go/token"
	"go/types"

	"github.com/fanmi/go-tla/internal/frontend"
	cslice "github.com/fanmi/go-tla/internal/slice"
	"golang.org/x/tools/go/ssa"
)

const maxFiniteDataProofSteps = 4096

// ProveFiniteData rechecks the current graph, not a cached purity assertion.
// Every call body is read-only and total under the existing no-implicit-panics
// domain. Cycles must count monotonically against a stable Go slice/string len
// or a portable nonnegative int constant.
// No trusted calls, guessed trip counts, shared stores or synchronization qualify.
func (a *Analyzer) ProveFiniteData(f *ssa.Function) bool {
	if f == nil || !cslice.HasCycle(f) {
		return false
	}
	p := finiteDataProof{program: a.program, remaining: maxFiniteDataProofSteps, visiting: map[*ssa.Function]bool{}, proved: map[*ssa.Function]bool{}}
	return p.function(f)
}

type finiteDataProof struct {
	program          *frontend.Program
	remaining        int
	visiting, proved map[*ssa.Function]bool
}

func (p *finiteDataProof) function(f *ssa.Function) bool {
	if f == nil || len(f.Blocks) == 0 || p.visiting[f] {
		return false
	}
	if p.proved[f] {
		return true
	}
	p.remaining--
	if p.remaining < 0 {
		return false
	}
	p.visiting[f] = true
	defer delete(p.visiting, f)
	for _, v := range f.Params {
		if !finiteDataType(v.Type(), 0) {
			return false
		}
	}
	for _, v := range f.FreeVars {
		if !finiteDataType(v.Type(), 0) {
			return false
		}
	}
	if !finiteDataType(f.Signature.Results(), 0) {
		return false
	}
	for _, component := range cslice.CyclicComponents(f) {
		if !lengthCountedComponent(component) {
			return false
		}
	}
	private := map[*ssa.Alloc]bool{}
	for _, bb := range f.Blocks {
		for _, i := range bb.Instrs {
			p.remaining--
			if p.remaining < 0 || frontend.UsesUnsafePointer(i) {
				return false
			}
			// Explicit whitelist: unknown instructions cannot acquire this proof.
			switch x := i.(type) {
			case *ssa.Alloc:
				// SSA may materialize a range's value-struct copy on the stack.
				// Any writes still require complete private-address-use proof.
			case *ssa.Store:
				if !privateFiniteDataStore(x, private) {
					return false
				}
			case *ssa.Call:
				if _, builtin := x.Common().Value.(*ssa.Builtin); builtin {
					if !readOnlyBuiltin(x.Common()) {
						return false
					}
				} else if !p.function(p.program.CallTarget(x)) {
					return false
				}
			case *ssa.UnOp:
				if x.Op != token.MUL && x.Op != token.NOT && x.Op != token.SUB && x.Op != token.XOR {
					return false
				}
			case *ssa.Phi, *ssa.BinOp, *ssa.Convert, *ssa.ChangeType, *ssa.Extract,
				*ssa.Slice, *ssa.Index, *ssa.IndexAddr, *ssa.Field, *ssa.FieldAddr,
				*ssa.If, *ssa.Jump, *ssa.Return, *ssa.DebugRef:
			default:
				return false
			}
			if v, ok := i.(ssa.Value); ok && !finiteDataType(v.Type(), 0) {
				return false
			}
			for _, operand := range i.Operands(nil) {
				if operand == nil || *operand == nil {
					continue
				}
				switch (*operand).(type) {
				case *ssa.Function, *ssa.Builtin:
					continue
				}
				if !finiteDataType((*operand).Type(), 0) {
					return false
				}
			}
		}
	}
	p.proved[f] = true
	return true
}

func finiteDataType(t types.Type, depth int) bool {
	remaining := 256
	return dataTypeGraph(t, depth, map[types.Type]bool{}, &remaining, false)
}

// Read-only data graphs may be recursive, but cannot conceal synchronization,
// callbacks or unsafe pointers. General finite proofs also exclude interfaces;
// only a concrete evaluator enforcing nil/explicit metadata values may opt in.
// A visited type closes a type cycle; every distinct edge must pass the check.
func dataTypeGraph(t types.Type, depth int, seen map[types.Type]bool, remaining *int, nilInterfaces bool) bool {
	if t == nil || depth > 64 {
		return false
	}
	if seen[t] {
		return true
	}
	*remaining--
	if *remaining < 0 {
		return false
	}
	seen[t] = true
	if n, ok := types.Unalias(t).(*types.Named); ok && n.Obj().Pkg() != nil {
		switch n.Obj().Pkg().Path() {
		case "sync", "sync/atomic", "internal/sync":
			return false
		}
	}
	if nilInterfaces && reflectionType(t) {
		return true
	}
	switch t := t.Underlying().(type) {
	case *types.Basic:
		return t.Info()&(types.IsBoolean|types.IsInteger|types.IsFloat|types.IsComplex|types.IsString) != 0
	case *types.Interface:
		// Only the concrete evaluator may opt in: it cannot construct non-nil
		// interface values and rejects boxing, assertion and dynamic dispatch.
		return nilInterfaces && t.Empty()
	case *types.Slice:
		return dataTypeGraph(t.Elem(), depth+1, seen, remaining, nilInterfaces)
	case *types.Array:
		return dataTypeGraph(t.Elem(), depth+1, seen, remaining, nilInterfaces)
	case *types.Pointer:
		return dataTypeGraph(t.Elem(), depth+1, seen, remaining, nilInterfaces)
	case *types.Map:
		return dataTypeGraph(t.Key(), depth+1, seen, remaining, nilInterfaces) && dataTypeGraph(t.Elem(), depth+1, seen, remaining, nilInterfaces)
	case *types.Struct:
		for i := range t.NumFields() {
			if !dataTypeGraph(t.Field(i).Type(), depth+1, seen, remaining, nilInterfaces) {
				return false
			}
		}
		return true
	case *types.Tuple:
		for i := range t.Len() {
			if !dataTypeGraph(t.At(i).Type(), depth+1, seen, remaining, nilInterfaces) {
				return false
			}
		}
		return true
	}
	return false
}

func intConstant(v ssa.Value, want int64) bool {
	c, ok := v.(*ssa.Const)
	if !ok || c.Value == nil || c.Value.Kind() != constant.Int {
		return false
	}
	n, ok := constant.Int64Val(c.Value)
	return ok && n == want
}

func incrementOf(v ssa.Value, phi *ssa.Phi) bool {
	add, ok := v.(*ssa.BinOp)
	return ok && add.Op == token.ADD && add.X == phi && intConstant(add.Y, 1)
}

// Constants are restricted to the common range of Go's 32- and 64-bit int.
// Larger constants refuse this proof rather than assuming the host architecture
// matches the analyzed target. These are proof bounds, never truncation counts.
func finiteDataBound(v ssa.Value, members map[*ssa.BasicBlock]bool) bool {
	if c, ok := v.(*ssa.Const); ok {
		if !types.Identical(c.Type(), types.Typ[types.Int]) || c.Value == nil || c.Value.Kind() != constant.Int {
			return false
		}
		n, ok := constant.Int64Val(c.Value)
		return ok && n >= 0 && n <= 1<<31-1
	}
	length, ok := v.(*ssa.Call)
	if !ok {
		return false
	}
	builtin, ok := length.Common().Value.(*ssa.Builtin)
	if !ok || builtin.Name() != "len" || len(length.Common().Args) != 1 {
		return false
	}
	value := length.Common().Args[0]
	switch t := value.Type().Underlying().(type) {
	case *types.Slice:
		if !finiteDataType(t.Elem(), 0) {
			return false
		}
	case *types.Basic:
		if t.Kind() != types.String {
			return false
		}
	default:
		return false
	}
	// The slice/string header is evaluated once, not reloaded per trip.
	if instruction, ok := value.(ssa.Instruction); ok && members[instruction.Block()] {
		return false
	}
	return true
}

func lengthCountedComponent(component []*ssa.BasicBlock) bool {
	members := map[*ssa.BasicBlock]bool{}
	for _, bb := range component {
		members[bb] = true
	}
	for _, header := range component {
		branch, ok := header.Instrs[len(header.Instrs)-1].(*ssa.If)
		if !ok || len(header.Succs) != 2 || !members[header.Succs[0]] || members[header.Succs[1]] {
			continue
		}
		cond, ok := branch.Cond.(*ssa.BinOp)
		if !ok || cond.Op != token.LSS {
			continue
		}
		if !finiteDataBound(cond.Y, members) {
			continue
		}
		phi, ok := cond.X.(*ssa.Phi)
		initial := int64(0)
		if !ok {
			step, isAdd := cond.X.(*ssa.BinOp)
			if !isAdd {
				continue
			}
			phi, ok = step.X.(*ssa.Phi)
			if !ok || !incrementOf(step, phi) {
				continue
			}
			initial = -1 // SSA range: increment before testing, starting from -1.
		}
		if phi.Block() != header || !types.Identical(phi.Type(), types.Typ[types.Int]) || len(phi.Edges) != len(header.Preds) {
			continue
		}
		valid, entry := true, false
		for i, pred := range header.Preds {
			if members[pred] {
				if !incrementOf(phi.Edges[i], phi) || initial == -1 && phi.Edges[i] != cond.X {
					valid = false
				}
			} else {
				entry = true
				if !intConstant(phi.Edges[i], initial) {
					valid = false
				}
			}
		}
		if !valid || !entry {
			continue
		}
		for _, bb := range component {
			if !header.Dominates(bb) {
				valid = false
			}
		}
		if !valid || cycleWithoutHeader(component, members, header) {
			continue
		}
		// The bound is in [0,maxInt]. Every taken back edge advances exactly
		// one from an index below it, so the control measure cannot wrap. An
		// unused increment computed on the final exit is not a back edge.
		return true
	}
	return false
}

func cycleWithoutHeader(component []*ssa.BasicBlock, members map[*ssa.BasicBlock]bool, header *ssa.BasicBlock) bool {
	color := map[*ssa.BasicBlock]int{}
	var visit func(*ssa.BasicBlock) bool
	visit = func(bb *ssa.BasicBlock) bool {
		if bb == header || !members[bb] {
			return false
		}
		if color[bb] == 1 {
			return true
		}
		if color[bb] == 2 {
			return false
		}
		color[bb] = 1
		for _, next := range bb.Succs {
			if visit(next) {
				return true
			}
		}
		color[bb] = 2
		return false
	}
	for _, bb := range component {
		if visit(bb) {
			return true
		}
	}
	return false
}
