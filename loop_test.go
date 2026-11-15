package template

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/types"
)

type item struct {
	Title string `expr:"title"`
}

type loopBase struct {
	ID int `expr:"id"`
}

type hidden struct {
	Secret string
}

type deep struct {
	Deep string
}

type id string

type namedMap map[string]any

func (namedMap) Title() string { return "t" }

type str string

func (s str) String() string { return string(s) }

type loopData struct {
	loopBase
	hidden
	*deep
	Name   string            `expr:"name"`
	Items  []item            `expr:"items"`
	Nums   []int             `expr:"nums"`
	Empty  []int             `expr:"empty"`
	Lists  [][]int           `expr:"lists"`
	Single []int             `expr:"single"`
	Mixed  any               `expr:"mixed"`
	Str    fmt.Stringer      `expr:"str"`
	Arr    [2]int            `expr:"arr"`
	Ages   map[string]int    `expr:"ages"`
	ByID   map[int]string    `expr:"byid"`
	Ratio  map[float64]int   `expr:"ratio"`
	UKeys  map[uint64]string `expr:"ukeys"`
	Words  map[string]int    `expr:"words"`
	ByName map[id]int        `expr:"byname"`
	NoMap  map[string]int    `expr:"nomap"`
	Flags  map[bool]int      `expr:"flags"`
	Boxed  any               `expr:"boxed"`
	Boxed2 any               `expr:"boxed2"`
	Skip   string            `expr:"-"`
	Plain  string
}

func (d loopData) Greet(s string) string { return "hi " + s }

func (d *loopData) Count() int { return len(d.Items) }

func (d loopData) First() *item { return &d.Items[0] }

func (d loopData) Take(it *item) string { return it.Title }

func (d loopData) Join(a, b item) string { return a.Title + b.Title }

func (d loopData) Drop(key string) string {
	delete(d.Ages, key)
	return ""
}

func newLoopData() *loopData {
	return &loopData{
		loopBase: loopBase{ID: 7},
		hidden:   hidden{Secret: "s"},
		Name:     "n",
		Items:    []item{{"a"}, {"b"}},
		Nums:     []int{1, 2, 3},
		Lists:    [][]int{{1, 2}, {3}},
		Single:   []int{0},
		Mixed:    []any{1, "a"},
		Str:      str("S"),
		Arr:      [2]int{4, 5},
		Ages:     map[string]int{"bob": 2, "al": 1, "cy": 3},
		ByID:     map[int]string{2: "b", 10: "j", -1: "n", 1: "a"},
		Ratio:    map[float64]int{2.5: 2, 10.5: 3, 0.5: 1},
		UKeys:    map[uint64]string{1 << 63: "x", 10: "j", 2: "b"},
		Words:    map[string]int{"k2": 1, "a": 2, "é": 3, "B": 4, "k10": 5},
		ByName:   map[id]int{"b": 2, "a": 1},
		Flags:    map[bool]int{true: 1},
		Boxed:    [2]int{6, 7},
		Boxed2:   map[string]int{"y": 2, "x": 1},
		Skip:     "skip",
		Plain:    "p",
	}
}

func newLoopMap() map[string]any {
	d := newLoopData()
	return map[string]any{
		"id":     d.ID,
		"Secret": d.Secret,
		"name":   d.Name,
		"items":  d.Items,
		"nums":   d.Nums,
		"empty":  d.Empty,
		"lists":  d.Lists,
		"single": d.Single,
		"mixed":  d.Mixed,
		"str":    d.Str,
		"arr":    d.Arr,
		"ages":   d.Ages,
		"byid":   d.ByID,
		"ratio":  d.Ratio,
		"ukeys":  d.UKeys,
		"words":  d.Words,
		"byname": d.ByName,
		"nomap":  d.NoMap,
		"flags":  d.Flags,
		"boxed":  d.Boxed,
		"boxed2": d.Boxed2,
		"Plain":  d.Plain,
	}
}

// A loopSetup is one way to give a template its env and its data.
type loopSetup struct {
	name    string
	opts    []expr.Option
	data    any
	methods bool // the data has methods
}

func (s loopSetup) hasEnv() bool { return s.opts != nil }

