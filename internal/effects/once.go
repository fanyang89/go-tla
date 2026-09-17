package effects

import (
	"go/ast"
	"go/constant"
	"go/types"
	"path/filepath"
	"runtime"

	"github.com/fanmi/go-tla/internal/discovery"
	"golang.org/x/tools/go/ssa"
)

const OnceModel = "Standard sync.Once.Do uses an atomic completed flag and a mutex: exactly one normal-return callback completes before waiting calls return; completed calls may bypass the still-held mutex. Standard atomic/mutex/runtime correctness is assumed. Callback bodies are analyzed; panic unwinding, Goexit, copies, reset, nil or unknown callbacks are not modeled."

type OnceProof struct {
	Callback *ssa.Function
	Receiver ssa.Value
	Captures []ssa.Value
	Operands []ssa.Value
}

func IsOnceDo(site ssa.CallInstruction) bool {
	f := site.Common().StaticCallee()
	return f != nil && f.Signature != nil && discovery.SyncTypeReceiver(f) == "Once" && f.Name() == "Do"
}

func (a *Analyzer) onceMethod(site ssa.CallInstruction, pkg, typ, name string, args ...ssa.Value) *ssa.Function {
	if site == nil || site.Common().IsInvoke() {
		return nil
	}
	f := a.program.CallTarget(site)
	if f == nil || f != site.Common().StaticCallee() || f.Name() != name || f.Object() == nil || f.Synthetic != "" || f.Signature == nil || f.Signature.Recv() == nil || !pointerNamed(f.Signature.Recv().Type(), pkg, typ) || f.Signature.Variadic() || len(f.TypeArgs()) != 0 || len(site.Common().Args) != len(args) || len(args) == 0 || f.Signature.Params().Len() != len(args)-1 || !types.Identical(f.Signature, f.Object().Type()) {
		return nil
	}
	decl, ok := f.Syntax().(*ast.FuncDecl)
	if !ok || decl.Body == nil || decl.Name.Pos() != f.Pos() {
		return nil
	}
	file := filepath.Clean(a.program.Fset.Position(f.Pos()).Filename)
	if filepath.Dir(file) != filepath.Join(runtime.GOROOT(), "src", filepath.FromSlash(pkg)) || a.program.Sources[file].Package != pkg {
		return nil
	}
	selection := types.NewMethodSet(args[0].Type()).Lookup(f.Object().Pkg(), name)
	if selection == nil || selection.Obj() != f.Object() {
		return nil
	}
	for i, arg := range args {
		if site.Common().Args[i] != arg {
			return nil
		}
		want := f.Signature.Recv().Type()
		if i > 0 {
			want = f.Signature.Params().At(i - 1).Type()
		}
		if !types.Identical(arg.Type(), want) {
			return nil
		}
	}
	results := f.Signature.Results()
	if name == "Load" {
		if results.Len() != 1 || !types.Identical(results.At(0).Type(), types.Typ[types.Bool]) {
			return nil
		}
	} else if results.Len() != 0 {
		return nil
	}
	return f
}

func onceBlocks(f *ssa.Function, lengths []int, successors [][]int) bool {
	if f == nil || len(f.Blocks) != len(lengths) || len(f.Params) != 2 || len(f.FreeVars) != 0 {
		return false
	}
	for n, bb := range f.Blocks {
		if bb.Index != n || bb.Parent() != f || len(bb.Instrs) != lengths[n] || len(bb.Succs) != len(successors[n]) {
			return false
		}
		for j, want := range successors[n] {
			if bb.Succs[j] != f.Blocks[want] {
				return false
			}
		}
		for _, i := range bb.Instrs {
			if i.Block() != bb || i.Parent() != f {
				return false
			}
		}
	}
	return true
}
func onceField(i ssa.Instruction, receiver ssa.Value, index int, name string) *ssa.FieldAddr {
	x, ok := i.(*ssa.FieldAddr)
	if !ok || x.X != receiver || x.Field != index {
		return nil
	}
	ptr, ok := receiver.Type().Underlying().(*types.Pointer)
	if !ok {
		return nil
	}
	s, ok := ptr.Elem().Underlying().(*types.Struct)
	if !ok || s.NumFields() <= index || s.Field(index).Name() != name || !types.Identical(x.Type(), types.NewPointer(s.Field(index).Type())) {
		return nil
	}
	return x
}
func onceCall(i ssa.Instruction) ssa.CallInstruction {
	c, ok := i.(*ssa.Call)
	if !ok {
		return nil
	}
	return c
}
func onceDefer(i ssa.Instruction) ssa.CallInstruction {
	c, ok := i.(*ssa.Defer)
	if !ok {
		return nil
	}
	return c
}
func onceValue(i ssa.Instruction) ssa.Value { v, _ := i.(*ssa.Call); return v }
func emptyReturn(i ssa.Instruction) bool    { r, ok := i.(*ssa.Return); return ok && len(r.Results) == 0 }
func emptyCallbackType(t types.Type) bool {
	s, ok := t.Underlying().(*types.Signature)
	return ok && s.Recv() == nil && !s.Variadic() && s.Params().Len() == 0 && s.Results().Len() == 0
}

