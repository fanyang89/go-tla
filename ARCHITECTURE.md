# Architecture and semantic contract

## Pipeline

```
Go source -> packages/types -> Go SSA -> static call graph + dependency slice
          -> atomic behavioral regions -> Concurrent Behavioral IR -> TLA+ -> TLC
```

There is no LLVM or TLA-specific representation in the frontend or behavioral IR.
The only backend implemented is TLA+. A future backend can consume `model.json`
without loading Go packages.

The passes are explicit functions/packages:

1. `frontend.Load` loads and type-checks the original packages. A restricted typed
   integer-range proof may produce in-memory Go overlays; these are reloaded and
   type-checked before SSA construction. On-disk source is never rewritten.
   Concurrent ParseFile calls record copied input through `sourcecapture.Collector`;
   parsing follows outside its lock, and normalization consumes its map after loading.
   Its finite bootstrap harness checks the capture lock protocol, not loader scheduling
   or map/byte contents (see [BOOTSTRAP.md](docs/BOOTSTRAP.md)).
2. `frontend.BuildSSA` constructs Go SSA; `BuildCallGraph` builds the static call graph
   and refines exact local interface boxes to their declared concrete methods. Receiver
   adaptation, promotion and unknown interface sources are not guessed. Call-target
   queries require agreement between the SSA proof and an explicit graph edge.
   A subsequent immutable-field pass follows unambiguous direct object parameters,
   proves one dominating allocation-frame store and records interface/function-field
   targets. Complete-graph rechecking, bounded proof traversal and per-invocation
   receiver/capture bindings prevent a candidate edge from becoming an alias guess.
3. `effects.Analyzer` caches call-effect summaries using graph-checked SSA call
   targets. Unresolved/unsafe/recursive effects never receive pure summaries.
   A separate finite-data proof checks monotone length-bounded SSA cycles, all
   transitive call bodies, data-only types and private writes. Rotated range-over-len
   requires a guarded zero entry, matching tested +1 back edges, header dominance
   and no hidden cycle; constant-range expansion limits are unchanged. Production
   lexical validators use this proof, not regex or trusted-call exemptions. Lowering freshly
   rechecks this proof and caller operands before eliding computation; returned
   data remains abstract and argument evaluation is preserved.
   Literal-input invocation proofs separately evaluate a bounded selected SSA path
   with owned private memory. They recheck arguments, graph/body and caller slices;
   unchosen effects are excluded by concrete facts, not callee-wide trust. No host
   application code is executed. Returned values still remain abstract in IR.
   Explicit operation models are reported separately from source proofs: the
   standard-toolchain byte-string-search primitive computes a bounded exact result
   and records its assembly-correctness assumption. Immutable TypeFor metadata
   has a checked standard-source/graph/SSA extraction chain, plus an explicit ABI
   correctness assumption. Only known metadata receivers admit standard Elem and
   direct FieldByName contracts; method ownership/signatures and computed field
   layout are checked. Standard reflection correctness is separately assumed.
   These tokens are not application values or IR resources. Unknown bodies remain
   refused; no reflective values, application methods or promoted search are run.
   Explicit trusted contracts remain visible; purity guides slicing, not permission
   to bypass reachable-body validation.
4. `discovery.Scan` returns typed primitives, roots and goroutine sites. Cached
   `lowering.functionPlan` consumes these facts and call summaries; contexts bind
   concrete resources and instantiate each discovered process entry.
5. `slice.FromRoots` consumes these roots and retains predecessor control and SSA
   operand dependencies. Its roots/control determine branch commits; retained data
   gates capacity, synchronization-resource and control extraction. This is still
   conservative predecessor slicing, not precise postdominator analysis.
6. `lowering/identity.go` separates static allocation/binding and dominating-store
   identity checks; `initializers.go` checks initialization and summary assumptions.
   An optional startup-GOMAXPROCS contract inventories all current SSA references,
   rejects mutation/escape/trust, and binds checked zero queries to exact direct
   channel capacities. Target/environment conditions are explicit provenance and
   assumptions, not executable Go-specific IR syntax or a host-setting inference.
   Unknown initialization and other environment effects remain independently checked.
   `abstract.Predicate` preserves constants/select dispatch and abstracts other
   conditions. Unknown effects are still inspected or rejected.
