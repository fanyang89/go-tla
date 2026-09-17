package effects

import (
	"bytes"
	"testing"

	"github.com/fanmi/go-tla/internal/frontend"
	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
)

func TestFiniteDataProof(t *testing.T) {
	for _, tc := range []struct {
		name, declaration string
		want              bool
	}{
		{"range", `func scan(s []int)int{n:=0;for _,v:=range s{n+=v};return n}`, true},
		{"for", `func scan(s []byte)bool{for i:=0;i<len(s);i++{if s[i]==0{return false}};return true}`, true},
		{"string-index", `func scan(s string)bool{for i:=0;i<len(s);i++{if s[i]==0{return false}};return true}`, true},
		{"continue", `func scan(s []int)int{n:=0;for i:=0;i<len(s);i++{if s[i]==0{continue};n++};return n}`, true},
		{"helper", `func positive(n int)bool{return n>0};func scan(s []int)bool{for _,n:=range s{if positive(n){return true}};return false}`, true},
		{"reset", `func scan(s []int)int{for i:=0;i<len(s);i++{i=0};return 0}`, false},
		{"decrement", `func scan(s []int)int{for i:=0;i<len(s);i--{};return 0}`, false},
		{"step-two", `func scan(s []int)int{for i:=0;i<len(s);i+=2{};return 0}`, false},
		{"narrow-overflow", `func scan(s []int)int{for i:=int8(0);int(i)<len(s);i++{};return 0}`, false},
		{"inclusive", `func scan(s []int)int{for i:=0;i<=len(s);i++{};return 0}`, false},
		{"changing-header", `func scan(s []int)int{for i:=0;i<len(s);i++{s=s[:len(s)-1]};return 0}`, false},
		{"nested-cycle", `func scan(s []int)int{for i:=0;i<len(s);i++{for{}};return 0}`, false},
		{"shared-write", `func scan(s []int)int{for i:=0;i<len(s);i++{s[i]=0};return 0}`, false},
		{"copy", `func scan(s []int)int{for range s{copy(s,s)};return 0}`, false},
		{"unknown", `func hidden();func scan(s []int)int{for range s{hidden()};return 0}`, false},
		{"recursive", `func scan(s []int)int{for range s{return scan(s)};return 0}`, false},
		{"channel", `func scan(s []int,c chan int)int{for range s{<-c};return 0}`, false},
		{"panic", `func scan(s []int)int{for range s{panic("bad")};return 0}`, false},
		{"allocation", `func scan(s []int)[]int{var out []int;for _,n:=range s{out=append(out,n)};return out}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testutil.Load(t, "package main;"+tc.declaration+";func main(){}")
			f := p.Roots[0].Func("scan")
			if f == nil {
				t.Fatal("missing source function")
			}
			a := New(p, []string{"fixture.hidden"}) // Cannot supply missing body proof.
			if got := a.ProveFiniteData(f); got != tc.want {
				var dump bytes.Buffer
				f.WriteTo(&dump)
				t.Fatalf("proof=%v want=%v\n%s", got, tc.want, &dump)
			}
		})
	}
}

func TestFiniteDataRechecksTransitiveGraph(t *testing.T) {
	p := testutil.Load(t, `package main;func value(n int)int{return n+1};func scan(s []int)int{n:=0;for _,v:=range s{n+=value(v)};return n};func main(){scan(nil)}`)
	f := p.Roots[0].Func("scan")
	a := New(p, nil)
	if !a.ProveFiniteData(f) {
		t.Fatal("valid proof rejected")
	}
	p.Calls.Nodes[f].Out = nil
	if a.ProveFiniteData(f) {
		t.Fatal("missing transitive graph edge accepted")
	}
}

func TestFiniteDataProofBudget(t *testing.T) {
	p := testutil.Load(t, `package main;func scan(s []int)int{n:=0;for _,v:=range s{n+=v};return n};func main(){}`)
	f := p.Roots[0].Func("scan")
	proof := finiteDataProof{program: p, remaining: 2, visiting: map[*ssa.Function]bool{}, proved: map[*ssa.Function]bool{}}
	if proof.function(f) || proof.remaining >= 0 {
		t.Fatal("exhausted proof budget did not refuse")
	}
}

func TestActualModelHasErrorsFiniteData(t *testing.T) {
	loaded, err := frontend.Load("../..", "./internal/behavior")
	if err != nil {
		t.Fatal(err)
	}
	p := frontend.BuildSSA(loaded)
	frontend.BuildCallGraph(p)
	for f := range p.Calls.Nodes {
		if f != nil && f.Name() == "HasErrors" && f.Pkg != nil && f.Pkg.Pkg.Path() == "github.com/fanmi/go-tla/internal/behavior" {
			if !New(p, nil).ProveFiniteData(f) {
				var dump bytes.Buffer
				f.WriteTo(&dump)
				t.Fatalf("actual production helper not proved:\n%s", &dump)
			}
			return
		}
	}
	t.Fatal("production HasErrors method not found")
}
