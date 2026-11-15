package template

import (
	"errors"
	"io"
	"testing"

	"github.com/expr-lang/expr"
)

var parseErrorTests = []struct {
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
		"syntax error",
		"{{ n + }}",
		"unexpected token EOF (t:1:6)\n | {{ n + }}\n | .....^",
	},
	{
		"minus before the end of an action",
		"{{ n-}}",
		"unexpected token EOF (t:1:5)\n | {{ n-}}\n | ....^",
	},
	{
		"closing braces with a space between them",
		"{{ n } }}",
		"unexpected token Bracket(\"}\") (t:1:6)\n | {{ n } }}\n | .....^",
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
		"unclosed comment",
		"a {{ /* c }}",
		"unclosed comment (t:1:3)\n | a {{ /* c }}\n | ..^",
	},
	{
		"line comment before the end of the action",
		"a {{ // c }}",
		"unclosed action (t:1:3)\n | a {{ // c }}\n | ..^",
	},
	{
		"end without if",
		"a {{ end }}",
		"unexpected {{end}} (t:1:3)\n | a {{ end }}\n | ..^",
	},
	{
		"second end",
		"{{ if true }}a{{ end }}{{ end }}",
		"unexpected {{end}} (t:1:24)\n | {{ if true }}a{{ end }}{{ end }}\n | .......................^",
	},
	{
		"else without if",
		"a {{ else }}",
		"unexpected {{else}} (t:1:3)\n | a {{ else }}\n | ..^",
	},
	{
		"else after else",
		"{{ if true }}a{{ else }}b{{ else }}c{{ end }}",
		"unexpected {{else}} (t:1:26)\n | {{ if true }}a{{ else }}b{{ else }}c{{ end }}\n | .........................^",
	},
	{
		"else with an expression",
		"{{ if true }}a{{ else n }}b{{ end }}",
		"unexpected token Identifier(\"n\") (t:1:23)\n | {{ if true }}a{{ else n }}b{{ end }}\n | ......................^",
	},
	{
		"else if without condition",
		"{{ if true }}a{{ else if }}b{{ end }}",
		"missing condition (t:1:15)\n | {{ if true }}a{{ else if }}b{{ end }}\n | ..............^",
	},
	{
		"else without end",
		"a {{ if true }}b{{ else }}c",
		"unclosed if (t:1:17)\n | a {{ if true }}b{{ else }}c\n | ................^",
	},
	{
		"else if without end",
		"a {{ if true }}b{{ else if true }}c",
		"unclosed if (t:1:17)\n | a {{ if true }}b{{ else if true }}c\n | ................^",
	},
	{
		"condition of else if is not a boolean",
		"{{ if true }}a{{ else if n }}b{{ end }}",
		"non-bool expression (type int) used as condition (t:1:26)\n | {{ if true }}a{{ else if n }}b{{ end }}\n | .........................^",
	},
	{
		"error in a condition of else if after multi-byte characters",
		"{{ if true }}é{{ /* é */ else /* é */ if nn }}b{{ end }}",
		"unknown name nn (t:1:42)\n | {{ if true }}é{{ /* é */ else /* é */ if nn }}b{{ end }}\n | .........................................^",
	},
	{
		"error in an else",
		"{{ if true }}a{{ else }}\n{{ nn }}{{ end }}",
		"unknown name nn (t:2:4)\n | {{ nn }}{{ end }}\n | ...^",
	},
	{
		"if without end",
		"a {{ if true }} b",
		"unclosed if (t:1:3)\n | a {{ if true }} b\n | ..^",
	},
	{
		"inner if without end",
		"{{ if true }}\n{{ if false }}{{ end }}",
		"unclosed if (t:1:1)\n | {{ if true }}\n | ^",
	},
	{
		"if without condition",
		"a {{ if }}b{{ end }}",
		"missing condition (t:1:3)\n | a {{ if }}b{{ end }}\n | ..^",
	},
	{
		"end with an expression",
		"{{ if true }}{{ end + 1 }}",
		"unexpected token Operator(\"+\") (t:1:21)\n | {{ if true }}{{ end + 1 }}\n | ....................^",
	},
	{
		"end with an expression and without if",
		"a {{ end + 1 }}",
		"unexpected token Operator(\"+\") (t:1:10)\n | a {{ end + 1 }}\n | .........^",
	},
	{
		"token after end after multi-byte characters",
		"{{ if true }}{{ /* ééé */ end x }}",
		"unexpected token Identifier(\"x\") (t:1:31)\n | {{ if true }}{{ /* ééé */ end x }}\n | ..............................^",
	},
	{
		"error in a condition after multi-byte characters",
		"{{ /* ééé */ if nn }}",
		"unknown name nn (t:1:17)\n | {{ /* ééé */ if nn }}\n | ................^",
	},
	{
		"condition of a named boolean type",
		"a {{ if mb }}b{{ end }}",
		"non-bool expression (type template.myBool) used as condition (t:1:9)\n | a {{ if mb }}b{{ end }}\n | ........^",
	},
	{
		"condition is a pointer to a boolean",
		"a {{ if p }}b{{ end }}",
		"non-bool expression (type *bool) used as condition (t:1:9)\n | a {{ if p }}b{{ end }}\n | ........^",
	},
	{
		"condition is not a boolean",
		"a {{ if n }}b{{ end }}",
		"non-bool expression (type int) used as condition (t:1:9)\n | a {{ if n }}b{{ end }}\n | ........^",
	},
	{
		"condition is not a boolean after a trim marker",
		"a {{- if  s -}} b{{ end }}",
		"non-bool expression (type string) used as condition (t:1:11)\n | a {{- if  s -}} b{{ end }}\n | ..........^",
	},
	{
		"error in a condition",
		"a {{ if nn }}b{{ end }}",
		"unknown name nn (t:1:9)\n | a {{ if nn }}b{{ end }}\n | ........^",
	},
	{
		"error in the body of an if",
		"{{ if true }}\n{{ nn }}{{ end }}",
		"unknown name nn (t:2:4)\n | {{ nn }}{{ end }}\n | ...^",
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
		"empty action with a no-break space",
		"a {{\u00a0}}",
		"empty action (t:1:3)\n | a {{\u00a0}}\n | ..^",
	},
	{
		"empty action with trim markers",
		"a {{- -}} b",
		"empty action (t:1:3)\n | a {{- -}} b\n | ..^",
	},
}