7. `lowering.regions` epsilon-closes sequential instruction paths, combining guards
   until the next behavioral effect. The temporary SSA-linked instruction graph is
   not the output IR. Local arithmetic/straight-line helpers do not become actions.
8. `lowering.Lower` emits processes, locations, guards, effects, source metadata,
   assumptions and precision diagnostics. WaitGroup phase validation rejects uses
   requiring a more elaborate waiter-generation model.
9. `behavior.Validate` checks the versioned generic contract; `behavior.Decode`
   validates saved JSON independently of Go. `tla.Validate` adds backend capability
   restrictions; `Generate` emits runtime state,
   transition actions, rendezvous actions, `Init`, `Next`, `Spec`, and configuration.
10. The CLI writes diagnostics and artifacts, or displays the inspection summary.
    `check` additionally invokes `internal/checker` to run explicitly provisioned
    TLC with a deadline, heap/log limits, framed result parsing, and saved evidence.
    `internal/artifact` serializes writers and invalidates old outputs before
    analysis. Neither layer changes model semantics or downloads executables.

Unit tests exercise recognition, call-graph inclusion, slicing and abstraction;
integration tests exercise the complete pipeline. Snapshots cover channel and
select IR/TLA and the eight examples' model sizes. Repeated extraction checks IR,
diagnostics, TLA, and config determinism. Actual TLC tests validate semantic outcomes,
not just syntax; they are optional locally but mandatory in the pinned verification
gate. See the [semantic test matrix](docs/SEMANTIC_TEST_MATRIX.md) and
[verification guide](docs/VERIFICATION.md).

The checker result and behavioral IR have independent version-1 contracts. The
result records the model contract identifiers and fingerprints the saved artifacts;
both share analyzer-binary provenance. Source summaries map simple trace `pc` locations to
candidate IR transitions, not a complete counterexample decoder. Timeout applies
to checker execution; package loading is cancellable, while SSA/lowering currently
check cancellation only at pass boundaries. See the
[verification contract](docs/VERIFICATION.md) for output ownership, incomplete
results, resource-limit scope, and exit codes.

## What survives slicing

Behavioral roots are spawn, channel allocation/communication/close, select,
mutex lock/unlock and WaitGroup operations. Calls that may reach roots, their
controlling branches, and data needed to resolve synchronization identities or
capacities are relevant. Unknown values controlling communication are not dropped.

The communication-only value domain has one payload token, represented as `0` by
the TLA backend. Arithmetic, payload transformations, unrelated local memory and
pure straight-line computation disappear. Shared ordinary memory values used by
conditions become nondeterministic; correlations between reads, channel payloads,
receive `ok` results and sequential expressions are not retained. This may permit
infeasible communication traces, but does not assume a particular unknown result.
Identity/capacity uncertainty is different: it is rejected rather than collapsed
into an arbitrary concrete resource.

Branching with no behavioral operations still preserves the possibility of normal
return. Unknown branches can choose either edge. Constants and SSA's select-result
index remain exact. The compiler-generated unreachable blocking-select panic is
retained as an assertion guarded by impossible select indices, not confused with
an explicit source panic.

## Behavioral IR

`internal/behavior` defines a guarded transition system: processes, finite local
variables, channels, mutexes, WaitGroups, initial active processes, transitions,
assertions and source positions (`package`, `function`, module-relative or
import-qualified file, line, column). Explicit terminals remove magic-name semantics.
Per-prefix IDs and post-region location naming avoid global SSA-instruction offsets.
Process sources identify declarations; spawn sources identify callers. See the
[versioned IR contract](docs/BEHAVIOR_IR.md) for semantics, decoder rules, backend
capabilities, provenance and naming guarantees. Full trace decoding is not implemented.

