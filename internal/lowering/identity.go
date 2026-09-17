package lowering

import (
	"go/token"
	"go/types"

	"github.com/fanmi/go-tla/internal/abstract"
	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/discovery"
	"golang.org/x/tools/go/ssa"
)

// Resources are static instances discovered before use; process reachability still
// gates access. Capacity extraction is checked against the retained data slice.
func (b *builder) allocateResources(fr *frame) {
	for _, bb := range fr.f.Blocks {
		for _, i := range bb.Instrs {
			switch x := i.(type) {
			case *ssa.MakeChan:
				if !b.requireData(fr, x.Size) {
					continue
				}
				n, ok := abstract.Integer(x.Size)
				if !ok || n < 0 || n > 1024 {
					b.diag("error", "dynamic-topology", "channel capacity must be a static integer in 0..1024", x.Pos())
					continue
				}
				id := b.fresh(fr.f.Name() + "_channel")
				fr.ids[x] = id
				b.m.Channels = append(b.m.Channels, behavior.Channel{ID: id, Capacity: n, Source: b.position(x.Pos(), fr.f)})
			case *ssa.Alloc:
				if aggregatePointer(x.Type()) {
					fr.ids[x] = b.fresh(fr.f.Name() + "_object")
					continue
				}
				if typ := discovery.SyncType(x.Type()); typ != "" {
					if typ != "Mutex" && typ != "WaitGroup" {
						b.diag("error", "sync-type", "unsupported sync type "+typ, x.Pos())
					} else {
						id := b.fresh(fr.f.Name() + "_" + x.Comment)
						fr.ids[x] = id
						b.addSync(id, typ)
					}
				}
			}
		}
	}
	b.prepareChannelFields(fr)
}

func (b *builder) addSync(id, typ string) {
	if b.resources[id] {
		return
	}
	b.resources[id] = true
	if typ == "Mutex" {
		b.m.Mutexes = append(b.m.Mutexes, id)
	} else {
		b.m.WaitGroups = append(b.m.WaitGroups, id)
	}
}
func relevantType(t types.Type) bool {
	if _, ok := t.Underlying().(*types.Chan); ok {
		return true
	}
	return discovery.SyncType(t) != "" || inlineSync(t) || inlineChannel(t) || aggregatePointer(t)
}

