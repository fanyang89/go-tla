# Full self-bootstrap acceptance contract

## Objective and current status

Objective: go-tla must produce executable TLA+ from its own real Go program and
provide reproducible verification evidence. **Not complete.** Existing logger and
source-capture component checks are useful prerequisites, not substitutes for this
objective. They do not authorize declaring a whole-program self-check successful.

The reference source entry is `./cmd/gotla`, not a separately rewritten coordinator.
A finite input/environment profile may be necessary, but must be explicit, tied to
actual source behavior and enforced by the analyzer. It must not remove troublesome
initializers, replace production calls with trusted no-ops, or truncate behavior
silently. A passing bounded concurrency model is not a theorem of compiler soundness,
functional correctness or arbitrary Go executions.

## Required end-to-end evidence

- [ ] The actual CLI entry and relevant production/dependency effects are analyzed;
      all accepted operations have consumed proofs or explicit, justified model
      semantics. Unresolved calls/identity/control continue to fail closed.
- [ ] Input, environment, finite-state bounds, excluded runtime behavior and checked
      properties are recorded. The model represents the real operation, not merely
      an empty/help path or the two existing component harnesses.
- [ ] The normal source -> SSA -> behavioral IR -> TLA+ pipeline emits executable
      self-model artifacts with nonempty meaningful synchronization and source/provenance
      evidence. A diagnostic-only model or unsupported exit does not satisfy this.
- [ ] The pinned actual TLC checks the generated self-model to completion. Results,
      checker identity, artifact hashes and any abstractions are inspectable.
- [ ] Mutating actual production synchronization produces the expected counterexample
      or justified refusal; the model must not remain vacuously successful.
- [ ] The self-analysis CLI regression is upgraded only with that evidence. Existing
      independent semantic/IR/backend/refusal tests, strict gate and full race suite
      pass without skipped required TLC checks or unreviewed snapshot regeneration.

The full-CLI regression currently correctly requires unsupported / 5 and no executable
artifacts. No completion is claimed until every item above has direct evidence.

## Reproducible attempt capture

`scripts/capture-self-check.py` captures the actual `./cmd/gotla` attempt from a clean
checkout into a new external directory. It checks the repository-pinned TLC digest,
records only selected build environment settings (not the entire environment),
fingerprints tracked files, selected dependency sources/module files, Go compiler/
assembler/linker and JAR, builds the real CLI, and executes its normal check pipeline.
Startup GOMAXPROCS and the analysis profile agree. Before/after fingerprints, git
revision/cleanliness, binary identity, result/model provenance and artifact hashes
must match. Unsupported results must contain no executable model or TLC evidence.

`manifest.json` retains command arguments, stdout/stderr hashes, capture failures and
the actual checker outcome. Existing evidence directories are never overwritten;
subprocess deadlines terminate their process groups. It returns the original checker
exit code for a valid capture, including 5 for unsupported, and 1 for capture failures.
The manifest always records `goalComplete: false`: even a future checker pass does
not by itself establish finite input/I/O/lifecycle scope or production mutation
sensitivity. This is evidence infrastructure, not an alternative bootstrap model.
Concurrent writers are excluded; selected source/tool fingerprints are not a native
header/toolchain closure, full host snapshot, or memory/disk sandbox.

The strict gate includes Python-standard-library tests rejecting stale versions,
profiles, trusted calls, hashes/paths, symlinks, unsupported/TLC contradictions and
incomplete success evidence. Full compiler semantics and bootstrap acceptance above
remain unchanged. Python 3 is required; no new package download is needed.

## Static deferred helpers (first full-self prerequisite)

Static, source-defined deferred helper/closure bodies are now analyzed at normal-return
cleanup. Registration captures identities; LIFO cleanup enters the actual helper
body and waits for its normal return before earlier cleanup can proceed. Primitive
Unlock/Done behavior and existing snapshots are unchanged. Dynamic/interface/field
defer targets, bound wrappers, repeated registration, foreign stacks and panic/recover
remain outside this increment. Trusting a deferred helper does not skip its body.

Twelve new actual TLC cases cover helper/closure/method cleanup, nested scopes, LIFO,
blocking, synchronization errors, argument identity snapshots, conditional registration
and multiple returns. Graph/slice corruption and unproved/recursive/dynamic bodies
remain rejected. Totals: 131 semantic TLC and 18 TLC CLI cases, plus the separate
full-CLI and re-entry refusals. The strict local gate and full race suite pass with
zero skips/failures; original snapshots are unchanged.

This removes the old early rejection at `cmd/gotla/main.go`'s `defer dir.Close()`.
The real self-input probe now reaches deeper `check.go`/artifact cleanup paths and
exposes more unsupported operations. Counts are not a coverage metric: the observed
probe reports 240 initializer, 124 unknown-effect, 268 identity, 250 receive-loop,
228 sync-method, 83 loop, 46 exception, 42 unsafe-pointer, 9 topology, 9 defer and
1 channel-field-alias diagnostics. Repeated inline sites contribute duplicates.
It still returns unsupported / 5 and emits only diagnostic `model.json` plus the
result envelope. No TLC self-proof exists yet.

Evidence: `$HOME/tmp/pi/gotla-self-bootstrap-defer/` contains `focused.log`, `gate.log`,
`gate-attempt1.log`, `race.log`, `self-check.log` and `cli/` artifacts. The initial gate
caught a stale rejection expectation for an empty literal defer; that case is now a
positive TLC test and unresolved dynamic defers remain refusal tests. These results
are local-only; no hosted pass is claimed.

## Length-bounded data computation

An independent typed-SSA proof now summarizes externally read-only helpers whose
cycles advance an ordinary int index by one against a stable slice/string length.
Each cycle must cross the dominating comparison header; changed bounds/counters,
wraparound-prone forms and bypass cycles fail. All transitive call bodies and data
types are checked without trusting external functions. Only proved private data
writes are admitted. Shared writes, I/O, synchronization, unsafe code, unknown calls
and recursion are not summarized. No iteration is unrolled or silently truncated.

Lowering rechecks the body/call graph and consumes retained caller operands; argument
evaluation still occurs. Scalar results remain abstract, including branch guards.
Initializer helpers use the same proof. Search/type budgets fail closed. The original
concurrency-loop and resource-identity restrictions remain in force.

This proof is consumed on the **actual own-source** `toolinfo.fromBuildInfo` and
`(*behavior.Model).HasErrors` methods in the complete CLI probe. There are two
source-attributed `finite-data-loop` diagnostics; unsupported-loop diagnostics drop
from 83 to 73, with other categories unchanged. This is a concrete proof result for
these helper computations, not a whole-program coverage percentage. Full self-input
still returns unsupported / 5 and emits no executable TLA+.

