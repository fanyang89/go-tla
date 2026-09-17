package lowering

import (
	"slices"
	"strings"
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/effects"
	"github.com/fanmi/go-tla/internal/testutil"
	"github.com/fanmi/go-tla/internal/tla"
	"golang.org/x/tools/go/ssa"
)

func TestReflectionLiteralConsumption(t *testing.T) {
	p := testutil.Load(t, `package main;import "reflect";func main(){_=reflect.TypeFor[int]()}`)
	m, err := Lower(p, Options{})
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, d := range m.Diagnostics {
		if d.Code == "constant-data-call" && strings.Contains(d.Message, "reflect.rtypeOf") {
			count++
			if d.Line == 0 {
				t.Fatal("source attribution missing")
			}
		}
	}
	if count != 3 || !slices.Contains(m.Assumptions, effects.ReflectionLiteralTypeModel) {
		t.Fatal("real reflect literal proofs missing")
	}
	if !m.HasErrors() {
		t.Fatal("other package initialization skipped")
	}
	if _, _, err := tla.Generate(m); err == nil {
		t.Fatal("unsupported reflection program emitted")
	}
	for _, pkg := range p.SSA.AllPackages() {
		if pkg.Pkg.Path() != "reflect" {
			continue
		}
		init := pkg.Func("init")
		b := &builder{p: p, m: &behavior.Model{}, effects: effects.New(p, nil), plans: map[*ssa.Function]*functionPlan{}}
		plan := b.plan(init)
		for c, s := range plan.Calls {
			if s.Callee == nil || s.Callee.String() != "reflect.rtypeOf" {
				continue
			}
			site := c.(*ssa.Call)
			fr := &frame{f: init, plan: plan}
			if !b.consumeConstantData(fr, site) {
				t.Fatal("fresh literal refused")
			}
			delete(plan.Slice.Data, site.Common().Args[0])
			if b.consumeConstantData(fr, site) {
				t.Fatal("missing box slice accepted")
			}
			plan.Slice.Data[site.Common().Args[0]] = true
			literal := site.Common().Args[0].(*ssa.MakeInterface).X
			delete(plan.Slice.Data, literal)
			if b.consumeConstantData(fr, site) {
				t.Fatal("missing literal slice accepted")
			}
			plan.Slice.Data[literal] = true
			s.Callee.Blocks[0].Instrs = s.Callee.Blocks[0].Instrs[:1]
			if b.consumeConstantData(fr, site) || !b.m.HasErrors() {
				t.Fatal("stale literal wrapper accepted")
			}
			return
		}
	}
	t.Fatal("literal site missing")
}
