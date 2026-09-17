package frontend

import (
	"go/token"
	"go/types"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/ssa"
)

// FieldCallProof connects one immutable allocation-frame store to a call. It is
// a source proof, not a concrete runtime resource binding; lowering supplies that
// binding separately for each invocation of the allocation's function.
type FieldCallProof struct {
	Target      *ssa.Function
	Object      ssa.Value
	Field       int
	Initializer *ssa.Store
	Value       ssa.Value
	Receiver    ssa.Value
}

// CallableField checks the source proof against the completed graph. No cached
// field proof is trusted after an edge/alias change.
func (p *Program) CallableField(site ssa.CallInstruction) *FieldCallProof {
	if p.Calls == nil {
		return nil
	}
	proof := p.fieldCall(site)
	if proof == nil || !hasCallEdge(p.Calls.Nodes[site.Parent()], site, proof.Target) {
		return nil
	}
	return proof
}

// CallableInitializer permits lowering to bind an initializer in its own frame,
// before another function reads it. This never invents identities for returned
// objects, callee-initialized fields or opaque values.
func (p *Program) CallableInitializer(store *ssa.Store) (ssa.Value, *ssa.Alloc) {
	if p.Calls == nil {
		return nil, nil
	}
	field, ok := store.Addr.(*ssa.FieldAddr)
	if !ok {
		return nil, nil
	}
	root, ok := field.X.(*ssa.Alloc)
	if !ok || root.Parent() != store.Parent() || !callableType(store.Val.Type()) {
		return nil, nil
	}
	if p.immutableField(root, field.Field) != store {
		return nil, nil
	}
	v := callableValue(store.Val)
	switch v.(type) {
	case *ssa.MakeInterface, *ssa.Function, *ssa.MakeClosure:
		return v, root
	default:
		return nil, nil
	}
}

func callableType(t types.Type) bool {
	switch t.Underlying().(type) {
	case *types.Interface, *types.Signature:
		return true
	}
	return false
}

func callableValue(v ssa.Value) ssa.Value {
	for {
		switch x := v.(type) {
		case *ssa.ChangeInterface:
			v = x.X
		case *ssa.ChangeType:
			v = x.X
		default:
			return v
		}
	}
}

func (p *Program) fieldCall(site ssa.CallInstruction) *FieldCallProof {
	load, ok := callableValue(site.Common().Value).(*ssa.UnOp)
	if !ok || load.Op != token.MUL || !callableType(load.Type()) {
		return nil
	}
	field, ok := load.X.(*ssa.FieldAddr)
	if !ok {
		return nil
	}
	root := p.origins().origin(field.X)
	if root == nil {
		return nil
	}
	store := p.immutableField(root, field.Field)
	if store == nil {
		return nil
	}
	proof := &FieldCallProof{Object: field.X, Field: field.Field, Initializer: store, Value: callableValue(store.Val)}
	if site.Common().IsInvoke() {
		box, ok := proof.Value.(*ssa.MakeInterface)
		if !ok {
			return nil
		}
		proof.Target = p.concreteMethod(site.Common().Method, box.X)
		proof.Receiver = box.X
	} else {
		switch value := proof.Value.(type) {
		case *ssa.Function:
			proof.Target = value
		case *ssa.MakeClosure:
			proof.Target, _ = value.Fn.(*ssa.Function)
		}
	}
	if proof.Target == nil {
		return nil
	}
	if !site.Common().IsInvoke() && (proof.Target.Synthetic != "" || proof.Target.Signature.Recv() != nil) {
		// Named functions and source closures only; bound-method wrappers need
		// their own capture/primitive contract, not a callable-field shortcut.
		return nil
	}
	if pkg := proof.Target.Pkg; pkg != nil && (pkg.Pkg.Path() == "sync" || pkg.Pkg.Path() == "sync/atomic") {
		return nil
	}
	return proof
}

// Do not iterate growing candidate sets: collect against the completed direct
// graph, then add the field edges as a separate stage. Full-graph queries recheck
// origins and can reject a proof invalidated by an additional incoming edge.
func (p *Program) refineCallableFields() {
	type candidate struct {
		site   ssa.CallInstruction
		target *ssa.Function
	}
	var candidates []candidate
	for f := range p.Calls.Nodes {
		if f == nil {
			continue
		}
		for _, bb := range f.Blocks {
			for _, i := range bb.Instrs {
				if site, ok := i.(ssa.CallInstruction); ok {
					if proof := p.fieldCall(site); proof != nil {
						candidates = append(candidates, candidate{site, proof.Target})
					}
				}
			}
		}
	}
	for _, c := range candidates {
		caller := p.Calls.CreateNode(c.site.Parent())
		if !hasCallEdge(caller, c.site, c.target) {
			callgraph.AddEdge(caller, c.site, p.Calls.CreateNode(c.target))
		}
	}
}

