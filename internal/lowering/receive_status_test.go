package lowering

import (
	"testing"

	"github.com/fanmi/go-tla/internal/testutil"
)

func TestReceiveStatusReturnedByCallRemainsAbstract(t *testing.T) {
	p := testutil.Load(t, `package main
func receive(ch chan int)bool{_,ok:=<-ch;return ok}
func main(){ch:=make(chan int);close(ch);if receive(ch){var bad chan int;close(bad)}}`)
	m, err := Lower(p, Options{})
	if err != nil || m.HasErrors() {
		t.Fatalf("lower: %v, %+v", err, m)
	}
	if len(m.AbstractedPredicates) != 1 {
		t.Fatal("unsupported status propagation silently claimed exact")
	}
}
