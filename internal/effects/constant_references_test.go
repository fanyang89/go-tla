package effects

import (
	"go/constant"
	"go/types"
	"math/big"
	"testing"

	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
)

func TestActualConstantFloatConstructor(t *testing.T) {
	p := testutil.Load(t, `package main;import "go/constant";func main(){_=constant.MakeInt64(1)}`)
	a := New(p, nil)
	for f := range p.Calls.Nodes {
		if f == nil || f.String() != "go/constant.init" {
			continue
		}
		for _, b := range f.Blocks {
			for _, i := range b.Instrs {
				if c, ok := i.(*ssa.Call); ok && c.Common().StaticCallee() != nil && c.Common().StaticCallee().String() == "go/constant.newFloat" {
					if !a.ProveConstantDataCall(c) {
						t.Fatal("actual zero Float constructor refused")
					}
					e := &dataEval{program: p, sizes: p.Packages[0].TypesSizes, steps: 100000, cells: 65536, owned: map[*dataCell]bool{}, stack: map[*ssa.Function]bool{}}
					value := e.function(p.CallTarget(c), nil)[0].pointer.value
					structure := value.typ.Underlying().(*types.Struct)
					found := false
					for n := range structure.NumFields() {
						if structure.Field(n).Name() == "prec" {
							prec, ok := constant.Uint64Val(value.elements[n].value.scalar)
							if !ok || prec != uint64(new(big.Float).SetPrec(512).Prec()) {
								t.Fatal("native precision mismatch")
							}
							found = true
						}
					}
					if !found {
						t.Fatal("precision field missing")
					}
					return
				}
			}
		}
	}
	t.Fatal("actual constructor invocation missing")
}

func TestConstantReferenceStorage(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       bool
	}{
		{"nil-map", `type T struct{M map[string]int};func build(n int)*T{return new(T)}`, true},
		{"private-cycle", `type T struct{N int;Next *T};func build(n int)*T{p:=new(T);p.Next=p;p.Next.N=n;if p.N!=n{panic("bad")};return p}`, true},
		{"nested-callback", `type U struct{F func()};type T struct{P *U};func build(n int)*T{return new(T)}`, false},
		{"nil-slice", `type T struct{S []int};func build(n int)*T{p:=new(T);if len(p.S)!=0{panic("bad")};return p}`, true},
		{"owned-pointer", `type T struct{P *int};func build(n int)*T{p:=new(T);p.P=new(n);if *p.P!=n{panic("bad")};return p}`, true},
		{"alias-reset", `type T struct{P *int};func build(n int)*T{p:=&T{new(n)};a:=&p.P;*p=T{new(3)};if **a!=3{panic("bad")};return p}`, true},
		{"nil-deref", `type T struct{P *int};func build(n int)*T{p:=new(T);*p.P=n;return p}`, false},
		{"shared-pointer", `var shared int;type T struct{P *int};func build(n int)*T{p:=&T{&shared};*p.P=n;return p}`, false},
		{"callback", `type T struct{F func()};func build(n int)*T{return new(T)}`, false},
		{"boxed-interface", `type T struct{V any};func build(n int)*T{return &T{V:n}}`, false},
		{"channel", `type T struct{C chan int};func build(n int)*T{return new(T)}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testutil.Load(t, "package main;"+tc.body+";func main(){_=build(2)}")
			main, _ := p.Main()
			a := New(p, nil)
			for _, b := range main.Blocks {
				for _, i := range b.Instrs {
					if c, ok := i.(*ssa.Call); ok {
						if got := a.ProveConstantDataCall(c); got != tc.want {
							t.Fatalf("proof=%v want=%v", got, tc.want)
						}
						return
					}
				}
			}
			t.Fatal("call missing")
		})
	}
}
