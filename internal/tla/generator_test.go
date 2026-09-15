package tla_test

import (
	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/tla"
	"strings"
	"testing"
)

func minimal() *behavior.Model {
	return &behavior.Model{SchemaVersion: behavior.SchemaVersion, Semantics: behavior.CommunicationSemantics, Termination: behavior.MainReturn,
		Name: "model", Outcome: "precisely-modeled", Assertions: []behavior.Assertion{{Kind: "NoSynchronizationErrors"}},
		Processes:    []behavior.Process{{ID: "main", Entry: "entry", Terminal: "main_Done", Locations: []string{"entry", "main_Done"}}},
		InitialState: behavior.InitialState{Main: "main", Active: []string{"main"}},
		Transitions:  []behavior.Transition{{ID: "Finish", Process: "main", Source: "entry", Destination: "main_Done", Guard: behavior.Guard{Kind: behavior.True}}}}
}
func TestGenerateBackendIndependentModel(t *testing.T) {
	spec, cfg, err := tla.Generate(minimal())
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"---- MODULE model ----", "Init ==", "Next ==", "Spec ==", "UNCHANGED vars"} {
		if !strings.Contains(spec, s) {
			t.Errorf("missing %s", s)
		}
	}
	if !strings.Contains(cfg, "CHECK_DEADLOCK TRUE") {
		t.Fatal("deadlock checking disabled")
	}
}
func TestExplicitTerminalAndInitialActivation(t *testing.T) {
	m := minimal()
	m.Processes[0].Terminal = "exit"
	m.Processes[0].Locations[1] = "exit"
	m.Transitions[0].Destination = "exit"
	m.Processes = append(m.Processes, behavior.Process{ID: "worker", Entry: "worker_entry", Terminal: "worker_exit", Locations: []string{"worker_entry", "worker_exit"}})
	m.InitialState.Active = append(m.InitialState.Active, "worker")
	m.Transitions = append(m.Transitions, behavior.Transition{ID: "WorkerFinish", Process: "worker", Source: "worker_entry", Destination: "worker_exit", Guard: behavior.Guard{Kind: behavior.True}})
	spec, _, err := tla.Generate(m)
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{`p = "worker" -> "worker_entry"`, `Terminated == pc["main"] = "exit"`, `Step_Finish ==`} {
		if !strings.Contains(spec, text) {
			t.Fatalf("ignored explicit IR field: %s", text)
		}
	}
	if strings.Contains(spec, "main_Done") {
		t.Fatal("backend inferred a magic terminal name")
	}
}

func TestBackendRestrictionsRemainSeparate(t *testing.T) {
	for name, mutate := range map[string]func(*behavior.Model){
		"shared-state": func(m *behavior.Model) { m.SharedState = []behavior.Variable{{Name: "shared", Domain: []int{0}}} },
		"cycle":        func(m *behavior.Model) { m.Transitions[0].Destination = "entry" },
		"runtime-sentinel": func(m *behavior.Model) {
			m.Processes[0].Entry = "Dormant"
			m.Processes[0].Locations[0] = "Dormant"
			m.Transitions[0].Source = "Dormant"
		},
		"backend-name":     func(m *behavior.Model) { m.Transitions[0].ID = "with-dash" },
		"missing-property": func(m *behavior.Model) { m.Assertions = nil },
		"concurrent-enrollment": func(m *behavior.Model) {
			m.Processes = append(m.Processes, behavior.Process{ID: "worker", Entry: "start", Terminal: "end", Locations: []string{"start", "end"}})
			m.InitialState.Active = append(m.InitialState.Active, "worker")
			m.WaitGroups = []string{"wg"}
			m.Transitions[0].Effects = []behavior.Effect{{Kind: behavior.WaitGroupAdd, Resource: "wg", Value: 1}}
		},
	} {
		t.Run(name, func(t *testing.T) {
			m := minimal()
			mutate(m)
			if err := behavior.Validate(m); err != nil {
				t.Fatalf("generic IR rejected backend capability case: %v", err)
			}
			if _, _, err := tla.Generate(m); err == nil {
				t.Fatal("unsupported backend capability silently accepted")
			}
		})
	}
}

func TestAmbiguousBlockingAlternativesRejected(t *testing.T) {
	m := minimal()
	m.Channels = []behavior.Channel{{ID: "ch", Capacity: 0}}
	send := m.Transitions[0]
	send.ID = "Send"
	send.Effects = []behavior.Effect{{Kind: behavior.Send, Resource: "ch"}}
	m.Transitions = append(m.Transitions, send)
	if err := behavior.Validate(m); err != nil {
		t.Fatal(err)
	}
	if _, _, err := tla.Generate(m); err == nil {
		t.Fatal("ready alternatives hid a possible committed block")
	}
	m.Processes[0].Locals = []behavior.Variable{{Name: "index", Domain: []int{0, 1}}}
	m.Transitions[0].Guard = behavior.Guard{Kind: behavior.Equal, Variable: "index", Value: 0}
	m.Transitions[1].Guard = behavior.Guard{Kind: behavior.Equal, Variable: "index", Value: 1}
	if _, _, err := tla.Generate(m); err != nil {
		t.Fatal("disjoint exact dispatch rejected:", err)
	}
}

func TestSourceCommentsCannotInjectDefinitions(t *testing.T) {
	m := minimal()
	m.Transitions[0].SourcePosition.File = "file.go\nInjected == TRUE\r"
	m.Assumptions = []string{"assumption\nOther == FALSE"}
	spec, _, err := tla.Generate(m)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(spec, "\nInjected") || strings.Contains(spec, "\nOther") {
		t.Fatal("metadata escaped a comment")
	}
}

func TestRejectUnknownIREffect(t *testing.T) {
	m := minimal()
	m.Transitions[0].Effects = []behavior.Effect{{Kind: "Unknown"}}
	if _, _, err := tla.Generate(m); err == nil {
		t.Fatal("silently ignored unknown effect")
	}
}
func TestRejectMultipleSchedulingEffects(t *testing.T) {
	m := minimal()
	m.Channels = []behavior.Channel{{ID: "c", Capacity: 1}}
	m.Transitions[0].Effects = []behavior.Effect{{Kind: behavior.Send, Resource: "c"}, {Kind: behavior.Send, Resource: "c"}}
	if _, _, err := tla.Generate(m); err == nil {
		t.Fatal("silently merged scheduling points")
	}
}