func TestParseError(t *testing.T) {
	for _, tt := range parseErrorTests {
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

var executeErrorTests = []struct {
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
		"in a condition",
		"a {{ if l[5] == 1 }}b{{ end }}",
		"index out of range: 5 (array length is 2) (t:1:10)\n | a {{ if l[5] == 1 }}b{{ end }}\n | .........^",
	},
	{
		"in the body of an if",
		"{{ if true }}\n{{ l[5] }}{{ end }}",
		"index out of range: 5 (array length is 2) (t:2:5)\n | {{ l[5] }}{{ end }}\n | ....^",
	},
	{
		"in a condition of else if",
		"{{ if false }}a{{ else if l[5] == 1 }}b{{ end }}",
		"index out of range: 5 (array length is 2) (t:1:28)\n | {{ if false }}a{{ else if l[5] == 1 }}b{{ end }}\n | ...........................^",
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

func TestExecuteError(t *testing.T) {
	for _, tt := range executeErrorTests {
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

func TestExecuteConditionError(t *testing.T) {
	tests := []struct {
		name string
		text string
		data map[string]any
		want string
	}{
		{
			"int",
			"x {{ if a }}b{{ end }}",
			map[string]any{"a": 1},
			"non-bool expression (type int) used as condition (t:1:9)\n | x {{ if a }}b{{ end }}\n | ........^",
		},
		{
			"nil",
			"x {{ if a }}b{{ end }}",
			map[string]any{},
			"non-bool expression (type nil) used as condition (t:1:9)\n | x {{ if a }}b{{ end }}\n | ........^",
		},
		{
			"int in else if",
			"x {{ if false }}b{{ else if a }}c{{ end }}",
			map[string]any{"a": 1},
			"non-bool expression (type int) used as condition (t:1:29)\n | x {{ if false }}b{{ else if a }}c{{ end }}\n | ............................^",
		},
		{
			"error without position in else if",
			"x {{ if false }}b{{ else if (if a { true } else { false }) }}c{{ end }}",
			map[string]any{"a": 1},
			"interface conversion: interface {} is int, not bool (t:1:18)\n | x {{ if false }}b{{ else if (if a { true } else { false }) }}c{{ end }}\n | .................^",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := execute(t, tt.text, tt.data)
			if err == nil {
				t.Fatal("no error")
			}
			if err.Error() != tt.want {
				t.Errorf("got\n%s\nwant\n%s", err, tt.want)
			}
			if want := "x "; got != want {
				t.Errorf("got output %q, want %q", got, want)
			}
		})
	}
}

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
	for _, text := range []string{"{{ nn }}", "{{ n", "{{ boom() }}"} {
		_, err := execute(t, text, testData, expr.Env(env{}), boomFunc)
		if err == nil {
			t.Fatalf("%q: no error", text)
		}
		if got := errors.Unwrap(err); got != nil {
			t.Errorf("%q: got %v, want nil", text, got)
		}
	}
}
