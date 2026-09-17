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

func TestChannelReceiverBoxes(t *testing.T) {
	const pre = `package main;type I interface{Run()};type W struct{ch chan int};func(w *W)Run(){w.ch<-1};`
	for _, tc := range []struct{ name, source, want string }{
		{"buffered", pre + `func main(){w:=&W{make(chan int,1)};var x I=w;x.Run()}`, "No error has been found"},
		{"goroutine", pre + `func main(){c:=make(chan int);w:=&W{c};var x I=w;go x.Run();<-c}`, "No error has been found"},
		{"deferred-send", pre + `func f(c chan int){w:=&W{c};var x I=w;defer x.Run()};func main(){c:=make(chan int,1);f(c);<-c}`, "No error has been found"},
		{"direct-defer", pre + `func f(c chan int){w:=&W{c};defer w.Run()};func main(){c:=make(chan int,1);f(c);<-c}`, "No error has been found"},
		{"distinct-objects", pre + `func main(){a:=make(chan int,1);b:=make(chan int,1);var x I=&W{a};var y I=&W{b};x.Run();y.Run();<-a;<-b}`, "No error has been found"},
		{"blocking", pre + `func main(){var x I=&W{make(chan int)};x.Run()}`, "Error: Deadlock reached"},
		{"closed-send", pre + `func main(){c:=make(chan int);close(c);var x I=&W{c};x.Run()}`, "Error: Invariant NoSynchronizationErrors is violated"},
		{"deferred-close", `package main;type I interface{Finish()};type C struct{c chan int};func(c *C)Finish(){close(c.c)};func main(){var x I=&C{make(chan int)};defer x.Finish()}`, "No error has been found"},
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
			if tc.want == "No error has been found" {
				if err != nil {
					t.Fatalf("native failure %v %s", err, out)
				}
			} else {
				want := "deadlock"
				if tc.name == "closed-send" {
					want = "send on closed channel"
				}
				if err == nil || !strings.Contains(string(out), want) {
					t.Fatalf("native mismatch %v %s", err, out)
				}
			}
		})
	}
}

func TestChannelReceiverBoxRefusals(t *testing.T) {
	const pre = `package main;type I interface{Run()};type W struct{ch chan int};func(w *W)Run(){};`
	for name, source := range map[string]string{
		"receiver-mutation":     `package main;type I interface{Run()};type W struct{ch chan int};func(w *W)Run(){w.ch=make(chan int)};func main(){var x I=&W{make(chan int)};x.Run()}`,
		"field-store":           pre + `type H struct{x I};func main(){h:=&H{x:&W{make(chan int)}};h.x.Run()}`,
		"parameter":             pre + `func f(I){};func main(){var x I=&W{make(chan int)};f(x)}`,
		"returned":              pre + `func f()I{return &W{make(chan int)}};func main(){f().Run()}`,
		"global":                pre + `var saved I;func main(){saved=&W{make(chan int)}}`,
		"late-initialization":   pre + `func main(){w:=&W{};var x I=w;w.ch=make(chan int);x.Run()}`,
		"replacement":           pre + `func main(){w:=&W{make(chan int)};var x I=w;x.Run();w.ch=make(chan int)}`,
		"receiver-and-argument": `package main;type I interface{Run(I)};type W struct{ch chan int};func(w *W)Run(I){};func main(){var x I=&W{make(chan int)};x.Run(x)}`,
	} {
		t.Run(name, func(t *testing.T) {
			m := fromSource(t, source)
			if !m.HasErrors() {
				t.Fatal("unproved box accepted")
			}
			if _, _, err := tla.Generate(m); err == nil {
				t.Fatal("unproved box emitted")
			}
		})
	}
}
