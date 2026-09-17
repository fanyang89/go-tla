package lowering

import (
	"fmt"
	"golang.org/x/tools/go/ssa"
)

func (b *builder) consumeFiniteData(fr *frame, site *ssa.Call, f *ssa.Function) bool {
	valid := b.p.CallTarget(site) == f && b.effects.ProveFiniteData(f)
	for _, operand := range site.Operands(nil) {
		if operand != nil && !b.requireData(fr, *operand) {
			valid = false
		}
	}
	if !valid {
		b.diag("error", "finite-data-contract", "finite computation lacks a current body/graph/slice proof", site.Pos())
	}
	return valid
}

func (b *builder) recordFiniteData(f *ssa.Function) {
	if b.loopReported[f] {
		return
	}
	if b.loopReported == nil {
		b.loopReported = map[*ssa.Function]bool{}
	}
	b.loopReported[f] = true
	b.diag("info", "finite-data-loop", "Proved read-only, length-bounded computation: "+f.String()+"; data results are abstract, no iteration truncation or trusted-call contract", f.Pos())
}

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
