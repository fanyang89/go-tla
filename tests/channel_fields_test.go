package tests

import (
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/tla"
)

const channelComponent = `package main;import "sync";type T struct{ch chan int;mu sync.Mutex};func(t *T)send(){t.mu.Lock();t.ch<-1;t.mu.Unlock()};func(t *T)replace(ch chan int){t.ch=ch};func main(){`

func TestImmutableChannelFields(t *testing.T) {
	for name, body := range map[string]string{
		"literal":              "x:=&T{ch:make(chan int,1)};x.send();<-x.ch",
		"channel-only":         "x:=struct{ch chan int}{make(chan int,1)};x.ch<-1;<-x.ch",
		"direction":            "ch:=make(chan int,1);x:=struct{out chan<- int}{ch};x.out<-1;<-ch",
		"bound-method":         "x:=&T{ch:make(chan int,1)};f:=x.send;f();<-x.ch",
		"assignment":           "var x T;x.ch=make(chan int,1);x.send();<-x.ch",
		"nil":                  "var x T;close(x.ch)",
		"explicit-nil":         "var x T;x.ch=nil;close(x.ch)",
		"nested":               "var x struct{inner T};x.inner.ch=make(chan int,1);x.inner.send();<-x.inner.ch",
		"spawn":                "x:=&T{ch:make(chan int)};go x.send();<-x.ch",
		"capture":              "var x T;x.ch=make(chan int);go func(){x.ch<-1}();<-x.ch",
		"captured-initializer": "ch:=make(chan int,1);done:=make(chan int);go func(){x:=T{ch:ch};x.send();done<-1}();<-ch;<-done",
	} {
		t.Run(name, func(t *testing.T) {
			m := fromSource(t, channelComponent+body+`}`)
			if m.HasErrors() {
				t.Fatalf("unsupported: %+v", m.Diagnostics)
			}
			if _, _, err := tla.Generate(m); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestChannelFieldInitializationRefusals(t *testing.T) {
	for name, body := range map[string]string{
		"future-store":             "var x T;x.ch<-1;x.ch=make(chan int,1)",
		"future-close":             "var x T;close(x.ch);x.ch=make(chan int)",
		"capture-before-store":     "var x T;f:=func(){x.ch<-1};x.ch=make(chan int,1);f()",
		"spawn-before-store":       "var x T;go x.send();x.ch=make(chan int)",
		"call-before-store":        "var x T;x.send();x.ch=make(chan int,1)",
		"reassignment":             "x:=&T{ch:make(chan int,1)};x.ch=make(chan int,1);x.ch<-1",
		"callee-write":             "var x T;x.replace(make(chan int,1));x.ch<-1",
		"later-callee-write":       "x:=&T{ch:make(chan int,1)};x.replace(make(chan int,1));x.ch<-1",
		"field-address":            "var x T;func(p *chan int){*p=make(chan int,1)}(&x.ch);x.ch<-1",
		"copy":                     "x:=T{ch:make(chan int,1)};y:=x;y.ch<-1",
		"conditional":              "var x T;if unknown(){x.ch=make(chan int,1)};x.ch<-1",
		"pointer-cell":             "x:=&T{ch:make(chan int,1)};p:=&x;(*p).ch<-1",
		"pointer-variable-capture": "x:=&T{ch:make(chan int,1)};go func(){x.ch<-1}()",
		"closure-write":            "var x T;x.ch=make(chan int,1);func(){x.ch=make(chan int,1)}();x.ch<-1",
	} {
		t.Run(name, func(t *testing.T) {
			m := fromSource(t, channelComponent+body+`};func unknown()bool`, "fixture.unknown")
			if !m.HasErrors() {
				t.Fatal("unproved channel field accepted")
			}
			switch name {
			case "future-store", "future-close", "capture-before-store", "spawn-before-store", "call-before-store", "reassignment", "conditional":
				found := false
				for _, d := range m.Diagnostics {
					if d.Code == "channel-field-initialization" {
						found = true
					}
				}
				if !found {
					t.Fatalf("initialization proof did not reject this case: %+v", m.Diagnostics)
				}
			}
			if _, _, err := tla.Generate(m); err == nil {
				t.Fatal("unsupported IR executable")
			}
		})
	}
}

func TestChannelFieldParametersAndInvocationContexts(t *testing.T) {
	m := fromSource(t, `package main;type T struct{ch chan int};func use(ch chan int){t:=T{ch:ch};t.ch<-1;<-t.ch};func main(){use(make(chan int,1));use(make(chan int,1))}`)
	if m.HasErrors() || len(m.Channels) != 2 {
		t.Fatalf("bad context bindings: %+v", m.Diagnostics)
	}
	if _, _, err := tla.Generate(m); err != nil {
		t.Fatal(err)
	}
	sends := map[string]int{}
	for _, tr := range m.Transitions {
		for _, e := range tr.Effects {
			if e.Kind == behavior.Send {
				sends[e.Resource]++
			}
		}
	}
	if len(sends) != 2 {
		t.Fatalf("invocation field bindings aliased: %v", sends)
	}
}

func TestTLCImmutableChannelFields(t *testing.T) {
	for _, c := range []struct{ name, body, want string }{
		{"rendezvous", "x:=&T{ch:make(chan int)};go x.send();<-x.ch", "No error has been found"},
		{"nil-send", "var x T;x.ch<-1", "Error: Deadlock reached"},
		{"nil-close", "var x T;close(x.ch)", "Error: Invariant NoSynchronizationErrors is violated"},
		{"closed-send", "x:=&T{ch:make(chan int,1)};close(x.ch);x.send()", "Error: Invariant NoSynchronizationErrors is violated"},
		{"shared-channel", "ch:=make(chan int,1);a:=&T{ch:ch};b:=&T{ch:ch};a.send();<-b.ch", "No error has been found"},
		{"distinct-channels", "a:=&T{ch:make(chan int,1)};b:=&T{ch:make(chan int,1)};a.send();<-b.ch", "Error: Deadlock reached"},
		{"default-before-peer", "var x T;x.ch=make(chan int);go func(){select{case <-x.ch:default:}}();x.ch<-1", "Error: Deadlock reached"},
	} {
		t.Run(c.name, func(t *testing.T) { checkTLC(t, fromSource(t, channelComponent+c.body+`}`), c.want) })
	}
}
