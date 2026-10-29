package exec

import (
	"go/types"

	"github.com/scttfrdmn/giri/pkg/interpreter"
)

// IsConcrete reports whether calls to fn are modeled concretely: the
// interpreter computes exactly what native code computes (giri's concrete
// intercept table wraps the real stdlib function). Functions giri models
// opaquely (placeholder results, as the crypto key-generation intercepts
// return) or approximately, methods, and functions outside the table report
// false. Functions of the loaded package itself are interpreted from source
// and are not covered by this query.
func IsConcrete(fn *types.Func) bool {
	if fn == nil || fn.Pkg() == nil {
		return false
	}
	if sig, ok := fn.Type().(*types.Signature); ok && sig.Recv() != nil {
		return false
	}
	return interpreter.IsConcreteIntercept(fn.Pkg().Path(), fn.Name())
}

// ConcreteFuncs lists every concretely modeled function as "pkg/path.Name",
// sorted. This is the set a grammar may admit.
func ConcreteFuncs() []string { return interpreter.ConcreteIntercepts() }
