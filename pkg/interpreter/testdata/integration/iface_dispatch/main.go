package main

type Greeter interface{ Greet() string }
type English struct{}
type French struct{}

func (e *English) Greet() string { return "hello" }
func (f *French) Greet() string  { return "bonjour" }
func greet(g Greeter) string     { return g.Greet() }

func main() {
	var g Greeter = &English{}
	_ = greet(g)
}
