# Acceptance evidence and release boundary

## Status

M1–M5 have local and hosted acceptance evidence for the documented restricted profile.
Revision `f19df4d` passed [hosted run 35173519428](https://github.com/fanyang89/go-tla/actions/runs/35173519428).
Its retained `verification-log` artifact was downloaded and checked: zero skips,
zero failed tests, with all three component groups passing. The run used Go 1.26.2,
Temurin Java 25.0.4.1 and the refreshed pinned TLC hash below.
This is not verification of arbitrary Go programs. The first hosted push run for
`c265c49` [failed workflow validation](https://github.com/fanyang89/go-tla/actions/runs/35171701731):
job-level `env` cannot reference `runner.temp`, so no verification job started.
The fix sets `TLC_JAR` in a shell step through `GITHUB_ENV` instead.
The [next hosted run](https://github.com/fanyang89/go-tla/actions/runs/35172916256)
started correctly but rejected an upstream-replaced TLC asset before testing.
With owner approval, the pin now uses official release asset `569238611`, SHA256
`066cd246d87a388dfde0f04c3b506007f4c0cb4708a5b5396f0552a005eb75b5`, verified
against the upstream release API. The strict local gate and full race suite pass
with this asset; original snapshots remain unchanged. Historical component timing
and state measurements retain their original checker hash, not the refreshed one.
The hosted verification gate is now satisfied for `f19df4d`; subsequent revisions
must pass their own runs. Branch protection and release publication are not claimed.
The workflow retains logs for 14 days; a local copy is saved under
`${TMPDIR:-$HOME/tmp}/pi/gotla-hosted-35173519428/verification.log`.
Non-failing action-runtime/setup-java deprecation warnings remain maintenance work.

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

The delivered hosted baseline included **95 semantic TLC cases and 11 CLI integration
cases** (four workflow cases plus seven component cases), with zero skips.
The later bootstrap constructor, exact-interface and immutable-field increments
plus static deferred helpers, finite data computations, pre-closed global channels
and private-array/literal-input computations bring the local semantic total to **163**.
Production bootstrap adds seven CLI/TLC
cases, bringing that total to **18**. The five logger cases cover immediate/released
output passes and missing Unlock, blocked cancellation/output deadlocks, with eight
explicit abstracted predicates. Two source-collector cases check the actual frontend
capture protocol and its missing-Unlock mutant, with no abstracted control predicates.
Neither family verifies data contents or general race freedom. The full CLI self-input
test still expects unsupported. See [BOOTSTRAP.md](BOOTSTRAP.md) for local-only evidence
and remaining blockers. Original snapshots/model-size baselines remain unchanged.
Full internal/CLI/integration/component/bootstrap race tests also pass locally.

## Reproduce the local gate

From the repository root, using Go from `go.mod` and Java 25:

```sh
TOOLS="${TMPDIR:-$HOME/tmp}/pi/gotla-tools"
mkdir -p "$TOOLS"
bash scripts/download-tlc.sh "$TOOLS/tla2tools.jar"
TLC_JAR="$TOOLS/tla2tools.jar" bash scripts/test-ci.sh
TLC_JAR="$TOOLS/tla2tools.jar" go test -race \
  ./internal/... ./cmd/gotla ./tests ./examples/components/... ./examples/bootstrap/... -count=1
go build -o "$TOOLS/gotla" ./cmd/gotla
"$TOOLS/gotla" check -tlc-jar "$TOOLS/tla2tools.jar" -timeout=30s \
  -out "$TOOLS/counter-good" ./examples/components/counter/good/harness
```

The strict script verifies the upstream pinned JAR checksum, requires Java/TLC,
clears ambient `GOFLAGS`, refuses snapshot regeneration and compares golden files.
Plain `go test ./...` without TLC is insufficient. The checked-in workflow invokes
this strict script and archives its log, but configuration is not hosted execution
proof. The owner configured the remote, pushed the initial revision, and authorized
subsequent repair pushes. No PR, workflow dispatch or branch-protection change was
made here.

Latest full-self prerequisite logs: `$HOME/tmp/pi/gotla-self-bootstrap-constant-data/{gate,race}.log`.
Private-array logs remain under `gotla-self-bootstrap-array-data/`.
Pre-closed-global logs remain under `gotla-self-bootstrap-closed-globals/`.
Initializer-contract logs remain under `gotla-self-bootstrap-init-targets/`.
Finite-data logs remain under `gotla-self-bootstrap-finite-data/`.
Deferred-helper logs remain under `gotla-self-bootstrap-defer/`.
Source-capture logs remain under `$HOME/tmp/pi/gotla-bootstrap-capture/`.
The full objective remains unfulfilled; see [SELF_BOOTSTRAP_GOAL.md](SELF_BOOTSTRAP_GOAL.md).
Earlier logger logs remain under `gotla-bootstrap-production/` and `gotla-bootstrap-io/`.
The delivered hosted-baseline refresh logs remain at
`${TMPDIR:-$HOME/tmp}/pi/gotla-tlc-refresh-{gate,race}.log`. Machine-specific logs/artifacts are not
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
