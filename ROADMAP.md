# Roadmap to a basically usable gotla

## Status and document ownership

The current implementation is a restricted, end-to-end MVP. The next milestone
is a usable tool for small, explicitly scoped Go concurrency components, **not**
a general Go verifier. This roadmap defines that target; it does not expand the
current supported language.

- [README.md](README.md): current installation and command usage.
- [ARCHITECTURE.md](ARCHITECTURE.md): current semantics, assumptions, and limitations.
- This document: target scope, acceptance gates, priorities, and progress.

Current companion guides:

- [docs/SUPPORTED_GO.md](docs/SUPPORTED_GO.md): supported, abstracted, and rejected Go patterns.
- [docs/VERIFICATION.md](docs/VERIFICATION.md): results, assumptions, false positives, and incomplete checks.
- [docs/SEMANTIC_TEST_MATRIX.md](docs/SEMANTIC_TEST_MATRIX.md): regression evidence and coverage gaps.

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
finite loop form is a constant integer range without a live index (up to 16 iterations). The `check` workflow is
implemented within the current restricted frontend scope.

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

Current restrictions include all reachable CFG cycles and recursion, mutable/global
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

- [ ] M1–M5 below meet their acceptance gates, with evidence recorded per change.
- [ ] A user can build the CLI, provision the documented TLC version, and run a
      component check using documented non-interactive commands.
- [ ] Successful, deadlocking, synchronization-error, unsupported, tool-failure,
      and incomplete runs have distinguishable CLI and machine-readable results.
- [ ] Checks record assumptions, trusted contracts, tool/configuration information,
      artifact locations, and model size; stale artifacts cannot masquerade as a
      new successful result.
- [ ] Three production-style component examples retain their concurrency source
      and have small explicit harnesses where needed: a finite worker pool, a
      close-driven pipeline, and a struct-based synchronized component.
- [ ] Each component has a correct version and an intentionally faulty version;
      tests assert the expected checker outcome, not just successful TLA parsing.
- [x] Supported loop forms, defers, and field identities each have positive,
      negative, and blocking/synchronization-error regression coverage.
- [ ] CI runs real TLC and fails if the required checker is unavailable; plain unit
      test success alone cannot satisfy the release gate.
- [ ] Documentation lists remaining restrictions and explains that completed
      abstract checking is neither arbitrary Go correctness nor leak/starvation
      verification.

Model size, state count, elapsed time, and peak resource use should be measured on
a recorded reference environment. Choose numerical budgets from those baselines;
do not invent performance guarantees before measuring the target examples.

## 5. Delivery milestones

| Milestone | Status | Deliverables | Acceptance gate |
|---|---|---|---|
| Scope definition | Done (documentation) | Target users/workflow, supported-domain boundary, non-goals, definition of done | This roadmap separates current capabilities from planned ones |
| M1: Quality baseline | Implemented; local gate passed | Semantic test matrix; Go/vet/snapshot/TLC CI; pinned checker provisioning; model statistics | Reproducible models and expected outcomes; strict gate cannot silently skip TLC; hosted run pending |
| M2: Check workflow | Implemented; local gate passed | `gotla check`; JAR/timeout/memory/worker/log settings; raw log and structured result; source candidates; stale-output handling | Real TLC pass/deadlock/invariant CLI tests and bounded subprocess/failed-output tests pass; hosted run pending |
| M3: Analysis and IR contract | Implemented; local gate passed | Consumed graph/effect/discovery/slice plans; separated identity checks; versioned IR and common validation; explicit terminals and stable source naming | Malformed-IR/pass/round-trip/naming tests pass; eight size baselines unchanged; pinned TLC gate passed |
| M4: Common Go patterns | Complete within the documented restricted profile | Static fields/direct receivers, restricted defers, proved constant integer ranges | Positive, refusal, identity, deadlock and synchronization-error regressions pass |
| M5: Component acceptance | In progress: worker-pool pair and close-driven pipeline variants validated | Three realistic correct/faulty components, harness guidance, measured verification reports | Expected checker outcomes without rewriting the component as a DSL or removing its concurrency |

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

**Next action:** validate a struct-based mutex-protected component and its faulty
variant, then finish the consolidated basically-usable acceptance evidence.
