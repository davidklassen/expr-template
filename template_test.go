package template

import (
	"io"
	"strings"
	"testing"

	"github.com/expr-lang/expr"
)

type env struct {
	N int    `expr:"n"`
	S string `expr:"s"`
	L []int  `expr:"l"`
}

var testData = env{
	N: 1,
	S: "str",
	L: []int{1, 2},
}

func execute(t *testing.T, text string, data any, opts ...expr.Option) (string, error) {
	t.Helper()
	tmpl, err := New("t", opts...).Parse(text)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	err = tmpl.Execute(&b, data)
	return b.String(), err
}

func TestExecute(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"empty", "", ""},
		{"text", "plain text\n", "plain text\n"},
		{"unicode text", "é→ ✓", "é→ ✓"},
		{"braces in text", "a { b } c }} d", "a { b } c }} d"},

		{"int", "{{ n }}", "1"},
		{"string", "{{ s }}", "str"},
		{"float", "{{ 1.5 }}", "1.5"},
		{"bool", "{{ true }}", "true"},
		{"nil", "{{ nil }}", "<nil>"},
		{"list", "{{ l }}", "[1 2]"},
		{"arithmetic", "{{ n + 1 }}", "2"},
		{"builtin", "{{ upper(s) }}", "STR"},
		{"text around actions", "a {{ n }} b {{ s }} c", "a 1 b str c"},
		{"adjacent actions", "{{ n }}{{ s }}", "1str"},
		{"no spaces", "{{n}}", "1"},
		{"action across lines", "{{ n +\n 1 }}", "2"},

		{"string with braces", `{{ "a}}b" }}`, "a}}b"},
		{"single-quoted string with braces", `{{ 'a}}b' }}`, "a}}b"},
		{"raw string with braces", "{{ `a}}b` }}", "a}}b"},
		{"escaped quote", `{{ "a\"}}b" }}`, `a"}}b`},
		{"map literal", `{{ {"a": {"b": 1}}.a.b }}`, "1"},
		{"comment with braces", "{{ n /* }} */ }}", "1"},
		{"left delimiter in string", `{{ "{{" }}`, "{{"},
		{"unicode in action", `{{ "é→" }} ✓`, "é→ ✓"},

		{"trim", "a  {{- n -}}  b", "a1b"},
		{"trim newlines", "a\n\n{{- n -}}\n\nb", "a1b"},
		{"trim tabs", "a\t{{- n -}}\r\n\tb", "a1b"},
		{"trim left only", "a  {{- n }}  b", "a1  b"},
		{"trim right only", "a  {{ n -}}  b", "a  1b"},
		{"trim at the ends", " {{- n -}} ", "1"},
		{"trim text between actions", "{{ n -}} \n {{- s }}", "1str"},
		{"trim after comment", "{{ n /* c */ -}} b", "1b"},
		{"trim keeps minus in text", "hello- {{- 3 -}} -world", "hello-3-world"},
		{"minus without space", "{{-3}}", "-3"},
		{"minus before braces", "{{ 5 -3}}", "2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := execute(t, tt.text, testData, expr.Env(env{}))
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got  %q\nwant %q", got, tt.want)
			}
		})
	}
}

func TestExecuteWithoutOptions(t *testing.T) {
	got, err := execute(t, "{{ a }}-{{ b }}", map[string]any{"a": 1, "b": "x"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "1-x"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExecuteError(t *testing.T) {
	_, err := execute(t, "a\n{{ l[5] }}", testData, expr.Env(env{}))
	if err == nil {
		t.Fatal("no error")
	}
	want := "index out of range: 5 (array length is 2) (1:3)\n |  l[5] \n | ..^"
	if err.Error() != want {
		t.Errorf("got\n%s\nwant\n%s", err, want)
	}
}

func TestExecuteBeforeParse(t *testing.T) {
	err := New("t").Execute(io.Discard, nil)
	if err == nil {
		t.Fatal("no error")
	}
	if want := `template "t" is not parsed`; err.Error() != want {
		t.Errorf("got %q, want %q", err, want)
	}
}

func TestNewCopiesOptions(t *testing.T) {
	opts := []expr.Option{expr.Env(env{})}
	tmpl := New("t", opts...)
	opts[0] = expr.Env(struct{}{})
	if _, err := tmpl.Parse("{{ n }}"); err != nil {
		t.Errorf("Parse used an option changed after New: %v", err)
	}
}

func TestName(t *testing.T) {
	if got := New("t").Name(); got != "t" {
		t.Errorf("got %q, want t", got)
	}
}

func TestMust(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Must did not panic")
		}
	}()
	Must(New("t").Parse("{{ n"))
}
