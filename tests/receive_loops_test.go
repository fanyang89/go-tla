package tests

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/tla"
)

func TestTLCReceiveLoops(t *testing.T) {
	for _, c := range []struct{ name, source, want string }{
		{"closed-empty", `package main;func main(){ch:=make(chan int);close(ch);for range ch {}}`, "No error has been found"},
		{"buffer-drain", `package main;func main(){ch:=make(chan int,2);ch<-1;ch<-2;close(ch);for range ch {}}`, "No error has been found"},
		{"stream", `package main;func main(){ch:=make(chan int);go func(){ch<-1;ch<-2;close(ch)}();for range ch {}}`, "No error has been found"},
		{"missing-close", `package main;func main(){ch:=make(chan int);go func(){ch<-1}();for range ch {}}`, "Error: Deadlock reached"},
		{"nil", `package main;func main(){var ch chan int;for range ch {}}`, "Error: Deadlock reached"},
		{"forward", `package main;func forward(in,out chan int){for value:=range in {out<-value*2};close(out)};func main(){in:=make(chan int);out:=make(chan int);go func(){in<-1;in<-2;close(in)}();go forward(in,out);for range out {}}`, "No error has been found"},
		{"incorrect-close", `package main;func main(){ch:=make(chan int,2);ch<-1;ch<-2;close(ch);for range ch {close(ch)}}`, "Error: Invariant NoSynchronizationErrors is violated"},
		{"explicit-status-loop", `package main;func main(){ch:=make(chan int);close(ch);for {_,ok:=<-ch;if !ok {break}}}`, "No error has been found"},
		{"deferred-done", `package main;import "sync";func consume(ch chan int,wg *sync.WaitGroup){defer wg.Done();for range ch {}};func main(){ch:=make(chan int);var wg sync.WaitGroup;wg.Add(1);go consume(ch,&wg);ch<-1;close(ch);wg.Wait()}`, "No error has been found"},
		{"blocked-cleanup", `package main;import "sync";func consume(ch chan int,wg *sync.WaitGroup){defer wg.Done();for range ch {}};func main(){ch:=make(chan int);var wg sync.WaitGroup;wg.Add(1);go consume(ch,&wg);ch<-1;wg.Wait()}`, "Error: Deadlock reached"},
		{"mutex-in-loop", `package main;import "sync";func main(){ch:=make(chan int,2);ch<-1;ch<-2;close(ch);var mu sync.Mutex;for range ch {mu.Lock();mu.Unlock()}}`, "No error has been found"},
		{"cleanup-across-loop", `package main;import "sync";func main(){ch:=make(chan int);var wg sync.WaitGroup;wg.Add(2);go func(){probe:=make(chan int);close(probe);_,ok:=<-probe;if !ok {defer wg.Done()};for range ch {};defer wg.Done()}();ch<-1;close(ch);wg.Wait()}`, "No error has been found"},
		{"sequential-loops", `package main;func main(){a:=make(chan int,1);b:=make(chan int,1);a<-1;b<-1;close(a);close(b);for range a {};for range b {}}`, "No error has been found"},
		{"more-than-integer-limit", `package main;func send(ch chan int){for range 16 {ch<-1}};func main(){ch:=make(chan int);go func(){send(ch);send(ch);close(ch)}();for range ch {}}`, "No error has been found"},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := fromSource(t, c.source)
			if len(m.AbstractedPredicates) != 0 {
				t.Fatal("loop exit was abstracted")
			}
			checkTLC(t, m, c.want)
		})
	}
}

func TestReceiveLoopRefusals(t *testing.T) {
	for name, body := range map[string]string{
		"pure":             "for {}",
		"ignored-status":   "for {<-ch}",
		"wrong-exit":       "for {_,ok:=<-ch;if ok {return}}",
		"allocation":       "for range ch {local:=make(chan int,1);local<-1}",
		"spawn":            "for range ch {go func(){ch<-1}()}",
		"defer":            "for range ch {defer mu.Unlock()}",
		"counter":          "for range ch {wg.Done()}",
		"positive-counter": "for range ch {wg.Add(1)}",
		"call":             "for range ch {helper()}",
		"nested":           "for range ch {for range ch {}}",
		"store":            "for range ch {global=1}",
		"bypass":           "for range ch {for {}}",
		"mutable-identity": "for {_,ok:=<-ch;if !ok {return};ch=other}",
	} {
		t.Run(name, func(t *testing.T) {
			m := fromSource(t, `package main;import "sync";var global int;func helper(){};func main(){ch:=make(chan int);_=ch;other:=make(chan int);_=other;var mu sync.Mutex;_= &mu;var wg sync.WaitGroup;_= &wg;`+body+`}`)
			if !m.HasErrors() {
				t.Fatal("unsafe repeated control accepted")
			}
			if _, _, err := tla.Generate(m); err == nil {
				t.Fatal("unsupported loop executable")
			}
		})
	}
}

func TestReceiveLoopIRFiniteStateContract(t *testing.T) {
	m := fromSource(t, `package main;func main(){ch:=make(chan int);close(ch);for range ch {}}`)
	if m.HasErrors() {
		t.Fatalf("lower: %+v", m.Diagnostics)
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	copy, err := behavior.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	spec, cfg, err := tla.Generate(m)
	if err != nil {
		t.Fatal(err)
	}
	spec2, cfg2, err := tla.Generate(copy)
	if err != nil || spec != spec2 || cfg != cfg2 {
		t.Fatal("cyclic IR round trip changed output")
	}
	for _, kind := range []string{"pure-subcycle", "spawn", "counter"} {
		t.Run(kind, func(t *testing.T) {
			bad, err := behavior.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			for i, tr := range bad.Transitions {
				if tr.Source != tr.Destination {
					continue
				}
				switch kind {
				case "pure-subcycle":
					bad.Transitions = append(bad.Transitions, behavior.Transition{ID: "Bypass", Process: tr.Process, Source: tr.Source, Destination: tr.Source, Guard: behavior.Guard{Kind: behavior.True}})
				case "spawn":
					bad.Processes = append(bad.Processes, behavior.Process{ID: "child", Entry: "ChildEntry", Terminal: "ChildDone", Locations: []string{"ChildEntry", "ChildDone"}})
					bad.Transitions[i].Effects = append(bad.Transitions[i].Effects, behavior.Effect{Kind: behavior.Spawn, Process: "child"})
				case "counter":
					bad.WaitGroups = append(bad.WaitGroups, "counter")
					bad.Transitions[i].Effects = append(bad.Transitions[i].Effects, behavior.Effect{Kind: behavior.WaitGroupAdd, Resource: "counter", Value: 1})
				}
				if err := behavior.Validate(bad); err != nil {
					t.Fatalf("invalid generic fixture: %v", err)
				}
				_, _, err := tla.Generate(bad)
				if err == nil || (!strings.Contains(err.Error(), "repeatable") && !strings.Contains(err.Error(), "every cycle")) {
					t.Fatalf("finite-state contract not checked: %v", err)
				}
				return
			}
			t.Fatal("missing receive back edge")
		})
	}
}
