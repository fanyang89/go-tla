package behavior

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/fanmi/go-tla/internal/diagnostic"
)

func validModel() *Model {
	return &Model{SchemaVersion: SchemaVersion, Semantics: CommunicationSemantics, Termination: MainReturn, Name: "fixture", Outcome: diagnostic.Precise,
		Processes: []Process{{ID: "main", Entry: "entry", Terminal: "done", Locations: []string{"entry", "done"}, Locals: []Variable{{Name: "v", Domain: []int{0, 1}, Initial: 0}}}},
		Channels:  []Channel{{ID: "ch", Capacity: 1}}, Mutexes: []string{"mu"}, WaitGroups: []string{"wg"},
		InitialState: InitialState{Main: "main", Active: []string{"main"}},
		Transitions:  []Transition{{ID: "finish", Process: "main", Source: "entry", Destination: "done", Guard: Guard{Kind: True}}},
		Assertions:   []Assertion{{Kind: "NoSynchronizationErrors"}}}
}

func TestValidateMalformedIR(t *testing.T) {
	cases := map[string]func(*Model){
		"unversioned":            func(m *Model) { m.SchemaVersion = 0 },
		"future-version":         func(m *Model) { m.SchemaVersion++ },
		"unknown-semantics":      func(m *Model) { m.Semantics = "other" },
		"unknown-termination":    func(m *Model) { m.Termination = "all-processes" },
		"unsupported":            func(m *Model) { m.Outcome = diagnostic.Unsupported },
		"unknown-outcome":        func(m *Model) { m.Outcome = "unknown" },
		"hidden-error":           func(m *Model) { m.Diagnostics = []diagnostic.Diagnostic{{Severity: "error"}} },
		"hidden-warning":         func(m *Model) { m.Diagnostics = []diagnostic.Diagnostic{{Severity: "warning"}} },
		"hidden-abstraction":     func(m *Model) { m.AbstractedPredicates = []Predicate{{Name: "unknown"}} },
		"duplicate-process":      func(m *Model) { m.Processes = append(m.Processes, m.Processes[0]) },
		"empty-process":          func(m *Model) { m.Processes[0].ID = "" },
		"missing-entry":          func(m *Model) { m.Processes[0].Entry = "missing" },
		"missing-terminal":       func(m *Model) { m.Processes[0].Terminal = "missing" },
		"duplicate-location":     func(m *Model) { m.Processes[0].Locations = append(m.Processes[0].Locations, "entry") },
		"unknown-source":         func(m *Model) { m.Transitions[0].Source = "missing" },
		"unknown-destination":    func(m *Model) { m.Transitions[0].Destination = "missing" },
		"unknown-process":        func(m *Model) { m.Transitions[0].Process = "other" },
		"terminal-outgoing":      func(m *Model) { m.Transitions[0].Source = "done" },
		"inactive-main":          func(m *Model) { m.InitialState.Active = nil },
		"unknown-active":         func(m *Model) { m.InitialState.Active = append(m.InitialState.Active, "unknown") },
		"duplicate-active":       func(m *Model) { m.InitialState.Active = append(m.InitialState.Active, "main") },
		"duplicate-transition":   func(m *Model) { m.Transitions = append(m.Transitions, m.Transitions[0]) },
		"empty-transition":       func(m *Model) { m.Transitions[0].ID = "" },
		"negative-capacity":      func(m *Model) { m.Channels[0].Capacity = -1 },
		"reserved-channel":       func(m *Model) { m.Channels[0].ID = "nil" },
		"duplicate-resource":     func(m *Model) { m.Mutexes = append(m.Mutexes, "ch") },
		"empty-domain":           func(m *Model) { m.Processes[0].Locals[0].Domain = nil },
		"duplicate-domain":       func(m *Model) { m.Processes[0].Locals[0].Domain = []int{0, 0} },
		"initial-outside-domain": func(m *Model) { m.Processes[0].Locals[0].Initial = 2 },
		"duplicate-variable":     func(m *Model) { m.SharedState = []Variable{{Name: "v", Domain: []int{0}, Initial: 0}} },
		"unknown-guard":          func(m *Model) { m.Transitions[0].Guard.Kind = "future" },
		"unknown-variable":       func(m *Model) { m.Transitions[0].Guard = Guard{Kind: Equal, Variable: "other"} },
		"ignored-guard-data":     func(m *Model) { m.Transitions[0].Guard.Value = 1 },
		"negated-choice":         func(m *Model) { m.Transitions[0].Guard = Guard{Kind: Choice, Negated: true} },
		"negated-nested-choice": func(m *Model) {
			m.Transitions[0].Guard = Guard{Kind: All, Negated: true, Terms: []Guard{{Kind: Choice}}}
		},
		"unknown-effect":      func(m *Model) { m.Transitions[0].Effects = []Effect{{Kind: "future"}} },
		"wrong-resource-kind": func(m *Model) { m.Transitions[0].Effects = []Effect{{Kind: Lock, Resource: "ch"}} },
		"missing-resource":    func(m *Model) { m.Transitions[0].Effects = []Effect{{Kind: Send, Resource: "missing"}} },
		"unknown-spawn":       func(m *Model) { m.Transitions[0].Effects = []Effect{{Kind: Spawn, Process: "missing"}} },
		"ignored-effect-data": func(m *Model) { m.Transitions[0].Effects = []Effect{{Kind: Send, Resource: "ch", Value: 1}} },
		"assignment-outside-domain": func(m *Model) {
			m.Transitions[0].Effects = []Effect{{Kind: AssignAbstractState, Variable: "v", Value: 2}}
		},
		"duplicate-assignment": func(m *Model) {
			m.Transitions[0].Effects = []Effect{{Kind: AssignAbstractState, Variable: "v"}, {Kind: AssignAbstractState, Variable: "v", Value: 1}}
		},
		"invalid-assertion-value": func(m *Model) { m.Transitions[0].Effects = []Effect{{Kind: Assert, Value: 2}} },
		"default-without-group":   func(m *Model) { m.Transitions[0].Guard = Guard{Kind: Default, Variable: "select"} },
		"noncommunication-group":  func(m *Model) { m.Transitions[0].ChoiceGroup = "select" },
		"duplicate-default": func(m *Model) {
			m.Transitions[0].ChoiceGroup = "select"
			m.Transitions[0].Guard = Guard{Kind: Default, Variable: "select"}
			other := m.Transitions[0]
			other.ID = "other"
			m.Transitions = append(m.Transitions, other)
		},
		"default-communication": func(m *Model) {
			m.Transitions[0].ChoiceGroup = "select"
			m.Transitions[0].Guard = Guard{Kind: Default, Variable: "select"}
			m.Transitions[0].Effects = []Effect{{Kind: Send, Resource: "ch"}}
		},
		"unknown-assertion": func(m *Model) { m.Assertions[0].Kind = "future" },
	}
	if err := Validate(nil); err == nil {
		t.Fatal("nil model accepted")
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			m := validModel()
			mutate(m)
			if err := Validate(m); err == nil {
				t.Fatal("malformed IR accepted")
			}
		})
	}
}

