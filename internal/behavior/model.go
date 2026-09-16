// Package behavior defines a language- and backend-independent guarded transition system.
package behavior

import "github.com/fanmi/go-tla/internal/diagnostic"

type Position struct {
	Package  string `json:"package,omitempty"`
	Function string `json:"function,omitempty"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitzero"`
	Column   int    `json:"column,omitzero"`
}

const SchemaVersion = 1
const CommunicationSemantics = "communication-v1"
const MainReturn = "main-return"

// Metadata is descriptive provenance, never executable backend syntax.
type Metadata struct {
	Producer  string              `json:"producer,omitempty"`
	Version   string              `json:"version,omitempty"`
	Revision  string              `json:"revision,omitempty"`
	Modified  string              `json:"modified,omitempty"`
	Language  string              `json:"language,omitempty"`
	Toolchain string              `json:"toolchain,omitempty"`
	Options   map[string][]string `json:"options,omitempty"`
}

type Model struct {
	SchemaVersion        int                     `json:"schemaVersion"`
	Semantics            string                  `json:"semantics"`
	Termination          string                  `json:"termination"`
	Metadata             Metadata                `json:"metadata"`
	Name                 string                  `json:"name"`
	Outcome              diagnostic.Outcome      `json:"outcome"`
	Processes            []Process               `json:"processes"`
	Channels             []Channel               `json:"channels"`
	Mutexes              []string                `json:"mutexes"`
	WaitGroups           []string                `json:"waitGroups"`
	SharedState          []Variable              `json:"sharedState"`
	InitialState         InitialState            `json:"initialState"`
	Transitions          []Transition            `json:"transitions"`
	Assertions           []Assertion             `json:"assertions"`
	AbstractedPredicates []Predicate             `json:"abstractedPredicates"`
	Diagnostics          []diagnostic.Diagnostic `json:"diagnostics"`
	Assumptions          []string                `json:"assumptions"`
}

type Process struct {
	ID        string     `json:"id"`
	Kind      string     `json:"kind"`
	Entry     string     `json:"entry"`
	Terminal  string     `json:"terminal"`
	Locations []string   `json:"locations"`
	Locals    []Variable `json:"locals"`
	Source    Position   `json:"source"`
}
type Variable struct {
	Name    string `json:"name"`
	Domain  []int  `json:"domain"`
	Initial int    `json:"initial"`
}
type Channel struct {
	ID       string   `json:"id"`
	Capacity int      `json:"capacity"`
	Source   Position `json:"source"`
}
type InitialState struct {
	Main   string   `json:"main"`
	Active []string `json:"active"`
}
type Predicate struct {
	Name   string   `json:"name"`
	Source Position `json:"source"`
	Reason string   `json:"reason"`
}
type Assertion struct {
	Kind        string `json:"kind"`
	Description string `json:"description"`
}

// Guards contain no backend expression text. Choice is a positive nondeterministic
// alternative marker, not a stored or negatable boolean. Default means none of the
// communication alternatives in its process/location/group is ready.
type Guard struct {
	Kind     string  `json:"kind"`
	Variable string  `json:"variable,omitempty"`
	Value    int     `json:"value,omitzero"`
	Negated  bool    `json:"negated,omitzero"`
	Terms    []Guard `json:"terms,omitempty"`
}

const (
	True    = "true"
	Choice  = "choice"
	Equal   = "equal"
	All     = "all"
	Default = "default"
)

type EffectKind string

const (
	Spawn               EffectKind = "Spawn"
	Send                EffectKind = "Send"
	Receive             EffectKind = "Receive"
	CloseChannel        EffectKind = "CloseChannel"
	Lock                EffectKind = "Lock"
	Unlock              EffectKind = "Unlock"
	WaitGroupAdd        EffectKind = "WaitGroupAdd"
	WaitGroupDone       EffectKind = "WaitGroupDone"
	WaitGroupWait       EffectKind = "WaitGroupWait"
	AssignAbstractState EffectKind = "AssignAbstractState"
	Assert              EffectKind = "Assert"
)

// Receive may bind Variable to a process-local {0,1} status: 1 for a delivered
// value (including buffered values after close), 0 for closed-and-empty. Nil
// receives never complete. The binding is atomic with the receive itself.
type Effect struct {
	Kind     EffectKind `json:"kind"`
	Resource string     `json:"resource,omitempty"`
	Process  string     `json:"process,omitempty"`
	Variable string     `json:"variable,omitempty"`
	Value    int        `json:"value,omitzero"`
}
type Transition struct {
	ChoiceGroup    string   `json:"choiceGroup,omitempty"`
	ID             string   `json:"id"`
	Process        string   `json:"process"`
	Source         string   `json:"source"`
	Guard          Guard    `json:"guard"`
	Effects        []Effect `json:"effects"`
	Destination    string   `json:"destination"`
	SourcePosition Position `json:"sourcePosition"`
}

func (m *Model) HasErrors() bool {
	if m == nil || m.Outcome == diagnostic.Unsupported {
		return true
	}
	for _, d := range m.Diagnostics {
		if d.Severity == "error" {
			return true
		}
	}
	return false
}
