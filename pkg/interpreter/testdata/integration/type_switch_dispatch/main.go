package main

type Animal interface{ Sound() string }
type Dog struct{}
type Cat struct{}
type Bird struct{}

func (d *Dog) Sound() string  { return "woof" }
func (c *Cat) Sound() string  { return "meow" }
func (b *Bird) Sound() string { return "tweet" }

func makeAnimal(kind string) Animal {
	switch kind {
	case "dog":
		return &Dog{}
	case "cat":
		return &Cat{}
	default:
		return &Bird{}
	}
}

func dispatch(a Animal) {
	switch a.(type) {
	case *Dog:
	case *Cat:
	case *Bird:
	}
}

func main() {
	dispatch(makeAnimal("dog"))
	dispatch(makeAnimal("cat"))
	dispatch(makeAnimal("bird"))
}
