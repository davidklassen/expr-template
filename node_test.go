package template

import (
	"fmt"
	"strings"
	"testing"

	"github.com/expr-lang/expr/file"
)

// dump writes one line for each node of the tree. A line has the part of the
// template text that the location of the node covers. For an expression, it
// also has the source of the program.
func dump(b *strings.Builder, text string, nodes []Node, indent string) {
	at := func(loc file.Location) string {
		return fmt.Sprintf("%q", string([]rune(text)[loc.From:loc.To]))
	}
	expr := func(e *Expr) string {
		return fmt.Sprintf("%s program %q", at(e.Location()), e.Program.Source().String())
	}
	for _, n := range nodes {
		switch n := n.(type) {
		case *TextNode:
			fmt.Fprintf(b, "%stext %s %q\n", indent, at(n.Location()), n.Text)
		case *ActionNode:
			fmt.Fprintf(b, "%saction %s expr %s\n", indent, at(n.Location()), expr(n.Expr))
		case *IfNode:
			fmt.Fprintf(b, "%sif %s cond %s\n", indent, at(n.Location()), expr(n.Cond))
			dump(b, text, n.Body, indent+"\t")
			if len(n.ElseBody) > 0 {
				fmt.Fprintf(b, "%selse\n", indent)
				dump(b, text, n.ElseBody, indent+"\t")
			}
		case *ForNode:
			fmt.Fprintf(b, "%sfor %s key %q elem %q collection %s\n", indent, at(n.Location()), n.Key, n.Elem, expr(n.Collection))
			dump(b, text, n.Body, indent+"\t")
		default:
			fmt.Fprintf(b, "%sunknown node %T\n", indent, n)
		}
	}
}

func TestNodes(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			"text",
			"é→",
			`text "é→" "é→"
`,
		},
		{
			"action",
			"é {{ a + 1 }} →",
			`text "é " "é "
action "{{ a + 1 }}" expr "a + 1 " program "a + 1 "
text " →" " →"
`,
		},
		{
			"trim markers",
			"é  {{- a -}}  →{{ b }}",
			`text "é" "é"
action "{{- a -}}" expr "a " program "a "
text "→" "→"
action "{{ b }}" expr "b " program "b "
`,
		},
		{
			"comment",
			"a{{ /* é */ }}b",
			`text "a" "a"
text "b" "b"
`,
		},
		{
			"if",
			"{{ if /* é */ a }}→{{ end }}",
			`if "{{ if /* é */ a }}" cond "a " program "a "
	text "→" "→"
`,
		},
		{
			"if with an empty else",
			"{{ if a }}{{ else }}{{ end }}",
			`if "{{ if a }}" cond "a " program "a "
`,
		},
		{
			"else if",
			"{{ if a }}1{{ else if b }}2{{ else }}{{ c }}{{ end }}",
			`if "{{ if a }}" cond "a " program "a "
	text "1" "1"
else
	if "{{ else if b }}" cond "b " program "b "
		text "2" "2"
	else
		action "{{ c }}" expr "c " program "c "
`,
		},
		{
			"for",
			"{{ for x in é }}{{ x }}{{ end }}",
			`for "{{ for x in é }}" key "" elem "x" collection "é " program "é "
	action "{{ x }}" expr "x " program "x "
`,
		},
		{
			"for with a key",
			"é{{- for k, v in m -}} {{ if v }}{{ k }}{{ end }} {{ end }}",
			`text "é" "é"
for "{{- for k, v in m -}}" key "k" elem "v" collection "m " program "m "
	if "{{ if v }}" cond "v " program "v "
		action "{{ k }}" expr "k " program "k "
	text " " " "
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := New("t").Parse(tt.text)
			if err != nil {
				t.Fatal(err)
			}
			var b strings.Builder
			dump(&b, tt.text, tmpl.Nodes(), "")
			if got := b.String(); got != tt.want {
				t.Errorf("got\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

func TestNodesBeforeParse(t *testing.T) {
	if got := New("t").Nodes(); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

// checkLocations checks that the location of each node and of each expression
// covers the right part of the template text.
func checkLocations(t *testing.T, text []rune, nodes []Node) {
	t.Helper()
	at := func(loc file.Location) string {
		if loc.From < 0 || loc.From > loc.To || loc.To > len(text) {
			t.Fatalf("location %v is outside the text", loc)
		}
		return string(text[loc.From:loc.To])
	}
	// text is a rune slice, and the conversion to runes replaces each byte
	// that is not valid UTF-8. valid does the same to the string to compare.
	valid := func(s string) string {
		return string([]rune(s))
	}
	action := func(n Node, e *Expr) {
		if got := at(n.Location()); !strings.HasPrefix(got, leftDelim) || !strings.HasSuffix(got, rightDelim) {
			t.Errorf("location of %T covers %q, want an action", n, got)
		}
		if got, want := at(e.Location()), valid(e.Program.Source().String()); got != want {
			t.Errorf("location of the expression covers %q, want %q", got, want)
		}
	}
	for _, n := range nodes {
		switch n := n.(type) {
		case *TextNode:
			if got, want := at(n.Location()), valid(n.Text); got != want {
				t.Errorf("location of the text covers %q, want %q", got, want)
			}
		case *ActionNode:
			action(n, n.Expr)
		case *IfNode:
			action(n, n.Cond)
			checkLocations(t, text, n.Body)
			checkLocations(t, text, n.ElseBody)
		case *ForNode:
			action(n, n.Collection)
			checkLocations(t, text, n.Body)
		default:
			t.Errorf("unknown node %T", n)
		}
	}
}
