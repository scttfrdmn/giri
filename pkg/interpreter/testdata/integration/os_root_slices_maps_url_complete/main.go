// os_root_slices_maps_url_complete exercises additions from v0.80.0:
// os.Root (Go 1.24): OpenRoot, Name, FS, Close, OpenRoot method;
// slices.Chunk (Go 1.23 iterator);
// net/url: Userinfo.Username and Userinfo.Password.
// Expected: 0 violations.
package main

import (
	"net/url"
	"os"
	"slices"
)

func main() {
	// os.Root (Go 1.24): sandboxed filesystem access.
	r, _ := os.OpenRoot(".")
	_ = r.Name()
	_ = r.FS()
	_ = r.Close()

	// (*Root).OpenRoot opens a sub-root within an existing root.
	sub, _ := r.OpenRoot("subdir")
	_ = sub

	// slices.Chunk (Go 1.23): returns iter.Seq[[]E].
	chunks := slices.Chunk([]int{1, 2, 3, 4, 5}, 2)
	_ = chunks

	// net/url Userinfo methods.
	info := url.User("alice")
	_ = info.Username()
	_, _ = info.Password()

	infoWithPw := url.UserPassword("bob", "secret")
	_ = infoWithPw.Username()
	_, _ = infoWithPw.Password()
}