Eight new TLC cases cover computation in critical sections, abstract predicate
counterexamples, initialization, blocked argument evaluation, deferred computations,
wrappers and value-struct copies. Tests also reject changing/narrow/overshooting
counters, cycles bypassing progress, shared writes, unknown callbacks and missing
cached graph/slice evidence. A unit test loads the real behavior package rather
than copying HasErrors. Full strict gate and race tests pass with zero skips/failures:
**139 semantic TLC and 18 TLC CLI cases**, plus the refusal regressions. Original
snapshots are unchanged. Evidence is in
`$HOME/tmp/pi/gotla-self-bootstrap-finite-data/{unit.log,focused.log,gate.log,race.log,self-check.log,cli/}`.
No hosted pass or full-self completion is claimed.

## Initializer proof consumption and attribution

Initializer calls now recheck cached callee summaries against the current call graph,
including acyclic pure constructors and calls nested within them. A stale outer or
transitive edge produces a contract error instead of admitting initialization.
Six proof-consumption cases cover intact/corrupted graphs for acyclic and finite
helpers. Four integration cases preserve refusal and call-site attribution for
source, unavailable, dynamic and interface calls. Diagnostic target names are not
proofs: an unresolved static candidate is explicitly marked unproved.

The complete self-input still reports 240 initializer refusals. The enriched messages
identify 104 calls to instantiated `golang.org/x/tools/go/ast/edge.info` and 11 to
`go/types.asGoVersion`; other targets include `os.NewFile`, `reflect.rtypeOf` and
`encoding/base64.NewEncoding`. Source inspection shows the edge metadata factory
uses TypeFor/Elem/FieldByName plus an explicit failure panic, while Go-version
normalization reaches strings.Cut and version parsing. These require actual proofs
or justified semantics, not blanket reflection/string allowlists or skipped init.

The strict gate and full race suite pass; counts remain 139 semantic and 18 CLI/TLC
cases, zero skipped required checks and unchanged original snapshots. Full self-input
remains unsupported / 5 with diagnostic-only artifacts. Local evidence:
`$HOME/tmp/pi/gotla-self-bootstrap-init-targets/{focused.log,gate.log,race.log,self-check.log,initializer-inventory.json,cli/}`.
No whole-self or hosted pass is claimed.

## Proved pre-closed globals

The actual `context.closedchan` creation and initialization close are now represented
as initial channel state, not skipped under a package exemption. The proof requires
a unique direct make/store in the compiler-generated package initializer, a subsequent
unconditional source init body that only closes that global, and a current unique
call-graph edge. All normal paths from the store reach the close call. Every SSA
function is checked for address escape/rebinding, including functions absent from
or unreachable in the call graph. Proof search is bounded and fails closed. Creation,
close consumption and later global identity resolution recheck these facts.

IR channels have an optional `initiallyClosed` boolean (default false); the saved IR
alone supplies the backend's empty/closed initial state. Existing receive status,
select/default, send and close semantics apply. Open globals, initialization sends,
conditional/repeated closure, factories and unknown effects remain unsupported.
Runtime sends/closes on a pre-closed channel produce the normal synchronization error.

Eight new actual TLC cases cover repeated and concurrent receives, buffered empty
status, range completion, select/default, send/close errors, JSON round trips and an
IR initial-state mutation that restores the expected deadlock. Refusal and corruption
tests cover order, current graph/body, capacity, escaped/rebound globals, removed graph
nodes and control paths bypassing closure. Strict gate and full race suite pass with
zero skips/failures and unchanged original snapshots: **147 semantic TLC and 18 CLI/TLC
cases**. The actual self-probe has creation/close evidence at context/context.go:423/426
and 238 initializer refusals instead of 240; other refusal categories are unchanged.
It still returns unsupported / 5 with no executable full self-model.

Local evidence:
`$HOME/tmp/pi/gotla-self-bootstrap-closed-globals/{focused.log,gate.log,race.log,self-check.log,cli/}`.
This is one real dependency effect modeled, not full bootstrap or a hosted CI claim.

## Private fixed-array computation

Private-data proofs now follow nested field/array-index addresses to a fresh allocation
and inspect every address use. Fixed-array elements must themselves be plain data;
reference/synchronization elements, slicing aliases, publication and caller/global
writes remain refused. This is a prerequisite for data-table constructors, not an
exemption for array-producing library functions.

Finite data-loop proofs also accept ordinary int constant bounds in 0..2147483647,
which cannot exceed either Go int width. Counter progress, exit and hidden-cycle
checks are unchanged. This permits private fixed-array fill loops without unrolling
or applying the concurrency expansion budget to pure computation. Larger constants,
narrow/changed/overshooting counters and synchronization loops remain refused.
Results still stay abstract and argument effects still execute.

Seven new real TLC cases cover array literals/nesting, 128-element fills using array
range or classic counters, abstract-value errors, blocked arguments and an empty
classic data loop. Type/address/constant-bound refusal tests cover the new boundaries.
The first strict run exposed a now-stale empty-classic-loop refusal; its synchronization
variant remains a refusal and the empty case is now checked with TLC. A second run
caught a test-source separator error. Both attempts are retained; the corrected full
gate and race suite pass with zero skips/failures and unchanged original snapshots.
Totals are **154 semantic TLC and 18 CLI/TLC cases**.

Full self-input remains unsupported / 5 with 238 initializer refusals and unchanged
other refusal counts. In particular, encoding/base64.NewEncoding also uses copy and
explicit validation panics: private-array support alone does not authorize that call.
No additional production initializer is claimed accepted by this increment.
Evidence: `$HOME/tmp/pi/gotla-self-bootstrap-array-data/` contains focused/gate/race
logs, both failed gate attempts, self-check output and diagnostic CLI artifacts.

## Literal-input SSA evaluation

A bounded evaluator now proves individual data calls with SSA-constant arguments.
It executes the actual selected SSA path, including transitive source calls, private
array/struct storage, owned slice views, overlapping copies and validation branches.
It never executes application/native callees on the host. Unknown/external memory,
unsafe operations, executed synchronization/I/O/panic, recursion and exhausted limits
fail closed. Arithmetic wrap and unsupported operations are refused rather than
approximated. Aggregate assignments retain existing field/element aliases; value
copies remain independent and phi updates are simultaneous.

This is a call-site proof, not a library allowlist or whole-function purity claim.
Fresh consumption checks arguments, current graph/body and retained caller operands;
returned values still stay abstract in the behavioral model. Limits and the supported
operation set are documented in SUPPORTED_GO.md. Other checked effect rules remain
independent alternatives; failed evaluation never produces a truncated proof.

