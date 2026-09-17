package tests

import (
	"testing"

	"github.com/fanmi/go-tla/internal/tla"
)

func TestTLCConstantDataCalls(t *testing.T) {
	for _, tc := range []struct{ name, decl, body, want string }{
		{"initializer-validation", `func build(n int)int{if n!=2{panic("bad")};return n};var data=build(2)`, `c:=make(chan int,1);c<-data;<-c`, "No error has been found"},
		{"private-helper", `type T struct{N int};func observe(p *T)int{return p.N};func build()*T{p:=&T{1};observe(p);return p};var data=build()`, `c:=make(chan int);close(c)`, "No error has been found"},
		{"private-slice", `func build()*[2]int{p:=new([2]int);s:=p[:];s[1]=2;return p};var data=build()`, `c:=make(chan int);close(c)`, "No error has been found"},
		{"private-copy", `func build(s string)*[4]byte{p:=new([4]byte);copy(p[:],s);if p[0]!='a'{panic("bad")};return p};var data=build("abcd")`, `c:=make(chan int);close(c)`, "No error has been found"},
		{"loop-validation", `func build(s string)*[4]byte{p:=new([4]byte);copy(p[:],s);for i:=0;i<len(s);i++{if p[i]!='a'{panic("bad")}};return p};var data=build("aaaa")`, `c:=make(chan int);close(c)`, "No error has been found"},
		{"runtime-literal", `func build(n int)int{if n!=2{panic("bad")};return n}`, `_=build(2);c:=make(chan int);close(c)`, "No error has been found"},
		{"abstract-result", `func build(n int)int{if n!=2{panic("bad")};return n}`, `c:=make(chan int);close(c);if build(2)==2{close(c)}`, "Invariant NoSynchronizationErrors is violated"},
		{"zero-iteration-effects", `func scan(s []int){for _,v:=range s{println(v)}}`, `scan(nil);c:=make(chan int);close(c)`, "No error has been found"},
		{"unchosen-effects", `func build(n int)int{if n==1{return 2};c:=make(chan int);close(c);return n}`, `_=build(1);c:=make(chan int);close(c)`, "No error has been found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := fromSource(t, "package main;"+tc.decl+";func main(){"+tc.body+"}")
			if m.HasErrors() {
				t.Fatalf("proved invocation refused: %+v", m.Diagnostics)
			}
			found := false
			for _, d := range m.Diagnostics {
				if d.Code == "constant-data-call" {
					found = true
				}
			}
			if !found {
				t.Fatal("missing per-call proof record")
			}
			checkTLC(t, m, tc.want)
		})
	}
}
func TestConstantDataRefusals(t *testing.T) {
	for name, source := range map[string]string{
		"bad-input":     `func build(n int)int{if n!=2{panic("bad")};return n};var data=build(1)`,
		"unknown-input": `var input=2;func build(n int)int{if n!=2{panic("bad")};return n};var data=build(input)`,
		"shared":        `var buffer [4]byte;func build(s string)int{return copy(buffer[:],s)};var data=build("abc")`,
		"global-read":   `var flag=true;func build(n int)int{if flag{panic("bad")};return n};var data=build(1)`,
		"bad-copy":      `func build(s string)*[4]byte{p:=new([4]byte);copy(p[:],s);if p[0]!='a'{panic("bad")};return p};var data=build("bcd")`,
	} {
		t.Run(name, func(t *testing.T) {
			m := fromSource(t, "package main;"+source+";func main(){}")
			if !m.HasErrors() {
				t.Fatal("unproved invocation accepted")
			}
			if _, _, err := tla.Generate(m); err == nil {
				t.Fatal("unproved invocation executable")
			}
		})
	}
}
