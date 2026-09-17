package lowering

import (
	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/effects"
	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
	"testing"
)

func TestOnceConsumesFreshProof(t *testing.T) {
	for _, broken := range []string{"graph", "receiver", "capture"} {
		t.Run(broken, func(t *testing.T) {
			p := testutil.Load(t, `package main;import "sync";func main(){var o sync.Once;c:=make(chan int);o.Do(func(){close(c)})}`)
			main, _ := p.Main()
			b := &builder{p: p, m: &behavior.Model{}, effects: effects.New(p, nil), plans: map[*ssa.Function]*functionPlan{}, names: map[string]int{}, resources: map[string]bool{}, channelFields: map[string]string{}}
			plan := b.plan(main)
			var site *ssa.Call
			for c, s := range plan.Calls {
				if s.Kind == effects.OnceDo {
					site = c.(*ssa.Call)
				}
			}
			if site == nil {
				t.Fatal("proof missing")
			}
			fr := &frame{f: main, plan: plan, ids: map[ssa.Value]string{}}
			b.allocateResources(fr)
			proof := b.effects.ProveOnceCall(site)
			if proof == nil || len(proof.Captures) == 0 {
				t.Fatal("capture proof missing")
			}
			switch broken {
			case "graph":
				p.Calls.Nodes[main].Out = nil
			case "receiver":
				delete(plan.Slice.Data, proof.Receiver)
			case "capture":
				delete(plan.Slice.Data, proof.Captures[0])
			}
			if got := b.onceCall(fr, site, "next"); got != "next" || !b.m.HasErrors() {
				t.Fatal("stale proof accepted")
			}
		})
	}
}

func TestOnceReadAndPublicationBoundaries(t *testing.T) {
	p := testutil.Load(t, `package main;import "sync";func main(){var o sync.Once;o.Do(func(){})}`)
	m, err := Lower(p, Options{})
	if err != nil || m.HasErrors() {
		t.Fatalf("lower: %v %+v", err, m)
	}
	if len(m.SharedState) != 1 {
		t.Fatal("completion state missing")
	}
	name := m.SharedState[0].Name
	committed, published := false, false
	for _, tr := range m.Transitions {
		if tr.Guard.Kind == behavior.Equal && tr.Guard.Variable == name && tr.Guard.Value == 0 && len(tr.Effects) == 0 {
			for _, next := range m.Transitions {
				if next.Source == tr.Destination && len(next.Effects) == 1 && next.Effects[0].Kind == behavior.Lock && next.Guard.Kind == behavior.True {
					committed = true
				}
			}
		}
		if len(tr.Effects) == 1 && tr.Effects[0].Kind == behavior.AssignAbstractState && tr.Effects[0].Variable == name {
			for _, next := range m.Transitions {
				if next.Source == tr.Destination && len(next.Effects) == 1 && next.Effects[0].Kind == behavior.Unlock {
					published = true
				}
			}
		}
	}
	if !committed || !published {
		t.Fatal("flag read/lock or done store/unlock incorrectly fused")
	}
}