The actual two encoding/base64.NewEncoding initialization calls now have source-located
proof records. A unit loads the installed constructor, compares its evaluated encoding
and decoding arrays with native Go, and refuses malformed/duplicate/newline alphabet
mutations. The complete CLI regression requires these two proofs while still requiring
unsupported / 5 and no executable self-artifacts. Initializer refusals fall from 238
to **236**; other refusal counts are unchanged. WithPadding/global-value propagation,
reflection factories and other dependency effects remain unproved.

Nine actual TLC cases cover validated initialization, owned copies/views/helpers,
normal calls, abstract result errors and provably unchosen/zero-iteration effects.
Additional tests invalidate arguments, graph, body and retained slices and exercise
alias-preserving resets, phi swaps, bounds/overflow, unknown/shared operations and
budget refusal. Earlier general-proof refusal tests now use genuinely external or
nonempty/unknown inputs where the new per-invocation rule cannot discharge them;
the newly proved cases have positive tests. Initial focused/gate failures are retained.
Corrected full gate and race tests pass: **163 semantic TLC and 18 CLI/TLC cases**,
zero skips/failures and unchanged original snapshots.

Evidence: `$HOME/tmp/pi/gotla-self-bootstrap-constant-data/` contains unit/focused/gate/
race logs, initial failed attempts, self-check output and diagnostic CLI artifacts.
Full bootstrap remains unproved; no hosted pass is claimed.

## Private zero-reference data storage

Literal-input evaluation now allows zero pointer/slice/map fields after checking the
entire data type graph. Synchronization, callbacks, interfaces and unsafe pointers
remain excluded even behind nil references. Nil fields allocate no backing storage;
all subsequent writes still require owned cells. Private cyclic references are
permitted without recursively cloning pointees. Aggregate replacement preserves
field addresses and ordinary pointer aliasing; external/global references and nil
dereferences still refuse. Map construction/mutation and floating-point arithmetic
are not added.

The actual go/constant.newFloat initializer and its fresh big.Float.SetPrec call are
now proved, with native precision comparison. The real CLI regression requires this
source-located proof in addition to the base64 proofs. Initializer refusals fall from
236 to **235**, with other refusal counts unchanged. Full self-input remains
unsupported / 5 and produces no executable model.

Three new TLC cases cover nil fields, owned pointer fields and reference arrays;
unit cases cover cyclic ownership, alias-preserving replacement, nil dereference,
external references and forbidden nested callback/interface/channel types. The old
reference-array refusal now uses an external pointer, while its formerly refused
owned-pointer variant is a positive TLC case. Initial focused failure is retained.
Strict gate and race tests pass with **166 semantic TLC and 18 CLI/TLC cases**, zero
skips/failures and unchanged snapshots. Evidence is under
`$HOME/tmp/pi/gotla-self-bootstrap-references/`. Results remain local, not hosted.

## Fixed-capacity global I/O semaphores

A shared creation/identity proof now supports initially open global channels made
with a constant capacity and a unique direct store. Complete SSA address inventory
still rejects rebinding and address escape, including functions omitted from the
call graph. The pre-closed pattern retains its additional unconditional-close proof.
Both patterns now reject stale capacity changes even when the new capacity remains
within 0..1024; this closes a proof-consumption consistency gap.

The actual x/tools packages.ioLimit, buildutil.ioLimit and loader.ioLimit creations
are proved and represented as empty open channels of capacities 20, 20 and 10. The
real CLI regression requires all three source records and resource states. Initializer
refusals fall from 235 to **232**, with other refusal counts unchanged. Dynamic CPU
semaphore capacities remain unproved, as do unknown initialization operations and
pre-main sends/receives. Full self-input remains unsupported / 5 with no executable
TLA+ or TLC run.

Seven new TLC cases cover rendezvous, buffering, runtime close, empty/full deadlocks,
and a semaphore with/without release. Corruption/refusal tests cover valid/invalid
capacity changes, graph-omitted rebinding, address escape and unsupported initializer
effects. Old bare-global refusal cases are replaced by positive TLC coverage; an
initializer-send refusal remains. Initial focused/gate failures are retained.
Strict gate and full race pass: **173 semantic TLC and 18 CLI/TLC cases**, zero
skips/failures and unchanged original snapshots. Evidence is under
`$HOME/tmp/pi/gotla-self-bootstrap-open-globals/`; it remains local-only.

## Read-only reference-bearing value copies

The finite-data proof now permits private value copies containing ordinary data
references. The destination must still trace to this invocation's allocation with
complete address-use proof. Tracing stops at a load: ownership of a copied slice
header or pointer never grants ownership of its backing array or pointee. Resetting
local headers is allowed; caller-header writes, publication and writes through
borrowed references remain rejected. Full type, operation, transitive body and loop
proofs remain mandatory; the general constructor rule is not widened.

The actual Model.Statistics method is now proved, alongside HasErrors. The full CLI
regression requires its source-located proof. Unsupported-loop diagnostics fall from
73 to **70** (repeated call sites), while initializer refusals remain **232** and
other counts are unchanged. The CLI still returns unsupported / 5, emitting no
executable self-model and running no TLC self-check.

Five new TLC cases cover slice/pointer value copies, private-header reset, abstract
result synchronization errors and blocking argument evaluation. Unit/refusal tests
cover shared backing arrays/pointees/headers, transitive writes, publication and
forbidden type graphs. A cached-proof mutation redirects a private copy store into
a global and must fail fresh consumption. Native Statistics tests remain unchanged.
Strict gate and full race pass with **178 semantic TLC and 18 CLI/TLC cases**, zero
skips/failures and unchanged snapshots. Evidence is local under
`$HOME/tmp/pi/gotla-self-bootstrap-value-copies/`.

## Scalar-array range normalization (latest increment)

The source normalizer now expands small by-value scalar-array ranges with declared
index/value bindings. It requires effective Go 1.22+ semantics, snapshots/evaluates
the array once, preserves per-iteration bindings, and retains function-level return
and defer behavior. Zero-length two-value ranges still evaluate their expression.
Name collisions, source locations and re-type-checking are covered. Key-only/pointer
arrays, assignment bindings, resource/reference elements, oversized arrays and
unsupported nested control are refused; existing expansion budgets still apply.

The actual artifact managedFiles table changed from an unexported, never-mutated
slice to a fixed five-element array. Begin and Write now have consumed five-iteration
proofs. No file operations, errors, dependencies or initializers are skipped. Native
artifact tests remain in force. The real CLI regression requires both source-located
proofs while retaining unsupported / 5 and diagnostic-only artifacts.

Loop refusals fall from 70 to **67**, but expansion reaches more previously hidden
operations: initializer 232, exception 47, unknown-effect 150, identity 272, unsafe 70,
receive-loop 356, sync-method 230, topology 9, defer 9 and channel-field-alias 1.
These counts are diagnostics, not coverage/completion metrics. Full bootstrap remains
unproved. Seven new TLC cases cover static topology/workers, zero count, saturation,
return, abstract index guards and deferred scope. Six native original-versus-expanded
execution cases cover snapshot/capture/evaluation behavior; old-language and source
position tests protect the transformation boundary.

