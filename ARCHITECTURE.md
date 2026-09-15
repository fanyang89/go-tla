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
3. `discovery.Recognize` / `IsRoot` identify typed concurrency primitives.
4. `discovery.Entries` identifies static goroutine sites; lowering instantiates one
   process per reachable call-context/spawn site. Synchronous calls are inlined
   into the calling process, not mistaken for new processes.
5. `slice.Compute` marks concurrency roots, all CFG predecessors controlling their
   reachability, and backwards SSA operand dependencies. This intentionally keeps
   more control than a precise postdominator slice would. Direct-call body checks
   recursively account for callee effects; unresolved dynamic calls are not omitted.
6. `abstract.Predicate` retains constant booleans and exact select-index dispatch;
   other conditions become independent nondeterministic choices. Lowering checks
   calls, resource identities, initializer assumptions and unsupported constructs.
7. `lowering.regions` epsilon-closes sequential instruction paths, combining guards
   until the next behavioral effect. The temporary SSA-linked instruction graph is
   not the output IR. Local arithmetic/straight-line helpers do not become actions.
8. `lowering.Lower` emits processes, locations, guards, effects, source metadata,
   assumptions and precision diagnostics. WaitGroup phase validation rejects uses
   requiring a more elaborate waiter-generation model.
9. `tla.Validate` checks supported IR features; `Generate` emits runtime state,
   transition actions, rendezvous actions, `Init`, `Next`, `Spec`, and configuration.
10. The CLI writes diagnostics and artifacts, or displays the inspection summary.
    TLC is deliberately a separately provisioned executable, not a hidden download.

Unit tests exercise recognition, call-graph inclusion, slicing and abstraction;
integration tests exercise the complete pipeline. Snapshots cover channel and
select IR/TLA. Optional actual TLC tests validate semantic outcomes, not just syntax.

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
assertions and source positions (`package`, `function`, file basename, line,
column). Transition IDs use function/process and source-line names plus stable
collision-disambiguating suffixes. File basenames plus package/function avoid
machine-specific absolute paths; full counterexample decoding is not implemented.

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
  distinct processes. Neither side can complete alone. Select send/receive cases
  participate in these same rendezvous actions.
* Select chooses nondeterministically among ready alternatives. Default is enabled
  only when none is ready, including matching rendezvous partners. Closed-channel
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
single-assignment nonescaping cells. Mutex/WaitGroup allocations and direct global
objects have stable identity. Struct fields, containers, changing captured channel
cells, indirect shared identity stores, dynamic callbacks/interface dispatch,
returned channel topology and different-identity phis are rejected when relevant.
Synchronization objects cannot be passed by value or copied/reset. This is not
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
