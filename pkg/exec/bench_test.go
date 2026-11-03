package exec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// BenchmarkKernelBatch measures Runs shaped like tumble's evaluations: one
// small kernel on 64 argument sets per Run, plus a single set, where per-Run
// and per-call overhead dominate over instruction cost (#270).
func BenchmarkKernelBatch(b *testing.B) {
	mix := make([][]any, 64)
	sums := make([][]any, 64)
	for i := range mix {
		mix[i] = []any{uint64(i*7919 + 3), uint64(i*104729 + 1)}
		xs := make([]uint32, 16)
		for j := range xs {
			xs[j] = uint32(i*j + 1)
		}
		sums[i] = []any{xs}
	}
	for _, m := range []struct {
		name string
		opts Options
	}{
		{"default", Options{}},
		{"both", Options{SingleGoroutine: true, SkipDetectors: true}},
	} {
		b.Run("MixU64x64/"+m.name, func(b *testing.B) { benchKernel(b, "MixU64", mix, m.opts) })
		b.Run("SumU32x64/"+m.name, func(b *testing.B) { benchKernel(b, "SumU32", sums, m.opts) })
		b.Run("MixU64x1/"+m.name, func(b *testing.B) { benchKernel(b, "MixU64", mix[:1], m.opts) })
	}
}

func benchKernel(b *testing.B, name string, args [][]any, opts Options) {
	fn := kernelFunc(b, name)
	var steps uint64
	for b.Loop() {
		res, err := Run(fn, args, opts)
		if err != nil {
			b.Fatal(err)
		}
		steps += res.Steps
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(steps), "ns/instr")
}

// BenchmarkLoadDir measures loading and building a generated package of 300
// small functions, with no imports and with seven stdlib imports (#270).
func BenchmarkLoadDir(b *testing.B) {
	for _, imports := range []bool{false, true} {
		name := "noimports"
		if imports {
			name = "stdlib7"
		}
		b.Run(name, func(b *testing.B) {
			dir := writeGenPackage(b, 300, imports)
			for b.Loop() {
				if _, err := LoadDir(dir); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// genSource returns the source of a package gen with n trivial functions
// F000…, optionally importing (and using) seven stdlib packages.
func genSource(n int, imports bool) string {
	var src strings.Builder
	src.WriteString("package gen\n")
	if imports {
		src.WriteString(`
import (
	"math"
	"math/bits"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var _ = []any{math.Abs, bits.Len, sort.Ints, strconv.Itoa, strings.ToUpper, unicode.IsDigit, utf8.RuneLen}
`)
	}
	for i := 0; i < n; i++ {
		fmt.Fprintf(&src, "\nfunc F%03d(x int) int { return x + %d }\n", i, i)
	}
	return src.String()
}

// writeGenPackage writes genSource(n, imports) as a module and returns its
// directory.
func writeGenPackage(tb testing.TB, n int, imports bool) string {
	tb.Helper()
	dir := tb.TempDir()
	for name, content := range map[string]string{
		"go.mod": "module gen\n\ngo 1.26\n",
		"gen.go": genSource(n, imports),
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			tb.Fatal(err)
		}
	}
	return dir
}

// BenchmarkLoader measures Loader.Load on the same packages as
// BenchmarkLoadDir, after one warm-up load caches the dependencies (#273).
func BenchmarkLoader(b *testing.B) {
	for _, imports := range []bool{false, true} {
		name := "noimports"
		if imports {
			name = "stdlib7"
		}
		b.Run(name, func(b *testing.B) {
			src := []byte(genSource(300, imports))
			l := NewLoader()
			if _, err := l.Load("gen", src); err != nil {
				b.Fatal(err)
			}
			for b.Loop() {
				if _, err := l.Load("gen", src); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
