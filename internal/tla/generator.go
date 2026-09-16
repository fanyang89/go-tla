// Package tla is a backend for behavioral IR; it never imports the Go frontend.
package tla

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fanmi/go-tla/internal/behavior"
)

type output struct {
	m       *behavior.Model
	text    strings.Builder
	caps    map[string]int
	actions []string
}

func quote(s string) string                          { return strconv.Quote(s) }
func stepName(t behavior.Transition) string          { return "Step_" + t.ID }
func rendezvousName(a, b behavior.Transition) string { return "Rendezvous_" + a.ID + "_" + b.ID }
func comment(s string) string                        { return strings.NewReplacer("\r", " ", "\n", " ").Replace(s) }
func set(ss []string) string {
	a := []string{}
	for _, s := range ss {
		a = append(a, quote(s))
	}
	return "{" + strings.Join(a, ", ") + "}"
}
func disj(xs []string) string {
	if len(xs) == 0 {
		return "FALSE"
	}
	return "(" + strings.Join(xs, " \\/ ") + ")"
}
func conj(xs []string) string {
	if len(xs) == 0 {
		return "TRUE"
	}
	return "(" + strings.Join(xs, " /\\ ") + ")"
}
func (o *output) line(s string, args ...any) { fmt.Fprintf(&o.text, s+"\n", args...) }
func communication(t behavior.Transition) (behavior.Effect, bool) {
	for _, e := range t.Effects {
		if e.Kind == behavior.Send || e.Kind == behavior.Receive {
			return e, true
		}
	}
	return behavior.Effect{}, false
}

