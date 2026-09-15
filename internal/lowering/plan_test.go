package lowering

import (
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/effects"
	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
)

func TestFunctionPlanDependenciesAreConsumed(t *testing.T) {
	p := testutil.Load(t, `package main;func worker(ch chan int){ch<-1};func main(){ch:=make(chan int);go worker(ch);<-ch}`)
	main, _ := p.Main()
	b := &builder{p: p, m: &behavior.Model{Outcome: "precisely-modeled"}, effects: effects.New(p, nil), plans: map[*ssa.Function]*functionPlan{}}
	plan := b.plan(main)
	if b.plan(main) != plan || len(plan.Entries) != 1 {
		t.Fatal("process discovery or plan caching missing")
	}
	for goCall, entry := range plan.Entries {
		if entry.Callee == nil || entry.Callee.Name() != "worker" || !plan.Slice.Roots[goCall] {
			t.Fatal("goroutine fact not tied to root and call graph")
		}
	}
	for instruction, primitive := range plan.Discovery.Primitives {
		if primitive.Kind != behavior.Receive {
			continue
		}
		if !plan.Slice.Roots[instruction] || !plan.Slice.Data[primitive.Resource] {
			t.Fatal("root resource missing from data slice")
		}
		fr := &frame{f: main, plan: plan}
		if !b.requireData(fr, primitive.Resource) {
			t.Fatal("valid retained data rejected")
		}
		delete(plan.Slice.Data, primitive.Resource)
		if b.requireData(fr, primitive.Resource) || !b.m.HasErrors() {
			t.Fatal("lowering ignored a broken data-slice contract")
		}
		return
	}
	t.Fatal("receive missing")
}

func TestLowerRequiresCallGraph(t *testing.T) {
	p := testutil.Load(t, `package main;func main(){}`)
	p.Calls = nil
	if _, err := Lower(p, Options{}); err == nil {
		t.Fatal("lowering proceeded without its call graph pass")
	}
}
