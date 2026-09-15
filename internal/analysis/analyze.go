// Package analysis orchestrates the explicit frontend passes; backends consume only its IR.
package analysis

import (
	"fmt"
	"io"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/frontend"
	"github.com/fanmi/go-tla/internal/lowering"
)

func Analyze(dir string, patterns []string, opts lowering.Options) (*behavior.Model, error) {
	ps, err := frontend.Load(dir, patterns...)
	if err != nil {
		return nil, err
	}
	p := frontend.BuildSSA(ps)
	frontend.BuildCallGraph(p)
	return lowering.Lower(p, opts)
}

// PrintStatistics reports IR size separately from checker state-space statistics.
func PrintStatistics(w io.Writer, m *behavior.Model) {
	s := m.Statistics()
	label := "Model size (IR)"
	if m.HasErrors() {
		label = "Partial model size (IR)"
	}
	fmt.Fprintf(w, "%s: processes=%d channels=%d mutexes=%d waitgroups=%d locations=%d transitions=%d predicates=%d\n",
		label, s.Processes, s.Channels, s.Mutexes, s.WaitGroups, s.Locations, s.Transitions, s.AbstractedPredicates)
}

func Inspect(w io.Writer, m *behavior.Model) {
	fmt.Fprintf(w, "Outcome: %s\n", m.Outcome)
	PrintStatistics(w, m)
	fmt.Fprintln(w, "Processes:")
	for _, p := range m.Processes {
		fmt.Fprintf(w, "  %s (%s)\n", p.ID, p.Kind)
	}
	fmt.Fprintln(w, "Channels:")
	for _, c := range m.Channels {
		fmt.Fprintf(w, "  %s capacity=%d\n", c.ID, c.Capacity)
	}
	fmt.Fprintln(w, "Synchronization:")
	for _, x := range m.Mutexes {
		fmt.Fprintf(w, "  mutex %s\n", x)
	}
	for _, x := range m.WaitGroups {
		fmt.Fprintf(w, "  waitgroup %s\n", x)
	}
	fmt.Fprintln(w, "Abstracted predicates:")
	for _, p := range m.AbstractedPredicates {
		fmt.Fprintf(w, "  %s (%s:%d)\n", p.Name, p.Source.File, p.Source.Line)
	}
	fmt.Fprintln(w, "Transitions:")
	for _, t := range m.Transitions {
		fmt.Fprintf(w, "  %s: %s -> %s", t.ID, t.Source, t.Destination)
		for _, e := range t.Effects {
			fmt.Fprintf(w, " %s(%s%s)", e.Kind, e.Resource, e.Process)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w, "Assumptions:")
	for _, a := range m.Assumptions {
		fmt.Fprintf(w, "  %s\n", a)
	}
}