// Generate is pass 9. Unsupported frontend results are rejected rather than emitted.
func Generate(m *behavior.Model) (string, string, error) {
	if err := Validate(m); err != nil {
		return "", "", err
	}
	o := &output{m: m, caps: map[string]int{}}
	channels := []string{}
	for _, c := range m.Channels {
		channels = append(channels, c.ID)
		o.caps[c.ID] = c.Capacity
	}
	procs := []string{}
	locals := []string{}
	for _, p := range m.Processes {
		procs = append(procs, p.ID)
		for _, v := range p.Locals {
			locals = append(locals, v.Name)
		}
	}
	o.line("---- MODULE model ----")
	o.line("EXTENDS Naturals, Integers, Sequences, TLC")
	o.line("\\* Generated from a backend-independent Concurrent Behavioral IR.")
	for _, a := range m.Assumptions {
		o.line("\\* Assumption: %s", comment(a))
	}
	o.line("ProcSet == %s", set(procs))
	o.line("ChannelSet == %s", set(channels))
	o.line("MutexSet == %s", set(m.Mutexes))
	o.line("WaitGroupSet == %s", set(m.WaitGroups))
	o.line("LocalSet == %s", set(locals))
	o.line("VARIABLES pc, queues, closed, locks, wg, local, fault, waiting")
	o.line("vars == <<pc, queues, closed, locks, wg, local, fault, waiting>>")
	pcs := []string{}
	active := map[string]bool{}
	for _, id := range m.InitialState.Active {
		active[id] = true
	}
	mainTerminal := ""
	for _, p := range m.Processes {
		loc := "Dormant"
		if active[p.ID] {
			loc = p.Entry
		}
		if p.ID == m.InitialState.Main {
			mainTerminal = p.Terminal
		}
		pcs = append(pcs, fmt.Sprintf("p = %s -> %s", quote(p.ID), quote(loc)))
	}
	o.line("Init ==")
	o.line("    /\\ pc = [p \\in ProcSet |-> CASE %s]", strings.Join(pcs, " [] "))
	o.line("    /\\ queues = [c \\in ChannelSet |-> <<>>]")
	o.line("    /\\ closed = [c \\in ChannelSet |-> FALSE]")
	o.line("    /\\ locks = [m \\in MutexSet |-> FALSE]")
	o.line("    /\\ wg = [w \\in WaitGroupSet |-> 0]")
	initial := []string{}
	for _, p := range m.Processes {
		for _, v := range p.Locals {
			initial = append(initial, fmt.Sprintf("v = %s -> %d", quote(v.Name), v.Initial))
		}
	}
	if len(initial) == 0 {
		o.line("    /\\ local = [v \\in LocalSet |-> 0]")
	} else {
		o.line("    /\\ local = [v \\in LocalSet |-> CASE %s]", strings.Join(initial, " [] "))
	}
	o.line("    /\\ fault = FALSE")
	o.line("    /\\ waiting = [p \\in ProcSet |-> FALSE]")
	o.line("Running == ~fault /\\ pc[%s] # %s", quote(m.InitialState.Main), quote(mainTerminal))
	o.line("NoSynchronizationErrors == ~fault")
	for _, t := range m.Transitions {
		if hasDefault(t.Guard) {
			continue
		}
		pos := t.SourcePosition
		o.line("\n\\* %s.%s %s:%d:%d", comment(pos.Package), comment(pos.Function), comment(pos.File), pos.Line, pos.Column)
		o.line("Pre_%s == Running /\\ pc[%s] = %s /\\ %s", t.ID, quote(t.Process), quote(t.Source), o.guard(t.Guard, t))
	}
	for _, t := range m.Transitions {
		if e, ok := communication(t); ok {
			o.line("Ready_%s == Pre_%s /\\ %s", t.ID, t.ID, o.ready(t, e))
		}
	}
	for _, t := range m.Transitions {
		if hasDefault(t.Guard) {
			o.line("Pre_%s == Running /\\ pc[%s] = %s /\\ %s", t.ID, quote(t.Process), quote(t.Source), o.guard(t.Guard, t))
		}
	}
	for _, t := range m.Transitions {
		o.register(t)
		o.single(t)
	}
	for _, a := range m.Transitions {
		ae, ok := communication(a)
		if !ok || ae.Kind != behavior.Send || ae.Resource == "nil" || o.caps[ae.Resource] != 0 {
			continue
		}
		for _, c := range m.Transitions {
			ce, ok := communication(c)
			if !ok || ce.Kind != behavior.Receive || ce.Resource != ae.Resource || a.Process == c.Process {
				continue
			}
			name := rendezvousName(a, c)
			o.line("\n%s ==", name)
			o.line("    /\\ Pre_%s /\\ Pre_%s /\\ ~closed[%s]", a.ID, c.ID, quote(ae.Resource))
			o.line("    /\\ (waiting[%s] \\/ waiting[%s])", quote(a.Process), quote(c.Process))
			o.updates([]behavior.Transition{a, c}, map[string]string{})
			o.actions = append(o.actions, name)
		}
	}
	o.line("\nTerminated == pc[%s] = %s /\\ UNCHANGED vars", quote(m.InitialState.Main), quote(mainTerminal))
	o.actions = append(o.actions, "Terminated")
	o.line("Next ==\n    \\/ %s", strings.Join(o.actions, "\n    \\/ "))
	o.line("Spec == Init /\\ [][Next]_vars")
	o.line("====")
	return o.text.String(), "SPECIFICATION Spec\nINVARIANT NoSynchronizationErrors\nCHECK_DEADLOCK TRUE\n", nil
}
func (o *output) guard(g behavior.Guard, t behavior.Transition) string {
	s := "TRUE"
	switch g.Kind {
	case behavior.True, behavior.Choice:
	case behavior.Equal:
		s = fmt.Sprintf("local[%s] = %d", quote(g.Variable), g.Value)
	case behavior.All:
		parts := []string{}
		for _, a := range g.Terms {
			parts = append(parts, o.guard(a, t))
		}
		s = conj(parts)
	case behavior.Default:
		parts := []string{}
		for _, c := range o.m.Transitions {
			if c.Process == t.Process && c.Source == t.Source && c.ChoiceGroup == g.Variable {
				if _, ok := communication(c); ok {
					parts = append(parts, "Ready_"+c.ID)
				}
			}
		}
		s = "~" + disj(parts)
	}
	if g.Negated {
		return "~(" + s + ")"
	}
	return s
}
func (o *output) ready(t behavior.Transition, e behavior.Effect) string {
	if e.Resource == "nil" {
		return "FALSE"
	}
	c := quote(e.Resource)
	cap := o.caps[e.Resource]
	parts := []string{"closed[" + c + "]"}
	if cap > 0 {
		if e.Kind == behavior.Send {
			parts = append(parts, fmt.Sprintf("Len(queues[%s]) < %d", c, cap))
		} else {
			parts = append(parts, "Len(queues["+c+"]) > 0")
		}
	} else {
		for _, other := range o.m.Transitions {
			oe, ok := communication(other)
			if ok && oe.Kind != e.Kind && oe.Resource == e.Resource && other.Process != t.Process {
				parts = append(parts, "(waiting["+quote(other.Process)+"] /\\ Pre_"+other.ID+")")
			}
		}
	}
	return disj(parts)
}
func (o *output) single(t behavior.Transition) {
	guards := []string{"Pre_" + t.ID}
	changes := map[string]string{}
	fault := "FALSE"
	for _, e := range t.Effects {
		r := quote(e.Resource)
		switch e.Kind {
		case behavior.Send:
			if e.Resource == "nil" {
				return
			}
			if o.caps[e.Resource] == 0 {
				guards = append(guards, "closed["+r+"]")
				fault = "TRUE"
			} else {
				guards = append(guards, fmt.Sprintf("(closed[%s] \\/ Len(queues[%s]) < %d)", r, r, o.caps[e.Resource]))
				fault = "closed[" + r + "]"
				changes["queues"] = fmt.Sprintf("IF closed[%s] THEN queues ELSE [queues EXCEPT ![%s] = Append(@, 0)]", r, r)
			}
		case behavior.Receive:
			if e.Resource == "nil" {
				return
			}
			if o.caps[e.Resource] == 0 {
				guards = append(guards, "closed["+r+"]")
			} else {
				guards = append(guards, fmt.Sprintf("(closed[%s] \\/ Len(queues[%s]) > 0)", r, r))
				changes["queues"] = fmt.Sprintf("IF Len(queues[%s]) > 0 THEN [queues EXCEPT ![%s] = Tail(@)] ELSE queues", r, r)
			}
		case behavior.CloseChannel:
			if e.Resource == "nil" {
				fault = "TRUE"
			} else {
				fault = "closed[" + r + "]"
				changes["closed"] = "[closed EXCEPT ![" + r + "] = TRUE]"
			}
		case behavior.Lock:
			guards = append(guards, "~locks["+r+"]")
			changes["locks"] = "[locks EXCEPT ![" + r + "] = TRUE]"
		case behavior.Unlock:
			fault = "~locks[" + r + "]"
			changes["locks"] = "[locks EXCEPT ![" + r + "] = FALSE]"
		case behavior.WaitGroupAdd, behavior.WaitGroupDone:
			d := e.Value
			if e.Kind == behavior.WaitGroupDone {
				d = -1
			}
			fault = fmt.Sprintf("wg[%s] + (%d) < 0", r, d)
			changes["wg"] = fmt.Sprintf("[wg EXCEPT ![%s] = @ + (%d)]", r, d)
		case behavior.WaitGroupWait:
			guards = append(guards, "wg["+r+"] = 0")
		case behavior.Spawn:
			guards = append(guards, "pc["+quote(e.Process)+"] = \"Dormant\"")
		case behavior.AssignAbstractState:
		case behavior.Assert:
			if e.Value == 0 {
				fault = "TRUE"
			}
		}
	}
	changes["fault"] = fault
	o.line("\n%s ==", stepName(t))
	o.line("    /\\ %s", conj(guards))
	o.updates([]behavior.Transition{t}, changes)
	o.actions = append(o.actions, stepName(t))
}
func (o *output) updates(ts []behavior.Transition, changes map[string]string) {
	pc := []string{}
	waiting := []string{}
	local := []string{}
	for _, t := range ts {
		waiting = append(waiting, fmt.Sprintf("![%s] = FALSE", quote(t.Process)))
		pc = append(pc, fmt.Sprintf("![%s] = %s", quote(t.Process), quote(t.Destination)))
		for _, e := range t.Effects {
			if e.Kind == behavior.Spawn {
				for _, p := range o.m.Processes {
					if p.ID == e.Process {
						pc = append(pc, fmt.Sprintf("![%s] = %s", quote(p.ID), quote(p.Entry)))
					}
				}
			}
			if e.Kind == behavior.AssignAbstractState {
				local = append(local, fmt.Sprintf("![%s] = %d", quote(e.Variable), e.Value))
			}
			if e.Kind == behavior.Receive && e.Variable != "" {
				status := "0"
				if len(ts) == 2 {
					status = "1"
				} else if o.caps[e.Resource] > 0 {
					status = fmt.Sprintf("IF Len(queues[%s]) > 0 THEN 1 ELSE 0", quote(e.Resource))
				}
				local = append(local, fmt.Sprintf("![%s] = %s", quote(e.Variable), status))
			}
		}
	}
	changes["pc"] = "[pc EXCEPT " + strings.Join(pc, ", ") + "]"
	changes["waiting"] = "[waiting EXCEPT " + strings.Join(waiting, ", ") + "]"
	if len(local) > 0 {
		changes["local"] = "[local EXCEPT " + strings.Join(local, ", ") + "]"
	}
	unchanged := []string{}
	for _, v := range []string{"pc", "queues", "closed", "locks", "wg", "local", "fault", "waiting"} {
		if x, ok := changes[v]; ok {
			o.line("    /\\ %s' = (%s)", v, x)
		} else {
			unchanged = append(unchanged, v)
		}
	}
	if len(unchanged) > 0 {
		o.line("    /\\ UNCHANGED <<%s>>", strings.Join(unchanged, ", "))
	}
}

