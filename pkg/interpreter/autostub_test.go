package interpreter

import (
	"go/types"
	"testing"
)

// errorType returns the predeclared error interface type from the universe.
func errorType() types.Type {
	return types.Universe.Lookup("error").Type()
}

func TestZeroForType(t *testing.T) {
	ptr := types.NewPointer(types.Typ[types.Int])
	iface := types.NewInterfaceType(nil, nil)
	slice := types.NewSlice(types.Typ[types.Byte])

	tests := []struct {
		name string
		typ  types.Type
		want interface{} // expected .Raw
	}{
		{"int", types.Typ[types.Int], int64(0)},
		{"int64", types.Typ[types.Int64], int64(0)},
		{"uint32", types.Typ[types.Uint32], int64(0)},
		{"uintptr", types.Typ[types.Uintptr], int64(0)},
		{"float64", types.Typ[types.Float64], float64(0)},
		{"float32", types.Typ[types.Float32], float64(0)},
		{"complex128", types.Typ[types.Complex128], complex128(0)},
		{"bool", types.Typ[types.Bool], false},
		{"string", types.Typ[types.String], ""},
		{"pointer", ptr, struct{}{}},                                   // opaque
		{"interface", iface, struct{}{}},                               // opaque
		{"slice", slice, struct{}{}},                                   // opaque
		{"unsafe.Pointer", types.Typ[types.UnsafePointer], struct{}{}}, // opaque
		{"error", errorType(), nil},                                    // nil error
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := zeroForType(tt.typ).Raw
			if got != tt.want {
				t.Errorf("zeroForType(%s).Raw = %#v, want %#v", tt.name, got, tt.want)
			}
		})
	}
}

func TestZeroForTypeNil(t *testing.T) {
	// A nil type degrades to opaque rather than panicking.
	if got := zeroForType(nil).Raw; got != (struct{}{}) {
		t.Errorf("zeroForType(nil).Raw = %#v, want opaque", got)
	}
}

func TestZeroResultValue(t *testing.T) {
	ptr := types.NewPointer(types.Typ[types.Int])

	// nil tuple → single opaque. A void function's sig.Results() is a nil
	// *types.Tuple (not a non-nil empty one), and the test/deferred callers pass
	// nil explicitly, so nil is the "no shape known" case → conservative opaque.
	if got := zeroResultValue(nil).Raw; got != (struct{}{}) {
		t.Errorf("zeroResultValue(nil).Raw = %#v, want opaque", got)
	}

	// 1 result → the scalar zero directly (not wrapped in a slice).
	single := types.NewTuple(types.NewVar(0, nil, "", types.Typ[types.Int]))
	if got := zeroResultValue(single).Raw; got != int64(0) {
		t.Errorf("zeroResultValue(int).Raw = %#v, want int64(0)", got)
	}

	// (T, error) → []Value{opaque, nil} so Extract unpacks both slots correctly.
	pair := types.NewTuple(
		types.NewVar(0, nil, "", ptr),
		types.NewVar(0, nil, "", errorType()),
	)
	got := zeroResultValue(pair)
	elems, ok := got.Raw.([]Value)
	if !ok {
		t.Fatalf("zeroResultValue((*int, error)).Raw = %#v, want []Value", got.Raw)
	}
	if len(elems) != 2 {
		t.Fatalf("want 2 elems, got %d", len(elems))
	}
	if elems[0].Raw != (struct{}{}) {
		t.Errorf("slot 0 = %#v, want opaque (non-nil)", elems[0].Raw)
	}
	if elems[1].Raw != nil {
		t.Errorf("slot 1 (error) = %#v, want nil", elems[1].Raw)
	}

	// 3-return (int, T, error) shape.
	triple := types.NewTuple(
		types.NewVar(0, nil, "", types.Typ[types.Int]),
		types.NewVar(0, nil, "", ptr),
		types.NewVar(0, nil, "", errorType()),
	)
	elems3, ok := zeroResultValue(triple).Raw.([]Value)
	if !ok || len(elems3) != 3 {
		t.Fatalf("triple: want []Value len 3, got %#v", zeroResultValue(triple).Raw)
	}
	if elems3[0].Raw != int64(0) || elems3[1].Raw != (struct{}{}) || elems3[2].Raw != nil {
		t.Errorf("triple shapes = %#v, %#v, %#v", elems3[0].Raw, elems3[1].Raw, elems3[2].Raw)
	}
}
