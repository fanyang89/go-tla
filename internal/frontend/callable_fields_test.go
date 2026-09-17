package frontend_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/fanmi/go-tla/internal/effects"
	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/ssa"
)

func TestCallableFieldProofRechecksGraphAndOrigins(t *testing.T) {
	p := testutil.Load(t, `package main
type writer struct{f func()};func good(){};func other(){}
func(w *writer)Run(){w.f()}
func main(){w:=&writer{f:good};w.Run();other()}`)
	var run *ssa.Function
	for f := range p.Calls.Nodes {
		if f != nil && f.Name() == "Run" && f.Pkg != nil && f.Pkg.Pkg.Path() == "fixture" {
			run = f
		}
	}
	if run == nil {
		t.Fatal("missing receiver method")
	}
	var site ssa.CallInstruction
	for _, bb := range run.Blocks {
		for _, i := range bb.Instrs {
			if c, ok := i.(*ssa.Call); ok {
				site = c
			}
		}
	}
	if site == nil {
		t.Fatal("missing field call")
	}
	proof := p.CallableField(site)
	if proof == nil || proof.Target.Name() != "good" || p.CallTarget(site) != proof.Target {
		t.Fatal("missing exact field proof")
	}
	value, root := p.CallableInitializer(proof.Initializer)
	if value != proof.Value || root == nil {
		t.Fatal("initializer not tied to allocation")
	}
	node := p.Calls.Nodes[run]
	old := node.Out
	node.Out = nil
	if p.CallableField(site) != nil || p.CallTarget(site) != nil {
		t.Fatal("field graph edge was decorative")
	}
	node.Out = old
	main, _ := p.Main()
	var wrong ssa.CallInstruction
	for _, bb := range main.Blocks {
		for _, i := range bb.Instrs {
			if c, ok := i.(*ssa.Call); ok && c.Common().StaticCallee() != nil && c.Common().StaticCallee().Name() == "other" {
				wrong = c
			}
		}
	}
	if wrong == nil {
		t.Fatal("missing unrelated call")
	}
	callgraph.AddEdge(p.Calls.Nodes[main], wrong, node)
	if p.CallableField(site) != nil || p.CallTarget(site) != nil {
		t.Fatal("extra unproved incoming alias ignored")
	}
	if effects.New(p, []string{proof.Target.String()}).Call(site).IsPure() {
		t.Fatal("trust bypassed invalid field-origin proof")
	}
}

func TestCallableFieldProofBudgetRefusesLongAliasChain(t *testing.T) {
	var source strings.Builder
	source.WriteString("package main\ntype writer struct{f func()};func good(){};func last(w *writer){w.f()}\n")
	for i := range 1025 {
		next := "last"
		if i < 1024 {
			next = fmt.Sprintf("pass%d", i+1)
		}
		fmt.Fprintf(&source, "func pass%d(w *writer){%s(w)}\n", i, next)
	}
	source.WriteString("func main(){w:=&writer{f:good};pass0(w)}")
	p := testutil.Load(t, source.String())
	last := p.Roots[0].Func("last")
	for _, bb := range last.Blocks {
		for _, i := range bb.Instrs {
			if site, ok := i.(*ssa.Call); ok {
				if p.CallableField(site) != nil || p.CallTarget(site) != nil {
					t.Fatal("proof budget became an assumed alias")
				}
				return
			}
		}
	}
	t.Fatal("missing field call")
}
