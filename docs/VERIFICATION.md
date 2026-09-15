# Verification workflow and result interpretation

## Extract, inspect, check

The current CLI has two commands; `gotla check` is planned, not implemented.

```sh
go build -o gotla ./cmd/gotla
./gotla inspect ./examples/unbuffered
./gotla analyze -out out ./examples/unbuffered
```

`inspect` prints processes, synchronization resources, transitions, abstractions,
and assumptions. `analyze` writes `model.json`, `model.tla`, and `model.cfg`, then
prints a copyable POSIX-shell command for TLC. Neither command runs TLC itself.
Their model-size summary counts **behavioral IR objects**, not reachable TLC states
or backend runtime actions. Partial unsupported models are labeled as partial by
`inspect`; do not use their counts as a complete program inventory.

Provision Java and the official TLC JAR explicitly. For a reproducible Linux/Bash
setup matching CI:

```sh
mkdir -p "$HOME/tmp/gotla-tools"
bash scripts/download-tlc.sh "$HOME/tmp/gotla-tools/tla2tools.jar"
export TLC_JAR="$HOME/tmp/gotla-tools/tla2tools.jar"
./gotla analyze -out out ./examples/unbuffered
# Copy the emitted Run TLC command, or run:
(cd out && java -cp "$TLC_JAR" tlc2.TLC -workers 1 model.tla)
```

The provisioning script downloads only on explicit invocation. It uses the
version/digest in [scripts/tlc.env](../scripts/tlc.env), checked against the official
GitHub release API, and stages the download before accepting its checksum. An
existing mismatched file fails verification and is not overwritten. The JAR is
not committed to the repository. `analyze` and tests never download it.

Manual model checking may use another explicitly chosen version; the strict
verification gate requires the pinned digest. Provisioning failure means the gate
did not run, not that the Go component passed.

## Interpret the two separate outcomes

First inspect extraction diagnostics and `model.json`'s outcome/assumptions.
Then interpret TLC separately:

| Observation | Interpretation |
|---|---|
| `No error has been found` with completed successful TLC execution | No modeled violation found in the explored finite model under its recorded assumptions |
| `Deadlock reached` | A reachable model state has no enabled action while main remains active; inspect the state trace and abstraction assumptions |
| `Invariant NoSynchronizationErrors is violated` | A modeled invalid channel, mutex, or WaitGroup operation is reachable |
| Unsupported extraction | No safe executable model was established; do not run a previous model as if it were this analysis |
| Java missing, invalid JAR, parsing error, process failure | Tool/setup failure, not a verified counterexample or pass |
| Timeout, memory exhaustion, cancellation, incomplete search | No completed verification result; never report as pass |

The current CLI uses status 0 for successful extraction/inspection, 1 for analysis
or output errors/unsupported input, and 2 for argument errors. It does not yet
normalize TLC exits into its own result categories. Do not classify every nonzero
TLC exit as a program bug; inspect its reported reason. Deadlock and invariant
counterexamples are also expected nonzero outcomes in the integration tests.

Unknown predicates, payload erasure, and lost shared-memory correlations may
produce false positives. “Precisely modeled” is relative to the supported domain,
not arbitrary Go correctness. Neither fairness/starvation nor goroutine leaks nor
payload correctness are checked. Main return ends the program even if a worker is
blocked. False trusted-call contracts invalidate conclusions.

## Artifacts and source references

Keep the source revision, command, `model.json`, `.tla`, `.cfg`, checker output,
Go/Java/TLC versions, and resource options together when investigating a result.
The JSON contains extraction diagnostics and assumptions; transitions carry source
metadata, and generated actions/comments provide source references. Complete trace
decoding/replay and structured checker-result files are future work.

Use a separate output directory per investigation. Unsupported extraction removes
stale `.tla`/`.cfg` files and writes diagnostic JSON, but early package-load failures
can leave previous output untouched. Check exit status and artifact provenance;
file existence alone is not evidence of successful current extraction.

## Tests and required CI gate

Fast local tests remain available without Java/TLC:

```sh
go test ./...
go vet ./...
```

Without `TLC_JAR`, the actual model-checker tests explicitly skip. **That run alone
is not the semantic verification gate.** To enforce checking without allowing a
missing checker to skip:

```sh
GOTLA_REQUIRE_TLC=1 TLC_JAR=/absolute/path/tla2tools.jar go test ./... -count=1
```

For the same strict, pinned gate used by CI:

```sh
TLC_JAR=/absolute/path/tla2tools.jar bash scripts/test-ci.sh
```

The script verifies the official digest and Java availability, prints tool
versions, clears ambient `GOFLAGS` that could filter tests, runs vet and the full
uncached verbose suite with required TLC, and checks that tracked snapshots were
not modified. Snapshot regeneration is forbidden in this gate. Each TLC test has
a 30-second deadline, 512 MiB heap, one worker, and checks outcome text **and** exit
status. Checker failure/timeouts fail tests.

[The GitHub Actions workflow](../.github/workflows/ci.yml) provisions Go from
`go.mod`, Java 25, and the pinned TLC; the job has a 15-minute timeout and preserves
its verification log when available. Configure the repository's branch protection
to require **Go and required TLC** separately; adding a workflow does not itself
change hosting policy. A local gate pass is not evidence that GitHub-hosted CI ran.

## Snapshots and regression baselines

- IR and TLA snapshots: `tests/testdata/{unbuffered,select}.{json,tla}`.
- All eight examples' IR size baseline: [model-sizes.json](../tests/testdata/model-sizes.json).
- All eight examples are checked for repeated-analysis IR/diagnostic/TLA/config
  determinism. This is not a promise of stable IDs across source/toolchain changes.
- The sequential example and its simple communication counterpart each have five
  IR transitions; extra local computation must not introduce scheduling actions.

Intentional baseline changes require semantic review, not automatic acceptance:

```sh
UPDATE_SNAPSHOTS=1 go test ./tests -run '^TestExamplesAndSnapshots$'
git diff -- tests/testdata
```

Review every baseline change, then run the strict gate with regeneration disabled.
New semantic features need successful, faulty, and rejected cases. See the
[coverage matrix](SEMANTIC_TEST_MATRIX.md) for current evidence and gaps.
