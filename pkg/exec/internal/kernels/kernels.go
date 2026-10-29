// Package kernels holds small functions used by pkg/exec tests. Each is
// called natively and interpreted by giri, and the results compared.
package kernels

import (
	"math"
	"math/bits"
	"sort"
	"strings"
)

func AddInt(a, b int) int { return a + b }

func Mix8(a, b int8) int8 { return a*b + a>>1 - b }

func MixU8(a, b uint8) uint8 { return a*b + a>>3 ^ b }

func Mix16(a, b int16) int16 { return a*b - b<<3 }

func MixU16(a, b uint16) uint16 { return a*b + ^a }

func Mix32(a, b int32) int32 { return a*b/3 + a%7 }

func MixU32(a, b uint32) uint32 { return a<<7 | a>>25 ^ b*2654435761 }

func Mix64(a, b int64) int64 { return a*b ^ a>>13 }

func MixU64(a, b uint64) uint64 {
	if a > b {
		return a/(b|1) + a>>60
	}
	return b%(a|1) ^ a*0x9E3779B97F4A7C15
}

func Shifts(x uint32, n uint) (uint32, uint32, int32) {
	return x << n, x >> n, int32(x) >> n
}

// Poly32 and Poly64 use explicit conversions, which the Go spec defines as
// rounding points that prevent fused multiply-add. giri never fuses; native
// arm64 code fuses x*y+z unless prevented (see docs/decisions.md).
func Poly32(x float32) float32 { return float32(float32(x*x)*0.1) + x/3 }

func Poly64(x float64) float64 { return float64(float64(x*x)*x) - float64(2*x) + 0.5 }

func Xor(a, b bool) bool { return a != b }

func Concat(a, b string) string { return a + "-" + b }

func SumU32(xs []uint32) uint32 {
	var s uint32
	for _, x := range xs {
		s += x
	}
	return s
}

func SumF64(xs []float64) float64 {
	s := 0.0
	for i := range xs {
		s += xs[i]
	}
	return s
}

func PrefixSums(xs []int64) []int64 {
	out := make([]int64, len(xs))
	var acc int64
	for i, x := range xs {
		acc += x
		out[i] = acc
	}
	return out
}

func Evens(xs []int) []int {
	var out []int
	for _, x := range xs {
		if x%2 == 0 {
			out = append(out, x)
		}
	}
	return out
}

func Reverse(xs []int16) []int16 {
	for i, j := 0, len(xs)-1; i < j; i, j = i+1, j-1 {
		xs[i], xs[j] = xs[j], xs[i]
	}
	return xs
}

func CountTrue(bs []bool) int {
	n := 0
	for _, b := range bs {
		if b {
			n++
		}
	}
	return n
}

func Join(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += ","
		}
		out += s
	}
	return out
}

func Upper(b []byte) []byte {
	out := make([]byte, len(b))
	copy(out, b)
	for i, c := range out {
		if c >= 'a' && c <= 'z' {
			out[i] = c - 32
		}
	}
	return out
}

func NilOrLen(xs []int) (int, bool) { return len(xs), xs == nil }

func DivMod(a, b uint64) (uint64, uint64) { return a / b, a % b }

func F2I(f float64) int32 { return int32(f) }

func I2F(u uint64) float32 { return float32(u) }

func Pop(x uint64) int { return bits.OnesCount64(x) + bits.LeadingZeros32(uint32(x)) }

func SqrtSum(xs []float64) float64 {
	s := 0.0
	for _, x := range xs {
		s += math.Sqrt(x)
	}
	return s
}

func SortCopy(xs []int) []int {
	ys := append([]int(nil), xs...)
	sort.Ints(ys)
	return ys
}

func Strs(s string) (string, []byte, string, int) {
	mid := s[1 : len(s)-1]
	b := []byte(s)
	b[0] = 'X'
	n := 0
	for _, r := range s {
		n += int(r)
	}
	return mid, b, strings.ToUpper(string(b)), n
}

func Bucket(xs []uint8) int {
	var h [4]int
	for _, x := range xs {
		h[x%4]++
	}
	return h[0]*1000 + h[1]*100 + h[2]*10 + h[3]
}

type pair struct {
	a, b int
	c    float64
}

func Fields(x, y int) float64 {
	var p pair
	p.a = x
	p.b = y * 2
	p.c = float64(p.a+p.b) / 4
	return p.c + float64(p.b)
}

func Fib(n int) int {
	if n < 2 {
		return n
	}
	return Fib(n-1) + Fib(n-2)
}

func ApplyTwice(x int) int {
	k := 3
	f := func(v int) int { return v*k + 1 }
	return f(f(x))
}

func MinMax(a, b, c uint64) (uint64, uint64) { return min(a, b, c), max(a, b, c) }

// TakesMap and ReturnsArray have types outside v0 marshalling.
func TakesMap(m map[string]int) int { return len(m) }

func ReturnsArray() [2]int { return [2]int{1, 2} }
