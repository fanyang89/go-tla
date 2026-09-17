package behavior

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"

	"github.com/fanmi/go-tla/internal/diagnostic"
)

// Decode reads exactly one versioned executable model, rejecting unknown JSON
// fields. Diagnostic-only partial artifacts may be read as JSON but not executed.
func Decode(r io.Reader) (*Model, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if err := checkJSONShape(json.NewDecoder(bytes.NewReader(data)), reflect.TypeFor[Model](), 0); err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	var m Model
	if err := d.Decode(&m); err != nil {
		return nil, err
	}
	var extra any
	if err := d.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("expected exactly one model JSON value")
	}
	if err := Validate(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

// Require exact field spellings and unique keys. encoding/json alone accepts
// case-insensitive field aliases and duplicate keys, unlike some other IR readers.
func checkJSONShape(d *json.Decoder, typ reflect.Type, depth int) error {
	if depth > 256 {
		return fmt.Errorf("JSON nesting exceeds 256 levels")
	}
	token, err := d.Token()
	if err != nil {
		return err
	}
	switch token {
	case json.Delim('{'):
		fields := map[string]reflect.Type{}
		if typ.Kind() == reflect.Struct {
			for i := range typ.NumField() {
				field := typ.Field(i)
				name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
				fields[name] = field.Type
			}
		} else if typ.Kind() != reflect.Map {
			return fmt.Errorf("unexpected JSON object for %v", typ)
		}
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return fmt.Errorf("duplicate/invalid JSON object key %q", key)
			}
			seen[name] = true
			child := fields[name]
			if typ.Kind() == reflect.Map {
				child = typ.Elem()
			}
			if child == nil {
				return fmt.Errorf("unknown JSON field %q in %v", name, typ)
			}
			if err := checkJSONShape(d, child, depth+1); err != nil {
				return err
			}
		}
		_, err = d.Token()
	case json.Delim('['):
		if typ.Kind() != reflect.Slice {
			return fmt.Errorf("unexpected JSON array for %v", typ)
		}
		for d.More() {
			if err := checkJSONShape(d, typ.Elem(), depth+1); err != nil {
				return err
			}
		}
		_, err = d.Token()
	}
	return err
}

type variableInfo struct {
	owner string
	value Variable
}

