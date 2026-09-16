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
		if !b.recordLoops(f) {
			return
		}
		if cslice.HasCycle(f) {
			b.diag("error", "initializer", "cyclic package initialization unsupported", f.Pos())
			return
		}
		for _, bb := range f.Blocks {
			for _, i := range bb.Instrs {
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
					if callee := b.effects.Call(x).Callee; callee != nil && callee.Name() == "init" {
						check(callee)
					} else if !b.effects.Call(x).IsPure() {
						b.diag("error", "initializer", "effectful or unknown package initialization unsupported", i.Pos())
					} else if summary := b.effects.Call(x); summary.Kind == effects.Pure {
						check(summary.Callee)
					}
				}
			}
		}
	}
	for _, p := range b.p.Roots {
		check(p.Func("init"))
	}
}
