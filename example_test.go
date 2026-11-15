package template_test

import (
	"errors"
	"fmt"
	"io"
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

func Example_if() {
	t := template.Must(template.New("cart").Parse(
		"{{ if n == 0 }}The cart is empty.{{ else if n == 1 }}1 item{{ else }}{{ n }} items{{ end }}\n"))
	for _, n := range []int{0, 1, 5} {
		err := t.Execute(os.Stdout, map[string]any{"n": n})
		if err != nil {
			log.Fatal(err)
		}
	}
	// Output:
	// The cart is empty.
	// 1 item
	// 5 items
}

func Example_forMap() {
	t := template.Must(template.New("stock").Parse(
		"{{ for name, count in stock }}{{ name }}: {{ count }}\n{{ end }}"))
	err := t.Execute(os.Stdout, map[string]any{
		"stock": map[string]int{"tea": 12, "cake": 3, "milk": 7},
	})
	if err != nil {
		log.Fatal(err)
	}
	// Output:
	// cake: 3
	// milk: 7
	// tea: 12
}

func ExampleTemplate_Print() {
	print := func(v any) (string, error) {
		if v == nil {
			return "", errors.New("no value")
		}
		return fmt.Sprint(v), nil
	}

	t := template.Must(template.New("config").Print(print).Parse(
		"user = {{ db.user }}\npassword = {{ db.passwrd }}\n"))
	err := t.Execute(io.Discard, map[string]any{
		"db": map[string]any{"user": "app", "password": "secret"},
	})
	fmt.Println(err)
	// Output:
	// no value (config:2:12)
	//  | password = {{ db.passwrd }}
	//  | ...........^
}
