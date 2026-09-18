package tests

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fanmi/go-tla/internal/tla"
)

func TestTLCBoundMethods(t *testing.T) {
	for _, tc := range []struct{ name, source, want, nativeError string }{
		{"once-channel", `package main;import "sync";type C chan int;func(c C)Close(){close(c)};func main(){var o sync.Once;c:=make(C);o.Do(c.Close);o.Do(c.Close);<-c}`, "No error has been found", ""},
		{"once-pointer", `package main;import "sync";type T struct{c chan int};func(t *T)Close(){close(t.c)};func main(){var o sync.Once;t:=&T{make(chan int)};o.Do(t.Close);o.Do(t.Close);<-t.c}`, "No error has been found", ""},
		{"once-unlock", `package main;import "sync";func main(){var m sync.Mutex;var o sync.Once;m.Lock();o.Do(m.Unlock);o.Do(m.Unlock);m.Lock();m.Unlock()}`, "No error has been found", ""},
		{"once-blocks", `package main;import "sync";func main(){var m sync.Mutex;var o sync.Once;m.Lock();o.Do(m.Lock)}`, "Error: Deadlock reached", "deadlock"},
		{"deferred-unlock", `package main;import "sync";func f(m *sync.Mutex){cleanup:=m.Unlock;defer cleanup()};func main(){var m sync.Mutex;m.Lock();f(&m);m.Lock();m.Unlock()}`, "No error has been found", ""},
		{"invalid-unlock", `package main;import "sync";func main(){var m sync.Mutex;cleanup:=m.Unlock;defer cleanup()}`, "Error: Invariant NoSynchronizationErrors is violated", "unlock of unlocked mutex"},
		{"receiver-snapshot", `package main;type C chan int;func(c C)Close(){close(c)};func f(a,b C){c:=a;cleanup:=c.Close;defer cleanup();c=b;_=c};func main(){a:=make(C);b:=make(C);f(a,b);<-a;close(b)}`, "No error has been found", ""},
		{"arguments", `package main;type C chan int;func(c C)Send(n int){c<-n};func f(c C){send:=c.Send;defer send(1)};func main(){c:=make(C,1);f(c);<-c}`, "No error has been found", ""},
		{"blocked-defer", `package main;type C chan int;func(c C)Send(n int){c<-n};func main(){c:=make(C);send:=c.Send;defer send(1)}`, "Error: Deadlock reached", "deadlock"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			checkTLC(t, fromSource(t, tc.source), tc.want)
			path := filepath.Join(t.TempDir(), "main.go")
			if err := os.WriteFile(path, []byte(tc.source), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			out, err := exec.CommandContext(ctx, "go", "run", path).CombinedOutput()
			if ctx.Err() != nil {
				t.Fatal("native timeout")
			}
			if tc.nativeError == "" {
				if err != nil {
					t.Fatalf("native failure %v %s", err, out)
				}
			} else if err == nil || !strings.Contains(string(out), tc.nativeError) {
				t.Fatalf("native mismatch %v %s", err, out)
			}
		})
	}
}

func TestBoundMethodRefusals(t *testing.T) {
	for name, source := range map[string]string{
		"interface-wrapper": `package main;import "sync";type I interface{Run()};type T int;func(T)Run(){};func main(){var o sync.Once;var x I=T(0);o.Do(x.Run)}`,
		"results":           `package main;type T int;func(T)Value()int{return 1};func main(){f:=T(0).Value;defer f()}`,
		"generic-receiver":  `package main;import "sync";type T[V any] struct{};func(*T[V])Run(){};func main(){var o sync.Once;x:=&T[int]{};o.Do(x.Run)}`,
		"sync-copy":         `package main;import "sync";type T struct{m sync.Mutex};func(T)Run(){};func main(){var o sync.Once;x:=T{};o.Do(x.Run)}`,
		"body-output":       `package main;import "sync";type T int;func(T)Run(){println("bad")};func main(){var o sync.Once;o.Do(T(1).Run)}`,
	} {
		t.Run(name, func(t *testing.T) {
			m := fromSource(t, source)
			if !m.HasErrors() {
				t.Fatal("unproved wrapper accepted")
			}
			if _, _, err := tla.Generate(m); err == nil {
				t.Fatal("unproved wrapper emitted")
			}
		})
	}
}
