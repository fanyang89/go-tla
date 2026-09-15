// Package discovery implements pass 3: recognize behavioral roots in typed SSA.
package discovery

import (
	"github.com/fanmi/go-tla/internal/behavior"
	"go/types"
	"golang.org/x/tools/go/ssa"
)

type Primitive struct {
	Kind     behavior.EffectKind
	Resource ssa.Value
	Delta    ssa.Value
}

func SyncType(t types.Type) string {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	n, ok := t.(*types.Named)
	if !ok || n.Obj().Pkg() == nil || n.Obj().Pkg().Path() != "sync" {
		return ""
	}
	return n.Obj().Name()
}
func Recognize(i ssa.Instruction) (Primitive, bool) {
	switch x := i.(type) {
	case *ssa.Go:
		return Primitive{Kind: behavior.Spawn}, true
	case *ssa.Send:
		return Primitive{Kind: behavior.Send, Resource: x.Chan}, true
	case *ssa.UnOp:
		if x.Op.String() == "<-" {
			return Primitive{Kind: behavior.Receive, Resource: x.X}, true
		}
	case *ssa.Call:
		c := x.Common()
		if b, ok := c.Value.(*ssa.Builtin); ok && b.Name() == "close" {
			return Primitive{Kind: behavior.CloseChannel, Resource: c.Args[0]}, true
		}
		f := c.StaticCallee()
		if f == nil || f.Signature.Recv() == nil || len(c.Args) == 0 {
			break
		}
		typ := SyncType(f.Signature.Recv().Type())
		var k behavior.EffectKind
		switch typ + "." + f.Name() {
		case "Mutex.Lock":
			k = behavior.Lock
		case "Mutex.Unlock":
			k = behavior.Unlock
		case "WaitGroup.Add":
			return Primitive{Kind: behavior.WaitGroupAdd, Resource: c.Args[0], Delta: c.Args[1]}, true
		case "WaitGroup.Done":
			k = behavior.WaitGroupDone
		case "WaitGroup.Wait":
			k = behavior.WaitGroupWait
		}
		if k != "" {
			return Primitive{Kind: k, Resource: c.Args[0]}, true
		}
	}
	return Primitive{}, false
}
func IsRoot(i ssa.Instruction) bool {
	if _, ok := Recognize(i); ok {
		return true
	}
	switch i.(type) {
	case *ssa.Select, *ssa.MakeChan:
		return true
	}
	return false
}

// Entries is pass 4: statically named goroutine sites (not an unbounded runtime pool).
func Entries(f *ssa.Function) []*ssa.Go {
	var out []*ssa.Go
	for _, b := range f.Blocks {
		for _, i := range b.Instrs {
			if g, ok := i.(*ssa.Go); ok {
				out = append(out, g)
			}
		}
	}
	return out
}

func SyncTypeReceiver(f *ssa.Function) string {
	if f.Signature.Recv() == nil {
		return ""
	}
	return SyncType(f.Signature.Recv().Type())
}
