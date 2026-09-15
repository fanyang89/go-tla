package frontend

import "golang.org/x/tools/go/ssa"

// CallTarget requires agreement between SSA's direct target and the explicit
// static call graph. Builtins and unresolved dynamic calls have no direct target.
func (p *Program) CallTarget(site ssa.CallInstruction) *ssa.Function {
	if p.Calls == nil {
		return nil
	}
	callee := site.Common().StaticCallee()
	if callee == nil {
		return nil
	}
	node := p.Calls.Nodes[site.Parent()]
	if node == nil {
		return nil
	}
	for _, edge := range node.Out {
		if edge.Site == site && edge.Callee.Func == callee {
			return callee
		}
	}
	return nil
}
