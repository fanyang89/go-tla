package lowering

import (
	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/testutil"
	"go/constant"
	"go/types"
	"golang.org/x/tools/go/ssa"
	"testing"
)

func TestOpenGlobalConsumesFreshProof(t *testing.T) {
	for _, broken := range []string{"none", "valid-capacity", "unbounded-capacity", "rebind"} {
		t.Run(broken, func(t *testing.T) {
			p := testutil.Load(t, `package main;var c=make(chan int,1);var data int;func unused(){data=1};func main(){c<-1;<-c}`)
			b := &builder{p: p, m: &behavior.Model{Outcome: "precisely-modeled"}, names: map[string]int{}}
			var allocation *ssa.MakeChan
			for _, bb := range p.Roots[0].Func("init").Blocks {
				for _, i := range bb.Instrs {
					if x, ok := i.(*ssa.MakeChan); ok {
						allocation = x
					}
				}
			}
			if allocation == nil || !b.consumeOpenGlobal(allocation) {
				t.Fatal("creation not proved")
			}
			proof := b.proveGlobalCreation(allocation)
			switch broken {
			case "valid-capacity":
				allocation.Size = ssa.NewConst(constant.MakeInt64(2), types.Typ[types.Int])
			case "unbounded-capacity":
				allocation.Size = ssa.NewConst(constant.MakeInt64(2048), types.Typ[types.Int])
			case "rebind":
				f := p.Roots[0].Func("unused")
				for _, bb := range f.Blocks {
					for _, i := range bb.Instrs {
						if s, ok := i.(*ssa.Store); ok {
							s.Addr = proof.global
							s.Val = ssa.NewConst(nil, allocation.Type())
						}
					}
				}
				delete(p.Calls.Nodes, f) // The inventory must not rely solely on graph nodes.
			}
			id := b.identity(&frame{}, proof.global, map[ssa.Value]bool{})
			if broken == "none" {
				if id == "invalid" || b.m.HasErrors() {
					t.Fatal("valid proof refused")
				}
			} else if id != "invalid" || !b.m.HasErrors() {
				t.Fatal("stale proof accepted")
			}
		})
	}
}