func loopSetups() []loopSetup {
	return []loopSetup{
		{name: "struct env", opts: []expr.Option{expr.Env(loopData{})}, data: *newLoopData(), methods: true},
		{name: "pointer env", opts: []expr.Option{expr.Env(&loopData{})}, data: newLoopData(), methods: true},
		{name: "struct env and pointer data", opts: []expr.Option{expr.Env(loopData{})}, data: newLoopData(), methods: true},
		{name: "map env", opts: []expr.Option{expr.Env(newLoopMap())}, data: newLoopMap()},
		{name: "no env and struct data", data: *newLoopData(), methods: true},
		{name: "no env and pointer data", data: newLoopData(), methods: true},
		{name: "no env and map data", data: newLoopMap()},
	}
}

var loopTests = []struct {
	name string
	text string
	want string
}{
	{"list", "{{ for x in nums }}{{ x }},{{ end }}", "1,2,3,"},
	{"empty body", "a{{ for x in nums }}{{ end }}b", "ab"},
	{"nil list", "a{{ for x in empty }}b{{ end }}c", "ac"},
	{"empty list", "a{{ for x in [] }}b{{ end }}c", "ac"},
	{"array", "{{ for x in arr }}{{ x + 1 }}{{ end }}", "56"},
	{"array of unknown type", "{{ for x in boxed }}{{ x }}{{ end }}", "67"},
	{"list expression", "{{ for x in filter(nums, # > 1) }}{{ x }}{{ end }}", "23"},
	{"list of unknown type", "{{ for x in mixed }}{{ x }}{{ end }}", "1a"},
	{"variable in an expression", "{{ for x in nums }}{{ x + 1 }}{{ end }}", "234"},
	{"field of the variable", "{{ for it in items }}{{ it.title }};{{ end }}", "a;b;"},
	{"name of the data in the body", "{{ for x in nums }}{{ name }}{{ x }} {{ end }}", "n1 n2 n3 "},
	{"with comments", "{{ for /* c */ x /* c */ in /* c */ nums }}{{ x }}{{ end /* c */ }}", "123"},
	{"trim", "a {{- for x in nums -}} {{ x }} {{- end -}} b", "a123b"},
	{"if in for", "{{ for x in nums }}{{ if x > 1 }}{{ x }}{{ else }}-{{ end }}{{ end }}", "-23"},
	{"for in if", "{{ if true }}{{ for x in nums }}{{ x }}{{ end }}{{ end }}", "123"},
	{"for after for", "{{ for x in nums }}{{ x }}{{ end }}{{ for x in single }}{{ x }}{{ end }}", "1230"},
	{"nested", "{{ for l in lists }}[{{ for x in l }}{{ x }}{{ end }}]{{ end }}", "[12][3]"},
	{"inner loop sees the outer variable", "{{ for a in nums }}{{ for b in single }}{{ a + b }}{{ end }}{{ end }}", "123"},
	{"variable shadows a name of the data", "{{ for name in nums }}{{ name + 1 }}{{ end }}{{ name }}", "234n"},
	{"variable shadows an outer variable", "{{ for x in nums }}{{ for x in single }}{{ x }}{{ end }}{{ x }}{{ end }}", "010203"},
	{"list of an inner loop is the outer variable", "{{ for x in lists }}{{ for x in x }}{{ x }}{{ end }}{{ end }}", "123"},
	{"variable named as a function", "{{ for len in nums }}{{ len }}{{ end }}{{ len(nums) }}", "1233"},
	{"index", "{{ for i, x in nums }}{{ i }}:{{ x }} {{ end }}", "0:1 1:2 2:3 "},
	{"index in an expression", "{{ for i, x in nums }}{{ i + x }}{{ end }}", "135"},
	{"index of an array", "{{ for i, x in arr }}{{ i }}{{ x }}{{ end }}", "0415"},
	{"index of a list of unknown type", "{{ for i, x in mixed }}{{ i }}{{ x }}{{ end }}", "011a"},
	{"index shadows a name of the data", "{{ for name, x in nums }}{{ name }}{{ end }}{{ name }}", "012n"},
	{"index of an outer loop in an inner loop", "{{ for i, l in lists }}{{ for j, x in l }}{{ i }}{{ j }}{{ x }} {{ end }}{{ end }}", "001 012 103 "},
	{"map", "{{ for v in ages }}{{ v }}{{ end }}", "123"},
	{"map with key", "{{ for k, v in ages }}{{ k }}={{ v }} {{ end }}", "al=1 bob=2 cy=3 "},
	{"map key and value in an expression", "{{ for k, v in ages }}{{ upper(k) }}{{ v + 1 }}{{ end }}", "AL2BOB3CY4"},
	{"map with int keys", "{{ for k, v in byid }}{{ k + 1 }}{{ v }} {{ end }}", "0n 2a 3b 11j "},
	{"map with float keys", "{{ for k, v in ratio }}{{ k }}:{{ v }} {{ end }}", "0.5:1 2.5:2 10.5:3 "},
	{"map with unsigned keys", "{{ for k, v in ukeys }}{{ k }}{{ v }} {{ end }}", "2b 10j 9223372036854775808x "},
	{"map with string keys in the order of their bytes", "{{ for k, v in words }}{{ k }} {{ end }}", "B a k10 k2 é "},
	{"map with keys of a named type", "{{ for k, v in byname }}{{ k }}{{ v }}{{ end }}", "a1b2"},
	{"map from groupBy", "{{ for k, v in groupBy(nums, # % 2) }}{{ k }}:{{ v }} {{ end }}", "0:[2] 1:[1 3] "},
	{"map literal", `{{ for k, v in {"b": 2, "a": 1} }}{{ k }}{{ v }}{{ end }}`, "a1b2"},
	{"map of unknown type", "{{ for k, v in boxed2 }}{{ k }}{{ v }}{{ end }}", "x1y2"},
	{"nil map", "a{{ for k, v in nomap }}b{{ end }}c", "ac"},
	{"empty map", "a{{ for k, v in {} }}b{{ end }}c", "ac"},
	{"list in a map loop", "{{ for k, v in ages }}{{ for x in single }}{{ k }}{{ v + x }}{{ end }}{{ end }}", "al1bob2cy3"},
}

