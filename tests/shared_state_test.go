package tests

import (
	"github.com/fanmi/go-tla/internal/behavior"
	"testing"
)

func TestTLCSharedState(t *testing.T) {
	for _, publish := range []bool{true, false} {
		name := "published"
		want := "No error has been found"
		if !publish {
			name = "missing-publication"
			want = "Error: Deadlock reached"
		}
		t.Run(name, func(t *testing.T) {
			m := fromSource(t, `package main;func main(){c:=make(chan int);go func(){close(c)}();<-c}`)
			m.SharedState = []behavior.Variable{{Name: "shared_flag", Domain: []int{0, 2}, Initial: 2}}
			for i := range m.Transitions {
				tr := &m.Transitions[i]
				for _, e := range tr.Effects {
					if e.Kind == behavior.Receive {
						tr.Guard = behavior.Guard{Kind: behavior.Equal, Variable: "shared_flag", Value: 0}
					}
					if e.Kind == behavior.CloseChannel && publish {
						tr.Effects = append(tr.Effects, behavior.Effect{Kind: behavior.AssignAbstractState, Variable: "shared_flag", Value: 0})
					}
				}
			}
			checkTLC(t, m, want)
		})
	}
}
