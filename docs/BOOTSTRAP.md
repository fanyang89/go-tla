# Self-analysis / bootstrap feasibility

## Current status

The active full-self-bootstrap objective and its still-unmet end-to-end gates are
recorded in [SELF_BOOTSTRAP_GOAL.md](SELF_BOOTSTRAP_GOAL.md). Component passes below
are prerequisites, not completion of that objective.

**Two production synchronization components now have finite-environment bootstrap.**
The logger used by `checker.Run` and source collector used by `frontend.LoadContext`
have passing TLC harnesses and actual missing-Unlock mutation counterexamples.
Logger blocking environments have additional checks. No trusted calls are used.
The complete CLI still returns **unsupported / 5**.

Current totals: **193 semantic TLC cases and 18 TLC CLI cases**, plus separate
full-CLI and reentrant-logger refusal tests. The prerequisite sections below record earlier stages;
their counts and unresolved-component statements describe those historical stages.

## Original full-CLI observation

The binary built from `a57d675` was run on its own actual entry point:

```sh
TOOLS="${TMPDIR:-$HOME/tmp}/pi/gotla-bootstrap"
mkdir -p "$TOOLS"
go build -o "$TOOLS/gotla" ./cmd/gotla
"$TOOLS/gotla" check -out "$TOOLS/cli" \
  -tlc-jar /absolute/path/to/pinned/tla2tools.jar ./cmd/gotla
# Current expected exit: 5 (unsupported), NOT 0.
```

The command loads and analyzes the source, writes `result.json` and a diagnostic
`model.json`, then refuses executable TLA+/TLC generation. It does not launch TLC.
This is a working **self-input smoke test**, not a successful verification of gotla,
not a self-hosting compiler, and not a proof of the analyzer's soundness.

The observed run had 324 initializer errors, 27 unknown-effect errors, 30
unsafe-pointer errors, 18 identity errors and additional control/defer/topology
refusals. These counts depend on the toolchain/source and are **not** a stable
baseline or independent bug count. Examples in our own CLI include general cleanup
at `cmd/gotla/main.go:56` and a dynamic/interface call at `main.go:111`. Dependency
initialization is inspected even when a dependency's runtime code is not reached.
Removing initializers or trusting the top-level checker would invalidate this test.

Local raw evidence: `$HOME/tmp/pi/gotla-bootstrap/{self-check.log,cli/result.json,cli/model.json}`.
No custom trusted calls were supplied. The existing root `tla2tools.jar` was not used
or modified. The explicit JAR path points to the refreshed pinned asset, but this
unsupported analysis does not reach checker provisioning.

## Continuous regression

`TestCheckSelfAnalysisBoundary` in `cmd/gotla/bootstrap_test.go` loads the real CLI
from its own package directory. It asserts:

- unsupported extraction and exit 5, with source-located diagnostics in the CLI;
- no trusted calls or TLC invocation;
- fresh, hash-validated diagnostic artifacts;
- no executable model, old checker log, stale successful result or leaked lock.

```sh
go test ./cmd/gotla -run '^TestCheckSelfAnalysisBoundary$' -count=1
```

No JAR is needed for this refusal test. The existing strict CI gate includes it.
A passing Go test means the refusal boundary is intact, **not** that gotla verified
itself. It is separate from the 193 positive/negative semantic TLC cases and 18
TLC CLI cases. Future support improvements must deliberately revise this expectation
with real checker evidence, rather than delete refusals to turn the test green.

The checksum-pinned strict local gate passed with zero skips/failures after adding
this regression. The self-analysis test also passed under `-race`. Logs are in
`$HOME/tmp/pi/gotla-bootstrap/{gate,race}.log`. That initial self-input-test commit
did not change production semantics or supported syntax; the constructor increment
below is a separate capability change. Neither increment has a claimed hosted pass.

## Original production-target selection

The selected target was `internal/checker/runner.go`'s `boundedLog.Write`:
it takes an inline Mutex, defers Unlock, writes through an `io.Writer`, records an
error and invokes a cancellation callback. Its lock belongs to actual production
code, unlike copying a small locking example into a bootstrap directory.

At that stage it was not a supported component harness. Obstacles included private entry
access, effectful package initialization, concrete target resolution for the writer
interface and cancellation function field, and externally scheduled calls from
`os/exec`. I/O and cancellation cannot be marked pure: doing so could hide blocking
or re-entry while the mutex is held. Merely constructing `boundedLog` without calling
Write would be a vacuous test. A future harness must state its finite caller set,
writer/error behavior and cancellation environment, and preserve the original Write.

