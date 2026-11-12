package template_test

import (
	"log"
	"os"

	"github.com/expr-lang/expr"

	template "github.com/davidklassen/expr-template"
)

func Example() {
	t := template.Must(template.New("hello").Parse("Hello, {{ name }}!\n"))
	err := t.Execute(os.Stdout, map[string]any{"name": "Alice"})
	if err != nil {
		log.Fatal(err)
	}
	// Output:
	// Hello, Alice!
}

func ExampleNew() {
	type Env struct {
		Items []string `expr:"items"`
		Price float64  `expr:"price"`
	}

	// With expr.Env, Parse checks the names and types in the template.
	t, err := template.New("order", expr.Env(Env{})).Parse(
		"{{ len(items) }} items, total {{ price * len(items) }}\n")
	if err != nil {
		log.Fatal(err)
	}
	err = t.Execute(os.Stdout, Env{Items: []string{"a", "b"}, Price: 1.5})
	if err != nil {
		log.Fatal(err)
	}
	// Output:
	// 2 items, total 3
}