Guards are structured `true`, `choice`, `equal`, conjunction, and select-default
predicates, not expression strings in TLA syntax. Effects include `Spawn`, `Send`,
`Receive`, `CloseChannel`, `Lock`, `Unlock`, `WaitGroupAdd/Done/Wait`,
`AssignAbstractState`, `Assert`, and whole-program `Exit`. A select's communication and index assignment
are one atomic transition. `choiceGroup` associates communication alternatives with
their default guard. The backend rejects unknown effects/guards instead of ignoring
extensions. General shared-state and arbitrary assertion backends are not implemented.

## Atomic regions and synchronization

* Local sequential work is epsilon-closed between scheduling points. Guards for
  a region are conjoined. Finishing a process is an explicit transition.
* A concurrency-relevant unknown branch has an explicit nondeterministic commit
  transition before entering its chosen region. Merging `Send` and `Skip` as
  competing readiness-guarded actions would incorrectly hide the choice to block.
* Every static goroutine starts dormant and is enabled only by its spawn action.
  Each synchronous invocation has separate SSA binding context. No runtime process
  pool is silently truncated.
* Channel objects are initialized before execution; identity is static and SSA
  dominance/spawn ordering prevents access before construction. Payloads are
  singleton tokens, so buffered channels are finite queues bounded by static capacity.
* Buffered sends require room; receives require data or closure. Receive on a
  closed empty channel succeeds. Sending on or closing a closed channel triggers
  the synchronization-error invariant. Closing nil also triggers it; nil send and
  receive block forever.
* An open unbuffered channel transfers only in a joint action advancing two
  distinct processes. Neither side can complete alone. Before blocking, an operation
  executes a `Register` action that records its pending offer. A blocking select
  registers all alternatives together only when none is ready; a default select
  never registers. Rendezvous requires at least one registered process; merely
  being poised at an operation is not a pending offer. Completion clears the
  registration, including all unchosen select alternatives. This backend runtime
  refinement of IR communication effects preserves default-before-peer schedules.
* Select chooses nondeterministically among ready alternatives. Default is enabled
  only when none is ready, including registered matching rendezvous partners, not unscheduled peers. Closed-channel
  sends count as ready but fail, matching Go. Dispatch into the chosen case is exact.
* Mutex state is a boolean, not an owner: Go permits unlock by another goroutine.
  Lock requires availability; unlocking an unlocked mutex is an error.
* WaitGroups use integer counters, block Wait until zero, and detect negative
  counters. Positive Add is supported only in main before any goroutine spawn or
  prior Wait on the group. Concurrent enrollment and reuse are rejected, so a
  counter suffices without pretending to model waiter generations. All control is
  acyclic at counter-changing sites, so the finite number of Add effects bounds counters.
* Main return terminates all goroutines, as in Go. Explicit `Exit` from any process
  also terminates the whole program immediately, bypassing all cleanup. Direct
  ordinary-process `os.Exit(int)` calls consume current signature/graph/slice facts;
  status values are abstract, not argument effects. Dependency initialization is
  not exempted. Either form of whole-program termination enables terminal
  stuttering. While main is active, no enabled action means a TLC deadlock.
  No fairness or starvation/liveness property is asserted by the MVP.

## Exact receive completion status

`lowering/receive_status.go` retains used SSA comma-ok/select status extracts as
finite process-local flags. Direct tests, negation and Boolean-constant comparisons
become exact equality guards; Phi/call/memory propagation keeps the existing
explicit conservative abstraction. Each invocation owns fresh flags.

The generic Receive effect can bind a local `{0,1}` destination. Validation checks
ownership/domain and duplicate writes, independently of TLA. The backend assigns
1 for rendezvous or a nonempty pre-receive buffer, 0 for closed-empty completion;
nil/open-empty blocking assigns nothing. Select index and status update together.
No payload values or backend expression strings are added to the IR. Exact status
is a prerequisite for close-driven control. Cyclic SSA has the separate proof below.

## Finite-state close-driven receive control

`slice.CyclicComponents` finds reachable SSA SCCs. Channel-range syntax is only a
frontend candidate, not a proof or expansion instruction. Lowering consumes each
cyclic component: exactly one status-producing receive, a dominating header whose
exact closed-empty branch exits the SCC, and no subcycle when the header is removed.
The input identity is evaluated outside the SCC and undergoes ordinary identity
validation. Repeating instructions are whitelisted: scalar computation, sends,
stable field access and direct close/Lock/Unlock. Calls, stores, topology creation,
select, counter updates and deferred registration/draining may not repeat.

