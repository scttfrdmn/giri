// ioutil_replacer verifies that io/ioutil intercepts and strings.NewReplacer
// (previously returning Value{},false) are correctly modeled.
//
// Expected: 0 violations.
package main

import (
	"io/ioutil"
	"strings"
)

func main() {
	// io/ioutil: ReadAll.
	data, err := ioutil.ReadAll(nil)
	_ = data
	_ = err

	// io/ioutil: ReadFile.
	contents, err2 := ioutil.ReadFile("/dev/null")
	_ = contents
	_ = err2

	// io/ioutil: WriteFile.
	err3 := ioutil.WriteFile("/dev/null", nil, 0644)
	_ = err3

	// io/ioutil: TempFile.
	f, err4 := ioutil.TempFile("", "giri-*")
	_ = f
	_ = err4

	// io/ioutil: TempDir.
	dir, err5 := ioutil.TempDir("", "giri-*")
	if len(dir) == 0 {
		var s []int
		_ = s[0] // canary: dir must be non-empty
	}
	_ = err5

	// io/ioutil: NopCloser.
	rc := ioutil.NopCloser(nil)
	_ = rc

	// strings.NewReplacer: previously returned Value{},false causing false positives.
	r := strings.NewReplacer("old", "new")
	if r == nil {
		var s []int
		_ = s[0] // canary: Replacer must be non-nil
	}
	result := r.Replace("hello old world")
	_ = result
}