func TestLoop(t *testing.T) {
	for _, setup := range loopSetups() {
		for _, tt := range loopTests {
			t.Run(setup.name+"/"+tt.name, func(t *testing.T) {
				got, err := execute(t, tt.text, setup.data, setup.opts...)
				if err != nil {
					t.Fatal(err)
				}
				if got != tt.want {
					t.Errorf("got  %q\nwant %q", got, tt.want)
				}
			})
		}
	}
}

func TestLoopMethods(t *testing.T) {
	for _, setup := range loopSetups() {
		if !setup.methods {
			continue
		}
		t.Run(setup.name, func(t *testing.T) {
			got, err := execute(t, "{{ for x in single }}{{ Take(First()) }}{{ end }}", setup.data, setup.opts...)
			if err != nil {
				t.Fatal(err)
			}
			if want := "a"; got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

// TestLoopEnv checks that an expression gives the same result inside a loop
// body as outside it. This fails if the loop env does not have every name of
// the env with the same type.
func TestLoopEnv(t *testing.T) {
	exprs := []string{
		"name", "id", "Secret", "Plain", "nums", "len(items)", "items[0].title", "mixed", "str", "str.String()",
		"empty == nil", "name + Plain", "id + 1", "Greet(name)", "Count()",
		// With an env option, these are errors. A loop body must give the same
		// errors.
		"Skip", "skip", "loopBase", "hidden", "nope", "name + 1", "str.Nope()", "items[0].nope", "Greet(1)",
	}
	run := func(text string, setup loopSetup) (string, bool) {
		tmpl, err := New("t", setup.opts...).Parse(text)
		if err != nil {
			return "parse: " + err.(*Error).Message, false
		}
		var b strings.Builder
		if err := tmpl.Execute(&b, setup.data); err != nil {
			return "execute: " + err.(*Error).Message, false
		}
		return b.String(), true
	}
	for _, setup := range loopSetups() {
		for _, e := range exprs {
			t.Run(setup.name+"/"+e, func(t *testing.T) {
				want, ok := run("{{ "+e+" }}", setup)
				if !ok && !setup.hasEnv() {
					// Without an env option, an unknown name is an error only
					// at run time, and only for struct data. In a loop body the
					// env is a map, and an unknown name is nil. TestLoopLimits
					// checks this difference.
					t.Skip(want)
				}
				if setup.name == "struct env and pointer data" && (e == "Greet(name)" || e == "Greet(1)") {
					// expr has a bug here. It calls a method by its index in
					// the type of the env option. The data is a pointer and has
					// one more method than that type, so outside a loop expr
					// calls the wrong method. newLoopEnv finds methods by name.
					t.Skip(want)
				}
				for _, text := range []string{
					"{{ for x in single }}{{ " + e + " }}{{ end }}",
					"{{ for x in single }}{{ for y in single }}{{ " + e + " }}{{ end }}{{ end }}",
				} {
					if got, _ := run(text, setup); got != want {
						t.Errorf("%s\ngot  %s\nwant %s", text, got, want)
					}
				}
			})
		}
	}
}

type counter struct {
	N int
}

func (c *counter) Inc() string {
	c.N++
	return ""
}

// TestLoopLimits checks the two limits of loops that the package doc states.
func TestLoopLimits(t *testing.T) {
	t.Run("method changes a field of the data", func(t *testing.T) {
		got, err := execute(t, "{{ for x in [1, 2] }}{{ Inc() }}{{ N }}{{ end }}{{ N }}", &counter{})
		if err != nil {
			t.Fatal(err)
		}
		if want := "002"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
	t.Run("unknown name without an env option", func(t *testing.T) {
		_, err := execute(t, "{{ nope }}", struct{}{})
		want := "cannot fetch nope from struct {} (t:1:4)\n | {{ nope }}\n | ...^"
		if err == nil || err.Error() != want {
			t.Errorf("got\n%v\nwant\n%s", err, want)
		}
		got, err := execute(t, "{{ for x in [1] }}{{ nope }}{{ end }}", struct{}{})
		if err != nil {
			t.Fatal(err)
		}
		if want := "<nil>"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func TestExecuteParallel(t *testing.T) {
	const text = "{{ for k, v in ages }}{{ k }}{{ for x in nums }}{{ Greet(name) }}{{ v + x }} {{ end }}{{ end }}"
	tmpl := Must(New("t", expr.Env(&loopData{})).Parse(text))
	data := newLoopData()
	var want strings.Builder
	if err := tmpl.Execute(&want, data); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			var got strings.Builder
			if err := tmpl.Execute(&got, data); err != nil {
				t.Error(err)
				return
			}
			if got.String() != want.String() {
				t.Errorf("got %q, want %q", got.String(), want.String())
			}
		})
	}
	wg.Wait()
}

var loopParseErrorTests = []struct {
	name string
	text string
	want string
}{
	{
		"list is a string",
		"a {{ for x in name }}{{ end }}",
		"cannot loop over string (t:1:15)\n | a {{ for x in name }}{{ end }}\n | ..............^",
	},
	{
		"list of an inner loop is not a list",
		"a {{ for x in nums }}{{ for y in x }}{{ end }}{{ end }}",
		"cannot loop over int (t:1:34)\n | a {{ for x in nums }}{{ for y in x }}{{ end }}{{ end }}\n | .................................^",
	},
	{
		"error in the list",
		"a {{ for x in nope }}{{ end }}",
		"unknown name nope (t:1:15)\n | a {{ for x in nope }}{{ end }}\n | ..............^",
	},
	{
		"wrong type of the variable of an array literal",
		"a {{ for x in [id, 2] }}{{ x + name }}{{ end }}",
		"invalid operation: + (mismatched types int and string) (t:1:30)\n | a {{ for x in [id, 2] }}{{ x + name }}{{ end }}\n | .............................^",
	},
	{
		"wrong type of the variable of a range",
		"a {{ for x in 1..3 }}{{ x + name }}{{ end }}",
		"invalid operation: + (mismatched types int and string) (t:1:27)\n | a {{ for x in 1..3 }}{{ x + name }}{{ end }}\n | ..........................^",
	},
	{
		"wrong field of the variable of a builtin",
		"a {{ for it in filter(items, true) }}{{ it.titel }}{{ end }}",
		"type template.item has no field titel (t:1:44)\n | a {{ for it in filter(items, true) }}{{ it.titel }}{{ end }}\n | ...........................................^",
	},
	{
		"wrong type of the variable",
		"a {{ for x in nums }}{{ x + name }}{{ end }}",
		"invalid operation: + (mismatched types int and string) (t:1:27)\n | a {{ for x in nums }}{{ x + name }}{{ end }}\n | ..........................^",
	},
	{
		"variable after the loop",
		"a {{ for x in nums }}{{ end }}{{ x }}",
		"unknown name x (t:1:34)\n | a {{ for x in nums }}{{ end }}{{ x }}\n | .................................^",
	},
	{
		"variable shadows a function",
		"a {{ for len in items }}{{ len(nums) }}{{ end }}",
		"template.item is not callable (t:1:28)\n | a {{ for len in items }}{{ len(nums) }}{{ end }}\n | ...........................^",
	},
	{
		"for without variable",
		"a {{ for }}b{{ end }}",
		"missing variable (t:1:3)\n | a {{ for }}b{{ end }}\n | ..^",
	},
	{
		"for without in",
		"a {{ for x }}b{{ end }}",
		"unexpected token EOF (t:1:11)\n | a {{ for x }}b{{ end }}\n | ..........^",
	},
	{
		"another word for in",
		"a {{ for x of nums }}b{{ end }}",
		"unexpected token Identifier(\"of\") (t:1:12)\n | a {{ for x of nums }}b{{ end }}\n | ...........^",
	},
	{
		"three variables",
		"a {{ for x, y, z in nums }}b{{ end }}",
		"unexpected token Operator(\",\") (t:1:14)\n | a {{ for x, y, z in nums }}b{{ end }}\n | .............^",
	},
	{
		"comma without first variable",
		"a {{ for , x in nums }}b{{ end }}",
		"unexpected token Operator(\",\") (t:1:10)\n | a {{ for , x in nums }}b{{ end }}\n | .........^",
	},
	{
		"comma without second variable",
		"a {{ for i, in nums }}b{{ end }}",
		"unexpected token Operator(\"in\") (t:1:13)\n | a {{ for i, in nums }}b{{ end }}\n | ............^",
	},
	{
		"comma at the end",
		"a {{ for i, }}b{{ end }}",
		"missing variable (t:1:3)\n | a {{ for i, }}b{{ end }}\n | ..^",
	},
	{
		"second variable named end",
		"a {{ for i, end in nums }}b{{ end }}",
		"unexpected token Identifier(\"end\") (t:1:13)\n | a {{ for i, end in nums }}b{{ end }}\n | ............^",
	},
	{
		"two variables with one name",
		"a {{ for x, x in nums }}b{{ end }}",
		"cannot redeclare x (t:1:13)\n | a {{ for x, x in nums }}b{{ end }}\n | ............^",
	},
	{
		"wrong type of the index",
		"a {{ for i, x in nums }}{{ i + name }}{{ end }}",
		"invalid operation: + (mismatched types int and string) (t:1:30)\n | a {{ for i, x in nums }}{{ i + name }}{{ end }}\n | .............................^",
	},
	{
		"wrong type of the key",
		"a {{ for k, v in ages }}{{ k + v }}{{ end }}",
		"invalid operation: + (mismatched types string and int) (t:1:30)\n | a {{ for k, v in ages }}{{ k + v }}{{ end }}\n | .............................^",
	},
	{
		"map with keys that cannot be sorted",
		"a {{ for k, v in flags }}b{{ end }}",
		"cannot sort keys of map[bool]int (t:1:18)\n | a {{ for k, v in flags }}b{{ end }}\n | .................^",
	},
	{
		"for without list",
		"a {{ for x in }}b{{ end }}",
		"missing list (t:1:3)\n | a {{ for x in }}b{{ end }}\n | ..^",
	},
	{
		"variable named end",
		"a {{ for end in nums }}b{{ end }}",
		"unexpected token Identifier(\"end\") (t:1:10)\n | a {{ for end in nums }}b{{ end }}\n | .........^",
	},
	{
		"variable named for",
		"a {{ for for in nums }}b{{ end }}",
		"unexpected token Identifier(\"for\") (t:1:10)\n | a {{ for for in nums }}b{{ end }}\n | .........^",
	},
	{
		"variable named if",
		"a {{ for if in nums }}b{{ end }}",
		"unexpected token Operator(\"if\") (t:1:10)\n | a {{ for if in nums }}b{{ end }}\n | .........^",
	},
	{
		"variable named nil",
		"a {{ for nil in nums }}b{{ end }}",
		"unexpected token Identifier(\"nil\") (t:1:10)\n | a {{ for nil in nums }}b{{ end }}\n | .........^",
	},
	{
		"variable named true",
		"a {{ for true in nums }}b{{ end }}",
		"unexpected token Identifier(\"true\") (t:1:10)\n | a {{ for true in nums }}b{{ end }}\n | .........^",
	},
	{
		"variable named false",
		"a {{ for false in nums }}b{{ end }}",
		"unexpected token Identifier(\"false\") (t:1:10)\n | a {{ for false in nums }}b{{ end }}\n | .........^",
	},
	{
		"variable named $env",
		"a {{ for $env in nums }}b{{ end }}",
		"unexpected token Identifier(\"$env\") (t:1:10)\n | a {{ for $env in nums }}b{{ end }}\n | .........^",
	},
	{
		"for without end",
		"a {{ for x in nums }}b",
		"unclosed for (t:1:3)\n | a {{ for x in nums }}b\n | ..^",
	},
	{
		"if around for without end",
		"a {{ if true }}{{ for x in nums }}b{{ end }}",
		"unclosed if (t:1:3)\n | a {{ if true }}{{ for x in nums }}b{{ end }}\n | ..^",
	},
	{
		"else in for",
		"a {{ for x in nums }}b{{ else }}c{{ end }}",
		"unexpected {{else}} (t:1:23)\n | a {{ for x in nums }}b{{ else }}c{{ end }}\n | ......................^",
	},
	{
		"for as a name",
		"a {{ (for) }}",
		"unknown name for (t:1:7)\n | a {{ (for) }}\n | ......^",
	},
	{
		"after multi-byte characters",
		"é {{ /* é */ for é /* é */ in /* é */ nope }}{{ end }}",
		"unknown name nope (t:1:39)\n | é {{ /* é */ for é /* é */ in /* é */ nope }}{{ end }}\n | ......................................^",
	},
}

func TestLoopParseError(t *testing.T) {
	for _, tt := range loopParseErrorTests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New("t", expr.Env(loopData{})).Parse(tt.text)
			if err == nil {
				t.Fatal("no error")
			}
			if err.Error() != tt.want {
				t.Errorf("got\n%s\nwant\n%s", err, tt.want)
			}
		})
	}
}

func TestLoopExecuteError(t *testing.T) {
	tests := []struct {
		name string
		text string
		data loopData
		out  string
		want string
	}{
		{
			"list is nil",
			"a {{ for x in mixed }}b{{ end }}",
			loopData{},
			"a ",
			"cannot loop over nil (t:1:15)\n | a {{ for x in mixed }}b{{ end }}\n | ..............^",
		},
		{
			"list is a string",
			"a {{ for x in mixed }}b{{ end }}",
			loopData{Mixed: "s"},
			"a ",
			"cannot loop over string (t:1:15)\n | a {{ for x in mixed }}b{{ end }}\n | ..............^",
		},
		{
			"map with keys that cannot be sorted",
			"a {{ for x in mixed }}b{{ end }}",
			loopData{Mixed: map[bool]int{true: 1}},
			"a ",
			"cannot sort keys of map[bool]int (t:1:15)\n | a {{ for x in mixed }}b{{ end }}\n | ..............^",
		},
		{
			"error in the body of a map loop",
			"a {{ for k, v in ages }}{{ k }}{{ lists[v] }}{{ end }}",
			*newLoopData(),
			"a al[3]bob",
			"index out of range: 2 (array length is 2) (t:1:40)\n | a {{ for k, v in ages }}{{ k }}{{ lists[v] }}{{ end }}\n | .......................................^",
		},
		{
			"map with keys of mixed kinds",
			"a {{ for x in mixed }}b{{ end }}",
			loopData{Mixed: map[any]int{"a": 1, 2: 2}},
			"a ",
			"cannot sort keys of map[interface {}]int (t:1:15)\n | a {{ for x in mixed }}b{{ end }}\n | ..............^",
		},
		{
			"map with integer and float keys",
			"a {{ for x in mixed }}b{{ end }}",
			loopData{Mixed: map[any]int{1: 1, 2.5: 2}},
			"a ",
			"cannot sort keys of map[interface {}]int (t:1:15)\n | a {{ for x in mixed }}b{{ end }}\n | ..............^",
		},
		{
			"map with signed and unsigned keys",
			"a {{ for x in mixed }}b{{ end }}",
			loopData{Mixed: map[any]int{1: 1, uint(2): 2}},
			"a ",
			"cannot sort keys of map[interface {}]int (t:1:15)\n | a {{ for x in mixed }}b{{ end }}\n | ..............^",
		},
		{
			"error without a position in the list",
			"a {{ for x in (if mixed { [1] } else { [2] }) }}b{{ end }}",
			loopData{Mixed: 5},
			"a ",
			"interface conversion: interface {} is int, not bool (t:1:3)\n | a {{ for x in (if mixed { [1] } else { [2] }) }}b{{ end }}\n | ..^",
		},
		{
			"map with a nil key",
			"a {{ for x in mixed }}b{{ end }}",
			loopData{Mixed: map[any]int{nil: 1}},
			"a ",
			"cannot sort keys of map[interface {}]int (t:1:15)\n | a {{ for x in mixed }}b{{ end }}\n | ..............^",
		},
		{
			"map with keys of an interface type that cannot be sorted",
			"a {{ for x in mixed }}b{{ end }}",
			loopData{Mixed: map[any]int{true: 1}},
			"a ",
			"cannot sort keys of map[interface {}]int (t:1:15)\n | a {{ for x in mixed }}b{{ end }}\n | ..............^",
		},
		{
			"error in the list",
			"a {{ for x in lists[5] }}b{{ end }}",
			*newLoopData(),
			"a ",
			"index out of range: 5 (array length is 2) (t:1:20)\n | a {{ for x in lists[5] }}b{{ end }}\n | ...................^",
		},
		{
			"error in the body",
			"a {{ for x in nums }}{{ x }}{{ lists[x] }}{{ end }}",
			*newLoopData(),
			"a 1[3]2",
			"index out of range: 2 (array length is 2) (t:1:37)\n | a {{ for x in nums }}{{ x }}{{ lists[x] }}{{ end }}\n | ....................................^",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := execute(t, tt.text, tt.data, expr.Env(loopData{}))
			if err == nil {
				t.Fatal("no error")
			}
			if err.Error() != tt.want {
				t.Errorf("got\n%s\nwant\n%s", err, tt.want)
			}
			if got != tt.out {
				t.Errorf("got output %q, want %q", got, tt.out)
			}
		})
	}
}

func TestLoopData(t *testing.T) {
	typed := types.Map{"nums": types.TypeOf([]int{}), "n": types.Int}
	tests := []struct {
		name string
		text string
		data any
		opts []expr.Option
		want string
	}{
		{
			"result type",
			"{{ for x in nums }}{{ x }}{{ end }}",
			*newLoopData(),
			[]expr.Option{expr.Env(loopData{}), expr.AsInt()},
			"123",
		},
		{
			"undefined variables",
			"{{ for x in single }}{{ undefined }}{{ end }}",
			*newLoopData(),
			[]expr.Option{expr.Env(loopData{}), expr.AllowUndefinedVariables()},
			"<nil>",
		},
		{
			"nil env",
			"{{ for x in [1, 2] }}{{ x }}{{ end }}",
			nil,
			[]expr.Option{expr.Env(nil)},
			"12",
		},
		{
			"no env and nil data",
			"{{ for x in [1, 2] }}{{ x }}{{ end }}",
			nil,
			nil,
			"12",
		},
		{
			"no env and nil map data",
			"{{ for x in [1, 2] }}{{ x }}{{ end }}",
			map[string]any(nil),
			nil,
			"12",
		},
		{
			"no env and data that is a pointer to a pointer",
			"{{ for x in single }}{{ name }}{{ end }}",
			new(newLoopData()),
			nil,
			"n",
		},
		{
			"no env and data that is not a struct or a map",
			"{{ for x in [1, 2] }}{{ x }}{{ end }}",
			42,
			nil,
			"12",
		},
		{
			"map of types",
			"{{ for x in nums }}{{ x + n }}{{ end }}",
			map[string]any{"nums": []int{1, 2}, "n": 1},
			[]expr.Option{expr.Env(typed)},
			"23",
		},
		{
			"map of types with extra names",
			"{{ for x in nums }}{{ x }}{{ extra }}{{ end }}",
			map[string]any{"nums": []int{1}, "extra": "e"},
			[]expr.Option{expr.Env(types.Map{"nums": types.TypeOf([]int{}), types.Extra: types.Any})},
			"1e",
		},
		{
			"variable shadows a function of the options",
			"{{ for double in nums }}{{ double }}{{ end }}{{ double(2) }}",
			*newLoopData(),
			[]expr.Option{expr.Env(loopData{}), expr.Function("double", func(args ...any) (any, error) {
				return args[0].(int) * 2, nil
			}, new(func(int) int))},
			"1234",
		},
		{
			"operator from a method of the env",
			"{{ for it in items }}{{ it + it }}{{ end }}",
			*newLoopData(),
			[]expr.Option{expr.Env(loopData{}), expr.Operator("+", "Join")},
			"aabb",
		},
		{
			"method of a map type",
			"{{ for x in [0] }}{{ Title() }}{{ end }}",
			namedMap{},
			nil,
			"t",
		},
		{
			"method of a map type that is the env",
			"{{ for x in [0] }}{{ Title() }}{{ end }}",
			namedMap{},
			[]expr.Option{expr.Env(namedMap{})},
			"t",
		},
		{
			"method of a map type and a key with its name",
			"{{ Title() }}{{ for x in [0] }}{{ Title() }}{{ end }}",
			namedMap{"Title": "key"},
			nil,
			"tt",
		},
		{
			"field of a nil embedded pointer",
			"{{ for x in single }}{{ Deep }}{{ end }}",
			*newLoopData(),
			[]expr.Option{expr.Env(loopData{})},
			"<nil>",
		},
		{
			"map with keys of an interface type",
			"{{ for k, v in mixed }}{{ k }}{{ v }}{{ end }}",
			loopData{Mixed: map[any]int{"b": 2, "a": 1}},
			[]expr.Option{expr.Env(loopData{})},
			"a1b2",
		},
		{
			"map with integer keys of mixed sizes",
			"{{ for k, v in mixed }}{{ k }}{{ v }} {{ end }}",
			loopData{Mixed: map[any]string{int8(10): "j", 2: "b", int64(-1): "n"}},
			[]expr.Option{expr.Env(loopData{})},
			"-1n 2b 10j ",
		},
		{
			"map with unsigned keys of mixed sizes",
			"{{ for v in mixed }}{{ v }}{{ end }}",
			loopData{Mixed: map[any]int{uint8(200): 2, uint16(300): 3, uint32(1): 1, uintptr(400): 4}},
			[]expr.Option{expr.Env(loopData{})},
			"1234",
		},
		{
			"map with float32 keys",
			"{{ for v in mixed }}{{ v }}{{ end }}",
			loopData{Mixed: map[float32]int{2.5: 2, 10.5: 3, 0.5: 1}},
			[]expr.Option{expr.Env(loopData{})},
			"123",
		},
		{
			"empty map with keys of an interface type",
			"a{{ for k, v in mixed }}b{{ end }}c",
			loopData{Mixed: map[any]int{}},
			[]expr.Option{expr.Env(loopData{})},
			"ac",
		},
		{
			"map with NaN keys",
			"{{ for v in mixed }}{{ v }}{{ end }}",
			loopData{Mixed: map[float64]int{math.NaN(): 0, math.NaN(): 0, math.NaN(): 0, 1: 1, 2: 2, 3: 3, 4: 4, 5: 5}},
			[]expr.Option{expr.Env(loopData{})},
			"00012345",
		},
		{
			"body removes an entry of the map",
			`{{ for k, v in ages }}{{ k }}{{ v }}{{ Drop("cy") }} {{ end }}{{ len(ages) }}`,
			*newLoopData(),
			[]expr.Option{expr.Env(loopData{})},
			"al1 bob2 cy3 2",
		},
		{
			"map with another value type",
			"{{ for x in nums }}{{ len(nums) }}{{ end }}",
			map[string][]int{"nums": {1, 2}},
			[]expr.Option{expr.Env(map[string][]int{"nums": nil})},
			"22",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := execute(t, tt.text, tt.data, tt.opts...)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got  %q\nwant %q", got, tt.want)
			}
		})
	}
}

