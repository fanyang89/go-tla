package behavior

// WaitGroupPhaseViolations identifies enrollment requiring waiter-generation
// semantics beyond a counter-only backend. Guards are conservatively ignored.
func WaitGroupPhaseViolations(m *Model) []Transition {
	var violations []Transition
	workersActive := false
	for _, id := range m.InitialState.Active {
		if id != m.InitialState.Main {
			workersActive = true
		}
	}
	for _, add := range m.Transitions {
		bad := false
		for _, effect := range add.Effects {
			if effect.Kind != WaitGroupAdd || effect.Value <= 0 {
				continue
			}
			if add.Process != m.InitialState.Main || workersActive {
				bad = true
			}
			visited := map[string]bool{}
			var before func(string)
			before = func(loc string) {
				if visited[loc] {
					return
				}
				visited[loc] = true
				for _, tr := range m.Transitions {
					if tr.Process != add.Process || tr.Destination != loc {
						continue
					}
					for _, e := range tr.Effects {
						if e.Kind == Spawn || (e.Kind == WaitGroupWait && e.Resource == effect.Resource) {
							bad = true
						}
					}
					before(tr.Source)
				}
			}
			before(add.Source)
		}
		if bad {
			violations = append(violations, add)
		}
	}
	return violations
}
