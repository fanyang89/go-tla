# Self-analysis / bootstrap feasibility

## Observed result

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
itself. It is separate from the 109 positive/negative semantic TLC cases and 11
TLC CLI cases. Future support improvements must deliberately revise this expectation
with real checker evidence, rather than delete refusals to turn the test green.

The checksum-pinned strict local gate passed with zero skips/failures after adding
this regression. The self-analysis test also passed under `-race`. Logs are in
`$HOME/tmp/pi/gotla-bootstrap/{gate,race}.log`. That initial self-input-test commit
did not change production semantics or supported syntax; the constructor increment
below is a separate capability change. Neither increment has a claimed hosted pass.

## Best first production target

The most useful next target is `internal/checker/runner.go`'s `boundedLog.Write`:
it takes an inline Mutex, defers Unlock, writes through an `io.Writer`, records an
error and invokes a cancellation callback. Its lock belongs to actual production
code, unlike copying a small locking example into a bootstrap directory.

This is not yet a supported component harness. Obstacles include private entry
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

## Incremental acceptance plan (partial; full self-verification remains unsupported)

1. Keep the complete CLI self-input refusal test and its actionable diagnostics.
2. Establish a real, restricted production-component entry/harness mechanism without
   copying its synchronization or adding broad trusted-call exemptions. Resolve the
   required concrete callbacks/identities and model or explicitly delimit effects.
3. Check the actual bounded-log locking protocol with two finite concurrent callers;
   retain the original body, test normal/error paths and a missing-unlock mutation.
   Require a nonempty synchronization model, actual TLC outcomes and source evidence.
   This still does not prove file bytes, cancellation semantics outside the stated
   environment, or every possible caller.
4. Expand to the parser callback and runner lifecycle only with independent effect,
   initialization and callback proofs. Full CLI self-verification remains a separate,
   much larger milestone involving context, processes, I/O and dynamic control.

Even a successful future self-check would only establish the selected modeled
properties under assumptions. Independent semantic tests, backend contract tests,
race tests and external checker validation remain necessary; self-analysis is not
an independent proof that the analyzer or TLC is correct.
