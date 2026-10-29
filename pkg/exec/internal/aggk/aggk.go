// Package aggk holds array and struct kernels from the tumble session's review
// of #258, run natively and interpreted by pkg/exec's TestAggregateKernels.
// Some contain deliberate oddities (x == x on NaN, dead writes a defer
// depends on), so lint is relaxed for this directory.
package aggk

import "image"

type S struct {
	A int
	B [3]int
	C float64
}

type P struct{ X, Y int }

type M struct {
	I8  int8
	U16 uint16
	B   bool
	F   float32
}

type Box struct {
	Ps [2]P
	N  int
}

func CopyArr(x int) []int {
	a := [4]int{1, 2, 3, 4}
	b := a
	b[1] = x
	b[3]++
	return []int{a[0], a[1], a[2], a[3], b[0], b[1], b[2], b[3]}
}

func CopyStruct(x int) []int {
	s := S{A: 1, B: [3]int{4, 5, 6}, C: 2}
	t := s
	t.A = x
	t.B[1] = x * 2
	return []int{s.A, s.B[0], s.B[1], s.B[2], int(s.C), t.A, t.B[0], t.B[1], t.B[2], int(t.C)}
}

//go:noinline
func modArr(a [4]int, x int) int {
	a[0] = x
	a[2] += x
	return a[0] + a[1] + a[2] + a[3]
}

//go:noinline
func modStruct(s S, x int) int {
	s.A = x
	s.B[2] = x
	return s.A + s.B[0] + s.B[1] + s.B[2]
}

func PassByValue(x int) []int {
	a := [4]int{1, 2, 3, 4}
	s := S{A: 7, B: [3]int{1, 1, 1}}
	r1 := modArr(a, x)
	r2 := modStruct(s, x)
	return []int{r1, r2, a[0], a[2], s.A, s.B[2]}
}

//go:noinline
func mkArr(x int) [4]int {
	var a [4]int
	for i := range a {
		a[i] = x + i
	}
	return a
}

//go:noinline
func mkStruct(x int) S { return S{A: x, B: [3]int{x, x + 1, x + 2}, C: float64(x) / 2} }

func Returned(x int) []int {
	a := mkArr(x)
	b := mkArr(x + 10)
	a[0] = 99
	s := mkStruct(x)
	s2 := s
	s2.B[0] = -1
	return []int{a[0], a[1], a[3], b[0], b[3], s.A, s.B[0], s.B[2], s2.B[0], int(s.C * 10)}
}

func ArrOfStruct(x int) []int {
	var ps [3]P
	ps[1].X = x
	q := &ps[2].Y
	*q = x * 3
	p := &ps[0]
	p.Y = 5
	c := ps
	c[1].X = 100
	v := ps[1]
	v.Y = 77
	return []int{ps[0].X, ps[0].Y, ps[1].X, ps[1].Y, ps[2].X, ps[2].Y, c[1].X, v.X, v.Y}
}

func StructOfArr(x int) []int {
	var b Box
	b.Ps[1].Y = x
	pp := &b.Ps[0].X
	*pp = 4
	b.N = len(b.Ps)
	c := b
	c.Ps[1] = P{9, 9}
	inner := b.Ps
	inner[0].X = -3
	return []int{b.Ps[0].X, b.Ps[0].Y, b.Ps[1].X, b.Ps[1].Y, b.N, c.Ps[1].X, c.Ps[0].X, inner[0].X}
}

func Equality(x int) []bool {
	a := [4]int{1, 2, 3, x}
	b := [4]int{1, 2, 3, 4}
	var z [4]int
	var zs S
	s := S{}
	e1 := a == b
	b[3] = x
	e2 := a == b
	s.B[1] = x
	e3 := s == zs
	s.B[1] = 0
	e4 := s == zs
	e5 := z == [4]int{}
	p1, p2 := P{1, x}, P{1, 4}
	e6 := p1 != p2
	var bx, by Box
	by.Ps[1].Y = x
	e7 := bx == by
	return []bool{e1, e2, e3, e4, e5, e6, e7}
}