The first focused run incorrectly expected precise evaluation of an index guard;
its log is retained. The corrected matrix separately tests unconditional return and
the conservative abstract-index counterexample. Strict gate and full race pass:
**185 semantic TLC and 18 CLI/TLC cases**, zero skips/failures, unchanged snapshots.
Evidence is local under `$HOME/tmp/pi/gotla-self-bootstrap-array-ranges/`.

Remaining work includes other dependency initialization/effects, other non-receive control loops,
returned/dynamic callback and object identities, runtime synchronization primitives,
I/O/context/process lifecycle semantics and an enforceable finite input/environment
profile. These are implementation tasks, not permission to weaken the acceptance
contract or treat additional component fixtures as completion.

## Immediate program exit prerequisite

The actual `cmd/gotla/main.go:112` call now consumes the explicit `os.Exit(int)`
operation and emits Exit instructions in the diagnostic IR. It is not classified
as pure and cannot be overridden by trusted-call options. Current signature,
call-graph target and argument-slice facts are rechecked. Ordinary process exit
terminates all modeled processes and skips outstanding cleanup; status values
remain abstract, argument effects remain scheduled, and Go test's panic-on-exit
interception is outside this declared domain.

The IR requires standalone Exit effects targeting the issuing process's terminal.
The backend stops all processes and pending offers without clearing synchronization
faults or releasing resources. Eight independent IR/TLC cases cover main/worker
exit, skipped/deferred cleanup, prior/competing faults, blocking before exit, and
removing the worker's exit (deadlock). Separate actual-source lowering tests cover
helper/worker control, argument effects, user-name collisions, trusted-call overrides,
and stale graph/signature/slice facts. They explicitly retain the whole-program
refusal caused by `os` initialization: isolated function tests are not full-program
TLC evidence. The real CLI regression requires its own source-located Exit effect.

Strict gate and full race pass with unchanged snapshots: **193 semantic TLC and
18 CLI/TLC cases**, zero skips/failures. Local evidence is under
`$HOME/tmp/pi/gotla-self-bootstrap-exit/` (`gate.log`, `race.log`, focused tests,
and `cli/{result,model}.json`). A diagnostic inspection initially tried iterating a
JSON null effects list; that inspection was corrected, not an analysis/checker failure.

The actual self-check is still **unsupported / 5**, listing only `model.json` and
not invoking TLC. Remaining diagnostics include 232 initializer, 67 loop, 46
exception-control, 148 unknown-effect, 272 identity, 70 unsafe, 356 receive-loop,
230 sync-method, 9 topology, 9 defer and 1 channel-field-alias errors. The reduction
of one exception and two unknown-effect diagnostics is not completion evidence.
All full-goal acceptance boxes remain unproven; finite environment enforcement,
dependency effects and the executable whole-self model still require implementation.

## Nil-interface constant storage prerequisite

The concrete literal-input evaluator now admits nil empty-interface fields,
parameters and returns, with a nil-only invariant checked on copying. Boxing,
including typed nil pointers, nonempty interfaces, interface conversion/assertion,
dynamic dispatch, external memory and nonliteral inputs remain refused. General
finite-data proofs still reject interfaces. SSA zero aggregate constants materialize
real zero field/element values; resetting an aggregate preserves existing subobject
addresses and restores scalar fields to zero.

This proves the real `go/ast.NewIdent("_")` call in `go/doc/exports.go:27`, without
an API allowlist or skipped initialization. Native comparison checks Name, NamePos
and nil Obj. The real CLI regression requires this source proof. A consumed-proof
mutation redirects a private nil-interface store to a global and must refuse.
Five actual TLC cases cover communication after construction, nil inputs, aggregate
reset aliases, package initialization and conservative abstract-result failure.

The first focused run exposed missing materialization of SSA zero aggregates and
fixtures that already used the ordinary pure-constructor path. The implementation
now handles zero aggregates; those fixtures explicitly store nil so they exercise
the intended concrete-proof boundary. The failed log remains retained.
Strict gate and full race pass: **198 semantic TLC and 18 CLI/TLC cases**, no skips
or failures, unchanged snapshots. Evidence is local under
`$HOME/tmp/pi/gotla-self-bootstrap-nil-interfaces/`.

Actual full self-analysis remains **unsupported / 5**, only `model.json`, no TLC.
Initializer diagnostics fall from 232 to **231**; other error counts remain those
of the preceding exit prerequisite. No complete-bootstrap requirement is declared
done by this incremental constructor proof.

## Explicit byte-search operation and version initialization

The constant evaluator now distinguishes source execution from explicit modeled
operations. The native, body-less standard-toolchain declaration of
`internal/bytealg.IndexByteString(string, byte) int` has exact first-byte-index/-1
semantics, evaluated without invoking loaded code and with each examined byte
charged against the shared step budget. Signature, declaration ownership, absent
Go body, source mapping, current call graph and source location under the running
GOROOT are checked. Available Go byte-search bodies are interpreted normally; arbitrary
unavailable functions and user-name collisions are not exempted.

**The standard assembly implementation's correctness is assumed, not proved.**
The assumption is returned with the concrete proof, consumed freshly, recorded in
model assumptions and exposed by `modeled-data-operation` diagnostics. This is not
a trusted pure-call skip: actual result values drive subsequent source branches,
and exhausted budgets or executed panics refuse the invocation. Bounded string
concatenation completes the selected version-parsing paths.

Native comparisons cover 20 byte-search cases, six version cases, and the actual
ten literal `go/types.asGoVersion` initializers. Graph, declaration, source and
signature corruption and budget exhaustion refuse. Consumed cached evidence is
rechecked. The nonliteral current-version initializer remains unsupported; standard
library initialization is never omitted to make the new operation usable.

Three additional real TLC cases test pure-Go concatenation, package initialization
and an abstract-result counterexample; they are not whole-program TLC checks of
strings/version imports. Strict gate and full race pass, no skips/failures and no
snapshot changes: **201 semantic TLC and 18 CLI/TLC cases**. Evidence is local under
`$HOME/tmp/pi/gotla-self-bootstrap-byte-search/`.

The real CLI regression requires ten source-located version proofs and corresponding
operation-model records plus one deduplicated model assumption. Actual self-analysis
remains **unsupported / 5**, with only `model.json` and no TLC invocation. Initializer
errors decrease from 231 to **221**; other error counts are unchanged. The full
bootstrap acceptance contract, including an enforced finite environment and an
executable meaningful self-model, remains unfulfilled.

## Immutable reflection metadata prerequisite

