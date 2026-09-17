package tla

import (
	"fmt"
	"slices"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/tlalex"
)

// Validate adds implementation capability checks to generic IR validation. Other
// backends must use behavior.Validate, not inherit TLA runtime conventions.
func Validate(m *behavior.Model) error {
	if err := behavior.Validate(m); err != nil {
		return err
	}
	if len(m.SharedState) != 0 {
		return fmt.Errorf("TLA backend does not support shared abstract variables")
	}
	if len(m.Assertions) != 1 || m.Assertions[0].Kind != "NoSynchronizationErrors" {
		return fmt.Errorf("TLA backend requires the NoSynchronizationErrors assertion")
	}
	checkID := func(id string) error {
		if !tlalex.Identifier(id) {
			return fmt.Errorf("TLA backend cannot encode identifier %q", id)
		}
		return nil
	}
	for _, p := range m.Processes {
		if err := checkID(p.ID); err != nil {
			return err
		}
		for _, loc := range p.Locations {
			if loc == "Dormant" {
				return fmt.Errorf("Dormant is reserved by the TLA runtime")
			}
			if err := checkID(loc); err != nil {
				return err
			}
		}
		for _, v := range p.Locals {
			if err := checkID(v.Name); err != nil {
				return err
			}
		}
		if err := finiteControl(m, p.ID); err != nil {
			return err
		}
	}
	for _, c := range m.Channels {
		if err := checkID(c.ID); err != nil {
			return err
		}
	}
	for _, ids := range [][]string{m.Mutexes, m.WaitGroups} {
		for _, id := range ids {
			if err := checkID(id); err != nil {
				return err
			}
		}
	}
	names := map[string]bool{}
	add := func(name string) error {
		if names[name] {
			return fmt.Errorf("TLA action name collision %q", name)
		}
		names[name] = true
		return nil
	}
	domains := map[string][]int{}
	for _, p := range m.Processes {
		for _, v := range p.Locals {
			domains[v.Name] = v.Domain
		}
	}
	for _, t := range m.Transitions {
		for _, other := range m.Transitions {
			if ambiguousChoice(t, other, domains) {
				return fmt.Errorf("ambiguous blocking alternatives at %s/%s require a select group or explicit branch commitment", t.Process, t.Source)
			}
		}
		if err := checkID(t.ID); err != nil {
			return err
		}
		visible := 0
		for _, e := range t.Effects {
			if e.Kind != behavior.AssignAbstractState {
				visible++
			}
			if e.Kind == behavior.Spawn {
				if e.Process == t.Process || slices.Contains(m.InitialState.Active, e.Process) {
					return fmt.Errorf("spawn target %q must be a distinct dormant process", e.Process)
				}
				for _, other := range m.Transitions {
					for _, oe := range other.Effects {
						if oe.Kind == behavior.Spawn && oe.Process == e.Process && other.ID != t.ID &&
							(other.Process != t.Process || pathExists(m, t.Process, t.Destination, other.Source)) {
							return fmt.Errorf("process %q may be spawned more than once", e.Process)
						}
					}
				}
			}
		}
		if visible > 1 {
			return fmt.Errorf("multiple scheduling effects in one transition unsupported")
		}
		for _, name := range []string{stepName(t), "Pre_" + t.ID, "Ready_" + t.ID, "Register_" + t.ID} {
			if err := add(name); err != nil {
				return err
			}
		}
	}
	for _, a := range m.Transitions {
		ae, ok := communication(a)
		if !ok || ae.Kind != behavior.Send || ae.Resource == "nil" {
			continue
		}
		cap := -1
		for _, ch := range m.Channels {
			if ch.ID == ae.Resource {
				cap = ch.Capacity
			}
		}
		if cap != 0 {
			continue
		}
		for _, b := range m.Transitions {
			be, ok := communication(b)
			if ok && be.Kind == behavior.Receive && be.Resource == ae.Resource && a.Process != b.Process {
				if err := add(rendezvousName(a, b)); err != nil {
					return err
				}
			}
		}
	}
	if bad := behavior.WaitGroupPhaseViolations(m); len(bad) > 0 {
		return fmt.Errorf("TLA counter-only WaitGroup requires single-phase enrollment: %s", bad[0].ID)
	}
	return nil
}

func outgoing(m *behavior.Model, process, loc string) []string {
	var next []string
	for _, t := range m.Transitions {
		if t.Process == process && t.Source == loc {
			next = append(next, t.Destination)
		}
	}
	return next
}
func pathExists(m *behavior.Model, process, from, to string) bool {
	seen := map[string]bool{}
	var visit func(string) bool
	visit = func(loc string) bool {
		if loc == to {
			return true
		}
		if seen[loc] {
			return false
		}
		seen[loc] = true
		for _, next := range outgoing(m, process, loc) {
			if visit(next) {
				return true
			}
		}
		return false
	}
	return visit(from)
}