Receive cycles survive region construction as actual IR back edges. There is no
user-supplied receive count and no assumption that a sender closes. Deferred sites
outside SCCs remain single-execution; their may-pending union/kill analysis now
uses a fixed point, while registration order still follows the acyclic sites.
This preserves conditional cleanup across normal returns after reception blocks.

The TLA backend independently rejects repeatable spawn/counter changes and any
cycle surviving removal of status-producing Receive edges. Existing bounded queue,
local-domain, Boolean lock/status and spawn-activation contracts then imply finite
modeled state. This is not a termination, fairness, concrete-memory or delivery-count
proof. Frontend profiles are provenance only and cannot bypass this backend check.
Pure loops, nested receive SCCs and unsupported repeated effects still fail closed.

## Proved integer-range normalization

`frontend/loops.go` recognizes `for range N` (or a blank iteration binding) when
Go type information proves N is an integer constant. This first form has no live
iteration variable, nested loops, labels or branch statements in the body. It
applies only to main-module function declarations and their nested function literals.
Nonpositive constants have zero iterations. Positive bounds must be at most 16;
expanded replacement text is limited to 256 KiB per file. Over-budget/unsupported
ranges remain unexpanded and are rejected when their owning function is reached.
Other cycles need the independent receive proof above or fail lowering. These are expansion limits, not user
assumptions that an unproved loop stops early.

Each iteration becomes a distinct lexical block, not a helper function. Fresh Go
syntax is re-type-checked and translated to fresh SSA values, preserving allocation,
spawn and local-variable identity. Return exits the original function; deferred
calls remain on that function's stack, rather than running at iteration boundaries.
A zero-trip body stays under `if false` to retain type checking and import usage.
A discarded reference to the constant bound also preserves imports used only there;
package initialization is never removed by this normalization.

Generated line directives preserve original file/line/column attribution, including
subsequent functions. Files already using line directives are not normalized.
Proof/rejection records are consumed by lowering and purity checks, with reached
proofs emitted as informational metadata. Unused unsupported functions do not fail
an entry point. Backends still receive only the independent behavioral IR;
no Go loop syntax, source overlays or runtime loop counter enters it.

## Restricted deferred cleanup

Direct deferred Mutex.Unlock and WaitGroup.Done are separate from immediate
primitive execution. Discovery retains registration and `RunDefers` as slice roots;
call summaries validate exact targets. `lowering/defers.go` captures resource
identities at registration sites and gives each invocation fresh finite local flags.
Source helpers reuse the ordinary callee binder: direct calls, local boxed interfaces
and immutable callable fields require current graph/receiver/initializer/slice proofs.
Returned closures and arbitrary dynamic dispatch have no new exemption.

Registration/drain sites remain outside cyclic SCCs and can run at most once.
Their reverse-postorder extends executable registration order; reversing the
registered subset gives LIFO without abstracting order. A fixed-point may-pending
analysis selects sites at each `RunDefers`/normal return, while exact flags preserve
conditional registration. Cleanup tests each flag, performs one primitive and clears
it atomically, or clears it and enters the actual source helper body. Earlier cleanup
waits for that body's normal return. Callee invocations cannot drain caller flags. Return expressions and
blocking operations before cleanup retain their original SSA order.

This emits only generic assignments, guards and existing synchronization effects;
there is no Go-specific defer stack in behavioral IR or TLA runtime. More than 64
sites per invocation, unproved targets or synthetic wrappers, alternate SSA defer
stacks and initializer defers are rejected. Explicit panic/recover remain unsupported and
implicit sequential panics remain excluded: normal-return cleanup is not unwinding.

## Identity and supported domain

