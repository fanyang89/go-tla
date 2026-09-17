package lowering

import (
	"slices"
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/effects"
	"github.com/fanmi/go-tla/internal/testutil"
	"github.com/fanmi/go-tla/internal/tla"
	"golang.org/x/tools/go/ssa"
)

func TestScalarFormatConsumption(t *testing.T) {
	for _, broken := range []string{"none", "operand", "store-root", "escape", "graph"} {
		t.Run(broken, func(t *testing.T) {
			p := testutil.Load(t, `package main;import "fmt";var saved []any;func main(){a:=[]any{1};_=fmt.Sprintf("%d",a...);saved=nil}`)
			main, _ := p.Main()
			b := &builder{p: p, m: &behavior.Model{}, effects: effects.New(p, nil), plans: map[*ssa.Function]*functionPlan{}}
			plan := b.plan(main)
			var site *ssa.Call
			for c, s := range plan.Calls {
				if s.Kind == effects.ScalarFormat {
					site = c.(*ssa.Call)
				}
			}
			if site == nil {
				t.Fatal("format model not classified")
			}
			proof := b.effects.ProveScalarFormat(site)
			fr := &frame{f: main, plan: plan}
			if !b.consumeScalarFormat(fr, site) || !slices.Contains(b.m.Assumptions, effects.ScalarFormatModel) {
				t.Fatal("format proof/assumption missing")
			}
			switch broken {
			case "operand":
				delete(plan.Slice.Data, proof.Stores[0].Val.(*ssa.MakeInterface).X)
			case "store-root":
				delete(plan.Slice.Roots, proof.Stores[0])
			case "escape":
				for _, bb := range main.Blocks {
					for _, i := range bb.Instrs {
						if s, ok := i.(*ssa.Store); ok && s != proof.Stores[0] {
							s.Val = site.Common().Args[1]
						}
					}
				}
			case "graph":
				p.Calls.Nodes[main].Out = nil
			}
			if b.consumeScalarFormat(fr, site) != (broken == "none") {
				t.Fatal("format proof freshness mismatch")
			}
			if broken == "none" && len(b.m.Assumptions) != 1 {
				t.Fatal("model assumption duplicated")
			}
		})
	}
}

func TestScalarFormatPreservesBlockingAndInitializers(t *testing.T) {
	p := testutil.Load(t, `package main;import("fmt";"sync");func main(){var m sync.Mutex;c:=make(chan int);m.Lock();_=fmt.Sprintf("%d",<-c);m.Unlock()}`)
	m, err := Lower(p, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !m.HasErrors() {
		t.Fatal("unrelated fmt/dependency initialization ignored")
	}
	if _, _, err := tla.Generate(m); err == nil {
		t.Fatal("unsupported package emitted")
	}
	if !slices.Contains(m.Assumptions, effects.ScalarFormatModel) {
		t.Fatal("formatting assumption missing")
	}
	kinds := map[behavior.EffectKind]bool{}
	for _, transition := range m.Transitions {
		for _, effect := range transition.Effects {
			kinds[effect.Kind] = true
		}
	}
	if !kinds[behavior.Lock] || !kinds[behavior.Unlock] || !kinds[behavior.Receive] {
		t.Fatalf("argument or critical-section effects lost: %v", kinds)
	}
}
