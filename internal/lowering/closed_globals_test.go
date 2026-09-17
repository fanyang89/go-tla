package lowering

import (
	"go/constant"
	"go/types"
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
)

func TestClosedGlobalConsumesFreshProof(t *testing.T) {
	for _, broken := range []string{"none", "graph", "body", "capacity", "valid-capacity", "order"} {
		t.Run(broken, func(t *testing.T) {
			p := testutil.Load(t, `package main;var ready=make(chan int);func init(){close(ready)};func main(){<-ready}`)
			root := p.Roots[0].Func("init")
			var makeChan *ssa.MakeChan
			for _, bb := range root.Blocks {
				for _, i := range bb.Instrs {
					if x, ok := i.(*ssa.MakeChan); ok {
						makeChan = x
					}
				}
			}
			b := &builder{p: p, m: &behavior.Model{Outcome: "precisely-modeled"}, names: map[string]int{}}
			if makeChan == nil || !b.consumeClosedGlobal(makeChan) {
				t.Fatal("valid initializer not proved")
			}
			proof := b.proveClosedGlobal(makeChan)
			switch broken {
			case "graph":
				p.Calls.Nodes[root].Out = nil
			case "body":
				proof.close.Call.Args = nil
			case "valid-capacity":
				makeChan.Size = ssa.NewConst(constant.MakeInt64(1), types.Typ[types.Int])
			case "capacity":
				makeChan.Size = ssa.NewConst(constant.MakeInt64(2048), types.Typ[types.Int])
			case "order":
				bb := proof.store.Block()
				storeIndex, callIndex := -1, -1
				for i, instruction := range bb.Instrs {
					if instruction == proof.store {
						storeIndex = i
					}
					if instruction == proof.call {
						callIndex = i
					}
				}
				if storeIndex < 0 || callIndex < 0 {
					t.Fatal("fixture changed")
				}
				bb.Instrs[storeIndex], bb.Instrs[callIndex] = bb.Instrs[callIndex], bb.Instrs[storeIndex]
			}
			consumed := b.consumeClosedGlobal(proof.call)
			id := b.identity(&frame{}, proof.global, map[ssa.Value]bool{})
			if broken == "none" {
				if !consumed || id == "invalid" || b.m.HasErrors() {
					t.Fatalf("valid proof refused: %+v", b.m.Diagnostics)
				}
			} else if consumed || id != "invalid" || !b.m.HasErrors() {
				t.Fatal("stale initialization proof accepted")
			}
		})
	}
}

func TestClosedGlobalInventoryNotLimitedToCallGraph(t *testing.T) {
	p := testutil.Load(t, `package main;var ready=make(chan int);func init(){close(ready)};func unused(){ready=nil};func main(){<-ready}`)
	delete(p.Calls.Nodes, p.Roots[0].Func("unused"))
	b := &builder{p: p}
	for _, bb := range p.Roots[0].Func("init").Blocks {
		for _, i := range bb.Instrs {
			if x, ok := i.(*ssa.MakeChan); ok {
				if b.proveClosedGlobal(x) != nil {
					t.Fatal("graph node removal hid a global rebind")
				}
				return
			}
		}
	}
	t.Fatal("missing global initializer")
}

func TestCloseMustBeReached(t *testing.T) {
	start, target, other, exit := &ssa.BasicBlock{}, &ssa.BasicBlock{}, &ssa.BasicBlock{}, &ssa.BasicBlock{}
	start.Succs = []*ssa.BasicBlock{target, other}
	other.Succs = []*ssa.BasicBlock{target}
	if !mustReachBlock(start, target, map[*ssa.BasicBlock]int{}) {
		t.Fatal("unconditional merge rejected")
	}
	other.Succs = []*ssa.BasicBlock{exit}
	if mustReachBlock(start, target, map[*ssa.BasicBlock]int{}) {
		t.Fatal("bypass exit accepted")
	}
	other.Succs = []*ssa.BasicBlock{start}
	if mustReachBlock(start, target, map[*ssa.BasicBlock]int{}) {
		t.Fatal("bypass cycle accepted")
	}
}
