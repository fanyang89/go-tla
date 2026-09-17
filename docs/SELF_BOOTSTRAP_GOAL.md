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
