# Semantic regression matrix

This matrix describes executable evidence, not a proof of full Go semantics.
Paths and test names refer to the current implementation. Actual TLC tests are
optional in a plain local test run but mandatory in the
[strict gate](VERIFICATION.md#tests-and-required-ci-gate).

## Byte-search model and bounded string concatenation

`TestConstantByteSearchNative` compares 20 concrete byte-search cases with native
Go, including NUL, UTF-8 bytes, empty input and absence. `TestConstantVersionModel`
compares six version cases; `TestActualGoVersionInitializers` checks the real literal
initializers and preserves nonliteral refusal. Model tests corrupt graph, signature,
declaration and source mapping, exhaust the shared budget, and reject user-name
collisions/other unavailable bodies. Lowering rechecks cached evidence and records
the explicit standard-toolchain assumption; the full CLI regression requires it.
These tests do not prove the assembly implementation.

Three additional actual TLC cases test pure-Go string concatenation: communication,
initialization and an abstract-result counterexample. They are not whole-program
TLC proofs of imported version/strings libraries. A separate regression retains
unsupported dependency initialization despite the admitted byte-search operation.

## Literal nil-interface storage

`TestConstantNilInterfaces` permits only nil empty-interface values in concrete
SSA evaluation, checking fields, comparisons, arguments, source returns, named
interfaces and aggregate reset aliases. Boxing (including typed nil pointers),
other nonempty interfaces, assertions, external reads/writes and nonliteral inputs refuse.
The immutable reflection metadata exception has its own model and checks below.
General finite-data proofs stay unchanged. `TestActualConstantIdentConstructor`
compares actual ast.NewIdent fields against native Go; the CLI refusal regression
requires the go/doc initializer's source proof. `TestNilInterfaceProofRechecksOwnership`
redirects a private nil store to a global and requires stale-proof rejection.
Five TLC cases cover constructor communication, nil inputs, aggregate reset,
initialization and a conservative abstract-result counterexample.

## Immediate program exit

`tests/exit_ir_test.go` uses an independent IR producer and eight actual TLC runs:
main/worker exit, skipped cleanup and exit within a cleanup helper pass; prior and
competing synchronization faults remain counterexamples; blocking before exit and
removing a worker's Exit instruction deadlock. JSON round trips, standalone-effect,
terminal-destination and unused-field refusals exercise the IR boundary.
`internal/lowering/exit_test.go` separately checks actual `os.Exit` source lowering,
helper/worker control, retained argument effects, skipped cleanup, current graph,
signature and slice corruption, and refusal to drop `os` initialization. User-named
Exit functions and pure-call trust cannot impersonate/override program termination.
The real CLI refusal regression requires its source-located Exit instruction.
These are not full-program TLC checks of a program importing `os`.

## Production-component bootstrap

Immutable reflection metadata has separate native/structural coverage:
`TestActualASTEdgeMetadata` compares every one of the 104 real AST edge initializers
with native field types/indices. Direct-query tests compare field presence, names,
package paths, tags, offsets, indices and anonymity. TypeFor wrappers and ABI
extraction bodies/graph edges are freshly checked; consumed ABI/method/argument
mutations refuse. Promoted fields, nil/invalid receivers, other methods and reflective
values remain refused. `TestReflectionMetadataPreservesInitializationRefusal` ensures
other reflection initialization is not omitted. These are not new reflection-import
TLC passes: standard runtime ABI and reflection correctness are explicit assumptions.

Rotated range-over-len SSA has five additional pinned TLC cases: locked computation,
empty input, initializer, abstract-result deadlock and blocking argument evaluation.
Fresh entry/step/comparison/bound mutations, hidden cycles/stores and cached-proof
consumption refuse; constant expansion refusals remain unchanged. Actual Identifier
and decimalDigits source bodies pass finite-data proof. Production lexical scanning
is compared with its old regex grammars through fixed/randomized and fuzz tests;
protocol classification and source-trace CLI tests continue to run.

The conditional startup GOMAXPROCS profile adds six semantic and two CLI/TLC cases:
capacity two passes while capacity one deadlocks for the same source, plus local,
closed-initial, prior-blocking and closed-send behavior. Native capacities at 1/2/3
agree. Current source/signature/graph/slice/profile and hidden-setter mutations refuse;
other initialization is not exempted. Profiled full-self analysis still refuses.

Builtin output now refuses instead of being silently elided. Two more semantic TLC
cases admit a shadowed source-function name and a literal-proved zero-output execution.
Executed output, builtin trust overrides and stale cached summaries are refusal tests.

Current total: **273 semantic TLC cases and 20 TLC CLI cases**, plus separate
unsupported full-CLI and reentrant-logger regressions. Counts in prerequisite sections below record their
respective historical increments.

`TestCheckProductionBoundedLog` checks the actual `internal/boundedlog.Writer`
used by `checker.Run`: two concurrent callers pass under a returning sink/bounded
notification environment; removing its actual deferred Unlock deadlocks; a zero
budget with blocking cancellation also deadlocks. A gated output receiver passes
when a third worker releases both writes, and deadlocks without that releaser.
`TestCheckReentrantLoggerBoundary` instead requires an unsupported result and no
checker execution for a self-captured writer; it is not a TLC deadlock proof.
All five models contain nonempty
production synchronization and explicit data abstractions. The test requires real
TLC evidence, source-located production effects/dispatches, no trust flags and a
mutation isolated from the working tree. Native policy/concurrency tests and race
checking remain separate. See [BOOTSTRAP.md](BOOTSTRAP.md) for exact scope/results.

`TestCheckProductionSourceCapture` adds the actual frontend collector's two-caller
pass and a missing-Unlock mutation deadlock. These models have no abstracted control
predicates but do not verify map contents or byte-copy semantics. Native tests cover
nil/empty inputs, independent copied storage, replacement and concurrent captures;
frontend/race regressions exercise its real integration. The production leaf uses
slices.Clone instead of bytes.Clone; initialization checks are unchanged. See the
bootstrap document for that dependency change's scope and evidence.

## Full-self prerequisite: scalar-array ranges

Seven `TestTLCScalarArrayRanges` cases cover topology, workers, zero iterations,
saturation, return, abstract index guards and defer scope. Native differential
execution covers array snapshots, single/zero-count evaluation, per-iteration
captures, return and generated-name collisions. Source-position/input-preservation
and old-language refusal tests protect normalization. The actual artifact file table
is a fixed array; full CLI refusal regression requires its two five-iteration proofs.

## Full-self prerequisite: read-only value copies

Five `TestTLCFiniteDataCopies` cases cover slice/pointer copies, local header reset,
abstract-result synchronization errors and blocked argument evaluation. Refusal tests
reject borrowed storage writes and forbidden type graphs. Fresh-consumption mutation
redirects a private copy into a global. The actual Model.Statistics source is proved,
and full CLI refusal regression requires its proof record without claiming self-TLC.

## Full-self prerequisite: open global channels

Seven `TestTLCOpenGlobalChannels` cases cover buffered/rendezvous operations, runtime
close, empty/full deadlocks and semaphore release/missing-release. Refusal tests
retain initializer I/O, conditional close, dynamic capacity and rebinding/address
escape. Fresh-proof tests corrupt capacity within/outside the supported range and
hide a rebind from the call graph. Full CLI refusal regression requires the three
actual x/tools I/O semaphore initial states and source proof records.

## Full-self prerequisite: owned reference storage

Three additional `TestTLCConstantDataCalls` cases cover zero slice fields, owned
pointer fields and pointer arrays. Refusal tests retain external-pointer coverage;
unit tests cover private cycles, nil dereferences and nested forbidden type graphs.
`TestActualConstantFloatConstructor` checks the installed go/constant.newFloat source
and compares the resulting precision with native Go. Full CLI regression requires
this proof while retaining the unsupported whole-self boundary.

## Full-self prerequisite: literal-input data calls

`TestTLCConstantDataCalls` adds nine actual TLC cases: initialization validation,
private helpers/views/copies, checked loops, runtime calls, abstract result errors,
unchosen effects and zero-iteration effects. Refusal/consumption tests reject changed
arguments/body/graph/slice evidence, executed panic, shared/unknown memory and budget
exhaustion. Evaluator tests cover value copies, alias-preserving aggregate reset,
overlapping copy, simultaneous phi values and integer/bounds failures.
`TestActualBase64EncodingConstantData` loads actual source, compares tables with native
Go and rejects invalid alphabet mutations. Full CLI refusal regression now requires
the two real initializer proofs but still requires no executable whole-self model.

## Data-table prerequisite: private arrays

`TestTLCPrivateArrayData` adds seven real TLC cases: literals, nested arrays, indexed
range and classic-counter fills, abstract-value errors, blocked argument evaluation
and an empty classic loop. Private constructor and finite-data tests reject shared
writes, publication, slicing/address escape, reference/synchronization elements,
counter changes and nonportable bounds. The classic synchronization-loop refusal
remains in `TestFiniteLoopRefusals`. Full CLI self-analysis remains unsupported.

## Full-self prerequisite: pre-closed global channels

`TestTLCClosedGlobalChannels` checks seven saved-IR cases: repeated/concurrent receives,
buffered empty status, range completion, select/default and send/close errors.
`TestTLCClosedGlobalStateMutation` proves changing the initial closed flag back to
false exposes deadlock. Source refusals reject missing/conditional/repeated close,
initialization sends, address escape/rebinding, factories and helper indirection.
Lowering tests invalidate graph, body, capacity and order, and ensure removing a
function from the graph cannot conceal a rebind in its SSA body. The actual whole-CLI
probe consumes this proof for context.closedchan but remains unsupported.

## Full-self prerequisite: finite data computation

`TestTLCFiniteDataComputations` adds eight actual TLC cases: critical-section
computation, abstract predicate deadlock/synchronization errors, initializer helpers,
blocked argument evaluation, deferred computation, wrappers and private value copies.
`TestFiniteDataProof` and `TestFiniteDataRefusals` reject counter resets/overflow forms,
changed bounds, progress-bypassing cycles, external writes and hidden effects.
Graph/slice and budget tests enforce consumed proof contracts.
`TestActualModelHasErrorsFiniteData` loads the actual production source. The full CLI
probe consumes this proof and toolinfo.fromBuildInfo's proof but remains unsupported.

## Bound method forwarding proofs

Nine native/TLC comparisons cover Once callback method values on channels, objects
and mutexes; blocked and invalid operations; deferred method values; arguments; and
receiver snapshots across reassignment. Frontend/lowering mutations reject stale
wrapper graphs, receiver/argument forwarding, body/signature, capture arity/slice,
creation ordering and exhausted inventories. Interface/generic/result-bearing wrappers,
resource copying and output remain refusals. The old bound-Unlock refusal is now an
analyzed native/TLC synchronization-error case, not a silent success.

Both actual godebug.Value and IncNonDefault Once sites have eligible API/callback
contracts; whole-self analysis still does not reach/consume them. No returned context
cancellation model or whole-self executable proof is established.

## Source-bound Once and finite shared state

Eleven native/TLC cases cover first/repeated/competing Do calls, global and field
identities, distinct callbacks, callback defers, blocked and reentrant callbacks,
completion-before-return waiting, and a no-Once double-close mutant. Two independent
IR/TLC cases verify nonzero shared-state initialization, cross-process publication,
and a missing-publication deadlock. Read/lock and store/unlock boundaries are asserted
in lowered IR rather than relying only on a passing schedule.

Mutations reject standard graph/body/branch/store/source changes, nil callbacks,
closure reordering and missing retained receiver/capture operands. Reset/copy, output,
unknown callbacks, panic and initializer calls cannot bypass the model using trust.
The actual internal/godebug.Setting.Value call has an eligible operation contract;
IncNonDefault is additionally covered by the later bound-method proof above.
Neither is a full-self proof.

## Channel-object receiver boxes

Eight paired native/TLC cases cover buffered/spawned sends, boxed and direct deferred
methods, distinct object channels, blocked and closed-channel sends, and deferred
close. The previous channel-box/deferred-channel-box refusals are now positive cases.
Box argument/storage/return escapes, late initialization, receiver mutation and
channel replacement still refuse executable emission. Unit mutations cover absent
graphs, changed current argument/return uses, missing definitions and the 4096-step
inventory budget, including deliberately stale Referrers caches. Whole-self analysis
still rejects the returned object in os/signal.NotifyContext and remains unsupported.

## Exact deferred interface/field dispatch

`tests/deferred_dispatch_test.go` checks ten cases with both native Go execution and
pinned TLC: empty/source interface methods, receiver snapshots, double close,
per-invocation function-field captures, interface fields, blocked cleanup, correct
and reversed LIFO order, and blocked argument evaluation. The formerly rejected
empty-interface defer is now one of these positive checks. Ordinary graph/receiver/
immutable-field binding is reused; no new callable-only or boxed-channel-object
identity rule is implied. Returned closures, ambiguous/nil targets, mutable fields,
primitive interface dispatch, output and panic remain refusal tests.

Unit mutations remove graph edges, retained operands/receivers, or the current field
initializer; synthetic wrappers and foreign defer stacks also refuse. Current full-self
analysis still refuses returned context cancellation and does not emit executable TLA+.

## Full-self prerequisite: static deferred helper bodies

`TestTLCDeferredHelpers` adds twelve checks for direct source functions/methods and
literal closures: LIFO close/send order, helper blocking, nested cleanup, invalid
unlock, captured argument identities, conditional registration and multiple returns.
`TestDeferredHelperRefusals` retains dynamic/recursive/unavailable/panicking and
unstable-capture boundaries. `TestDeferredHelperConsumesGraphAndSlice` corrupts cached
graph/slice facts to ensure they cannot be ignored. The former empty-closure defer
refusal is now a positive TLC case; unknown dynamic defers remain rejected.
These are prerequisites, not a complete CLI self-proof; see
[SELF_BOOTSTRAP_GOAL.md](SELF_BOOTSTRAP_GOAL.md).

## Bootstrap prerequisite: immutable callable fields

`TestTLCImmutableCallableFields` adds ten actual TLC checks for interface fields,
named/captured callbacks, blocked cleanup, synchronization faults, resource receiver
aliasing, per-invocation captures, pure calls and combined writer/callback fields.
The total is **119 semantic TLC cases**, with **11** unchanged TLC CLI cases and a
separate unsupported CLI self-analysis test.

`TestImmutableCallableFieldRefusals` and `TestImmutableInterfaceFieldRefusals` reject
unproved initialization/aliasing, callee mutations, escapes, wrappers and unstable
captures. `TestCallableFieldProofRechecksGraphAndOrigins`,
`TestCallableFieldProofBudgetRefusesLongAliasChain` and
`TestCallableFieldBindingAndSliceAreConsumed` enforce completed-graph agreement,
proof budgets, allocation-frame bindings and dependency-slice consumption.

## Bootstrap prerequisite: exact local interface dispatch

`TestTLCDirectInterfaceCalls` adds ten real TLC cases: relay argument binding,
shared-mutex deadlock, distinct mutexes, cross-worker unlock, invalid unlock,
joined workers, a named-channel receiver, interface widening, pure receiver proof
retention and deferred cleanup blocked inside a method. The total is **109**
semantic TLC cases, plus **11** unchanged TLC CLI cases.

`TestDirectInterfaceRefusals` exercises unresolved receiver sources, adaptations,
copies/escapes, recursive/unknown effects, unsupported defers and primitive trust
bypass. `TestDirectInterfaceGraphAndReceiverAgreement`,
`TestUnknownInterfaceParameterHasNoGraphTarget` and
`TestInterfaceProofConsumesGraphAndSlice` verify exact target/receiver agreement
and fail-closed pass consumption. Self-analysis of the full CLI remains unsupported.

## Untouched zero synchronization storage

Four TLC cases cover ordinary constructor fields beside zero Mutex/WaitGroup/Once
storage and an untouched mutex array. Native Go checks independent zero-state owners,
Once completion counts, wait groups and constant-string/shared-empty behavior. Type
budget/depth, publication, field-address corruption, current use inventories and
synchronization copy/address/operation refusals are covered. Constructors do not
confer returned synchronization identities. Actual godebug.New and constant.MakeString
bodies qualify; ten real initializer refusals disappear (94 to 84), not a self pass.

## Channel return identity proofs

Nine paired native/TLC cases cover channel factories/forwarding, separate invocation
identities, nil and directional channels, goroutine forwarding, common-identity
branches, deferred close errors and blocked cleanup. Mutation tests remove return
operands/roots/store roots, replace store values, duplicate stores, add recovery-block
effects and exhaust the inventory; stale referrer lists do not replace current uses.
Different identities, tuple/object returns, captured result mutation, output,
recursion and explicit recovery remain refusals. Actual context emptyCtx and
withoutCancelCtx Done nil identities are checked against native behavior; these are
not yet consumed by the whole-self attempt, which still returns unsupported / 5.

## Private reference constructor extension

Six TLC cases cover private pointer/interface/container-header storage, including
three formerly refused non-nil boxes. Native Go checks preserved pointer/interface,
slice and map aliases, callback non-execution during construction and type metadata
identity. No header grants ownership of its referent. Borrowed writes, callbacks,
publication, synchronization value copies and returned channel identity remain refused.
Current-use tests clear stale referrers, inject publication and exhaust the inventory.
Actual go/types.NewPointer/NewTuple and x/tools SSA newVar/anonVar bodies are checked;
seven corresponding self-initializer refusals disappear (101 to 94). Existing consumed
literal/nil-interface proofs remain required, rather than bypassed by broader purity.
This remains an unsupported self attempt, not an executable bootstrap proof.

## Bootstrap prerequisite: private data constructors

`TestPrivateDataConstructorSummaries` and `TestStandardErrorConstructorIsPrivateData`
exercise fresh scalar/value-struct allocation stores, return-only escape and the
installed `errors.New` source. Shared writes, map mutations, mutating/I/O builtins,
unsafe conversions, reference/synchronization fields, phis and escaping addresses
remain outside the proof. This is not a package initializer allowlist.

`TestPrivateDataInitializerRefusals` checks that unsafe helper effects still prevent
executable models. `TestTLCPrivateDataInitializers` adds four actual TLC cases:
record/rendezvous and scalar/buffered pass; a boxed-error initializer does not hide
a deadlock; nested data construction does not hide a double close. The semantic
total is now **99**; the **11** TLC CLI cases are unchanged. The separate
`TestCheckSelfAnalysisBoundary` still expects unsupported, not a self-verification pass.

## Frontend, abstraction, and artifact contracts

| Contract | Evidence | Expected result |
|---|---|---|
| Typed primitive recognition and call graph | `internal/discovery/primitives_test.go`: `TestRecognizeTypedSSAPrimitives` | Recognize real SSA operations and include the worker in the static call graph |
| Conservative backward control/data slice | `internal/slice/concurrency_test.go`: `TestBackwardControlAndData` | Preserve control and dependencies leading to communication |
| Cyclic control detection | Same file: `TestCycleDetection` | Reachable cycle recognized, not silently truncated |
| Constant vs unknown predicates | `internal/abstract/predicates_test.go`: `TestPredicateConstantAndNondeterminism` | Constants retained; unknown predicate represented as choice |
| Trusted returns and static closures | `tests/analysis_test.go`: `TestTrustedResultAndStaticClosure` | Unknown result remains abstract and send survives; static closure supported |
| Sequential region collapse | Same file: `TestExamplesAndSnapshots` | Unbuffered and sequential examples each retain five IR transitions |
| Reproducibility | Same file: `TestDiagnosticsAndIRDeterministic` | Two independent analyses of each of eight examples yield equal IR/diagnostics/TLA/config |
| Artifact and size baselines | Same file: `TestExamplesAndSnapshots`; `internal/behavior/statistics_test.go` | IR/TLA snapshots match; eight-example size snapshot matches; counts reflect IR, not TLC states |
| Backend-only IR consumption | `internal/tla/generator_test.go`: `TestGenerateBackendIndependentModel` | Generate from a hand-built behavioral model without SSA |
| Unsupported backend features | Same file: `TestRejectUnknownIREffect`, `TestRejectMultipleSchedulingEffects` | Refuse unknown effects and multiple scheduling effects in a transition |
| CLI outcomes/artifact safety | `cmd/gotla/main_test.go`: `TestCLIAnalyzeInspectAndStaleArtifacts` | Success writes outputs/stats/hint; inspect does not print runnable hint; unsupported removes stale executable models |
| Copyable TLC invocation | `cmd/gotla/hint_test.go` | JAR lookup, absolute paths, quotes, and setup guidance are correct |
| Missing required checker | `tests/main_test.go`: `TestTLCPrerequisites` and `TestMain` | Optional absent JAR permits local skips; required absent/invalid path fails |

The backward slice is intentionally conservative; its tests do not claim precise
postdominator analysis or general pointer analysis. M3 adds the following contracts:

| Contract | Evidence | Expected result |
|---|---|---|
| Consumed graph/effect/data facts | `internal/effects/summary_test.go`, `internal/lowering/plan_test.go` | Missing graph or retained data fails closed; unsafe/dynamic calls cannot become pure |
| Generic versioned IR validation | `internal/behavior/validate_test.go` | Malformed references, domains, effects, guards, versions, duplicate/case-aliased JSON keys and excessive nesting rejected |
| Separate backend capabilities | `internal/tla/generator_test.go` | Generic-valid unsupported features rejected by TLA; explicit terminals/activation consumed; ambiguous blocking choices refused |
| Saved IR completeness | `tests/ir_contract_test.go`: `TestVersionedExamplesRoundTrip` | All eight models round-trip into identical TLA/config without SSA |
| Stable names and source paths | Same file | Sequential edits do not rename behavior; other-process changes do not renumber actions; same-basename files remain distinct; process/spawn positions are accurate |
| Provenance integration | `internal/toolinfo/info_test.go`, `cmd/gotla/check_test.go` | IR and checker envelope agree on model contract and binary provenance |

See [the IR contract](BEHAVIOR_IR.md). These structural checks are not a proof of
frontend alias correctness or equivalence with arbitrary Go.

## Actual TLC outcomes: 33 checks

All cases are in [tests/tlc_test.go](../tests/tlc_test.go). Tests assert the expected
message and whether the checker exits successfully; output substring checks alone
are not treated as sufficient evidence.

### Eight examples: `TestTLCExamples`

| Examples | Expected TLC result |
|---|---|
| `unbuffered`, `buffered`, `select`, `mutex`, `waitgroup`, `sequential` | Completed with no error |
| `deadlock` | Deadlock |
| `unknown` (explicit `strings.HasPrefix` contract) | Possible deadlock through abstract predicate |

### Nineteen edge cases: `TestTLCBlockingAndErrors`

| Cases | Expected TLC result |
|---|---|
| `nil-receive`, `nil-send`, `buffer-full` | Deadlock |
| `nil-close`, `closed-send`, `double-close` | Synchronization-error invariant violation |
| `closed-drain` | No error; closed receive/drain remains possible |
| `main-exit` | No error; main return ends all workers |
| `select-ready-no-default`, `select-nil-default`, `select-exact-dispatch` | No error; readiness and dispatch exclude invalid alternatives |
| `select-closed-send` | Synchronization-error invariant violation |
| `select-rendezvous` | No error |
| `select-self-no-rendezvous` | Deadlock; one process cannot match itself |
| `mutex-lock-twice` | Deadlock |
| `mutex-invalid-unlock` | Synchronization-error invariant violation |
| `mutex-cross-process-unlock` | No error; Go mutexes are not goroutine-owned |
| `waitgroup-negative` | Synchronization-error invariant violation |
| `waitgroup-blocked` | Deadlock |

### One branch commitment case: `TestTLCAbstractBranchCommitsBeforeBlocking`

An unknown main predicate can commit to an unmatched send instead of returning.
TLC must find a deadlock; an always-enabled skip must not hide the blocked branch.

### Five registered-offer cases: `TestTLCRegisteredOffersAndSelectDefault`

| Case | Expected TLC result |
|---|---|
| `default-before-poised-send` | Deadlock reachable when worker chooses default before parent registers send |
| `default-before-blocking-select` | Deadlock reachable before peer registers its select alternatives |
| `blocking-select-pair` | No error; blocking multi-case selects can rendezvous |
| `two-default-selects-never-rendezvous` | No error; no invented rendezvous between two nonblocking polls |
| `registered-select-closed` | No error; a registered receive can complete after close |

## M4 increment: six inline-field TLC checks

`tests/fields_test.go` adds `TestTLCStaticSyncFields`: shared-lock deadlock,
distinct inline fields, cross-worker unlock, invalid unlock, WaitGroup completion,
and missing completion. Together with the original suite these are 39 semantic
TLC cases, separate from CLI integration tests.

`TestStaticSyncFields` checks local/global/nested objects, fresh allocations,
receiver/subobject calls, stable aliases and captures. `TestSyncFieldRefusalBoundaries`
and `TestSyncAggregateInitializerCopyRejected` retain copy/reset, value receiver,
ambiguous/nil/future identities, interface, channel/pointer field and container
refusals. Immutable channel-field initialization is covered by the next increment;
restricted defers and proved integer ranges are covered below.

## M4 increment: seven immutable-channel-field TLC checks

`tests/channel_fields_test.go` adds `TestTLCImmutableChannelFields`: rendezvous,
nil-send deadlock, nil-close/closed-send faults, shared/distinct channel identities,
and default-before-peer deadlock. The total semantic suite is now 46 cases, plus
four CLI integration cases at that increment.

Extraction tests cover local literals/assignments, nested fields, nil, direction
conversions, receiver/bound-method calls, captures and independent invocation
contexts. Refusal tests cover future/conditional stores, capture/spawn/call before
initialization, duplicate/callee/closure writes, escaped field addresses, copies and
unproved pointer cells. Ordering cases require the initialization diagnostic, not
merely an unrelated unsupported error. Former blanket field rejection tests now
reject mutable fields; accepted forms have explicit positive and TLC coverage.

## M4 increment: eleven restricted-defer TLC checks

`tests/defers_test.go` adds `TestTLCRestrictedDefers`: Unlock/Done success, invalid
unlock/negative counter, blocked return expression, conditional registration,
multiple returns, unselected select case, receiver capture, nested/repeated frames,
and cleanup through synchronization fields. The semantic total is now 57 TLC
cases, separate from four CLI integration cases.

`TestDeferIRPreservesConditionalLIFO` explores every exact-flag path of a conditional,
multiple-return model and checks registration order, reverse cleanup order and empty
pending state at terminal. `TestRestrictedDeferRefusals` covers unsupported targets,
method values, nil identity, panic/recover, loops, initializers and the explicit
64-site budget. `internal/discovery/defers_test.go` checks that registration is a
root, not an immediate unlock, and rejects an alternate SSA defer stack.

## M4 increment: nine proved integer-range TLC checks

`tests/loops_test.go` adds `TestTLCProvedIntegerRanges`: finite worker creation,
distinct channel allocations, double-close synchronization error, too many sends,
deferred cleanup not running per iteration, distinct deferred mutex identities,
missing completion, return stopping subsequent iterations, and zero iterations.
The semantic total is now 66 TLC cases, separate from four CLI integration cases.

Extraction tests cover constant/imported bounds, blank bindings, zero/negative
counts, helper/closure/initializer proof propagation, resource identity and original
source locations. Refusal tests cover live indices, nonconstant/classic/channel
ranges, nested loops, break/continue, iteration/byte limits and defer-site overflow
after expansion. Unused unsupported functions do not reject main. Frontend tests
check source immutability and proof ownership after a preceding expanded function.

## M5 prerequisite: thirteen exact receive-status TLC checks

`TestTLCReceiveStatus` checks closed-empty false, buffered draining true then false,
status capture before subsequent close, unbuffered rendezvous true, closure wakeup
false, nil/open-empty blocking, a fault gated by closed status, and buffered,
rendezvous, closure-wakeup, send and default select cases. It rejects accidental
predicate abstraction in these direct-status cases. The semantic total is now 79
TLC cases, separate from six CLI integration cases.

`TestReceiveStatusIRContract` verifies saved-IR round trips and rejection of missing,
non-Boolean/shared/foreign-process destinations, duplicate writes in either order and send bindings.
A lowering regression keeps status returned through a call conservatively abstract.
This prerequisite alone did not validate channel ranges or a pipeline.

## M5: fourteen finite-state receive-cycle TLC checks

`TestTLCReceiveLoops` checks closed-empty and buffered draining, unbuffered streams,
missing close, nil input, forwarding, wrong close, explicit status exits, pending
cleanup during blocking, conditional cleanup across loops, mutex operations,
sequential loops and 32 deliveries (beyond the integer expansion limit). Cycles
remain in IR; no receive count is guessed. The semantic TLC total is now 93.

`TestReceiveLoopRefusals` covers pure/ignored-status cycles, wrong exits, repeated
allocation/spawn/defer/stores/counter changes/helper calls, nested receive cycles,
bypassing subcycles and changing channel identity. SCC unit tests cover empty,
single, sequential and nested cases. `TestReceiveLoopIRFiniteStateContract` checks
saved-model round trips and backend refusal of pure subcycles and repeatable
spawn/counter updates independently of the frontend.

`TestCheckPipelineComponents` adds three real CLI cases: proper close passes,
missing output close deadlocks, and early close violates synchronization safety.
Exact topology/model-size and source/provenance checks accompany actual outcomes.
There are now nine CLI integration cases, separate from the 93 semantic cases.
Native tests verify the correct component with channel capacities 0, 1 and 2.

## M5 completion: captured inline-sync objects and mutex component

`TestTLCCapturedSyncObjects` adds two actual TLC cases: cross-goroutine access
preserves a shared object's identity, and distinct captured objects do not collapse.
`TestCapturedSyncObjectRefusals` covers future/conditional initialization, reassignment,
closure/nested-closure writes, escaped cells, nil access and whole-object copies.
The semantic TLC total is now 95.

`TestCheckCounterComponents` adds correct/missing-unlock CLI variants, including
exact topology/size, result/artifact provenance and source candidates. There are now
11 CLI cases (four workflow plus seven component variants). Native Go tests cover
the correct counter, with full race testing separate from TLC's communication model.
[ACCEPTANCE.md](ACCEPTANCE.md) records local completion and the hosted-CI limitation.

## Earlier M5 component acceptance

`cmd/gotla/components_test.go` runs the finite batch worker-pool library harnesses
through `check` and real TLC: correct joins pass; missing Done deadlocks. Both
assert artifact/provenance validation, exact model-size baselines, zero abstracted
predicates/custom trusted calls, and source candidates for the counterexample.
These add two CLI integration cases (six total), separate from the semantic TLC
cases. The correct library also has a native Go result test. See
[component evidence](COMPONENTS.md) for boundaries, measurements and pending work.

## Check workflow regressions (M2)

The original 33 checks above remain unchanged. Additional workflow tests cover:

| Contract | Evidence | Expected result |
|---|---|---|
| Real end-to-end CLI/TLC outcomes | `cmd/gotla/check_test.go`: `TestCheckTLCIntegration` | Real pass/deadlock/invariant outcomes have matching gotla statuses/exits, artifact hashes, provenance, and source candidates; tiny deadline is incomplete |
| Checker outcome authentication | `internal/checker/protocol_test.go` | Framed completion/outcome and exit code must agree; prose-only, conflicting, truncated, and unknown-error output never passes |
| Resource and subprocess handling | `internal/checker/runner_test.go` | Shell-free Java stand-in exercises timeout, log limit, memory exhaustion, tool failure, and cancellation; logs remain bounded and workspace is cleaned |
| Setup and options | Same file | Missing JAR/Java and invalid resource options fail explicitly |
| Failed extraction and stale results | `cmd/gotla/check_test.go` | Load failure, unsupported source, cancelled analysis, and missing tool replace/invalidate old success; `analyze` also removes previous checker artifacts |
| Writer isolation and partial output | `internal/artifact/directory_test.go` and CLI lock tests | Concurrent writers cannot modify current results; unknown directory contents are not recursively removed; temporary writes are cleaned |
| Result separation | `check_test.go`, `protocol_test.go` | Extraction precision is independent of checker status; exit mapping is stable; argument errors do not start/overwrite a run |

Fake-Java subprocess tests are control-path tests, not actual TLC semantic evidence.
Source candidates are intentionally not a complete trace replay/causality proof.

## Fail-closed extraction

All following groups are in [tests/analysis_test.go](../tests/analysis_test.go).
They assert unsupported extraction and failure to emit executable TLA+.

| Test group | Cases |
|---|---|
| `TestUnsupportedIsNeverExecutable` | Dynamic capacity; unproved loops/recursion; unavailable effects (including discarded results); dynamic calls; channel/goroutine/unknown initialization; general defer/panic; mutable channel fields; changing captures; Mutex copying; WaitGroup reuse/concurrent Add; RWMutex |
| `TestSynchronizationIdentityRequiresDominatingInitialization` | Future-store send/close, store after spawn/capture, conditional initialization; a dominating-store positive control remains supported |
| `TestUnsafeSynchronizationMutationRejected` | Mutex/WaitGroup state mutation, casts, helper and initializer unsafe effects; an unused unsafe function positive control remains supported |

## Known coverage limits and next additions

- These are small finite examples, not an exhaustive equivalence proof or a
  differential reference interpreter. Successful tests do not prove global soundness.
- CI provisions a pinned toolchain/checker; other platforms, Go versions, and TLC
  releases do not yet have a compatibility matrix.
- Model size baselines are IR counts. TLC state-space counts and performance are
  logged, not frozen across platforms/versions or enforced as performance guarantees.
- A completed host CI run and branch-protection configuration are external evidence;
  workflow presence alone proves neither.
- Complete trace/source replay and broader third-party effect-summary coverage
  remain follow-up work. M3's malformed-IR/source-path tests do not replace these.
- M4 must add semantics-specific positive, deadlocking/error, and rejection cases
  before admitting more field/defer forms or broader finite loops. Existing
  rejection tests must not simply be deleted without replacement evidence.
