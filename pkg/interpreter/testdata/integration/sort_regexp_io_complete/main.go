// sort_regexp_io_complete exercises additions from v0.73.0:
// sort.SearchStrings/SearchInts/SearchFloat64s, sort.StringsAreSorted/IntsAreSorted/Float64sAreSorted,
// regexp.CompilePOSIX/MustCompilePOSIX/Expand/ExpandString/FindSubmatchIndex/FindAllSubmatchIndex,
// io.NewOffsetWriter/SectionReader.Size, path/filepath.IsLocal.
// Expected: 0 violations.
package main

import (
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func main() {
	// sort.SearchStrings / SearchInts / SearchFloat64s.
	ss := []string{"apple", "banana", "cherry"}
	_ = sort.SearchStrings(ss, "banana")
	ii := []int{1, 2, 3, 4, 5}
	_ = sort.SearchInts(ii, 3)
	ff := []float64{1.1, 2.2, 3.3}
	_ = sort.SearchFloat64s(ff, 2.2)

	// sort.StringsAreSorted / IntsAreSorted / Float64sAreSorted.
	_ = sort.StringsAreSorted(ss)
	_ = sort.IntsAreSorted(ii)
	_ = sort.Float64sAreSorted(ff)

	// regexp.CompilePOSIX / MustCompilePOSIX.
	re1, err1 := regexp.CompilePOSIX(`[a-z]+`)
	_, _ = re1, err1
	re2 := regexp.MustCompilePOSIX(`[0-9]+`)
	_ = re2

	// regexp.FindSubmatchIndex / FindAllSubmatchIndex.
	re3 := regexp.MustCompile(`(h\w+)`)
	_ = re3.FindSubmatchIndex([]byte("hello"))
	_ = re3.FindAllSubmatchIndex([]byte("hello world"), -1)

	// regexp.Expand / ExpandString.
	re4 := regexp.MustCompile(`(?P<name>\w+)`)
	src := []byte("hello")
	match := re4.FindSubmatchIndex(src)
	dst := re4.Expand([]byte{}, []byte("$name"), src, match)
	_ = dst
	dst2 := re4.ExpandString([]byte{}, "$name", "hello", match)
	_ = dst2

	// io.NewSectionReader + SectionReader.Size.
	r := strings.NewReader("hello world")
	sr := io.NewSectionReader(r, 0, int64(r.Len()))
	_ = sr.Size()

	// io.NewOffsetWriter.
	var buf strings.Builder
	ow := io.NewOffsetWriter(writerAt{&buf}, 0)
	_ = ow

	// path/filepath.IsLocal.
	_ = filepath.IsLocal("foo/bar")
	_ = filepath.IsLocal("/absolute")
}

// writerAt wraps strings.Builder to satisfy io.WriterAt.
type writerAt struct{ b *strings.Builder }

func (w writerAt) WriteAt(p []byte, off int64) (int, error) {
	return w.b.Write(p)
}
