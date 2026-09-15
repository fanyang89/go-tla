package lowering

import (
	"github.com/fanmi/go-tla/internal/frontend"
	"golang.org/x/tools/go/ssa"
)

func (b *builder) checkUnsafePointer(i ssa.Instruction) {
	if frontend.UsesUnsafePointer(i) {
		b.diag("error", "unsafe-pointer", "unsafe pointer operations may mutate synchronization identity or state; unsupported", i.Pos())
	}
}
