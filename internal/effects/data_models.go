package effects

import (
	"go/ast"
	"go/constant"
	"go/types"
	"path/filepath"
	"runtime"
	"slices"

	"golang.org/x/tools/go/ssa"
)

// ByteSearchModel is an explicit standard-toolchain semantics assumption, not
// a purity proof of assembly. Each examined byte is evaluated under the shared
// execution budget; the result participates in the surrounding SSA execution.
const ByteSearchModel = "Standard Go internal/bytealg.IndexByteString returns the first equal byte index or -1, without application-visible effects; assembly correctness is assumed, not proved."

func (e *dataEval) modeledDataFunction(f *ssa.Function, args []dataValue) (dataValue, bool) {
	if f.Pkg == nil || f.Pkg.Pkg.Path() != "internal/bytealg" || f.Name() != "IndexByteString" || len(f.Blocks) != 0 || f.Synthetic != "" || len(f.TypeArgs()) != 0 {
		return dataValue{}, false
	}
	sig := f.Signature
	if sig.Recv() != nil || sig.Variadic() || sig.Params().Len() != 2 || sig.Results().Len() != 1 || len(args) != 2 ||
		!types.Identical(sig.Params().At(0).Type(), types.Typ[types.String]) || !types.Identical(sig.Params().At(1).Type(), types.Typ[types.Uint8]) || !types.Identical(sig.Results().At(0).Type(), types.Typ[types.Int]) {
		return dataValue{}, false
	}
	// Bind the declaration to the loaded standard-library source, not a user
	// function with the same spelling or a removed/overridden SSA body.
	decl, ok := f.Syntax().(*ast.FuncDecl)
	file := filepath.Clean(e.program.Fset.Position(f.Pos()).Filename)
	if !ok || decl.Body != nil || decl.Name.Name != "IndexByteString" || f.Object() == nil || f.Pkg.Pkg.Scope().Lookup(f.Name()) != f.Object() ||
		filepath.Dir(file) != filepath.Join(runtime.GOROOT(), "src", "internal", "bytealg") || e.program.Sources[file].Package != "internal/bytealg" {
		return dataValue{}, false
	}
	e.checkScalar(args[0])
	e.checkScalar(args[1])
	if args[0].scalar.Kind() != constant.String || args[1].scalar.Kind() != constant.Int {
		refuseData()
	}
	s := constant.StringVal(args[0].scalar)
	needle := e.integer(args[1])
	if needle < 0 || needle > 255 {
		refuseData()
	}
	index := -1
	for i := range len(s) {
		e.step()
		if int(s[i]) == needle {
			index = i
			break
		}
	}
	result := dataValue{typ: sig.Results().At(0).Type(), scalar: constant.MakeInt64(int64(index))}
	e.checkScalar(result)
	if !slices.Contains(e.modeledOperations, ByteSearchModel) {
		e.modeledOperations = append(e.modeledOperations, ByteSearchModel)
	}
	return result, true
}