The other small synchronization site is `frontend.LoadContext`'s source-capture
mutex inside the parser callback. Its caller is `go/packages`; the callback's
concurrency is not an explicit `go` statement in our wrapper. Mutable maps, callback
resolution and library initialization still need contracts. The current models do
not prove map contents or general race freedom.

## Bootstrap prerequisite: private data constructor proofs

The next increment admits acyclic helpers that initialize fresh scalar/value-struct
allocations and only expose their addresses on return. The proof follows all SSA
address uses, never a package/function allowlist. It rejects reference-bearing or
synchronization data, early publication, closure capture and address-taking calls.
Helper summaries also reject shared map updates and mutating/I/O builtins: a private
allocation elsewhere must not excuse an external write. Initializer traversal,
unsafe checks, call-graph consumption and synchronization identity rules remain.

This proves the real installed `errors.New` body from source, with no trust flag.
It does not accept the entire `errors` package: reflective initialization in
`errors/wrap.go` still has unsupported calls. On Go 1.26.2, repeating the real CLI
probe reduced initializer errors from **324 to 240**; other error categories remained.
These are diagnostic observations, not a coverage metric. The CLI still returns
**unsupported / 5**, writes only diagnostic artifacts and does not run TLC.
`boundedLog.Write` has not yet been formally checked.

New tests cover data/boxed-error constructors, isolation/escape refusals, actual
standard-library source and four real TLC outcomes (two passes, a deadlock and a
synchronization error). The total is 99 semantic TLC cases plus 11 TLC CLI cases;
the full CLI refusal regression remains separate. The strict pinned gate and full
internal/CLI/integration/component race suite passed locally with zero skips/failures,
without changing original snapshots. Evidence is in
`$HOME/tmp/pi/gotla-bootstrap-data/{gate.log,race.log,self-check.log,cli/result.json}`.
This increment has not been pushed or claimed as a hosted CI pass.

## Bootstrap prerequisite: exact local interface dispatch

The next increment refines the explicit call graph for SSA interface values boxed
in the calling function with a proved concrete non-generic named receiver. The
implementation inspects the actual method body, binds its concrete receiver before
ordinary arguments and retains the box/receiver in the consumed slice even for a
pure method. No methods are trusted merely because their names resemble `Write`.
Deleted or incorrect graph edges and missing receiver dependencies fail closed.

Ten real TLC cases exercise channel argument positions, shared/distinct mutexes,
cross-worker unlock, invalid unlock, joined workers, named-channel receivers,
interface widening, pure methods and blocked method cleanup. Refusal coverage
includes interface parameters/fields/phis, returned/nil interfaces, promotions,
pointer/value adaptation, generics, recursive/effectful bodies, copies, channel
object escapes and primitive trust bypass. The total is now **109** semantic TLC
cases plus the unchanged **11** TLC CLI cases and separate self-input refusal test.
Original snapshots/sizes, the strict local gate and the full race suite pass, with
zero skipped/failing tests. This increment has no claimed hosted run.

This is a prerequisite, **not resolution of stored writer/cancellation callbacks**.
The repeated CLI self-check still returns `unsupported / 5` with 240 initializer
errors and the same other error counts; it emits no resolved-interface diagnostic
on its currently lowered path and does not run TLC. It would be misleading to
claim improved CLI coverage from the fixture results alone. `boundedLog.Write`
remains unverified. Evidence:
`$HOME/tmp/pi/gotla-bootstrap-interface/{gate.log,race.log,self-check.log,cli/result.json}`.

Next work must prove finite receiver/callback bindings through the component's
actual fields or choose another explicit component-entry contract without erasing
I/O, cancellation or re-entry behavior. The direct-box proof does not authorize
such an extension by itself.

## Bootstrap prerequisite: immutable callable fields

A new proof follows a directly allocated object's pointer through unambiguous
ordinary calls and checks every object/selected-field use. A direct interface or
function field may be initialized once, before every read and object call/spawn.
Callees cannot replace or initialize it; opaque aliases and escapes are refused.
Interface boxing, named functions and source closures produce exact targets, not
an implementer-count guess. The graph is refined in a separate stage, and target
queries recheck the complete proof. Lowering binds receivers/captures in each
allocation invocation and consumes the initializer and reader dependency slices.

