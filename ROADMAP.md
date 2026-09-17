# Roadmap to a basically usable gotla

## Status and document ownership

The implementation has local M1–M5 acceptance evidence for small, explicitly scoped
Go concurrency components, **not** arbitrary Go verification. The hosted-CI verification
gate passed for `f19df4d` (run `35173519428`); future revisions need their own passes. This roadmap records the target and evidence; only the
supported-domain guide defines the admitted language.

- [README.md](README.md): current installation and command usage.
- [ARCHITECTURE.md](ARCHITECTURE.md): current semantics, assumptions, and limitations.
- This document: target scope, acceptance gates, priorities, and progress.

Current companion guides:

- [docs/SUPPORTED_GO.md](docs/SUPPORTED_GO.md): supported, abstracted, and rejected Go patterns.
- [docs/VERIFICATION.md](docs/VERIFICATION.md): results, assumptions, false positives, and incomplete checks.
- [docs/SEMANTIC_TEST_MATRIX.md](docs/SEMANTIC_TEST_MATRIX.md): regression evidence and coverage gaps.
- [docs/ACCEPTANCE.md](docs/ACCEPTANCE.md): local/hosted acceptance matrix and release boundaries.

## 1. What “basically usable” means

### Intended user and workflow

A Go developer can analyze a small concurrent component in its existing module,
with an explicit main-package analysis harness when necessary. The developer
should be able to:

1. Identify the supported input scope before running analysis.
2. Inspect the extracted processes, resources, transitions, and abstractions.
3. Run extraction and TLC through one command, without composing Java commands.
4. Distinguish a completed check from a counterexample, unsupported input, or an
   incomplete/failed checking run.
5. Locate the relevant source operations and inspect the assumptions behind a
   finding, without reverse-engineering anonymous TLA+ actions.
6. Reproduce the check using saved artifacts and recorded tool/configuration data.

The user supplies ordinary Go source, not a behavioral DSL. A harness may choose
finite inputs, construct resources, and invoke the component, but should not
replace its concurrency implementation. A harness result applies to that harness
and its declared environment, not every possible caller of a library.

### Verification target

The first usable version checks **whole-program deadlock and modeled
synchronization errors** in a finite communication-oriented abstraction.

- Preserve blocking sends/receives, select readiness/default behavior, lock
  availability, WaitGroup blocking, spawning, closing, and normal main termination.
- Preserve a branch's commitment to a potentially blocking operation; a currently
  enabled alternative must not erase the possibility of blocking.
- Report a completed successful run as “no modeled violation found under the
  recorded assumptions,” not “the Go program is safe.”
- An abstract counterexample may be infeasible in Go. Present it as a potential
  problem until the abstraction and source have been inspected.
- A timeout, resource limit, cancellation, unsupported construct, or tool error is
  not a successful verification result.
- Main return terminates the program. A worker left blocked after main returns is
  not, by itself, a whole-program deadlock. Goroutine leaks and starvation require
  separate future verification targets.

### Required scope for the target version

| Area | Target boundary |
|---|---|
| Entry | Exactly one main package; documented explicit harnesses for library components |
| Processes | Statically identifiable goroutine instances; finite worker creation with proved bounds |
| Channels | Static identities/capacities, nil/closed behavior, buffered queues, rendezvous, select/default |
| Synchronization | Mutex and single-phase WaitGroup; no implicit ownership restriction on Go mutex unlock |
| Common Go patterns | Static synchronization fields and direct receiver calls; restricted synchronization defers; proved finite loops |
| Computation | Remove irrelevant sequential work; abstract concurrency-controlling values with visible diagnostics |
| Calls | Inspect supported bodies or require explicit, recorded summaries/contracts; unknown effects cannot be silently discarded |
| Output | Independent behavioral IR, readable TLA+, diagnostics, raw checker log, and machine-readable check result |
| Usability | One-command check, bounded resource use, stable result categories, actionable source references |

This table describes the **target**, not today's support. Inline Mutex/WaitGroup
fields, local immutable channel fields and direct pointer receivers have an initial
implementation, as do direct deferred Unlock/Done on normal-return paths. Mutable/global
channel fields, pointer fields and general defers remain unavailable. The first proved
finite loop form is a constant integer range without a live index (up to 16 iterations).
Close-driven receive cycles instead require a finite-state SCC proof, not a trip
count. Immutable pointer-cell captures of inline-sync structs without channel fields
are supported under the dominating-store rules. The `check` workflow is implemented
within this restricted frontend scope.

## 2. Current baseline

Implemented:

- Typed Go package loading, Go SSA, static call graph, concurrency discovery,
  conservative dependency slicing, atomic regions, and an independent behavioral IR.
- Static goroutine instances and direct synchronous call lowering.
- Channels, select/default, Mutex, and restricted single-phase WaitGroup semantics.
- Explicit unknown-branch commitment and registered unbuffered offers, preserving
  default-before-peer and blocked-branch schedules.
- Conservative rejection of ambiguous synchronization identities, unsafe pointer
  operations, and unresolved call effects; recorded trusted-call assumptions.
- `inspect` summaries, `analyze` output (`model.json`, `model.tla`, `model.cfg`), and
  `check` execution with bounded Java/TLC, raw logs, versioned result JSON, and
  best-effort source candidates.
- Stable check outcome/exit categories, per-directory output locks, early stale
  artifact invalidation, and nonfinal `running` markers.
- A copyable TLC command after successful analysis, using `TLC_JAR` or a local JAR.
- Eight examples, unit/rejection/snapshot tests, and 33 actual TLC checks (optional
  in ordinary local tests, required in the strict CI gate).
- IR-size summaries and eight-example size/determinism baselines.
- An explicit checksum-pinned TLC provisioning script and required verification
  workflow; missing/mismatched checker and snapshot regeneration fail the gate.

Current restrictions include unproved CFG cycles and recursion, mutable/global
channel fields, pointer fields and synchronization containers, general aliasing,
general defers/recover/explicit panic, and dynamic
process/resource topology. TLC is explicitly provisioned and can run through `check` or manually. Plain
`go test ./...` skips actual TLC tests unless `TLC_JAR` is configured.

The current supported-domain assumption excludes implicit sequential runtime
panics and resource exhaustion. It does not exclude modeled synchronization
errors. See [ARCHITECTURE.md](ARCHITECTURE.md) for the authoritative full contract.

