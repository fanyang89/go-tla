package discovery

import (
	"go/types"
	"golang.org/x/tools/go/ssa"
)

// Function is the explicit primitive/process discovery result consumed by slicing
// and context-sensitive lowering. All keys refer to the same immutable SSA function.
type Function struct {
	Roots      map[ssa.Instruction]bool
	Primitives map[ssa.Instruction]Primitive
	Goroutines []*ssa.Go
}

func Scan(f *ssa.Function) Function {
	r := Function{Roots: map[ssa.Instruction]bool{}, Primitives: map[ssa.Instruction]Primitive{}}
	for _, bb := range f.Blocks {
		for _, i := range bb.Instrs {
			if primitive, ok := Recognize(i); ok {
				r.Primitives[i] = primitive
				r.Roots[i] = true
			}
			switch x := i.(type) {
			case *ssa.Store:
				if _, field := x.Addr.(*ssa.FieldAddr); field {
					if _, channel := x.Val.Type().Underlying().(*types.Chan); channel {
						r.Roots[i] = true
					}
				}
			case *ssa.Go:
				r.Goroutines = append(r.Goroutines, x)
			case *ssa.Select, *ssa.MakeChan, *ssa.Defer, *ssa.RunDefers:
				r.Roots[i] = true
			}
		}
	}
	return r
}
