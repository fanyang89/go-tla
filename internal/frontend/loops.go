package frontend

import (
	"bytes"
	"cmp"
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"strings"

	"github.com/fanmi/go-tla/internal/behavior"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

// Expansion limits cause explicit rejection, never a truncated model.
const MaxLoopIterations = 16
const MaxLoopExpansionBytes = 256 * 1024

type loopOwner struct {
	Package, File string
	Line, Column  int
}

// LoopProof is a consumed, source-attributed normalization result.
type LoopProof struct {
	Source       behavior.Position
	Count        int
	ArrayRange   bool   // Snapshot-based scalar array expansion.
	ChannelRange bool   // Candidate only: SSA must prove finite-state receive control.
	Reason       string // Nonempty means the loop was retained and must be refused if reached.
}

// Loaded keeps re-type-checked packages and their original-source loop proofs.
type Loaded struct {
	Packages []*packages.Package
	loops    map[loopOwner][]LoopProof
}
type loopEdit struct {
	start, end int
	text       []byte
}

func ownerKey(pkg string, pos token.Position) loopOwner {
	return loopOwner{pkg, filepath.Clean(pos.Filename), pos.Line, pos.Column}
}
func (p *Program) LoopProofs(f *ssa.Function) []LoopProof {
	if f == nil || f.Pkg == nil {
		return nil
	}
	return p.loops[ownerKey(f.Pkg.Pkg.Path(), p.Fset.Position(f.Pos()))]
}

// normalizeLoops runs only after the original program type-checks. The replacement
// is Go source (not IR), reloaded/type-checked before SSA construction. It duplicates
// lexical blocks, never wraps iterations in functions, so return/defer scope is kept.
func normalizeLoops(ps []*packages.Package, sources map[string][]byte) (map[string][]byte, map[loopOwner][]LoopProof) {
	overlay := map[string][]byte{}
	proofs := map[loopOwner][]LoopProof{}
	index := sourceIndex(ps)
	packages.Visit(ps, nil, func(p *packages.Package) {
		if p.Module == nil || !p.Module.Main {
			return
		}
		positions := &Program{Fset: p.Fset, Sources: index}
		for _, file := range p.Syntax {
			filename := p.Fset.PositionFor(file.Pos(), false).Filename
			src, ok := sources[filename]
			if !ok {
				continue
			}
			lineDirectives := false
			for _, group := range file.Comments {
				for _, comment := range group.List {
					if strings.HasPrefix(comment.Text, "//line ") || strings.HasPrefix(comment.Text, "/*line ") {
						lineDirectives = true
					}
				}
			}
			names := map[string]bool{}
			ast.Inspect(file, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok {
					names[id.Name] = true
				}
				return true
			})
			var edits []loopEdit
			expandedBytes := 0
			var inspectFunction func(ast.Node, token.Pos)
			inspectFunction = func(body ast.Node, owner token.Pos) {
				key := ownerKey(p.PkgPath, p.Fset.Position(owner))
				ast.Inspect(body, func(n ast.Node) bool {
					if lit, ok := n.(*ast.FuncLit); ok {
						inspectFunction(lit.Body, lit.Pos())
						return false
					}
					loop, ok := n.(*ast.RangeStmt)
					if !ok {
						return true
					}
					proof := LoopProof{Source: positions.Position(loop.Pos(), nil)}
					proof.Source.Package = p.PkgPath
					if _, ok := p.TypesInfo.TypeOf(loop.X).Underlying().(*types.Chan); ok {
						proof.ChannelRange = true
						proofs[key] = append(proofs[key], proof)
						return false
					}
					count, reason := rangeCount(p.TypesInfo, loop)
					_, arrayRange := p.TypesInfo.TypeOf(loop.X).Underlying().(*types.Array)
					if arrayRange {
						count, reason = arrayRangeCount(p.TypesInfo, file, loop)
					}
					proof.ArrayRange = arrayRange
					proof.Count, proof.Reason = count, reason
					if reason == "" {
						ast.Inspect(loop.Body, func(n ast.Node) bool {
							switch n.(type) {
							case *ast.ForStmt, *ast.RangeStmt, *ast.BranchStmt, *ast.LabeledStmt:
								proof.Reason = "nested loops, labels and branch statements in a range body are unsupported"
								return false
							}
							return true
						})
					}
					if proof.Reason == "" && (lineDirectives || strings.ContainsAny(filename, "\r\n") || strings.Contains(filename, "*/")) {
						proof.Reason = "loop expansion with existing line directives or directive-unsafe filenames is unsupported"
					}
					if proof.Reason == "" {
						start := p.Fset.PositionFor(loop.Pos(), false).Offset
						end := p.Fset.PositionFor(loop.End(), false).Offset
						bodyStart := p.Fset.PositionFor(loop.Body.Pos(), false)
						bodyEnd := p.Fset.PositionFor(loop.Body.End(), false).Offset
						bodyText := src[bodyStart.Offset:bodyEnd]
						// Zero-trip bodies remain type-checked (including import uses), but cannot
						// execute. Positive counts get distinct lexical blocks and SSA allocations.
						copies := max(count, 1)
						if len(bodyText) > (MaxLoopExpansionBytes-expandedBytes)/copies {
							proof.Reason = "constant range exceeds the 256 KiB per-file expansion budget; not truncated"
						} else {
							var text bytes.Buffer
							temporary := fmt.Sprintf("__gotla_array_%d", start)
							for names[temporary] {
								temporary += "_"
							}
							names[temporary] = true
							text.WriteString("{\n")
							if arrayRange {
								fmt.Fprintf(&text, "%s := ", temporary)
							} else {
								text.WriteString("_ = ")
							}
							exprStart := p.Fset.PositionFor(loop.X.Pos(), false)
							exprEnd := p.Fset.PositionFor(loop.X.End(), false).Offset
							fmt.Fprintf(&text, "/*line %s:%d:%d*/", filename, exprStart.Line, exprStart.Column)
							text.Write(src[exprStart.Offset:exprEnd])
							text.WriteByte('\n')
							if arrayRange {
								fmt.Fprintf(&text, "_ = %s\n", temporary)
							}
							for iteration := range copies {
								if count == 0 {
									text.WriteString("if false ")
								}
								if arrayRange {
									text.WriteString("{\n")
									arrayRangeBindings(&text, loop, temporary, iteration)
								}
								// A block comment directive preserves the opening brace's exact column.
								fmt.Fprintf(&text, "/*line %s:%d:%d*/", filename, bodyStart.Line, bodyStart.Column)
								text.Write(bodyText)
								text.WriteByte('\n')
								if arrayRange {
									text.WriteString("}\n")
								}
							}
							text.WriteString("}\n")
							after := p.Fset.PositionFor(loop.End(), false)
							fmt.Fprintf(&text, "//line %s:%d:%d\n", filename, after.Line, after.Column)
							replacement := text.Bytes()
							if len(replacement) > MaxLoopExpansionBytes-expandedBytes {
								proof.Reason = "constant range exceeds the 256 KiB per-file expansion budget; not truncated"
							} else {
								expandedBytes += len(replacement)
								edits = append(edits, loopEdit{start, end, slices.Clone(replacement)})
							}
						}
					}
					proofs[key] = append(proofs[key], proof)
					// Nested edits would overlap copied syntax; nested forms are explicitly refused.
					return false
				})
			}
			for _, decl := range file.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
					inspectFunction(fn.Body, fn.Name.Pos())
				}
			}
			if len(edits) == 0 {
				continue
			}
			slices.SortFunc(edits, func(a, b loopEdit) int { return cmp.Compare(a.start, b.start) })
			var out bytes.Buffer
			offset := 0
			for _, edit := range edits {
				out.Write(src[offset:edit.start])
				out.Write(edit.text)
				offset = edit.end
			}
			out.Write(src[offset:])
			overlay[filename] = out.Bytes()
		}
	})
	return overlay, proofs
}

func rangeCount(info *types.Info, loop *ast.RangeStmt) (int, string) {
	if loop.Key != nil {
		id, ok := loop.Key.(*ast.Ident)
		if !ok || id.Name != "_" {
			return 0, "only integer ranges without an iteration variable are supported"
		}
	}
	if loop.Value != nil {
		return 0, "integer range must not bind a second value"
	}
	value := info.Types[loop.X].Value
	if value == nil || value.Kind() != constant.Int {
		return 0, "range bound must be a compile-time integer constant"
	}
	bound, ok := constant.Int64Val(value)
	if !ok || bound > MaxLoopIterations {
		return 0, "constant range exceeds 16 iterations; not truncated"
	}
	if bound <= 0 {
		return 0, ""
	}
	return int(bound), ""
}