## 3. Non-negotiable semantic boundaries

1. **Trustworthiness before coverage.** If identity, control, or effects cannot be
   handled safely, emit unsupported diagnostics rather than a plausible model.
2. **No silent bounds.** Initially accept loops only when a finite iteration count
   can be proved for the supported form. Reject an over-budget expansion rather
   than truncate its behaviors. Any future user-supplied bound must be described
   as bounded checking, not conservative whole-program coverage.
3. **No automatic purity for unknown calls.** A trusted call is an explicit user
   contract about termination, normal return, and absence of shared/concurrent
   effects. Record it; do not add broad library allowlists merely to pass examples.
4. **Separate abstraction from failure.** Keep precise-within-domain,
   conservatively-abstracted, and unsupported extraction outcomes distinct from
   checker outcomes such as deadlock or incomplete checking.
5. **Keep the IR backend-independent.** Go-specific analysis stays in the frontend;
   TLA+ runtime encodings stay in the backend. Document generic communication
   semantics so a later backend does not have to infer them from generated TLA+.
6. **Do not redefine support through documentation.** New supported syntax requires
   implemented semantics, rejection tests, and checker regressions before moving
   from planned to supported.

## 4. Definition of done

The basically usable milestone is complete only when all of these are satisfied:

- [x] M1–M5 local acceptance gates pass, with evidence recorded per change; the
      hosted gate also passed for `f19df4d` (run `35173519428`).
- [x] A user can build the CLI, provision the documented TLC version, and run a
      component check using documented non-interactive commands.
- [x] Successful, deadlocking, synchronization-error, unsupported, tool-failure,
      and incomplete runs have distinguishable CLI and machine-readable results.
- [x] Checks record assumptions, trusted contracts, tool/configuration information,
      artifact locations, and model size; stale artifacts cannot masquerade as a
      new successful result.
- [x] Three production-style component examples retain their concurrency source
      and have small explicit harnesses where needed: a finite worker pool, a
      close-driven pipeline, and a struct-based synchronized component.
- [x] Each component has a correct version and an intentionally faulty version;
      tests assert the expected checker outcome, not just successful TLA parsing.
- [x] Supported loop forms, defers, and field identities each have positive,
      negative, and blocking/synchronization-error regression coverage.
- [x] CI runs real TLC and fails if the required checker is unavailable; plain unit
      test success alone cannot satisfy the release gate.
- [x] Documentation lists remaining restrictions and explains that completed
      abstract checking is neither arbitrary Go correctness nor leak/starvation
      verification.

Model size, state count, elapsed time, and peak resource use should be measured on
a recorded reference environment. Choose numerical budgets from those baselines;
do not invent performance guarantees before measuring the target examples.

## 5. Delivery milestones

| Milestone | Status | Deliverables | Acceptance gate |
|---|---|---|---|
| Scope definition | Done (documentation) | Target users/workflow, supported-domain boundary, non-goals, definition of done | This roadmap separates current capabilities from planned ones |
| M1: Quality baseline | Implemented; local gate passed | Semantic test matrix; Go/vet/snapshot/TLC CI; pinned checker provisioning; model statistics | Reproducible models and expected outcomes; strict gate cannot silently skip TLC; hosted run 35173519428 passed |
| M2: Check workflow | Implemented; local gate passed | `gotla check`; JAR/timeout/memory/worker/log settings; raw log and structured result; source candidates; stale-output handling | Real TLC pass/deadlock/invariant CLI tests and bounded subprocess/failed-output tests pass; hosted run 35173519428 passed |
| M3: Analysis and IR contract | Implemented; local gate passed | Consumed graph/effect/discovery/slice plans; separated identity checks; versioned IR and common validation; explicit terminals and stable source naming | Malformed-IR/pass/round-trip/naming tests pass; eight size baselines unchanged; pinned TLC gate passed |
| M4: Common Go patterns | Complete within the documented restricted profile | Static fields/direct receivers, restricted defers, proved constant integer ranges | Positive, refusal, identity, deadlock and synchronization-error regressions pass |
| M5: Component acceptance | Complete locally: three component families, seven correct/faulty CLI/TLC checks | Three realistic correct/faulty components, harness guidance, measured verification reports | Expected checker outcomes without rewriting the component as a DSL or removing its concurrency |

Implement milestones in this order. M2 should use existing abstractions rather
than undertake M3's refactoring prematurely; integrate its metadata with the
versioned IR contract in M3. Do not add new frontend support until M1's regression
gates are established.

### M4 substeps and refusal rules

1. **Static fields and direct receivers:** resolve fields by statically known
   object plus field path. Preserve initialization-order checks. Reject copying
   synchronization objects, ambiguous object sources, and mutable identity aliases.
2. **Restricted defers:** start with `defer mu.Unlock()` and `defer wg.Done()`.
   Preserve argument evaluation, registration timing, conditional registration,
   LIFO order, and every supported normal return. Do not pretend normal-return
   lowering implements panic/recover behavior.
3. **Proved finite loops:** begin with narrowly recognized constant-bound forms.
   Distinguish local computation from repeated behavioral operations; verify
   termination/bounds before eliminating or expanding either. Repeated spawn or
   allocation needs distinct finite identities. Nested forms and expansion size
   must have explicit acceptance/rejection rules. Worker receive loops and channel
   ranges are not automatically supported merely because a test uses few values.

### M5 evidence per component

Record the source/harness boundary, model process/resource/transition counts,
abstract predicates and contracts, checker version/options, state counts, elapsed
checking time, and the expected result for both versions. Faults should include
missing communication, omitted completion signaling, and incorrect channel closure.
If a component requires a new semantic capability, document and implement that
capability; do not hide it behind a false trusted-call contract.

## 6. Deferred scope

The following are not requirements for the basically usable milestone:

- General infinite control flow, recursion, or dynamic goroutine/channel topology.
- Full pointer/alias analysis or arbitrary interface/callback dispatch.
- RWMutex, WaitGroup reuse/concurrent enrollment, or complete Go runtime modeling.
- General panic/recover, reflection/unsafe behavior, network/syscall blocking,
  or broad standard-library concurrency summaries.
- Precise payload verification, arbitrary temporal properties, fairness,
  starvation, goroutine-leak analysis, or proof of functional correctness.
