package effects

import (
	"go/constant"
	"go/version"
	"strconv"
	"testing"

	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
)

func TestActualGoVersionInitializers(t *testing.T) {
	p := testutil.Load(t, `package main;import "go/types";func main(){_=types.NewPackage("fixture","fixture")}`)
	a := New(p, nil)
	proved, unknown := 0, 0
	for f := range p.Calls.Nodes {
		if f == nil || f.String() != "go/types.init" {
			continue
		}
		for _, bb := range f.Blocks {
			for _, i := range bb.Instrs {
				site, ok := i.(*ssa.Call)
				if !ok || site.Common().StaticCallee() == nil || site.Common().StaticCallee().String() != "go/types.asGoVersion" {
					continue
				}
				literal, ok := site.Common().Args[0].(*ssa.Const)
				if !ok {
					if a.ProveConstantDataCall(site) {
						t.Fatal("nonliteral version accepted")
					}
					unknown++
					continue
				}
				if proof := a.ProveConstantData(site); proof == nil || len(proof.ModeledOperations) != 1 {
					t.Fatal("actual version initializer refused")
				}
				e := &dataEval{program: p, sizes: p.Packages[0].TypesSizes, steps: 100000, cells: 65536, owned: map[*dataCell]bool{}, stack: map[*ssa.Function]bool{}}
				value := e.function(p.CallTarget(site), []dataValue{e.literal(literal)})[0]
				if constant.StringVal(value.scalar) != version.Lang(constant.StringVal(literal.Value)) {
					t.Fatal("actual initializer disagrees with native Go")
				}
				proved++
			}
		}
	}
	if proved < 10 || unknown == 0 {
		t.Fatalf("unexpected initializer inventory: proved=%d unknown=%d", proved, unknown)
	}
}

func TestConstantVersionModel(t *testing.T) {
	for _, input := range []string{"go1.25", "go1.21rc2", "go1.20.5-custom", "invalid", "go1", "go1.99999999999999999999"} {
		t.Run(input, func(t *testing.T) {
			p := testutil.Load(t, `package main;import "go/version";func main(){_=version.Lang(`+strconv.Quote(input)+`)}`)
			main, _ := p.Main()
			a := New(p, nil)
			for _, bb := range main.Blocks {
				for _, i := range bb.Instrs {
					if site, ok := i.(*ssa.Call); ok {
						proof := a.ProveConstantData(site)
						if proof == nil {
							for f := range p.Calls.Nodes {
								if f != nil && f.String() == "internal/bytealg.IndexByteString" {
									t.Logf("byte search: syntax %T, synthetic %q, source %v", f.Syntax(), f.Synthetic, p.Fset.Position(f.Pos()))
								}
							}
							t.Fatal("version evaluation refused")
						}
						if len(proof.ModeledOperations) != 1 {
							t.Fatal("missing declared byte-search model")
						}
						e := &dataEval{program: p, sizes: p.Packages[0].TypesSizes, steps: 100000, cells: 65536, owned: map[*dataCell]bool{}, stack: map[*ssa.Function]bool{}}
						value := e.function(p.CallTarget(site), []dataValue{e.literal(site.Common().Args[0].(*ssa.Const))})[0]
						if got := constant.StringVal(value.scalar); got != version.Lang(input) {
							t.Fatalf("got %q want %q", got, version.Lang(input))
						}
						return
					}
				}
			}
			t.Fatal("call missing")
		})
	}
}
