package tla

import (
	"fmt"

	"github.com/fanmi/go-tla/internal/behavior"
)

// Finite state does not imply termination. Repeatable control must cross an
// observable status-producing receive, and cannot grow counters or spawn again.
// Queues, locks, locations and local domains are already bounded by the IR.
func finiteControl(m *behavior.Model, process string) error {
	for _, t := range m.Transitions {
		if t.Process != process || !pathExists(m, process, t.Destination, t.Source) {
			continue
		}
		for _, e := range t.Effects {
			switch e.Kind {
			case behavior.Spawn, behavior.WaitGroupAdd, behavior.WaitGroupDone:
				return fmt.Errorf("repeatable %s at %s cannot guarantee finite counters/process instances", e.Kind, t.ID)
			}
		}
	}
	color := map[string]int{}
	var visit func(string) bool
	visit = func(loc string) bool {
		if color[loc] == 1 {
			return true
		}
		if color[loc] == 2 {
			return false
		}
		color[loc] = 1
		for _, t := range m.Transitions {
			if t.Process != process || t.Source != loc {
				continue
			}
			statusReceive := false
			for _, e := range t.Effects {
				if e.Kind == behavior.Receive && e.Variable != "" {
					statusReceive = true
				}
			}
			if !statusReceive && visit(t.Destination) {
				return true
			}
		}
		color[loc] = 2
		return false
	}
	for _, p := range m.Processes {
		if p.ID == process {
			for _, loc := range p.Locations {
				if visit(loc) {
					return fmt.Errorf("cyclic process control must cross a status-producing receive on every cycle")
				}
			}
		}
	}
	return nil
}
