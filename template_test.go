package template

import (
	"io"
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
	{"trim unicode white space", "a\u00a0\f{{- n -}}\u2003\vb", "a1b"},
	{"trim marker with unicode white space", "a {{-\u00a0n\u00a0-}} b", "a1b"},
	{"minus without space", "{{-3}}", "-3"},
	{"minus before braces", "{{ 5 -3}}", "2"},

	{"comment", "a{{/* c */}}b", "ab"},
	{"comment with spaces", "a{{ /* c */ }}b", "ab"},
	{"comment across lines", "a{{/* c\nd */}}b", "ab"},
	{"comment with braces only", "a{{/* }} */}}b", "ab"},
	{"two comments", "a{{ /* c */ /* d */ }}b", "ab"},
	{"line comment", "a{{ // c\n}}b", "ab"},
	{"comment keeps white space", "a {{/* c */}} b", "a  b"},
	{"trim around comment", "a {{- /* c */ -}} b", "ab"},
	{"trim before comment", "a {{- /* c */}} b", "a b"},
	{"trim after comment only", "a {{/* c */ -}} b", "a b"},
	{"comment between actions", "{{ n }}{{/* c */}}{{ s }}", "1str"},
	{"comment only", "{{/* c */}}", ""},
	{"comment with a form feed", "a{{ /* c */ \f }}b", "ab"},
	{"comment before expression", "a{{ /* c */ n }}b", "a1b"},
	{"line comment before expression", "a{{ // c\n n }}b", "a1b"},

	{"if true", "a{{ if n == 1 }}b{{ end }}c", "abc"},
	{"if false", "a{{ if n == 2 }}b{{ end }}c", "ac"},
	{"if with actions", "{{ if true }}{{ n }}-{{ s }}{{ end }}", "1-str"},
	{"if with empty body", "a{{ if true }}{{ end }}b", "ab"},
	{"if without spaces", "{{if true}}a{{end}}", "a"},
	{"if across lines", "{{ if n ==\n 1 }}a{{ end }}", "a"},
	{"if with comment", "{{ if /* c */ true }}a{{ end /* c */ }}", "a"},
	{"nested if", "{{ if true }}a{{ if false }}b{{ end }}c{{ end }}d", "acd"},
	{"nested if in false", "{{ if false }}a{{ if true }}b{{ end }}c{{ end }}d", "d"},
	{"if after if", "{{ if true }}a{{ end }}{{ if true }}b{{ end }}", "ab"},
	{"trim in if", "a {{- if true -}} b {{- end -}} c", "abc"},
	{"if with parentheses", "{{ if(n == 1) }}a{{ end }}", "a"},
	{"if expression of expr", "{{ (if n == 1 { 2 } else { 3 }) }}", "2"},
	{"false if does not run its body", "a{{ if false }}{{ l[5] }}{{ end }}b", "ab"},

	{"else of a true if", "{{ if n == 1 }}a{{ else }}b{{ end }}c", "ac"},
	{"else of a false if", "{{ if n == 2 }}a{{ else }}b{{ end }}c", "bc"},
	{"else with actions", "{{ if false }}{{ n }}{{ else }}{{ s }}{{ end }}", "str"},
	{"else without spaces", "{{if false}}a{{else}}b{{end}}", "b"},
	{"else with comment", "{{ if false }}a{{ else /* c */ }}b{{ end }}", "b"},
	{"empty else", "a{{ if false }}b{{ else }}{{ end }}c", "ac"},
	{"trim around else", "{{ if false }} a {{- else -}} b {{- end }}", "b"},
	{"else if, first is true", "{{ if n == 1 }}a{{ else if n == 2 }}b{{ else }}c{{ end }}", "a"},
	{"else if, second is true", "{{ if n == 2 }}a{{ else if n == 1 }}b{{ else }}c{{ end }}", "b"},
	{"else if, none is true", "{{ if n == 2 }}a{{ else if n == 3 }}b{{ else }}c{{ end }}", "c"},
	{"else if without else", "x{{ if n == 2 }}a{{ else if n == 3 }}b{{ end }}y", "xy"},
	{"three else if", "{{ if n == 4 }}a{{ else if n == 3 }}b{{ else if n == 2 }}c{{ else if n == 1 }}d{{ end }}", "d"},
	{"else if across lines", "{{ if false }}a{{ else\nif\nn == 1 }}b{{ end }}", "b"},
	{"if in else", "{{ if false }}a{{ else }}{{ if true }}b{{ end }}c{{ end }}", "bc"},
	{"if with else in if", "{{ if true }}{{ if false }}a{{ else }}b{{ end }}{{ else }}c{{ end }}", "b"},
	{"trim around else if", "{{ if false }} a {{- else if true -}} b {{- end }}", "b"},
	{"true if does not evaluate else if", "{{ if true }}a{{ else if l[5] == 1 }}b{{ end }}", "a"},
	{"true else if does not evaluate the next else if", "{{ if false }}a{{ else if true }}b{{ else if l[5] == 1 }}c{{ end }}", "b"},
	{"keyword in string", `{{ "if" }}{{ "end" }}{{ "for" }}`, "ifendfor"},

	{"for", "{{ for x in l }}{{ x }},{{ end }}", "1,2,"},
	{"for with if", "{{ for x in l }}{{ if x == n }}a{{ else }}b{{ end }}{{ end }}", "ab"},
	{"nested for", "{{ for x in l }}{{ for y in l }}{{ x * y }} {{ end }}{{ end }}", "1 2 2 4 "},
	{"trim in for", "{{ for x in l -}} {{ x }} {{- end }}", "12"},
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
	data := map[string]any{"end": 1, "ending": 2, "iffy": 3, "t": true, "If": 4, "IF": 5}
	tests := []struct {
		name string
		text string
		want string
	}{
		{"in parentheses", "{{ (end) }}", "1"},
		{"not the first word", "{{ 1 + end }}", "2"},
		{"in a condition", "{{ if end == 1 }}a{{ end }}", "a"},
		{"prefix of a name", "{{ ending }}{{ iffy }}", "23"},
		{"if in another case", "{{ If }}{{ IF }}", "45"},
		{"end in another case", "{{ if t }}{{ END }}{{ end }}", "<nil>"},
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

func TestExecuteWithoutOptions(t *testing.T) {
	got, err := execute(t, "{{ a }}-{{ b }}", map[string]any{"a": 1, "b": "x"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "1-x"; got != want {
		t.Errorf("got %q, want %q", got, want)
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

func FuzzParse(f *testing.F) {
	for _, tt := range executeTests {
		f.Add(tt.text)
	}
	for _, tt := range loopTests {
		f.Add(tt.text)
	}
	f.Fuzz(func(t *testing.T, text string) {
		tmpl, err := New("t", expr.Env(env{})).Parse(text)
		if err != nil {
			return
		}
		var b strings.Builder
		err = tmpl.Execute(&b, testData)
		if strings.Contains(text, leftDelim) {
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		if got := b.String(); got != text {
			t.Errorf("got %q, want %q", got, text)
		}
	})
}