func FloatEq(x float64) []bool {
	z := 0 * x
	a := [2]float64{x, 0}
	b := [2]float64{x, -z} // negative zero for x >= 0: equal to +0
	type F struct{ F float32 }
	f1, f2 := F{float32(x)}, F{float32(x)}
	return []bool{a == b, f1 == f2, a == a}
}

func Literals(x int) []int {
	a := [5]int{x, 2}
	b := [5]int{3: x, 1: 7}
	c := [...]int{4: 1}
	s := S{B: [3]int{2: x}}
	p := P{Y: x}
	return []int{a[0], a[1], a[4], b[0], b[1], b[3], len(c), c[4], s.A, s.B[2], p.X, p.Y}
}

func RangeCopy(x int) []int {
	a := [3]int{1, 2, 3}
	var out []int
	for i, v := range a {
		a[2] = x
		out = append(out, i, v)
	}
	ps := [2]P{{1, 2}, {3, 4}}
	for _, p := range ps {
		ps[1].X = x
		out = append(out, p.X, p.Y)
	}
	return out
}

func Accum(n int) []int {
	var acc [4]int
	for i := 0; i < n; i++ {
		acc[i%4] += i
		next := acc
		next[0] = acc[3]
		acc = next
	}
	return acc[:]
}

func VarIndex(i int) int {
	a := [4]int{10, 20, 30, 40}
	v := a // value copy
	return v[i]
}

func VarIndexAddr(i int) int {
	a := [4]int{10, 20, 30, 40}
	a[i] = 5
	return a[0] + a[1] + a[2] + a[3]
}

func ValIndexStruct(i int) int {
	ps := [3]P{{1, 2}, {3, 4}, {5, 6}}
	f := func() [3]P { return ps }
	return f()[i].Y
}

func PtrArr(x int) []int {
	a := [4]int{1, 2, 3, 4}
	p := &a
	p[2] = 9
	b := *p
	*p = [4]int{x, x, x, x}
	p[1] = 0
	return []int{a[0], a[1], a[2], a[3], b[2], b[0]}
}

func PtrStruct(x int) []int {
	s := S{A: 1}
	p := &s
	*p = S{A: x, B: [3]int{1, 2, 3}}
	q := *p
	p.B[0] = 50
	return []int{s.A, s.B[0], s.B[1], q.B[0]}
}

func Mixed(a int8, b uint16, f float32) []float64 {
	m := M{I8: a, U16: b, B: a > 0, F: f}
	m.I8 += 100
	m.U16 *= 3
	m.F = m.F * 1.1
	n := m
	n.I8 -= 120
	bb := 0.0
	if m.B {
		bb = 1
	}
	return []float64{float64(m.I8), float64(m.U16), bb, float64(m.F), float64(n.I8)}
}

func MixedArr(a int8) []int8 {
	var arr [3]int8
	arr[0] = a
	arr[1] = a + 100
	c := arr
	c[2] = c[1] * 2
	return []int8{arr[0], arr[1], arr[2], c[2]}
}

func SliceOfStruct(x int) []int {
	s := make([]P, 3)
	s[1].Y = x
	s[2] = P{7, 8}
	v := s[2]
	return []int{s[0].X, s[1].Y, s[2].X, s[2].Y, v.X, v.Y}
}

func AppendStruct(x int) []int {
	var s []P
	s = append(s, P{1, x})
	s = append(s, P{3, 4})
	return []int{s[0].X, s[0].Y, s[1].X, s[1].Y}
}

func CopyStructSlice(x int) []int {
	a := []P{{1, x}, {3, 4}}
	b := make([]P, 2)
	copy(b, a)
	return []int{b[0].X, b[0].Y, b[1].X, b[1].Y}
}

func AppendArr(x int) []int {
	var s [][2]int
	s = append(s, [2]int{x, x + 1})
	return []int{s[0][0], s[0][1]}
}