// directArguments deliberately excludes field dispatch, preventing recursive
// alias justifications through the very field being proved.
func (p *Program) directArguments(site ssa.CallInstruction) (*ssa.Function, []ssa.Value) {
	f := site.Common().StaticCallee()
	args := site.Common().Args
	if f == nil {
		var receiver ssa.Value
		f, receiver = p.directInvoke(site)
		if f != nil {
			args = append([]ssa.Value{receiver}, args...)
		}
	}
	if f == nil || !hasCallEdge(p.Calls.Nodes[site.Parent()], site, f) {
		return nil, nil
	}
	return f, args
}

// Only direct object allocations and unambiguous pointer parameters qualify.
// Every known incoming edge must agree, not just the caller currently lowered.
const maxCallableProofSteps = 1024

type objectOrigins struct {
	p         *Program
	visiting  map[ssa.Value]bool
	memo      map[ssa.Value]*ssa.Alloc
	remaining int
}

func (p *Program) origins() *objectOrigins {
	return &objectOrigins{p: p, visiting: map[ssa.Value]bool{}, memo: map[ssa.Value]*ssa.Alloc{}, remaining: maxCallableProofSteps}
}

func (o *objectOrigins) origin(v ssa.Value) (result *ssa.Alloc) {
	o.remaining--
	if o.remaining < 0 || o.visiting[v] {
		return nil
	}
	if known, ok := o.memo[v]; ok {
		return known
	}
	o.visiting[v] = true
	defer func() { delete(o.visiting, v); o.memo[v] = result }()
	p := o.p
	switch x := v.(type) {
	case *ssa.Alloc:
		pointer, ok := x.Type().Underlying().(*types.Pointer)
		if !ok {
			return nil
		}
		if _, ok := pointer.Elem().Underlying().(*types.Struct); ok {
			return x
		}
	case *ssa.Parameter:
		node := p.Calls.Nodes[x.Parent()]
		if node == nil || len(node.In) == 0 {
			return nil
		}
		index := -1
		for i, param := range x.Parent().Params {
			if param == x {
				index = i
			}
		}
		var root *ssa.Alloc
		for _, edge := range node.In {
			if edge.Site == nil {
				return nil
			}
			target, args := p.directArguments(edge.Site)
			if target != x.Parent() || index < 0 || index >= len(args) {
				return nil
			}
			next := o.origin(args[index])
			if next == nil || root != nil && next != root {
				return nil
			}
			root = next
		}
		return root
	}
	return nil
}

func (p *Program) immutableField(root *ssa.Alloc, index int) *ssa.Store {
	proof := fieldIsolation{program: p, root: root, index: index, seen: map[ssa.Value]bool{}, stores: map[*ssa.Store]bool{}, origins: p.origins(), remaining: maxCallableProofSteps}
	if !proof.object(root) || len(proof.stores) != 1 {
		return nil
	}
	for store := range proof.stores {
		field, ok := store.Addr.(*ssa.FieldAddr)
		if !ok || field.X != root || store.Parent() != root.Parent() {
			return nil
		}
		for _, use := range proof.before {
			if !dominatesInstruction(store, use) {
				return nil
			}
		}
		return store
	}
	return nil
}

type fieldIsolation struct {
	program   *Program
	root      *ssa.Alloc
	index     int
	seen      map[ssa.Value]bool
	stores    map[*ssa.Store]bool
	before    []ssa.Instruction
	origins   *objectOrigins
	remaining int
}

func (s *fieldIsolation) object(v ssa.Value) bool {
	if s.seen[v] {
		return true
	}
	s.seen[v] = true
	if s.origins.origin(v) != s.root {
		return false
	}
	refs := v.Referrers()
	if refs == nil {
		return false
	}
	for _, use := range *refs {
		s.remaining--
		if s.remaining < 0 {
			return false
		}
		switch x := use.(type) {
		case *ssa.FieldAddr:
			if x.Field == s.index && !s.field(x) {
				return false
			}
		case *ssa.Call, *ssa.Go:
			site := use.(ssa.CallInstruction)
			f, args := s.program.directArguments(site)
			if f == nil || len(f.Blocks) == 0 {
				return false
			}
			if use.Parent() == s.root.Parent() {
				s.before = append(s.before, use)
			}
			for i, arg := range args {
				if arg == v && (i >= len(f.Params) || !s.object(f.Params[i])) {
					return false
				}
			}
		case *ssa.DebugRef:
		default:
			// Includes object copies/resets, pointer cells, boxing, closure
			// captures, returned objects, pointer conversions and phi aliases.
			return false
		}
	}
	return true
}

func (s *fieldIsolation) field(addr *ssa.FieldAddr) bool {
	refs := addr.Referrers()
	if refs == nil {
		return false
	}
	for _, use := range *refs {
		s.remaining--
		if s.remaining < 0 {
			return false
		}
		switch x := use.(type) {
		case *ssa.Store:
			if x.Addr != addr {
				return false
			}
			s.stores[x] = true
		case *ssa.UnOp:
			if x.Op != token.MUL {
				return false
			}
			if x.Parent() == s.root.Parent() {
				s.before = append(s.before, x)
			}
		case *ssa.DebugRef:
		default:
			return false
		}
	}
	return true
}

func dominatesInstruction(before, after ssa.Instruction) bool {
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
