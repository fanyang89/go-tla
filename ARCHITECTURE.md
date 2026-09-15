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

1. `frontend.Load` loads real packages, dependencies, syntax and type information.
2. `frontend.BuildSSA` constructs Go SSA; `BuildCallGraph` builds the static call graph.
3. `effects.Analyzer` caches call-effect summaries using graph-checked SSA call
   targets. Unknown/dynamic/unsafe/recursive effects never receive pure summaries.
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
`AssignAbstractState`, and `Assert`. A select's communication and index assignment
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
  acyclic, so the total finite number of Add effects bounds counter values.
* Main return terminates all goroutines, as in Go. Only main termination enables
  terminal stuttering. While main is active, no enabled action means a TLC deadlock.
  No fairness or starvation/liveness property is asserted by the MVP.

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
channel cells, indirect shared identity stores, dynamic callbacks/interface dispatch,
returned channel topology and different-identity phis are rejected when relevant.
Synchronization objects cannot be passed by value or copied/reset. Reachable SSA
operations consuming or producing `unsafe.Pointer` (including casts and indirect
pointer cells) are rejected, as are unsafe effects in initialization and inspected
helpers. This prevents raw writes from bypassing the modeled channel/lock/counter
state and is separate from the implicit sequential-panic assumption. This is not
pointer analysis and must not be presented as one.

All reachable SSA CFG cycles and recursive calls are rejected, including ordinary
loops and repeated spawn/allocation. This is restrictive but prevents unexplained
truncation, unbounded state and silent nontermination assumptions. Capacities must
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
explicit panic/recover/defer, RWMutex, unsupported sync APIs and unsafe identities
produce errors. Unsupported results cannot be emitted as executable TLA+.

An explicit `-trust-call` contract asserts total, side-effect-free execution with no
synchronization and normal return; its result remains nondeterministic. A bodyless
call without a contract is an error even if its result is discarded. Reachable
ordinary calls with inspectable acyclic bodies are traversed, so unknown effects
cannot hide inside a sequential helper.

The supported-domain assumption excludes implicit sequential runtime panics and
resource exhaustion. This assumption is emitted as a diagnostic and model metadata.
It covers, for example, arithmetic/bounds/nil-dereference failures in otherwise
ordinary computation. It does **not** excuse synchronization failures, explicit
panic/defer/recover, unknown call effects or divergent loops. Runtime scheduling
internals, allocation failures, signal handling, unsafe/reflection semantics,
network/syscall blocking and the entire Go runtime are not modeled. Unknown calls
reaching such behavior are rejected unless a user supplies a truthful contract.

False positives arise from independent abstract predicates, erased payload values,
lost shared-memory correlations, and collapsing sequential regions. Counterexamples
must be inspected against these assumptions. General functional correctness,
property-directed refinement, arbitrary temporal properties, bounded-loop inference,
WaitGroup reuse, full alias analysis and counterexample replay remain future work.
