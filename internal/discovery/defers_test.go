package discovery_test

import (
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/discovery"
	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
)

func TestDeferredRegistrationIsNotImmediateUnlock(t *testing.T) {
	p := testutil.Load(t, `package main;import "sync";func main(){var mu sync.Mutex;defer mu.Unlock()}`)
	f, _ := p.Main()
	facts := discovery.Scan(f)
	for _, bb := range f.Blocks {
		for _, i := range bb.Instrs {
			d, ok := i.(*ssa.Defer)
			if !ok {
				continue
			}
			primitive, ok := discovery.Deferred(d)
			if !ok || primitive.Kind != behavior.Unlock || primitive.Resource == nil {
				t.Fatal("missing deferred primitive")
			}
			if !facts.Roots[d] {
				t.Fatal("registration omitted from control/data roots")
			}
			if _, immediate := facts.Primitives[d]; immediate {
				t.Fatal("registration became immediate execution")
			}
			d.DeferStack = d.Common().Args[0]
			if _, ok := discovery.Deferred(d); ok {
				t.Fatal("foreign defer stack accepted")
			}
			return
		}
	}
	t.Fatal("missing SSA defer")
}
