package template

import (
	"errors"
	"io"
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

func TestExecuteError(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			"index out of range",
			"a\n{{ l[5] }}",
			"index out of range: 5 (array length is 2) (t:2:5)\n | {{ l[5] }}\n | ....^",
		},
		{
			"second action",
			"{{ n }} {{ l[5] }}",
			"index out of range: 5 (array length is 2) (t:1:13)\n | {{ n }} {{ l[5] }}\n | ............^",
		},
		{
			"after a trim marker",
			"a {{- l[5] -}} b",
			"index out of range: 5 (array length is 2) (t:1:8)\n | a {{- l[5] -}} b\n | .......^",
		},
		{
			"after multi-byte characters",
			`é→ {{ "日" + string(l[5]) }}`,
			"index out of range: 5 (array length is 2) (t:1:21)\n | é→ {{ \"日\" + string(l[5]) }}\n | ....................^",
		},
		{
			"error of a function",
			"a {{ fail() }}",
			"failed (t:1:6)\n | a {{ fail() }}\n | .....^",
		},
		{
			"panic with a string",
			"a {{ boom() }}",
			"boom (t:1:6)\n | a {{ boom() }}\n | .....^",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := execute(t, tt.text, testData, expr.Env(env{}), failFunc, boomFunc)
			if err == nil {
				t.Fatal("no error")
			}
			if err.Error() != tt.want {
				t.Errorf("got\n%s\nwant\n%s", err, tt.want)
			}
		})
	}
}

var errFail = errors.New("failed")

var failFunc = expr.Function("fail", func(...any) (any, error) {
	return nil, errFail
}, new(func() string))

var boomFunc = expr.Function("boom", func(...any) (any, error) {
	panic("boom")
}, new(func() string))

func TestExecuteErrorWithoutPosition(t *testing.T) {
	_, err := execute(t, "ab {{   a }}", map[string]any{"a": "s"}, expr.AsBool())
	if err == nil {
		t.Fatal("no error")
	}
	want := "invalid operation: bool(string) (t:1:4)\n | ab {{   a }}\n | ...^"
	if err.Error() != want {
		t.Errorf("got\n%s\nwant\n%s", err, want)
	}
}

func TestExecuteErrorAfterSecondParse(t *testing.T) {
	tmpl := New("t", expr.Env(env{}))
	Must(tmpl.Parse("first {{ n }}"))
	Must(tmpl.Parse("b\n{{ l[5] }}"))
	err := tmpl.Execute(io.Discard, testData)
	if err == nil {
		t.Fatal("no error")
	}
	want := "index out of range: 5 (array length is 2) (t:2:5)\n | {{ l[5] }}\n | ....^"
	if err.Error() != want {
		t.Errorf("got\n%s\nwant\n%s", err, want)
	}
}

func TestExecuteErrorCause(t *testing.T) {
	_, err := execute(t, "{{ fail() }}", testData, expr.Env(env{}), failFunc)
	if !errors.Is(err, errFail) {
		t.Errorf("got %v, want an error that wraps errFail", err)
	}
	e, ok := errors.AsType[*Error](err)
	if !ok {
		t.Fatalf("got %T, want *Error", err)
	}
	if e.Unwrap() != errFail {
		t.Errorf("got cause %v, want errFail", e.Unwrap())
	}
}

func TestExecuteErrorWithoutCause(t *testing.T) {
	_, err := execute(t, "{{ boom() }}", testData, expr.Env(env{}), boomFunc)
	if err == nil {
		t.Fatal("no error")
	}
	if got := errors.Unwrap(err); got != nil {
		t.Errorf("got cause %v, want nil", got)
	}
}

func TestExecuteErrorKeepsOutput(t *testing.T) {
	got, err := execute(t, "a {{ n }} b {{ l[5] }} c", testData, expr.Env(env{}))
	if err == nil {
		t.Fatal("no error")
	}
	if want := "a 1 b "; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

type failWriter struct {
	n int
}

var errWrite = errors.New("write failed")

func (w *failWriter) Write(p []byte) (int, error) {
	if w.n == 0 {
		return 0, errWrite
	}
	w.n--
	return len(p), nil
}

func TestExecuteWriteError(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"text", "{{ n }} a"},
		{"action", "a {{ n }}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := Must(New("t", expr.Env(env{})).Parse(tt.text))
			if err := tmpl.Execute(&failWriter{n: 1}, testData); err != errWrite {
				t.Errorf("got %v, want errWrite", err)
			}
		})
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
