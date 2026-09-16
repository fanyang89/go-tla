# Semantic regression matrix

This matrix describes executable evidence, not a proof of full Go semantics.
Paths and test names refer to the current implementation. Actual TLC tests are
optional in a plain local test run but mandatory in the
[strict gate](VERIFICATION.md#tests-and-required-ci-gate).

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

## M5 component acceptance (in progress)

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
