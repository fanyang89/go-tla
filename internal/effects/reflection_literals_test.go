package effects

import (
	"go/constant"
	"go/types"
	"reflect"
	"testing"

	"github.com/fanmi/go-tla/internal/frontend"
	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
)

func literalTypeSites(t *testing.T) (*frontend.Program, []*ssa.Call) {
	t.Helper()
	p := testutil.Load(t, `package main;import "reflect";func main(){_=reflect.TypeFor[int]()}`)
	var sites []*ssa.Call
	for _, pkg := range p.SSA.AllPackages() {
		if pkg.Pkg.Path() != "reflect" {
			continue
		}
		for _, bb := range pkg.Func("init").Blocks {
			for _, i := range bb.Instrs {
				if c, ok := i.(*ssa.Call); ok && c.Common().StaticCallee() != nil && c.Common().StaticCallee().Name() == "rtypeOf" {
					sites = append(sites, c)
				}
			}
		}
	}
	if len(sites) != 3 {
		t.Fatalf("expected three real reflect literals, got %d", len(sites))
	}
	return p, sites
}

func TestReflectionLiteralInitializers(t *testing.T) {
	p, sites := literalTypeSites(t)
	a := New(p, nil)
	native := map[string]reflect.Type{"string": reflect.TypeFor[string](), "[]byte": reflect.TypeFor[[]byte](), "uint8": reflect.TypeFor[uint8]()}
	seen := map[string]bool{}
	for _, site := range sites {
		proof := a.ProveConstantData(site)
		if proof == nil || len(proof.ModeledOperations) != 1 || proof.ModeledOperations[0] != ReflectionLiteralTypeModel {
			t.Fatal("literal metadata assumption missing")
		}
		e := &dataEval{program: p, sizes: p.Packages[0].TypesSizes, steps: 100000, cells: 65536, owned: map[*dataCell]bool{}}
		typ, ok := e.literalRuntimeType(site)
		if !ok {
			t.Fatal("literal metadata refused")
		}
		n := native[typ.String()]
		if n == nil || int64(n.Size()) != e.sizes.Sizeof(typ) || seen[typ.String()] {
			t.Fatalf("native metadata mismatch: %v", typ)
		}
		seen[typ.String()] = true
	}
}

func TestReflectionLiteralDoesNotAdmitGeneralBoxing(t *testing.T) {
	p := testutil.Load(t, `package main;func box(n int)any{return n};func main(){_=box(1)}`)
	main, _ := p.Main()
	a := New(p, nil)
	for _, bb := range main.Blocks {
		for _, i := range bb.Instrs {
			if site, ok := i.(*ssa.Call); ok {
				if a.ProveConstantData(site) != nil {
					t.Fatal("general interface boxing admitted")
				}
				return
			}
		}
	}
	t.Fatal("box call missing")
}

func TestReflectionLiteralFreshness(t *testing.T) {
	for _, broken := range []string{"argument", "nested-interface", "graph", "wrapper", "abi", "source", "budget", "order"} {
		t.Run(broken, func(t *testing.T) {
			p, sites := literalTypeSites(t)
			a := New(p, nil)
			site := sites[0]
			if a.ProveConstantData(site) == nil {
				t.Fatal("baseline refused")
			}
			f := p.CallTarget(site)
			switch broken {
			case "argument":
				site.Common().Args[0].(*ssa.MakeInterface).X = f.Params[0]
			case "nested-interface":
				site.Common().Args[0].(*ssa.MakeInterface).X = ssa.NewConst(nil, f.Params[0].Type())
			case "graph":
				p.Calls.Nodes[f].Out = nil
			case "wrapper":
				f.Blocks[0].Instrs = f.Blocks[0].Instrs[:1]
			case "source":
				delete(p.Sources, p.Fset.Position(f.Pos()).Filename)
			case "abi":
				for fn := range p.Calls.Nodes {
					if fn != nil && fn.String() == "internal/abi.NoEscape" {
						for _, i := range fn.Blocks[0].Instrs {
							if op, ok := i.(*ssa.BinOp); ok {
								op.Y = ssa.NewConst(constant.MakeInt64(1), types.Typ[types.Uintptr])
							}
						}
					}
				}
			case "order":
				box := site.Common().Args[0].(*ssa.MakeInterface)
				is := site.Block().Instrs
				bi, ci := -1, -1
				for n, i := range is {
					if i == box {
						bi = n
					}
					if i == site {
						ci = n
					}
				}
				if bi < 0 || ci < 0 {
					t.Fatal("same-block boxing missing")
				}
				is[bi], is[ci] = is[ci], is[bi]
			case "budget":
				site.Common().Args[0].(*ssa.MakeInterface).X = ssa.NewConst(constant.MakeString(string(make([]byte, 256*1024+1))), types.Typ[types.String])
			}
			if a.ProveConstantData(site) != nil {
				t.Fatal("stale/invalid literal metadata proof accepted")
			}
		})
	}
}
