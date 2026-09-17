package lowering

import (
	"go/constant"
	"go/types"
	"slices"
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/effects"
	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
)

func TestReflectionMetadataConsumption(t *testing.T) {
	for _, broken := range []string{"abi", "method", "field-name"} {
		t.Run(broken, func(t *testing.T) {
			p := testutil.Load(t, `package main;import "reflect";func info(name string)reflect.StructField{f,ok:=reflect.TypeFor[*struct{X int}]().Elem().FieldByName(name);if !ok{panic("missing")};return f};func main(){_=info("X")}`)
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
				t.Fatal("metadata proof refused")
			}
			if !slices.Contains(b.m.Assumptions, effects.ReflectionTypeModel) || !slices.Contains(b.m.Assumptions, effects.ReflectionQueryModel) {
				t.Fatal("metadata assumptions missing")
			}
			changed := false
			if broken == "field-name" {
				site.Common().Args[0] = ssa.NewConst(constant.MakeString("Missing"), types.Typ[types.String])
				changed = true
			}
			for f := range p.Calls.Nodes {
				if f == nil {
					continue
				}
				if broken == "abi" && f.String() == "internal/abi.NoEscape" {
					for _, bb := range f.Blocks {
						for _, i := range bb.Instrs {
							if x, ok := i.(*ssa.BinOp); ok {
								x.Y = ssa.NewConst(constant.MakeInt64(1), types.Typ[types.Uintptr])
								changed = true
							}
						}
					}
				}
				if broken == "method" && f == p.Roots[0].Func("info") {
					for _, bb := range f.Blocks {
						for _, i := range bb.Instrs {
							if c, ok := i.(*ssa.Call); ok && c.Common().IsInvoke() && c.Common().Method.Name() == "Elem" {
								iface := c.Common().Value.Type().Underlying().(*types.Interface)
								for n := range iface.NumMethods() {
									if iface.Method(n).Name() == "Key" {
										c.Common().Method = iface.Method(n)
										changed = true
									}
								}
							}
						}
					}
				}
			}
			if !changed {
				t.Fatal("mutation missing")
			}
			if b.consumeConstantData(&frame{f: main, plan: plan}, site) || !b.m.HasErrors() {
				t.Fatal("stale reflection proof accepted")
			}
		})
	}
}
