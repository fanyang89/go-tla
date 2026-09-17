package tests

import (
	"testing"

	"github.com/fanmi/go-tla/internal/lowering"
	"github.com/fanmi/go-tla/internal/testutil"
)

func TestTLCRuntimeProcsProfile(t *testing.T) {
	const prefix = `package main;import("runtime";"sync");var _ sync.Mutex;`
	for _, tc := range []struct {
		name, body, want string
		procs            int
	}{
		{"global-two", `var c=make(chan int,runtime.GOMAXPROCS(0));func main(){c<-1;c<-2;<-c;<-c}`, "No error has been found", 2},
		{"global-one-blocks", `var c=make(chan int,runtime.GOMAXPROCS(0));func main(){c<-1;c<-2;<-c;<-c}`, "Error: Deadlock reached", 1},
		{"local", `func main(){c:=make(chan int,runtime.GOMAXPROCS(0));c<-1;c<-2;<-c;<-c}`, "No error has been found", 2},
		{"closed-initial", `var c=make(chan int,runtime.GOMAXPROCS(0));func init(){close(c)};func main(){<-c}`, "No error has been found", 2},
		{"query-after-block", `func main(){var c chan int;<-c;_=runtime.GOMAXPROCS(0)}`, "Error: Deadlock reached", 2},
		{"closed-send", `func main(){c:=make(chan int,runtime.GOMAXPROCS(0));close(c);c<-1}`, "Error: Invariant NoSynchronizationErrors is violated", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testutil.Load(t, prefix+tc.body)
			m, err := lowering.Lower(p, lowering.Options{RuntimeProcs: tc.procs})
			if err != nil {
				t.Fatal(err)
			}
			checkTLC(t, m, tc.want)
		})
	}
}
