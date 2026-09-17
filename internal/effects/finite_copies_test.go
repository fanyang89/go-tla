package effects

import (
	"github.com/fanmi/go-tla/internal/testutil"
	"testing"
)

func TestFiniteDataReferenceCopies(t *testing.T) {
	for _, tc := range []struct {
		name, decl string
		want       bool
	}{
		{"slice-value", `type E struct{S []int};func scan(es []E)int{n:=0;for _,e:=range es{n+=len(e.S)};return n}`, true},
		{"pointer-value", `type E struct{P *int};func scan(es []E)int{n:=0;for _,e:=range es{n+=*e.P};return n}`, true},
		{"local-header", `type E struct{S []int};func scan(es []E)int{n:=0;for _,e:=range es{e.S=nil;n+=len(e.S)};return n}`, true},
		{"shared-backing", `type E struct{S []int};func scan(es []E)int{for _,e:=range es{e.S[0]=1};return 0}`, false},
		{"shared-pointee", `type E struct{P *int};func scan(es []E)int{for _,e:=range es{*e.P=1};return 0}`, false},
		{"shared-array", `type E struct{P *[2]int};func scan(es []E)int{for _,e:=range es{e.P[0]=1};return 0}`, false},
		{"shared-header", `type E struct{S []int};func scan(es []E)int{for i:=range es{es[i].S=nil};return 0}`, false},
		{"helper-write", `type E struct{P *int};func put(p *int){*p=1};func scan(es []E)int{for _,e:=range es{put(e.P)};return 0}`, false},
		{"publish", `type E struct{S []int};var shared []int;func scan(es []E)int{for _,e:=range es{shared=e.S};return 0}`, false},
		{"channel", `type E struct{C chan int};func scan(es []E)int{n:=0;for _,e:=range es{n+=len(e.C)};return n}`, false},
		{"callback", `type E struct{F func()};func scan(es []E)int{n:=0;for range es{n++};return n}`, false},
		{"interface", `type E struct{V any};func scan(es []E)int{n:=0;for range es{n++};return n}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testutil.Load(t, "package main;"+tc.decl+";func main(){}")
			if got := New(p, nil).ProveFiniteData(p.Roots[0].Func("scan")); got != tc.want {
				t.Fatalf("proof=%v want=%v", got, tc.want)
			}
		})
	}
}
