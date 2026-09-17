package tests

import (
	"testing"

	"github.com/fanmi/go-tla/internal/tla"
)

func TestImmutableInterfaceFieldRefusals(t *testing.T) {
	prefix := `package main
import "sync"
type runner interface{Run()};type sink struct{};func(sink)Run(){}
type writer struct{mu sync.Mutex;out runner};func(w *writer)Run(){w.out.Run()}
func change(w *writer){w.out=sink{}}
func use(r runner){w:=&writer{out:r};w.Run()}
`
	for name, body := range map[string]string{
		"nil":                   `w:=&writer{out:nil};w.Run()`,
		"future-store":          `w:=&writer{};w.Run();w.out=sink{}`,
		"parameter-initializer": `use(sink{})`,
		"callee-mutation":       `w:=&writer{out:sink{}};w.Run();change(w)`,
		"callee-initialization": `w:=&writer{};change(w);w.Run()`,
		"returned-box":          `w:=&writer{out:get()};w.Run()`,
	} {
		t.Run(name, func(t *testing.T) {
			m := fromSource(t, prefix+`func get()runner{return sink{}};func main(){`+body+`}`)
			if !m.HasErrors() {
				t.Fatal("unproved interface slot accepted")
			}
			if _, _, err := tla.Generate(m); err == nil {
				t.Fatal("unproved interface slot executable")
			}
		})
	}
}
