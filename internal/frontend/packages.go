// Package frontend loads real Go packages with type information and constructs SSA.
package frontend

import (
	"context"
	"fmt"
	"go/token"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/static"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

type Program struct {
	Packages []*packages.Package
	SSA      *ssa.Program
	Roots    []*ssa.Package
	Fset     *token.FileSet
	Calls    *callgraph.Graph
	Sources  map[string]SourceFile
}

// Load is pass 1a. Dir permits callers/tests to analyze an independent module.
func Load(dir string, patterns ...string) ([]*packages.Package, error) {
	return LoadContext(context.Background(), dir, patterns...)
}

func LoadContext(ctx context.Context, dir string, patterns ...string) ([]*packages.Package, error) {
	ps, err := packages.Load(&packages.Config{Mode: packages.LoadAllSyntax | packages.NeedModule, Dir: dir, Context: ctx}, patterns...)
	if err != nil {
		return nil, err
	}
	var loadErr error
	packages.Visit(ps, nil, func(p *packages.Package) {
		if len(p.Errors) > 0 && loadErr == nil {
			loadErr = fmt.Errorf("load %s: %s", p.PkgPath, p.Errors[0])
		}
	})
	if loadErr != nil {
		return nil, loadErr
	}
	if len(ps) == 0 {
		return nil, fmt.Errorf("no packages matched")
	}
	return ps, nil
}

// BuildSSA is pass 1b. Dependencies retain bodies so unknown effects are not inferred pure.
func BuildSSA(ps []*packages.Package) *Program {
	prog, roots := ssautil.AllPackages(ps, ssa.InstantiateGenerics)
	prog.Build()
	return &Program{Packages: ps, SSA: prog, Roots: roots, Fset: prog.Fset, Sources: sourceIndex(ps)}
}

// BuildCallGraph is pass 2. Unresolved dynamic sites are rejected during reachable lowering.
func BuildCallGraph(p *Program) { p.Calls = static.CallGraph(p.SSA) }
func (p *Program) Main() (*ssa.Function, error) {
	var main *ssa.Function
	for _, pkg := range p.Roots {
		if f := pkg.Func("main"); f != nil {
			if main != nil {
				return nil, fmt.Errorf("select exactly one main package")
			}
			main = f
		}
	}
	if main == nil {
		return nil, fmt.Errorf("MVP requires a main package entry point")
	}
	return main, nil
}
