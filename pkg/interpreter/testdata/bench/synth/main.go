// synth is a mixed workload for BenchmarkCLISynth (#270): four goroutines
// doing sort, map, struct, append and strings.Builder work, synchronized
// with a WaitGroup and a Mutex. It is a benchmark input, not an integration
// test, so it is not in the integration table.
package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

type point struct{ x, y int }

func (p point) dist() int { return p.x*p.x + p.y*p.y }

func work(n int) int {
	xs := make([]int, n)
	for i := range xs {
		xs[i] = (i * 7919) % 1009
	}
	sort.Ints(xs)
	m := map[int]int{}
	for _, x := range xs {
		m[x%97]++
	}
	pts := make([]point, 0, n)
	for i := 0; i < n; i++ {
		pts = append(pts, point{i, n - i})
	}
	s := 0
	for _, p := range pts {
		s += p.dist()
	}
	var b strings.Builder
	for i := 0; i < 50; i++ {
		b.WriteString("x")
	}
	return s + len(m) + len(b.String())
}

func main() {
	var wg sync.WaitGroup
	var mu sync.Mutex
	total := 0
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(k int) {
			defer wg.Done()
			r := work(1500 + k)
			mu.Lock()
			total += r
			mu.Unlock()
		}(g)
	}
	wg.Wait()
	fmt.Println(total)
}
