package main

func div(a, b int) (int, bool) {
	return a / b, b != 0
}

func main() {
	q, ok := div(10, 2)
	_, _ = q, ok
}