The concrete evaluator now admits immutable reflect.Type tokens from TypeFor[T].
Its current standard-source declaration, concrete type argument, retained call
edges and the exact SSA/dataflow chain through abi.TypeFor, abi.TypeOf, abi.NoEscape
and reflect.toRType are checked. Added instructions, altered ABI field selection,
casts, pointer arithmetic and removed graph edges refuse. This structurally binds
an explicit operation model: **runtime ABI/type metadata correctness is assumed,
not a proof of unsafe representation or compiler correctness**.

Known type tokens alone admit explicit standard Elem and direct FieldByName
queries. Current method objects/signatures and source ownership are checked;
conflicting target facts refuse. These models read Go type descriptions and target
sizes, never application memory. Direct StructField results include name, package,
type, tag, offset, index and anonymity; missing fields return zero/false only when
no promoted search is needed. Query standard-library correctness is a separate
recorded assumption, not an execution proof of reflection's implementation.
Nil/invalid receivers, promoted fields, other methods and reflect.Value remain
unsupported. A described channel or function is not an actual resource or callback.
Results stay abstract outside the per-invocation proof.

Native comparison covers **all 104 real x/tools AST edge initializers**, plus direct
field layouts/tags/presence and six type categories. Cached consumption rechecks
ABI bodies, methods and literal field-name arguments. Other dependency initializers
are still refused; these reflection-import programs are not advertised as whole
source TLC proofs. Existing **201 semantic TLC and 18 CLI/TLC** cases still pass,
with strict gate/full race, no skips/failures and unchanged snapshots. Evidence:
`$HOME/tmp/pi/gotla-self-bootstrap-reflection/`.

Actual self-analysis records all 104 AST edge proofs and five direct TypeFor proofs,
with exactly two deduplicated reflection assumptions. Initializer errors decrease
from 221 to **112**; other error counts are unchanged. It still returns
**unsupported / 5**, emits only diagnostic model.json, and does not invoke TLC.
The full finite-environment/executable-self-model acceptance contract is not complete.

## Production lexical scanning and rotated length ranges

The actual TLA validator and TLC protocol parser no longer initialize three
regular expressions. Shared ASCII identifier predicates and explicit header/PC
scanning preserve the old grammar, including ASCII-only whitespace, unanchored
PC matches, last duplicate entry, numeric conversion and malformed-frame handling.
Message classification and the evidence required for success are unchanged. The
old regexes remain test oracles, not production initializers; this is a lexical
implementation change, not a substitute coordinator or a dependency exemption.

Differential fixtures, 20,000 generated protocol inputs, byte/identifier tests and
15-second fuzz runs pass (92,633 protocol and 2,525,798 identifier executions in
this local run). Large malformed inputs are included. These tests do not constitute
a general payload/compiler correctness proof.

Go 1.26 emits rotated SSA for range-over-len: entry tests 0 < bound, and the bottom
increments/tests before returning to the header phi. A fresh structural proof now
requires every entry/back edge to have that form, the same stable len bound, exact
+1 progress, header dominance and no cycle avoiding the header. The measure cannot
wrap because only indices below len take back edges. This rule deliberately does
not relax constant-range normalization/expansion limits. Real Identifier and
checker decimalDigits bodies pass the finite-data proof; results remain abstract.
Entry/step/comparison/bound mutations, hidden effects/cycles and stale consumed
proofs refuse.

The first focused run exposed the rotated shape; the first full gate exposed an
unintended overlap with constant-range expansion refusals. Restricting this rule to
len bounds restored all original refusal tests without weakening them. Both failed
logs remain under `$HOME/tmp/pi/gotla-self-bootstrap-lexical/` alongside final gate,
race, fuzz and actual CLI evidence. Five new pinned TLC cases cover normal locking,
empty input, initialization, abstract-result deadlock and blocking argument effects:
**206 semantic TLC + 18 CLI/TLC**, no skips/failures, unchanged snapshots.

Self-analysis remains **unsupported / 5**. Project lexical initializer errors are
gone; total initializer refusals fall from 112 to **109**. Only diagnostic model.json
is emitted; no executable self TLA+ or self TLC proof exists. The full goal and
finite-environment requirements remain active and incomplete.

## Conditional startup processor-setting contract

`-runtime-procs N` now provides a small, explicit environment boundary for an ordinary
linux/amd64 Go executable started with `GOMAXPROCS=N` (1..1024). It is conditional:
the flag neither sets nor infers the analyzer host environment. Standard runtime
semantics, including startup disabling of automatic updates and read-only zero
queries, are assumed rather than source-proved. The value is recorded in hashed
model metadata, result provenance and a clearly conditional assumption.

The analyzer checks current standard-source/signature/target identity and inventories
all SSA references, not just graph-reachable functions. Only direct zero queries
are admitted outside the standard-runtime boundary. Setters, resets, escaped API
values and trusted-call combinations refuse, including unused/graph-omitted setters.
Budget exhaustion refuses. Consumed queries freshly check current and cached graph
facts and all retained operands; global capacity/identity proofs also refresh.
Unknown effects and all other initialization remain independently checked. Arithmetic
or wrapped capacities are not guessed. Other query results remain abstract.

Native executable comparisons confirm channel capacities at startup values 1, 2
and 3. Six new semantic TLC cases and two actual CLI/TLC cases show capacity-sensitive
pass/deadlock outcomes, local/global/closed channels and preserved blocking/errors.
Mutation tests cover arguments, graph, slices, profile metadata, runtime declaration,
source ownership, hidden setters and cached global capacities. Strict gate/full race
pass with no skips/failures and unchanged snapshots: **212 semantic + 20 CLI/TLC**.

The actual CLI at N=2 now consumes both x/tools CPU semaphores at capacity two, with
two source-attributed runtime query records. Initializer refusals are **105** under
this profile, versus **109** without it. The new production option/contract paths
also expose more other unsupported behavior; raw diagnostic counts are not coverage.
Both self-inputs remain **unsupported / 5**, diagnostic-only, with no checker command
or executable self TLA+. The full finite input/I/O/lifecycle environment and bootstrap
acceptance checklist remain incomplete.

Evidence is local under `$HOME/tmp/pi/gotla-self-bootstrap-runtime-procs/`. The first
probe exposed a nil-frame argument-slice check during initialization; it was fixed
and the original panic log retained. Subsequent complete CLI probes terminate with
the intended unsupported result. This prerequisite does not authorize completion.

## Boxed-literal runtime type metadata

The three actual `reflect.rtypeOf` initializers (`string`, `[]byte`, `uint8`) now
have a narrowly guarded immutable-metadata operation model. Admission checks the
standard declaration, source ownership, exact wrapper, current ABI TypeOf/NoEscape
chain, caller graph, dominating literal box and existing literal/type budgets.
The unsafe runtime representation remains an explicit correctness assumption,
not an implementation proof. No interpreter-level application interface or runtime
pointer is created, and the returned metadata remains abstract.

