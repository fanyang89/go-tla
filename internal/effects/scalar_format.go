package effects

import (
	"go/ast"
	"go/types"
	"path/filepath"
	"runtime"

	"github.com/fanmi/go-tla/internal/abstract"
	"golang.org/x/tools/go/ssa"
)

const ScalarFormatModel = "Standard Go fmt.Sprintf/Sprint/Sprintln with only unnamed basic scalar or nil arguments performs no application callbacks, I/O or application synchronization. Standard formatting/private-pool/runtime correctness and normal resource availability are assumed, not proved. Format contents and returned strings remain abstract; no finite payload bound is established."

type ScalarFormatProof struct {
	Operands []ssa.Value
	Stores   []*ssa.Store
}

func (e *dataEval) formatMethod(site *ssa.Call, name string, receiver ssa.Value, args ...ssa.Value) bool {
	f := e.program.CallTarget(site)
	if f == nil || f != site.Common().StaticCallee() || f.Object() == nil || f.Name() != name || f.Signature.Recv() == nil || f.Signature.Variadic() || !pointerNamed(f.Signature.Recv().Type(), "fmt", "pp") || f.Signature.Results().Len() != 0 || f.Signature.Params().Len() != len(args) || len(site.Common().Args) != len(args)+1 || site.Common().Args[0] != receiver {
		return false
	}
	obj := f.Object()
	if obj.Pkg() == nil || obj.Pkg().Path() != "fmt" || !types.Identical(f.Signature, obj.Type()) {
		return false
	}
	selection := types.NewMethodSet(receiver.Type()).Lookup(obj.Pkg(), name)
	if selection == nil || selection.Obj() != obj {
		return false
	}
	decl, ok := f.Syntax().(*ast.FuncDecl)
	if !ok || decl.Body == nil || decl.Name.Pos() != f.Pos() {
		return false
	}
	file := filepath.Clean(e.program.Fset.Position(f.Pos()).Filename)
	if filepath.Dir(file) != filepath.Join(runtime.GOROOT(), "src", "fmt") || e.program.Sources[file].Package != "fmt" {
		return false
	}
	for n, arg := range args {
		if site.Common().Args[n+1] != arg || !types.Identical(arg.Type(), f.Signature.Params().At(n).Type()) {
			return false
		}
	}
	return true
}

func (e *dataEval) scalarFormatShape(f *ssa.Function) bool {
	if f == nil || f.Pkg == nil || f.Pkg.Pkg.Path() != "fmt" || f.Object() == nil || f.Pkg.Pkg.Scope().Lookup(f.Name()) != f.Object() {
		return false
	}
	params, method := 1, ""
	switch f.Name() {
	case "Sprintf":
		params, method = 2, "doPrintf"
	case "Sprint":
		method = "doPrint"
	case "Sprintln":
		method = "doPrintln"
	default:
		return false
	}
	sig := f.Signature
	if sig == nil || sig.Recv() != nil || !sig.Variadic() || sig.Params().Len() != params || sig.Results().Len() != 1 || !types.Identical(sig, f.Object().Type()) || !types.Identical(sig.Results().At(0).Type(), types.Typ[types.String]) {
		return false
	}
	if params == 2 && !types.Identical(sig.Params().At(0).Type(), types.Typ[types.String]) {
		return false
	}
	slice, ok := sig.Params().At(params - 1).Type().Underlying().(*types.Slice)
	if !ok {
		return false
	}
	iface, ok := slice.Elem().Underlying().(*types.Interface)
	if !ok || !iface.Empty() {
		return false
	}
	decl, ok := f.Syntax().(*ast.FuncDecl)
	if !ok || decl.Body == nil || decl.Name.Pos() != f.Pos() {
		return false
	}
	file := filepath.Clean(e.program.Fset.Position(f.Pos()).Filename)
	if filepath.Dir(file) != filepath.Join(runtime.GOROOT(), "src", "fmt") || e.program.Sources[file].Package != "fmt" {
		return false
	}
	is := exactBlock(f, 7)
	if is == nil {
		return false
	}
	printer := e.standardCall(is[0], "fmt", "newPrinter")
	if printer == nil || !pointerNamed(printer.Type(), "fmt", "pp") {
		return false
	}
	call, ok := is[1].(*ssa.Call)
	args := make([]ssa.Value, len(f.Params))
	for n, param := range f.Params {
		args[n] = param
	}
	if !ok || !e.formatMethod(call, method, printer, args...) {
		return false
	}
	addr := fieldAddress(is[2], printer, 0, "buf")
	if addr == nil {
		return false
	}
	buf := loaded(is[3], addr)
	if buf == nil {
		return false
	}
	text := converted(is[4], buf)
	if text == nil || !types.Identical(text.Type(), types.Typ[types.String]) {
		return false
	}
	free, ok := is[5].(*ssa.Call)
	return ok && e.formatMethod(free, "free", printer) && returns(is[6], text)
}