This handles a mutex-bearing writer calling a stored interface and a stored callback
with channel/synchronization effects. Ten additional real TLC cases cover delivery,
blocked callbacks retaining a lock/defer, callback synchronization errors, receiver
aliasing, and distinct constructor invocations/captures. Refusal tests cover future/
conditional/multiple stores, callee mutation, escaped field addresses, object copies/
resets, returned/global/phi objects, ambiguous origins, nil/opaque initializers,
method wrappers and mutable/future captures. Graph corruption, missing frame bindings
and missing slice facts fail closed. A 1024-step bound on each origin/use proof
refuses excessively long alias chains rather than assuming their safety.

The local total is now **119 semantic TLC cases plus 11 TLC CLI cases** and the
separate CLI refusal test. Strict pinned-TLC/vet/snapshot and full race suites passed,
with zero skips/failures. An initial gate exposed an ambiguous test lookup for a
method named Run (a standard-library method could be selected); the test now also
matches its package, passed 20 repetitions, and the complete gate/race reruns passed.
Logs, including the failed first attempt, are under `$HOME/tmp/pi/gotla-bootstrap-fields/`.
This increment has not been pushed or claimed as a hosted pass.

The repeated full-CLI probe still reports `unsupported / 5`, 240 initializer errors
and unchanged other error categories, with no resolved field-call diagnostic on its
lowered path. It emits only diagnostic artifacts; `boundedLog.Write` remains
unverified in its actual caller environment. A concrete next step is to isolate its
unchanged locking implementation in a production leaf package and supply a small
finite caller/writer/cancellation harness; copying a lock skeleton or trusting I/O
would not count. The existing checker path must use the same extracted production
implementation, with native behavior/race regressions retained.

## First production-component bootstrap

The implementation was extracted to `internal/boundedlog/writer.go`; `checker.Run`
now constructs that Writer. A comparison against the original method confirmed the
body is identical after receiver/field renaming and replacing its global error read
with an injected `LimitError`. The runner still supplies the **same original error
sentinel**, file writer, byte budget and context cancellation function. Lock/defer,
truncation, error precedence and callback order were not rewritten. The new leaf
imports only sync; its concrete callers supply I/O and cancellation dependencies.

`examples/bootstrap/boundedlog/main.go` imports this production package. It provides
exactly two concurrent one-byte writes, a returning sink and a cancellation callback
that sends to a capacity-two channel. Each invocation can cancel once. The model
contains **3 processes, 1 mutex, 1 waitgroup and 1 channel**. It has **8 abstracted
data predicates**, reports `conservatively-abstracted`, and inspects both stored call
targets. It does not mark the writer or cancellation function pure by contract.

The initial `TestCheckProductionBoundedLog` increment added three actual CLI/TLC cases:

| Environment / mutation | Exit / status | Locations / transitions | Generated / distinct states |
|---|---|---|---|
| Production Writer, returning sink, bounded notifications | 0 / passed | 36 / 55 | 531 / 259 |
| Same source with only deferred Unlock removed | 3 / deadlock | 32 / 45 | 137 / 72 |
| Production Writer, zero budget, blocking notification | 3 / deadlock | 36 / 55 | 128 / 67 |

The mutation test parses the actual production file, removes the one Unlock defer
in an isolated temporary module, preserves source positions and records both hashes.
It does not edit the working tree or a dependency cache. Assertions require effects
located in `internal/boundedlog/writer.go`, both invocations' proved writer/callback
dispatches, a nonempty model, real checker evidence, artifact hashes, no trusted calls
and source candidates for counterexamples. The blocking environment is checked, not
executed natively: its unbuffered notification has no receiver, and its zero budget
forces an error on a nonempty write.

Native tests separately cover byte truncation, exact/empty/zero budgets, short writes,
output errors, cancellation under the mutex, retained error state after later success
and 64 concurrent writes sharing one budget. The ordinary finite harness also runs
natively. These tests and the checker runner's existing subprocess/limit/cancellation
regressions pass under the race detector.

Reproduce the two checked-in environments (never `go run` the blocked one):

```sh
go run ./cmd/gotla check -out /absolute/path/good \
  -tlc-jar "$TLC_JAR" ./examples/bootstrap/boundedlog
# 0

go run ./cmd/gotla check -out /absolute/path/blocked \
  -tlc-jar "$TLC_JAR" ./examples/bootstrap/boundedlog/blocked
# gotla returns 3; go run itself reports a nonzero wrapper exit.

GOTLA_REQUIRE_TLC=1 go test ./cmd/gotla \
  -run '^TestCheckProductionBoundedLog$' -count=1 -v
```

