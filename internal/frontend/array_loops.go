package frontend

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"go/version"
	"io"
)

// Restrict expansion to by-value scalar arrays with declared iteration values.
// Two-value range evaluates and snapshots its array exactly once, including for
// length zero. Key-only and pointer-array ranges have different evaluation rules.
func arrayRangeCount(info *types.Info, file *ast.File, loop *ast.RangeStmt) (int, string) {
	a, ok := info.TypeOf(loop.X).Underlying().(*types.Array)
	if !ok {
		return 0, "range is not a by-value array"
	}
	if version.Compare(info.FileVersions[file], "go1.22") < 0 {
		return 0, "array value range requires proved Go 1.22+ per-iteration variable semantics"
	}
	key, keyOK := loop.Key.(*ast.Ident)
	value, valueOK := loop.Value.(*ast.Ident)
	if loop.Tok != token.DEFINE || !keyOK || !valueOK || value.Name == "_" || key == nil {
		return 0, "array range requires declared index/value identifiers and a nonblank value"
	}
	elem, ok := a.Elem().Underlying().(*types.Basic)
	if !ok || elem.Info()&(types.IsBoolean|types.IsInteger|types.IsFloat|types.IsComplex|types.IsString) == 0 {
		return 0, "array range requires scalar data elements; resource/reference copies are unsupported"
	}
	if a.Len() > MaxLoopIterations {
		return 0, "constant array range exceeds 16 iterations; not truncated"
	}
	return int(a.Len()), ""
}

func arrayRangeBindings(w io.Writer, loop *ast.RangeStmt, temporary string, index int) {
	key := loop.Key.(*ast.Ident).Name
	value := loop.Value.(*ast.Ident).Name
	if key != "_" {
		fmt.Fprintf(w, "%s := %d; _ = %s\n", key, index, key)
	}
	// Slice indexing also type-checks for a zero-length array. The zero-trip
	// binding is inside if false, preserving body/import checks without executing it.
	fmt.Fprintf(w, "%s := %s[:][%d]; _ = %s\n", value, temporary, index, value)
}
