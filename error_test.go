package template

import (
	"errors"
	"testing"

	"github.com/expr-lang/expr"
)

func TestParseError(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			"unknown name",
			"a {{ nn }}",
			"unknown name nn (t:1:6)\n | a {{ nn }}\n | .....^",
		},
		{
			"unknown name on second line",
			"a\n{{ nn }}",
			"unknown name nn (t:2:4)\n | {{ nn }}\n | ...^",
		},
		{
			"second line of an action",
			"{{ n +\n nn }}",
			"unknown name nn (t:2:2)\n |  nn }}\n | .^",
		},
		{
			"second action",
			"{{ n }} {{ nn }}",
			"unknown name nn (t:1:12)\n | {{ n }} {{ nn }}\n | ...........^",
		},
		{
			"column after multi-byte characters",
			"é→ {{ nn }}",
			"unknown name nn (t:1:7)\n | é→ {{ nn }}\n | ......^",
		},
		{
			"multi-byte characters in an action",
			`{{ "é→" + nn }}`,
			"unknown name nn (t:1:11)\n | {{ \"é→\" + nn }}\n | ..........^",
		},
		{
			"after a trim marker",
			"a {{- nn -}} b",
			"unknown name nn (t:1:7)\n | a {{- nn -}} b\n | ......^",
		},
		{
			"mismatched types",
			`{{ n + "a" }}`,
			"invalid operation: + (mismatched types int and string) (t:1:6)\n | {{ n + \"a\" }}\n | .....^",
		},
		{
			"syntax error",
			"{{ n + }}",
			"unexpected token EOF (t:1:7)\n | {{ n + }}\n | ......^",
		},
		{
			"lexer error",
			"a {{ n ~ 1 }}",
			"unrecognized character: U+007E '~' (t:1:3)\n | a {{ n ~ 1 }}\n | ..^",
		},
		{
			"lexer error in the text after an unclosed action",
			"Hello {{ n }\nDon't forget.\n",
			"literal not terminated (t:1:7)\n | Hello {{ n }\n | ......^",
		},
		{
			"unclosed action",
			"a {{ n",
			"unclosed action (t:1:3)\n | a {{ n\n | ..^",
		},
		{
			"unclosed action on second line",
			"a\nb {{ n }",
			"unclosed action (t:2:3)\n | b {{ n }\n | ..^",
		},
		{
			"unclosed action after multi-byte characters",
			"é {{",
			"unclosed action (t:1:3)\n | é {{\n | ..^",
		},
		{
			"empty action",
			"a {{ }}",
			"empty action (t:1:3)\n | a {{ }}\n | ..^",
		},
		{
			"empty action without spaces",
			"{{}}",
			"empty action (t:1:1)\n | {{}}\n | ^",
		},
		{
			"empty action with a newline",
			"{{\n}}",
			"empty action (t:1:1)\n | {{\n | ^",
		},
		{
			"empty action after multi-byte characters",
			"é\n日 {{ }}",
			"empty action (t:2:3)\n | 日 {{ }}\n | ..^",
		},
		{
			"empty action with trim markers",
			"a {{- -}} b",
			"empty action (t:1:3)\n | a {{- -}} b\n | ..^",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New("t", expr.Env(env{})).Parse(tt.text)
			if err == nil {
				t.Fatal("no error")
			}
			if err.Error() != tt.want {
				t.Errorf("got\n%s\nwant\n%s", err, tt.want)
			}
		})
	}
}

func TestParseErrorWithoutPosition(t *testing.T) {
	_, err := New("t", expr.Env(env{}), expr.AsBool()).Parse("x {{ n }}")
	if err == nil {
		t.Fatal("no error")
	}
	want := "expected bool, but got int (t:1:3)\n | x {{ n }}\n | ..^"
	if err.Error() != want {
		t.Errorf("got\n%s\nwant\n%s", err, want)
	}
}

func TestErrorFields(t *testing.T) {
	_, err := New("name", expr.Env(env{})).Parse("a\n {{ nn }}")
	e, ok := errors.AsType[*Error](err)
	if !ok {
		t.Fatalf("got %T, want *Error", err)
	}
	want := Error{
		Name:    "name",
		Line:    2,
		Column:  5,
		Message: "unknown name nn",
		snippet: "\n |  {{ nn }}\n | ....^",
	}
	if *e != want {
		t.Errorf("got  %+v\nwant %+v", *e, want)
	}
}

func TestErrorUnwrap(t *testing.T) {
	for _, text := range []string{"{{ nn }}", "{{ n"} {
		_, err := New("t", expr.Env(env{})).Parse(text)
		if err == nil {
			t.Fatalf("%q: no error", text)
		}
		if got := errors.Unwrap(err); got != nil {
			t.Errorf("%q: got %v, want nil", text, got)
		}
	}
}
