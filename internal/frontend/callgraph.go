package frontend

import (
	"go/types"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/static"
	"golang.org/x/tools/go/ssa"
)

// BuildCallGraph is pass 2. Refine direct edges only with exact local interface
// boxing proofs. No points-to guess or whole-program implementer enumeration is
// used. A separate immutable-field proof can refine single-origin field calls.
func BuildCallGraph(p *Program) {
	p.Calls = static.CallGraph(p.SSA)
	queue := make([]*ssa.Function, 0, len(p.Calls.Nodes))
	for f := range p.Calls.Nodes {
		if f != nil {
			queue = append(queue, f)
		}
	}
	seen := map[*ssa.Function]bool{}
	for len(queue) > 0 {
		f := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		if seen[f] {
			continue
		}
		seen[f] = true
		caller := p.Calls.CreateNode(f)
		for _, bb := range f.Blocks {
			for _, i := range bb.Instrs {
				site, ok := i.(ssa.CallInstruction)
				if !ok {
					continue
				}
				target := site.Common().StaticCallee()
				if target == nil {
					target, _ = p.directInvoke(site)
				}
				if target == nil {
					continue
				}
				if !hasCallEdge(caller, site, target) {
					callgraph.AddEdge(caller, site, p.Calls.CreateNode(target))
				}
				// Include the implementation's own direct calls even if it was
				// absent from the original static graph.
				if !seen[target] {
					queue = append(queue, target)
				}
			}
		}
	}
	p.refineCallableFields()
}

// CallTarget requires agreement between the SSA target proof and the explicit
// graph. Builtins and unresolved dynamic calls have no target.
func (p *Program) CallTarget(site ssa.CallInstruction) *ssa.Function {
	if p.Calls == nil {
		return nil
	}
	callee := site.Common().StaticCallee()
	if callee == nil {
		callee, _ = p.directInvoke(site)
	}
	if callee == nil {
		if proof := p.fieldCall(site); proof != nil {
			callee = proof.Target
		}
	}
	if callee != nil && hasCallEdge(p.Calls.Nodes[site.Parent()], site, callee) {
		return callee
	}
	return nil
}

// InvokeReceiver returns the concrete receiver only for a graph-checked invoke.
// CallCommon.Args omits it for invokes, unlike an ordinary concrete method call.
func (p *Program) InvokeReceiver(site ssa.CallInstruction) ssa.Value {
	callee, receiver := p.directInvoke(site)
	if callee == nil || p.CallTarget(site) != callee {
		return nil
	}
	return receiver
}

func hasCallEdge(node *callgraph.Node, site ssa.CallInstruction, callee *ssa.Function) bool {
	if node == nil {
		return false
	}
	for _, edge := range node.Out {
		if edge.Site == site && edge.Callee.Func == callee {
			return true
		}
	}
	return false
}

// directInvoke accepts a concrete named value (or its pointer) boxed at this call
// site's function. Interface widening preserves the same box. Promotions, implicit
// pointer/value adaptation and generic instantiations require separate proofs.
func (p *Program) directInvoke(site ssa.CallInstruction) (*ssa.Function, ssa.Value) {
	c := site.Common()
	if !c.IsInvoke() {
		return nil, nil
	}
	v := c.Value
	for {
		change, ok := v.(*ssa.ChangeInterface)
		if !ok {
			break
		}
		if change.Parent() != site.Parent() {
			return nil, nil
		}
		v = change.X
	}
	box, ok := v.(*ssa.MakeInterface)
	if !ok || box.Parent() != site.Parent() {
		return nil, nil
	}
	f := p.concreteMethod(c.Method, box.X)
	if f == nil {
		return nil, nil
	}
	return f, box.X
}

func (p *Program) concreteMethod(want *types.Func, receiver ssa.Value) *ssa.Function {
	if constant, ok := receiver.(*ssa.Const); ok && constant.IsNil() {
		return nil
	}
	t := types.Unalias(receiver.Type())
	if pointer, ok := t.(*types.Pointer); ok {
		t = types.Unalias(pointer.Elem())
	}
	named, ok := t.(*types.Named)
	if !ok || named.TypeArgs().Len() != 0 || named.TypeParams().Len() != 0 {
		return nil
	}
	if pkg := named.Obj().Pkg(); pkg != nil && (pkg.Path() == "sync" || pkg.Path() == "sync/atomic") {
		// Primitive invokes require dedicated discovery semantics, not trust.
		return nil
	}
	selection := types.NewMethodSet(receiver.Type()).Lookup(want.Pkg(), want.Name())
	if selection == nil || len(selection.Index()) != 1 {
		return nil
	}
	method, ok := selection.Obj().(*types.Func)
	if !ok {
		return nil
	}
	sig := method.Type().(*types.Signature)
	if sig.Recv() == nil || !types.Identical(sig.Recv().Type(), receiver.Type()) {
		return nil
	}
	return p.SSA.FuncValue(method)
}
