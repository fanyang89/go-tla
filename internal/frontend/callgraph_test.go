package frontend_test

import (
	"testing"

	"github.com/fanmi/go-tla/internal/effects"
	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/ssa"
)

func TestDirectInterfaceGraphAndReceiverAgreement(t *testing.T) {
	p := testutil.Load(t, `package main
type worker interface{Run(chan int)}
type sender struct{}
func helper(ch chan int){ch<-1}
func(s sender)Run(ch chan int){helper(ch)}
func main(){var w worker=sender{};ch:=make(chan int);go w.Run(ch);<-ch}`)
	main, err := p.Main()
	if err != nil {
		t.Fatal(err)
	}
	var invoke ssa.CallInstruction
	for _, bb := range main.Blocks {
		for _, i := range bb.Instrs {
			if site, ok := i.(ssa.CallInstruction); ok && site.Common().IsInvoke() {
				invoke = site
			}
		}
	}
	if invoke == nil {
		t.Fatal("missing actual SSA invoke")
	}
	target := p.CallTarget(invoke)
	receiver := p.InvokeReceiver(invoke)
	if target == nil || target.Name() != "Run" || receiver == nil {
		t.Fatal("missing exact target/receiver proof")
	}
	if got := effects.New(p, nil).Call(invoke); got.Callee != target || got.Kind != effects.Primitive {
		t.Fatalf("spawn summary did not consume refined graph: %+v", got)
	}
	foundHelper := false
	for _, bb := range target.Blocks {
		for _, i := range bb.Instrs {
			if site, ok := i.(ssa.CallInstruction); ok && site.Common().StaticCallee() != nil && site.Common().StaticCallee().Name() == "helper" {
				foundHelper = true
				if p.CallTarget(site) == nil {
					t.Fatal("implementation's direct helper omitted from graph")
				}
			}
		}
	}
	if !foundHelper {
		t.Fatal("missing nested helper call")
	}
	// A guessed graph edge cannot replace the exact SSA boxing proof.
	node := p.Calls.Nodes[main]
	node.Out = nil
	callgraph.AddEdge(node, invoke, p.Calls.CreateNode(main))
	if p.CallTarget(invoke) != nil || p.InvokeReceiver(invoke) != nil {
		t.Fatal("wrong graph target admitted")
	}
	if got := effects.New(p, nil).Call(invoke); got.Callee != nil {
		t.Fatal("effect summary bypassed graph agreement")
	}
	p.Calls = nil
	if p.CallTarget(invoke) != nil || p.InvokeReceiver(invoke) != nil {
		t.Fatal("missing graph admitted")
	}
}

func TestUnknownInterfaceParameterHasNoGraphTarget(t *testing.T) {
	p := testutil.Load(t, `package main
type I interface{Run()};type impl struct{};func(impl)Run(){}
func apply(i I){i.Run()}
func main(){apply(impl{})}`)
	f := p.Roots[0].Func("apply")
	found := false
	for _, bb := range f.Blocks {
		for _, i := range bb.Instrs {
			if site, ok := i.(ssa.CallInstruction); ok && site.Common().IsInvoke() {
				found = true
				if p.CallTarget(site) != nil || p.InvokeReceiver(site) != nil {
					t.Fatal("one known implementer was mistaken for a proof")
				}
			}
		}
	}
	if !found {
		t.Fatal("missing parameter invoke")
	}
}
