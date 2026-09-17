package tests

import (
	"strings"
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/tla"
)

func TestBuiltinOutputRefused(t *testing.T) {
	for name, body := range map[string]string{
		"print":            `func main(){print("visible")}`,
		"println":          `func main(){println("visible")}`,
		"helper":           `func output(){println("visible")};func main(){output()}`,
		"initializer":      `func init(){println("visible")};func main(){}`,
		"constructor":      `func build()int{print("visible");return 1};var x=build();func main(){}`,
		"goroutine":        `func main(){go println("visible")}`,
		"deferred":         `func main(){defer println("visible")}`,
		"literal-executed": `func output(n int){for i:=0;i<n;i++{println(i)}};func main(){output(1)}`,
	} {
		t.Run(name, func(t *testing.T) {
			m := fromSource(t, "package main;"+body)
			if !m.HasErrors() {
				t.Fatal("output discarded")
			}
			if _, _, err := tla.Generate(m); err == nil {
				t.Fatal("unmodeled output emitted")
			}
			if name == "print" || name == "println" || name == "helper" {
				found := false
				for _, d := range m.Diagnostics {
					if d.Code == "output-effects" && d.File != "" && d.Line > 0 && strings.Contains(d.Message, "explicit I/O model") {
						found = true
					}
				}
				if !found {
					t.Fatalf("output diagnostic missing: %+v", m.Diagnostics)
				}
			}
		})
	}
}

func TestBuiltinOutputRetainsArgumentEffects(t *testing.T) {
	m := fromSource(t, `package main;func main(){c:=make(chan int);println(<-c)}`)
	receive := false
	for _, transition := range m.Transitions {
		for _, effect := range transition.Effects {
			receive = receive || effect.Kind == behavior.Receive
		}
	}
	if !m.HasErrors() || !receive {
		t.Fatal("output or argument receive was discarded")
	}
}

func TestTLCOutputProofBoundaries(t *testing.T) {
	for name, source := range map[string]string{
		"shadowed-name":   `package main;func println(s string){};func main(){println("not I/O");c:=make(chan int);close(c)}`,
		"zero-executions": `package main;func output(n int){for i:=0;i<n;i++{println(i)}};func main(){output(0);c:=make(chan int);close(c)}`,
	} {
		t.Run(name, func(t *testing.T) { checkTLC(t, fromSource(t, source), "No error has been found") })
	}
}
