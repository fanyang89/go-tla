package frontend_test

import (
	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
	"testing"
)

func TestCallbackBodyGraphDoesNotInventInvocation(t *testing.T) {
	p := testutil.Load(t, `package main;func cleanup(){};func apply(f func()){f()};func main(){apply(func(){cleanup()})}`)
	main, _ := p.Main()
	var callback, apply *ssa.Function
	for _, bb := range main.Blocks {
		for _, i := range bb.Instrs {
			if c, ok := i.(*ssa.Call); ok && c.Common().StaticCallee() != nil && c.Common().StaticCallee().Name() == "apply" {
				apply = p.CallTarget(c)
				callback, _ = c.Common().Args[0].(*ssa.Function)
			}
		}
	}
	if callback == nil || apply == nil {
		t.Fatal("fixture missing")
	}
	found := false
	for _, bb := range callback.Blocks {
		for _, i := range bb.Instrs {
			if c, ok := i.(*ssa.Call); ok {
				if p.CallTarget(c) == nil || p.CallTarget(c).Name() != "cleanup" {
					t.Fatal("callback body edge missing")
				}
				found = true
			}
		}
	}
	if !found {
		t.Fatal("callback call missing")
	}
	for _, bb := range apply.Blocks {
		for _, i := range bb.Instrs {
			if c, ok := i.(*ssa.Call); ok && p.CallTarget(c) != nil {
				t.Fatal("dynamic parameter invocation guessed")
			}
		}
	}
}