func TestSelectCannotBundleCaseBody(t *testing.T) {
	for _, defaultCase := range []bool{false, true} {
		m := validModel()
		tr := &m.Transitions[0]
		tr.ChoiceGroup = "select"
		if defaultCase {
			tr.Guard = Guard{Kind: Default, Variable: "select"}
		} else {
			tr.Effects = []Effect{{Kind: Receive, Resource: "ch"}}
		}
		tr.Effects = append(tr.Effects, Effect{Kind: Lock, Resource: "mu"})
		if Validate(m) == nil {
			t.Fatal("select bundled a blocking case body")
		}
	}
}

func TestVersionedIRRoundTrip(t *testing.T) {
	m := validModel()
	if err := Validate(m); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(m, got) {
		t.Fatal("round trip changed model")
	}
	for _, bad := range []string{string(data) + " {}", strings.Replace(string(data), `"schemaVersion":1`, `"schemaVersion":999,"schemaVersion":1`, 1), strings.Replace(string(data), `"schemaVersion"`, `"SchemaVersion"`, 1), strings.Replace(string(data), `"name":`, `"unknownField":true,"name":`, 1), `{"schemaVersion":999}`} {
		if _, err := Decode(strings.NewReader(bad)); err == nil {
			t.Fatal("invalid JSON contract accepted")
		}
	}
	m.Transitions[0].Guard = Guard{Kind: Equal, Variable: "v", Value: 100}
	if err := Validate(m); err != nil {
		t.Fatal("an out-of-domain equality is a valid false predicate:", err)
	}
}

func TestNestedGuardAndJSONLimits(t *testing.T) {
	m := validModel()
	g := Guard{Kind: All, Terms: make([]Guard, 1)}
	g.Terms[0] = g
	m.Transitions[0].Guard = g
	if Validate(m) == nil {
		t.Fatal("cyclic guard representation accepted")
	}
	data, err := json.Marshal(validModel())
	if err != nil {
		t.Fatal(err)
	}
	guard := strings.Repeat(`{"kind":"all","terms":[`, 150) + `{"kind":"true"}` + strings.Repeat(`]}`, 150)
	data = bytes.Replace(data, []byte(`{"kind":"true"}`), []byte(guard), 1)
	if _, err := Decode(bytes.NewReader(data)); err == nil {
		t.Fatal("excessively nested JSON accepted")
	}
}

func TestForeignLocalRejected(t *testing.T) {
	m := validModel()
	m.Processes = append(m.Processes, Process{ID: "worker", Entry: "entry", Terminal: "done", Locations: []string{"entry", "done"}, Locals: []Variable{{Name: "other", Domain: []int{0}}}})
	m.Transitions[0].Guard = Guard{Kind: Equal, Variable: "other"}
	if Validate(m) == nil {
		t.Fatal("foreign local read accepted")
	}
	m.Transitions[0].Guard = Guard{Kind: True}
	m.Transitions[0].Effects = []Effect{{Kind: AssignAbstractState, Variable: "other"}}
	if Validate(m) == nil {
		t.Fatal("foreign local write accepted")
	}
}
