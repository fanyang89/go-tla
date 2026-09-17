package tests

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"

	"github.com/fanmi/go-tla/internal/tla"
)

func TestTLCClosedGlobalChannels(t *testing.T) {
	for _, tc := range []struct{ name, capacity, body, want string }{
		{"receive", "0", `<-ready;<-ready`, "No error has been found"},
		{"workers", "0", `done:=make(chan int);go func(){<-ready;done<-1}();go func(){<-ready;done<-2}();<-done;<-done`, "No error has been found"},
		{"buffered-empty-status", "2", `_,ok:=<-ready;if ok{close(ready)}`, "No error has been found"},
		{"range", "0", `for range ready{close(ready)}`, "No error has been found"},
		{"select-default", "0", `select{case <-ready:default:close(ready)}`, "No error has been found"},
		{"send-error", "0", `ready<-1`, "Invariant NoSynchronizationErrors is violated"},
		{"close-error", "0", `close(ready)`, "Invariant NoSynchronizationErrors is violated"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := fromSource(t, `package main;var ready=make(chan int,`+tc.capacity+`);func init(){close(ready)};func main(){`+tc.body+`}`)
			if m.HasErrors() {
				t.Fatalf("closed global refused: %+v", m.Diagnostics)
			}
			if len(m.Channels) == 0 || !m.Channels[0].InitiallyClosed {
				t.Fatalf("missing initial closed state: %+v", m.Channels)
			}
			found := 0
			for _, d := range m.Diagnostics {
				if d.Code == "closed-global-init" {
					found++
				}
			}
			if found != 2 {
				t.Fatalf("missing creation/close provenance: %+v", m.Diagnostics)
			}
			data, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := behavior.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if !decoded.Channels[0].InitiallyClosed {
				t.Fatal("saved IR lost initial channel state")
			}
			checkTLC(t, decoded, tc.want)
		})
	}
}

func TestTLCClosedGlobalStateMutation(t *testing.T) {
	m := fromSource(t, `package main;var ready=make(chan int);func init(){close(ready)};func main(){<-ready}`)
	if m.HasErrors() || len(m.Channels) != 1 {
		t.Fatalf("invalid fixture: %+v", m.Diagnostics)
	}
	m.Channels[0].InitiallyClosed = false
	checkTLC(t, m, "Deadlock reached")
}

func TestClosedGlobalRefusals(t *testing.T) {
	for name, decl := range map[string]string{
		"conditional": `var ready=make(chan int);func condition()bool{return true};func init(){if condition(){close(ready)}}`,
		"twice":       `var ready=make(chan int);func init(){close(ready);close(ready)}`,
		"two-inits":   `var ready=make(chan int);func init(){close(ready)};func init(){close(ready)}`,
		"rebind":      `var ready=make(chan int);func init(){close(ready)};func unused(){ready=nil}`,
		"address":     `var ready=make(chan int);var ptr=&ready;func init(){close(ready)}`,
		"filled":      `var ready=make(chan int,1);func init(){ready<-1;close(ready)}`,
		"helper":      `var ready=make(chan int);func finish(){close(ready)};func init(){finish()}`,
		"returned":    `func create()chan int{return make(chan int)};var ready=create();func init(){close(ready)}`,
	} {
		t.Run(name, func(t *testing.T) {
			m := fromSource(t, "package main;"+decl+";func main(){<-ready}")
			if !m.HasErrors() {
				t.Fatal("unproved global initialization accepted")
			}
			if _, _, err := tla.Generate(m); err == nil {
				t.Fatal("unproved global initialization executable")
			}
		})
	}
}
