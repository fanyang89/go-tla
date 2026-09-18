package tests

import (
	"testing"

	"github.com/fanmi/go-tla/internal/tla"
)

func TestTLCPrivateArrayData(t *testing.T) {
	for _, tc := range []struct{ name, decl, body, want string }{
		{"empty-classic", `const marker=0`, `for i:=0;i<2;i++ {}`, "No error has been found"},
		{"literal", `func build()*[2]int{return &[2]int{1,2}};var table=build()`, `c:=make(chan int,1);c<-table[0];<-c`, "No error has been found"},
		{"nested", `type T struct{Rows [2][2]int};func build()*T{p:=new(T);p.Rows[1][0]=3;return p};var table=build()`, `c:=make(chan int);close(c)`, "No error has been found"},
		{"indexed-range", `func build()*[128]int{p:=new([128]int);for i:=range p{p[i]=i};return p};var table=build()`, `c:=make(chan int,1);c<-table[127];<-c`, "No error has been found"},
		{"classic-counter", `func build()*[128]int{p:=new([128]int);for i:=0;i<128;i++{p[i]=i};return p};var table=build()`, `c:=make(chan int);close(c)`, "No error has been found"},
		{"abstract-value", `func build()*[2]int{return &[2]int{1,2}};var table=build()`, `c:=make(chan int);close(c);if table[0]==1{close(c)}`, "Invariant NoSynchronizationErrors is violated"},
		{"argument-effects", `func sum(n int)int{for i:=0;i<32;i++{n+=i};return n};func input(c chan int)int{return <-c}`, `c:=make(chan int);_=sum(input(c))`, "Deadlock reached"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := fromSource(t, "package main;"+tc.decl+";func main(){"+tc.body+"}")
			if m.HasErrors() {
				t.Fatalf("private array computation refused: %+v", m.Diagnostics)
			}
			checkTLC(t, m, tc.want)
		})
	}
}

func TestPrivateArrayDataRefusals(t *testing.T) {
	for name, decl := range map[string]string{
		"slice-alias":        `var shared []int;func build()*[2]int{p:=new([2]int);s:=p[:];shared=s;s[0]=1;return p}`,
		"reference-elements": `var shared int;func build()*[2]*int{return &[2]*int{&shared,new(2)}}`,
		"channel-elements":   `func build()*[2]chan int{p:=new([2]chan int);p[0]=make(chan int);return p}`,
		"counter-reset":      `func build()*[128]int{p:=new([128]int);for i:=0;i<128;i++{i=0;p[i]=i};return p}`,
	} {
		t.Run(name, func(t *testing.T) {
			m := fromSource(t, "package main;"+decl+";var table=build();func main(){}")
			if !m.HasErrors() {
				t.Fatal("unproved array initializer accepted")
			}
			if _, _, err := tla.Generate(m); err == nil {
				t.Fatal("unproved array initializer emitted TLA+")
			}
		})
	}
}
