package tla

import (
	"fmt"
	"github.com/fanmi/go-tla/internal/behavior"
)

// Validate prevents a new frontend/IR producer from silently using an effect the backend ignores.
func Validate(m *behavior.Model) error {
	if len(m.Processes) == 0 {
		return fmt.Errorf("model has no processes")
	}
	ps := map[string]behavior.Process{}
	resources := map[string]string{"nil": "channel"}
	vars := map[string]bool{}
	ids := map[string]bool{}
	add := func(id, kind string) error {
		if id == "" || resources[id] != "" {
			return fmt.Errorf("duplicate/empty resource %q", id)
		}
		resources[id] = kind
		return nil
	}
	for _, c := range m.Channels {
		if c.Capacity < 0 {
			return fmt.Errorf("negative capacity")
		}
		if err := add(c.ID, "channel"); err != nil {
			return err
		}
	}
	for _, r := range m.Mutexes {
		if err := add(r, "mutex"); err != nil {
			return err
		}
	}
	for _, r := range m.WaitGroups {
		if err := add(r, "waitgroup"); err != nil {
			return err
		}
	}
	for _, p := range m.Processes {
		if _, ok := ps[p.ID]; ok {
			return fmt.Errorf("duplicate process")
		}
		ps[p.ID] = p
		for _, v := range p.Locals {
			if vars[v.Name] {
				return fmt.Errorf("duplicate variable")
			}
			vars[v.Name] = true
		}
	}
	if _, ok := ps[m.InitialState.Main]; !ok {
		return fmt.Errorf("missing main process")
	}
	var guard func(behavior.Guard) error
	guard = func(g behavior.Guard) error {
		switch g.Kind {
		case behavior.True, behavior.Choice, behavior.Default:
		case behavior.Equal:
			if !vars[g.Variable] {
				return fmt.Errorf("unknown guard variable")
			}
		case behavior.All:
			for _, t := range g.Terms {
				if err := guard(t); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("unsupported guard %q", g.Kind)
		}
		return nil
	}
	for _, t := range m.Transitions {
		if ids[t.ID] {
			return fmt.Errorf("duplicate transition")
		}
		ids[t.ID] = true
		if _, ok := ps[t.Process]; !ok {
			return fmt.Errorf("unknown process")
		}
		if err := guard(t.Guard); err != nil {
			return err
		}
		visible := 0
		for _, e := range t.Effects {
			kind := ""
			switch e.Kind {
			case behavior.Send, behavior.Receive, behavior.CloseChannel:
				kind = "channel"
				visible++
			case behavior.Lock, behavior.Unlock:
				kind = "mutex"
				visible++
			case behavior.WaitGroupAdd, behavior.WaitGroupDone, behavior.WaitGroupWait:
				kind = "waitgroup"
				visible++
			case behavior.Spawn:
				visible++
				if _, ok := ps[e.Process]; !ok {
					return fmt.Errorf("unknown spawn process")
				}
			case behavior.AssignAbstractState:
				if !vars[e.Variable] {
					return fmt.Errorf("unknown assignment variable")
				}
			case behavior.Assert:
				visible++
			default:
				return fmt.Errorf("unsupported effect %q", e.Kind)
			}
			if kind != "" && resources[e.Resource] != kind {
				return fmt.Errorf("invalid %s resource %q", kind, e.Resource)
			}
		}
		if visible > 1 {
			return fmt.Errorf("multiple scheduling effects in one transition unsupported")
		}
	}
	if len(m.SharedState) > 0 {
		return fmt.Errorf("shared-state variables not supported by this backend")
	}
	for _, a := range m.Assertions {
		if a.Kind != "NoSynchronizationErrors" {
			return fmt.Errorf("unsupported assertion %q", a.Kind)
		}
	}
	return nil
}
