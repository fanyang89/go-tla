// Package frontend loads real Go packages with type information and constructs SSA.
package frontend

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
	"sync"
)

type Program struct {
	Packages []*packages.Package
	SSA      *ssa.Program
	Roots    []*ssa.Package
	Fset     *token.FileSet
	Calls    *callgraph.Graph
	Sources  map[string]SourceFile
	loops    map[loopOwner][]LoopProof
}

// Load is pass 1a. Dir permits callers/tests to analyze an independent module.
func Load(dir string, patterns ...string) (*Loaded, error) {
	return LoadContext(context.Background(), dir, patterns...)
}

func LoadContext(ctx context.Context, dir string, patterns ...string) (*Loaded, error) {
	sources := map[string][]byte{}
	var mu sync.Mutex
	config := &packages.Config{Mode: packages.LoadAllSyntax | packages.NeedModule, Dir: dir, Context: ctx}
	config.ParseFile = func(fset *token.FileSet, filename string, src []byte) (*ast.File, error) {
		mu.Lock()
		sources[filename] = bytes.Clone(src)
		mu.Unlock()
		return parser.ParseFile(fset, filename, src, parser.ParseComments|parser.SkipObjectResolution)
	}
	ps, err := loadPackages(config, patterns...)
	if err != nil {
		return nil, err
	}
	overlay, proofs := normalizeLoops(ps, sources)
	clear(sources)
	if len(overlay) > 0 {
		config.Overlay = overlay
		config.ParseFile = nil
		ps, err = loadPackages(config, patterns...)
		if err != nil {
			return nil, fmt.Errorf("type-check normalized finite loops: %w", err)
		}
	}
	return &Loaded{Packages: ps, loops: proofs}, nil
}

func loadPackages(config *packages.Config, patterns ...string) ([]*packages.Package, error) {
	ps, err := packages.Load(config, patterns...)
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
func BuildSSA(loaded *Loaded) *Program {
	ps := loaded.Packages
	prog, roots := ssautil.AllPackages(ps, ssa.InstantiateGenerics)
	prog.Build()
	return &Program{Packages: ps, SSA: prog, Roots: roots, Fset: prog.Fset, Sources: sourceIndex(ps), loops: loaded.loops}
}

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
