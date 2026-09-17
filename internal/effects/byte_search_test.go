package effects

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/types"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
)

func TestConstantByteSearchNative(t *testing.T) {
	var source strings.Builder
	source.WriteString(`package main;import "strings";func main(){`)
	for _, s := range []string{"", "aba", "\x00a", "é"} {
		for _, needle := range []byte{0, 'a', 0xc3, 0xa9, 255} {
			fmt.Fprintf(&source, "_=strings.IndexByte(%q,%d);", s, needle)
		}
	}
	source.WriteString("}")
	p := testutil.Load(t, source.String())
	main, _ := p.Main()
	a := New(p, nil)
	count := 0
	for _, bb := range main.Blocks {
		for _, i := range bb.Instrs {
			if site, ok := i.(*ssa.Call); ok {
				proof := a.ProveConstantData(site)
				if proof == nil || len(proof.ModeledOperations) != 1 || proof.ModeledOperations[0] != ByteSearchModel {
					t.Fatal("missing explicit model")
				}
				e := &dataEval{program: p, sizes: p.Packages[0].TypesSizes, steps: 100000, cells: 65536, owned: map[*dataCell]bool{}, stack: map[*ssa.Function]bool{}}
				args := []dataValue{e.literal(site.Common().Args[0].(*ssa.Const)), e.literal(site.Common().Args[1].(*ssa.Const))}
				result := e.function(p.CallTarget(site), args)[0]
				got, ok := constant.Int64Val(result.scalar)
				want := strings.IndexByte(constant.StringVal(args[0].scalar), byte(e.integer(args[1])))
				if !ok || got != int64(want) {
					t.Fatalf("got %d want %d", got, want)
				}
				count++
			}
		}
	}
	if count != 20 {
		t.Fatalf("cases=%d", count)
	}
}

func TestByteSearchModelContracts(t *testing.T) {
	for _, broken := range []string{"graph", "declaration", "source", "signature", "budget"} {
		t.Run(broken, func(t *testing.T) {
			p := testutil.Load(t, `package main;import "strings";func main(){_=strings.IndexByte("aba",97)}`)
			main, _ := p.Main()
			a := New(p, nil)
			var site *ssa.Call
			var primitive *ssa.Function
			for _, bb := range main.Blocks {
				for _, i := range bb.Instrs {
					if c, ok := i.(*ssa.Call); ok {
						site = c
					}
				}
			}
			for f := range p.Calls.Nodes {
				if f != nil && f.String() == "internal/bytealg.IndexByteString" {
					primitive = f
				}
			}
			if site == nil || primitive == nil || !a.ProveConstantDataCall(site) {
				t.Fatal("baseline model refused")
			}
			switch broken {
			case "graph":
				p.Calls.Nodes[p.CallTarget(site)].Out = nil
			case "declaration":
				primitive.Syntax().(*ast.FuncDecl).Body = &ast.BlockStmt{}
			case "source":
				delete(p.Sources, filepath.Clean(p.Fset.Position(primitive.Pos()).Filename))
			case "signature":
				primitive.Signature = types.NewSignatureType(nil, nil, nil, types.NewTuple(types.NewVar(0, nil, "s", types.Typ[types.String]), types.NewVar(0, nil, "c", types.Typ[types.Int])), types.NewTuple(types.NewVar(0, nil, "", types.Typ[types.Int])), false)
			case "budget":
				site.Common().Args[0] = ssa.NewConst(constant.MakeString(strings.Repeat("x", 100001)), types.Typ[types.String])
			}
			if a.ProveConstantDataCall(site) {
				t.Fatal("invalid primitive contract accepted")
			}
		})
	}
}

func TestByteSearchDoesNotExemptUnknownCalls(t *testing.T) {
	for _, decl := range []string{`func IndexByteString(s string,c byte)int {panic("effect")}`, `func IndexByteString(s string,c byte)int`} {
		p := testutil.Load(t, `package main;`+decl+`;func main(){_=IndexByteString("aba",97)}`)
		main, _ := p.Main()
		found := false
		for _, bb := range main.Blocks {
			for _, i := range bb.Instrs {
				if site, ok := i.(*ssa.Call); ok {
					found = true
					if New(p, nil).ProveConstantDataCall(site) {
						t.Fatal("name-based exemption")
					}
				}
			}
		}
		if !found {
			t.Fatal("call missing")
		}
	}
}

func TestConstantStringConcatenation(t *testing.T) {
	p := testutil.Load(t, `package main;func join(s string)string{v:=s+"suffix";if v!="prefixsuffix"{panic("bad")};return v};func main(){_=join("prefix")}`)
	main, _ := p.Main()
	a := New(p, nil)
	for _, bb := range main.Blocks {
		for _, i := range bb.Instrs {
			if site, ok := i.(*ssa.Call); ok {
				if !a.ProveConstantDataCall(site) {
					t.Fatal("string concatenation refused")
				}
				site.Common().Args[0] = ssa.NewConst(constant.MakeString(strings.Repeat("x", 256*1024)), types.Typ[types.String])
				if a.ProveConstantDataCall(site) {
					t.Fatal("over-budget concatenation accepted")
				}
				return
			}
		}
	}
	t.Fatal("call missing")
}
