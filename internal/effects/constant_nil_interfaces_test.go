package effects

import (
	"go/ast"
	"go/constant"
	"go/types"
	"testing"

	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
)

func TestConstantNilInterfaces(t *testing.T) {
	for _, tc := range []struct {
		name, body, call string
		want             bool
	}{
		{"field", `type T struct{V any};func build(n int)*T{return new(T)}`, "build(2)", true},
		{"comparison", `type T struct{V any};func build(n int)*T{p:=new(T);if p.V!=nil{panic("bad")};if !(p.V==nil){panic("bad")};return p}`, "build(2)", true},
		{"parameter", `type T struct{V any};func build(v any)*T{return &T{V:v}}`, "build(nil)", true},
		{"returned", `type T struct{V any};func zero()any{return nil};func build(n int)*T{return &T{V:zero()}}`, "build(2)", true},
		{"named", `type E interface{};type T struct{V E};func build(n int)*T{return new(T)}`, "build(2)", true},
		{"array-reset", `type T struct{V [2]any;N int};func build(n int)*T{p:=new(T);p.N=n;v:=&p.V[0];num:=&p.N;*p=T{};if *v!=nil||*num!=0{panic("bad")};return p}`, "build(2)", true},
		{"nonliteral-nil", `type T struct{V any};func zero()any{return nil};func build(v any)*T{return &T{V:v}}`, "build(zero())", false},
		{"scalar-box", `type T struct{V any};func build(n int)*T{return &T{V:n}}`, "build(2)", false},
		{"typed-nil-box", `type T struct{V any};func build(n int)*T{var p *int;return &T{V:p}}`, "build(2)", false},
		{"channel-box", `type T struct{V any};func build(n int)*T{return &T{V:make(chan int)}}`, "build(2)", false},
		{"callback-box", `type T struct{V any};func build(n int)*T{return &T{V:func(){}}}`, "build(2)", false},
		{"method-interface", `type T struct{V error};func build(n int)*T{return new(T)}`, "build(2)", false},
		{"external-read", `var shared any;type T struct{V any};func build(n int)*T{return &T{V:shared}}`, "build(2)", false},
		{"publication", `var shared any;type T struct{V any};func build(n int)*T{p:=new(T);shared=p.V;return p}`, "build(2)", false},
		{"assertion", `type T struct{V any};func build(n int)*T{p:=new(T);_=p.V.(int);return p}`, "build(2)", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testutil.Load(t, "package main;"+tc.body+";func main(){_="+tc.call+"}")
			main, _ := p.Main()
			a := New(p, nil)
			for _, bb := range main.Blocks {
				for _, i := range bb.Instrs {
					if call, ok := i.(*ssa.Call); ok && call.Common().StaticCallee().Name() == "build" {
						if got := a.ProveConstantDataCall(call); got != tc.want {
							t.Fatalf("proof=%v want=%v", got, tc.want)
						}
						if a.ProveFiniteData(p.CallTarget(call)) {
							t.Fatal("general finite proof admitted interface graph")
						}
						return
					}
				}
			}
			t.Fatal("call missing")
		})
	}
}

func TestActualConstantIdentConstructor(t *testing.T) {
	p := testutil.Load(t, `package main;import "go/ast";func main(){_=ast.NewIdent("_")}`)
	main, _ := p.Main()
	a := New(p, nil)
	for _, bb := range main.Blocks {
		for _, i := range bb.Instrs {
			if site, ok := i.(*ssa.Call); ok {
				if !a.ProveConstantDataCall(site) {
					t.Fatal("actual ast.NewIdent refused")
				}
				e := &dataEval{program: p, sizes: p.Packages[0].TypesSizes, steps: 100000, cells: 65536, owned: map[*dataCell]bool{}, stack: map[*ssa.Function]bool{}}
				v := e.function(p.CallTarget(site), []dataValue{e.literal(site.Common().Args[0].(*ssa.Const))})[0].pointer.value
				native := ast.NewIdent("_")
				s := v.typ.Underlying().(*types.Struct)
				seen := 0
				for n := range s.NumFields() {
					f := v.elements[n].value
					switch s.Field(n).Name() {
					case "Name":
						if constant.StringVal(f.scalar) != native.Name {
							t.Fatal("name mismatch")
						}
						seen++
					case "NamePos":
						pos, ok := constant.Int64Val(f.scalar)
						if !ok || pos != int64(native.NamePos) {
							t.Fatal("position mismatch")
						}
						seen++
					case "Obj":
						if f.pointer != nil || native.Obj != nil {
							t.Fatal("object mismatch")
						}
						seen++
					}
				}
				if seen != 3 {
					t.Fatal("unexpected actual constructor shape")
				}
				return
			}
		}
	}
	t.Fatal("actual call missing")
}