- Automatic argument/topology synthesis for arbitrary library entry functions.
- Full counterexample decoding/replay, property-directed refinement, or new backends.

Basic source references and checker-result summaries remain in scope; complete
counterexample replay does not. These deferrals are revisitable only with explicit
semantic contracts and evidence, not by weakening rejection rules.

## 7. Progress rules and next action

Use one atomic commit per independent change. Every capability update must include
its tests, documentation boundary, validation commands/results, and remaining
limitations. Mark a milestone complete only after its acceptance gate is met;
writing its plan or skipping its checker tests does not count as implementation.

### M1 validation record

- The strict local gate (`TLC_JAR=/absolute/path/tla2tools.jar bash scripts/test-ci.sh`)
  passed with Go 1.26.2, Java 25, and the official checksum-pinned TLC 1.8.0 JAR.
- All 33 actual TLC checks ran; no tests skipped. Vet, IR/TLA snapshots, all eight
  example size baselines, and repeated-extraction determinism checks passed.
- Negative checks confirmed failure for a missing required JAR, nonexistent explicit
  JAR path, wrong checksum, and enabled snapshot regeneration. Provisioning rejected
  an existing corrupted artifact without overwriting it.
- Shell syntax, workflow YAML structure, and local Markdown links were checked.
  GitHub-hosted execution and branch protection have **not** been verified or changed;
  maintainers must require the workflow job in repository policy separately.
- No supported-language or synchronization semantics were expanded by M1.

### M2 validation record

- `gotla check` records `result.json` schema version 1 and `tlc.log`; model JSON/IR
  semantics and supported Go scope remain unchanged.
- Actual CLI/TLC tests cover completed pass, deadlock, and synchronization-error
  outcomes, with hashes/provenance and source candidates. Deadline, cancellation,
  log limit, setup/protocol failure, stale-output, and concurrent-writer paths have
  focused regression coverage. The original 33 semantic TLC checks remain in place.
- Go tests, vet, strict pinned-TLC gate, and focused race checks passed locally.
  Hosted CI has not been run or claimed.
- Bounds apply to Java/TLC execution (including startup), JVM heap, and combined
  output. They do not bound Go SSA/lowering time, total process memory, or state-store
  disk use. Cancellation is checked between analysis passes. These limits are explicit
  in the CLI help and verification guide, not described as a whole-pipeline sandbox.
- Full trace decoding/replay, automatic source-build provenance capture, and handling
  abandoned output locks without operator inspection remain outside this milestone.

### M3 validation record — [DONE:5]

- Call summaries consume the static graph; cached function plans consume discovery,
  process-entry and backward-slice results. Capacity/resource/control extraction
  checks retained data. Identity/dominance and initialization checks are separated
  from effect summaries and atomic-region lowering, not bypassed by purity claims.
- Behavioral IR schema 1 has explicit communication/termination policies, terminal
  locations and analyzer/configuration provenance. Strict saved-IR decoding and
  generic validation are independent of TLA capability restrictions. The checker
  envelope records the model contract and shares binary provenance.
- New tests cover malformed/version-mismatched IR, duplicate/case-aliased JSON keys,
  bounded nesting, broken pass contracts, backend-only restrictions, and saved-IR
  round trips for all eight examples. Source tests distinguish same-basename files
  and declarations from spawn sites; unrelated SSA/process edits preserve names.
- Both full IR snapshots were checked for equivalent behavior modulo renamed
  identities, source metadata and explicit contract fields. All eight model-size
  baselines are unchanged. Golden provenance alone uses tool/version placeholders;
  real artifacts retain the actual values.
- Go/vet tests and the strict checksum-pinned TLC gate passed locally, with the
  original 33 semantic checks plus four CLI cases and zero skips. Internal/CLI
  race tests passed. Hosted CI has not been run or claimed.
- No fields, defers, loops or other frontend syntax were admitted by this refactor.
  See [the IR contract](docs/BEHAVIOR_IR.md) for backend limits, source-path policy,
  version compatibility and remaining provenance/trace limitations.

### M4 progress: inline synchronization fields

- Implemented static local/global object identities and nested inline Mutex/WaitGroup
  field paths. Direct pointer receivers/parameters and stable closure captures bind
  to the same objects; distinct objects/fields cannot collapse into one resource.
- Whole-aggregate copies, value receivers/arguments, resets, initializer copies,
  ambiguous/nil objects, pointer/channel fields and container elements remain rejected.
  Existing future-store and captured-cell dominance tests remain intact.
- `tests/fields_test.go` adds extraction/refusal tests and six actual TLC cases:
  repeated lock deadlock, distinct fields, cross-worker unlock, invalid unlock,
  WaitGroup completion and missing completion. The strict pinned gate passed with
  all original 33 semantic cases and four CLI cases, with zero skips; original
  snapshots/sizes unchanged. Full internal/CLI/integration race tests also passed locally.
- This is a self-contained first increment, **not completion of M4 / step 6**.

### M4 progress: immutable channel fields

- Local allocated structs and nested value fields can hold channel identities from
  direct allocations, parameters, resolved captures or nil. Fields sharing a channel
  retain the alias; binding a field does not allocate a new channel.
- The allocation-frame proof scans all object/field uses, not merely the slice.
  Initialization must be unique and dominate reads and calls/captures/spawns exposing
  any part of the object. Field writes seed the data slice and are erased only with
  an explicit per-frame proof. Callee writes, reassignment, escaped field addresses,
  aggregate copies/resets and unproved object-pointer cells remain rejected.
- Tests replace the former blanket channel-field rejection with a mutable-field
  rejection plus positive/refusal coverage, including direct initialization-order
  diagnostics. Seven additional actual TLC cases cover rendezvous, nil/closed errors,
  shared/distinct identities and default-before-peer deadlock. The previous 39
  semantic cases and four CLI cases remain; original snapshots/sizes are unchanged.
  The strict checksum-pinned TLC/vet/test gate and full internal/CLI/integration race
  tests passed locally, with zero skips. Hosted CI has not been run or claimed.
- Global channel-field access, returned object identities and general alias analysis
  are still outside this increment. See [SUPPORTED_GO.md](docs/SUPPORTED_GO.md) for
  exact accepted forms, including the current pointer-variable capture restriction.