**Scope:** this establishes the modeled lock/callback protocol only for the supplied
finite environments. It is not a proof of byte budgets, error values, file contents,
race freedom, fairness or termination. It does not model arbitrary `os.File`,
`os/exec`, context propagation, synchronous re-entry or other callers. Abstract
counterexample traces are not automatic concrete Go replays. The blocking case
shows why a returning-callback assumption matters; extraction alone makes neither
I/O nor cancellation safe. Independent tests remain necessary.

The strict checksum-pinned gate and full race suite (including bootstrap examples)
passed locally with zero skips/failures and unchanged original snapshots. Totals are
119 semantic and 14 TLC CLI cases. Persistent logs and result/model/TLC artifacts are
under `$HOME/tmp/pi/gotla-bootstrap-production/`, including `good`, `missing-unlock`,
`blocking-cancel`, `cli`, `mutation.json`, `gate.log` and `race.log`. These bootstrap
changes have no claimed hosted pass. The repeated complete-CLI probe still returns
unsupported / 5, with 240 initializer errors and the same other diagnostic counts;
it does not run TLC.

## Blocking output and re-entry boundaries

Two further environments exercise the production Writer with a named-channel
receiver stored in its Output interface. Its actual Write body receives a permit
before returning. No I/O method is declared pure or assumed to return:

| Environment | Status / exit | Processes / channels | Locations / transitions | Generated / distinct states |
|---|---|---|---|---|
| `examples/bootstrap/boundedlog/io` | passed / 0 | 4 / 2 | 45 / 62 | 1780 / 719 |
| `examples/bootstrap/boundedlog/io/blocked` | deadlock / 3 | 3 / 2 | 38 / 55 | 58 / 38 |

Both use the unchanged production Writer, one mutex, one waitgroup and eight
abstracted predicates. The passing environment joins two writers and a releaser
that supplies exactly two permits. The blocked environment omits the releaser;
output waits while holding the production mutex. Cancellation still uses bounded
notifications. This checks explicit I/O scheduling, not real filesystem behavior.
The passing environment also executes natively under the race detector; never run
the blocked environment natively.

`TestCheckProductionBoundedLog` now covers all five TLC cases. In addition to the
existing production-effect/dispatch assertions, it checks both output receivers'
source-located receive transitions (delivery and closed-channel alternatives).
An initial focused run caught two test-expectation mistakes: there are four receive
transitions, not two; and the re-entry refusal below is located at the production
identity use rather than the harness capture. Expectations now match the explicit
IR/proof diagnostics; the full gate/race reruns passed. The failed first log is kept.

`examples/bootstrap/boundedlog/reentrant` captures the writer pointer in its own
cancellation callback and calls Write again while the outer lock is held. The
current initialization/alias proof cannot establish that capture. The separate
`TestCheckReentrantLoggerBoundary` requires **unsupported / 5**, the capture-order
and source-located production identity diagnostics, no trust or TLC invocation, and
no stale executable artifacts. A nonexistent JAR confirms that this is an analysis
refusal, **not a checked deadlock or pass**. Never execute this environment natively.

The full checksum-pinned gate and race suite passed locally with zero skips/failures;
original snapshots are unchanged. Totals are 119 semantic and 16 TLC CLI cases, plus
the two refusal regressions. Logs and real CLI artifacts are in
`$HOME/tmp/pi/gotla-bootstrap-io/` (`released`, `blocked`, `reentry`, `gate.log`,
`race.log`, `focused-attempt1.log`). These remain local-only results. The complete
CLI is still unsupported; neither production nor analyzer semantics changed in this
environment-coverage increment.

## Second production component: source capture

`frontend.LoadContext` now calls `internal/sourcecapture.Collector.Record` from its
actual ParseFile callback. The collector preserves the original order: lock, clone
source bytes, store by filename, unlock. Parsing remains outside the lock. The
frontend reads and clears the captured map after package loading returns, as before.
The collector is not a replacement for `parser.ParseFile` or `go/packages`.