func SliceLitStruct(x int) []int {
	s := []P{{1, x}, {3, 4}}
	t := s[1]
	return []int{s[0].Y, t.X, t.Y}
}

func RangeSliceStruct(x int) []int {
	s := []P{{1, x}, {3, 4}}
	var out []int
	for _, p := range s {
		out = append(out, p.X, p.Y)
	}
	return out
}

func MapStructKey(x int) []int {
	m := map[P]int{}
	m[P{1, 2}] = 5
	m[P{1, x}] += 3
	return []int{len(m), m[P{1, 2}]}
}

func MapArrKey(x int) []int {
	m := map[[2]int]int{}
	m[[2]int{1, 2}] = 5
	m[[2]int{1, x}] += 3
	return []int{len(m), m[[2]int{1, 2}]}
}

func MapStructVal(x int) []int {
	m := map[int]P{}
	m[1] = P{x, 2}
	p := m[1]
	p.X++
	return []int{m[1].X, m[1].Y, p.X}
}

func IfaceEq(x int) []bool {
	var a, b any = [2]int{1, x}, [2]int{1, 2}
	var c, d any = P{1, x}, P{1, 2}
	return []bool{a == b, c == d}
}

func IfaceAssert(x int) []int {
	var a any = P{x, 3}
	p := a.(P)
	p.X++
	q := a.(P)
	return []int{p.X, q.X, q.Y}
}

func ArrSlice(x int) []int {
	a := [4]int{4, 3, 2, x}
	s := a[1:3]
	s[0] = 100
	b := a
	return []int{a[1], b[1], len(s), s[1]}
}

func Closure(x int) []int {
	a := [3]int{1, 2, 3}
	f := func() { a[1] = x }
	b := a
	f()
	return []int{a[1], b[1]}
}

func Phi(x int) []int {
	a := [2]int{1, 2}
	b := [2]int{3, 4}
	var c [2]int
	if x > 0 {
		c = a
	} else {
		c = b
	}
	a[0] = 9
	return []int{c[0], c[1]}
}

func StructStrings(x string) []string {
	type T struct {
		S string
		A [2]string
	}
	t := T{S: x, A: [2]string{"a", x}}
	u := t
	u.A[0] = "z"
	eq := "n"
	if t == (T{x, [2]string{"a", x}}) {
		eq = "y"
	}
	return []string{t.S, t.A[0], t.A[1], u.A[0], eq}
}

func Swap(x int) []int {
	a := [2]P{{1, 2}, {3, x}}
	a[0], a[1] = a[1], a[0]
	return []int{a[0].X, a[0].Y, a[1].X, a[1].Y}
}

func Nested2D(x int) []int {
	var g [3][3]int
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			g[i][j] = i*3 + j + x
		}
	}
	row := g[1]
	row[0] = -1
	h := g
	h[2][2] = 0
	return []int{g[1][0], row[0], g[2][2], h[2][2], h[0][1]}
}

func ZeroSizeAndBlank(x int) []bool {
	type Z struct {
		E struct{}
		A int
		_ int
		B [0]int
		C int
	}
	z1 := Z{A: x, C: 2}
	z2 := Z{A: x, C: 2}
	return []bool{z1 == z2, z1.A == x}
}

func PtrInStruct(x int) []int {
	type R struct {
		P *int
		V int
	}
	n := x
	r := R{&n, 1}
	r2 := r
	*r2.P = 42
	return []int{n, *r.P, r2.V}
}

func DeferArr(x int) (res int) {
	a := [2]int{x, 1}
	defer func(b [2]int) { res = b[0] + b[1] }(a)
	a[0] = 1000
	return 0
}

func MapScalar(x int) []int {
	m := map[int]int{}
	m[1] = 5
	m[x] += 3
	return []int{len(m), m[1]}
}

func DeferScalar(x int) (res int) {
	a := x
	defer func(b int) { res = b + 1 }(a)
	a = 1000
	return 0
}

func DeferArrNoParam(x int) (res int) {
	a := [2]int{x, 1}
	b := a
	defer func() { res = b[0] + b[1] }()
	return 0
}

