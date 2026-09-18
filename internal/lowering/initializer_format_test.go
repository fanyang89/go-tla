package lowering

import (
	"slices"
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/effects"
	"github.com/fanmi/go-tla/internal/testutil"
	"github.com/fanmi/go-tla/internal/tla"
)

func TestInitializerFormatAssumptionsAndIndependentRefusal(t *testing.T) {
	p := testutil.Load(t, `package main;import "fmt";var values [40]string;func init(){for i:=0;i<40;i++{values[i]=fmt.Sprintf("%d",i)}};func main(){c:=make(chan int,1);c<-1;<-c}`)
	b := &builder{p: p, m: &behavior.Model{}, effects: effects.New(p, nil)}
	if !b.consumeInitializerData(p.Roots[0].Func("init#1")) || !slices.Contains(b.m.Assumptions, effects.ScalarFormatModel) {
		t.Fatal("composed assumption not consumed")
	}
	m, err := Lower(p, Options{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range m.Diagnostics {
		if d.Code == "initializer-data" && d.File == "main.go" {
			found = true
		}
	}
	if !found || !slices.Contains(m.Assumptions, effects.ScalarFormatModel) || !m.HasErrors() {
		t.Fatal("composed proof or independent initialization refusal missing")
	}
	if _, _, err := tla.Generate(m); err == nil {
		t.Fatal("unsupported imported initialization emitted executable")
	}
}