// ProveScalarFormat verifies a current standard wrapper and a nonescaping local
// argument array. It is an explicit operation model, not a proof of fmt's body.
func (a *Analyzer) ProveScalarFormat(site *ssa.Call) *ScalarFormatProof {
	if site == nil || site.Common().IsInvoke() || len(site.Common().Args) < 1 || len(site.Common().Args) > 2 {
		return nil
	}
	f := a.program.CallTarget(site)
	e := &dataEval{program: a.program}
	if f != site.Common().StaticCallee() || !e.scalarFormatShape(f) || len(site.Common().Args) != f.Signature.Params().Len() || !types.Identical(site.Type(), types.Typ[types.String]) {
		return nil
	}
	for n, arg := range site.Common().Args {
		if !types.Identical(arg.Type(), f.Signature.Params().At(n).Type()) {
			return nil
		}
	}
	proof := &ScalarFormatProof{Operands: append([]ssa.Value{site.Common().Value}, site.Common().Args...)}
	argIndex := len(site.Common().Args) - 1
	args := site.Common().Args[argIndex]
	if c, ok := args.(*ssa.Const); ok && c.IsNil() {
		return proof
	}
	view, ok := args.(*ssa.Slice)
	if !ok || view.Low != nil || view.High != nil || view.Max != nil {
		return nil
	}
	allocation, ok := view.X.(*ssa.Alloc)
	if !ok {
		return nil
	}
	ptr, ok := allocation.Type().Underlying().(*types.Pointer)
	if !ok {
		return nil
	}
	array, ok := ptr.Elem().Underlying().(*types.Array)
	if !ok || array.Len() > 64 || !types.Identical(array.Elem(), f.Signature.Params().At(argIndex).Type().Underlying().(*types.Slice).Elem()) {
		return nil
	}
	caller := site.Parent()
	if caller == nil || view.Parent() != caller || allocation.Parent() != caller {
		return nil
	}
	// Rebuild uses from current instructions, not stale SSA referrer caches.
	uses := map[ssa.Value][]ssa.Instruction{}
	order := map[ssa.Instruction]int{}
	remaining := 4096
	for _, bb := range caller.Blocks {
		for n, i := range bb.Instrs {
			remaining--
			if remaining < 0 || i.Parent() != caller || i.Block() != bb {
				return nil
			}
			order[i] = n
			if _, debug := i.(*ssa.DebugRef); debug {
				continue
			}
			for _, operand := range i.Operands(nil) {
				remaining--
				if remaining < 0 {
					return nil
				}
				if operand != nil {
					uses[*operand] = append(uses[*operand], i)
				}
			}
		}
	}
	dominates := func(i, j ssa.Instruction) bool {
		n, ok := order[i]
		m, found := order[j]
		return ok && found && i.Block().Dominates(j.Block()) && (i.Block() != j.Block() || n < m)
	}
	if !dominates(allocation, view) || !dominates(view, site) || len(uses[view]) != 1 || uses[view][0] != site {
		return nil
	}
	seen := map[int]bool{}
	for _, use := range uses[allocation] {
		if use == view {
			continue
		}
		index, ok := use.(*ssa.IndexAddr)
		if !ok || index.X != allocation || !dominates(allocation, index) {
			return nil
		}
		n, ok := abstract.Integer(index.Index)
		if !ok || n < 0 || int64(n) >= array.Len() || seen[n] || len(uses[index]) != 1 {
			return nil
		}
		store, ok := uses[index][0].(*ssa.Store)
		if !ok || store.Addr != index || !dominates(index, store) || !dominates(store, site) || !types.Identical(store.Val.Type(), array.Elem()) {
			return nil
		}
		switch val := store.Val.(type) {
		case *ssa.Const:
			if !val.IsNil() {
				return nil
			}
		case *ssa.MakeInterface:
			basic, ok := types.Unalias(val.X.Type()).(*types.Basic)
			if !ok || basic.Info()&(types.IsBoolean|types.IsNumeric|types.IsString) == 0 || !dominates(val, store) {
				return nil
			}
			proof.Operands = append(proof.Operands, val.X)
		default:
			return nil
		}
		seen[n] = true
		proof.Stores = append(proof.Stores, store)
		proof.Operands = append(proof.Operands, index, index.Index, store.Val)
	}
	proof.Operands = append(proof.Operands, view, allocation)
	return proof
}
