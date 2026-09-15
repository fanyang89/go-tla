package tests

import (
	"testing"

	"github.com/fanmi/go-tla/internal/tla"
)

func TestStaticSyncFields(t *testing.T) {
	for name, body := range map[string]string{
		"local":              "var x T;x.mu.Lock();x.mu.Unlock()",
		"allocation":         "x:=new(T);x.lock();x.unlock()",
		"literal":            "x:=&T{};x.lock();x.unlock()",
		"same-pointer-alias": "var x T;p:=&x;q:=p;p.lock();q.unlock()",
		"subobject-receiver": "var x struct{inner T};x.inner.lock();x.inner.unlock()",
		"distinct-objects":   "var x,y T;x.mu.Lock();y.mu.Lock();y.mu.Unlock();x.mu.Unlock()",
		"nested":             "var x struct{inner T};x.inner.mu.Lock();x.inner.mu.Unlock()",
		"receiver":           "var x T;x.lock();x.unlock()",
		"closure":            "var x T;done:=make(chan int);go func(){x.mu.Lock();x.mu.Unlock();done<-1}();<-done",
		"global":             "global.mu.Lock();global.mu.Unlock()",
	} {
		t.Run(name, func(t *testing.T) {
			m := fromSource(t, `package main;import "sync";type T struct{mu sync.Mutex};var global T;func(t *T)lock(){t.mu.Lock()};func(t *T)unlock(){t.mu.Unlock()};func main(){`+body+`}`)
			if m.HasErrors() {
				t.Fatalf("unsupported: %+v", m.Diagnostics)
			}
			if _, _, err := tla.Generate(m); err != nil {
				t.Fatal(err)
			}
			want := 1
			if name == "distinct-objects" {
				want = 2
			}
			if len(m.Mutexes) != want {
				t.Fatalf("wrong field identities: %v", m.Mutexes)
			}
		})
	}
}

func TestSyncFieldRefusalBoundaries(t *testing.T) {
	for name, body := range map[string]string{
		"copy":                  "var x T;y:=x;y.mu.Lock()",
		"reset":                 "var x T;x.mu.Lock();x=T{}",
		"value-receiver":        "var x T;x.copyMethod()",
		"value-argument":        "var x T;copyArg(x)",
		"different-objects":     "var x,y T;p:=&x;if unknown(){p=&y};p.mu.Lock()",
		"nil-receiver":          "var p *T;p.lock()",
		"pointer-field":         "var x T;holder:=struct{p *T}{&x};holder.p.lock()",
		"mutable-channel-field": "x:=struct{ch chan int}{make(chan int,1)};x.ch=make(chan int,1);x.ch<-1",
		"array-element":         "var x [2]T;x[0].mu.Lock()",
		"future-pointer-store":  "var p *T;func(){p.lock()}();p=new(T)",
		"interface":             "var x T;var i interface{lock()}=&x;callInterface(i)",
	} {
		t.Run(name, func(t *testing.T) {
			m := fromSource(t, `package main;import "sync";type T struct{mu sync.Mutex};func(t *T)lock(){t.mu.Lock()};func(t T)copyMethod(){t.mu.Lock()};func copyArg(t T){};func unknown()bool;func callInterface(i interface{lock()}){i.lock()};func main(){`+body+`}`, "fixture.unknown")
			if !m.HasErrors() {
				t.Fatal("unsafe/unsupported field identity accepted")
			}
			if _, _, err := tla.Generate(m); err == nil {
				t.Fatal("unsupported model executable")
			}
		})
	}
}

func TestSyncAggregateInitializerCopyRejected(t *testing.T) {
	m := fromSource(t, `package main;import "sync";type T struct{mu sync.Mutex};var a,b T;func init(){b=a};func main(){b.mu.Lock()}`)
	if !m.HasErrors() {
		t.Fatal("initializer copied synchronization aggregate")
	}
	if _, _, err := tla.Generate(m); err == nil {
		t.Fatal("unsupported initializer executable")
	}
}

func TestTLCStaticSyncFields(t *testing.T) {
	for _, c := range []struct{ name, source, want string }{
		{"shared-lock-deadlock", `package main;import "sync";type T struct{mu sync.Mutex};func(t *T)lock(){t.mu.Lock()};func main(){var x T;x.lock();x.lock()}`, "Error: Deadlock reached"},
		{"distinct-fields", `package main;import "sync";type T struct{a,b sync.Mutex};func main(){var x T;x.a.Lock();x.b.Lock();x.b.Unlock();x.a.Unlock()}`, "No error has been found"},
		{"cross-worker-unlock", `package main;import "sync";type T struct{mu sync.Mutex};func(t *T)unlock(done chan int){t.mu.Unlock();done<-1};func main(){var x T;x.mu.Lock();done:=make(chan int);go x.unlock(done);<-done}`, "No error has been found"},
		{"invalid-unlock", `package main;import "sync";type T struct{mu sync.Mutex};func main(){var x T;x.mu.Unlock()}`, "Error: Invariant NoSynchronizationErrors is violated"},
		{"waitgroup", `package main;import "sync";type T struct{wg sync.WaitGroup};func(t *T)done(){t.wg.Done()};func main(){var x T;x.wg.Add(1);go x.done();x.wg.Wait()}`, "No error has been found"},
		{"missing-done", `package main;import "sync";type T struct{wg sync.WaitGroup};func main(){var x T;x.wg.Add(1);x.wg.Wait()}`, "Error: Deadlock reached"},
	} {
		t.Run(c.name, func(t *testing.T) { checkTLC(t, fromSource(t, c.source), c.want) })
	}
}
