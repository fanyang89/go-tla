package tests

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/tla"
)

func TestTLCReceiveStatus(t *testing.T) {
	for _, c := range []struct{ name, source, want string }{
		{"closed-empty", `package main;func main(){ch:=make(chan int);close(ch);_,ok:=<-ch;if ok {var bad chan int;close(bad)}}`, "No error has been found"},
		{"buffered-drain", `package main;func main(){ch:=make(chan int,1);ch<-1;close(ch);_,first:=<-ch;if !first {var bad chan int;close(bad)};_,last:=<-ch;if last {var bad chan int;close(bad)}}`, "No error has been found"},
		{"status-captured", `package main;func main(){ch:=make(chan int,1);ch<-1;_,ok:=<-ch;close(ch);if ok==false {var bad chan int;close(bad)}}`, "No error has been found"},
		{"rendezvous", `package main;func main(){ch:=make(chan int);go func(){ch<-1}();_,ok:=<-ch;if true!=ok {var bad chan int;close(bad)}}`, "No error has been found"},
		{"close-wakeup", `package main;func main(){ch:=make(chan int);go func(){close(ch)}();_,ok:=<-ch;if ok {var bad chan int;close(bad)}}`, "No error has been found"},
		{"nil-blocks", `package main;func main(){var ch chan int;_,ok:=<-ch;if !ok {return}}`, "Error: Deadlock reached"},
		{"open-empty-blocks", `package main;func main(){ch:=make(chan int);_,ok:=<-ch;if !ok {return}}`, "Error: Deadlock reached"},
		{"closed-fault", `package main;func main(){ch:=make(chan int);close(ch);_,ok:=<-ch;if !ok {close(ch)}}`, "Error: Invariant NoSynchronizationErrors is violated"},
		{"select-buffered", `package main;func main(){ch:=make(chan int,1);ch<-1;close(ch);select{case _,ok:=<-ch:if !ok {var bad chan int;close(bad)};default:var bad chan int;close(bad)}}`, "No error has been found"},
		{"select-rendezvous", `package main;func main(){ch:=make(chan int);go func(){ch<-1}();select{case _,ok:=<-ch:if !ok {var bad chan int;close(bad)}}}`, "No error has been found"},
		{"select-close-wakeup", `package main;func main(){ch:=make(chan int);done:=make(chan int);go func(){select{case _,ok:=<-ch:if ok {var bad chan int;close(bad)}};done<-1}();close(ch);<-done}`, "No error has been found"},
		{"select-send", `package main;func main(){ch:=make(chan int,1);var never chan int;select{case ch<-1:case _,ok:=<-never:if !ok {close(never)}};<-ch}`, "No error has been found"},
		{"select-default", `package main;func main(){var ch chan int;select{case _,ok:=<-ch:if !ok {var bad chan int;close(bad)};default:}}`, "No error has been found"},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := fromSource(t, c.source)
			if len(m.AbstractedPredicates) != 0 {
				t.Fatalf("receive status became nondeterministic: %+v", m.AbstractedPredicates)
			}
			checkTLC(t, m, c.want)
		})
	}
}

func TestReceiveStatusIRContract(t *testing.T) {
	m := fromSource(t, `package main;func main(){ch:=make(chan int);close(ch);_,ok:=<-ch;if ok {close(ch)}}`)
	if m.HasErrors() {
		t.Fatalf("unsupported: %+v", m.Diagnostics)
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := behavior.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	before, cfg, err := tla.Generate(m)
	if err != nil {
		t.Fatal(err)
	}
	after, other, err := tla.Generate(decoded)
	if err != nil || before != after || cfg != other {
		t.Fatal("receive status round trip changed semantics")
	}
	for _, name := range []string{"unknown", "domain", "shared", "foreign", "duplicate-before", "duplicate-after", "send-binding"} {
		t.Run(name, func(t *testing.T) {
			bad, err := behavior.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			for i := range bad.Transitions {
				for j, e := range bad.Transitions[i].Effects {
					if e.Kind != behavior.Receive || e.Variable == "" {
						continue
					}
					switch name {
					case "unknown":
						bad.Transitions[i].Effects[j].Variable = "missing"
					case "domain":
						bad.Processes[0].Locals[0].Domain = []int{0, 1, 2}
					case "shared":
						bad.SharedState = bad.Processes[0].Locals
						bad.Processes[0].Locals = nil
					case "foreign":
						bad.Processes = append(bad.Processes, behavior.Process{ID: "other", Entry: "OtherEntry", Terminal: "OtherDone", Locations: []string{"OtherEntry", "OtherDone"}, Locals: bad.Processes[0].Locals})
						bad.Processes[0].Locals = nil
					case "send-binding":
						bad.Transitions[i].Effects[j].Kind = behavior.Send
					case "duplicate-before":
						bad.Transitions[i].Effects = append([]behavior.Effect{{Kind: behavior.AssignAbstractState, Variable: e.Variable, Value: 1}}, bad.Transitions[i].Effects...)
					case "duplicate-after":
						bad.Transitions[i].Effects = append(bad.Transitions[i].Effects, behavior.Effect{Kind: behavior.AssignAbstractState, Variable: e.Variable, Value: 1})
					}
					if behavior.Validate(bad) == nil {
						t.Fatalf("accepted invalid receive binding: %s", name)
					}
					if _, _, err := tla.Generate(bad); err == nil {
						t.Fatal("invalid binding executable")
					}
					return
				}
			}
			t.Fatal("missing receive binding")
		})
	}
}
