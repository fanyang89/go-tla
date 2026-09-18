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

func TestTLCChannelReturns(t *testing.T) {
	for _, tc := range []struct{ name, source, want, native string }{
		{"factory", `package main;func build()chan int{c:=make(chan int,1);c<-1;return c};func main(){<-build()}`, "No error has been found", ""},
		{"forward", `package main;func forward(c chan int)chan int{return c};func again(c chan int)chan int{return forward(c)};func main(){c:=make(chan int,1);again(c)<-1;<-c}`, "No error has been found", ""},
		{"deferred-close", `package main;func shut(c chan int){close(c)};func build()chan int{c:=make(chan int);defer shut(c);return c};func main(){close(build())}`, "Invariant NoSynchronizationErrors is violated", "close of closed channel"},
		{"deferred-block", `package main;func wait(c chan int){<-c};func build()chan int{c:=make(chan int);defer wait(c);return c};func main(){close(build())}`, "Deadlock reached", "deadlock"},
		{"same-identity", `package main;func pick(c chan int,b bool)chan int{if b{return c};return c};func main(){c:=make(chan int);close(pick(c,true));<-c}`, "No error has been found", ""},
		{"distinct-invocations", `package main;func build()chan int{return make(chan int)};func main(){a:=build();b:=build();close(a);close(b)}`, "No error has been found", ""},
		{"nil", `package main;func empty()chan int{return nil};func main(){<-empty()}`, "Deadlock reached", "deadlock"},
		{"goroutine-forward", `package main;func forward(c chan int)chan int{return c};func main(){c:=make(chan int);go func(){<-forward(c)}();c<-1}`, "No error has been found", ""},
		{"direction", `package main;type C chan int;func view(c C)<-chan int{return c};func main(){c:=make(C,1);c<-1;<-view(c)}`, "No error has been found", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := fromSource(t, tc.source)
			checkTLC(t, m, tc.want)
			path := filepath.Join(t.TempDir(), "main.go")
			if err := os.WriteFile(path, []byte(tc.source), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			out, err := exec.CommandContext(ctx, "go", "run", path).CombinedOutput()
			if ctx.Err() != nil {
				t.Fatal(ctx.Err())
			}
			if tc.native == "" {
				if err != nil {
					t.Fatalf("native %v: %s", err, out)
				}
			} else if err == nil || !strings.Contains(string(out), tc.native) {
				t.Fatalf("native outcome %v: %s", err, out)
			}
		})
	}
}

func TestChannelReturnRefusals(t *testing.T) {
	for name, source := range map[string]string{
		"mutable-named-result": `package main;func f(a,b chan int)(c chan int){defer func(){c=b}();return a};func main(){close(f(make(chan int),make(chan int)))}`,
		"recover":              `package main;func f()(c chan int){defer func(){recover()}();panic("bad")};func main(){close(f())}`,
		"different":            `package main;func pick(a,b chan int,x bool)chan int{if x{return a};return b};func main(){close(pick(make(chan int),make(chan int),true))}`,
		"output":               `package main;func f()chan int{println("effect");return make(chan int)};func main(){close(f())}`,
		"recursive":            `package main;func f()chan int{return f()};func main(){close(f())}`,
		"tuple":                `package main;func f()(chan int,int){return make(chan int),1};func main(){c,_:=f();close(c)}`,
		"object":               `package main;type T struct{c chan int};func f()*T{return &T{make(chan int)}};func main(){close(f().c)}`,
	} {
		t.Run(name, func(t *testing.T) {
			m := fromSource(t, source)
			if !m.HasErrors() {
				t.Fatal("unproved return accepted")
			}
		})
	}
}