Native public type metadata agrees with the three source argument types/layouts.
Mutation tests reject changed arguments (including nested interface boxes), graph,
wrapper, ABI, source, ordering and oversized literals. Consumption rechecks both
the retained box and its literal operand. General boxing and other reflection
initialization remain refused; the real CLI boundary regression requires exactly
three source-attributed proofs and the additional deduplicated assumption.

Strict gate/full race pass, without skips/failures or snapshot changes. Existing
TLC totals stay **212 semantic + 20 CLI**; this change adds no independently executable
reflection-import model. Actual self-analysis remains **unsupported / 5**, with
**102** initializer refusals under startup GOMAXPROCS=2, or **106** without the profile.
Only a diagnostic model is emitted; there is still no full self TLA+ or self TLC run.
Evidence and initial failed attempts are retained under
`$HOME/tmp/pi/gotla-self-bootstrap-reflection-literals/`.

## Source-bound basic-scalar formatting

`fmt.Sprintf` now has an explicit operation model restricted to unnamed basic
scalar/nil arguments, with no application callbacks, I/O or application synchronization.
The current standard declaration/signature, exact wrapper and internal call edges
are checked; standard formatting/private-pool/runtime correctness and normal resource
availability remain assumptions. This is not a proof of fmt internals or a finite
payload profile. Strings/results are abstract and no host formatting is evaluated.

Admission reconstructs current SSA uses for a nonescaping, single-consumer local
variadic array (at most 64 slots, 4096 instruction/operand inspection budget).
One dominating store per populated slot and literal nil or unnamed-basic boxes are
required. Named/custom-method values, shared/escaped/reused slices, alternate stores,
partial slices, missing graph/source facts and budget exhaustion refuse. Store roots
and scalar operands are retained and freshly consumed; blocking argument evaluation
and critical-section effects survive. Other initialization remains independent.

Native execution confirms a scalar call inside a locked region and a named-value
String callback that the proof correctly refuses. Mutation tests exercise current
wrapper/graph/source, store ordering/aliasing, index/value/slice changes, budget,
retained operands and store roots. Actual self-input consumes 25 scalar-format sites,
including production lowering diagnostics and go/types/version.go. Startup-N=2
initializer refusals drop from 102 to **101**; identity refusals drop 340 to 292,
receive-loop 420 to 396 and unsupported sync-method 282 to 234. These duplicated
site counts are not a coverage measure.

The strict gate/full race pass without skips/failures and with unchanged snapshots.
TLC totals remain **212 semantic + 20 CLI**: there is no new positive full-fmt-import
or full-self TLC result. Self-analysis still returns **unsupported / 5**, with only
a diagnostic model. Evidence: `$HOME/tmp/pi/gotla-self-bootstrap-format/`.

## Fail-closed builtin output boundary

Audit found that ordinary Go `print`/`println` calls were classified as sequential
builtins and dropped even though they perform output. That exception is removed:
executed builtin output now refuses emission pending an explicit I/O model. Current
call/initializer checks also reject output behind stale cached builtin summaries.
Named source functions are distinguished from actual SSA builtins; trust-call names
cannot override the language operation. Blocking argument evaluation is retained.

Native execution demonstrates observable output. CLI tests verify unsupported / 5,
no TLC invocation, and stale executable-artifact removal with and without attempted
builtin trust. Direct/helper/initializer/constructor/go/defer cases refuse. Two TLC
cases preserve legitimate source-name shadowing and literal-proved zero executions;
executed output is not treated as a no-op. Totals: **214 semantic + 20 CLI/TLC**.
Strict gate/full race pass without skips/failures or snapshot changes.

This closes a soundness gap; it does not provide the missing finite I/O environment
or a self-bootstrap proof. Evidence: `$HOME/tmp/pi/gotla-self-bootstrap-builtin-output/`.

## Scalar Sprint/Sprintln prerequisite

The source-bound formatting model now checks all three string-returning variants:
Sprintf/doPrintf, Sprint/doPrint and Sprintln/doPrintln. Exact standard wrapper,
method identity, graph, signature/arity, private-array and fresh slice/store proofs
are still required. No output-I/O function, named-value callback or shared/escaping
argument slice is admitted. The explicit standard-formatting/runtime assumption
covers these variants; returned strings remain abstract.

Native tests check spacing/newlines and demonstrate the rejected String callbacks.
Changed wrapper methods, graph, arity, escapes and missing retained store roots
refuse; receive arguments and independent fmt initialization errors remain present.
An actual-source unit test proves checker.Run's fmt.Sprint(cfg.Workers) call, but the
full-self probe does **not** yet consume that site. It still reports 25 existing scalar
formatting sites, 101 initializer refusals under N=2, and unsupported / 5 with no
executable self TLA+ or checker invocation. This distinction is an acceptance boundary,
not a reason to weaken the full objective.

Strict gate/full race pass without skips/failures or snapshot changes. TLC totals
remain 214 semantic + 20 CLI. Evidence: `$HOME/tmp/pi/gotla-self-bootstrap-sprint/`.

## Fixed checker input-file enumeration

The actual checker.Run no longer uses a map to enumerate its two private input
files. A two-element string array fixes the order to model.tla then model.cfg;
contents, 0600 writes, immediate error returns and deferred private-workspace cleanup
are retained. This is a production file-enumeration change, not a replacement CLI
or an exemption for external I/O. The existing source-array normalization proves
exactly two iterations without truncation.

A native subprocess oracle verifies both exact file contents (including Unicode)
and permissions in the private working directory. Swapped contents deliberately
fail the oracle, and both success/failure workspaces are cleaned. Existing actual
CLI/TLC cases, strict gate and full race pass without skips or snapshot changes;
TLC totals remain 214 + 20. The real self-input regression requires the source-located
two-iteration proof and refuses any return to the former checker range rejection.

Whole-self analysis advances to the next real blocker: the returned context timeout
cancellation function used by defer stop(). Unsupported loops drop 69 to 68 while
unsupported defers increase 13 to 14; this is a frontier change, not whole-program
coverage. Startup-N=2 still has 101 initializer refusals and returns unsupported / 5,
with only diagnostic model.json and no checker command. The checker worker-formatting
site is still not a consumed full-self proof. Evidence:
`$HOME/tmp/pi/gotla-self-bootstrap-checker-inputs/`.

## Exactly bound deferred dispatch

Source-defined helpers invoked through the existing local-boxed-interface and
immutable-callable-field proofs now use the ordinary fresh callee binder during
defer registration. Resource identities are captured then; cleanup executes the
actual source body in LIFO order, including blocking and synchronization failures.
Graph, retained operands/receiver and current field initializer are not bypassed.
No new general object identity, returned-closure, primitive-interface, wrapper,
I/O or exception model is introduced.

