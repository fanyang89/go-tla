package slice_test

import (
	cslice "github.com/fanmi/go-tla/internal/slice"
	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
	"testing"
)

func TestBackwardControlAndData(t *testing.T) {
	p := testutil.Load(t, `package main
 func worker(ch chan int,x int){ y:=x*13; if y>4{ch<-y} }
 func main(){}`)
	f := p.Roots[0].Func("worker")
	s := cslice.Compute(f, func(*ssa.Call) bool { return false })
	if len(s.Roots) != 1 || len(s.Control) != 1 || !s.Data[f.Params[1]] {
		t.Fatalf("missing backward dependencies: %+v", s)
	}
	if cslice.HasCycle(f) {
		t.Fatal("acyclic worker marked cyclic")
	}
}
func TestCycleDetection(t *testing.T) {
	p := testutil.Load(t, `package main;func main(){for {}}`)
	f, _ := p.Main()
	if !cslice.HasCycle(f) {
		t.Fatal("cycle disappeared")
	}
}