Precise identity supports channel allocations, direct parameters, constant nil,
direction conversions, same-identity phis, and static closure captures through
single-assignment nonescaping cells. Their unique initialization must dominate
every load and closure capture, including instruction order inside a basic block.
A later or conditionally executed store cannot retroactively define a captured
channel. Capturing before initialization is rejected even if a later invocation
might make it safe; closure writes and multiple stores are also rejected. Mutex/WaitGroup allocations and direct global
objects have stable identity. Inline Mutex/WaitGroup fields now use a static local
allocation or direct global object plus nested value-field path. Direct pointer
receivers/parameters and stable closure captures preserve that object identity.
Synchronization state remains zero-initialized; whole-aggregate loads/copies,
value receivers/arguments, resets and initializer copies are rejected. Local channel
fields now bind to an existing channel (or nil) after an allocation-frame proof:
all field-address uses are inspected, writes are unique, and initialization dominates
all reads and object/subobject calls, captures and spawns. Field stores seed the data
slice; only individually proved stores can be discarded. Callees may read, but not
initialize or mutate, these bindings. No field-specific state or syntax enters the IR.
Global/mutable channel fields, pointer fields and unproved object-pointer cells,
containers, changing captured
channel cells, indirect shared identity stores, unproved callbacks/interface dispatch,
returned channel topology and different-identity phis are rejected when relevant.
Synchronization objects cannot be passed by value or copied/reset. Reachable SSA
operations consuming or producing `unsafe.Pointer` (including casts and indirect
pointer cells) are rejected, as are unsafe effects in initialization and inspected
helpers. This prevents raw writes from bypassing the modeled channel/lock/counter
state and is separate from the implicit sequential-panic assumption. This is not
pointer analysis and must not be presented as one.

After integer-range normalization, remaining SSA cycles require the close-driven
receive SCC proof. Recursive calls, classic indexed loops, dynamic integer ranges,
nested receive cycles and repeated effects outside the whitelist remain unsupported. Repeated
spawn/allocation in an accepted expansion has distinct static identities; there is
no unexplained truncation or silent nontermination assumption. Capacities must
be static integers in 0..1024; Add deltas must be static in -1024..1024.

Package initialization is checked, not silently erased. Application and dependency
initializers with concurrency, unknown effects or cycles are rejected. Narrow
standard-library environment summaries cover initialization of `sync`, `sync/atomic`
and their actual GOROOT dependencies. Explicitly trusted `strings` or `math` calls
also permit their GOROOT initialization dependency summaries. Every summarized
package name is recorded. This exemption never applies to third-party packages or
ordinary calls to atomic APIs; it is not a general standard-library purity rule.

## Unknown behavior, outcomes, and soundness limits

Outcomes are `precisely-modeled`, `conservatively-abstracted`, and `unsupported`.
“Precisely modeled” means precise **within the declared communication-only domain
and environment assumptions**, not all Go semantics. Payload data is outside this
domain. Unknown control predicates and trusted return values produce warnings and
an abstracted outcome. Unresolved side effects/nontermination, unavailable bodies,
explicit panic/recover, unsupported defer forms, RWMutex, unsupported sync APIs and unsafe identities
produce errors. Unsupported results cannot be emitted as executable TLA+.

An explicit `-trust-call` contract asserts total, side-effect-free execution with no
synchronization and normal return; its result remains nondeterministic. A bodyless
call without a contract is an error even if its result is discarded. Reachable
ordinary calls with inspectable supported bodies are traversed, so unknown effects
cannot hide inside a sequential helper.

The supported-domain assumption excludes implicit sequential runtime panics and
resource exhaustion. This assumption is emitted as a diagnostic and model metadata.
It covers, for example, arithmetic/bounds/nil-dereference failures in otherwise
ordinary computation. It does **not** excuse synchronization failures, explicit
panic/recover, unsupported defer forms, unknown call effects or divergent loops. Runtime scheduling
internals, allocation failures, signal handling, unsafe/reflection semantics,
network/syscall blocking and the entire Go runtime are not modeled. Unknown calls
reaching such behavior are rejected unless a user supplies a truthful contract.

False positives arise from independent abstract predicates, erased payload values,
lost shared-memory correlations, and collapsing sequential regions. Counterexamples
must be inspected against these assumptions. General functional correctness,
property-directed refinement, arbitrary temporal properties, broader loop-bound inference,
WaitGroup reuse, full alias analysis and counterexample replay remain future work.
