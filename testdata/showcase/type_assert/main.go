// This program compiles cleanly, passes go vet, and passes go test -race.
// Giri detects the runtime type assertion panic that go vet cannot see.
package main

type Animal interface {
	Sound() string
}

type Dog struct{}
type Cat struct{}

func (d *Dog) Sound() string { return "woof" }
func (c *Cat) Sound() string { return "meow" }

// makeAnimal returns different concrete types depending on the argument.
func makeAnimal(kind string) Animal {
	if kind == "dog" {
		return &Dog{}
	}
	return &Cat{}
}

func main() {
	// kind="cat" → concrete type is *Cat
	a := makeAnimal("cat")
	// No comma-ok: this panics at runtime because interface holds *Cat, not *Dog.
	// go vet cannot determine the return type of makeAnimal("cat") statically.
	// Giri interprets the code, follows the "cat" branch, and catches the panic.
	d := a.(*Dog)
	_ = d.Sound()
}
