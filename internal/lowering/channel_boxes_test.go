package lowering

import (
	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
	"testing"
)

func TestChannelBoxCurrentUses(t *testing.T) {
	for _, broken := range []string{"none", "graph", "argument", "return", "definition", "budget"} {
		t.Run(broken, func(t *testing.T) {
			p := testutil.Load(t, `package main;type I interface{Run()};type W struct{ch chan int};func(w *W)Run(){};func main(){w:=&W{make(chan int)};var x I=w;x.Run()}`)
			main, _ := p.Main()
			b := &builder{p: p}
			var box *ssa.MakeInterface
			var site *ssa.Call
			var ret *ssa.Return
			for _, bb := range main.Blocks {
				for _, i := range bb.Instrs {
					switch x := i.(type) {
					case *ssa.MakeInterface:
						box = x
					case *ssa.Call:
						if x.Common().IsInvoke() {
							site = x
						}
					case *ssa.Return:
						ret = x
					}
				}
			}
			if box == nil || site == nil || ret == nil || !b.channelReceiverBox(box) {
				t.Fatal("valid box proof missing")
			}
			// Deliberately stale caches: proofs must read the current instruction operands.
			*box.Referrers() = nil
			switch broken {
			case "graph":
				p.Calls.Nodes[main].Out = nil
			case "argument":
				site.Common().Args = append(site.Common().Args, box)
			case "return":
				ret.Results = []ssa.Value{box}
			case "definition":
				for _, bb := range main.Blocks {
					for j, i := range bb.Instrs {
						if i == box {
							bb.Instrs = append(bb.Instrs[:j], bb.Instrs[j+1:]...)
							break
						}
					}
				}
			case "budget":
				for range 4097 {
					ret.Block().Instrs = append(ret.Block().Instrs, ret)
				}
			}
			if got := b.channelReceiverBox(box); got != (broken == "none") {
				t.Fatalf("proof=%v for %s", got, broken)
			}
		})
	}
}

func TestChannelObjectInventoryIgnoresReferrerCache(t *testing.T) {
	p := testutil.Load(t, `package main;type W struct{ch chan int};func main(){w:=&W{make(chan int)};_=w.ch}`)
	main, _ := p.Main()
	var root *ssa.Alloc
	for _, bb := range main.Blocks {
		for _, i := range bb.Instrs {
			if x, ok := i.(*ssa.Alloc); ok && aggregatePointer(x.Type()) {
				root = x
			}
		}
	}
	if root == nil {
		t.Fatal("allocation missing")
	}
	before, ok := channelObjectUses(root)
	if !ok || len(before) == 0 {
		t.Fatal("uses missing")
	}
	*root.Referrers() = nil
	after, ok := channelObjectUses(root)
	if !ok || len(after) != len(before) {
		t.Fatal("cached uses became authority")
	}
}