// callee consumes the call summary's graph-checked target and binds only statically
// resolvable synchronization identities. It never performs general alias analysis.
func (b *builder) callee(fr *frame, site ssa.CallInstruction, f *ssa.Function, pos token.Pos) (*ssa.Function, map[ssa.Value]string) {
	c := site.Common()
	if f == nil {
		b.diag("error", "unknown-effects", "dynamic/interface call may synchronize or diverge; unsupported", pos)
		return nil, nil
	}
	if len(f.Blocks) == 0 {
		b.diag("error", "unknown-effects", "unavailable body for "+f.String()+"; requires explicit total side-effect-free trust contract", pos)
		return nil, nil
	}
	args := c.Args
	if c.IsInvoke() {
		receiver := b.p.InvokeReceiver(site)
		if receiver == nil || b.p.CallTarget(site) != f {
			b.diag("error", "call-contract", "interface target lacks a matching receiver and call-graph proof", pos)
			return nil, nil
		}
		if !b.requireData(fr, c.Value) || !b.requireData(fr, receiver) {
			return nil, nil
		}
		args = append([]ssa.Value{receiver}, c.Args...)
		b.diag("info", "resolved-interface-call", "proved local interface target: "+f.String(), pos)
	}
	bind := map[ssa.Value]string{}
	for j, p := range f.Params {
		if discovery.SyncType(p.Type()) != "" || inlineSync(p.Type()) || channelAggregate(p.Type()) {
			if _, ok := p.Type().(*types.Pointer); !ok {
				b.diag("error", "sync-copy", "passing synchronization objects by value unsupported", pos)
			}
		}
		if j < len(args) && relevantType(p.Type()) {
			bind[p] = b.identity(fr, args[j], map[ssa.Value]bool{})
		}
	}
	if mc, ok := c.Value.(*ssa.MakeClosure); ok {
		for j, v := range f.FreeVars {
			if relevantType(v.Type()) || capturedChannel(v.Type()) || capturedSyncObject(v.Type()) {
				bind[v] = b.identity(fr, mc.Bindings[j], map[ssa.Value]bool{})
			}
		}
	}
	return f, bind
}
func (b *builder) identity(fr *frame, v ssa.Value, seen map[ssa.Value]bool) string {
	if v == nil {
		return "nil"
	}
	if id, ok := fr.ids[v]; ok {
		return id
	}
	if seen[v] {
		b.diag("error", "sync-identity", "cyclic synchronization alias unsupported", v.Pos())
		return "invalid"
	}
	seen[v] = true
	defer delete(seen, v)
	switch x := v.(type) {
	case *ssa.Const:
		if x.IsNil() {
			return "nil"
		}
	case *ssa.ChangeType:
		return b.identity(fr, x.X, seen)
	case *ssa.Convert:
		if relevantType(x.Type()) {
			return b.identity(fr, x.X, seen)
		}
	case *ssa.FieldAddr:
		if id := b.fieldIdentity(fr, x, seen); id != "" {
			return id
		}
	case *ssa.Global:
		typ := discovery.SyncType(x.Type())
		if typ == "Mutex" || typ == "WaitGroup" || aggregatePointer(x.Type()) {
			id := b.globals[x]
			if id == "" {
				id = b.fresh("global_" + x.Pkg.Pkg.Path() + "_" + x.Name())
				b.globals[x] = id
				if typ != "" {
					b.addSync(id, typ)
				}
			}
			return id
		}
	case *ssa.UnOp:
		if x.Op == token.MUL {
			return b.identity(fr, x.X, seen)
		}
	case *ssa.Alloc:
		// Exactly one store must dominate every load/capture, including same-block
		// instruction order. A unique future store is not an initialized value.
		var value ssa.Value
		var store *ssa.Store
		var uses []ssa.Instruction
		count, safe := 0, true
		if refs := x.Referrers(); refs != nil {
			for _, r := range *refs {
				switch z := r.(type) {
				case *ssa.Store:
					if z.Addr == x {
						value, store = z.Val, z
						count++
					} else {
						safe = false
					}
				case *ssa.UnOp, *ssa.MakeClosure:
					uses = append(uses, r)
				case *ssa.DebugRef:
				default:
					safe = false
				}
			}
		}
		if count == 1 && safe {
			for _, use := range uses {
				if !instructionDominates(store, use) {
					b.diag("error", "sync-initialization-order", "synchronization identity initialization must dominate every load and closure capture; future or conditional stores unsupported", use.Pos())
					return "invalid"
				}
			}
			return b.identity(fr, value, seen)
		}
	case *ssa.Phi:
		id := ""
		for _, edge := range x.Edges {
			a := b.identity(fr, edge, seen)
			if id != "" && id != a {
				b.diag("error", "dynamic-topology", "channel/synchronization phi has different possible identities", x.Pos())
				return "invalid"
			}
			id = a
		}
		return id
	}
	b.diag("error", "sync-identity", "cannot safely resolve synchronization identity: "+v.String(), v.Pos())
	return "invalid"
}
func capturedChannel(t types.Type) bool {
	p, ok := t.(*types.Pointer)
	if !ok {
		return false
	}
	_, ok = p.Elem().Underlying().(*types.Chan)
	return ok
}
func instructionDominates(before, after ssa.Instruction) bool {
	if before.Parent() != after.Parent() || before.Block() == nil || after.Block() == nil {
		return false
	}
	if before.Block() != after.Block() {
		return before.Block().Dominates(after.Block())
	}
	for _, i := range before.Block().Instrs {
		if i == before {
			return true
		}
		if i == after {
			return false
		}
	}
	return false
}
