package main

type Stringer interface{ String() string }
type Foo struct{}
type Bar struct{}

func (f *Foo) String() string { return "foo" }
func (b *Bar) String() string { return "bar" }
func makeStringer(n int) Stringer {
	if n%2 == 0 {
		return &Foo{}
	}
	return &Bar{}
}

func main() {
	s := makeStringer(3) // returns *Bar (3 is odd)
	f := s.(*Foo)        // panics: interface holds *Bar, not *Foo
	_ = f
}
