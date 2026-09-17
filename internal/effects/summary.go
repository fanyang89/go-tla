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
	Unknown      Kind = "unknown"
	Primitive    Kind = "synchronization"
	Trusted      Kind = "trusted"
	Builtin      Kind = "sequential-builtin"
	Pure         Kind = "local-computation"
	Inspect      Kind = "inspect-body"
	FiniteData   Kind = "finite-read-only-computation"
	ConstantData Kind = "literal-input-data-computation"
	ScalarFormat Kind = "modeled-scalar-formatting"
)

type Summary struct {
	Kind   Kind
	Callee *ssa.Function
}

func (s Summary) IsPure() bool {
	return s.Kind == Trusted || s.Kind == Builtin || s.Kind == Pure || s.Kind == FiniteData || s.Kind == ConstantData || s.Kind == ScalarFormat
}

type Analyzer struct {
	program  *frontend.Program
	trusted  map[string]bool
	calls    map[ssa.CallInstruction]Summary
	pure     map[*ssa.Function]bool
	visiting map[*ssa.Function]bool
	finite   map[*ssa.Function]bool
}

func New(p *frontend.Program, trusted []string) *Analyzer {
	a := &Analyzer{program: p, trusted: map[string]bool{}, calls: map[ssa.CallInstruction]Summary{}, pure: map[*ssa.Function]bool{}, visiting: map[*ssa.Function]bool{}, finite: map[*ssa.Function]bool{}}
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
	if d, ok := site.(*ssa.Defer); ok {
		if _, supported := discovery.Deferred(d); supported && s.Callee != nil {
			s.Kind = Primitive
		}
		a.calls[site] = s
		return s
	}
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
		} else {
			proved, known := a.finite[s.Callee]
			if !known {
				proved = a.ProveFiniteData(s.Callee)
				a.finite[s.Callee] = proved
			}
			if proved {
				s.Kind = FiniteData
			} else if call, ok := site.(*ssa.Call); ok && a.ProveConstantDataCall(call) {
				s.Kind = ConstantData
			}
		}
	}
	if s.Kind != Primitive && s.Kind != Trusted {
		if call, ok := site.(*ssa.Call); ok && a.ProveScalarFormat(call) != nil {
			s.Kind = ScalarFormat
		}
	}
	a.calls[site] = s
	return s
}
func (a *Analyzer) pureFunction(f *ssa.Function) (pure bool) {
	for _, proof := range a.program.LoopProofs(f) {
		if proof.Reason != "" {
			return false
		}
	}
	if p, ok := a.pure[f]; ok {
		return p
	}
	if a.visiting[f] || len(f.Blocks) == 0 || cslice.HasCycle(f) {
		return false
	}
	a.visiting[f] = true
	defer func() { delete(a.visiting, f); a.pure[f] = pure }()
	private := map[*ssa.Alloc]bool{}
	for _, bb := range f.Blocks {
		for _, i := range bb.Instrs {
			if discovery.IsRoot(i) || frontend.UsesUnsafePointer(i) {
				return false
			}
			switch x := i.(type) {
			case *ssa.Defer, *ssa.Panic:
				return false
			case *ssa.Call:
				summary := a.Call(x)
				if !summary.IsPure() || summary.Kind == Builtin && !readOnlyBuiltin(x.Common()) {
					return false
				}
			case *ssa.Store:
				if alloc, ok := x.Addr.(*ssa.Alloc); !ok || alloc.Heap {
					if !privateDataStore(x, private) {
						return false
					}
				}
			case *ssa.MapUpdate:
				// A private allocation elsewhere does not excuse a shared map write.
				return false
			}
		}
	}
	return true
}

// OutputBuiltin identifies language-level output, never a same-named source function.
func OutputBuiltin(site ssa.CallInstruction) string {
	if builtin, ok := site.Common().Value.(*ssa.Builtin); ok && (builtin.Name() == "print" || builtin.Name() == "println") {
		return builtin.Name()
	}
	return ""
}

func sequentialBuiltin(c *ssa.CallCommon) bool {
	b, ok := c.Value.(*ssa.Builtin)
	if !ok {
		return false
	}
	switch b.Name() {
	case "len", "cap", "append", "copy", "delete", "clear", "min", "max", "complex", "real", "imag":
		return true
	}
	return false
}
