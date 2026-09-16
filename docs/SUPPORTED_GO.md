# Supported Go: current MVP

This guide describes implemented behavior, not the target in
[ROADMAP.md](../ROADMAP.md). The full semantic contract is in
[ARCHITECTURE.md](../ARCHITECTURE.md).

## Read the outcome first

| Extraction outcome | Meaning |
|---|---|
| `precisely-modeled` | Precise within the declared communication-only domain and environment assumptions, not all Go semantics |
| `conservatively-abstracted` | Possible control behavior is retained with abstraction; infeasible traces may be added |
| `unsupported` | Safe extraction was not established; executable TLA+ must not be generated |

These are extraction outcomes, not TLC results. In particular, a precisely modeled
program may deadlock. Successful extraction does not mean successful verification.

## Supported patterns and limits

| Go pattern | Current treatment |
|---|---|
| One main package, direct acyclic helper calls | Supported; helpers are lowered in the caller's process |
| `go worker(ch)` / statically bound closure | One statically known process per reachable spawn context |
| `make(chan T)` / constant capacity | Supported; capacity must be in 0..1024 |
| Send / receive / close | Supported, including nil blocking, closed-channel receive and synchronization errors |
| `select`, including default | Ready cases are nondeterministic; default is unavailable when a communication is ready |
| Channel parameters / direction conversions | Supported when identity is statically resolved |
| Captured single-assignment channel cell | Initialization must dominate every load and capture; no later mutation |
| Local or direct global Mutex / WaitGroup | Stable identities; passing/copying these objects by value is rejected |
| Inline Mutex / WaitGroup fields, including nested value structs | Static allocation/global object plus field path; zero initialization only for synchronization state |
| Direct pointer-receiver methods on these objects | Supported with statically resolved receiver identities; closures can capture stable object pointers |
| Immutable channel fields in local allocated objects | At most one allocation-frame initialization, dominating every read and object escape; omitted initialization means nil |
| Mutex Lock / Unlock | Boolean lock availability, including unlock from another goroutine and invalid-unlock errors |
| Direct `defer mu.Unlock()` / `defer wg.Done()` | Static receiver capture, conditional registration and LIFO cleanup at normal returns; at most 64 sites per invocation |
| WaitGroup Add / Done / Wait | Single phase only; positive Add in main before any spawn or prior Wait; constant delta in -1024..1024 |
| Local arithmetic / payload processing | Removed when irrelevant and otherwise within supported computation/call rules |
| Nonconstant concurrency-controlling predicate | Abstract nondeterministic branch with explicit commitment before blocking |
| Channel payload and ordinary memory value correlations | Not tracked; conditions depending on them may be independently abstracted |
| Explicit trusted call | User asserts total, side-effect-free, normally returning, nonsynchronizing behavior; results remain abstract |

The current scope does not verify payload correctness or arbitrary shared-memory
properties. Recognizing a type or method name alone does not establish safe
identity or supported surrounding control flow.

### A supported communication skeleton

```go
func worker(ch chan int) { ch <- 1 }

func main() {
    ch := make(chan int)
    go worker(ch)
    <-ch
}
```

See the runnable [unbuffered example](../examples/unbuffered/main.go). Local
computation between behavioral boundaries need not become separate model steps.

### Static synchronization fields

```go
type component struct { mu sync.Mutex }

func (c *component) work(done chan int) {
    c.mu.Lock()
    c.mu.Unlock()
    done <- 1
}

func main() {
    c := new(component)
    done := make(chan int)
    go c.work(done)
    <-done
}
```

With `import "sync"`, this is supported without replacing the component's locking.
Distinct objects and distinct inline Mutex/WaitGroup fields remain distinct resources; repeated
access through a direct pointer parameter/receiver refers to the same resource.
Nested value-struct fields and zero-initialized direct globals are supported too.
Copying/loading whole synchronization-bearing aggregates, value receivers/arguments,
whole-object resets and initializer copies are rejected, even for zero values.
Pointer fields, array/slice elements, returned object identities, nil receivers
and ambiguous object sources remain unsupported. Existing captured-cell dominance
checks are not relaxed. Restricted normal-return cleanup is described below.

### Immutable channel fields

```go
type endpoint struct { ch chan int }

func main() {
    var e endpoint
    e.ch = make(chan int)
    go func() { e.ch <- 1 }()
    <-e.ch
}
```

Channel fields may also be initialized in a local struct literal, including nested
value fields. Direct pointer-receiver calls and bound methods can read them. Fields
initialized with the same channel retain that alias; fields initialized with different
allocations remain distinct. An uninitialized field is nil, with normal nil-channel
blocking/close-error behavior, not an invented channel.

The proof inspects every SSA use, not only sliced operations. A field can have at
most one store in its object's allocation frame. That store must dominate every
field read and every call/capture/spawn exposing the object or its subobjects.
This conservatively requires even unrelated channel-field initializations before
an object escape. Initializers use direct allocations, channel parameters, already
resolved captures, nil, or direction/type conversions of those values. Field stores
seed the dependency slice; only proved initialization stores may be erased.

