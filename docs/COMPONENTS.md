# Component acceptance (M5, in progress)

## Finite batch worker pool

The source under `examples/components/workerpool/{good,bad}/pool.go` is a real Go
library component, not an IR fixture. It starts two workers, submits two jobs,
collects two results and waits for completion. Each worker handles one job. This
is a **single-use finite batch pool**, not a long-lived worker service or a claim
that arbitrary receive loops are supported.

The `harness/main.go` files only allocate distinct unbuffered channels and invoke
`RunBatch`. All spawning, job/result communication and completion synchronization
remain inside the component. Neither harness supplies a fake completion signal or
replaces a concurrent operation. The bad component differs by omitting deferred
`Done`: results still arrive, but `Wait` cannot complete. It must report deadlock,
not merely parse as a valid model.

API preconditions: exactly one caller, one invocation, initially empty distinct
channels, and no external channel users. Ordinary implicit-panic/resource-exhaustion
exclusions and the main-return termination policy still apply. There are no custom
trusted calls or abstracted predicates. Standard `sync` initialization uses the
existing documented environment summary. Add/Done are intentional here: the
checker does not support `WaitGroup.Go`, and the fault under test is lost completion.

Payload arithmetic is not a checked property. A native Go test checks the result
84; TLC checks the communication/synchronization projection, not the arithmetic.
The faulty variant is model-checked, not executed as a native test that would hang.

### Reproduce

From the repository root, with the pinned JAR provisioned as in
[VERIFICATION.md](VERIFICATION.md):

```sh
go build -o ./gotla ./cmd/gotla
TLC_JAR="$HOME/tmp/pi/gotla-m1-tools/tla2tools.jar"
./gotla check -tlc-jar "$TLC_JAR" -timeout=30s \
  -out "$HOME/tmp/pi/workerpool-good" \
  ./examples/components/workerpool/good/harness
# Expected exit 0, result.json status "passed".
./gotla check -tlc-jar "$TLC_JAR" -timeout=30s \
  -out "$HOME/tmp/pi/workerpool-bad" \
  ./examples/components/workerpool/bad/harness
# Expected exit 3, result.json status "deadlock".
```

`TestCheckWorkerPoolComponents` runs both through the CLI entry point and real TLC,
checking exit/status, topology, source candidates, no hidden trusted predicates,
and artifact hashes/provenance via the shared result validator. The strict test
gate requires TLC; these tests must not silently skip there.

### Local reference measurement

Measured from the M5 worker-pool change based on `597e505` (working-tree build).
Linux x86_64, AMD Ryzen 9 9955HX, Go 1.26.2, OpenJDK 25.0.4.1 (Red Hat).
Pinned TLC JAR SHA256:
`2c903dcd6f50f12b0c0a2e14c1406be782126c7cee64f3e0b8550c2020870293`.
Its reported version is `TLC2 Version 2026.09.14.200039 (rev: 5dbdb42)`.
Options: one worker, 512 MiB JVM heap, 30 s checker deadline, 16 MiB log limit.
These limits do not bound the entire Go frontend or total process-tree memory.

| Variant | Processes/channels/WGs | Locations/transitions | Generated/distinct states | TLC elapsed | Full command wall time | GNU time max RSS (KiB) | Result |
|---|---|---|---|---|---|---|---|
| good | 3/2/1 | 22/21 | 151/78 | 625 ms | 1.06 s | 149636 | passed / 0 |
| bad | 3/2/1 | 18/15 | 76/42 | 612 ms | 1.06 s | 147000 | deadlock / 3 |

Both have zero mutexes and zero abstracted predicates. State counts are one
reference run, not a scheduling-independent performance guarantee. GNU
`/usr/bin/time -v` wraps the built CLI, including child execution; its reported
maximum RSS is **not** a sampled aggregate peak of concurrently live processes.
Raw local evidence is under `$HOME/tmp/pi/gotla-m5-workerpool/`: per-variant
`result.json`, `model.json`, TLA/config, TLC log, stdout and `.time` measurements.
Those machine-specific artifacts are not committed.

The measured model sizes establish component regression baselines. Time/RSS remain
observations rather than brittle CI thresholds; the checker retains its explicit
30-second incomplete outcome instead of treating a timeout as success.

## Remaining M5 work

- A genuine close-driven pipeline and an incorrect-close/missing-communication
  variant. Channel ranges/general receive loops remain unsupported; implement the
  necessary semantics/proof before claiming this component passes. Replacing a
  close-driven loop with a guessed receive count is not acceptable evidence.
- A struct-based mutex-protected component with a reproducible faulty variant.
- A consolidated acceptance report and delivery gate. This first pair alone does
  not complete M5 or the basically-usable milestone.
