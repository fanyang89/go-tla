package lowering

import (
	"fmt"
	"golang.org/x/tools/go/ssa"
)

// Only reached functions publish loop proof/rejection records. Unused unsupported
// library helpers must not make an otherwise supported entry point fail.
func (b *builder) recordLoops(f *ssa.Function) bool {
	valid := true
	for _, proof := range b.p.LoopProofs(f) {
		if proof.ChannelRange {
			continue
		}
		pos := proof.Source
		pos.Function = b.p.Position(f.Pos(), f).Function
		if proof.Reason != "" {
			b.diagAt("error", "unsupported-loop", proof.Reason, pos)
			valid = false
		} else if !b.loopReported[f] {
			message := fmt.Sprintf("Proved integer range bound: %d iterations at %s:%d:%d; expanded without truncation", proof.Count, pos.File, pos.Line, pos.Column)
			b.diagAt("info", "proved-loop", message, pos)
			b.m.Assumptions = append(b.m.Assumptions, message)
		}
	}
	b.loopReported[f] = true
	return valid
}
