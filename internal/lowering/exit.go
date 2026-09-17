package lowering

import (
	"slices"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/discovery"
	"golang.org/x/tools/go/ssa"
)

func (b *builder) consumeExit(fr *frame, site *ssa.Call) bool {
	primitive, ok := discovery.Recognize(site)
	target := b.p.CallTarget(site)
	valid := ok && primitive.Kind == behavior.Exit && target != nil && target == site.Common().StaticCallee() && fr.plan.Calls[site].Callee == target
	for _, operand := range site.Operands(nil) {
		if operand != nil && !b.requireData(fr, *operand) {
			valid = false
		}
	}
	if !valid {
		b.diag("error", "exit-contract", "program exit lacks a current signature/graph/argument-slice proof", site.Pos())
		return false
	}
	assumption := "os.Exit models ordinary process termination without deferred cleanup; exit status is abstract and Go test panic-on-exit interception is excluded."
	if !slices.Contains(b.m.Assumptions, assumption) {
		b.m.Assumptions = append(b.m.Assumptions, assumption)
	}
	b.diag("info", "program-exit", "modeled immediate whole-program os.Exit without cleanup", site.Pos())
	return true
}