func TestLoopDataError(t *testing.T) {
	tests := []struct {
		name string
		text string
		opts []expr.Option
		want string
	}{
		{
			"nil env",
			"{{ for x in [1, 2] }}{{ y }}{{ end }}",
			[]expr.Option{expr.Env(nil)},
			"unknown name y (t:1:25)\n | {{ for x in [1, 2] }}{{ y }}{{ end }}\n | ........................^",
		},
		{
			"map of types",
			"{{ for x in nums }}{{ x + s }}{{ end }}",
			[]expr.Option{expr.Env(types.Map{"nums": types.TypeOf([]int{}), "s": types.String})},
			"invalid operation: + (mismatched types int and string) (t:1:25)\n | {{ for x in nums }}{{ x + s }}{{ end }}\n | ........................^",
		},
		{
			"map with a struct value",
			"{{ for it in items }}{{ it.titel }}{{ end }}",
			[]expr.Option{expr.Env(map[string]any{"items": []item{}})},
			"type template.item has no field titel (t:1:28)\n | {{ for it in items }}{{ it.titel }}{{ end }}\n | ...........................^",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New("t", tt.opts...).Parse(tt.text)
			if err == nil {
				t.Fatal("no error")
			}
			if err.Error() != tt.want {
				t.Errorf("got\n%s\nwant\n%s", err, tt.want)
			}
		})
	}
}