One sequential dependency deliberately changed: the leaf uses **slices.Clone** in
place of bytes.Clone. An isolated bytes.Clone capture probe was refused by the
unchanged initializer checker because the bytes dependency brings in reflection
initialization from `errors/wrap.go`. No initializer or unsafe check was waived.
The production leaf actually uses slices.Clone; the model inspects that installed
source and its ordinary builtin append, not a trusted or overlay replacement.
Native tests compare nil/empty/content behavior with bytes.Clone, verify independent
nonempty storage, replacement by filename and 64 concurrent captures. Slice capacity
and empty-slice backing-storage retention are explicitly not part of the capture
contract; this is not a claim of identical allocation behavior between the two APIs.

`examples/bootstrap/sourcecapture/main.go` supplies two finite concurrent callers to
this same production collector. `TestCheckProductionSourceCapture` adds two real
CLI/TLC cases, requiring no trusted calls, source-located production Lock/Unlock
effects, checked artifact hashes and source candidates for the mutant:

| Source | Status / exit | Locations / transitions | Generated / distinct states |
|---|---|---|---|
| Production Collector.Record | passed / 0 | 18 / 19 | 97 / 51 |
| Same source with only Unlock removed | deadlock / 3 | 16 / 19 | 39 / 23 |

Both models have three processes, one mutex, one waitgroup, no channels and no
abstracted control predicates. Zero predicates does **not** mean map contents, byte
copying or general race freedom are verified: these remain sequential data outside
the synchronization projection. The harness does not establish real package-loader
callback scheduling, parsing semantics, error recovery or the full frontend lifecycle.
A mutation runs only in an isolated temporary module; the real source is unchanged.
The initial test expectation counted one direct Unlock transition per caller; the
IR has normal and error alternatives. The corrected assertion checks both, and the
failed first focused log is retained.

The full pinned gate and race suite passed locally with zero skips/failures, including
native frontend/loop normalization and capture tests. Original snapshots are unchanged.
There are now **119 semantic TLC cases and 18 TLC CLI cases**, plus full-CLI/re-entry
refusals. The repeated CLI probe remains unsupported / 5 with 240 initializer errors
and unchanged other categories. No analyzer acceptance rule was broadened.
Evidence: `$HOME/tmp/pi/gotla-bootstrap-capture/`, including the refused `probe-result`,
`production`, `missing-unlock`, `cli`, `mutation.json`, `gate.log` and `race.log`.
These results are local-only; bootstrap changes have no claimed hosted pass.

## Full-self prerequisite: static deferred helpers

Normal-return cleanup now supports source-defined static functions/methods and
literal closures, with inspected bodies, registration-time resource bindings,
LIFO flags and ordinary nested frames. A blocking helper prevents earlier cleanup;
missing graph/slice proofs still reject. Twelve new TLC cases exercise these effects,
including bad LIFO order and blocked cleanup. Dynamic/interface/field targets,
wrappers, initializer defers and panic/recover remain unsupported.

The complete CLI now gets past its original `defer dir.Close()` rejection and exposes
more downstream loop, runtime synchronization and effect restrictions. It still
returns unsupported / 5, with no executable self-model. Diagnostic growth is not a
regression-count or coverage metric. The strict local gate and full race suite pass
with 131 semantic and 18 CLI/TLC cases and unchanged original snapshots. See
[the full-goal audit](SELF_BOOTSTRAP_GOAL.md) for concrete remaining gates and evidence.

## Full-self prerequisite: finite read-only data loops

Length-bounded externally read-only computations now have a typed-SSA termination
and effect proof, without unrolling or trusted-call shortcuts. It is consumed on the
actual `toolinfo.fromBuildInfo` and `behavior.Model.HasErrors` bodies during CLI
self-analysis. Results remain abstract; argument effects are still evaluated. Eight
new real TLC cases and graph/slice/budget/refusal tests bring totals to 139 semantic
and 18 CLI/TLC cases. Strict gate and full race tests pass with unchanged snapshots.
The complete CLI still returns unsupported / 5: this is not the full goal. Details
and evidence are in [SELF_BOOTSTRAP_GOAL.md](SELF_BOOTSTRAP_GOAL.md).

## Full-self prerequisite: pre-closed global channel state

The actual `context.closedchan` allocation and initialization close now have a consumed
unique-initialization proof. Its empty/closed state is persisted in IR and honored by
TLC; unknown initializer effects remain refusals. Eight additional real TLC cases,
including concurrent receives and a mutated initial-state deadlock, bring totals to
147 semantic and 18 CLI/TLC cases. Strict gate/race tests pass with unchanged original
snapshots. Initializer refusals fall from 240 to 238, but complete CLI self-analysis
still returns unsupported / 5. See the full-self contract for scope and evidence.

