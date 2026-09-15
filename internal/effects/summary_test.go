package effects_test

import (
	"testing"

	"github.com/fanmi/go-tla/internal/effects"
	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
)

func TestCallSummariesConsumeGraphAndContracts(t *testing.T) {
	p := testutil.Load(t, `package main
func pure(x int)int{return x+1}
func external()bool
func send(ch chan int){ch<-1}
func recursive(){recursive()}
func main(){ch:=make(chan int);pure(1);external();send(ch);go send(ch);recursive()}`)
	main, err := p.Main()
	if err != nil {
		t.Fatal(err)
	}
	analyzer := effects.New(p, nil)
	var external *ssa.Call
	want := map[string]effects.Kind{"pure": effects.Pure, "external": effects.Unknown, "send": effects.Inspect, "recursive": effects.Inspect}
	for _, bb := range main.Blocks {
		for _, i := range bb.Instrs {
			if call, ok := i.(*ssa.Call); ok {
				name := call.Common().StaticCallee().Name()
				summary := analyzer.Call(call)
				if summary.Kind != want[name] || summary.Callee == nil {
					t.Fatalf("%s summary: %+v", name, summary)
				}
				if analyzer.Call(call) != summary {
					t.Fatal("cached summary changed")
				}
				if name == "external" {
					external = call
				}
			}
		}
	}
	if external == nil {
		t.Fatal("external site missing")
	}
	if s := effects.New(p, []string{"fixture.external"}).Call(external); s.Kind != effects.Trusted || !s.IsPure() {
		t.Fatalf("contract not consumed: %+v", s)
	}
	p.Calls = nil
	if s := effects.New(p, []string{"fixture.external"}).Call(external); s.Kind != effects.Unknown || s.Callee != nil {
		t.Fatal("call graph was decorative")
	}
}

func TestUnsafeAndDynamicBodiesCannotBecomePure(t *testing.T) {
	p := testutil.Load(t, `package main
import "unsafe"
func corrupt(p unsafe.Pointer){*(*int)(p)=0}
func apply(f func()){f()}
func main(){var x int;corrupt(unsafe.Pointer(&x));apply(func(){})}`)
	main, _ := p.Main()
	a := effects.New(p, nil)
	for _, bb := range main.Blocks {
		for _, i := range bb.Instrs {
			if call, ok := i.(*ssa.Call); ok && a.Call(call).IsPure() {
				t.Fatal("unsafe/dynamic body summarized as local computation")
			}
		}
	}
}
