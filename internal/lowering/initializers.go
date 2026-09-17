package lowering

import (
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/fanmi/go-tla/internal/discovery"
	"github.com/fanmi/go-tla/internal/effects"
	cslice "github.com/fanmi/go-tla/internal/slice"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

// Runtime initialization is an explicit environment summary, not an inferred
// purity claim. Only actual GOROOT packages below these narrow roots qualify.
func (b *builder) initializers() {
	roots := map[string]bool{"sync": true, "sync/atomic": true}
	for _, call := range b.opts.TrustedCalls {
		for _, pkg := range []string{"strings", "math"} {
			if strings.HasPrefix(call, pkg+".") {
				roots[pkg] = true
			}
		}
	}
	summarized := map[string]bool{}
	var summarize func(*packages.Package)
	summarize = func(p *packages.Package) {
		if summarized[p.PkgPath] || len(p.GoFiles) == 0 {
			return
		}
		if !strings.HasPrefix(p.GoFiles[0], filepath.Join(runtime.GOROOT(), "src")+string(filepath.Separator)) {
			return
		}
		summarized[p.PkgPath] = true
		for _, dep := range p.Imports {
			summarize(dep)
		}
	}
	packages.Visit(b.p.Packages, func(p *packages.Package) bool {
		if roots[p.PkgPath] {
			summarize(p)
		}
		return true
	}, nil)
	names := []string{}
	for name := range summarized {
		names = append(names, name)
	}
	slices.Sort(names)
	if len(names) > 0 {
		b.m.Assumptions = append(b.m.Assumptions, "Standard-library initialization summarized as completed before main: "+strings.Join(names, ", "))
		b.diag("info", "runtime-init-summary", b.m.Assumptions[len(b.m.Assumptions)-1], 0)
	}
	seen := map[*ssa.Function]bool{}
	var check func(*ssa.Function)
	check = func(f *ssa.Function) {
		if f == nil || seen[f] {
			return
		}
		seen[f] = true
		if f.Pkg != nil && summarized[f.Pkg.Pkg.Path()] {
			return
		}
		if b.effects.ProveFiniteData(f) {
			b.recordFiniteData(f)
			return
		}
		if !b.recordLoops(f) {
			return
		}
		if cslice.HasCycle(f) {
			b.diag("error", "initializer", "cyclic package initialization unsupported", f.Pos())
			return
		}
		for _, bb := range f.Blocks {
			for _, i := range bb.Instrs {
				if b.consumeClosedGlobal(i) {
					continue
				}
				b.checkUnsafePointer(i)
				if discovery.IsRoot(i) {
					b.diag("error", "initializer", "concurrency or channel construction in package init unsupported", i.Pos())
				}
				switch x := i.(type) {
				case *ssa.Panic, *ssa.Defer:
					b.diag("error", "initializer", "exception control in package initialization unsupported", i.Pos())
				case *ssa.UnOp:
					if inlineSync(x.Type()) || channelAggregate(x.Type()) {
						b.diag("error", "initializer", "loading synchronization aggregates by value in initializer unsupported", i.Pos())
					}
				case *ssa.Store:
					if discovery.SyncType(x.Val.Type()) != "" || inlineSync(x.Val.Type()) || channelAggregate(x.Val.Type()) {
						b.diag("error", "initializer", "synchronization object copying in initializer unsupported", i.Pos())
					}
				case *ssa.Call:
					summary := b.effects.Call(x)
					if summary.Callee != nil && b.p.CallTarget(x) != summary.Callee {
						b.diag("error", "call-contract", "initializer callee lacks a current matching call-graph proof: "+summary.Callee.String()+" (in "+f.String()+")", x.Pos())
						continue
					}
					if callee := summary.Callee; callee != nil && callee.Name() == "init" {
						check(callee)
					} else if !summary.IsPure() {
						target := "unresolved dynamic target"
						if callee := summary.Callee; callee != nil {
							target = callee.String()
						} else if callee := x.Common().StaticCallee(); callee != nil {
							target = "unproved static target " + callee.String()
						} else if x.Common().IsInvoke() {
							target = "unresolved interface method " + x.Common().Method.FullName()
						}
						b.diag("error", "initializer", "effectful or unknown package initialization unsupported: "+target+" (in "+f.String()+")", i.Pos())
					} else if summary.Kind == effects.Pure || summary.Kind == effects.FiniteData {
						if summary.Kind == effects.FiniteData && !b.effects.ProveFiniteData(summary.Callee) {
							b.diag("error", "finite-data-contract", "initializer computation lacks a current body/graph proof", x.Pos())
						} else {
							check(summary.Callee)
						}
					}
				}
			}
		}
	}
	for _, p := range b.p.Roots {
		check(p.Func("init"))
	}
}
