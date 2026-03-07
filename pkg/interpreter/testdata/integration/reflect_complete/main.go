// reflect_complete exercises reflect package additions from v0.75.0:
// NewAt, Complex/SetComplex/CanComplex, OverflowInt/OverflowUint/OverflowFloat/OverflowComplex,
// Select, Swapper, SetZero, Equal.
// NOTE: Pointer/UnsafePointer/UnsafeAddr are intentionally omitted — their return
// values (uintptr) legitimately trigger Rule 5 (unsafe.Pointer survives GC point).
// Also exercises log.Output.
// Expected: 0 violations.
package main

import (
	"log"
	"reflect"
	"unsafe"
)

func main() {
	// reflect.NewAt — pass an unsafe.Pointer in a single expression (Rule 5 safe).
	var x int = 42
	v := reflect.NewAt(reflect.TypeOf(x), unsafe.Pointer(&x))
	_ = v

	// reflect.Value.Complex / SetComplex / CanComplex.
	c := complex(1.0, 2.0)
	vc := reflect.ValueOf(&c).Elem()
	_ = vc.Complex()
	vc.SetComplex(complex(3.0, 4.0))
	_ = vc.CanComplex()

	// reflect.Value.Overflow* predicates.
	vi := reflect.ValueOf(int8(0))
	_ = vi.OverflowInt(1000)
	vu := reflect.ValueOf(uint8(0))
	_ = vu.OverflowUint(1000)
	vf := reflect.ValueOf(float32(0))
	_ = vf.OverflowFloat(1e40)
	vcc := reflect.ValueOf(complex64(0))
	_ = vcc.OverflowComplex(complex(1e40, 0))

	// reflect.Select.
	ch := make(chan int, 1)
	ch <- 1
	cases := []reflect.SelectCase{
		{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)},
	}
	chosen, recv, recvOK := reflect.Select(cases)
	_, _, _ = chosen, recv, recvOK

	// reflect.Swapper.
	sl := []int{3, 1, 2}
	swap := reflect.Swapper(sl)
	_ = swap

	// reflect.Value.SetZero (Go 1.20).
	var n int
	vn := reflect.ValueOf(&n).Elem()
	vn.SetZero()

	// reflect.Value.Equal (Go 1.20).
	va := reflect.ValueOf(1)
	vb := reflect.ValueOf(2)
	_ = va.Equal(vb)

	// log.Output.
	logger := log.New(log.Writer(), "test: ", 0)
	_ = logger.Output(2, "hello from logger")
	_ = log.Output(2, "hello from package")
}
