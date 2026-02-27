// errors_new verifies that errors.New, errors.Is, errors.As, and errors.Unwrap
// are intercepted cleanly and return correct values (#67).
//
// Expected: 0 violations.
package main

import "errors"

var ErrNotFound = errors.New("not found")

func findItem(id int) error {
	if id < 0 {
		return errors.New("invalid id")
	}
	if id == 0 {
		return ErrNotFound
	}
	return nil
}

func main() {
	err := findItem(-1)
	if err == nil {
		panic("expected error")
	}

	err2 := findItem(0)
	_ = errors.Is(err2, ErrNotFound)
	_ = errors.As(err2, &err)
	_ = errors.Unwrap(err2)

	err3 := findItem(1)
	if err3 != nil {
		panic("unexpected error for valid id")
	}
}
