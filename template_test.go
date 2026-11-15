package template

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"strings"
	"testing"

	"github.com/expr-lang/expr"
)

type myBool bool

type env struct {
	N  int    `expr:"n"`
	S  string `expr:"s"`
	L  []int  `expr:"l"`
	MB myBool `expr:"mb"`
	P  *bool  `expr:"p"`
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

var executeTests = []struct {
	name string
	text string
	want string
}{
	{"empty", "", ""},
	{"text", "plain text\n", "plain text\n"},
	{"braces in text", "a { b } c }} d", "a { b } c }} d"},

	{"int", "{{ n }}", "1"},
	{"string", "{{ s }}", "str"},
	{"nil", "{{ nil }}", "<nil>"},
	{"list", "{{ l }}", "[1 2]"},
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
	{"output is not escaped", `{{ "<b>&" }}`, "<b>&"},

	{"trim", "a  {{- n -}}  b", "a1b"},
	{"trim tabs", "a\t{{- n -}}\r\n\tb", "a1b"},
	{"trim left only", "a  {{- n }}  b", "a1  b"},
	{"left trim keeps white space before the text", " a {{- n }}", " a1"},
	{"trim right only", "a  {{ n -}}  b", "a  1b"},
	{"trim at the ends", " {{- n -}} ", "1"},
	{"trim text between actions", "{{ n -}} \n {{- s }}", "1str"},
	{"trim unicode white space", "a\u00a0\f{{- n -}}\u2003\vb", "a1b"},
	{"trim marker with unicode white space", "a {{-\u00a0n\u00a0-}} b", "a1b"},
	{"minus without space", "{{-3}}", "-3"},

	{"comment", "a{{/* c */}}b", "ab"},
	{"line comment", "a{{ // c\n}}b", "ab"},
	{"trim around comment", "a {{- /* c */ -}} b", "ab"},
	{"comment only", "{{/* c */}}", ""},
	{"comment before expression", "a{{ /* c */ n }}b", "a1b"},

	{"if true", "a{{ if n == 1 }}b{{ end }}c", "abc"},
	{"if false", "a{{ if n == 2 }}b{{ end }}c", "ac"},
	{"if with actions", "{{ if true }}{{ n }}-{{ s }}{{ end }}", "1-str"},
	{"if with empty body", "a{{ if true }}{{ end }}b", "ab"},
	{"if with comment", "{{ if /* c */ true }}a{{ end /* c */ }}", "a"},
	{"nested if", "{{ if true }}a{{ if false }}b{{ end }}c{{ end }}d", "acd"},
	{"nested if in false", "{{ if false }}a{{ if true }}b{{ end }}c{{ end }}d", "d"},
	{"trim in if", "a {{- if true -}} b {{- end -}} c", "abc"},
	{"if with parentheses", "{{ if(n == 1) }}a{{ end }}", "a"},
	{"if expression of expr", "{{ (if n == 1 { 2 } else { 3 }) }}", "2"},
	{"false if does not run its body", "a{{ if false }}{{ l[5] }}{{ end }}b", "ab"},
	{"if compares with nil", "{{ if p != nil }}a{{ else }}b{{ end }}", "b"},

	{"else of a true if", "{{ if n == 1 }}a{{ else }}b{{ end }}c", "ac"},
	{"else of a false if", "{{ if n == 2 }}a{{ else }}b{{ end }}c", "bc"},
	{"else with actions", "{{ if false }}{{ n }}{{ else }}{{ s }}{{ end }}", "str"},
	{"else with comment", "{{ if false }}a{{ else /* c */ }}b{{ end }}", "b"},
	{"empty else", "a{{ if false }}b{{ else }}{{ end }}c", "ac"},
	{"trim around else", "{{ if false }} a {{- else -}} b {{- end }}", "b"},
	{"else if, first is true", "{{ if n == 1 }}a{{ else if n == 2 }}b{{ else }}c{{ end }}", "a"},
	{"else if, second is true", "{{ if n == 2 }}a{{ else if n == 1 }}b{{ else }}c{{ end }}", "b"},
	{"else if, none is true", "{{ if n == 2 }}a{{ else if n == 3 }}b{{ else }}c{{ end }}", "c"},
	{"else if without else", "x{{ if n == 2 }}a{{ else if n == 3 }}b{{ end }}y", "xy"},
	{"three else if", "{{ if n == 4 }}a{{ else if n == 3 }}b{{ else if n == 2 }}c{{ else if n == 1 }}d{{ end }}", "d"},
	{"if in else", "{{ if false }}a{{ else }}{{ if true }}b{{ end }}c{{ end }}", "bc"},
	{"if with else in if", "{{ if true }}{{ if false }}a{{ else }}b{{ end }}{{ else }}c{{ end }}", "b"},
	{"true if does not evaluate else if", "{{ if true }}a{{ else if l[5] == 1 }}b{{ end }}", "a"},
}

func TestExecute(t *testing.T) {
	for _, tt := range executeTests {
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

func TestKeywordAsName(t *testing.T) {
	data := map[string]any{"end": 1, "for": 6, "ending": 2, "iffy": 3, "t": true, "If": 4, "IF": 5}
	tests := []struct {
		name string
		text string
		want string
	}{
		{"in parentheses", "{{ (end) }}{{ (for) }}", "16"},
		{"in a string", `{{ "if" }}{{ "end" }}{{ "for" }}`, "ifendfor"},
		{"not the first word", "{{ 1 + end }}", "2"},
		{"in a condition", "{{ if end == 1 }}a{{ end }}", "a"},
		{"prefix of a name", "{{ ending }}{{ iffy }}", "23"},
		{"in another case", "{{ If }}{{ IF }}{{ if t }}{{ END }}{{ end }}", "45<nil>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := execute(t, tt.text, data)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got  %q\nwant %q", got, tt.want)
			}
		})
	}
}

