package tests

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/tla"
)

// An independent IR producer substitutes an Exit instruction for a marker close.
// Actual os.Exit source recognition/control is tested separately in lowering;
// importing os must not bypass unsupported dependency initialization here.
func exitIR(t *testing.T, body string) *behavior.Model {
	t.Helper()
	m := fromSource(t, "package main;func stop(c chan int){close(c)};func main(){"+body+"}")
	if m.HasErrors() {
		t.Fatalf("invalid IR fixture: %+v", m.Diagnostics)
	}
	exits := 0
	for i, transition := range m.Transitions {
		if !strings.Contains(transition.SourcePosition.Function, "stop") {
			continue
		}
		for _, e := range transition.Effects {
			if e.Kind == behavior.CloseChannel {
				m.Transitions[i].Effects = []behavior.Effect{{Kind: behavior.Exit}}
				for _, p := range m.Processes {
					if p.ID == transition.Process {
						m.Transitions[i].Destination = p.Terminal
					}
				}
				exits++
			}
		}
	}
	if exits != 1 {
		t.Fatalf("marker count=%d", exits)
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := behavior.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func TestTLCProgramExitIR(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"main", `marker:=make(chan int);blocked:=make(chan int);stop(marker);<-blocked`, "No error has been found"},
		{"worker", `marker:=make(chan int);blocked:=make(chan int);go func(){stop(marker)}();<-blocked`, "No error has been found"},
		{"cleanup-skipped", `marker:=make(chan int);blocked:=make(chan int);defer func(){<-blocked}();stop(marker)`, "No error has been found"},
		{"deferred-helper", `marker:=make(chan int);blocked:=make(chan int);defer func(){<-blocked}();defer stop(marker)`, "No error has been found"},
		{"prior-fault", `marker:=make(chan int);c:=make(chan int);close(c);close(c);stop(marker)`, "Invariant NoSynchronizationErrors is violated"},
		{"competing-fault", `marker:=make(chan int);c:=make(chan int);go func(){stop(marker)}();close(c);close(c)`, "Invariant NoSynchronizationErrors is violated"},
		{"blocked-before-exit", `marker:=make(chan int);blocked:=make(chan int);<-blocked;stop(marker)`, "Deadlock reached"},
	} {
		t.Run(tc.name, func(t *testing.T) { checkTLC(t, exitIR(t, tc.body), tc.want) })
	}
}
func TestTLCProgramExitMutation(t *testing.T) {
	m := exitIR(t, `marker:=make(chan int);blocked:=make(chan int);go func(){stop(marker)}();<-blocked`)
	for i, tr := range m.Transitions {
		for _, e := range tr.Effects {
			if e.Kind == behavior.Exit {
				m.Transitions[i].Effects = nil
			}
		}
	}
	checkTLC(t, m, "Deadlock reached")
}
func TestProgramExitIRContract(t *testing.T) {
	for _, broken := range []string{"continuation", "compound", "resource", "status"} {
		t.Run(broken, func(t *testing.T) {
			m := exitIR(t, `marker:=make(chan int);stop(marker)`)
			for i, tr := range m.Transitions {
				for j, e := range tr.Effects {
					if e.Kind != behavior.Exit {
						continue
					}
					switch broken {
					case "continuation":
						m.Transitions[i].Destination = tr.Source
					case "compound":
						m.Transitions[i].Effects = append(m.Transitions[i].Effects, behavior.Effect{Kind: behavior.Assert, Value: 1})
					case "resource":
						m.Transitions[i].Effects[j].Resource = m.Channels[0].ID
					case "status":
						m.Transitions[i].Effects[j].Value = 3
					}
				}
			}
			if err := behavior.Validate(m); err == nil {
				t.Fatal("invalid Exit accepted")
			}
			if _, _, err := tla.Generate(m); err == nil {
				t.Fatal("invalid Exit executable")
			}
		})
	}
}
