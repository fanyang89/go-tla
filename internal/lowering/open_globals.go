package lowering

import (
	"github.com/fanmi/go-tla/internal/behavior"
	"golang.org/x/tools/go/ssa"
)

// A creation proof does not discharge sends, receives or closes during init.
// Those instructions still pass through the normal initializer checks.
func (b *builder) consumeOpenGlobal(i ssa.Instruction) bool {
	makeChan, ok := i.(*ssa.MakeChan)
	if !ok {
		return false
	}
	proof := b.proveGlobalCreation(makeChan)
	if proof == nil {
		return false
	}
	if b.openGlobals == nil {
		b.openGlobals = map[*ssa.Global]*globalChannelProof{}
	}
	if b.globals == nil {
		b.globals = map[*ssa.Global]string{}
	}
	id := b.fresh("global_" + proof.global.String())
	b.globals[proof.global] = id
	b.openGlobals[proof.global] = proof
	b.m.Channels = append(b.m.Channels, behavior.Channel{ID: id, Capacity: proof.capacity, Source: b.position(makeChan.Pos(), makeChan.Parent())})
	b.diag("info", "open-global-init", "proved unique initially empty global channel creation: "+proof.global.String(), makeChan.Pos())
	return true
}
