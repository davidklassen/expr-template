package template_test

import (
	"errors"
	"fmt"
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

func ExampleError() {
	type Env struct {
		Name string `expr:"name"`
	}

	_, err := template.New("hello", expr.Env(Env{})).Parse("Hello, {{ nmae }}!\n")
	fmt.Println(err)

	if e, ok := errors.AsType[*template.Error](err); ok {
		fmt.Println(e.Name, e.Line, e.Column)
	}
	// Output:
	// unknown name nmae (hello:1:11)
	//  | Hello, {{ nmae }}!
	//  | ..........^
	// hello 1 11
}

func Example_for() {
	type Item struct {
		Name  string  `expr:"name"`
		Price float64 `expr:"price"`
	}
	type Env struct {
		Items []Item `expr:"items"`
	}

	t := template.Must(template.New("order", expr.Env(Env{})).Parse(
		"{{ for item in items }}{{ item.name }}: {{ item.price }}\n{{ end }}"))
	err := t.Execute(os.Stdout, Env{Items: []Item{{"tea", 2.5}, {"cake", 4}}})
	if err != nil {
		log.Fatal(err)
	}
	// Output:
	// tea: 2.5
	// cake: 4
}
