package effects

import (
	"github.com/fanmi/go-tla/internal/frontend"
	"testing"
)

func TestActualLexicalFiniteData(t *testing.T) {
	for path, names := range map[string][]string{"./internal/tlalex": {"Identifier"}, "./internal/checker": {"decimalDigits"}} {
		loaded, err := frontend.Load("../..", path)
		if err != nil {
			t.Fatal(err)
		}
		p := frontend.BuildSSA(loaded)
		frontend.BuildCallGraph(p)
		a := New(p, nil)
		for _, name := range names {
			f := p.Roots[0].Func(name)
			if f == nil || !a.ProveFiniteData(f) {
				t.Fatalf("actual finite lexical helper refused: %s.%s", path, name)
			}
		}
	}
}
