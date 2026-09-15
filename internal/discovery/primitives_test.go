package discovery_test

import (
	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/discovery"
	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
	"testing"
)

func TestRecognizeTypedSSAPrimitives(t *testing.T) {
	p := testutil.Load(t, `package main
 import "sync"
 func worker(ch chan int) { ch <- 1 }
 func main(){ch:=make(chan int,2); go worker(ch); <-ch; close(ch); var mu sync.Mutex; mu.Lock(); mu.Unlock(); var wg sync.WaitGroup; wg.Add(1); wg.Done(); wg.Wait(); select{case <-ch:default:}}
 `)
	main, err := p.Main()
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[behavior.EffectKind]int{}
	selects, makes := 0, 0
	for _, b := range main.Blocks {
		for _, i := range b.Instrs {
			if x, ok := discovery.Recognize(i); ok {
				kinds[x.Kind]++
			}
			switch i.(type) {
			case *ssa.Select:
				selects++
			case *ssa.MakeChan:
				makes++
			}
		}
	}
	for _, k := range []behavior.EffectKind{behavior.Spawn, behavior.Receive, behavior.CloseChannel, behavior.Lock, behavior.Unlock, behavior.WaitGroupAdd, behavior.WaitGroupDone, behavior.WaitGroupWait} {
		if kinds[k] != 1 {
			t.Errorf("%s recognized %d times", k, kinds[k])
		}
	}
	if selects != 1 || makes != 1 || len(discovery.Entries(main)) != 1 {
		t.Fatal("missing select, channel allocation, or goroutine entry")
	}
	worker := p.Roots[0].Func("worker")
	if _, ok := discovery.Recognize(worker.Blocks[0].Instrs[0]); !ok {
		t.Fatal("send not recognized")
	}
	if p.Calls.Nodes[worker] == nil {
		t.Fatal("worker missing from static call graph")
	}
}