// Validate checks the backend-independent structural and communication-v1
// contract. It does not prove reachability, finite counters, or Go alias safety.
// Backend-specific capability restrictions must be checked separately.
func Validate(m *Model) error {
	if m == nil {
		return fmt.Errorf("nil model")
	}
	if m.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported IR schema version %d (require %d; unversioned models must be regenerated)", m.SchemaVersion, SchemaVersion)
	}
	if m.Semantics != CommunicationSemantics || m.Termination != MainReturn {
		return fmt.Errorf("unsupported semantics or termination policy")
	}
	if m.Name == "" || (m.Outcome != diagnostic.Precise && m.Outcome != diagnostic.Abstracted) || m.HasErrors() {
		return fmt.Errorf("model must be named and successfully extracted without error diagnostics")
	}
	if len(m.Processes) == 0 {
		return fmt.Errorf("model has no processes")
	}
	for _, d := range m.Diagnostics {
		if d.Severity != "info" && d.Severity != "warning" {
			return fmt.Errorf("unknown diagnostic severity %q", d.Severity)
		}
		if d.Severity == "warning" && m.Outcome != diagnostic.Abstracted {
			return fmt.Errorf("warning requires conservatively-abstracted outcome")
		}
	}
	if len(m.AbstractedPredicates) > 0 && m.Outcome != diagnostic.Abstracted {
		return fmt.Errorf("abstract predicates require conservatively-abstracted outcome")
	}
	resources := map[string]string{"nil": "channel"}
	addResource := func(id, kind string) error {
		if id == "" || resources[id] != "" {
			return fmt.Errorf("empty, duplicate, or reserved resource %q", id)
		}
		resources[id] = kind
		return nil
	}
	for _, c := range m.Channels {
		if c.Capacity < 0 {
			return fmt.Errorf("channel %q has negative capacity", c.ID)
		}
		if err := addResource(c.ID, "channel"); err != nil {
			return err
		}
	}
	for _, pair := range []struct {
		kind string
		ids  []string
	}{{"mutex", m.Mutexes}, {"waitgroup", m.WaitGroups}} {
		for _, id := range pair.ids {
			if err := addResource(id, pair.kind); err != nil {
				return err
			}
		}
	}
	processes := map[string]Process{}
	locations := map[string]map[string]bool{}
	variables := map[string]variableInfo{}
	addVariable := func(v Variable, owner string) error {
		if _, exists := variables[v.Name]; exists || v.Name == "" || len(v.Domain) == 0 {
			return fmt.Errorf("empty/duplicate variable or empty domain: %q", v.Name)
		}
		seen := map[int]bool{}
		for _, value := range v.Domain {
			if seen[value] {
				return fmt.Errorf("variable %q has duplicate domain value", v.Name)
			}
			seen[value] = true
		}
		if !seen[v.Initial] {
			return fmt.Errorf("variable %q initial value is outside its domain", v.Name)
		}
		variables[v.Name] = variableInfo{owner, v}
		return nil
	}
	for _, v := range m.SharedState {
		if err := addVariable(v, ""); err != nil {
			return err
		}
	}
	for _, p := range m.Processes {
		if _, exists := processes[p.ID]; exists || p.ID == "" {
			return fmt.Errorf("duplicate/empty process %q", p.ID)
		}
		processes[p.ID] = p
		locs := map[string]bool{}
		for _, loc := range p.Locations {
			if loc == "" || locs[loc] {
				return fmt.Errorf("process %q has empty/duplicate location %q", p.ID, loc)
			}
			locs[loc] = true
		}
		if !locs[p.Entry] || !locs[p.Terminal] {
			return fmt.Errorf("process %q entry/terminal must name declared locations", p.ID)
		}
		locations[p.ID] = locs
		for _, v := range p.Locals {
			if err := addVariable(v, p.ID); err != nil {
				return err
			}
		}
	}
	active := map[string]bool{}
	for _, id := range m.InitialState.Active {
		if _, ok := processes[id]; !ok || active[id] {
			return fmt.Errorf("unknown/duplicate initially active process %q", id)
		}
		active[id] = true
	}
	if !active[m.InitialState.Main] {
		return fmt.Errorf("main process must exist and be initially active")
	}
	ids := map[string]bool{}
	for _, t := range m.Transitions {
		p, ok := processes[t.Process]
		if !ok || !locations[t.Process][t.Source] || !locations[t.Process][t.Destination] {
			return fmt.Errorf("transition %q references an unknown process/location", t.ID)
		}
		if t.Source == p.Terminal {
			return fmt.Errorf("transition %q leaves a terminal location", t.ID)
		}
		if t.ID == "" || ids[t.ID] {
			return fmt.Errorf("duplicate/empty transition %q", t.ID)
		}
		ids[t.ID] = true
		defaults := 0
		if err := validateGuard(t.Guard, t, variables, false, &defaults, 0); err != nil {
			return fmt.Errorf("transition %q: %w", t.ID, err)
		}
		if defaults > 1 {
			return fmt.Errorf("transition %q has multiple default guards", t.ID)
		}
		writes := map[string]bool{}
		communications := 0
		for _, e := range t.Effects {
			if defaults > 0 && e.Kind != AssignAbstractState {
				return fmt.Errorf("default selection must precede its body effects")
			}
			if t.ChoiceGroup != "" && e.Kind != AssignAbstractState && e.Kind != Send && e.Kind != Receive {
				return fmt.Errorf("select communication must precede its body effects")
			}
			resource, process, variable, value := false, false, false, false
			kind := ""
			switch e.Kind {
			case Exit:
				if len(t.Effects) != 1 || t.Destination != p.Terminal {
					return fmt.Errorf("Exit must be standalone and target its process terminal")
				}
			case Send, Receive, CloseChannel:
				kind, resource = "channel", true
				if e.Kind == Receive && e.Variable != "" {
					variable = true
					v, ok := variables[e.Variable]
					if !ok || v.owner != t.Process || len(v.value.Domain) != 2 || !slices.Contains(v.value.Domain, 0) || !slices.Contains(v.value.Domain, 1) || writes[e.Variable] {
						return fmt.Errorf("transition %q has invalid/duplicate receive status destination %q", t.ID, e.Variable)
					}
					writes[e.Variable] = true
				}
				if e.Kind != CloseChannel {
					communications++
				}
			case Lock, Unlock:
				kind, resource = "mutex", true
			case WaitGroupAdd:
				kind, resource, value = "waitgroup", true, true
			case WaitGroupDone, WaitGroupWait:
				kind, resource = "waitgroup", true
			case Spawn:
				process = true
				if _, ok := processes[e.Process]; !ok {
					return fmt.Errorf("transition %q spawns unknown process %q", t.ID, e.Process)
				}
			case AssignAbstractState:
				variable, value = true, true
				v, ok := variables[e.Variable]
				if !ok || (v.owner != "" && v.owner != t.Process) || !slices.Contains(v.value.Domain, e.Value) || writes[e.Variable] {
					return fmt.Errorf("transition %q has invalid/duplicate variable assignment %q", t.ID, e.Variable)
				}
				writes[e.Variable] = true
			case Assert:
				value = true
				if e.Value != 0 && e.Value != 1 {
					return fmt.Errorf("Assert requires boolean 0 or 1")
				}
			default:
				return fmt.Errorf("unsupported effect %q", e.Kind)
			}
			if resource && resources[e.Resource] != kind {
				return fmt.Errorf("transition %q: invalid %s resource %q", t.ID, kind, e.Resource)
			}
			if (!resource && e.Resource != "") || (!process && e.Process != "") || (!variable && e.Variable != "") || (!value && e.Value != 0) {
				return fmt.Errorf("transition %q: unused fields on effect %s", t.ID, e.Kind)
			}
		}
		if t.ChoiceGroup != "" && defaults == 0 && communications != 1 {
			return fmt.Errorf("select group member must communicate or be default")
		}
	}
	// A default belongs to exactly one select at this process/location. Duplicate
	// defaults are malformed; backend capabilities further constrain competitors.
	for _, t := range m.Transitions {
		if t.ChoiceGroup == "" {
			continue
		}
		defaults := 0
		for _, other := range m.Transitions {
			if other.Process != t.Process || other.Source != t.Source || other.ChoiceGroup != t.ChoiceGroup {
				continue
			}
			if containsDefault(other.Guard) {
				defaults++
			}
		}
		if defaults > 1 {
			return fmt.Errorf("select group %q has duplicate defaults at %q", t.ChoiceGroup, t.Source)
		}
	}
	assertions := map[string]bool{}
	for _, a := range m.Assertions {
		if a.Kind != "NoSynchronizationErrors" || assertions[a.Kind] {
			return fmt.Errorf("unknown/duplicate assertion %q", a.Kind)
		}
		assertions[a.Kind] = true
	}
	return nil
}

