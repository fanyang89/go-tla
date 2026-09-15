// Package effects summarizes Go calls for slicing and conservative initializer checks.
// A non-pure summary is not permission to ignore a call: lowering must inspect or reject it.
package effects

import (
	"github.com/fanmi/go-tla/internal/discovery"
	"github.com/fanmi/go-tla/internal/frontend"
	cslice "github.com/fanmi/go-tla/internal/slice"
	"golang.org/x/tools/go/ssa"
)

type Kind string

const (
	Unknown   Kind = "unknown"
	Primitive Kind = "synchronization"
	Trusted   Kind = "trusted"
	Builtin   Kind = "sequential-builtin"
	Pure      Kind = "local-computation"
	Inspect   Kind = "inspect-body"
)

type Summary struct {
	Kind   Kind
	Callee *ssa.Function
}

func (s Summary) IsPure() bool { return s.Kind == Trusted || s.Kind == Builtin || s.Kind == Pure }

type Analyzer struct {
	program  *frontend.Program
	trusted  map[string]bool
	calls    map[ssa.CallInstruction]Summary
	pure     map[*ssa.Function]bool
	visiting map[*ssa.Function]bool
}

func New(p *frontend.Program, trusted []string) *Analyzer {
	a := &Analyzer{program: p, trusted: map[string]bool{}, calls: map[ssa.CallInstruction]Summary{}, pure: map[*ssa.Function]bool{}, visiting: map[*ssa.Function]bool{}}
	for _, name := range trusted {
		a.trusted[name] = true
	}
	return a
}
func (a *Analyzer) Call(site ssa.CallInstruction) Summary {
	if s, ok := a.calls[site]; ok {
		return s
	}
	s := Summary{Kind: Unknown, Callee: a.program.CallTarget(site)}
	switch {
	case discovery.IsRoot(site):
		s.Kind = Primitive
	case sequentialBuiltin(site.Common()):
		s.Kind = Builtin
	case s.Callee != nil && a.trusted[s.Callee.String()]:
		s.Kind = Trusted
	case s.Callee != nil && len(s.Callee.Blocks) > 0:
		s.Kind = Inspect
		if a.pureFunction(s.Callee) {
			s.Kind = Pure
		}
	}
	a.calls[site] = s
	return s
}
func (a *Analyzer) pureFunction(f *ssa.Function) (pure bool) {
	if p, ok := a.pure[f]; ok {
		return p
	}
	if a.visiting[f] || len(f.Blocks) == 0 || cslice.HasCycle(f) {
		return false
	}
	a.visiting[f] = true
	defer func() { delete(a.visiting, f); a.pure[f] = pure }()
	for _, bb := range f.Blocks {
		for _, i := range bb.Instrs {
			if discovery.IsRoot(i) || frontend.UsesUnsafePointer(i) {
				return false
			}
			switch x := i.(type) {
			case *ssa.Defer, *ssa.Panic:
				return false
			case *ssa.Call:
				if !a.Call(x).IsPure() {
					return false
				}
			case *ssa.Store:
				if alloc, ok := x.Addr.(*ssa.Alloc); !ok || alloc.Heap {
					return false
				}
			}
		}
	}
	return true
}
func sequentialBuiltin(c *ssa.CallCommon) bool {
	b, ok := c.Value.(*ssa.Builtin)
	if !ok {
		return false
	}
	switch b.Name() {
	case "len", "cap", "append", "copy", "delete", "clear", "min", "max", "complex", "real", "imag", "print", "println":
		return true
	}
	return false
}
