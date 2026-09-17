package tests

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type deferredDispatchCase struct{ name, source, want string }

func deferredDispatchCases() []deferredDispatchCase {
	const iface = `package main;type cleanup interface{Finish()};type closer chan int;func(x closer)Finish(){close(x)};`
	return []deferredDispatchCase{
		{"empty-interface-body", `package main;type I interface{Run()};type worker struct{};func(worker)Run(){};func main(){var i I=worker{};defer i.Run()}`, "No error has been found"},
		{"interface", iface + `func f(c chan int){var x cleanup=closer(c);defer x.Finish()};func main(){c:=make(chan int);f(c);<-c}`, "No error has been found"},
		{"receiver-snapshot", iface + `func f(a,b chan int){var x cleanup=closer(a);defer x.Finish();x=closer(b);_=x};func main(){a:=make(chan int);b:=make(chan int);f(a,b);<-a;close(b)}`, "No error has been found"},
		{"interface-double-close", iface + `func main(){c:=make(chan int);var x cleanup=closer(c);defer x.Finish();close(c)}`, "Error: Invariant NoSynchronizationErrors is violated"},
		{"function-field", `package main;import "sync";type hook struct{mu sync.Mutex;f func()};func run(c chan int){h:=&hook{f:func(){close(c)}};h.mu.Lock();defer h.mu.Unlock();defer h.f()};func main(){a:=make(chan int);b:=make(chan int);run(a);run(b);<-a;<-b}`, "No error has been found"},
		{"interface-field", `package main;import "sync";type cleanup interface{Finish()};type closer chan int;func(x closer)Finish(){close(x)};type holder struct{mu sync.Mutex;x cleanup};func f(c chan int){h:=&holder{x:closer(c)};h.mu.Lock();defer h.mu.Unlock();defer h.x.Finish()};func main(){c:=make(chan int);f(c);<-c}`, "No error has been found"},
		{"field-blocking", `package main;import "sync";type hook struct{mu sync.Mutex;f func()};func main(){c:=make(chan int);h:=&hook{f:func(){<-c}};h.mu.Lock();defer h.mu.Unlock();defer h.f()}`, "Error: Deadlock reached"},
		{"lifo", iface + `type sender chan int;func(x sender)Finish(){x<-1};func f(c chan int){var a cleanup=closer(c);var b cleanup=sender(c);defer a.Finish();defer b.Finish()};func main(){c:=make(chan int,1);f(c);<-c}`, "No error has been found"},
		{"reversed-lifo", iface + `type sender chan int;func(x sender)Finish(){x<-1};func f(c chan int){var a cleanup=closer(c);var b cleanup=sender(c);defer b.Finish();defer a.Finish()};func main(){c:=make(chan int,1);f(c)}`, "Error: Invariant NoSynchronizationErrors is violated"},
		{"argument-blocking", `package main;type I interface{Finish(int)};type C chan int;func(c C)Finish(n int){close(c)};func main(){c:=make(chan int);wait:=make(chan int);var x I=C(c);defer x.Finish(<-wait)}`, "Error: Deadlock reached"},
	}
}

func TestTLCDeferredDispatch(t *testing.T) {
	for _, tc := range deferredDispatchCases() {
		t.Run(tc.name, func(t *testing.T) { checkTLC(t, fromSource(t, tc.source), tc.want) })
	}
}

func TestNativeDeferredDispatch(t *testing.T) {
	for _, tc := range deferredDispatchCases() {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "main.go")
			if err := os.WriteFile(path, []byte(tc.source), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			out, err := exec.CommandContext(ctx, "go", "run", path).CombinedOutput()
			if ctx.Err() != nil {
				t.Fatalf("native timeout: %s", out)
			}
			if tc.want == "No error has been found" {
				if err != nil {
					t.Fatalf("native failure: %v %s", err, out)
				}
				return
			}
			want := "deadlock"
			if tc.name == "interface-double-close" {
				want = "close of closed channel"
			}
			if tc.name == "reversed-lifo" {
				want = "send on closed channel"
			}
			if err == nil || !strings.Contains(string(out), want) {
				t.Fatalf("native mismatch: %v %s", err, out)
			}
		})
	}
}
