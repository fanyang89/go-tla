# Behavioral IR contract (version 1)

`model.json` is a language- and backend-independent guarded transition system.
Its schema is defined by `internal/behavior/model.go` and checked by
`behavior.Validate`. `behavior.Decode` reads saved executable models without SSA.
Only TLA+ is currently implemented. This contract does not expand supported Go.

## Versioning and validation

Executable models require `schemaVersion: 1`, `semantics: "communication-v1"`,
`termination: "main-return"`, and a successful extraction outcome. Unversioned
MVP artifacts must be regenerated; there is no guessed migration. Future structural
or semantic extensions require an explicit supported version/semantics contract.
Unknown fields, misspelled field casing, duplicate JSON keys, additional JSON values,
unknown guards/effects/assertions and error-bearing models are rejected.
Decoder nesting is limited to 256 levels; guard nesting to 64 levels.
Diagnostic-only unsupported artifacts remain inspectable JSON, not executable IR.

Validation checks unique identities, resource kinds, process/location references,
explicit entries and terminals, initial activation, finite variable domains,
local ownership, assignments and select/default grouping. It does not prove Go
alias safety, reachability, finite counters, or that every backend supports a model.
Backends must perform their own capability checks and refuse unsupported features.

`metadata` records producer/version, optional analyzer-binary VCS revision/dirty
state, language, toolchain and analysis options. These are descriptive, not executable
expressions. Gotla records `acyclic-static-identity`, or `finite-state-static-identity`
when receive cycles are proved, plus normalized trusted-call names. Binary provenance is not provenance of the analyzed source.
Metadata option keys may describe producer-specific configuration.

`result.json` retains its independent checker-envelope version 1, with additive
`modelSchemaVersion`, `modelSemantics` and `terminationPolicy` fields when extraction
produces a model. It fingerprints the complete saved artifacts and shares binary
provenance with the IR. A valid IR header is not a completed verification result.

## State and transitions

- Process IDs denote declared concrete instances, not unbounded templates. Each
  process has an explicit entry, terminal and location set. Initially active
  processes start at entry; others are dormant. Terminals have no outgoing edges.
- Channels start empty. An omitted/false channel `initiallyClosed` field means
  open; true means already closed before process execution. This is declarative
  IR state, not an instruction for the backend to re-run Go initialization.
  Mutexes start unlocked and WaitGroup counters zero.
  Ordinary payloads have one abstract token. Variable domains are explicit finite
  integer sets, with declared initial values. Locals belong to one process.
- A transition tests its guard at its source and atomically performs its effects
  and moves to its destination. Assignment RHS values are constants, and duplicate
  writes are invalid. There is no arbitrary backend expression text.
- `true`, `equal`, and `all` describe boolean predicates/conjunctions, optionally
  negated. Equality outside a variable domain is a valid false predicate.
  `choice` is a positive unconstrained alternative marker, not a stored boolean
  or correlated proposition; negating it is invalid. Concurrency-relevant unknown
  branches must commit to a destination before a possibly blocking operation.
- `default` positively references its transition's select group. Group identity
  is scoped by process and source location. Default is available only when none of
  that group's communication alternatives is ready. It is not a user boolean.

## Communication, waiting, and errors

- `Spawn` activates a declared dormant instance at entry. A worker's terminal
  does not terminate other processes. Main reaching its explicit terminal ends
  the whole program; blocked workers afterward are not whole-program deadlock.
- Buffered `Send` needs space; `Receive` needs data or closure. Closed buffered
  channels can drain, and closed empty receives complete. Nil sends/receives block.
  Sending to a closed channel and closing a nil/already-closed channel are errors.
- `Receive` may optionally set its `variable` destination atomically: 1 when a
  value is delivered (including draining a closed buffer), 0 for closed-and-empty.
  A blocked receive makes no assignment. The destination must belong to the
  receiving process, have exactly domain `{0,1}`, and not be written again in the
  same transition. Status reflects the receive, not a later channel-state query.
  Receives without a destination retain their original semantics. This is an
  additive schema-1/communication-v1 capability; older strict validators reject
  the formerly unused Receive variable field rather than silently dropping it.
  Consumers must still validate capabilities before executing saved IR.
