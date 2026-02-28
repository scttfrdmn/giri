// container_list verifies that container/list, container/heap, and
// container/ring intercepts work (#99).
//
// Expected: 0 violations.
package main

import (
	"container/list"
	"container/ring"
)

func main() {
	// container/list.
	l := list.New()
	l.PushBack(1)
	l.PushBack(2)
	l.PushFront(0)
	_ = l.Len()
	f := l.Front()
	_ = f
	b := l.Back()
	_ = b
	l.Init()

	// container/ring.
	r := ring.New(3)
	_ = r.Len()
	r2 := r.Next()
	_ = r2
	r3 := r.Prev()
	_ = r3
	r4 := r.Move(1)
	_ = r4
	r.Do(func(v interface{}) {
		_ = v
	})
}