func TestConditionIgnoresResultType(t *testing.T) {
	data := map[string]any{"a": true, "n": 1}
	got, err := execute(t, "{{ if a }}x{{ end }}{{ n }}", data, expr.AsInt())
	if err != nil {
		t.Fatal(err)
	}
	if want := "x1"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	_, err = execute(t, "x {{ if b }}y{{ end }}", data, expr.AsBool())
	if err == nil {
		t.Fatal("no error")
	}
	want := "non-bool expression (type nil) used as condition (t:1:9)\n | x {{ if b }}y{{ end }}\n | ........^"
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

func TestFailedParse(t *testing.T) {
	tmpl := New("t", expr.Env(env{}))
	if got, err := tmpl.Parse("{{"); got != nil || err == nil {
		t.Fatalf("got %v, %v, want nil and an error", got, err)
	}
	err := tmpl.Execute(io.Discard, testData)
	if want := `template "t" is not parsed`; err == nil || err.Error() != want {
		t.Errorf("got %v, want %s", err, want)
	}

	Must(tmpl.Parse("a\n{{ l[5] }}"))
	if _, err := tmpl.Parse("{{"); err == nil {
		t.Fatal("no error")
	}
	err = tmpl.Execute(io.Discard, testData)
	want := "index out of range: 5 (array length is 2) (t:2:5)\n | {{ l[5] }}\n | ....^"
	if err == nil || err.Error() != want {
		t.Errorf("got\n%v\nwant\n%s", err, want)
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

func FuzzParse(f *testing.F) {
	for _, tt := range executeTests {
		f.Add(tt.text)
	}
	for _, tt := range loopTests {
		f.Add(tt.text)
	}
	for _, tt := range parseErrorTests {
		f.Add(tt.text)
	}
	for _, tt := range executeErrorTests {
		f.Add(tt.text)
	}
	for _, tt := range loopParseErrorTests {
		f.Add(tt.text)
	}
	data := newLoopMap()
	maps.Copy(data, map[string]any{"n": 1, "s": "str", "l": []int{1, 2}})
	f.Fuzz(func(t *testing.T, text string) {
		fuzzExecute(t, text, testData, expr.Env(env{}))
		fuzzExecute(t, text, data)
		if tmpl, err := New("t").Parse(text); err == nil {
			checkLocations(t, []rune(text), tmpl.Nodes())
		}
	})
}

// fuzzExecute parses and executes text. An error must be an *Error with a
// position inside text. A text without an action must be the output.
func fuzzExecute(t *testing.T, text string, data any, opts ...expr.Option) {
	t.Helper()
	got, err := execute(t, text, data, opts...)
	if err != nil {
		e, ok := errors.AsType[*Error](err)
		if !ok {
			t.Fatalf("got %T, want *Error", err)
		}
		if lines := strings.Count(text, "\n") + 1; e.Line < 1 || e.Line > lines || e.Column < 1 {
			t.Fatalf("position %d:%d is outside the text", e.Line, e.Column)
		}
	}
	if strings.Contains(text, leftDelim) {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if got != text {
		t.Errorf("got %q, want %q", got, text)
	}
}

func TestPrint(t *testing.T) {
	print := func(v any) (string, error) {
		return fmt.Sprintf("<%v>", v), nil
	}
	text := "a {{ 1 }} {{ if ok }}{{ for x in nums }}{{ x }}{{ end }}{{ end }} {{ nil }}"
	tmpl := Must(New("t").Print(print).Parse(text))
	var b strings.Builder
	if err := tmpl.Execute(&b, map[string]any{"ok": true, "nums": []int{2, 3}}); err != nil {
		t.Fatal(err)
	}
	if got, want := b.String(), "a <1> <2><3> <<nil>>"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPrintError(t *testing.T) {
	errNil := errors.New("cannot print nil")
	print := func(v any) (string, error) {
		if v == nil {
			return "ignored", errNil
		}
		return fmt.Sprint(v), nil
	}
	tests := []struct {
		name string
		text string
		out  string
		want string
	}{
		{
			"action",
			"a {{ 1 }}\nb {{- m.x }} c",
			"a 1\nb",
			"cannot print nil (t:2:3)\n | b {{- m.x }} c\n | ..^",
		},
		{
			"action in a loop",
			"{{ for x in [1, nil] }}{{ x }};{{ end }}",
			"1;",
			"cannot print nil (t:1:24)\n | {{ for x in [1, nil] }}{{ x }};{{ end }}\n | .......................^",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := Must(New("t").Print(print).Parse(tt.text))
			var b strings.Builder
			err := tmpl.Execute(&b, map[string]any{"m": map[string]any{}})
			if err == nil {
				t.Fatal("no error")
			}
			if err.Error() != tt.want {
				t.Errorf("got\n%s\nwant\n%s", err, tt.want)
			}
			if !errors.Is(err, errNil) {
				t.Errorf("errors.Is(err, errNil) is false")
			}
			if got := b.String(); got != tt.out {
				t.Errorf("got output %q, want %q", got, tt.out)
			}
		})
	}
}
