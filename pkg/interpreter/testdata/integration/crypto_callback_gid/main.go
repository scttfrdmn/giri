// crypto_callback_gid indexes out of range inside a callback that giri's
// intercepts run inline: crypto/subtle.WithDataIndependentTiming and
// crypto/fips140.WithoutEnforcement. Native Go panics in the first callback.
// Before #293 these callbacks ran on goroutine 0, which doesn't exist.
package main

import (
	"crypto/fips140"
	"crypto/subtle"
	"fmt"
)

func main() {
	xs := []int{1, 2}
	fips140.WithoutEnforcement(func() {
		fmt.Println(len(xs))
	})
	subtle.WithDataIndependentTiming(func() {
		i := 3
		fmt.Println(xs[i])
	})
}
