package tests

import (
	"context"
	"github.com/fanmi/go-tla/internal/tla"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOnceRefusals(t *testing.T) {
	for name, source := range map[string]string{
		"direct-spawn": `package main;import "sync";func main(){var o sync.Once;go o.Do(func(){})}`,
		"direct-defer": `package main;import "sync";func main(){var o sync.Once;defer o.Do(func(){})}`,
		"nil":          `package main;import "sync";func main(){var o sync.Once;o.Do(nil)}`,
		"parameter":    `package main;import "sync";func run(f func()){var o sync.Once;o.Do(f)};func main(){run(func(){})}`,
		"reset":        `package main;import "sync";func main(){var o sync.Once;o.Do(func(){});o=sync.Once{}}`,
		"copy":         `package main;import "sync";func run(o sync.Once){o.Do(func(){})};func main(){var o sync.Once;run(o)}`,
		"panic":        `package main;import "sync";func main(){var o sync.Once;o.Do(func(){panic("bad")})}`,
		"output":       `package main;import "sync";func main(){var o sync.Once;o.Do(func(){println("bad")})}`,
		"initializer":  `package main;import "sync";var o sync.Once;func init(){o.Do(func(){})};func main(){}`,
		"unknown-body": `package main;import "sync";func opaque();func main(){var o sync.Once;o.Do(opaque)}`,
	} {
		t.Run(name, func(t *testing.T) {
			m := fromSource(t, source, "(*sync.Once).Do")
			if !m.HasErrors() {
				t.Fatal("unproved Once accepted")
			}
			if _, _, err := tla.Generate(m); err == nil {
				t.Fatal("unproved Once emitted")
			}
		})
	}
}

func TestTLCOnce(t *testing.T) {
	for _, tc := range []struct{ name, source, want string }{
		{"reentrant", `package main;import "sync";func main(){var o sync.Once;o.Do(func(){o.Do(func(){})})}`, "Error: Deadlock reached"},
		{"waits-for-callback", `package main;import "sync";func main(){var o sync.Once;started:=make(chan int);release:=make(chan int);go func(){o.Do(func(){started<-1;<-release})}();<-started;o.Do(func(){});close(release)}`, "Error: Deadlock reached"},
		{"callback-defers", `package main;import "sync";func main(){var o sync.Once;c:=make(chan int);o.Do(func(){defer func(){close(c)}()});<-c;o.Do(func(){close(c)})}`, "No error has been found"},
		{"without-once-mutant", `package main;func main(){c:=make(chan int);f:=func(){close(c)};f();f()}`, "Error: Invariant NoSynchronizationErrors is violated"},
		{"one-call", `package main;import "sync";func main(){var o sync.Once;c:=make(chan int);o.Do(func(){close(c)});<-c}`, "No error has been found"},
		{"twice", `package main;import "sync";func main(){var o sync.Once;c:=make(chan int);f:=func(){close(c)};o.Do(f);o.Do(f);<-c}`, "No error has been found"},
		{"competing", `package main;import "sync";func use(o *sync.Once,c chan int,w *sync.WaitGroup){defer w.Done();o.Do(func(){close(c)})};func main(){var o sync.Once;var w sync.WaitGroup;c:=make(chan int);w.Add(2);go use(&o,c,&w);go use(&o,c,&w);w.Wait();<-c}`, "No error has been found"},
		{"callback-blocks", `package main;import "sync";func main(){var o sync.Once;c:=make(chan int);o.Do(func(){<-c})}`, "Error: Deadlock reached"},
		{"different-callback", `package main;import "sync";func main(){var o sync.Once;c:=make(chan int);o.Do(func(){close(c)});o.Do(func(){c<-1})}`, "No error has been found"},
		{"field", `package main;import "sync";type T struct{o sync.Once};func main(){x:=&T{};c:=make(chan int);x.o.Do(func(){close(c)});x.o.Do(func(){close(c)})}`, "No error has been found"},
		{"global", `package main;import "sync";var o sync.Once;func main(){c:=make(chan int);o.Do(func(){close(c)});o.Do(func(){close(c)})}`, "No error has been found"},
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
				if tc.name == "without-once-mutant" {
					want = "close of closed channel"
				}
				if err == nil || !strings.Contains(string(out), want) {
					t.Fatalf("native mismatch %v %s", err, out)
				}
			}
		})
	}
}
