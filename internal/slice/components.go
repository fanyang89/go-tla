package slice

import "golang.org/x/tools/go/ssa"

// CyclicComponents returns reachable cyclic SCCs in deterministic DFS order.
func CyclicComponents(f *ssa.Function) [][]*ssa.BasicBlock {
	index, low := map[*ssa.BasicBlock]int{}, map[*ssa.BasicBlock]int{}
	onStack := map[*ssa.BasicBlock]bool{}
	var stack []*ssa.BasicBlock
	var components [][]*ssa.BasicBlock
	next := 0
	var visit func(*ssa.BasicBlock)
	visit = func(bb *ssa.BasicBlock) {
		next++
		index[bb] = next
		low[bb] = next
		stack = append(stack, bb)
		onStack[bb] = true
		for _, succ := range bb.Succs {
			if index[succ] == 0 {
				visit(succ)
				low[bb] = min(low[bb], low[succ])
			} else if onStack[succ] {
				low[bb] = min(low[bb], index[succ])
			}
		}
		if low[bb] != index[bb] {
			return
		}
		var component []*ssa.BasicBlock
		for {
			last := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[last] = false
			component = append(component, last)
			if last == bb {
				break
			}
		}
		cyclic := len(component) > 1
		for _, succ := range bb.Succs {
			if succ == bb {
				cyclic = true
			}
		}
		if cyclic {
			components = append(components, component)
		}
	}
	if len(f.Blocks) > 0 {
		visit(f.Blocks[0])
	}
	return components
}
