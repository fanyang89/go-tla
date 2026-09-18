package frontend

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/ssa"
)

// BoundMethodClosure validates receiver capture and current creation ordering.
func (p *Program) BoundMethodClosure(value ssa.Value, use ssa.Instruction) *ssa.Function {
	c, ok := value.(*ssa.MakeClosure)
	if !ok || c.Parent() != use.Parent() || c.Block() == nil || use.Block() == nil {
		return nil
	}
	f, ok := c.Fn.(*ssa.Function)
	if !ok {
		return nil
	}
	target := p.BoundMethodTarget(f)
	if target == nil || len(c.Bindings) != 1 || c.Bindings[0] == nil || !types.Identical(c.Bindings[0].Type(), f.FreeVars[0].Type()) {
		return nil
	}
	ci, ui, steps := -1, -1, 0
	for _, bb := range use.Parent().Blocks {
		for index, i := range bb.Instrs {
			steps++
			if steps > 4096 {
				return nil
			}
			if i == c && i.Block() == bb {
				ci = index
			}
			if i == use && i.Block() == bb {
				ui = index
			}
		}
	}
	if ci < 0 || ui < 0 || !c.Block().Dominates(use.Block()) || (c.Block() == use.Block() && ci >= ui) {
		return nil
	}
	return target
}

// BoundMethodTarget recognizes only the generated, adaptation-free forwarding
// wrapper. Its receiver is a captured value, not a later load of a source variable.
// This proof grants no purity, resource identity or permission to skip the body.
func (p *Program) BoundMethodTarget(f *ssa.Function) *ssa.Function {
	if f == nil || f.Prog != p.SSA || !strings.HasPrefix(f.Synthetic, "bound method wrapper") || f.Syntax() != nil || f.Signature == nil || f.Signature.Recv() != nil || f.Signature.Variadic() || f.Signature.Results().Len() != 0 || len(f.TypeArgs()) != 0 || len(f.FreeVars) != 1 || len(f.Params) != f.Signature.Params().Len() || len(f.Blocks) != 1 || f.Recover != nil {
		return nil
	}
	bb := f.Blocks[0]
	if bb.Parent() != f || bb.Index != 0 || len(bb.Preds) != 0 || len(bb.Succs) != 0 || len(bb.Instrs) != 2 {
		return nil
	}
	call, ok := bb.Instrs[0].(*ssa.Call)
	if !ok || call.Block() != bb || call.Common().IsInvoke() {
		return nil
	}
	ret, ok := bb.Instrs[1].(*ssa.Return)
	if !ok || ret.Block() != bb || len(ret.Results) != 0 {
		return nil
	}
	target := p.CallTarget(call)
	if target == nil || target != call.Common().StaticCallee() || target.Prog != p.SSA || target.Synthetic != "" || target.Object() == nil || f.Object() != target.Object() || target.Signature == nil || target.Signature.Recv() == nil || target.Signature.Variadic() || target.Signature.Results().Len() != 0 || target.Signature.Params().Len() != len(f.Params) || len(target.TypeArgs()) != 0 || len(target.Blocks) == 0 || !types.Identical(target.Signature, target.Object().Type()) {
		return nil
	}
	decl, ok := target.Syntax().(*ast.FuncDecl)
	if !ok || decl.Body == nil || decl.Name.Pos() != target.Pos() {
		return nil
	}
	recv := f.FreeVars[0]
	if recv.Parent() != f || !types.Identical(recv.Type(), target.Signature.Recv().Type()) {
		return nil
	}
	base := types.Unalias(recv.Type())
	if ptr, ok := base.(*types.Pointer); ok {
		base = types.Unalias(ptr.Elem())
	}
	named, ok := base.(*types.Named)
	if !ok || named.TypeArgs().Len() != 0 {
		return nil
	}
	method := types.NewMethodSet(recv.Type()).Lookup(target.Object().Pkg(), target.Name())
	if method == nil || method.Obj() != target.Object() {
		return nil
	}
	args := call.Common().Args
	if len(args) != len(f.Params)+1 || args[0] != recv {
		return nil
	}
	for i, param := range f.Params {
		if param.Parent() != f || args[i+1] != param || !types.Identical(param.Type(), f.Signature.Params().At(i).Type()) || !types.Identical(param.Type(), target.Signature.Params().At(i).Type()) {
			return nil
		}
	}
	return target
}
