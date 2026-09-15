package lowering

import "github.com/fanmi/go-tla/internal/behavior"

// A counter is sufficient for a single enrollment phase. Reject reuse and
// concurrent positive Add rather than hiding Go's waiter-generation misuse rules.
func (b *builder) checkWaitGroupPhases() {
	for _, add := range b.m.Transitions {
		for _, effect := range add.Effects {
			if effect.Kind != behavior.WaitGroupAdd || effect.Value <= 0 {
				continue
			}
			unsafe := add.Process != b.m.InitialState.Main
			visited := map[string]bool{}
			var before func(string)
			before = func(loc string) {
				if visited[loc] {
					return
				}
				visited[loc] = true
				for _, tr := range b.m.Transitions {
					if tr.Process != add.Process || tr.Destination != loc {
						continue
					}
					for _, e := range tr.Effects {
						if e.Kind == behavior.Spawn || (e.Kind == behavior.WaitGroupWait && e.Resource == effect.Resource) {
							unsafe = true
						}
					}
					before(tr.Source)
				}
			}
			before(add.Source)
			if unsafe {
				b.diag("error", "waitgroup-phase", "positive WaitGroup.Add must occur in main before any goroutine spawn or Wait on that group; reuse/concurrent enrollment unsupported", 0)
			}
		}
	}
}