- This increment alone does not complete M4 / step 6.

### M4 progress: restricted normal-return defers

- Implemented direct `defer mu.Unlock()` and `defer wg.Done()` with static receiver
  capture and exact per-invocation registration flags. Acyclic topological ordering
  plus a may-pending analysis preserves conditional registration, LIFO and all
  normal-return paths. Cleanup follows return-expression evaluation and cannot
  run while the function is blocked before returning.
- Nested/ repeated invocations own distinct flags. Executed cleanup clears its flag;
  compiler drain points do not run registrations twice. More than 64 sites per
  invocation is an explicit unsupported result, not silent truncation.
- General function/closure/method-value defers, alternate stacks, initializer defers,
  explicit panic/recover and cycles are still refused. Trusted contracts cannot
  bypass the defer whitelist; implicit-panic assumptions are unchanged.
- Eleven additional actual TLC cases cover Unlock/Done, synchronization faults,
  blocked return expressions, conditional/unselected registration, multiple returns,
  receiver capture, nested/repeated frames and field-based cleanup. An exhaustive
  single-process IR-path test checks LIFO and complete draining on conditional paths.
- The strict pinned-TLC/vet/test gate passed locally: 57 semantic TLC cases plus four
  CLI integration cases, with zero skips. Full internal/CLI/integration race tests
  passed; original snapshots and model-size baselines remain unchanged. Hosted CI
  has not been run or claimed.
- This defer increment alone did not complete M4 / step 6.

### M4 completion: proved constant integer ranges

- Added a typed-source proof/normalization pass before SSA: only main-module
  `for range N`/blank bindings with compile-time integer bounds. Original Go must
  type-check first; in-memory overlays are then reloaded/type-checked. No source
  file is rewritten and no Go-specific loop operation enters behavioral IR.
- Each iteration is a distinct lexical block with distinct SSA allocations/spawns.
  Return and deferred cleanup retain their original function scope. Zero-trip
  bodies and constant-bound references retain imports and their initialization.
  Source directives preserve original positions, including subsequent functions.
- Bounds above 16, over 256 KiB of expanded text per file, live indices, nested
  loop syntax, body labels/branch statements and existing line directives are
  explicitly refused. Classic/dynamic/channel ranges and unbounded receive loops
  remain unsupported. No bound is guessed and no behavior is silently truncated.
- Loop proof/rejection records are consumed by lowering and purity analysis;
  initialization helper proofs are recorded too. Unused unsupported helpers do not
  reject an otherwise supported entry point. Existing alias, initialization,
  WaitGroup phase and deferred-site restrictions remain in force after expansion.
- Nine new actual TLC cases cover finite workers, repeated identities, double
  close, blocked sends, cleanup timing, omitted completion, early return and zero
  iterations. Extraction/refusal/source-immutability/proof-ownership tests cover
  the accepted form and expansion limits. The semantic total is 66 TLC cases plus
  four CLI integration cases.
- The strict pinned-TLC/vet/test gate passed locally with zero skips. Full
  internal/CLI/integration race tests passed separately after the combined command
  exceeded its harness time allowance. Original snapshots and model-size baselines
  remain unchanged. Hosted CI has not been run or claimed.

### M5 progress: finite batch worker pool

- Added real library components and allocation-only main harnesses under
  `examples/components/workerpool`. The correct single-use pool submits two jobs,
  receives their results and joins two workers. Its faulty counterpart omits Done
  but preserves job/result communication, exposing a Wait deadlock.
- Both variants run through real CLI/TLC integration with expected status/exit,
  artifact hashes/provenance, source candidates and model-size baselines. Native Go
  tests cover the correct component's result; payload arithmetic is not a TLC claim.
- Recorded model/state counts, tool/configuration, elapsed time and GNU time RSS
  with its measurement limitations in [COMPONENTS.md](docs/COMPONENTS.md). No custom
  trusted calls or abstracted predicates are needed. The pair adds two end-to-end
  CLI checks, separate from the existing 66 semantic TLC cases and four CLI cases.
- The pinned-TLC/vet/test gate passed locally with zero skips. Full internal/CLI/
  integration/component race tests passed; the original snapshots and size
  baselines are unchanged. No hosted CI execution is claimed.
- M5 / step 7 remains incomplete. A finite single-use pool does not stand in for a
  general receive-loop worker service or the required close-driven pipeline.

### M5 prerequisite: exact receive completion status

- Direct comma-ok and select receive status now drives exact guards (including
  negation/Boolean-constant comparisons). Delivered values produce true, closed
  empty reads false; nil/open-empty blocking produces no result. Buffered draining
  and unbuffered rendezvous/wakeup preserve these distinctions atomically.
- Generic Receive optionally binds a process-local Boolean-domain destination.
  IR validation enforces ownership, domain and single writes; the TLA backend
  implements the binding without Go syntax or payload tracking. Saved IR round
  trips and malformed bindings are tested. Existing models remain unchanged.
- Thirteen new actual TLC cases cover these semantics, bringing the semantic total
  to 79, separate from six CLI integration cases. Status propagated through calls,
  Phi/shared memory or Boolean payloads is not claimed exact.
- The pinned-TLC/vet/test gate passed locally with zero skips, and full internal/
  CLI/integration/component race tests passed. Existing snapshots and size
  baselines remain unchanged. Hosted CI execution is not claimed.
- This status prerequisite alone did not admit channel ranges or complete M5.

### M5 progress: finite-state receive cycles and a real pipeline

- Added a reachable-SSA-SCC proof for close-driven receive control. Every accepted
  cyclic component has one dominating comma-ok receive, an exact closed-empty
  exit, a channel identity evaluated outside the cycle and no reception-bypassing
  subcycle. Repeating allocation/spawn/defer/store/helper/select/counter effects
  are rejected; scalar computations, sends, stable fields and close/Lock/Unlock
  remain supported under ordinary identity/effect rules.
- Cycles are preserved in behavioral IR, not unrolled to a guessed receive count.
  The backend independently rejects repeatable spawn/counter updates and cycles
  lacking status-producing receives. Bounded modeled queues/locals/locks and
  fixed process identities imply finite state, not termination or fairness.
- Deferred registrations/drains remain acyclic; may-pending analysis now reaches
  a fixed point across receive SCCs. Cleanup remains pending during a missing-close
  block and executes on supported normal returns afterward.
