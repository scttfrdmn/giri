package main

import "unsafe"

func main() {
	x := new([4]byte)
	p := unsafe.Add(unsafe.Pointer(x), 1<<20)
	_ = (*byte)(p)
}