Reassignment, initialization in a called method, field-address escape, whole-object
copy/reset, field-to-field initializer loads, global channel-field access and returned
objects remain unsupported. Capturing an addressable value object (`var e endpoint`)
is supported after initialization; capturing a pointer variable through a `**endpoint`
cell is currently rejected, even when a human can see it is immutable. Direct pointer
receiver calls such as `go e.send()` do not require that pointer-cell capture.

### Restricted deferred cleanup

```go
func (c *component) work(done chan int) {
    c.mu.Lock()
    defer c.mu.Unlock()
    done <- 1
}
```

Only direct `sync.Mutex.Unlock` and `sync.WaitGroup.Done` method calls are accepted
as defers. Receivers are resolved from the values evaluated at registration, not
from later variable assignments. Reaching a defer registers cleanup; it does not
unlock or decrement immediately. Conditional/unselected registrations remain exact.
Cleanup executes in LIFO order at normal returns, after return-expression evaluation.
A function blocked before returning has not executed its deferred cleanup.

Each inlined invocation has separate registration flags. Nested callees drain only
their own registrations. Flags are cleared as calls execute; multiple compiler
`RunDefers` points cannot run the same registration twice. Main's own cleanup precedes
program termination; workers interrupted by main return are not guaranteed cleanup.

Control remains acyclic, so every defer site executes at most once per invocation.
The implementation accepts at most 64 sites; exceeding this is unsupported, never
truncated cleanup or an assumed stack bound. Cleanup effects retain the defer site's
source position. Plain helper-function/closure defers, bound method values, deferred
Lock/Wait/Add/close, foreign defer stacks, initializer defers, panic/recover and loops
remain unsupported. A trusted-call contract does not extend this whitelist. Implicit
sequential panics remain excluded by the recorded domain assumption; this is not
panic unwinding or exception-safety verification.

### Unknown predicates require effect analysis

```go
if external.ShouldRetry(req) {
    ch <- req
}
```

The predicate result may be abstract, but that is **not** permission to ignore
unknown effects of `ShouldRetry`. Its body must be supported, or the user must
provide a truthful `-trust-call` contract. An unresolved call without such a
contract is rejected even when its result is discarded. A call that may block,
spawn, mutate shared state, or fail to return must not be declared pure just to
make analysis succeed.

### Initialization order matters

```go
var ch chan int
func() { ch <- 1 }()
ch = make(chan int, 1)
```

The later assignment cannot supply an identity to the earlier nil send. The MVP
rejects this captured-cell pattern with `sync-initialization-order`, rather than
retroactively substituting the new channel. Some safe capture-before-initialization
patterns are also rejected; this is intentional conservative scope restriction.

## Unsupported patterns

| Pattern | Why it is currently rejected |
|---|---|
| Reachable loops, even constant-count loops; recursion | No implemented bound/termination proof pass |
| Repeated/dynamic spawning or channel topology | No proved finite identity expansion |
| Mutable/global channel fields, pointer fields and synchronization objects in containers | Only local allocation-frame immutable channel fields and inline Mutex/WaitGroup fields have identity proofs |
| Different-identity phis, changing captures, returned channel topology | Identity cannot be selected safely by current rules |
| Dynamic callbacks/interface dispatch | No safe resolved-call support for these sites |
| General `defer`, explicit panic/recover | Only direct deferred Unlock/Done on normal-return paths are modeled; no panic unwinding |
| RWMutex and other unsupported synchronization APIs | No corresponding implemented semantics |
| WaitGroup reuse or concurrent positive enrollment | Counter-only representation does not model waiter generations |
| Reachable unsafe pointer operations, including inspected helpers/init | Can bypass modeled resource identity/state |
| Application/dependency initialization with concurrency, unknown calls, or cycles | Initializers are checked, not silently dropped |

Narrow standard-library initializer summaries are recorded in the model. They do
not authorize ordinary atomic APIs, arbitrary standard-library calls, or
third-party initializers. Unreachable functions are not rejected merely for
containing unsupported instructions.

## Assumptions and intended usage

- Implicit sequential runtime panics and resource exhaustion are excluded by an
  explicit supported-domain assumption. Synchronization errors are **not** excluded.
- Normal main return terminates all goroutines. Blocked workers after main exits
  are not automatically reported as a whole-program deadlock.
- Use one main package as the analysis entry. For a library, write an explicit main
  harness that chooses the inputs/resources under test and calls the real component;
  do not rewrite its concurrency away. The result covers that harness, not all users
  of the library.
- Inspect diagnostics and assumptions before interpreting TLC results. Unsupported
  source should be reduced or reported, not bypassed with false trust contracts.

See [verification guidance](VERIFICATION.md) and the
[semantic regression matrix](SEMANTIC_TEST_MATRIX.md) for executable evidence and
known coverage gaps.