- Fourteen new actual TLC cases cover empty/closed/buffered/nil streams, forwarding,
  omitted/incorrect close, explicit comma-ok loops, deferred cleanup across loops,
  mutex use, sequential loops and 32 deliveries without a guessed receive limit.
  Refusal/SCC/IR tests cover repeated effects, wrong exits, subcycles, saved models
  and malicious repeatable spawn/counter effects. The semantic TLC total is 93.
- Added a genuine producer/transform/reducer library with allocation-only harnesses.
  Both consumers retain their channel ranges. Correct, missing-output-close and
  early-output-close variants respectively report passed, deadlock and a
  synchronization error through real CLI/TLC checks (nine CLI cases total).
  Native tests, model-size baselines and measured resource/state reports are in
  [COMPONENTS.md](docs/COMPONENTS.md).
- The strict pinned-TLC/vet/test gate passed locally with zero skips, as did full
  internal/CLI/integration/component race tests. Original snapshots and size
  baselines remain unchanged. Hosted CI execution is not claimed.
- M5 / step 7 remains incomplete: the mutex-protected component and consolidated
  acceptance/delivery report are still outstanding.

### M5 completion: mutex component and consolidated local acceptance

- Added a real inline-Mutex counter with Increment/Value methods, concurrent batch
  orchestration and allocation-only harnesses. Correct deferred Unlock passes;
  omitted Unlock deadlocks. Native tests cover sample values separately from TLC's
  synchronization projection. Exact model-size and artifact/source checks apply.
- The original closure syntax exposed a missing receiver-pointer capture capability.
  Added only immutable pointer-cell capture for inline-sync structs with no channel
  fields, consuming the existing unique dominating-store proof. Future/conditional
  stores, reassignment, nested closure writes, escapes, nil use and copies remain
  rejected; shared and distinct identities have two new real TLC checks.
- Three component families now have seven real CLI/TLC variants. The local gate
  totals 95 semantic TLC checks and 11 CLI cases. [COMPONENTS.md](docs/COMPONENTS.md)
  records model/state counts, timing/RSS, source boundaries and limitations.
  [ACCEPTANCE.md](docs/ACCEPTANCE.md) consolidates the local milestone evidence,
  reproduction commands, result interpretation, delivery order and deferred scope.
- M5 / step 7 is complete locally. This does not claim hosted CI or full release
  acceptance; the unchecked hosted gate above still requires execution evidence.

### Delivery acceptance — step 8

- The owner configured/pushed the remote and authorized repair pushes. Hosted
  validation exposed an unavailable job-level runner context, then a replaced
  upstream TLC release asset. Both failures were fixed without bypassing the gate.
- The refreshed pin was checked against the official asset digest and passed the
  strict local gate plus full race suite; historical measurements retain their
  original checker identity. Hosted run `35173519428` passed on `f19df4d`.
- Downloaded the retained verification log: zero skips/failures, all component
  groups passing. [ACCEPTANCE.md](docs/ACCEPTANCE.md) records exact evidence.
- Steps 1–8 now have the required implementation/planning and verification evidence
  for the explicitly restricted basically-usable target. Delivery order and deferred
  scope remain in sections 5–6; this is not arbitrary Go correctness or a claim of
  published release artifacts, configured branch protection, or indefinite log retention.

**Next action:** require fresh CI for subsequent changes. Address action deprecation
warnings as maintenance; expand semantic scope only with explicit proofs and tests.

### B1: restricted production-component bootstrap — locally complete

- Constructor, exact local interface and immutable callable-field proofs were added
  incrementally, without trusting standard-library I/O or dropping initializer checks.
- Extracted the actual runner logger into `internal/boundedlog.Writer`, preserving
  its Write body after field/dependency renaming. The checker and finite harness
  call the same production implementation; the runner retains its error sentinel.
- Two concurrent callers pass TLC with a returning sink/bounded notification callback.
  Removing the actual deferred Unlock and using a blocking cancellation environment
  each produce deadlock counterexamples. Models contain nonempty production locking,
  proved field dispatch and explicit data abstractions; no trusted-call flags.
- The strict local gate and full race suite pass, with 119 semantic TLC cases and
  14 TLC CLI cases, zero skips/failures and unchanged original snapshots. Native
  logger tests separately cover budget/error behavior and concurrent writes.
- Full CLI self-analysis still returns unsupported / 5. This does not verify
  arbitrary file I/O, context/process lifecycles, re-entry, payload correctness or
  the analyzer's soundness. Bootstrap commits have no claimed hosted pass.
  [BOOTSTRAP.md](docs/BOOTSTRAP.md) records exact assumptions, measurements and logs.
- Further bootstrap work requires independent parser-callback or lifecycle contracts;
  do not replace unresolved effects with trusted no-ops to claim whole-program success.

### B1 follow-up: blocking environments and refusal regression

- Added explicit gated output to the unchanged production Writer: two callers pass
  with a joined releaser and deadlock without one. Source-located receive effects
  ensure output blocking is modeled rather than treated as pure.
- A self-captured, reentrant cancellation environment is explicitly refused with no
  executable/stale artifacts or TLC invocation. It is not advertised as a checked
  counterexample; the initialization/alias proof remains conservative.
- Full local gate/race suites pass: 119 semantic and 16 TLC CLI cases, plus full-CLI
  and re-entry refusal regressions. Original snapshots and production/analyzer
  semantics are unchanged. Evidence is in [BOOTSTRAP.md](docs/BOOTSTRAP.md).
  No new hosted pass or whole-program self-verification is claimed.

### B2: production source-capture protocol — locally complete

- Extracted `internal/sourcecapture.Collector.Record`; the real frontend ParseFile
  callback invokes it before parsing. Lock/copy/map-store/unlock order is preserved,
  and map consumption/clear remains after package loading completes.
- Changed the leaf's copy dependency from bytes.Clone to slices.Clone, not an analyzer
  exemption. The unchanged bytes dependency probe still refuses errors-package
  initialization. Native tests cover nil/empty/content behavior, ownership,
  replacement and concurrent records; capacity/retention are outside the contract.
- Two finite callers pass TLC; removing only the actual Record Unlock deadlocks.
  Both cases contain production source-located locking, with no trust flags. This
  verifies the synchronization projection, not map/byte values or general race freedom.