func hasDefault(g behavior.Guard) bool {
	if g.Kind == behavior.Default {
		return true
	}
	for _, x := range g.Terms {
		if hasDefault(x) {
			return true
		}
	}
	return false
}

// register separates being poised at an operation from having executed it and
// blocked. In particular, a default select cannot observe an unscheduled peer.
// A blocking select registers all its alternatives atomically, not one chosen case.
func (o *output) register(t behavior.Transition) {
	if _, ok := communication(t); !ok {
		return
	}
	alternatives := []string{}
	for _, other := range o.m.Transitions {
		if other.Process != t.Process || other.Source != t.Source {
			continue
		}
		same := other.ID == t.ID || (t.ChoiceGroup != "" && other.ChoiceGroup == t.ChoiceGroup)
		if !same {
			continue
		}
		if hasDefault(other.Guard) {
			return
		} // Default selects never wait.
		if _, ok := communication(other); ok {
			alternatives = append(alternatives, "Ready_"+other.ID)
		}
	}
	name := "Register_" + t.ID
	o.line("\n%s ==", name)
	o.line("    /\\ Pre_%s /\\ ~waiting[%s] /\\ ~%s", t.ID, quote(t.Process), disj(alternatives))
	o.line("    /\\ waiting' = [waiting EXCEPT ![%s] = TRUE]", quote(t.Process))
	o.line("    /\\ UNCHANGED <<pc, queues, closed, locks, wg, local, fault>>")
	o.actions = append(o.actions, name)
}
