// Package lowering implements process discovery, abstraction and behavioral lowering.
package lowering

import (
	"fmt"
	"go/constant"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"

	"github.com/fanmi/go-tla/internal/abstract"
	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/diagnostic"
	"github.com/fanmi/go-tla/internal/discovery"
	"github.com/fanmi/go-tla/internal/frontend"
	cslice "github.com/fanmi/go-tla/internal/slice"
	"golang.org/x/tools/go/ssa"
)

type Options struct{ TrustedCalls []string }
type edge struct {
	group   string
	to      string
	guard   behavior.Guard
	effects []behavior.Effect
	pos     behavior.Position
}
type node struct{ edges []edge }
type builder struct {
	p         *frontend.Program
	m         *behavior.Model
	opts      Options
	nodes     map[string]*node
	serial    int
	stack     map[*ssa.Function]bool
	resources map[string]bool
}
type frame struct {
	f       *ssa.Function
	prefix  string
	ids     map[ssa.Value]string
	nodes   map[ssa.Instruction]string
	selects map[*ssa.Select]string
	process string
}

// Lower runs passes 3–8. Errors leave an inspectable partial IR, never an executable model.
func Lower(p *frontend.Program, opts Options) (*behavior.Model, error) {
	main, err := p.Main()
	if err != nil {
		return nil, err
	}
	m := &behavior.Model{Name: "model", Outcome: diagnostic.Precise, Assertions: []behavior.Assertion{{Kind: "NoSynchronizationErrors", Description: "No closed-channel send/close, invalid unlock, or negative WaitGroup counter"}}}
	b := &builder{p: p, m: m, opts: opts, nodes: map[string]*node{}, stack: map[*ssa.Function]bool{}, resources: map[string]bool{}}
	m.Assumptions = append(m.Assumptions, "Communication-only analysis assumes no implicit sequential runtime panics or resource exhaustion; synchronization failures remain modeled.", "Go main return terminates the whole program, including blocked workers.", "Trusted-call contracts assert total, side-effect-free execution and no synchronization; return values are abstract.")
	b.diag("info", "supported-domain", m.Assumptions[0], main.Pos())
	b.initializers()
	b.process(main, nil, "main", main.Pos())
	m.InitialState = behavior.InitialState{Main: "main", Active: []string{"main"}}
	b.regions()
	b.checkWaitGroupPhases()
	return m, nil
}
func (b *builder) fresh(prefix string) string {
	b.serial++
	return fmt.Sprintf("%s_%d", sanitize(prefix), b.serial)
}
func sanitize(s string) string {
	var w strings.Builder
	for _, c := range s {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' {
			w.WriteRune(c)
		} else {
			w.WriteByte('_')
		}
	}
	return w.String()
}
func (b *builder) position(pos token.Pos, f *ssa.Function) behavior.Position {
	if pos == token.NoPos && f != nil {
		pos = f.Pos()
	}
	p := b.p.Fset.Position(pos)
	s := behavior.Position{File: filepath.Base(p.Filename), Line: p.Line, Column: p.Column}
	if f != nil {
		s.Function = f.Name()
		if f.Pkg != nil {
			s.Package = f.Pkg.Pkg.Path()
		}
	}
	return s
}
func (b *builder) diag(severity, code, msg string, pos token.Pos) {
	p := b.p.Fset.Position(pos)
	b.m.Diagnostics = append(b.m.Diagnostics, diagnostic.Diagnostic{Severity: severity, Code: code, Message: msg, File: filepath.Base(p.Filename), Line: p.Line})
	if severity == "error" {
		b.m.Outcome = diagnostic.Unsupported
	} else if severity == "warning" && b.m.Outcome != diagnostic.Unsupported {
		b.m.Outcome = diagnostic.Abstracted
	}
}
func (b *builder) trusted(c *ssa.CallCommon) bool {
	f := c.StaticCallee()
	if f == nil {
		return false
	}
	name := f.String()
	for _, s := range b.opts.TrustedCalls {
		if s == name {
			return true
		}
	}
	return false
}
func builtinSafe(c *ssa.CallCommon) bool {
	v, ok := c.Value.(*ssa.Builtin)
	if !ok {
		return false
	}
	switch v.Name() {
	case "len", "cap", "append", "copy", "delete", "clear", "min", "max", "complex", "real", "imag", "print", "println":
		return true
	}
	return false
}
func (b *builder) pureCall(c *ssa.CallCommon, seen map[*ssa.Function]bool) bool {
	if b.trusted(c) || builtinSafe(c) {
		return true
	}
	f := c.StaticCallee()
	if f == nil || len(f.Blocks) == 0 || seen[f] || cslice.HasCycle(f) {
		return false
	}
	seen[f] = true
	defer delete(seen, f)
	for _, bb := range f.Blocks {
		for _, i := range bb.Instrs {
			if discovery.IsRoot(i) {
				return false
			}
			switch x := i.(type) {
			case *ssa.Defer, *ssa.Panic:
				return false
			case *ssa.Call:
				if !b.pureCall(x.Common(), seen) {
					return false
				}
			case *ssa.Store: // Pure summaries may not hide shared writes that control later communication.
				if a, ok := x.Addr.(*ssa.Alloc); !ok || a.Heap {
					return false
				}
			}
		}
	}
	return true
}
func (b *builder) process(f *ssa.Function, bindings map[ssa.Value]string, id string, pos token.Pos) {
	entry := b.fresh(id + "_entry")
	b.nodes[entry] = &node{}
	b.m.Processes = append(b.m.Processes, behavior.Process{ID: id, Kind: f.Name(), Entry: entry, Source: b.position(pos, f)})
	end := id + "_Done"
	b.nodes[end] = &node{}
	start := b.function(f, bindings, id, end)
	b.nodes[entry].edges = []edge{{to: start, guard: behavior.Guard{Kind: behavior.True}}}
}
func (b *builder) function(f *ssa.Function, bindings map[ssa.Value]string, process, end string) string {
	if b.stack[f] || cslice.HasCycle(f) {
		b.diag("error", "unbounded-control", "recursion or cyclic control flow unsupported (no proved finite bound)", f.Pos())
		return end
	}
	if len(f.Blocks) == 0 {
		b.diag("error", "unknown-effects", "function body unavailable: "+f.String(), f.Pos())
		return end
	}
	b.stack[f] = true
	defer delete(b.stack, f)
	fr := &frame{f: f, prefix: b.fresh(f.Name()), ids: map[ssa.Value]string{}, nodes: map[ssa.Instruction]string{}, selects: map[*ssa.Select]string{}, process: process}
	for v, id := range bindings {
		fr.ids[v] = id
	}
	for _, bb := range f.Blocks {
		for _, i := range bb.Instrs {
			fr.nodes[i] = b.fresh(f.Name() + "_" + fmt.Sprintf("L%d", b.p.Fset.Position(i.Pos()).Line))
			b.nodes[fr.nodes[i]] = &node{}
			if s, ok := i.(*ssa.Select); ok {
				v := b.fresh(process + "_select")
				fr.selects[s] = v
				domain := []int{-1}
				for n := range len(s.States) {
					domain = append(domain, n)
				}
				for j := range b.m.Processes {
					if b.m.Processes[j].ID == process {
						b.m.Processes[j].Locals = append(b.m.Processes[j].Locals, behavior.Variable{Name: v, Domain: domain, Initial: -1})
					}
				}
			}
		}
	}
	// Static resources are discovered before use, but no process can reach them before its spawn.
	for _, bb := range f.Blocks {
		for _, i := range bb.Instrs {
			switch x := i.(type) {
			case *ssa.MakeChan:
				n, ok := abstract.Integer(x.Size)
				if !ok || n < 0 || n > 1024 {
					b.diag("error", "dynamic-topology", "channel capacity must be a static integer in 0..1024", x.Pos())
					continue
				}
				id := b.fresh(f.Name() + "_channel")
				fr.ids[x] = id
				b.m.Channels = append(b.m.Channels, behavior.Channel{ID: id, Capacity: n, Source: b.position(x.Pos(), f)})
			case *ssa.Alloc:
				if typ := discovery.SyncType(x.Type()); typ != "" {
					if typ != "Mutex" && typ != "WaitGroup" {
						b.diag("error", "sync-type", "unsupported sync type "+typ, x.Pos())
					} else {
						id := b.fresh(f.Name() + "_" + x.Comment)
						fr.ids[x] = id
						b.addSync(id, typ)
					}
				}
			}
		}
	}
	sl := cslice.Compute(f, func(c *ssa.Call) bool { return !b.pureCall(c.Common(), map[*ssa.Function]bool{}) })
	_ = discovery.Entries(f) // Explicit static goroutine-entry discovery; contexts are instantiated below.
	for _, bb := range f.Blocks {
		for j, i := range bb.Instrs {
			src := fr.nodes[i]
			next := end
			if j+1 < len(bb.Instrs) {
				next = fr.nodes[bb.Instrs[j+1]]
			}
			e := edge{to: next, guard: behavior.Guard{Kind: behavior.True}, pos: b.position(i.Pos(), f)}
			if p, ok := discovery.Recognize(i); ok {
				if g, ok := i.(*ssa.Go); ok {
					callee, bind := b.callee(fr, g.Common(), g.Pos())
					if callee != nil {
						id := b.fresh(callee.Name() + "_goroutine")
						b.process(callee, bind, id, g.Pos())
						e.effects = []behavior.Effect{{Kind: behavior.Spawn, Process: id}}
					}
				} else {
					id := b.identity(fr, p.Resource, map[ssa.Value]bool{})
					if id == "nil" && p.Kind != behavior.Send && p.Kind != behavior.Receive && p.Kind != behavior.CloseChannel {
						b.diag("error", "sync-identity", "nil synchronization pointer unsupported", i.Pos())
					}
					ef := behavior.Effect{Kind: p.Kind, Resource: id}
					if p.Kind == behavior.WaitGroupAdd {
						n, ok := abstract.Integer(p.Delta)
						if !ok || n < -1024 || n > 1024 {
							b.diag("error", "waitgroup-delta", "WaitGroup.Add requires a static delta in -1024..1024", i.Pos())
						}
						ef.Value = n
					}
					e.effects = []behavior.Effect{ef}
				}
				b.nodes[src].edges = append(b.nodes[src].edges, e)
				continue
			}
			switch x := i.(type) {
			case *ssa.If:
				for k, dst := range bb.Succs {
					g := abstract.Predicate(x.Cond, fr.selects, k == 0)
					if g.Kind == behavior.Choice && sl.Control[x] && k == 0 {
						name := x.Cond.Name()
						if c, ok := x.Cond.(*ssa.Call); ok && c.Common().StaticCallee() != nil {
							name = c.Common().StaticCallee().String()
						}
						b.m.AbstractedPredicates = append(b.m.AbstractedPredicates, behavior.Predicate{Name: name, Source: b.position(x.Cond.Pos(), f), Reason: "data result controlling concurrency is nondeterministic"})
						b.diag("warning", "abstract-predicate", "predicate "+name+" affects concurrency; modeled as nondeterministic boolean", x.Cond.Pos())
					}
					b.nodes[src].edges = append(b.nodes[src].edges, edge{to: fr.nodes[dst.Instrs[0]], guard: g, pos: e.pos})
				}
				continue
			case *ssa.Jump:
				e.to = fr.nodes[bb.Succs[0].Instrs[0]]
			case *ssa.Return:
				e.to = end
			case *ssa.Select:
				for k, s := range x.States {
					kind := behavior.Receive
					if s.Dir == types.SendOnly {
						kind = behavior.Send
					}
					b.nodes[src].edges = append(b.nodes[src].edges, edge{group: src, to: next, guard: behavior.Guard{Kind: behavior.True}, effects: []behavior.Effect{{Kind: kind, Resource: b.identity(fr, s.Chan, map[ssa.Value]bool{})}, {Kind: behavior.AssignAbstractState, Variable: fr.selects[x], Value: k}}, pos: b.position(s.Pos, f)})
				}
				if !x.Blocking {
					b.nodes[src].edges = append(b.nodes[src].edges, edge{group: src, to: next, guard: behavior.Guard{Kind: behavior.Default, Variable: src}, effects: []behavior.Effect{{Kind: behavior.AssignAbstractState, Variable: fr.selects[x], Value: -1}}, pos: e.pos})
				}
				continue
			case *ssa.Call:
				if b.trusted(x.Common()) {
					name := x.Common().StaticCallee().String()
					b.diag("warning", "trusted-call", "trusted total side-effect-free contract for "+name+"; result is abstract", x.Pos())
					b.m.Assumptions = append(b.m.Assumptions, "Trusted call: "+name)
				} else if builtinSafe(x.Common()) { // discarded sequential computation
				} else {
					callee, bind := b.callee(fr, x.Common(), x.Pos())
					if callee != nil {
						if discovery.SyncTypeReceiver(callee) != "" {
							b.diag("error", "sync-method", "unsupported synchronization method "+callee.String(), x.Pos())
						} else {
							e.to = b.function(callee, bind, process, next)
						}
					}
				}
			case *ssa.Panic:
				if syntheticSelectPanic(x) {
					e.effects = []behavior.Effect{{Kind: behavior.Assert, Value: 0}}
				} else {
					b.diag("error", "exception-control", "explicit panic unsupported", i.Pos())
				}
			case *ssa.Defer, *ssa.RunDefers:
				b.diag("error", "exception-control", "defer, panic and recover unsupported", i.Pos())
			case *ssa.Store:
				if relevantType(x.Val.Type()) {
					if _, ok := x.Addr.(*ssa.Alloc); !ok {
						b.diag("error", "dynamic-topology", "storing synchronization identities through shared/indirect memory unsupported", x.Pos())
					}
				}
				if discovery.SyncType(x.Val.Type()) != "" {
					b.diag("error", "sync-copy", "copying or resetting synchronization objects unsupported", x.Pos())
				}
			}
			b.nodes[src].edges = append(b.nodes[src].edges, e)
		}
	}
	return fr.nodes[f.Blocks[0].Instrs[0]]
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
	return discovery.SyncType(t) != ""
}
func (b *builder) callee(fr *frame, c *ssa.CallCommon, pos token.Pos) (*ssa.Function, map[ssa.Value]string) {
	f := c.StaticCallee()
	if f == nil {
		b.diag("error", "unknown-effects", "dynamic/interface call may synchronize or diverge; unsupported", pos)
		return nil, nil
	}
	if len(f.Blocks) == 0 {
		b.diag("error", "unknown-effects", "unavailable body for "+f.String()+"; requires explicit total side-effect-free trust contract", pos)
		return nil, nil
	}
	bind := map[ssa.Value]string{}
	for j, p := range f.Params {
		if discovery.SyncType(p.Type()) != "" {
			if _, ok := p.Type().(*types.Pointer); !ok {
				b.diag("error", "sync-copy", "passing synchronization objects by value unsupported", pos)
			}
		}
		if j < len(c.Args) && relevantType(p.Type()) {
			bind[p] = b.identity(fr, c.Args[j], map[ssa.Value]bool{})
		}
	}
	if mc, ok := c.Value.(*ssa.MakeClosure); ok {
		for j, v := range f.FreeVars {
			if relevantType(v.Type()) || capturedChannel(v.Type()) {
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
	case *ssa.Global:
		typ := discovery.SyncType(x.Type())
		if typ == "Mutex" || typ == "WaitGroup" {
			id := sanitize(x.Pkg.Pkg.Path() + "_" + x.Name())
			b.addSync(id, typ)
			return id
		}
	case *ssa.UnOp:
		if x.Op == token.MUL {
			return b.identity(fr, x.X, seen)
		}
	case *ssa.Alloc:
		// A captured channel cell is safe only if there is exactly one direct store and no escape other than closures/loads.
		var value ssa.Value
		count := 0
		safe := true
		if refs := x.Referrers(); refs != nil {
			for _, r := range *refs {
				switch z := r.(type) {
				case *ssa.Store:
					if z.Addr == x {
						value = z.Val
						count++
					} else {
						safe = false
					}
				case *ssa.UnOp, *ssa.DebugRef, *ssa.MakeClosure:
				default:
					safe = false
				}
			}
		}
		if count == 1 && safe {
			return b.identity(fr, value, seen)
		}
	case *ssa.Phi:
		id := ""
		for _, e := range x.Edges {
			a := b.identity(fr, e, seen)
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

// SSA synthesizes this unreachable panic after exhaustive blocking-select dispatch.
// Keep it as an assertion, rather than silently deleting it or rejecting valid selects.
func syntheticSelectPanic(p *ssa.Panic) bool {
	if p.Pos() != token.NoPos {
		return false
	}
	box, ok := p.X.(*ssa.MakeInterface)
	if !ok {
		return false
	}
	c, ok := box.X.(*ssa.Const)
	return ok && c.Value != nil && c.Value.Kind() == constant.String && constant.StringVal(c.Value) == "blocking select matched no case"
}
