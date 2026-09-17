package effects

import (
	"go/constant"
	"go/token"
	"go/types"
	"testing"

	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
)

func TestRotatedRangeProofFreshness(t *testing.T) {
	for _, mutation := range []string{"entry", "step", "comparison", "bound"} {
		t.Run(mutation, func(t *testing.T) {
			p := testutil.Load(t, `package main;func scan(s string)int{n:=0;for i:=range len(s){n+=int(s[i])};return n};func main(){_=scan("abc")}`)
			f := p.Roots[0].Func("scan")
			a := New(p, nil)
			if !a.ProveFiniteData(f) {
				t.Fatal("rotated range refused")
			}
			changed := false
			for _, bb := range f.Blocks {
				for _, i := range bb.Instrs {
					if phi, ok := i.(*ssa.Phi); ok && phi.Comment == "rangeint.iter" && mutation == "entry" {
						for j, v := range phi.Edges {
							if _, ok := v.(*ssa.Const); ok {
								phi.Edges[j] = ssa.NewConst(constant.MakeInt64(1), types.Typ[types.Int])
								changed = true
							}
						}
					}
					if op, ok := i.(*ssa.BinOp); ok {
						if mutation == "step" && op.Op == token.ADD {
							if phi, ok := op.X.(*ssa.Phi); ok && phi.Comment == "rangeint.iter" {
								op.Y = ssa.NewConst(constant.MakeInt64(2), types.Typ[types.Int])
								changed = true
							}
						}
						if step, ok := op.X.(*ssa.BinOp); ok && step.Op == token.ADD && op.Op == token.LSS {
							switch mutation {
							case "comparison":
								op.Op = token.LEQ
								changed = true
							case "bound":
								op.Y = ssa.NewConst(constant.MakeInt64(10), types.Typ[types.Int])
								changed = true
							}
						}
					}
				}
			}
			if !changed {
				t.Fatal("mutation missing")
			}
			if a.ProveFiniteData(f) {
				t.Fatal("stale rotated-loop proof accepted")
			}
		})
	}
}

func TestRotatedRangeRefusesHiddenEffectsAndCycles(t *testing.T) {
	for _, body := range []string{`for i:=range len(s){escaped=i}`, `for i:=range len(s){for s[i]=='x'{}}`, `for i:=range n{_=i}`} {
		p := testutil.Load(t, `package main;var escaped int;func scan(s string,n int){`+body+`};func main(){scan("abc",3)}`)
		if New(p, nil).ProveFiniteData(p.Roots[0].Func("scan")) {
			t.Fatal("unbounded or effectful range accepted")
		}
	}
}
