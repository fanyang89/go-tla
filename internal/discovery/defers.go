package discovery

import (
	"github.com/fanmi/go-tla/internal/behavior"
	"golang.org/x/tools/go/ssa"
)

// Deferred recognizes only direct, zero-argument cleanup methods. It does not
// classify registration as execution of the cleanup primitive.
func Deferred(d *ssa.Defer) (Primitive, bool) {
	c := d.Common()
	f := c.StaticCallee()
	if d.DeferStack != nil || f == nil || f.Signature.Recv() == nil || len(c.Args) != 1 {
		return Primitive{}, false
	}
	var kind behavior.EffectKind
	switch SyncType(f.Signature.Recv().Type()) + "." + f.Name() {
	case "Mutex.Unlock":
		kind = behavior.Unlock
	case "WaitGroup.Done":
		kind = behavior.WaitGroupDone
	default:
		return Primitive{}, false
	}
	return Primitive{Kind: kind, Resource: c.Args[0]}, true
}
