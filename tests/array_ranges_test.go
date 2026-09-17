package tests

import (
	"github.com/fanmi/go-tla/internal/tla"
	"strings"
	"testing"
)

func TestTLCScalarArrayRanges(t *testing.T) {
	for _, tc := range []struct{ name, decl, body, want string }{
		{"topology", `var values=[2]int{1,2}`, `for _,v:=range values{c:=make(chan int,1);c<-v;<-c}`, "No error has been found"},
		{"workers", `const marker=0`, `c:=make(chan int);for _,v:=range [2]int{1,2}{go func(){c<-v}()};<-c;<-c`, "No error has been found"},
		{"zero", `const marker=0`, `for _,v:=range [0]int{}{c:=make(chan int);c<-v};c:=make(chan int);close(c)`, "No error has been found"},
		{"saturated", `const marker=0`, `c:=make(chan int,1);for _,v:=range [2]int{1,2}{c<-v}`, "Deadlock reached"},
		{"return", `func work(c chan int){for _,v:=range [2]int{1,2}{_=v;return};c<-1}`, `c:=make(chan int);work(c)`, "No error has been found"},
		{"abstract-index", `func work(c chan int){for i,v:=range [2]int{1,2}{if i==0{return};c<-v}}`, `c:=make(chan int);work(c)`, "Deadlock reached"},
		{"defer-scope", `func finish(c chan int){close(c)}`, `c:=make(chan int);for _,v:=range [2]int{1,2}{_=v;defer finish(c)}`, "Invariant NoSynchronizationErrors is violated"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := fromSource(t, "package main;"+tc.decl+";func main(){"+tc.body+"}")
			if m.HasErrors() {
				t.Fatalf("array range refused: %+v", m.Diagnostics)
			}
			found := false
			for _, d := range m.Diagnostics {
				if d.Code == "proved-loop" && strings.Contains(d.Message, "scalar array") {
					found = true
				}
			}
			if !found {
				t.Fatal("missing array expansion proof")
			}
			checkTLC(t, m, tc.want)
		})
	}
}

func TestScalarArrayRangeRefusals(t *testing.T) {
	for name, body := range map[string]string{
		"over-budget":        `for _,v:=range [17]int{}{_=v;c:=make(chan int);close(c)}`,
		"assignment":         `v:=0;for _,v=range [2]int{}{c:=make(chan int);c<-v}`,
		"pointer-array":      `p:=new([2]int);for _,v:=range p{c:=make(chan int);c<-v}`,
		"reference-elements": `for _,v:=range [2]*int{}{_=v;c:=make(chan int);close(c)}`,
		"nested":             `for _,v:=range [2]int{}{_=v;for range 2{c:=make(chan int);close(c)}}`,
		"break":              `for _,v:=range [2]int{}{_=v;break}`,
	} {
		t.Run(name, func(t *testing.T) {
			m := fromSource(t, "package main;func main(){"+body+"}")
			if !m.HasErrors() {
				t.Fatal("unproved array range accepted")
			}
			if _, _, err := tla.Generate(m); err == nil {
				t.Fatal("unproved range executable")
			}
		})
	}
}
