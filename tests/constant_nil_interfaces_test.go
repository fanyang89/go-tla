package tests

import "testing"

func TestTLCConstantNilInterfaces(t *testing.T) {
	for _, tc := range []struct{ name, decl, call, want string }{
		{"field", `type T struct{V any};func build(n int)*T{p:=new(T);p.V=nil;return p}`, `_=build(2);c:=make(chan int);go func(){c<-1}();<-c`, "No error has been found"},
		{"nil-argument", `type T struct{V any};func build(v any)*T{p:=&T{V:v};if p.V!=nil{panic("bad")};return p}`, `_=build(nil)`, "No error has been found"},
		{"array-reset", `type T struct{V [2]any;N int};func build(n int)*T{p:=new(T);p.N=n;v:=&p.V[0];num:=&p.N;*p=T{};if *v!=nil||*num!=0{panic("bad")};return p}`, `_=build(2)`, "No error has been found"},
		{"initializer", `type T struct{V any};func build(n int)*T{p:=new(T);p.V=nil;return p};var value=build(2)`, `c:=make(chan int);close(c);<-c`, "No error has been found"},
		{"abstract-result", `type T struct{V any};func build(n int)*T{p:=new(T);p.V=nil;return p}`, `c:=make(chan int);if build(2).V==nil{close(c)}else{close(c);close(c)}`, "Invariant NoSynchronizationErrors is violated"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := fromSource(t, "package main;"+tc.decl+";func main(){"+tc.call+"}")
			if m.HasErrors() {
				t.Fatalf("nil interface refused: %+v", m.Diagnostics)
			}
			proved := false
			for _, d := range m.Diagnostics {
				if d.Code == "constant-data-call" {
					proved = true
				}
			}
			if !proved {
				t.Fatal("missing consumed literal-input proof")
			}
			checkTLC(t, m, tc.want)
		})
	}
}

func TestConstantInterfaceBoxingRefused(t *testing.T) {
	for _, value := range []string{"2", "(*int)(nil)", "make(chan int)", "func(){}"} {
		t.Run(value, func(t *testing.T) {
			m := fromSource(t, `package main;type T struct{V any};func build()*T{return &T{V:`+value+`}};var value=build();func main(){}`)
			if !m.HasErrors() {
				t.Fatal("non-nil interface initialization accepted")
			}
		})
	}
}