func validateGuard(g Guard, t Transition, vars map[string]variableInfo, negative bool, defaults *int, depth int) error {
	if depth > 64 {
		return fmt.Errorf("guard nesting exceeds 64 levels")
	}
	negative = negative || g.Negated
	switch g.Kind {
	case True:
		if g.Variable != "" || g.Value != 0 || len(g.Terms) != 0 {
			return fmt.Errorf("unused fields on true guard")
		}
	case Choice:
		if negative || g.Variable != "" || g.Value != 0 || len(g.Terms) != 0 {
			return fmt.Errorf("choice is a positive nondeterministic alternative marker, not a negatable boolean")
		}
	case Equal:
		v, ok := vars[g.Variable]
		if !ok || (v.owner != "" && v.owner != t.Process) || len(g.Terms) != 0 {
			return fmt.Errorf("unknown/foreign guard variable %q or unused terms", g.Variable)
		}
	case All:
		if g.Variable != "" || g.Value != 0 {
			return fmt.Errorf("unused fields on conjunction")
		}
		for _, term := range g.Terms {
			if err := validateGuard(term, t, vars, negative, defaults, depth+1); err != nil {
				return err
			}
		}
	case Default:
		if negative || g.Variable == "" || g.Variable != t.ChoiceGroup || g.Value != 0 || len(g.Terms) != 0 {
			return fmt.Errorf("default must positively reference its select group")
		}
		*defaults++
	default:
		return fmt.Errorf("unsupported guard %q", g.Kind)
	}
	return nil
}

func containsDefault(g Guard) bool {
	if g.Kind == Default {
		return true
	}
	for _, t := range g.Terms {
		if containsDefault(t) {
			return true
		}
	}
	return false
}