Ten cases run both natively and under pinned TLC, covering empty and effectful
methods, receiver snapshots, instance-specific field captures, blocking arguments/
cleanup, and LIFO reversal/double-close failures. Mutation tests reject missing graph/
slice/origin proofs, synthetic wrappers and foreign defer stacks. Mutable/ambiguous/
nil dispatch, returned closures, output, panic and existing object-identity refusals
remain fail-closed. Initial attempts exposed unchanged channel-boxing/callable-only
object restrictions; fixtures were scoped to already supported identities rather
than weakening those rules. An obsolete empty-interface defer refusal moved to a
positive native/TLC check; the failed gate is preserved.

Strict gate and full race pass without skips or snapshot regeneration. Totals are
224 semantic + 20 CLI TLC cases. The actual N=2 self-check still has 101 initializer,
68 loop and 14 defer refusals, including checker.Run's returned timeout cancellation.
It remains unsupported / 5 with no executable self-model or TLC invocation. This
increment supplies a reusable cleanup proof, not full bootstrap completion.
Evidence: `$HOME/tmp/pi/gotla-self-bootstrap-deferred-dispatch/`.

## Channel-bearing receiver boxes

Channel-bearing allocated objects may now be locally boxed for exact receiver calls,
including spawned and supported deferred source methods. Every current box use must
have the matching receiver and call-graph proof; ordinary argument/storage/return
escapes remain rejected. Initialization must dominate boxing, as well as other object
exposures. Channel-field/object use inventories are rebuilt from current operands with
a 4096-step instruction/operand budget per scan; missing definitions and exhaustion
refuse rather than trusting stale Referrers or truncating uses. Direct source-method
defers also participate in initialization dominance checks.

Eight native/TLC comparisons cover successful communication, distinct objects,
blocking, closed sends and cleanup. Tests mutate graphs, operands, definitions and
inventory size while deliberately invalidating Referrers caches. Late/repeated field
initialization, receiver mutation and unproved box escapes refuse. Two former blanket
boxing refusals moved to positive tests; no general points-to or returned-object model
was introduced. Strict gate/full race pass, snapshots unchanged, 232 + 20 TLC totals.

The real N=2 self-check remains unsupported / 5: 101 initializer, 68 loop and 14 defer
refusals remain, as does the channel-object return in os/signal/signal.go:283.
No executable self-model or TLC invocation is emitted. Context creation/cancellation
and the other full acceptance requirements remain unproved.
Evidence: `$HOME/tmp/pi/gotla-self-bootstrap-channel-boxes/`.

## Source-bound Once.Do and finite shared control

Direct Once.Do now has a standard-source/signature/graph/control-shape contract.
A zero Once is represented with a generic mutex and shared finite completion flag.
The initial atomic flag read commits before lock acquisition; slow callers recheck
under the lock. The actual callback body and normal defers run before completion is
stored, and unlock remains a separate scheduling step. Standard atomic/mutex/runtime
correctness is explicitly assumed; callback exceptions and arbitrary callbacks are
not erased. No Go-specific backend effect or unbounded counter was introduced.

TLA now handles the generic IR's already-declared finite shared variables with explicit
initial values, guards and assignments. Callback function values have their own source
body graph edges, without resolving dynamic parameter calls or claiming reachability.
Unknown/nil/synthetic callbacks, reset/copy, panic, output and initialization effects
remain refused, including attempted Once.Do trust overrides.

Eleven native/TLC comparisons and two independent shared-state IR/TLC cases cover
contention, waiting for callback completion, reentrancy, cleanup, a double-close mutant,
nonzero shared initialization and missing publication. Unit mutations cover source/
body/graph/flag-store/closure-order/retained-operand facts. The actual dependency
internal/godebug.Setting.Value has one eligible contract; IncNonDefault's bound wrapper
still refuses. Whole-self analysis does not yet consume a Once call, so this is an
operation-model prerequisite, not whole-source callback verification.

The strict gate passes with 245 semantic + 20 CLI TLC cases and unchanged snapshots.
The combined gate/race command exceeded its outer 600-second limit after the gate;
the independently rerun complete race suite passed. Initial compile/probe/backend/
callback-graph failures are preserved where logged, not presented as successful gates.
Actual self-check still returns unsupported / 5 (101 initializer, 68 loop, 14 defer
refusals), with no executable self TLA+ or self TLC invocation.
Evidence: `$HOME/tmp/pi/gotla-self-bootstrap-once/`.

## Bound-method callback and cleanup forwarding

Once callbacks and direct defers can now consume a narrow generated-method wrapper
proof: one typed receiver capture, a single graph-checked source method call forwarding
all arguments unchanged, and void return. Current method identity/set, declaration,
wrapper CFG, signatures, capture types/arity and creation-before-use are required.
Resource captures also require retained slice operands; a missing capture cannot be
recovered merely from an existing resource ID. The creation inventory is bounded to
4096 instructions. Actual wrapper/target bodies still execute; the proof supplies no
purity, new resource identity or returned-function exemption.

Nine paired native/TLC cases cover Once on bound channel/object/mutex methods,
blocking, invalid unlock, deferred arguments and receiver snapshots. Mutations and
refusals cover graph/receiver/argument/body/signature corruption, missing capture data,
late creation, budget exhaustion, interface/generic/result-bearing wrappers, copies
and output. The former bound-Unlock refusal became an analyzed synchronization-error
case; the initial stale-expectation gate failure is preserved. The actual godebug
Value and IncNonDefault sites now both have eligible operation contracts, not proofs
of their transitive effects or whole-self consumption.

Strict gate/full race pass with unchanged snapshots; totals are 254 semantic + 20
CLI TLC cases. Real N=2 self-analysis still consumes no Once call and remains
unsupported / 5 with 101 initializer, 68 loop and 14 defer refusals. There is still
no executable self-model, self TLC run, or complete finite environment/lifecycle proof.
Evidence: `$HOME/tmp/pi/gotla-self-bootstrap-bound-methods/`.

## Private reference-bearing constructors

The private allocation/store effect proof now permits opaque reference/header copies
(pointer, interface, slice, map, function and channel) in fields and array elements.
It neither follows stored references nor transfers ownership of backing storage,
permits unknown callbacks, or supplies a returned synchronization identity. Inline
synchronization value copies remain outside this proof. Current instruction/operand
inventories (4096 steps) replace cached SSA referrers for address/returned-box uses.
Missing definitions, escapes and exhaustion refuse. Existing stronger literal-input
proofs retain priority where applicable, including the consumed ast.NewIdent proof.