- The full pinned gate/race suites pass: 119 semantic and 18 TLC CLI cases, zero
  skips/failures, original snapshots unchanged. Full CLI self-analysis is still
  unsupported / 5. Evidence and dependency-change limits are in
  [BOOTSTRAP.md](docs/BOOTSTRAP.md); no hosted pass is claimed for these changes.
- Actual go/packages callback scheduling, parsing and context/process lifecycles
  remain outside this finite capture harness.

### Full-self goal: not complete; static deferred helper prerequisite implemented

- The objective is the actual CLI's executable self-model and real TLC evidence,
  not a relabeled component pass. [SELF_BOOTSTRAP_GOAL.md](docs/SELF_BOOTSTRAP_GOAL.md)
  records the explicit unfulfilled acceptance gates and remaining implementation work.
- Source-defined direct function/method/closure defers now enter their inspected
  bodies during LIFO cleanup. Bindings are captured at registration; blocking, nested
  defers and normal-return ordering are preserved. Graph/slice loss fails closed.
  Dynamic targets, wrappers, foreign stacks and panic/recover remain unsupported.
- Twelve new TLC semantic checks bring the local total to 131, with 18 CLI/TLC cases.
  The strict gate and full race suite pass with zero skips/failures and unchanged
  snapshots. The stale empty-closure rejection became a positive test; dynamic
  deferred calls retain negative coverage.
- The real self-input now gets beyond the former main.go defer refusal but still
  returns unsupported / 5 on deeper effects/identity/control. No executable full
  self-model or hosted pass is claimed. Continue toward the full goal; do not mark
  it complete from these prerequisite results.

### Full-self continuation: length-bounded data computation

- Added an independent typed-SSA termination/effect proof for externally read-only
  helpers with monotone ordinary-int counters bounded by stable slice/string lengths.
  Every cycle crosses the comparison header; transitive effects and data types are
  checked. Private data writes retain their isolation proof. No unrolling, guessed
  trip count or external trust replaces a missing proof.
- Current graph/body and retained caller operands are rechecked during lowering.
  Argument effects and abstract return predicates remain modeled. Counter resets,
  narrow/overshooting indices, changing bounds, hidden cycles/effects and exhausted
  proof budgets fail closed.
- The real CLI self-input consumes proofs for `toolinfo.fromBuildInfo` and
  `behavior.Model.HasErrors`; loop refusals drop from 83 to 73. The CLI is still
  unsupported / 5 and has no executable full self-model.
- Eight new actual TLC cases bring totals to 139 semantic and 18 CLI/TLC cases.
  Strict pinned gate and full race suite pass with zero skips/failures; original
  snapshots are unchanged. See [SELF_BOOTSTRAP_GOAL.md](docs/SELF_BOOTSTRAP_GOAL.md)
  for the continuing goal, remaining gates and local-only evidence.

### Full-self continuation: initializer contracts

- Initializer calls now consume current graph evidence even when their purity summary
  was cached; missing outer/transitive edges fail closed. Six contract cases cover
  ordinary and finite-data helpers, and four integration cases preserve rejected
  source/external/dynamic/interface call attribution.
- Refusals identify target and containing function without treating candidate names
  as proof. The real CLI's 240 initializer refusals include 104 `edge.info` calls
  and 11 `go/types.asGoVersion` calls. Reflection metadata construction and version
  parsing remain real dependencies to model, not permission to omit initialization.
- Full pinned gate and race tests pass with unchanged snapshots and TLC totals;
  the real CLI remains unsupported / 5. See the full-self contract for evidence.

### Full-self continuation: pre-closed global channels

- A unique direct global channel creation followed by a proved unconditional
  initialization close now becomes explicit empty/closed IR state. The proof checks
  current source/graph/order and all SSA global-address uses; hidden rebinding,
  address escape, conditional/repeated close and unknown effects remain refused.
- The actual `context.closedchan` now has creation/close proof records. Initialization
  refusals decrease from 240 to 238 without a context-package exemption. The whole
  CLI remains unsupported / 5; no executable full self-model is claimed.
- Eight added actual TLC cases exercise saved-state semantics, concurrent receives,
  select/status/range behavior, synchronization errors and an initial-state mutation.
  Totals: 147 semantic and 18 CLI/TLC cases. Strict pinned gate and race suite pass,
  zero skips/failures, original snapshots unchanged. Local-only evidence and remaining
  full-goal gates are recorded in SELF_BOOTSTRAP_GOAL.md.

### Data-table prerequisite: private fixed arrays

- Private-data proofs now cover nested array/field addresses with complete use checks.
  Reference/synchronization elements, publication, slicing aliases and shared writes
  remain refused. Finite data computations also admit constant int bounds in the
  common 32-/64-bit range, with unchanged counter/progress/exit proofs and no truncation.
- Seven new actual TLC cases bring totals to 154 semantic and 18 CLI/TLC cases.
  Updated the obsolete empty classic-loop refusal to a synchronization-loop refusal
  and added a positive TLC check. Corrected a new fixture separator error; failed
  attempts are retained. Full strict gate and race suite then pass, snapshots unchanged.
- This prerequisite does not remove an actual CLI refusal yet: self-input still
  returns unsupported / 5 with 238 initializer refusals. Library constructors needing
  copy, reflection or explicit panic-path reasoning are not silently admitted.

### Full-self continuation: literal-input SSA data calls

- Added bounded evaluation of selected SSA paths for constant-argument data calls.
  All writes target owned storage; actual transitive bodies, validation branches,
  copies and value/alias semantics are checked. Unknown/external operations and
  exhausted limits refuse the proof. No application code is run on the host and
  no function-name allowlist replaces body inspection.
- Lowering rechecks arguments, graph/body and caller slices. The real base64 alphabet
  initializers now pass this proof, reducing self-input initializer refusals to 236.
  Native table comparison and invalid alphabet mutations verify the actual source.
  Full self-input remains unsupported / 5; no executable whole-self model exists.
- Nine new TLC cases bring totals to 163 semantic and 18 CLI/TLC cases. Updated
  general-rule refusals whose empty/literal inputs now have per-call proofs, preserving
  external/nonempty refusal coverage and adding positive cases. Strict gate and race
  tests pass with zero skips/failures and unchanged snapshots. Evidence and remaining
  full-goal obligations remain in SELF_BOOTSTRAP_GOAL.md.

