package tests

import (
	"github.com/fanmi/go-tla/internal/tla"
	"testing"
)

func TestTLCOpenGlobalChannels(t *testing.T) {
	for _, tc := range []struct{ name, decl, body, want string }{
		{"rendezvous", `var c=make(chan int)`, `go func(){c<-1}();<-c`, "No error has been found"},
		{"buffered", `var c=make(chan int,2)`, `c<-1;c<-2;<-c;<-c`, "No error has been found"},
		{"close-in-main", `var c=make(chan int,1)`, `close(c);_,ok:=<-c;if ok{c<-1}`, "No error has been found"},
		{"empty", `var c=make(chan int,1)`, `<-c`, "Deadlock reached"},
		{"saturated", `var c=make(chan int,1)`, `c<-1;c<-2`, "Deadlock reached"},
		{"semaphore", `var c=make(chan int,1);var done=make(chan int);func work(){c<-1;<-c;done<-1}`, `go work();go work();<-done;<-done`, "No error has been found"},
		{"missing-release", `var c=make(chan int,1);var done=make(chan int);func work(){c<-1;done<-1}`, `go work();go work();<-done;<-done`, "Deadlock reached"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := fromSource(t, "package main;"+tc.decl+";func main(){"+tc.body+"}")
			if m.HasErrors() {
				t.Fatalf("global creation refused: %+v", m.Diagnostics)
			}
			found := false
			for _, d := range m.Diagnostics {
				if d.Code == "open-global-init" {
					found = true
				}
			}
			if !found {
				t.Fatal("missing global creation proof")
			}
			for _, c := range m.Channels {
				if c.InitiallyClosed {
					t.Fatal("open channel marked closed")
				}
			}
			checkTLC(t, m, tc.want)
		})
	}
}

func TestOpenGlobalRefusals(t *testing.T) {
	for name, source := range map[string]string{
		"rebind":            `var c=make(chan int,1);func main(){c=make(chan int,1)}`,
		"unused-rebind":     `var c=make(chan int,1);func unused(){c=nil};func main(){}`,
		"address-escape":    `var c=make(chan int,1);func foreign(*chan int);func main(){foreign(&c)}`,
		"init-send":         `var c=make(chan int,1);func init(){c<-1};func main(){<-c}`,
		"init-receive":      `var c=make(chan int,1);func init(){<-c};func main(){}`,
		"conditional-close": `var c=make(chan int);var flag bool;func init(){if flag{close(c)}};func main(){<-c}`,
		"dynamic-capacity":  `var size=1;var c=make(chan int,size);func main(){}`,
	} {
		t.Run(name, func(t *testing.T) {
			m := fromSource(t, "package main;"+source)
			if !m.HasErrors() {
				t.Fatal("unproved initialization accepted")
			}
			if _, _, err := tla.Generate(m); err == nil {
				t.Fatal("unproved initialization executable")
			}
		})
	}
}