- An open zero-capacity channel requires a joint send/receive between distinct
  processes. A process merely poised at an operation is not a pending offer:
  execution must first register a blocked operation. A blocking select registers
  all its alternatives together only when none is ready; a default select never
  registers. A match needs at least one registered peer. Completion cancels the
  registration and every unchosen alternative. This preserves default-before-peer
  schedules; treating all poised peers as immediately ready is incorrect.
- Select chooses nondeterministically among ready alternatives. Closed sends are
  ready but fail if chosen. Communication and the selected-index assignment are
  atomic. Case-body operations occur later, after selection. A default transition
  only selects default; it cannot bundle the body of that case.
- `Lock` waits for an unlocked mutex. `Unlock` has no goroutine ownership test;
  unlocking an unlocked mutex is an error.
- `WaitGroupAdd` changes a counter by its constant delta; `Done` subtracts one.
  Negative counters are errors; `Wait` blocks until zero. Correspondence with Go
  requires the supported single-enrollment phase, not arbitrary reuse or concurrent
  enrollment. The current counter-only backend checks this restriction separately.
- `AssignAbstractState` assigns a declared finite-domain value. `Assert` uses 0/1
  for false/true; false raises a synchronization fault. `NoSynchronizationErrors`
  requires that no such fault occurs. Unknown assertion kinds are rejected.

Waiting/registration describes observable scheduling requirements, not mandated
backend variables. No `pc`, `waiting`, Java, TLC, TLA expression or runtime action
name is stored in the IR. Alternative backends must preserve these requirements,
not infer a different rendezvous contract from generic guarded transitions.

## Current TLA capability profile

After common validation, TLA rejects shared abstract state, missing synchronization
error checking, unsafe/repeated spawn targets, and multiple scheduling effects per
transition. Assignments may accompany one scheduling effect. Cycles must cross a
status-producing Receive on every cycle: removing those edges must leave an acyclic
graph. Any repeatable Spawn, WaitGroupAdd or WaitGroupDone is rejected, even when a
guard appears to limit it. Together with bounded queues/locals and Boolean locks,
this ensures finite modeled state without trusting source-specific loop annotations.
It does not prove termination or require a channel eventually to close.
Positive WaitGroup enrollment is main-only, before worker activation or prior Wait.
Ambiguous blocking alternatives need a select group, equivalent duplicate effects,
provably disjoint finite-local guards, or an explicit branch-commit location.
Backend identifiers must fit its identifier alphabet; `Dormant` is a reserved
runtime location. Generated action-name collisions are checked. These are backend
restrictions, not hidden requirements of the generic schema validator.

## Source and naming stability

Main-module source files are module-relative, not invocation-directory-relative.
Dependency files are import-qualified; unmapped files have an explicit `unmapped/`
prefix. Package/function remain separate. These logical paths are not universally
openable by joining them to `result.json`'s invocation directory. Process sources
identify declarations; spawn transitions identify the caller's spawn operation.

Names use per-prefix collision counters. Public locations are assigned per process
from behavioral regions, not SSA instruction numbers. Actions include process,
operation and source line/column; rendezvous names use transition identities, not
global array offsets. Irrelevant straight-line SSA edits and changes in another
process therefore need not rename unchanged behavior. Arbitrary source edits,
reordered same-name instances, toolchain changes or source line movement can still
change IDs. IDs are not content hashes or persistent cross-revision trace keys.

Eight examples round-trip through saved IR into identical TLA/config. Golden tests
normalize only descriptive binary/toolchain provenance; real artifacts retain it.
The size baseline is unchanged by M3. See [the regression matrix](SEMANTIC_TEST_MATRIX.md)
and [architecture](../ARCHITECTURE.md) for implementation evidence and soundness limits.
