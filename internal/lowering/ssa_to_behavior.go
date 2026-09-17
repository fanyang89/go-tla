// Package lowering implements process discovery, abstraction and behavioral lowering.
package lowering

import (
	"fmt"
	"go/constant"
	"go/token"
	"go/types"
	"strings"

	"github.com/fanmi/go-tla/internal/abstract"
	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/diagnostic"
	"github.com/fanmi/go-tla/internal/discovery"
	"github.com/fanmi/go-tla/internal/effects"
	"github.com/fanmi/go-tla/internal/frontend"
	"golang.org/x/tools/go/ssa"
)

type Options struct{ TrustedCalls []string }
type edge struct {
	boundary bool
	group    string
	to       string
	guard    behavior.Guard
	effects  []behavior.Effect
	pos      behavior.Position
}
type node struct{ edges []edge }
type builder struct {
	p              *frontend.Program
	m              *behavior.Model
	opts           Options
	nodes          map[string]*node
	names          map[string]int
	globals        map[*ssa.Global]string
	fields         map[string]string
	channelFields  map[string]string
	callableFields map[callableFieldKey]callableFieldBinding
	stack          map[*ssa.Function]bool
	resources      map[string]bool
	effects        *effects.Analyzer
	plans          map[*ssa.Function]*functionPlan
	loopReported   map[*ssa.Function]bool
}
type frame struct {
	f             *ssa.Function
	plan          *functionPlan
	ids           map[ssa.Value]string
	nodes         map[ssa.Instruction]string
	selects       map[*ssa.Select]string
	receiveStatus map[ssa.Value]string
	process       string
	fieldStores   map[*ssa.Store]bool
	defers        map[*ssa.Defer]deferredCall
	cleanup       map[ssa.Instruction][]deferredCall
}

