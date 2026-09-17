package tests

import (
	"testing"

	"github.com/fanmi/go-tla/internal/tla"
)

func TestTLCDirectInterfaceCalls(t *testing.T) {
	for _, tc := range []struct{ name, source, outcome string }{
		{"relay-arguments", `package main
type relay interface{Run(chan int,chan int)};type worker struct{}
func(worker)Run(in,out chan int){<-in;out<-1}
func main(){var r relay=worker{};in:=make(chan int);out:=make(chan int);go r.Run(in,out);in<-1;<-out}`, "No error has been found"},
		{"shared-mutex-deadlock", `package main
import "sync"
type locker interface{Lock()};type box struct{mu sync.Mutex}
func(b *box)Lock(){b.mu.Lock()}
func main(){b:=&box{};var l locker=b;l.Lock();l.Lock()}`, "Deadlock reached"},
		{"distinct-mutexes", `package main
import "sync"
type locker interface{Lock();Unlock()};type box struct{mu sync.Mutex}
func(b *box)Lock(){b.mu.Lock()};func(b *box)Unlock(){b.mu.Unlock()}
func main(){a:=&box{};b:=&box{};var x locker=a;var y locker=b;x.Lock();y.Lock();x.Unlock();y.Unlock()}`, "No error has been found"},
		{"cross-worker-unlock", `package main
import "sync"
type releaser interface{Release(chan int)};type box struct{mu sync.Mutex}
func(b *box)Release(done chan int){b.mu.Unlock();done<-1}
func main(){b:=&box{};b.mu.Lock();var r releaser=b;done:=make(chan int);go r.Release(done);<-done}`, "No error has been found"},
		{"invalid-unlock", `package main
import "sync"
type releaser interface{Release()};type box struct{mu sync.Mutex}
func(b *box)Release(){b.mu.Unlock()}
func main(){b:=&box{};var r releaser=b;r.Release()}`, "Invariant NoSynchronizationErrors is violated"},
		{"joined-workers", `package main
import "sync"
type worker interface{Run(*sync.WaitGroup)};type box struct{mu sync.Mutex}
func(b *box)Run(w *sync.WaitGroup){defer w.Done();b.mu.Lock();defer b.mu.Unlock()}
func main(){b:=&box{};var r worker=b;var w sync.WaitGroup;w.Add(2);go r.Run(&w);go r.Run(&w);w.Wait()}`, "No error has been found"},
		{"named-channel", `package main
type sender interface{Send()};type pipe chan int
func(p pipe)Send(){p<-1}
func main(){p:=make(pipe);var s sender=p;go s.Send();<-p}`, "No error has been found"},
		{"interface-widening", `package main
type one interface{Send(chan int)};type two interface{Send(chan int);Extra()};type worker struct{}
func(worker)Send(ch chan int){ch<-1};func(worker)Extra(){}
func main(){var both two=worker{};var s one=both;ch:=make(chan int);go s.Send(ch);<-ch}`, "No error has been found"},
		{"pure-body-retains-proof", `package main
type valuer interface{Value()int};type value int
func(v value)Value()int{return int(v)}
func main(){var v valuer=value(1);_ = v.Value();ch:=make(chan int,1);ch<-1;<-ch}`, "No error has been found"},
		{"blocked-body-retains-defer", `package main
import "sync"
type worker interface{Run(chan int)};type box struct{mu sync.Mutex}
func(b *box)Run(ready chan int){b.mu.Lock();defer b.mu.Unlock();ready<-1;var ch chan int;<-ch}
func main(){b:=&box{};var w worker=b;ready:=make(chan int);go w.Run(ready);<-ready;b.mu.Lock()}`, "Deadlock reached"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := fromSource(t, tc.source)
			if m.HasErrors() {
				t.Fatalf("proved invoke refused: %+v", m.Diagnostics)
			}
			proved := false
			for _, d := range m.Diagnostics {
				if d.Code == "resolved-interface-call" && d.File != "" && d.Line > 0 {
					proved = true
				}
			}
			if !proved {
				t.Fatal("missing source-located interface proof")
			}
			if len(m.Metadata.Options["trustedCalls"]) != 0 {
				t.Fatal("interface call unexpectedly trusted")
			}
			checkTLC(t, m, tc.outcome)
		})
	}
}

func TestDirectInterfaceRefusals(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		trusted      []string
	}{
		{"parameter", `package main
type I interface{Run()};type worker struct{};func(worker)Run(){}
func apply(i I){i.Run()};func main(){apply(worker{})}`, nil},
		{"field", `package main
type I interface{Run()};type worker struct{};func(worker)Run(){}
type holder struct{f I};func main(){h:=holder{f:worker{}};h.f.Run()}`, nil},
		{"phi", `package main
type I interface{Run()};type first struct{};type second struct{};func(first)Run(){};func(second)Run(){}
var flag bool;func main(){var i I=first{};if flag{i=second{}};i.Run()}`, nil},
		{"returned-interface", `package main
type I interface{Run()};type worker struct{};func(worker)Run(){};func get()I{return worker{}}
func main(){get().Run()}`, nil},
		{"nil-interface", `package main
type I interface{Run()};func main(){var i I;i.Run()}`, nil},
		{"typed-nil", `package main
type I interface{Run()};type worker struct{};func(*worker)Run(){};func main(){var p *worker;var i I=p;i.Run()}`, nil},
		{"pointer-value-adaptation", `package main
type I interface{Run()};type worker struct{};func(worker)Run(){};func main(){var i I=&worker{};i.Run()}`, nil},
		{"promoted", `package main
type I interface{Run()};type worker struct{};func(*worker)Run(){};type outer struct{worker};func main(){var i I=&outer{};i.Run()}`, nil},
		{"generic", `package main
type I interface{Run()};type worker[T any] struct{};func(*worker[T])Run(){};func main(){var i I=&worker[int]{};i.Run()}`, nil},
		{"sync-copy", `package main
import "sync"
type I interface{Run()};type worker struct{mu sync.Mutex};func(worker)Run(){};func main(){var i I=worker{};i.Run()}`, nil},
		{"primitive-trust-bypass", `package main
import "sync"
func main(){var mu sync.Mutex;var l sync.Locker=&mu;l.Lock()}`, []string{"(*sync.Mutex).Lock"}},
		{"recursive-body", `package main
type I interface{Run()};type worker struct{};func(w *worker)Run(){var i I=w;i.Run()};func main(){var i I=&worker{};i.Run()}`, nil},
		{"body-still-inspected", `package main
type I interface{Run()};type worker struct{};func external();func(worker)Run(){external()};func main(){var i I=worker{};i.Run()}`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := fromSource(t, tc.source, tc.trusted...)
			if !m.HasErrors() {
				t.Fatal("unproved interface pattern accepted")
			}
			if _, _, err := tla.Generate(m); err == nil {
				t.Fatal("unproved interface pattern executable")
			}
		})
	}
}
