package main

type Stringer interface{ String() string }
type Foo struct{}

func (f *Foo) String() string { return "foo" }
func makeStringer() Stringer  { return &Foo{} }

func main() {
	s := makeStringer()
	f := s.(*Foo) // succeeds: interface holds *Foo
	_ = f.String()
}
