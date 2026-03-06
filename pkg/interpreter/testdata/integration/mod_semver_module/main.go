// mod_semver_module verifies that golang.org/x/mod/semver and
// golang.org/x/mod/module calls are correctly intercepted.
//
// Expected: 0 violations.
package main

import (
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

func main() {
	// semver: IsValid.
	ok := semver.IsValid("v1.2.3")
	_ = ok
	bad := semver.IsValid("not-semver")
	_ = bad

	// semver: Compare.
	cmp := semver.Compare("v1.0.0", "v2.0.0")
	_ = cmp

	// semver: Canonical / Major.
	canon := semver.Canonical("v1.2.3")
	_ = canon
	maj := semver.Major("v1.2.3")
	_ = maj

	// semver: Max.
	mx := semver.Max("v1.0.0", "v2.0.0")
	_ = mx

	// semver: Sort.
	versions := []string{"v1.2.3", "v0.1.0", "v2.0.0"}
	semver.Sort(versions)

	// module: CheckPath.
	err := module.CheckPath("example.com/foo")
	_ = err

	// module: CheckImportPath.
	err2 := module.CheckImportPath("example.com/foo/bar")
	_ = err2

	// module: EscapePath.
	esc, err3 := module.EscapePath("example.com/Foo")
	_ = esc
	_ = err3

	// module: CanonicalVersion.
	cv := module.CanonicalVersion("v1.0.0")
	_ = cv

	// module: IsPseudoVersion.
	isPseudo := module.IsPseudoVersion("v0.0.0-20190101120000-abcdefabcdef")
	_ = isPseudo
}