func AppendScalar(x int) []int {
	var s []int
	s = append(s, x, 2)
	return s
}

func IfaceScalarEq(x int) []bool {
	var a, b any = x, 2
	return []bool{a == b}
}

func IfaceArrOnly(x int) []bool {
	var a, b any = [2]int{1, x}, [2]int{1, 2}
	return []bool{a == b}
}

func IfaceStructAssert(x int) []int {
	var a any = [2]int{x, 3}
	b := a.([2]int)
	return []int{b[0], b[1]}
}

var gArr = [3]int{5, 6, 7}
var gS = P{8, 9}

func Globals(x int) []int {
	c := gArr
	gArr[1] = x
	d := gS
	gS.Y = x
	return []int{c[1], gArr[1], d.Y, gS.Y, gArr[2]}
}

type Q P

func Convert(x int) []int {
	p := P{x, 2}
	q := Q(p)
	q.X++
	return []int{p.X, q.X, q.Y}
}

func NaNStruct(x float64) []bool {
	type F struct{ V float64 }
	a := F{x}
	return []bool{a == a, [1]float64{x} == [1]float64{x}}
}

func PtrEqStruct(x int) []bool {
	type R struct{ P *int }
	n, m := x, x
	return []bool{R{&n} == R{&n}, R{&n} == R{&m}}
}

func IfaceFieldEq(x int) []bool {
	type R struct{ I any }
	return []bool{R{x} == R{x}, R{x} == R{x + 1}}
}

func ClearArr(x int) []int {
	a := [3]int{x, x, x}
	clear(a[:])
	return a[:]
}

func AppendIntoArr(x int) []int {
	a := [4]int{1, 2, 3, 4}
	s := append(a[:1], x)
	return []int{a[1], s[1], len(s)}
}

func ConstIndexPtr(x int) int {
	a := [3]int{1, 2, x}
	p := &a
	return p[2] + len(p)
}

func ArrayOfStrings(x string) []string {
	a := [2]string{x, "b"}
	b := a
	b[0] = "c"
	return []string{a[0], b[0]}
}

func StructCompareInLoop(n int) int {
	target := P{3, 6}
	c := 0
	for i := 0; i < n; i++ {
		if (P{i, 2 * i}) == target {
			c++
		}
	}
	return c
}

func ArgSliceToArr(s []int) []int {
	var a [3]int
	copy(a[:], s)
	b := a
	a[0] = -1
	return []int{a[0], b[0], b[1], b[2]}
}

func SliceToArrConv(s []int) []int {
	a := [2]int(s)
	s[0] = 99
	return []int{a[0], a[1]}
}

func ChanStruct(x int) []int {
	ch := make(chan P, 1)
	ch <- P{x, 2}
	p := <-ch
	return []int{p.X, p.Y}
}

func ChanScalar(x int) int {
	ch := make(chan int, 1)
	ch <- x
	return <-ch
}

func GlobalsScalar(x int) []int {
	c := gArr[1]
	return []int{c, gS.Y}
}

func ExtStruct(x int) int {
	p := image.Pt(x, 2)
	return p.X + p.Y
}

func ExtStructArr(x int) int {
	r := image.Rect(0, 0, x, 3)
	q := r
	return q.Max.X
}

func ChanArrIdx(x int) int {
	ch := make(chan [2]int, 1)
	ch <- [2]int{x, 1}
	a := <-ch
	return a[0]
}

func RecoverIdx(i int) (r int) {
	defer func() {
		if recover() != nil {
			r = -1
		}
	}()
	a := [3]int{1, 2, 3}
	v := a
	return v[i]
}

func RecoverSliceIdx(i int) (r int) {
	defer func() {
		if recover() != nil {
			r = -1
		}
	}()
	s := []int{1, 2, 3}
	return s[i]
}

func ClearStructSlice(x int) []int {
	s := []P{{x, x}, {x, x}}
	clear(s)
	return []int{s[0].X, s[0].Y, s[1].X, s[1].Y}
}
