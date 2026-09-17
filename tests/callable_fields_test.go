package tests

import (
	"testing"

	"github.com/fanmi/go-tla/internal/tla"
)

func TestTLCImmutableCallableFields(t *testing.T) {
	for _, tc := range []struct{ name, source, outcome string }{
		{"interface-field", `package main
import "sync"
type sender interface{Send(chan int)};type sink struct{};func(sink)Send(ch chan int){ch<-1}
type writer struct{mu sync.Mutex;out sender};func(w *writer)Run(ch chan int){w.mu.Lock();defer w.mu.Unlock();w.out.Send(ch)}
func main(){w:=&writer{out:sink{}};ch:=make(chan int);go w.Run(ch);<-ch}`, "No error has been found"},
		{"named-callback", `package main
import "sync"
type writer struct{mu sync.Mutex;f func(chan int)};func send(ch chan int){ch<-1}
func(w *writer)Run(ch chan int){w.mu.Lock();defer w.mu.Unlock();w.f(ch)}
func main(){w:=&writer{f:send};ch:=make(chan int);go w.Run(ch);<-ch}`, "No error has been found"},
		{"captured-callback", `package main
import "sync"
type cancel func();type writer struct{mu sync.Mutex;f cancel};func(w *writer)Run(){w.mu.Lock();defer w.mu.Unlock();w.f()}
func main(){ch:=make(chan int);w:=&writer{f:func(){ch<-1}};go w.Run();<-ch}`, "No error has been found"},
		{"blocked-callback", `package main
import "sync"
type writer struct{mu sync.Mutex;f func()};func(w *writer)Run(){w.mu.Lock();defer w.mu.Unlock();w.f()}
func main(){ready:=make(chan int);w:=&writer{f:func(){ready<-1;var ch chan int;<-ch}};go w.Run();<-ready;w.mu.Lock()}`, "Deadlock reached"},
		{"callback-error", `package main
import "sync"
type writer struct{mu sync.Mutex;f func()};func(w *writer)Run(){w.f()}
func main(){ch:=make(chan int);w:=&writer{f:func(){close(ch);close(ch)}};w.Run()}`, "Invariant NoSynchronizationErrors is violated"},
		{"receiver-identity", `package main
import "sync"
type locker interface{Lock()};type sink struct{mu sync.Mutex};func(s *sink)Lock(){s.mu.Lock()}
type writer struct{mu sync.Mutex;out locker};func(w *writer)Run(){w.out.Lock()}
func main(){s:=&sink{};w:=&writer{out:s};w.Run();s.mu.Lock()}`, "Deadlock reached"},
		{"distinct-invocations", `package main
import "sync"
type locker interface{Lock()};type sink struct{mu sync.Mutex};func(s *sink)Lock(){s.mu.Lock()}
type writer struct{mu sync.Mutex;out locker};func(w *writer)Run(){w.out.Lock()}
func use(s *sink){w:=&writer{out:s};w.Run()}
func main(){a:=&sink{};b:=&sink{};use(a);use(b)}`, "No error has been found"},
		{"capture-invocations", `package main
import "sync"
type writer struct{mu sync.Mutex;f func()};func(w *writer)Run(){w.f()}
func use(ch chan int){w:=&writer{f:func(){ch<-1}};go w.Run();<-ch}
func main(){a:=make(chan int);b:=make(chan int);use(a);use(b)}`, "No error has been found"},
		{"pure-field-call", `package main
import "sync"
type writer struct{mu sync.Mutex;f func()};func nothing(){};func(w *writer)Run(){w.f()}
func main(){w:=&writer{f:nothing};w.Run();ch:=make(chan int,1);ch<-1;<-ch}`, "No error has been found"},
		{"two-fields", `package main
import "sync"
type sender interface{Send(chan int)};type sink struct{};func(sink)Send(ch chan int){ch<-1}
type writer struct{mu sync.Mutex;out sender;done func()};func(w *writer)Run(ch chan int){w.mu.Lock();defer w.mu.Unlock();w.out.Send(ch);w.done()}
func main(){done:=make(chan int);w:=&writer{out:sink{},done:func(){done<-1}};ch:=make(chan int);go w.Run(ch);<-ch;<-done}`, "No error has been found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := fromSource(t, tc.source)
			if m.HasErrors() {
				t.Fatalf("immutable callable refused: %+v", m.Diagnostics)
			}
			found := false
			for _, d := range m.Diagnostics {
				if d.Code == "resolved-field-call" && d.File != "" && d.Line > 0 {
					found = true
				}
			}
			if !found {
				t.Fatal("missing field call proof")
			}
			if len(m.Metadata.Options["trustedCalls"]) != 0 {
				t.Fatal("field call unexpectedly trusted")
			}
			checkTLC(t, m, tc.outcome)
		})
	}
}

func TestImmutableCallableFieldRefusals(t *testing.T) {
	prefix := `package main
import "sync"
type writer struct{mu sync.Mutex;f func()}
func good(){}
func other(){}
func(w *writer)Run(){w.f()}
func mutate(w *writer){w.f=other}
func inspect(p *func()){}
func returned()*writer{return &writer{f:good}}
var flag bool
var global writer
`
	for name, body := range map[string]string{
		"bound-method":         `var mu sync.Mutex;w:=&writer{f:mu.Unlock};w.Run()`,
		"uninitialized":        `w:=&writer{};w.Run()`,
		"nil":                  `w:=&writer{f:nil};w.Run()`,
		"future-store":         `w:=&writer{};w.Run();w.f=good`,
		"conditional-store":    `w:=&writer{};if flag{w.f=good};w.Run()`,
		"reassignment":         `w:=&writer{f:good};w.f=other;w.Run()`,
		"callee-write":         `w:=&writer{f:good};mutate(w);w.Run()`,
		"write-after-use":      `w:=&writer{f:good};w.Run();mutate(w)`,
		"field-address-escape": `w:=&writer{f:good};inspect(&w.f);w.Run()`,
		"object-copy":          `w:=&writer{f:good};copy:=*w;copy.Run()`,
		"object-reset":         `w:=&writer{f:good};*w=writer{f:good};w.Run()`,
		"returned-object":      `w:=returned();w.Run()`,
		"global":               `global.f=good;global.Run()`,
		"object-phi":           `a:=&writer{f:good};b:=&writer{f:other};w:=a;if flag{w=b};w.Run()`,
		"multiple-origins":     `a:=&writer{f:good};b:=&writer{f:other};a.Run();b.Run()`,
		"closure-object-alias": `w:=&writer{f:good};go func(){w.Run()}()`,
		"callback-phi":         `f:=good;if flag{f=other};w:=&writer{f:f};w.Run()`,
		"capture-future-store": `var ch chan int;w:=&writer{f:func(){ch<-1}};go w.Run();ch=make(chan int,1);<-ch`,
		"capture-reassignment": `ch:=make(chan int,1);w:=&writer{f:func(){ch<-1}};ch=make(chan int,1);w.Run()`,
	} {
		t.Run(name, func(t *testing.T) {
			m := fromSource(t, prefix+`func main(){`+body+`}`)
			if !m.HasErrors() {
				t.Fatal("unproved field binding accepted")
			}
			if _, _, err := tla.Generate(m); err == nil {
				t.Fatal("unproved field binding executable")
			}
		})
	}
}
