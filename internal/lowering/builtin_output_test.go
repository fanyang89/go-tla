package lowering

import (
	"strings"
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/effects"
	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
)

func TestInitializerRejectsOutputAfterCachedBuiltin(t *testing.T) {
	p := testutil.Load(t, `package main;var text string;var size=len(text);func output(){println("visible")};func main(){}`)
	a := effects.New(p, nil)
	var site *ssa.Call
	var output ssa.Value
	for _, bb := range p.Roots[0].Func("init").Blocks {
		for _, i := range bb.Instrs {
			if c, ok := i.(*ssa.Call); ok {
				if builtin, ok := c.Common().Value.(*ssa.Builtin); ok && builtin.Name() == "len" {
					site = c
				}
			}
		}
	}
	for _, bb := range p.Roots[0].Func("output").Blocks {
		for _, i := range bb.Instrs {
			if c, ok := i.(*ssa.Call); ok {
				output = c.Common().Value
			}
		}
	}
	if site == nil || output == nil || a.Call(site).Kind != effects.Builtin {
		t.Fatal("cached builtin fixture missing")
	}
	site.Common().Value = output
	b := &builder{p: p, m: &behavior.Model{}, effects: a, loopReported: map[*ssa.Function]bool{}}
	b.initializers()
	found := false
	for _, d := range b.m.Diagnostics {
		if d.Code == "initializer" && strings.Contains(d.Message, "builtin println") {
			found = true
		}
	}
	if !found || !b.m.HasErrors() {
		t.Fatal("cached sequential builtin hid current output")
	}
}
