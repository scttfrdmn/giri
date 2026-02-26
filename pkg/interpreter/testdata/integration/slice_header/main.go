package main

import (
	"reflect"
	"unsafe"
)

func main() {
	s := []int{1, 2, 3}
	// Rule 6: casting unsafe.Pointer to *reflect.SliceHeader is deprecated.
	// Use unsafe.SliceData / unsafe.Slice instead (Go 1.17+).
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&s))
	_ = sh
}
