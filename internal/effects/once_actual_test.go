package effects

import (
	"github.com/fanmi/go-tla/internal/frontend"
	"golang.org/x/tools/go/ssa"
	"testing"
)

func TestActualGodebugOnceContract(t *testing.T) {
	loaded, err := frontend.LoadContext(t.Context(), "../checker", ".")
	if err != nil {
		t.Fatal(err)
	}
	p := frontend.BuildSSA(loaded)
	frontend.BuildCallGraph(p)
	a := New(p, nil)
	count, refused := 0, 0
	for f := range p.Calls.Nodes {
		if f == nil || f.Pkg == nil || f.Pkg.Pkg.Path() != "internal/godebug" || (f.Name() != "Value" && f.Name() != "IncNonDefault") {
			continue
		}
		for _, bb := range f.Blocks {
			for _, i := range bb.Instrs {
				if c, ok := i.(*ssa.Call); ok && IsOnceDo(c) {
					if f.Name() == "IncNonDefault" {
						if a.ProveOnceCall(c) != nil {
							t.Fatal("bound wrapper unexpectedly admitted")
						}
						refused++
						continue
					}
					if a.ProveOnceCall(c) == nil {
						t.Fatalf("actual Once operation contract missing: %s", f)
					}
					count++
				}
			}
		}
	}
	if count != 1 || refused != 1 {
		t.Fatal("actual dependency operation missing")
	}
	// This proves eligibility of the API/callback binding, not the callback's
	// transitive effects or successful whole-CLI consumption.
	t.Logf("checked %d actual internal/godebug Once contracts", count)
}
