package tests

import (
	"testing"

	"github.com/fanmi/go-tla/internal/tla"
)

const dataScan = `func scan(s []int)int{n:=0;for _,v:=range s{n+=v};return n};`

func TestTLCFiniteDataComputations(t *testing.T) {
	for _, tc := range []struct{ name, source, want string }{
		{"locked-computation", `package main;import "sync";` + dataScan + `func main(){var m sync.Mutex;m.Lock();_=scan([]int{1,2});m.Unlock()}`, "No error has been found"},
		{"predicate-deadlock", `package main;` + dataScan + `func main(){c:=make(chan int);if scan(nil)>0{<-c}}`, "Error: Deadlock reached"},
		{"predicate-sync-error", `package main;` + dataScan + `func main(){c:=make(chan int);close(c);if scan(nil)>0{close(c)}}`, "Error: Invariant NoSynchronizationErrors is violated"},
		{"initializer", `package main;` + dataScan + `var n=scan([]int{1,2});func main(){c:=make(chan int);close(c)}`, "No error has been found"},
		{"argument-blocks", `package main;` + dataScan + `func input(c chan int)[]int{<-c;return nil};func main(){c:=make(chan int);_=scan(input(c))}`, "Error: Deadlock reached"},
		{"deferred-computation", `package main;import "sync";` + dataScan + `func f(w *sync.WaitGroup){defer w.Done();defer scan([]int{1,2})};func main(){var w sync.WaitGroup;w.Add(1);go f(&w);w.Wait()}`, "No error has been found"},
		{"wrapper", `package main;` + dataScan + `func wrapper(s []int)int{return scan(s)};func main(){c:=make(chan int,1);c<-wrapper([]int{1,2});<-c}`, "No error has been found"},
		{"value-struct", `package main;type D struct{Value int};func scan(s []D)int{n:=0;for _,d:=range s{n+=d.Value};return n};func main(){c:=make(chan int,1);c<-scan([]D{{1},{2}});<-c}`, "No error has been found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := fromSource(t, tc.source)
			found := false
			for _, d := range m.Diagnostics {
				if d.Code == "finite-data-loop" {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing consumed finite computation proof: %+v", m.Diagnostics)
			}
			checkTLC(t, m, tc.want)
		})
	}
}

func TestFiniteDataRefusals(t *testing.T) {
	for name, source := range map[string]string{
		"writes":           `package main;func scan(s []int){for i:=range s{s[i]=1}};func main(){scan([]int{0})}`,
		"print":            `package main;func scan(s []int){for _,v:=range s{println(v)}};func main(){scan([]int{1})}`,
		"callback":         `package main;func scan(s []int,f func()){for range s{f()}};func main(){scan(nil,func(){})}`,
		"synchronization":  `package main;import "sync";func scan(s []int,m *sync.Mutex){for range s{m.Lock();m.Unlock()}};func main(){var m sync.Mutex;scan(nil,&m)}`,
		"counter-reset":    `package main;func scan(s []int){for i:=0;i<len(s);i++{i=0}};func main(){scan([]int{1,2})}`,
		"narrow-counter":   `package main;func scan(s []int){for i:=int8(0);int(i)<len(s);i++{}};func main(){scan(make([]int,128))}`,
		"dynamic-capacity": `package main;` + dataScan + `func main(){c:=make(chan int,scan(nil));c<-1}`,
	} {
		t.Run(name, func(t *testing.T) {
			m := fromSource(t, source)
			if !m.HasErrors() {
				t.Fatal("unproved computation admitted")
			}
			if _, _, err := tla.Generate(m); err == nil {
				t.Fatal("unsupported computation executable")
			}
		})
	}
}
