package lowering

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/types"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/fanmi/go-tla/internal/abstract"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func (b *builder) runtimeGetter() *ssa.Function {
	for _, pkg := range b.p.SSA.AllPackages() {
		if pkg.Pkg.Path() != "runtime" {
			continue
		}
		// This initial environment contract is deliberately target-specific.
		for name, want := range map[string]string{"GOOS": "linux", "GOARCH": "amd64"} {
			c, ok := pkg.Pkg.Scope().Lookup(name).(*types.Const)
			if !ok || c.Val().Kind() != constant.String || constant.StringVal(c.Val()) != want {
				return nil
			}
		}
		f := pkg.Func("GOMAXPROCS")
		if f == nil || f.Pkg != pkg || f.Object() == nil || pkg.Pkg.Scope().Lookup("GOMAXPROCS") != f.Object() {
			return nil
		}
		sig := f.Signature
		if sig == nil || sig.Recv() != nil || sig.Variadic() || sig.Params().Len() != 1 || sig.Results().Len() != 1 || !types.Identical(sig.Params().At(0).Type(), types.Typ[types.Int]) || !types.Identical(sig.Results().At(0).Type(), types.Typ[types.Int]) || !types.Identical(sig, f.Object().Type()) {
			return nil
		}
		decl, ok := f.Syntax().(*ast.FuncDecl)
		if !ok || decl.Body == nil || decl.Name.Name != "GOMAXPROCS" || decl.Name.Pos() != f.Pos() {
			return nil
		}
		file := filepath.Clean(b.p.Fset.Position(f.Pos()).Filename)
		if filepath.Dir(file) != filepath.Join(runtime.GOROOT(), "src", "runtime") || b.p.Sources[file].Package != "runtime" {
			return nil
		}
		return f
	}
	return nil
}

func (b *builder) runtimeQuery(site *ssa.Call, getter *ssa.Function) bool {
	if getter == nil || site == nil || site.Common().IsInvoke() || site.Common().StaticCallee() != getter || b.p.CallTarget(site) != getter || len(site.Common().Args) != 1 || !types.Identical(site.Type(), types.Typ[types.Int]) {
		return false
	}
	arg := site.Common().Args[0]
	n, ok := abstract.Integer(arg)
	return ok && n == 0 && types.Identical(arg.Type(), types.Typ[types.Int])
}

// Every current reference is checked, including functions omitted from the call
// graph. Runtime internals implement the declared standard-runtime boundary;
// application/dependency writes, resets and escaped API values are not exempt.
func (b *builder) runtimeProcsContract() bool {
	if b.opts.RuntimeProcs == 0 || b.opts.Validate() != nil {
		return false
	}
	setting := b.m.Metadata.Options["startup.GOMAXPROCS"]
	if len(setting) != 1 || setting[0] != strconv.Itoa(b.opts.RuntimeProcs) {
		return false
	}
	getter := b.runtimeGetter()
	if getter == nil {
		return false
	}
	remaining := 1000000
	for f := range ssautil.AllFunctions(b.p.SSA) {
		if f == nil {
			continue
		}
		origin := f.Origin()
		if origin == nil {
			origin = f
		}
		if origin.Pkg == getter.Pkg {
			continue
		}
		for _, bb := range f.Blocks {
			for _, i := range bb.Instrs {
				remaining--
				if remaining < 0 {
					return false
				}
				if _, debug := i.(*ssa.DebugRef); debug {
					continue
				}
				for _, operand := range i.Operands(nil) {
					if operand == nil {
						continue
					}
					target, ok := (*operand).(*ssa.Function)
					if !ok || target.Pkg == nil || target.Pkg.Pkg.Path() != "runtime" {
						continue
					}
					switch target.Name() {
					case "SetDefaultGOMAXPROCS":
						return false
					case "GOMAXPROCS":
						call, ok := i.(*ssa.Call)
						if !ok || call.Common().Value != target || target != getter || !b.runtimeQuery(call, getter) {
							return false
						}
					}
				}
			}
		}
	}
	return true
}

func (b *builder) prepareRuntimeProcs() {
	if b.opts.RuntimeProcs == 0 {
		return
	}
	if !b.runtimeProcsContract() {
		b.diag("error", "runtime-procs-contract", "startup GOMAXPROCS requires a checked linux/amd64 runtime and a complete mutation/escape-free API inventory", 0)
		return
	}
	assumption := fmt.Sprintf("Conditional runtime environment: ordinary linux/amd64 Go process starts with GOMAXPROCS=%d, disabling automatic updates. Standard-runtime query semantics are assumed, not proved; application/dependency setters, resets, API escapes and trusted calls are refused. Other input/environment effects remain unsupported unless independently modeled.", b.opts.RuntimeProcs)
	b.m.Assumptions = append(b.m.Assumptions, assumption)
	b.diag("info", "runtime-procs-profile", assumption, 0)
}

func (b *builder) consumeRuntimeQuery(fr *frame, site *ssa.Call) bool {
	if b.opts.RuntimeProcs == 0 || !b.runtimeQuery(site, b.runtimeGetter()) {
		return false
	}
	valid := b.runtimeProcsContract()
	if fr != nil && fr.plan.Calls[site].Callee != b.p.CallTarget(site) {
		valid = false
	}
	if fr != nil {
		for _, operand := range site.Operands(nil) {
			if operand != nil && !b.requireData(fr, *operand) {
				valid = false
			}
		}
	}
	if !valid {
		b.diag("error", "runtime-procs-contract", "runtime query lacks a current environment/signature/graph/argument proof", site.Pos())
		return false
	}
	b.diag("info", "runtime-procs-query", fmt.Sprintf("modeled runtime.GOMAXPROCS(0) under startup GOMAXPROCS=%d; only direct channel capacities consume this exact value", b.opts.RuntimeProcs), site.Pos())
	return true
}

func (b *builder) channelCapacity(v ssa.Value) (int, bool) {
	if n, ok := abstract.Integer(v); ok {
		return n, true
	}
	call, ok := v.(*ssa.Call)
	if !ok || b.opts.RuntimeProcs == 0 || !b.runtimeQuery(call, b.runtimeGetter()) || !b.runtimeProcsContract() {
		return 0, false
	}
	return b.opts.RuntimeProcs, true
}
