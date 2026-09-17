package tests

import (
	"testing"

	"github.com/fanmi/go-tla/internal/tla"
)

func TestTLCPrivateDataInitializers(t *testing.T) {
	for _, tc := range []struct{ name, prefix, main, outcome string }{
		{"record-rendezvous", `type config struct{label string;count int};func newConfig()*config{return &config{label:"ready",count:2}};var c=newConfig()`, `ch:=make(chan int);go func(){ch<-1}();<-ch`, "No error has been found"},
		{"boxed-error-deadlock", `type problem struct{message string};func(p *problem)Error()string{return p.message};func newProblem()error{return &problem{message:"limit"}};var limit=newProblem()`, `ch:=make(chan int);ch<-1`, "Deadlock reached"},
		{"nested-data-close-error", `type item struct{value int};type config struct{item item};func newConfig()*config{c:=new(config);c.item.value=2;return c};var c=newConfig()`, `ch:=make(chan int);close(ch);close(ch)`, "Invariant NoSynchronizationErrors is violated"},
		{"returned-scalar", `func value()*int{return new(2)};var n=value()`, `ch:=make(chan int,1);ch<-1;<-ch`, "No error has been found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := fromSource(t, "package main;"+tc.prefix+";func main(){"+tc.main+"}")
			if m.HasErrors() {
				t.Fatalf("private data initializer refused: %+v", m.Diagnostics)
			}
			if len(m.Metadata.Options["trustedCalls"]) != 0 {
				t.Fatal("constructor must not need a trust contract")
			}
			checkTLC(t, m, tc.outcome)
		})
	}
}

func TestPrivateDataInitializerRefusals(t *testing.T) {
	for name, body := range map[string]string{
		"publication":      `p:=&record{value:1};published=p;return p`,
		"shared-write":     `p:=&record{value:1};shared.value=2;return p`,
		"map-write":        `p:=&record{value:1};ledger[1]=2;return p`,
		"shared-copy":      `p:=&record{value:1};copy(buffer,"x");return p`,
		"shared-delete":    `p:=&record{value:1};delete(ledger,1);return p`,
		"shared-clear":     `p:=&record{value:1};clear(buffer);return p`,
		"escaping-address": `p:=&record{value:1};observe(p);return p`,
		"goroutine":        `p:=&record{value:1};go observe(p);return p`,
		"defer":            `p:=&record{value:1};defer observe(p);return p`,
		"unknown-call":     `p:=&record{value:1};external();return p`,
	} {
		t.Run(name, func(t *testing.T) {
			m := fromSource(t, `package main
type record struct{value int}
var shared record
var published *record
var ledger=map[int]int{}
var buffer=make([]byte,2)
func observe(p *record)int{return p.value}
func external()
func construct()*record{`+body+`}
var data=construct()
func main(){ch:=make(chan int,1);ch<-1;<-ch}`)
			found := false
			for _, d := range m.Diagnostics {
				if d.Code == "initializer" && d.Severity == "error" {
					found = true
				}
			}
			if !found {
				t.Fatal("unsafe constructor lost initializer refusal")
			}
			if _, _, err := tla.Generate(m); err == nil {
				t.Fatal("unsafe initializer generated executable model")
			}
		})
	}
}