// Lower runs passes 3–8. Errors leave an inspectable partial IR, never an executable model.
func Lower(p *frontend.Program, opts Options) (*behavior.Model, error) {
	if p.Calls == nil {
		return nil, fmt.Errorf("call graph must be built before lowering")
	}
	main, err := p.Main()
	if err != nil {
		return nil, err
	}
	m := &behavior.Model{SchemaVersion: behavior.SchemaVersion, Semantics: behavior.CommunicationSemantics, Termination: behavior.MainReturn, Metadata: modelMetadata(opts), Name: "model", Outcome: diagnostic.Precise, Assertions: []behavior.Assertion{{Kind: "NoSynchronizationErrors", Description: "No closed-channel send/close, invalid unlock, or negative WaitGroup counter"}}}
	b := &builder{p: p, m: m, opts: opts, nodes: map[string]*node{}, stack: map[*ssa.Function]bool{}, resources: map[string]bool{}, effects: effects.New(p, opts.TrustedCalls), plans: map[*ssa.Function]*functionPlan{}, names: map[string]int{}, globals: map[*ssa.Global]string{}, fields: map[string]string{}, channelFields: map[string]string{}, loopReported: map[*ssa.Function]bool{}}
	m.Assumptions = append(m.Assumptions, "Communication-only analysis assumes no implicit sequential runtime panics or resource exhaustion; synchronization failures remain modeled.", "Go main return terminates the whole program, including blocked workers.", "Trusted-call contracts assert total, side-effect-free execution and no synchronization; return values are abstract.")
	b.diag("info", "supported-domain", m.Assumptions[0], main.Pos())
	b.initializers()
	b.process(main, nil, "main")
	m.InitialState = behavior.InitialState{Main: "main", Active: []string{"main"}}
	b.regions()
	b.checkWaitGroupPhases()
	if !m.HasErrors() {
		if err := behavior.Validate(m); err != nil {
			return nil, fmt.Errorf("lowered IR violates its contract: %w", err)
		}
	}
	return m, nil
}
func (b *builder) fresh(prefix string) string {
	prefix = sanitize(prefix)
	b.names[prefix]++
	return fmt.Sprintf("%s_%d", prefix, b.names[prefix])
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
	return b.p.Position(pos, f)
}
func (b *builder) diag(severity, code, msg string, pos token.Pos) {
	b.diagAt(severity, code, msg, b.p.Position(pos, nil))
}
func (b *builder) diagAt(severity, code, msg string, p behavior.Position) {
	b.m.Diagnostics = append(b.m.Diagnostics, diagnostic.Diagnostic{Severity: severity, Code: code, Message: msg, File: p.File, Line: p.Line})
	if severity == "error" {
		b.m.Outcome = diagnostic.Unsupported
	} else if severity == "warning" && b.m.Outcome != diagnostic.Unsupported {
		b.m.Outcome = diagnostic.Abstracted
	}
}
func (b *builder) process(f *ssa.Function, bindings map[ssa.Value]string, id string) {
	entry := b.fresh(id + "_entry")
	b.nodes[entry] = &node{}
	b.m.Processes = append(b.m.Processes, behavior.Process{ID: id, Kind: f.Name(), Entry: entry, Terminal: id + "_Done", Source: b.position(f.Pos(), f)})
	end := id + "_Done"
	b.nodes[end] = &node{}
	start := b.function(f, bindings, id, end)
	b.nodes[entry].edges = []edge{{to: start, guard: behavior.Guard{Kind: behavior.True}}}
}
func (b *builder) function(f *ssa.Function, bindings map[ssa.Value]string, process, end string) string {
	if !b.recordLoops(f) {
		return end
	}
	plan := b.plan(f)
	if b.stack[f] {
		b.diag("error", "unbounded-control", "recursive calls unsupported", f.Pos())
		return end
	}
	if plan.Cyclic && !b.proveReceiveLoops(f, plan) {
		return end
	}
	if len(f.Blocks) == 0 {
		b.diag("error", "unknown-effects", "function body unavailable: "+f.String(), f.Pos())
		return end
	}
	b.stack[f] = true
	defer delete(b.stack, f)
	fr := &frame{f: f, plan: plan, ids: map[ssa.Value]string{}, nodes: map[ssa.Instruction]string{}, selects: map[*ssa.Select]string{}, receiveStatus: map[ssa.Value]string{}, process: process, fieldStores: map[*ssa.Store]bool{}, defers: map[*ssa.Defer]deferredCall{}, cleanup: map[ssa.Instruction][]deferredCall{}}
	for v, id := range bindings {
		fr.ids[v] = id
	}
	for _, bb := range f.Blocks {
		for _, i := range bb.Instrs {
			b.checkUnsafePointer(i)
			b.prepareReceiveStatus(fr, i)
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
	b.allocateResources(fr)
	if !b.prepareDefers(fr) {
		return end
	}
	sl := plan.Slice
	for _, bb := range f.Blocks {
		for j, i := range bb.Instrs {
			src := fr.nodes[i]
			next := end
			if j+1 < len(bb.Instrs) {
				next = fr.nodes[bb.Instrs[j+1]]
			}
			e := edge{to: next, guard: behavior.Guard{Kind: behavior.True}, pos: b.position(i.Pos(), f)}
			if p, ok := plan.Discovery.Primitives[i]; ok {
				if g, ok := i.(*ssa.Go); ok {
					entry := plan.Entries[g]
					callee, bind := b.callee(fr, g, entry.Callee, g.Pos())
					if callee != nil {
						id := b.fresh(callee.Name() + "_goroutine")
						b.process(callee, bind, id)
						e.effects = []behavior.Effect{{Kind: behavior.Spawn, Process: id}}
					}
				} else {
					if !b.requireData(fr, p.Resource) {
						continue
					}
					id := b.identity(fr, p.Resource, map[ssa.Value]bool{})
					if id == "nil" && p.Kind != behavior.Send && p.Kind != behavior.Receive && p.Kind != behavior.CloseChannel {
						b.diag("error", "sync-identity", "nil synchronization pointer unsupported", i.Pos())
					}
					ef := behavior.Effect{Kind: p.Kind, Resource: id}
					if recv, ok := i.(*ssa.UnOp); ok && p.Kind == behavior.Receive {
						ef.Variable = fr.receiveStatus[recv]
					}
					if p.Kind == behavior.WaitGroupAdd {
						if !b.requireData(fr, p.Delta) {
							continue
						}
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
				if sl.Control[x] && !b.requireData(fr, x.Cond) {
					continue
				}
				for k, dst := range bb.Succs {
					g, exact := receiveStatusGuard(x.Cond, fr.receiveStatus, k == 0)
					if !exact {
						g = abstract.Predicate(x.Cond, fr.selects, k == 0)
					}
					if g.Kind == behavior.Choice && sl.Control[x] && k == 0 {
						name := x.Cond.Name()
						if c, ok := x.Cond.(*ssa.Call); ok && c.Common().StaticCallee() != nil {
							name = c.Common().StaticCallee().String()
						}
						b.m.AbstractedPredicates = append(b.m.AbstractedPredicates, behavior.Predicate{Name: name, Source: b.position(x.Cond.Pos(), f), Reason: "data result controlling concurrency is nondeterministic"})
						b.diag("warning", "abstract-predicate", "predicate "+name+" affects concurrency; modeled as nondeterministic boolean", x.Cond.Pos())
					}
					b.nodes[src].edges = append(b.nodes[src].edges, edge{to: fr.nodes[dst.Instrs[0]], guard: g, pos: b.position(x.Cond.Pos(), f), boundary: g.Kind == behavior.Choice && sl.Control[x]})
				}
				continue
			case *ssa.Jump:
				e.to = fr.nodes[bb.Succs[0].Instrs[0]]
			case *ssa.Return:
				e.to = b.cleanupChain(fr, x, end)
			case *ssa.Select:
				for k, s := range x.States {
					if !b.requireData(fr, s.Chan) {
						continue
					}
					kind := behavior.Receive
					if s.Dir == types.SendOnly {
						kind = behavior.Send
					}
					communication := behavior.Effect{Kind: kind, Resource: b.identity(fr, s.Chan, map[ssa.Value]bool{})}
					if kind == behavior.Receive {
						communication.Variable = fr.receiveStatus[x]
					}
					effects := []behavior.Effect{communication, {Kind: behavior.AssignAbstractState, Variable: fr.selects[x], Value: k}}
					if flag := fr.receiveStatus[x]; flag != "" && kind == behavior.Send {
						effects = append(effects, behavior.Effect{Kind: behavior.AssignAbstractState, Variable: flag, Value: 0})
					}
					b.nodes[src].edges = append(b.nodes[src].edges, edge{group: src, to: next, guard: behavior.Guard{Kind: behavior.True}, effects: effects, pos: b.position(s.Pos, f)})
				}
				if !x.Blocking {
					effects := []behavior.Effect{{Kind: behavior.AssignAbstractState, Variable: fr.selects[x], Value: -1}}
					if flag := fr.receiveStatus[x]; flag != "" {
						effects = append(effects, behavior.Effect{Kind: behavior.AssignAbstractState, Variable: flag, Value: 0})
					}
					b.nodes[src].edges = append(b.nodes[src].edges, edge{group: src, to: next, guard: behavior.Guard{Kind: behavior.Default, Variable: src}, effects: effects, pos: e.pos})
				}
				continue
			case *ssa.Call:
				summary := plan.Calls[x]
				if summary.Kind == effects.Trusted {
					name := summary.Callee.String()
					b.diag("warning", "trusted-call", "trusted total side-effect-free contract for "+name+"; result is abstract", x.Pos())
					b.m.Assumptions = append(b.m.Assumptions, "Trusted call: "+name)
				} else if summary.Kind == effects.Builtin { // discarded sequential computation
				} else {
					callee, bind := b.callee(fr, x, summary.Callee, x.Pos())
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
			case *ssa.Defer:
				d, ok := fr.defers[x]
				if !ok {
					b.diag("error", "unsupported-defer", "defer outside proved normal control flow unsupported", x.Pos())
					continue
				}
				e.effects = []behavior.Effect{{Kind: behavior.AssignAbstractState, Variable: d.flag, Value: 1}}
			case *ssa.RunDefers:
				e.to = b.cleanupChain(fr, x, next)
			case *ssa.UnOp:
				if x.Op == token.MUL && (inlineSync(x.Type()) || channelAggregate(x.Type())) {
					b.diag("error", "sync-copy", "loading synchronization aggregates by value unsupported", x.Pos())
				}
			case *ssa.Store:
				if relevantType(x.Val.Type()) {
					if _, ok := x.Addr.(*ssa.Alloc); !ok && !fr.fieldStores[x] {
						b.diag("error", "dynamic-topology", "storing synchronization identities through shared/indirect memory unsupported", x.Pos())
					}
				}
				if discovery.SyncType(x.Val.Type()) != "" || inlineSync(x.Val.Type()) || channelAggregate(x.Val.Type()) {
					b.diag("error", "sync-copy", "copying or resetting synchronization objects unsupported", x.Pos())
				}
			}
			b.nodes[src].edges = append(b.nodes[src].edges, e)
		}
	}
	return fr.nodes[f.Blocks[0].Instrs[0]]
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
