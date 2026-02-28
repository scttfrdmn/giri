// flag_parse verifies that flag package intercepts work (#88).
//
// Expected: 0 violations.
package main

import "flag"

func main() {
	// Define flags of each supported type.
	name := flag.String("name", "world", "greeting target")
	count := flag.Int("count", 1, "repetition count")
	verbose := flag.Bool("verbose", false, "enable verbose output")
	ratio := flag.Float64("ratio", 1.0, "scaling ratio")

	// Parse reads os.Args (noop in the interpreter).
	flag.Parse()

	// Dereference the flag values.
	_ = *name    // "world"
	_ = *count   // 1
	_ = *verbose // false
	_ = *ratio   // 1.0

	// Introspection.
	_ = flag.NArg()  // 0
	_ = flag.NFlag() // 0
	_ = flag.Arg(0)  // ""
	_ = flag.Args()  // []

	// Parsed reports whether Parse has been called.
	_ = flag.Parsed() // true

	// Lookup finds a defined flag by name.
	f := flag.Lookup("name")
	_ = f // nil (conservative) or *flag.Flag
}
