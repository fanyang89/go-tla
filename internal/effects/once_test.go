package effects

import (
	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
	"testing"
)

func TestOnceStandardProof(t *testing.T) {
	p := testutil.Load(t, `package main;import "sync";func main(){var o sync.Once;o.Do(func(){})}`)
	main, _ := p.Main()
	a := New(p, nil)
	found := false
	for _, bb := range main.Blocks {
		for _, i := range bb.Instrs {
			if c, ok := i.(*ssa.Call); ok && IsOnceDo(c) {
				found = true
				if a.ProveOnceCall(c) == nil {
					t.Fatal("standard proof failed")
				}
				if a.Call(c).Kind != OnceDo {
					t.Fatal("operation not classified")
				}
			}
		}
	}
	if !found {
		t.Fatal("Once.Do missing")
	}
}