func (a *Analyzer) onceShape(f *ssa.Function) bool {
	if !onceBlocks(f, []int{3, 2, 1}, [][]int{{2, 1}, {2}, {}}) || f.Recover != nil || !emptyCallbackType(f.Params[1].Type()) {
		return false
	}
	is := f.Blocks[0].Instrs
	flag := onceField(is[0], f.Params[0], 1, "done")
	if flag == nil || a.onceMethod(onceCall(is[1]), "sync/atomic", "Bool", "Load", flag) == nil {
		return false
	}
	branch, ok := is[2].(*ssa.If)
	if !ok || branch.Cond != onceValue(is[1]) {
		return false
	}
	call := onceCall(f.Blocks[1].Instrs[0])
	slow := a.onceMethod(call, "sync", "Once", "doSlow", f.Params[0], f.Params[1])
	if slow == nil {
		return false
	}
	if _, ok := f.Blocks[1].Instrs[1].(*ssa.Jump); !ok {
		return false
	}
	if !emptyReturn(f.Blocks[2].Instrs[0]) {
		return false
	}
	if !onceBlocks(slow, []int{7, 1, 4, 2}, [][]int{{3, 2}, {}, {3}, {}}) || slow.Recover != slow.Blocks[1] {
		return false
	}
	is = slow.Blocks[0].Instrs
	lock := onceField(is[0], slow.Params[0], 2, "m")
	unlock := onceField(is[2], slow.Params[0], 2, "m")
	flag = onceField(is[4], slow.Params[0], 1, "done")
	if lock == nil || unlock == nil || flag == nil || a.onceMethod(onceCall(is[1]), "sync", "Mutex", "Lock", lock) == nil || a.onceMethod(onceDefer(is[3]), "sync", "Mutex", "Unlock", unlock) == nil || a.onceMethod(onceCall(is[5]), "sync/atomic", "Bool", "Load", flag) == nil {
		return false
	}
	deferred, ok := is[3].(*ssa.Defer)
	if !ok || deferred.DeferStack != nil {
		return false
	}
	branch, ok = is[6].(*ssa.If)
	if !ok || branch.Cond != onceValue(is[5]) || !emptyReturn(slow.Blocks[1].Instrs[0]) {
		return false
	}
	is = slow.Blocks[2].Instrs
	flag = onceField(is[0], slow.Params[0], 1, "done")
	deferred, ok = is[1].(*ssa.Defer)
	if flag == nil || !ok || deferred.DeferStack != nil || len(deferred.Common().Args) != 2 {
		return false
	}
	value, ok := deferred.Common().Args[1].(*ssa.Const)
	if !ok || value.Value == nil || value.Value.Kind() != constant.Bool || !constant.BoolVal(value.Value) || a.onceMethod(deferred, "sync/atomic", "Bool", "Store", flag, value) == nil {
		return false
	}
	invoke, ok := is[2].(*ssa.Call)
	if !ok || invoke.Common().IsInvoke() || invoke.Common().Value != slow.Params[1] || len(invoke.Common().Args) != 0 {
		return false
	}
	if _, ok := is[3].(*ssa.Jump); !ok {
		return false
	}
	if _, ok := slow.Blocks[3].Instrs[0].(*ssa.RunDefers); !ok {
		return false
	}
	return emptyReturn(slow.Blocks[3].Instrs[1])
}

func onceClosureBefore(c *ssa.MakeClosure, site *ssa.Call) bool {
	if c.Parent() != site.Parent() || c.Block() == nil || site.Block() == nil {
		return false
	}
	ci, si, steps := -1, -1, 0
	for _, bb := range site.Parent().Blocks {
		for index, i := range bb.Instrs {
			steps++
			if steps > 4096 {
				return false
			}
			if i == c && i.Block() == bb {
				ci = index
			}
			if i == site && i.Block() == bb {
				si = index
			}
		}
	}
	return ci >= 0 && si >= 0 && c.Block().Dominates(site.Block()) && (c.Block() != site.Block() || ci < si)
}

func (a *Analyzer) ProveOnceCall(site *ssa.Call) *OnceProof {
	if !IsOnceDo(site) || len(site.Common().Args) != 2 {
		return nil
	}
	args := site.Common().Args
	f := a.onceMethod(site, "sync", "Once", "Do", args...)
	if f == nil || !a.onceShape(f) {
		return nil
	}
	proof := &OnceProof{Receiver: args[0], Operands: []ssa.Value{site.Common().Value, args[0], args[1]}}
	switch x := args[1].(type) {
	case *ssa.Function:
		proof.Callback = x
	case *ssa.MakeClosure:
		if !onceClosureBefore(x, site) {
			return nil
		}
		proof.Callback, _ = x.Fn.(*ssa.Function)
		proof.Captures = x.Bindings
		proof.Operands = append(proof.Operands, x.Fn)
		proof.Operands = append(proof.Operands, x.Bindings...)
	default:
		return nil
	}
	cb := proof.Callback
	if cb == nil || cb.Prog != a.program.SSA || cb.Syntax() == nil || cb.Synthetic != "" || len(cb.Blocks) == 0 || !emptyCallbackType(cb.Signature) || len(cb.FreeVars) != len(proof.Captures) || len(cb.TypeArgs()) != 0 {
		return nil
	}
	for i, v := range cb.FreeVars {
		if !types.Identical(v.Type(), proof.Captures[i].Type()) {
			return nil
		}
	}
	return proof
}
