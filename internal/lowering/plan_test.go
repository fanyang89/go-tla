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

func TestInterfaceProofConsumesGraphAndSlice(t *testing.T) {
	p := testutil.Load(t, `package main;type I interface{Value()int};type number int;func(n number)Value()int{return int(n)};func main(){var i I=number(1);_ = i.Value()}`)
	main, _ := p.Main()
	b := &builder{p: p, m: &behavior.Model{Outcome: "precisely-modeled"}, effects: effects.New(p, nil), plans: map[*ssa.Function]*functionPlan{}}
	plan := b.plan(main)
	for site, summary := range plan.Calls {
		if !site.Common().IsInvoke() {
			continue
		}
		receiver := p.InvokeReceiver(site)
		if summary.Kind != effects.Pure || receiver == nil || !plan.Discovery.Roots[site] || !plan.Slice.Data[receiver] || !plan.Slice.Data[site.Common().Value] {
			t.Fatal("pure interface target lost its receiver proof slice")
		}
		fr := &frame{f: main, plan: plan}
		if f, _ := b.callee(fr, site, summary.Callee, site.Pos()); f == nil || b.m.HasErrors() {
			t.Fatal("valid invoke proof rejected")
		}
		delete(plan.Slice.Data, receiver)
		if f, _ := b.callee(fr, site, summary.Callee, site.Pos()); f != nil || !b.m.HasErrors() {
			t.Fatal("missing receiver slice was ignored")
		}
		plan.Slice.Data[receiver] = true
		p.Calls.Nodes[main].Out = nil
		if f, _ := b.callee(fr, site, summary.Callee, site.Pos()); f != nil {
			t.Fatal("cached summary bypassed broken call graph")
		}
		if d := b.m.Diagnostics[len(b.m.Diagnostics)-1]; d.Code != "call-contract" {
			t.Fatalf("missing graph contract diagnostic: %+v", d)
		}
		return
	}
	t.Fatal("missing invoke")
}

func TestCallableFieldBindingAndSliceAreConsumed(t *testing.T) {
	p := testutil.Load(t, `package main;import "sync";type writer struct{mu sync.Mutex;f func()};func good(){};func(w *writer)Run(){w.f()};func main(){w:=&writer{f:good};w.Run()}`)
	var run *ssa.Function
	for f := range p.Calls.Nodes {
		if f != nil && f.Name() == "Run" && f.Pkg != nil && f.Pkg.Pkg.Path() == "fixture" {
			run = f
		}
	}
	if run == nil {
		t.Fatal("missing method")
	}
	b := &builder{p: p, m: &behavior.Model{Outcome: "precisely-modeled"}, effects: effects.New(p, nil), plans: map[*ssa.Function]*functionPlan{}, callableFields: map[callableFieldKey]callableFieldBinding{}}
	plan := b.plan(run)
	for site, summary := range plan.Calls {
		proof := p.CallableField(site)
		if proof == nil {
			continue
		}
		fr := &frame{f: run, plan: plan, ids: map[ssa.Value]string{run.Params[0]: "object"}}
		key := callableFieldKey{"object", proof.Field}
		binding := callableFieldBinding{initializer: proof.Initializer}
		b.callableFields[key] = binding
		if f, _ := b.callee(fr, site, summary.Callee, site.Pos()); f == nil || b.m.HasErrors() {
			t.Fatal("valid field binding refused")
		}
		delete(b.callableFields, key)
		if f, _ := b.callee(fr, site, summary.Callee, site.Pos()); f != nil {
			t.Fatal("missing allocation binding ignored")
		}
		b.callableFields[key] = binding
		delete(plan.Slice.Data, site.Common().Value)
		if f, _ := b.callee(fr, site, summary.Callee, site.Pos()); f != nil {
			t.Fatal("missing field load dependency ignored")
		}
		plan.Slice.Data[site.Common().Value] = true
		p.Calls.Nodes[run].Out = nil
		if f, _ := b.callee(fr, site, summary.Callee, site.Pos()); f != nil {
			t.Fatal("cached field target bypassed missing graph edge")
		}
		return
	}
	t.Fatal("missing field proof")
}

func TestLowerRequiresCallGraph(t *testing.T) {
	p := testutil.Load(t, `package main;func main(){}`)
	p.Calls = nil
	if _, err := Lower(p, Options{}); err == nil {
		t.Fatal("lowering proceeded without its call graph pass")
	}
}
