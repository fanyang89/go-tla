package effects

import (
	"go/constant"
	"go/types"
	"testing"

	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
)

func TestScalarFormatArguments(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		want         bool
	}{
		{"basic", `func main(){_=fmt.Sprintf("%v %s %d",true,"a",2)}`, true},
		{"dynamic", `func text(format string,n int)string{return fmt.Sprintf(format,n)};func main(){_=text("%d",2)}`, true},
		{"nil", `func main(){_=fmt.Sprintf("%v",nil)}`, true},
		{"empty", `func main(){_=fmt.Sprintf("bad %v")}`, true},
		{"zero-slots", `func main(){a:=new([2]any);_=fmt.Sprintf("%v",a[:]...)}`, true},
		{"named", `type N int;func(n N)String()string{panic("callback")};func main(){_=fmt.Sprintf("%v",N(1))}`, false},
		{"slice", `func main(){_=fmt.Sprintf("%v",[]byte("bytes"))}`, false},
		{"shared", `var a=[]any{1};func main(){_=fmt.Sprintf("%v",a...)}`, false},
		{"escape", `var saved []any;func main(){a:=[]any{1};saved=a;_=fmt.Sprintf("%v",a...)}`, false},
		{"overwrite", `func main(){a:=[]any{1};a[0]=2;_=fmt.Sprintf("%v",a...)}`, false},
		{"conditional", `func text(b bool){a:=new([1]any);if b{a[0]=1};_=fmt.Sprintf("%v",a[:]...)};func main(){text(true)}`, false},
		{"repeat-use", `func main(){a:=[]any{1};_=fmt.Sprintf("%v",a...);_=fmt.Sprintf("%v",a...)}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testutil.Load(t, `package main;import "fmt";`+tc.source)
			a := New(p, nil)
			found := false
			for _, f := range p.Roots[0].Members {
				fn, ok := f.(*ssa.Function)
				if !ok {
					continue
				}
				for _, bb := range fn.Blocks {
					for _, i := range bb.Instrs {
						site, ok := i.(*ssa.Call)
						if !ok || site.Common().StaticCallee() == nil || site.Common().StaticCallee().String() != "fmt.Sprintf" {
							continue
						}
						found = true
						proof := a.ProveScalarFormat(site)
						if (proof != nil) != tc.want {
							t.Fatalf("proof=%v want=%v", proof, tc.want)
						}
						if (a.Call(site).Kind == ScalarFormat) != tc.want {
							t.Fatal("classification disagrees")
						}
					}
				}
			}
			if !found {
				t.Fatal("format call missing")
			}
		})
	}
}

func TestScalarFormatFreshness(t *testing.T) {
	for _, broken := range []string{"graph", "wrapper", "source", "index", "store-order", "hidden-use", "box", "slice", "budget"} {
		t.Run(broken, func(t *testing.T) {
			p := testutil.Load(t, `package main;import "fmt";var saved []any;func main(){a:=[]any{1};_=fmt.Sprintf("%d",a...);saved=nil}`)
			main, _ := p.Main()
			a := New(p, nil)
			var site *ssa.Call
			for _, bb := range main.Blocks {
				for _, i := range bb.Instrs {
					if c, ok := i.(*ssa.Call); ok {
						site = c
					}
				}
			}
			if site == nil {
				t.Fatal("call missing")
			}
			proof := a.ProveScalarFormat(site)
			if proof == nil || a.Call(site).Kind != ScalarFormat {
				t.Fatal("baseline refused")
			}
			f := p.CallTarget(site)
			switch broken {
			case "graph":
				p.Calls.Nodes[f].Out = nil
			case "wrapper":
				f.Blocks[0].Instrs = f.Blocks[0].Instrs[:6]
			case "source":
				delete(p.Sources, p.Fset.Position(f.Pos()).Filename)
			case "index":
				proof.Stores[0].Addr.(*ssa.IndexAddr).Index = ssa.NewConst(constant.MakeInt64(2), types.Typ[types.Int])
			case "store-order":
				is := main.Blocks[0].Instrs
				si, ci := 0, 0
				for n, i := range is {
					if i == proof.Stores[0] {
						si = n
					}
					if i == site {
						ci = n
					}
				}
				is[si], is[ci] = is[ci], is[si]
			case "hidden-use":
				for _, bb := range main.Blocks {
					for _, i := range bb.Instrs {
						if s, ok := i.(*ssa.Store); ok && s != proof.Stores[0] {
							s.Val = site.Common().Args[1]
						}
					}
				}
			case "box":
				proof.Stores[0].Val.(*ssa.MakeInterface).X = site.Common().Args[1]
			case "slice":
				site.Common().Args[1].(*ssa.Slice).Low = ssa.NewConst(constant.MakeInt64(0), types.Typ[types.Int])
			case "budget":
				for range 4097 {
					main.Blocks[0].Instrs = append(main.Blocks[0].Instrs, site)
				}
			}
			if a.ProveScalarFormat(site) != nil {
				t.Fatal("stale scalar format proof accepted")
			}
		})
	}
}

func TestActualGoTypesVersionFormatting(t *testing.T) {
	p := testutil.Load(t, `package main;import _ "go/types";func main(){}`)
	a := New(p, nil)
	count := 0
	for _, pkg := range p.SSA.AllPackages() {
		if pkg.Pkg.Path() != "go/types" {
			continue
		}
		for _, bb := range pkg.Func("init").Blocks {
			for _, i := range bb.Instrs {
				if c, ok := i.(*ssa.Call); ok && c.Common().StaticCallee() != nil && c.Common().StaticCallee().String() == "fmt.Sprintf" {
					if a.ProveScalarFormat(c) == nil {
						t.Fatal("actual version formatting refused")
					}
					count++
				}
			}
		}
	}
	if count != 1 {
		t.Fatalf("actual version initializer count=%d", count)
	}
}
