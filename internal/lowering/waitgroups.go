package lowering

import "github.com/fanmi/go-tla/internal/behavior"

func (b *builder) checkWaitGroupPhases() {
	for _, tr := range behavior.WaitGroupPhaseViolations(b.m) {
		b.diagAt("error", "waitgroup-phase", "positive WaitGroup.Add must occur in main before any goroutine spawn or Wait on that group; reuse/concurrent enrollment unsupported", tr.SourcePosition)
	}
}