Six TLC cases cover constructor storage and formerly refused opaque non-nil boxes;
native Go checks alias preservation, callback non-execution and type metadata identity.
Mutations/refusals check publication, borrowed writes, callback execution, stale use
lists, exhausted inventories and unproved returned channel identity. Actual installed
go/types.NewPointer/NewTuple and x/tools SSA newVar/anonVar bodies are checked.
Initial failed tests/gate are retained: blanket reference refusals needed promotion,
while lost literal-proof diagnostics required fixing proof selection, not relaxing the
existing proof tests. Strict gate/full race pass with zero skips/failures and unchanged
snapshots; totals are 260 semantic + 20 CLI TLC cases.

Seven corresponding real initializer refusals disappear (101 to 94). Other frontier
counts remain unchanged (including 68 loop and 14 defer refusals). The real CLI still
returns unsupported / 5 without executable artifacts or a self TLC command. Finite
input/environment/lifecycle, returned context identities and full self-verification
remain unproved. Evidence: `$HOME/tmp/pi/gotla-self-bootstrap-reference-constructors/`.

## Revalidate initializer purity at consumption

A mutation regression demonstrated that a cached pure constructor summary could
survive changing its private field store to a global pointer store; both direct and
transitive cases previously produced no error. Initializer consumers now rebuild the
pure/private-store/body/transitive-effects proof using fresh summary caches, preserving
only explicitly supplied trusted contracts. Failure reports `pure-data-contract`.
The valid constructor still passes; both stale cases refuse executable generation.
Existing graph-corruption tests accept the more specific purity diagnostic.

The failing pre-fix regression is preserved. Strict gate and full race pass with no
skips/failures; totals remain 260 semantic + 20 CLI TLC cases and snapshots unchanged.
This closes a proof-freshness hole, not an additional self-program semantic frontier.
Actual self-check remains unsupported / 5, without executable artifacts or a self TLC
command. Evidence: `$HOME/tmp/pi/gotla-self-bootstrap-purity/`.

## One channel identity across normal returns

Source calls with one channel result can now transfer an invocation-specific identity
from the analyzed callee to the caller. All normal returns must agree; the body and
normal deferred cleanup execute before caller continuation. Return values and result
slot stores are retained slice roots/dependencies. A fresh 4096-step instruction/operand
inventory requires private, single-assignment result slots and dominating stores; it
does not rely on cached referrers. Only a checked predecessor-free recovery-only block
of loads and return is excluded from normal identity checks, consistently with existing
panic/recovery refusals and the declared implicit-panic boundary.

Nine native/TLC pairs cover factories, forwarding, separate invocations, nil channels,
directional views, common-return branches, goroutines, deferred double-close and
blocked cleanup. Mutations cover retained operands/roots/stores, store substitution,
duplicate stores, recovery effects and budget exhaustion. Different identities,
mutable captured result slots, tuple/object returns, recursion, output and explicit
recovery refuse. Actual context emptyCtx/withoutCancelCtx Done identities are checked
independently against native nil-channel results. They are not context lifecycle proofs.

Initial focused failures exposed the synthetic recovery result loads; evidence is
retained. Strict gate and full race pass with zero skips/failures; totals are 269 semantic
+ 20 CLI TLC cases and snapshots are unchanged. The real self attempt still consumes
no channel-return proof; frontier counts remain 94 initializer, 68 loop and 14 defer
refusals, among other unsupported effects/identities. It emits no executable self-model
or self TLC command. Evidence: `$HOME/tmp/pi/gotla-self-bootstrap-channel-returns/`.

## Ordinary fields beside untouched zero synchronization storage

Private constructor stores can now initialize ordinary fields alongside untouched
Mutex/WaitGroup/Once storage. The fresh owner remains private until return; typed
field/index paths, current bounded use inventories and a 256-node/depth-64 container
proof are required. Synchronization field addresses, value/owner copies, resets,
publication and operations do not qualify. This is a Go zero-allocation/store proof,
not a model that discards locking or a proof of returned synchronization identity.
Other synchronization/atomic types are not admitted by this rule.

Four TLC cases cover each primitive and an untouched mutex array. Native Go checks
independent owners/zero state, Once callback counts, wait-group operations and constant
string values/shared empty identity. Refusals/mutations cover copies, operations,
returned identities, typed-field corruption, publication, stale referrers, exhausted
use/type budgets and type depth. Initial focused failures are retained: old blanket
field refusals became positives, while a corrupted FieldAddr exposed the need to
recheck current field/index types, which was fixed rather than weakening that test.
Strict gate/full race pass without skips/failures; 273 semantic + 20 CLI TLC cases,
unchanged snapshots.

Actual internal/godebug.New and go/constant.MakeString bodies qualify. The latter may
return shared empty data on another path; the effect proof does not assert a fresh
result on every return. Nine godebug constructor refusals and one MakeString refusal
are removed from real dependency initialization (94 to 84). Other diagnostic counts
remain unchanged. Real self-check is still unsupported / 5 and has no executable
self-model or TLC command. Finite environment, context/process lifecycle and remaining
source effects/identities remain unproved. Evidence:
`$HOME/tmp/pi/gotla-self-bootstrap-dormant-sync/`.

## Finite startup data writes, not runtime purity

A distinct initializer consumer may now prove cyclic data initialization, including
ordinary shared data stores and map creation/update/lookup. Its fresh transitive
body/type/call-graph proof excludes synchronization, channels, callbacks, interfaces,
unsafe pointers, output, explicit panic/recovery and unproved cycles. Existing step
and type-graph budgets apply. Classic monotone counters may start at nonnegative
portable constants and use named int types; strict bounds, exact progress and no-wrap
requirements remain. No runtime call gains a startup proof or a pure-effect summary.
Previously cached pure/finite initializer summaries still require their own fresh proofs.

Six TLC cases cover maps, named/nonzero loops, shared arrays/publication and a subsequent
deadlock. Old shared/published array initializer refusals are promoted under the new
startup-only contract, with matching runtime refusals retained. Native Go checks the
keyword table, all ASCII regexp.QuoteMeta results and a named-counter map. Refusal and
current-graph tests cover effect/type/control boundaries. Initial gate failures are
preserved (old expectations and a native oracle's overescaped backslash, both corrected).
Strict gate/full race pass with no skips/failures; 279 semantic + 20 CLI TLC cases,
unchanged snapshots.

Actual go/token.init#1 and regexp.init#1 now have consumed `initializer-data` proofs
and explicit startup/data-abstraction assumptions. Initializer refusals decrease from
84 to 82; other diagnostic counts remain unchanged. Data payload correctness, finite
input/environment bounds, memory/time limits and general runtime mutation semantics
are not established. The real CLI still returns unsupported / 5, with no executable
self-model or self TLC command. Evidence:
`$HOME/tmp/pi/gotla-self-bootstrap-initializer-data/`.
