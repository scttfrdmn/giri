// fieldaddr_valid verifies that valid struct field accesses produce no violations.
//
// Expected: 0 violations.
package main

type Point struct {
	X, Y int
}

func newPoint(x, y int) *Point {
	return &Point{X: x, Y: y}
}

func main() {
	p := newPoint(3, 4)
	_ = p.X
	_ = p.Y

	// Stack-allocated struct: also valid.
	q := Point{X: 1, Y: 2}
	_ = q.X
}
