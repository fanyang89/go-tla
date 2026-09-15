package lowering

import (
	"fmt"
	"github.com/fanmi/go-tla/internal/behavior"
)

// regions is pass 7/8: epsilon-close ordinary computation into atomic behavioral
// regions. Only behavioral roots, completion and exact select dispatch survive.
func (b *builder) regions() {
	for pi := range b.m.Processes {
		p := &b.m.Processes[pi]
		todo := []string{p.Entry}
		seen := map[string]bool{}
		for len(todo) > 0 {
			src := todo[0]
			todo = todo[1:]
			if seen[src] {
				continue
			}
			seen[src] = true
			p.Locations = append(p.Locations, src)
			var walk func(string, []behavior.Guard, map[string]bool)
			walk = func(cur string, guards []behavior.Guard, path map[string]bool) {
				if path[cur] {
					return
				}
				path[cur] = true
				defer delete(path, cur)
				n := b.nodes[cur]
				if n == nil {
					return
				}
				if cur == p.ID+"_Done" {
					if src != cur {
						b.m.Transitions = append(b.m.Transitions, behavior.Transition{ID: fmt.Sprintf("%s_Finish_%d", p.ID, len(b.m.Transitions)), Process: p.ID, Source: src, Guard: combine(guards), Destination: cur, SourcePosition: p.Source})
						todo = append(todo, cur)
					}
					return
				}
				for _, e := range n.edges {
					gs := append(append([]behavior.Guard{}, guards...), e.guard)
					if len(e.effects) == 0 {
						walk(e.to, gs, path)
						continue
					}
					name := string(e.effects[0].Kind)
					b.m.Transitions = append(b.m.Transitions, behavior.Transition{ID: fmt.Sprintf("%s_%s_L%d_%d", p.ID, name, e.pos.Line, len(b.m.Transitions)), Process: p.ID, Source: src, Guard: combine(gs), Effects: e.effects, Destination: e.to, SourcePosition: e.pos, ChoiceGroup: e.group})
					todo = append(todo, e.to)
				}
			}
			walk(src, nil, map[string]bool{})
		}
	}
}
func combine(gs []behavior.Guard) behavior.Guard {
	out := []behavior.Guard{}
	for _, g := range gs {
		if g.Kind == behavior.True && !g.Negated {
			continue
		}
		out = append(out, g)
	}
	if len(out) == 0 {
		return behavior.Guard{Kind: behavior.True}
	}
	if len(out) == 1 {
		return out[0]
	}
	return behavior.Guard{Kind: behavior.All, Terms: out}
}
