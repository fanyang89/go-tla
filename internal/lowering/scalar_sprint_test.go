package lowering

import (
	"strings"
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/effects"
	"github.com/fanmi/go-tla/internal/testutil"
	"github.com/fanmi/go-tla/internal/tla"
	"golang.org/x/tools/go/ssa"
)

func TestScalarSprintConsumption(t *testing.T) {
	for _, name := range []string{"Sprint", "Sprintln"} {
		t.Run(name, func(t *testing.T) {
			p := testutil.Load(t, `package main;import "fmt";func main(){_=fmt.`+name+`(7)}`)
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
				t.Fatal("variant missing")
			}
			fr := &frame{f: main, plan: plan}
			proof := b.effects.ProveScalarFormat(site)
			if !b.consumeScalarFormat(fr, site) {
				t.Fatal("valid variant refused")
			}
			found := false
			for _, d := range b.m.Diagnostics {
				if d.Code == "scalar-format-call" && strings.Contains(d.Message, "fmt."+name+";") {
					found = true
				}
			}
			if !found {
				t.Fatal("variant attribution missing")
			}
			delete(plan.Slice.Roots, proof.Stores[0])
			if b.consumeScalarFormat(fr, site) {
				t.Fatal("stale store-root proof accepted")
			}
		})
	}
}

func TestScalarSprintRetainsArguments(t *testing.T) {
	for _, name := range []string{"Sprint", "Sprintln"} {
		t.Run(name, func(t *testing.T) {
			p := testutil.Load(t, `package main;import "fmt";func main(){c:=make(chan int);_=fmt.`+name+`(<-c)}`)
			m, err := Lower(p, Options{})
			if err != nil {
				t.Fatal(err)
			}
			receive, format := false, false
			for _, transition := range m.Transitions {
				for _, effect := range transition.Effects {
					receive = receive || effect.Kind == behavior.Receive
				}
			}
			for _, d := range m.Diagnostics {
				format = format || d.Code == "scalar-format-call"
			}
			if !receive || !format || !m.HasErrors() {
				t.Fatal("argument blocking or independent initialization refusal lost")
			}
			if _, _, err := tla.Generate(m); err == nil {
				t.Fatal("unsupported fmt import emitted")
			}
		})
	}
}
