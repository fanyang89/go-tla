# Local acceptance and release boundary

## Status

M1–M5 have local acceptance evidence for the documented restricted profile.
This is not verification of arbitrary Go programs. The hosted GitHub Actions job
has **not** been run or verified from this workspace; branch protection, release
publication and remote CI success are not claimed. The remaining release gate is
an actual successful hosted run against the delivered revision, with its log retained.

## Component matrix

Each harness constructs resources and calls the real Go library component. It
neither replaces synchronization nor injects fake completion. All seven checks
have no custom trusted calls and zero abstracted control predicates; standard
initialization/domain assumptions are still explicit.

| Component / variant | Expected status / exit | Regression |
|---|---|---|
| workerpool/good | passed / 0 | `TestCheckWorkerPoolComponents/good` |
| workerpool/bad (missing Done) | deadlock / 3 | `TestCheckWorkerPoolComponents/bad` |
| pipeline/good | passed / 0 | `TestCheckPipelineComponents/good` |
| pipeline/missingclose | deadlock / 3 | `TestCheckPipelineComponents/missingclose` |
| pipeline/earlyclose | synchronization-error / 4 | `TestCheckPipelineComponents/earlyclose` |
| counter/good | passed / 0 | `TestCheckCounterComponents/good` |
| counter/bad (missing Unlock) | deadlock / 3 | `TestCheckCounterComponents/bad` |

The shared result validator checks artifact hashes and model/checker provenance.
Counterexamples require source candidates. Exact topology and location/transition
baselines are asserted per variant. [COMPONENTS.md](COMPONENTS.md) records source/
harness contracts, state counts, tool options, wall/checker times and RSS limitations.
Native correct-component tests cover sample numeric outputs under Go's race detector;
TLC does not prove arithmetic or general shared-memory race freedom.

## Milestone evidence

| Gate | Evidence |
|---|---|
| M1 semantic/engineering baseline | Pinned-JAR strict gate, vet, actual TLC, deterministic IR/TLA snapshots and eight original model-size baselines |
| M2 workflow | Actual success/deadlock/synchronization-error/timeout CLI tests; unsupported/load/tool failures, cancellation, output locks and stale-artifact tests |
| M3 IR/pass contract | Strict decoder/common validator, separate backend capabilities, graph/effect/slice contracts, saved-IR round trips and source naming tests |
| M4 patterns | Positive/refusal/semantic coverage for fields, normal-return defers and proved integer ranges; no silent expansion truncation |
| M5 components | Three component families and seven actual CLI/TLC outcomes above; close-driven receive SCCs and immutable inline-sync pointer captures have independent semantic/refusal tests |

The current local gate includes **95 semantic TLC cases and 11 CLI integration
cases** (four workflow cases plus seven component cases), with zero skips when run
through the strict script. Existing snapshots/model-size baselines remain unchanged.
Full internal/CLI/integration/component race tests also pass locally.

## Reproduce the local gate

From the repository root, using Go from `go.mod` and Java 25:

```sh
TOOLS="${TMPDIR:-$HOME/tmp}/pi/gotla-tools"
mkdir -p "$TOOLS"
bash scripts/download-tlc.sh "$TOOLS/tla2tools.jar"
TLC_JAR="$TOOLS/tla2tools.jar" bash scripts/test-ci.sh
TLC_JAR="$TOOLS/tla2tools.jar" go test -race \
  ./internal/... ./cmd/gotla ./tests ./examples/components/... -count=1
go build -o "$TOOLS/gotla" ./cmd/gotla
"$TOOLS/gotla" check -tlc-jar "$TOOLS/tla2tools.jar" -timeout=30s \
  -out "$TOOLS/counter-good" ./examples/components/counter/good/harness
```

The strict script verifies the upstream pinned JAR checksum, requires Java/TLC,
clears ambient `GOFLAGS`, refuses snapshot regeneration and compares golden files.
Plain `go test ./...` without TLC is insufficient. The checked-in workflow invokes
this strict script and archives its log, but configuration is not hosted execution
proof. No push, PR, workflow dispatch or branch-protection change was made here.

Latest local logs: `$HOME/tmp/pi/gotla-m5-counter-gate.log` and
`$HOME/tmp/pi/gotla-m5-counter-race.log`. Machine-specific logs/artifacts are not
committed. Historical evidence and atomic revisions are tracked in [ROADMAP.md](../ROADMAP.md).

## Interpretation and deferred work

A pass means no modeled deadlock/synchronization violation was found under the
recorded harness and assumptions. Main return ends the program; this is not leak,
starvation, fairness or termination verification. Receive loops have finite
**modeled state**, not a proved bound on runtime deliveries or concrete memory.
Implicit sequential panics/resource exhaustion are excluded; trusted contracts can
invalidate findings if false. Incomplete/tool-failed/unsupported runs are not passes.

Delivery order remains semantic soundness and regression gates first, then checking
workflow/contracts, supported production patterns and component validation. Next
capabilities require new proofs/tests—not broader acceptance by name alone. General
aliasing, dynamic topology, RWMutex, WaitGroup reuse/Go, arbitrary temporal/payload
properties, panic/recover, counterexample replay and additional backends remain
outside this acceptance. See [SUPPORTED_GO.md](SUPPORTED_GO.md) and
[VERIFICATION.md](VERIFICATION.md) before applying results to another component.
