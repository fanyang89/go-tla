package lowering

import (
	"go/types"

	"golang.org/x/tools/go/ssa"
)

func channelResult(f *ssa.Function) bool {
	if f == nil || f.Signature == nil || f.Signature.Results().Len() != 1 {
		return false
	}
	_, ok := f.Signature.Results().At(0).Type().Underlying().(*types.Chan)
	return ok
}

// The body and its cleanup are lowered before consuming this invocation's
// result. All normal returns must retain the same channel identity; there is
// no dynamic points-to choice and no transfer of object/closure ownership.
func (b *builder) returnedChannel(fr *frame) string {
	if !channelResult(fr.f) {
		return "invalid"
	}
	id := ""
	for _, bb := range fr.f.Blocks {
		if recoveryReturnBlock(fr.f, bb) {
			continue
		}
		for _, i := range bb.Instrs {
			r, ok := i.(*ssa.Return)
			if !ok {
				continue
			}
			if len(r.Results) != 1 || r.Results[0] == nil || r.Parent() != fr.f || r.Block() != bb || !types.Identical(r.Results[0].Type(), fr.f.Signature.Results().At(0).Type()) || !fr.plan.Slice.Roots[r] || !b.requireData(fr, r.Results[0]) {
				b.diag("error", "return-contract", "channel return lacks a current typed retained operand proof", r.Pos())
				return "invalid"
			}
			value, ok := b.returnOrigin(fr, r.Results[0])
			if !ok {
				return "invalid"
			}
			next := b.identity(fr, value, map[ssa.Value]bool{})
			if next == "" || next == "invalid" {
				return "invalid"
			}
			if id != "" && id != next {
				b.diag("error", "return-identity", "channel returns have different possible identities", r.Pos())
				return "invalid"
			}
			id = next
		}
	}
	if id == "" {
		b.diag("error", "return-contract", "channel result has no proved return", fr.f.Pos())
		return "invalid"
	}
	b.diag("info", "channel-return", "proved one channel identity across this invocation's returns; body and cleanup analyzed", fr.f.Pos())
	return id
}
