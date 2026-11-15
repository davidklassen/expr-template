package template

import (
	"fmt"
	"strings"
	"testing"

	"github.com/expr-lang/expr/file"
)

var nodeTests = []struct {
	name string
	text string
	want string
}{
	{
		"trim markers",
		"é  {{- a + 1 -}}  →{{ b }}",
		`text "é"
action "{{- a + 1 -}}" expr "a + 1"
text "→"
action "{{ b }}" expr "b"
`,
	},
	{
		"trim marker after the action only",
		"{{ a -}} b",
		`action "{{ a -}}" expr "a"
text "b"
`,
	},
	{
		"minus that is not a trim marker",
		"{{-3}}",
		`action "{{-3}}" expr "-3"
`,
	},
	{
		"comment",
		"a{{ /* é */ }}b",
		`text "a"
text "b"
`,
	},
	{
		"if",
		"{{ if /* é */ a /* é */ }}→{{ end }}",
		`if "{{ if /* é */ a /* é */ }}" cond "a"
	text "→"
`,
	},
	{
		"if with an empty else",
		"{{ if a }}{{ else }}{{ end }}",
		`if "{{ if a }}" cond "a"
`,
	},
	{
		"else if",
		"{{ if a }}1{{ else if b }}2{{ else }}{{ c }}{{ end }}",
		`if "{{ if a }}" cond "a"
	text "1"
else
	if "{{ else if b }}" cond "b"
		text "2"
	else
		action "{{ c }}" expr "c"
`,
	},
	{
		"for",
		"{{ for x in é }}{{ x }}{{ end }}",
		`for "{{ for x in é }}" key "" elem "x" collection "é"
	action "{{ x }}" expr "x"
`,
	},
	{
		"for with a key",
		"é{{- for k, v in m -}} {{ if v }}{{ k }}{{ end }} {{ end }}",
		`text "é"
for "{{- for k, v in m -}}" key "k" elem "v" collection "m"
	if "{{ if v }}" cond "v"
		action "{{ k }}" expr "k"
	text " "
`,
	},
}

func TestNodes(t *testing.T) {
	for _, tt := range nodeTests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := New("t").Parse(tt.text)
			if err != nil {
				t.Fatal(err)
			}
			text := []rune(tt.text)
			checkLocations(t, text, 0, tmpl.Nodes())
			var b strings.Builder
			dump(&b, text, tmpl.Nodes(), "")
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

// dump writes one line for each node. For an action and for an expression, the
// line shows the part of text that the location covers.
func dump(b *strings.Builder, text []rune, nodes []Node, indent string) {
	at := func(n interface{ Location() file.Location }) string {
		loc := n.Location()
		return fmt.Sprintf("%q", string(text[loc.From:loc.To]))
	}
	for _, n := range nodes {
		switch n := n.(type) {
		case *TextNode:
			fmt.Fprintf(b, "%stext %q\n", indent, n.Text)
		case *ActionNode:
			fmt.Fprintf(b, "%saction %s expr %s\n", indent, at(n), at(n.Expr))
		case *IfNode:
			fmt.Fprintf(b, "%sif %s cond %s\n", indent, at(n), at(n.Cond))
			dump(b, text, n.Body, indent+"\t")
			if len(n.ElseBody) > 0 {
				fmt.Fprintf(b, "%selse\n", indent)
				dump(b, text, n.ElseBody, indent+"\t")
			}
		case *ForNode:
			fmt.Fprintf(b, "%sfor %s key %q elem %q collection %s\n", indent, at(n), n.Key, n.Elem, at(n.Collection))
			dump(b, text, n.Body, indent+"\t")
		}
	}
}

// checkLocations checks the locations of nodes and of their expressions against
// text, the text of the template. The nodes must be in order and start at the
// rune offset from or after it.
func checkLocations(t *testing.T, text []rune, from int, nodes []Node) {
	t.Helper()
	// covers checks that loc covers want in text. text is a rune slice, and
	// the conversion from a string to runes replaces each byte that is not
	// valid UTF-8. So want gets the same conversion before the comparison.
	covers := func(what string, loc file.Location, want string) {
		t.Helper()
		if loc.From < 0 || loc.From > loc.To || loc.To > len(text) {
			t.Fatalf("location %v of %s is outside the text", loc, what)
		}
		if got, want := string(text[loc.From:loc.To]), string([]rune(want)); got != want {
			t.Errorf("location of %s covers %q, want %q", what, got, want)
		}
	}
	action := func(n Node, e *Expr) {
		t.Helper()
		loc := n.Location()
		covers("the action", file.Location{From: loc.From, To: loc.From + len(leftDelim)}, leftDelim)
		covers("the action", file.Location{From: loc.To - len(rightDelim), To: loc.To}, rightDelim)
		covers("the expression", e.Location(), e.Program.Source().String())
		if e.Location().From < loc.From+len(leftDelim) || e.Location().To > loc.To-len(rightDelim) {
			t.Errorf("expression at %v is outside its action at %v", e.Location(), loc)
		}
	}
	for _, n := range nodes {
		loc := n.Location()
		if loc.From < from {
			t.Errorf("%T at %v starts before %d", n, loc, from)
		}
		from = loc.To
		switch n := n.(type) {
		case *TextNode:
			if n.Text == "" {
				t.Errorf("empty text node at %v", loc)
			}
			covers("the text", loc, n.Text)
		case *ActionNode:
			action(n, n.Expr)
		case *IfNode:
			action(n, n.Cond)
			checkLocations(t, text, loc.To, n.Body)
			checkLocations(t, text, loc.To, n.ElseBody)
		case *ForNode:
			action(n, n.Collection)
			checkLocations(t, text, loc.To, n.Body)
		default:
			t.Errorf("unknown node %T", n)
		}
	}
}
