package abstract_test

import (
	"github.com/fanmi/go-tla/internal/abstract"
	"github.com/fanmi/go-tla/internal/behavior"
	"go/constant"
	"go/types"
	"golang.org/x/tools/go/ssa"
	"testing"
)

func TestPredicateConstantAndNondeterminism(t *testing.T) {
	c := ssa.NewConst(constant.MakeBool(true), types.Typ[types.Bool])
	if abstract.Predicate(c, nil, true).Kind != behavior.True || !abstract.Predicate(c, nil, false).Negated {
		t.Fatal("constant predicate lost")
	}
	if abstract.Predicate(nil, nil, true).Kind != behavior.Choice {
		t.Fatal("unknown predicate must be a choice")
	}
}