## Data-table prerequisite: private arrays and constant bounds

Private fixed-array writes and portable constant-bounded data loops now have consumed
isolation/termination proofs. Shared/reference-bearing data and unproved aliases or
effects remain refused. Seven more TLC cases bring totals to 154 semantic and 18
CLI/TLC cases; the strict gate and race suite pass. This increment does not yet remove
an actual self-input refusal: complete CLI analysis remains unsupported / 5 with 238
initializer errors. See the full-self contract for the remaining constructor obstacles.

## Full-self prerequisite: literal-input invocation evaluation

A bounded SSA evaluator now proves the two actual base64.NewEncoding initializer
calls, including private array copies and selected validation paths, without a name
allowlist. It only admits fully evaluated data-only invocations and rechecks the
current source/graph/argument proof. Results remain abstract in behavioral IR.
Nine new TLC cases bring totals to 163 semantic and 18 CLI/TLC cases; strict gate and
race tests pass. Native array comparison and malformed alphabet mutations test the
real constructor. The full CLI still returns unsupported / 5 with 236 initialization
refusals. See SELF_BOOTSTRAP_GOAL.md for evidence and remaining requirements.

A subsequent owned-reference increment proves the actual go/constant.newFloat
initializer with nil slice storage and SetPrec, reducing initializer refusals to 235.
Three additional TLC cases bring current totals to 166 semantic and 18 CLI/TLC cases.
Nil references never authorize external memory or synchronization-bearing type graphs.
Full self-input still refuses; see SELF_BOOTSTRAP_GOAL.md for the latest evidence.

Fixed-capacity open-global creation now proves the three actual x/tools I/O
semaphores (20/20/10). Fresh identity checks include exact capacity consistency for
both open and pre-closed channels. Seven new TLC cases bring current totals to 173
semantic and 18 CLI/TLC cases. Full self-input still refuses with 232 initializer
diagnostics; dynamic CPU capacities and other initialization effects remain unproved.

Read-only private value-copy proofs now cover the real Model.Statistics helper,
without treating copied references as ownership of their pointees/backing arrays.
Five additional TLC cases bring totals to 178 semantic and 18 CLI/TLC cases.
Unsupported-loop diagnostics fall from 73 to 70; initializer refusals remain 232.
Full bootstrap remains unsupported; latest evidence is in SELF_BOOTSTRAP_GOAL.md.

Scalar-array range normalization now proves the two actual artifact file-table
loops. The table is a fixed five-element array; I/O and error paths remain intact.
Seven new TLC cases and native transformation comparisons bring current totals to
185 semantic and 18 CLI/TLC cases. Loop refusals fall to 67, but more underlying
unsupported operations are exposed; this is not full-bootstrap completion.

The actual CLI's `os.Exit(code)` now lowers to an explicit immediate program-exit
instruction, not a pure-call exemption. It bypasses cleanup and terminates workers;
argument effects are retained. Eight independent IR/TLC tests and source-lowering
contract tests bring totals to 193 semantic and 18 CLI/TLC cases. Dependency
initializers are unchanged; full self-check still returns unsupported / 5 and emits
only the diagnostic model. Evidence: `gotla-self-bootstrap-exit/` under the local
`~/tmp/pi` evidence directory.

## Incremental acceptance plan (partial; full self-verification remains unsupported)

1. **Done:** keep the complete CLI self-input refusal test and its actionable diagnostics.
2. **Done for the bounded logger:** establish a real, restricted production-component entry/harness mechanism without
   copying its synchronization or adding broad trusted-call exemptions. Resolve the
   required concrete callbacks/identities and model or explicitly delimit effects.
3. **Done under the documented finite environments:** check the actual bounded-log locking protocol with two finite concurrent callers;
   retain the original body, test normal/error paths and a missing-unlock mutation.
   Require a nonempty synchronization model, actual TLC outcomes and source evidence.
   This still does not prove file bytes, cancellation semantics outside the stated
   environment, or every possible caller.
4. **Source-capture protocol done; full callback/lifecycle not done:** expand beyond
   the capture operation to actual parser/package-loader and runner lifecycles only
   with independent effect, initialization and callback proofs. Full CLI self-verification remains a separate,
   much larger milestone involving context, processes, I/O and dynamic control.

Even a successful future self-check would only establish the selected modeled
properties under assumptions. Independent semantic tests, backend contract tests,
race tests and external checker validation remain necessary; self-analysis is not
an independent proof that the analyzer or TLC is correct.
