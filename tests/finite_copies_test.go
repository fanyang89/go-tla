package tests

import (
	"github.com/fanmi/go-tla/internal/tla"
	"testing"
)

func TestTLCFiniteDataCopies(t *testing.T) {
	for _, tc := range []struct{ name, decl, body, want string }{
		{"slice-copy", `type E struct{S []int};func scan(es []E)int{n:=0;for _,e:=range es{n+=len(e.S)};return n}`, `_=scan([]E{{S:[]int{1}}});c:=make(chan int);close(c)`, "No error has been found"},
		{"local-header", `type E struct{S []int};func scan(es []E)int{n:=0;for _,e:=range es{e.S=nil;n+=len(e.S)};return n}`, `_=scan([]E{{S:[]int{1}}});c:=make(chan int);close(c)`, "No error has been found"},
		{"pointer-copy", `type E struct{P *int};func scan(es []E)int{n:=0;for _,e:=range es{n+=*e.P};return n}`, `_=scan([]E{{P:new(1)}});c:=make(chan int);close(c)`, "No error has been found"},
		{"abstract-result", `type E struct{S []int};func scan(es []E)int{n:=0;for _,e:=range es{n+=len(e.S)};return n}`, `c:=make(chan int);close(c);if scan([]E{{S:[]int{1}}})>0{close(c)}`, "Invariant NoSynchronizationErrors is violated"},
		{"argument-effect", `type E struct{S []int};func scan(es []E)int{n:=0;for _,e:=range es{n+=len(e.S)};return n};func input()[]E{c:=make(chan int);<-c;return nil}`, `_=scan(input())`, "Deadlock reached"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := fromSource(t, "package main;"+tc.decl+";func main(){"+tc.body+"}")
			if m.HasErrors() {
				t.Fatalf("private value copy refused: %+v", m.Diagnostics)
			}
			found := false
			for _, d := range m.Diagnostics {
				if d.Code == "finite-data-loop" {
					found = true
				}
			}
			if !found {
				t.Fatal("missing finite computation proof")
			}
			checkTLC(t, m, tc.want)
		})
	}
}

func TestFiniteDataCopyRefusals(t *testing.T) {
	for name, body := range map[string]string{
		"backing-write": `for _,e:=range es{e.S[0]=1}`,
		"pointee-write": `for _,e:=range es{*e.P=1}`,
		"header-write":  `for i:=range es{es[i].S=nil}`,
	} {
		t.Run(name, func(t *testing.T) {
			m := fromSource(t, "package main;type E struct{S []int;P *int};func scan(es []E){"+body+"};func main(){scan([]E{{S:[]int{1},P:new(1)}})}")
			if !m.HasErrors() {
				t.Fatal("shared write summarized as private")
			}
			if _, _, err := tla.Generate(m); err == nil {
				t.Fatal("unproved shared write executable")
			}
		})
	}
}
