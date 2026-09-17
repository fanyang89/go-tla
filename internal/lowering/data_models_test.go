package lowering

import (
	"go/ast"
	"slices"
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/effects"
	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
)

func TestConstantModelEvidenceAndFreshness(t *testing.T) {
	p := testutil.Load(t, `package main;import "strings";func main(){_=strings.IndexByte("aba",97)}`)
	main, _ := p.Main()
	b := &builder{p: p, m: &behavior.Model{Outcome: "precisely-modeled"}, effects: effects.New(p, nil), plans: map[*ssa.Function]*functionPlan{}}
	plan := b.plan(main)
	var site *ssa.Call
	for c, s := range plan.Calls {
		if s.Kind == effects.ConstantData {
			site = c.(*ssa.Call)
		}
	}
	if site == nil || !b.consumeConstantData(&frame{f: main, plan: plan}, site) {
		t.Fatal("missing computation proof")
	}
	if !slices.Contains(b.m.Assumptions, effects.ByteSearchModel) {
		t.Fatal("standard-toolchain assumption hidden")
	}
	changed := false
	for f := range p.Calls.Nodes {
		if f != nil && f.String() == "internal/bytealg.IndexByteString" {
			f.Syntax().(*ast.FuncDecl).Body = &ast.BlockStmt{}
			changed = true
		}
	}
	if !changed {
		t.Fatal("assembly declaration missing")
	}
	if b.consumeConstantData(&frame{f: main, plan: plan}, site) || !b.m.HasErrors() {
		t.Fatal("stale operation model accepted")
	}
}
