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
| Mutex Lock / Unlock | Boolean lock availability, including unlock from another goroutine and invalid-unlock errors |
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
Distinct objects and distinct inline fields remain distinct resources; repeated
access through a direct pointer parameter/receiver refers to the same resource.
Nested value-struct fields and zero-initialized direct globals are supported too.
Copying/loading whole synchronization-bearing aggregates, value receivers/arguments,
whole-object resets and initializer copies are rejected, even for zero values.
Pointer/channel fields, array/slice elements, returned object identities, nil receivers
and ambiguous object sources are not supported by this first M4 increment. Existing
captured-cell dominance checks are not relaxed. No deferred cleanup is supported yet.

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
| Channel/pointer fields and synchronization objects in containers | No immutable field-initialization or container identity analysis yet; only inline Mutex/WaitGroup value fields are supported |
| Different-identity phis, changing captures, returned channel topology | Identity cannot be selected safely by current rules |
| Dynamic callbacks/interface dispatch | No safe resolved-call support for these sites |
| `defer`, explicit panic/recover | No modeled deferred-execution/unwinding semantics |
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
