package effects

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
)

func TestScalarFormatNativeBoundary(t *testing.T) {
	source := `package main
import("fmt";"sync")
var calls int
type N int
func(n N)String()string{calls++;return "callback"}
func main(){var mu sync.Mutex;mu.Lock();s:=fmt.Sprintf("%s/%d/%t/%v","x",7,true,nil);mu.Unlock();bad:=fmt.Sprintf("%v",N(1));fmt.Println(s,bad,calls)}
`
	p := testutil.Load(t, source)
	main, _ := p.Main()
	a := New(p, nil)
	accepted, refused := 0, 0
	for _, bb := range main.Blocks {
		for _, i := range bb.Instrs {
			site, ok := i.(*ssa.Call)
			if !ok || site.Common().StaticCallee() == nil {
				continue
			}
			switch site.Common().StaticCallee().String() {
			case "fmt.Sprintf":
				if a.ProveScalarFormat(site) != nil {
					accepted++
				} else {
					refused++
				}
			case "fmt.Println":
				if a.ProveScalarFormat(site) != nil {
					t.Fatal("output I/O admitted")
				}
			}
		}
	}
	if accepted != 1 || refused != 1 {
		t.Fatalf("model boundary=%d/%d", accepted, refused)
	}
	cmd := exec.CommandContext(t.Context(), "go", "run", ".")
	cmd.Dir = filepath.Dir(p.Fset.Position(main.Pos()).Filename)
	cmd.Env = append(os.Environ(), "GOFLAGS=", "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("native: %v: %s", err, out)
	}
	if strings.TrimSpace(string(out)) != "x/7/true/<nil> callback 1" {
		t.Fatalf("native scalar/callback behavior differs: %s", out)
	}
}
