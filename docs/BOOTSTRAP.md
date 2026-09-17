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
itself. It is separate from the 95 positive/negative semantic TLC cases and 11
TLC CLI cases. Future support improvements must deliberately revise this expectation
with real checker evidence, rather than delete refusals to turn the test green.

The checksum-pinned strict local gate passed with zero skips/failures after adding
this regression. The self-analysis test also passed under `-race`. Logs are in
`$HOME/tmp/pi/gotla-bootstrap/{gate,race}.log`. No production semantics or supported
syntax changed; a hosted run for this new regression has not yet been claimed.

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

## Incremental acceptance plan (not implemented)

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
