// template_execute verifies that text/template intercepts work (#84).
//
// Expected: 0 violations.
package main

import (
	"os"
	"text/template"
)

func main() {
	// template.New returns a *Template.
	t := template.New("greeting")
	_ = t

	// Parse populates the template body.
	t2, err := t.Parse("Hello, {{.Name}}!")
	_ = t2  // non-nil *Template
	_ = err // nil

	// Execute renders the template to a writer.
	err2 := t2.Execute(os.Stdout, map[string]string{"Name": "Giri"})
	_ = err2 // nil

	// template.Must panics if error is non-nil, returns template otherwise.
	t3 := template.Must(template.New("safe").Parse("static text"))
	_ = t3

	// Funcs registers custom functions (chainable).
	t4 := template.New("funcs").Funcs(template.FuncMap{
		"upper": func(s string) string { return s },
	})
	_ = t4
}
