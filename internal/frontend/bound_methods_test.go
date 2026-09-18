package frontend_test

import (
	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
	"testing"
)

func TestBoundMethodForwardingProof(t *testing.T) {
	for _, broken := range []string{"none", "graph", "receiver", "argument", "wrapper-kind", "body", "signature"} {
		t.Run(broken, func(t *testing.T) {
			p := testutil.Load(t, `package main;type T chan int;func(t T)Send(n int){t<-n};func main(){t:=make(T,1);f:=t.Send;defer f(1)}`)
			main, _ := p.Main()
			var wrapper *ssa.Function
			for _, bb := range main.Blocks {
				for _, i := range bb.Instrs {
					if c, ok := i.(*ssa.MakeClosure); ok {
						wrapper, _ = c.Fn.(*ssa.Function)
					}
				}
			}
			if wrapper == nil || p.BoundMethodTarget(wrapper) == nil {
				t.Fatalf("wrapper proof missing: %v synthetic=%q syntax=%T object=%v recv=%v free=%d params=%d body=%v", wrapper, wrapper.Synthetic, wrapper.Syntax(), wrapper.Object(), wrapper.Signature.Recv(), len(wrapper.FreeVars), len(wrapper.Params), wrapper.Blocks[0].Instrs)
			}
			call := wrapper.Blocks[0].Instrs[0].(*ssa.Call)
			switch broken {
			case "graph":
				p.Calls.Nodes[wrapper].Out = nil
			case "receiver":
				call.Common().Args[0] = ssa.NewConst(nil, call.Common().Args[0].Type())
			case "argument":
				call.Common().Args[1] = ssa.NewConst(nil, call.Common().Args[1].Type())
			case "wrapper-kind":
				wrapper.Synthetic = "unproved thunk"
			case "body":
				wrapper.Blocks[0].Instrs = append(wrapper.Blocks[0].Instrs, wrapper.Blocks[0].Instrs[1])
			case "signature":
				wrapper.Signature = nil
			}
			if got := p.BoundMethodTarget(wrapper); (got != nil) != (broken == "none") {
				t.Fatal("invalid wrapper accepted or valid refused")
			}
		})
	}
}
