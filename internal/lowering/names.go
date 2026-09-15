package lowering

import (
	"fmt"

	"github.com/fanmi/go-tla/internal/behavior"
)

// Internal SSA instruction nodes never become the public location namespace.
// Number behavioral locations per process only after atomic regions are formed.
func (b *builder) canonicalLocations() {
	for pi := range b.m.Processes {
		p := &b.m.Processes[pi]
		locs := map[string]string{p.Entry: p.ID + "_entry", p.Terminal: p.ID + "_Done"}
		next := 0
		for _, loc := range p.Locations {
			if locs[loc] == "" {
				next++
				locs[loc] = fmt.Sprintf("%s_step_%d", p.ID, next)
			}
		}
		groups := map[string]string{}
		for _, tr := range b.m.Transitions {
			if tr.Process == p.ID && tr.ChoiceGroup != "" && groups[tr.ChoiceGroup] == "" {
				groups[tr.ChoiceGroup] = fmt.Sprintf("%s_select_group_%d", p.ID, len(groups)+1)
			}
		}
		for ti := range b.m.Transitions {
			tr := &b.m.Transitions[ti]
			if tr.Process != p.ID {
				continue
			}
			tr.Source, tr.Destination = locs[tr.Source], locs[tr.Destination]
			tr.ChoiceGroup = groups[tr.ChoiceGroup]
			tr.Guard = renameGroups(tr.Guard, groups)
		}
		p.Entry, p.Terminal = locs[p.Entry], locs[p.Terminal]
		for i, loc := range p.Locations {
			p.Locations[i] = locs[loc]
		}
	}
}
func renameGroups(g behavior.Guard, groups map[string]string) behavior.Guard {
	if g.Kind == behavior.Default {
		g.Variable = groups[g.Variable]
	}
	terms := make([]behavior.Guard, len(g.Terms))
	for i, term := range g.Terms {
		terms[i] = renameGroups(term, groups)
	}
	if len(terms) > 0 {
		g.Terms = terms
	}
	return g
}
