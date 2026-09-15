package tla_test

import (
	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/tla"
	"strings"
	"testing"
)

func minimal() *behavior.Model {
	return &behavior.Model{Name: "model", Processes: []behavior.Process{{ID: "main", Entry: "entry", Locations: []string{"entry", "main_Done"}}}, InitialState: behavior.InitialState{Main: "main", Active: []string{"main"}}, Transitions: []behavior.Transition{{ID: "Finish", Process: "main", Source: "entry", Destination: "main_Done", Guard: behavior.Guard{Kind: behavior.True}}}}
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
