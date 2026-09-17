package effects

import (
	"github.com/fanmi/go-tla/internal/testutil"
	"go/constant"
	"golang.org/x/tools/go/ssa"
	"testing"
)

func TestOnceProofMutations(t *testing.T) {
	for _, mutation := range []string{"graph", "slow-graph", "branch", "store-false", "unlock", "nil-callback", "source", "closure-order"} {
		t.Run(mutation, func(t *testing.T) {
			p := testutil.Load(t, `package main;import "sync";func main(){var o sync.Once;c:=make(chan int);o.Do(func(){close(c)})}`)
			main, _ := p.Main()
			a := New(p, nil)
			var site *ssa.Call
			for _, bb := range main.Blocks {
				for _, i := range bb.Instrs {
					if c, ok := i.(*ssa.Call); ok && IsOnceDo(c) {
						site = c
					}
				}
			}
			if site == nil || a.ProveOnceCall(site) == nil {
				t.Fatal("baseline missing")
			}
			f := p.CallTarget(site)
			slow := p.CallTarget(f.Blocks[1].Instrs[0].(*ssa.Call))
			switch mutation {
			case "closure-order":
				closure := site.Common().Args[1].(*ssa.MakeClosure)
				ci, si := -1, -1
				for i, ins := range site.Block().Instrs {
					if ins == closure {
						ci = i
					}
					if ins == site {
						si = i
					}
				}
				if ci < 0 || si < 0 {
					t.Fatal("closure fixture missing")
				}
				site.Block().Instrs[ci], site.Block().Instrs[si] = site.Block().Instrs[si], site.Block().Instrs[ci]
			case "graph":
				p.Calls.Nodes[main].Out = nil
			case "slow-graph":
				p.Calls.Nodes[slow].Out = nil
			case "branch":
				f.Blocks[0].Succs[0], f.Blocks[0].Succs[1] = f.Blocks[0].Succs[1], f.Blocks[0].Succs[0]
			case "store-false":
				d := slow.Blocks[2].Instrs[1].(*ssa.Defer)
				d.Common().Args[1] = ssa.NewConst(constant.MakeBool(false), d.Common().Args[1].Type())
			case "unlock":
				slow.Blocks[0].Instrs[3] = slow.Blocks[0].Instrs[1]
			case "nil-callback":
				site.Common().Args[1] = ssa.NewConst(nil, site.Common().Args[1].Type())
			case "source":
				delete(p.Sources, p.Fset.Position(f.Pos()).Filename)
			}
			if a.ProveOnceCall(site) != nil {
				t.Fatal("mutated contract accepted")
			}
		})
	}
}