### Full-self continuation: owned data references

- Bounded invocation evaluation now supports zero data references and owned pointer
  graphs, retaining full type-graph exclusions and cell ownership checks.
- The actual go/constant.newFloat initializer is proved; native precision comparison
  and full-CLI proof-consumption regression cover the production source.
- Three new TLC cases bring totals to 166 semantic and 18 CLI/TLC cases. Strict gate
  and race pass; full self-input still refuses with 235 initializer diagnostics.
  See docs/SELF_BOOTSTRAP_GOAL.md for reproducible evidence and remaining scope.

### Full-self continuation: open global I/O semaphores

- Prove unique constant-capacity global channel creation and stable identity using
  complete SSA address inventory; preserve independent initializer-effect checks.
- Model the three real x/tools I/O semaphores as initially empty/open. Require their
  source proof records and capacities in the full-CLI refusal regression.
- Recheck exact capacity for open and pre-closed resources, including in-range changes.
- Seven new TLC cases bring totals to 173 semantic and 18 CLI/TLC cases; strict gate
  and race pass. Full self-input still refuses with 232 initializer diagnostics.
  Evidence and remaining requirements: docs/SELF_BOOTSTRAP_GOAL.md.

### Full-self continuation: read-only reference-bearing value copies

- Permit local value copies inside the full finite-data proof, without extending
  ownership through loaded references or widening general constructor purity.
- Prove the real Model.Statistics helper; keep borrowed writes/publication rejected
  and invalidate cached proofs when a private store becomes a global store.
- Five new TLC cases bring totals to 178 semantic and 18 CLI/TLC cases. Strict gate
  and race pass. Full self-input still refuses: 70 loop and 232 initializer diagnostics.
  Evidence and remaining requirements: docs/SELF_BOOTSTRAP_GOAL.md.

### Full-self continuation: scalar-array range normalization

- Expand small by-value scalar arrays with Go 1.22+ declared index/value semantics,
  retaining snapshot/evaluation count, lexical bindings, return/defer and source locations.
- Use a fixed array for the actual managed artifact file table; consume both real
  five-iteration proofs without dropping I/O or error paths.
- Seven new TLC cases bring totals to 185 semantic and 18 CLI/TLC cases; native
  transformation comparisons, strict gate and race pass. Self-input remains unsupported:
  67 loop refusals, with additional underlying effects now exposed by expansion.
  Evidence and remaining requirements: docs/SELF_BOOTSTRAP_GOAL.md.

### Full-self continuation: immediate program exit

- Add an explicit backend-independent Exit instruction and ordinary-process
  `os.Exit(int)` semantics, not an ignored/pure library call. Preserve argument
  effects and stop every process without running remaining cleanup.
- Consume current signature, call graph and data-slice facts; check standalone
  terminal-only IR exits and prevent trusted-call overrides.
- The real CLI refusal model contains its source-located Exit instruction. Eight
  independent IR/TLC cases plus source-lowering/corruption tests pass; totals are
  193 semantic and 18 CLI/TLC cases. Strict gate and full race pass, snapshots unchanged.
- Full self-input remains unsupported / 5 with no executable artifacts or TLC run.
  Initialization and the enforceable full input/environment profile remain unfinished.

### Full-self continuation: nil-interface constant storage

- Admit nil empty-interface storage only in bounded concrete SSA evaluation;
  retain boxing, invocation, assertion, external-memory and nonliteral-input refusals.
  Keep general finite-data type rules unchanged and materialize SSA zero aggregates
  with value-copy/alias-preserving reset semantics.
- Prove the real go/doc ast.NewIdent initializer and compare its fields with native
  Go; recheck ownership when consuming cached constructor proofs.
- Five new TLC cases bring totals to 198 semantic and 18 CLI/TLC cases. Strict gate
  and full race pass with unchanged snapshots. Initializer refusals fall to 231;
  the full self-input still returns unsupported / 5 with diagnostic-only artifacts.

### Full-self continuation: explicit byte search and version initialization

- Add a checked standard-toolchain byte-string-search operation model, with exact
  finite input evaluation and a recorded assembly-correctness assumption rather
  than a name-only pure-call exemption. Retain current graph/signature/declaration/
  source checks, budget refusal and ordinary interpretation of available Go bodies.
- Add bounded string concatenation. Native comparisons cover byte search, version
  strings and the actual ten literal go/types version initializers; the nonliteral
  current version and other unknown initialization remain refused.
- Three string-concatenation TLC cases bring totals to 201 semantic and 18 CLI/TLC
  cases. Strict gate and full race pass; snapshots unchanged. Full self-analysis
  still returns unsupported / 5; initializer errors decrease from 231 to 221.

### Full-self continuation: immutable reflection metadata

- Add source/graph/SSA-bound TypeFor metadata tokens and explicit standard Elem/
  direct FieldByName models. Record ABI and standard-reflection assumptions;
  never construct runtime values, invoke application methods or search promoted
  fields. Other reflection initialization remains independently checked.
- Compare all 104 actual x/tools AST edge initializers with native Go metadata,
  alongside type/layout/tag/presence checks. Reject altered extraction chains,
  methods, arguments and stale consumed proofs.
- Real self-analysis consumes those 104 proofs and five encoding/json TypeFor
  initializers; initializer refusals decrease from 221 to 112. Strict gate and
  full race pass, snapshots unchanged; totals remain 201 semantic and 18 CLI/TLC
  cases. Still unsupported / 5, with no executable self-model or self TLC run.

### Full-self continuation: production lexical scanning

- Replace the project's three lexical regexp initializers with ASCII scanning:
  shared TLA identifiers, framed TLC headers and trace PC entries. Preserve the
  exact old grammar, duplicate-entry behavior, numeric conversion and outcome
  classification; retain regex oracles only in tests. Differential fixtures,
  deterministic randomized inputs and both fuzz targets pass.
- Prove Go 1.26's rotated integer-range SSA for stable len bounds, with guarded
  zero entry, exact tested increments, dominance and no hidden cycles. Keep
  constant-range expansion limits unchanged. Actual Identifier/decimalDigits
  source proofs and cached-proof mutation refusals pass.
