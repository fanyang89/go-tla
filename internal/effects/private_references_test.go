package effects

import (
	"slices"
	"testing"

	"github.com/fanmi/go-tla/internal/frontend"
	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
)

func TestPrivateReferenceConstructors(t *testing.T) {
	const pre = `package main;type I interface{Run()};type B struct{p *int;s []int;m map[string]int;f func();i I};`
	for _, tc := range []struct {
		name, body string
		pure       bool
	}{
		{"copy-headers", `func construct(p *int,s []int,m map[string]int,f func(),i I)*B{return &B{p,s,m,f,i}}`, true},
		{"borrowed-pointee", `func construct(p *int)*B{b:=&B{p:p};*b.p=7;return b}`, false},
		{"borrowed-slice", `func construct(s []int)*B{b:=&B{s:s};b.s[0]=7;return b}`, false},
		{"borrowed-map", `func construct(m map[string]int)*B{b:=&B{m:m};b.m["x"]=7;return b}`, false},
		{"callback", `func construct(f func())*B{b:=&B{f:f};b.f();return b}`, false},
		{"invoke", `func construct(i I)*B{b:=&B{i:i};b.i.Run();return b}`, false},
		{"published", `var global *B;func construct(p *int)*B{b:=&B{p:p};global=b;return b}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testutil.Load(t, pre+tc.body+`;func main(){}`)
			a := New(p, nil)
			f := p.Roots[0].Func("construct")
			if got := a.pureFunction(f); got != tc.pure {
				t.Fatalf("pure=%v want=%v", got, tc.pure)
			}
		})
	}
}

func TestActualTypePointerConstructor(t *testing.T) {
	loaded, err := frontend.LoadContext(t.Context(), "../frontend", ".")
	if err != nil {
		t.Fatal(err)
	}
	p := frontend.BuildSSA(loaded)
	frontend.BuildCallGraph(p)
	a := New(p, nil)
	want := map[string]bool{"go/types.NewPointer": false, "go/types.NewTuple": false, "golang.org/x/tools/go/ssa.newVar": false, "golang.org/x/tools/go/ssa.anonVar": false}
	for f := range p.Calls.Nodes {
		if f == nil || f.Pkg == nil {
			continue
		}
		key := f.Pkg.Pkg.Path() + "." + f.Name()
		if _, ok := want[key]; ok {
			want[key] = true
			if !a.pureFunction(f) {
				t.Fatalf("actual constructor not proved: %s", key)
			}
		}
	}
	for key, found := range want {
		if !found {
			t.Errorf("dependency constructor missing: %s", key)
		}
	}
}

func TestPrivateConstructorUsesAreCurrent(t *testing.T) {
	for _, mutation := range []string{"none", "escape", "budget", "definition"} {
		t.Run(mutation, func(t *testing.T) {
			p := testutil.Load(t, `package main;type B struct{p *int};var sink *B;func construct(p *int)*B{b:=&B{p:p};sink=nil;return b};func main(){}`)
			f := p.Roots[0].Func("construct")
			var root *ssa.Alloc
			var store, global *ssa.Store
			for _, bb := range f.Blocks {
				for _, i := range bb.Instrs {
					switch x := i.(type) {
					case *ssa.Alloc:
						root = x
					case *ssa.Store:
						if _, ok := x.Addr.(*ssa.Global); ok {
							global = x
						} else {
							store = x
						}
					}
				}
			}
			if root == nil || store == nil || global == nil || !privateDataStore(store, map[*ssa.Alloc]bool{}) {
				t.Fatal("fixture missing")
			}
			*root.Referrers() = nil
			if mutation == "escape" {
				global.Val = root
			}
			if mutation == "definition" {
				for _, bb := range f.Blocks {
					bb.Instrs = slices.DeleteFunc(bb.Instrs, func(i ssa.Instruction) bool { return i == root })
				}
			}
			if mutation == "budget" {
				last := f.Blocks[0].Instrs[len(f.Blocks[0].Instrs)-1]
				for range 4097 {
					f.Blocks[0].Instrs = append(f.Blocks[0].Instrs, last)
				}
			}
			if got := privateDataStore(store, map[*ssa.Alloc]bool{}); got != (mutation == "none") {
				t.Fatal("stale ownership inventory accepted or valid refused")
			}
		})
	}
}
