package tla

import (
	"slices"

	"github.com/fanmi/go-tla/internal/behavior"
)

// Registration represents a blocked operation, not a branch commitment. Require
// a select group, equivalent duplicates, or provably disjoint local guards before
// allowing competing operations at one PC. Other producers must insert a choice
// commit location instead of hiding a possible block behind a ready alternative.
func ambiguousChoice(a, b behavior.Transition, domains map[string][]int) bool {
	if a.Process != b.Process || a.Source != b.Source || a.ID == b.ID {
		return false
	}
	blocking := func(t behavior.Transition) bool {
		for _, e := range t.Effects {
			switch e.Kind {
			case behavior.Send, behavior.Receive, behavior.Lock, behavior.WaitGroupWait:
				return true
			}
		}
		return false
	}
	if !blocking(a) && !blocking(b) {
		return false
	}
	if a.ChoiceGroup != "" && a.ChoiceGroup == b.ChoiceGroup {
		return false
	}
	if a.Destination == b.Destination && a.ChoiceGroup == b.ChoiceGroup && slices.Equal(a.Effects, b.Effects) {
		return false
	}
	constraints := map[string][]int{}
	impossible := false
	var apply func(behavior.Guard)
	apply = func(g behavior.Guard) {
		switch g.Kind {
		case behavior.True:
			if g.Negated {
				impossible = true
			}
		case behavior.Equal:
			values, ok := constraints[g.Variable]
			if !ok {
				values = slices.Clone(domains[g.Variable])
			}
			filtered := []int{}
			for _, value := range values {
				if (value == g.Value) != g.Negated {
					filtered = append(filtered, value)
				}
			}
			constraints[g.Variable] = filtered
			if len(filtered) == 0 {
				impossible = true
			}
		case behavior.All:
			// Ignoring a negated conjunction is an over-approximation of its guard;
			// it can only cause rejection, never an invalid disjointness proof.
			if !g.Negated {
				for _, term := range g.Terms {
					apply(term)
				}
			}
		}
	}
	apply(a.Guard)
	apply(b.Guard)
	return !impossible
}