- Five new TLC cases bring totals to 206 semantic + 18 CLI/TLC; strict gate/race
  pass, snapshots unchanged. Self-input remains unsupported / 5, with initializer
  refusals reduced from 112 to 109. No executable self-model or self TLC yet.

### Full-self continuation: conditional startup processor setting

- Add `-runtime-procs N`, an explicit linux/amd64 startup GOMAXPROCS condition,
  not a host-setting inference or a general environment sandbox. Record the
  condition and standard-runtime assumption in model/result provenance.
- Inventory all current SSA references and refuse setters/resets/escapes/trust;
  freshly consume source/signature/graph/slice proofs for zero queries and exact
  local/global channel capacities. Preserve other initializer/effect refusals.
- Native capacities at 1/2/3 agree. Six new semantic + two CLI/TLC cases bring
  totals to 212 + 20; strict gate/full race pass with unchanged snapshots.
- Actual N=2 self-input consumes both x/tools CPU-limit channels. It remains
  unsupported / 5 with 105 initializer refusals (109 without the profile), no
  executable self-model or self TLC proof. The full finite environment is not done.

### Full-self continuation: source-bound Once.Do

- Model checked standard Once.Do with analyzed source callbacks, mutex-protected
  first execution, committed flag reads and separate completion/unlock steps.
- Enable declared finite shared IR variables in TLA without modeling arbitrary Go
  shared memory. Callback body edges do not resolve dynamic parameter calls.
- Eleven native/TLC and two generic shared-state TLC cases, contract mutations and
  refusal tests; strict gate/full race pass, 245 + 20 TLC totals, unchanged snapshots.
- Real godebug.Value has an eligible contract; bound wrappers remain refused. The
  whole-self attempt still consumes no Once call and remains unsupported / 5;
  context lifecycle, finite environment and full executable proof remain unfinished.

### Full-self continuation: channel receiver boxes

- Admit only graph-proved receiver uses of locally boxed channel-bearing objects;
  require initialization before boxing and keep arbitrary escapes unsupported.
- Inspect current SSA uses with a checked 4096-step budget instead of cached use
  lists. Eight native/TLC cases plus mutation/refusal tests; strict gate/full race
  pass, 232 + 20 TLC totals, unchanged snapshots.
- Actual self-check still refuses returned os/signal context identity and cancellation;
  unsupported / 5 remains, without executable self-model or self TLC evidence.

### Full-self continuation: exact deferred dispatch

- Reuse fresh local-interface/immutable-field call proofs at defer registration;
  preserve captured identities, actual helper effects and LIFO cleanup.
- Ten paired native/TLC checks and stale-proof/refusal tests; strict gate/full race
  pass, 224 + 20 TLC totals, unchanged snapshots. Returned context cancellation,
  general dynamic dispatch and previously unmodeled identities are not admitted.
- Actual N=2 self-check remains unsupported / 5 (101 initializer, 68 loop, 14 defer
  refusals), without executable self artifacts. Full acceptance is still unmet.

### Full-self continuation: fixed checker input files

- Use a finite, deterministic two-file array in actual checker.Run; preserve both
  contents, file modes, error returns and cleanup. Native subprocess tests verify
  contents/permissions and reject swapped inputs.
- Actual self-input consumes the two-iteration source proof. The next blocker is
  deferred returned context cancellation; loops 69→68, defers 13→14. Self-analysis
  remains unsupported / 5 with 101 N=2 initializer refusals, not a full proof.
- Strict gate/full race pass; 214 + 20 TLC totals and snapshots unchanged.

### Full-self continuation: checker argument formatting

- Extend guarded scalar formatting to Sprint/Sprintln, preserving exact variant
  methods, arity, source/graph, nonescaping arguments and consumed store roots.
- Native and mutation tests pass; an actual-source unit proof covers checker.Run's
  worker argument. The whole-self probe does not yet consume that site.
- Strict gate/full race pass, 214 + 20 TLC totals and snapshots unchanged. Whole-self
  remains unsupported / 5 with 101 N=2 initializer refusals; no completion claimed.

### Full-self I/O boundary audit

- Stop silently eliding Go print/println: require an explicit output model, refresh
  initializer/call checks, and reject builtin trust-name overrides.
- Native/CLI/cached-proof tests preserve refusal and argument effects. Two TLC cases
  cover shadowed source names and proved zero executions; totals 214 + 20.
- Strict gate/full race pass with unchanged snapshots. This soundness correction
  does not complete the missing I/O profile or full self-bootstrap proof.

### Full-self continuation: basic-scalar formatting

- Admit source/graph-checked fmt.Sprintf only for nil or nonescaping local scalar
  argument arrays. Refuse application callbacks, aliases, stale stores/slices and
  unknown values; record standard formatting/private-runtime correctness explicitly.
- Retain argument evaluation and store roots, freshly consuming every proof. Native
  and corruption tests pass; actual own-source/stdlib formatting sites are covered.
- Strict gate/full race pass, snapshots and 212 + 20 TLC totals unchanged. Actual
  N=2 self-input remains unsupported / 5 with 101 initializer refusals; no executable
  self-model or TLC self-proof. Finite input/I/O/lifecycle work remains open.

### Full-self evidence capture

- Provide a fixed-real-entry capture command with a clean-source prerequisite,
  pinned JAR, selected source/tool fingerprints, explicit startup processor setting,
  before/after drift checks and validated result/artifact provenance.
- Retain failed attempts and propagate unsupported / 5 rather than relabeling a
  diagnostic model as a proof. Keep `goalComplete: false` independently of capture.
- Add standard-library Python evidence-corruption tests to the strict gate. This
  infrastructure does not satisfy the remaining full-self semantic requirements.

### Full-self continuation: literal type metadata

- Guard the actual three reflect.rtypeOf initializers by current standard wrapper,
  ABI chain, graph, dominating literal box and budgets; record the explicit runtime
  metadata assumption without admitting general boxing or reflective execution.
- Native type/layout comparisons and stale source/graph/argument/ABI/slice mutations
  pass. Strict gate/full race pass; snapshots and TLC totals (212 + 20) are unchanged.
- Actual self-input remains unsupported / 5: 102 initializer refusals under startup
  GOMAXPROCS=2, or 106 without a profile. No executable self TLA+ or self TLC yet.
