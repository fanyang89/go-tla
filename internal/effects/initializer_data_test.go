package effects

import (
	"testing"

	"github.com/fanmi/go-tla/internal/frontend"
	"github.com/fanmi/go-tla/internal/testutil"
)

func TestFiniteInitializerData(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		want         bool
	}{
		{"map", `var m map[int]int;func fill(){m=make(map[int]int);for i:=0;i<40;i++{m[i]=i}}`, true},
		{"offset", `var m map[int]int;func fill(){m=make(map[int]int);for i:=7;i<40;i++{m[i]=i}}`, true},
		{"named", `type T int;var m map[T]T;func fill(){m=make(map[T]T);for i:=T(7);i<T(40);i++{m[i]=i}}`, true},
		{"lookup", `var m map[int]int;func fill(){m=make(map[int]int);for i:=7;i<40;i++{m[i]=m[i-1]+1}}`, true},
		{"negative", `var n int;func fill(){for i:=-2;i<40;i++{n=i}}`, false},
		{"nonprogress", `var n int;func fill(){for i:=7;i<40;i+=0{n=i}}`, false},
		{"overflow", `var n int;func fill(){for i:=7;i<=2147483647;i++{n=i}}`, false},
		{"interface", `var m map[int]any;func fill(){m=make(map[int]any);for i:=0;i<40;i++{m[i]=i}}`, false},
		{"channel", `var m map[int]chan int;func fill(){m=make(map[int]chan int);for i:=0;i<40;i++{m[i]=nil}}`, false},
		{"output", `var n int;func fill(){for i:=0;i<40;i++{println(i);n=i}}`, false},
		{"callback", `var f func();func fill(){for i:=0;i<40;i++{f()}}`, false},
		{"panic", `func fill(){for i:=0;i<40;i++{panic("bad")}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testutil.Load(t, `package main;`+tc.source+`;func main(){}`)
			a := New(p, nil)
			f := p.Roots[0].Func("fill")
			if got := a.ProveInitializerData(f); got != tc.want {
				t.Fatalf("startup=%v want=%v", got, tc.want)
			}
			if tc.want && (a.ProvePure(f) || a.ProveFiniteData(f)) {
				t.Fatal("startup writes acquired runtime purity")
			}
		})
	}
}

func TestActualTableInitializationData(t *testing.T) {
	loaded, err := frontend.LoadContext(t.Context(), "../frontend", ".")
	if err != nil {
		t.Fatal(err)
	}
	p := frontend.BuildSSA(loaded)
	frontend.BuildCallGraph(p)
	a := New(p, nil)
	found := map[string]bool{"go/token": false, "regexp": false}
	for f := range p.Calls.Nodes {
		if f == nil || f.Pkg == nil || f.Name() != "init#1" {
			continue
		}
		path := f.Pkg.Pkg.Path()
		if _, ok := found[path]; ok {
			found[path] = true
			if !a.ProveInitializerData(f) {
				t.Errorf("actual table loop refused: %s", path)
			}
		}
	}
	for path, ok := range found {
		if !ok {
			t.Errorf("actual table initializer missing: %s", path)
		}
	}
}

func TestInitializerDataRechecksGraph(t *testing.T) {
	p := testutil.Load(t, `package main;var n int;func put(i int){n=i};func fill(){for i:=2;i<40;i++{put(i)}};func main(){}`)
	a := New(p, nil)
	f := p.Roots[0].Func("fill")
	if !a.ProveInitializerData(f) {
		t.Fatal("fixture refused")
	}
	p.Calls.Nodes[f].Out = nil
	if a.ProveInitializerData(f) {
		t.Fatal("cached startup graph proof accepted")
	}
}
