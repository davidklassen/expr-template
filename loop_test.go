package template

import (
	"fmt"
	"strings"
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

type namedMap map[string]any

func (namedMap) Title() string { return "t" }

type str string

func (s str) String() string { return string(s) }

type loopData struct {
	loopBase
	hidden
	*deep
	Name   string       `expr:"name"`
	Items  []item       `expr:"items"`
	Nums   []int        `expr:"nums"`
	Empty  []int        `expr:"empty"`
	Lists  [][]int      `expr:"lists"`
	Single []int        `expr:"single"`
	Mixed  any          `expr:"mixed"`
	Str    fmt.Stringer `expr:"str"`
	Arr    [2]int       `expr:"arr"`
	Boxed  any          `expr:"boxed"`
	Skip   string       `expr:"-"`
	Plain  string
}

func (d loopData) Greet(s string) string { return "hi " + s }

func (d *loopData) Count() int { return len(d.Items) }

func (d loopData) First() *item { return &d.Items[0] }

func (d loopData) Take(it *item) string { return it.Title }

func (d loopData) Join(a, b item) string { return a.Title + b.Title }

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
		Boxed:    [2]int{6, 7},
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
		"boxed":  d.Boxed,
		"Plain":  d.Plain,
	}
}

// A loopSetup is one way to give a template its env and its data.
type loopSetup struct {
	name    string
	opts    []expr.Option
	data    any
	methods bool // the data has the method Greet
	pointer bool // the data has the method Count
}

func (s loopSetup) hasEnv() bool { return s.opts != nil }

func loopSetups() []loopSetup {
	return []loopSetup{
		{name: "struct env", opts: []expr.Option{expr.Env(loopData{})}, data: *newLoopData(), methods: true},
		{name: "pointer env", opts: []expr.Option{expr.Env(&loopData{})}, data: newLoopData(), methods: true, pointer: true},
		{name: "struct env and pointer data", opts: []expr.Option{expr.Env(loopData{})}, data: newLoopData(), methods: true},
		{name: "map env", opts: []expr.Option{expr.Env(newLoopMap())}, data: newLoopMap()},
		{name: "no env and struct data", data: *newLoopData(), methods: true},
		{name: "no env and pointer data", data: newLoopData(), methods: true, pointer: true},
		{name: "no env and map data", data: newLoopMap()},
	}
}

var loopTests = []struct {
	name string
	text string
	want string
}{
	{"list", "{{ for x in nums }}{{ x }},{{ end }}", "1,2,3,"},
	{"text only", "{{ for x in nums }}a{{ end }}", "aaa"},
	{"empty body", "a{{ for x in nums }}{{ end }}b", "ab"},
	{"nil list", "a{{ for x in empty }}b{{ end }}c", "ac"},
	{"empty list", "a{{ for x in [] }}b{{ end }}c", "ac"},
	{"array literal", "{{ for x in [1, 2] }}{{ x }}{{ end }}", "12"},
	{"array", "{{ for x in arr }}{{ x + 1 }}{{ end }}", "56"},
	{"array of unknown type", "{{ for x in boxed }}{{ x }}{{ end }}", "67"},
	{"range", "{{ for i in 1..3 }}{{ i }}{{ end }}", "123"},
	{"list expression", "{{ for x in filter(nums, # > 1) }}{{ x }}{{ end }}", "23"},
	{"list of unknown type", "{{ for x in mixed }}{{ x }}{{ end }}", "1a"},
	{"variable in an expression", "{{ for x in nums }}{{ x + 1 }}{{ end }}", "234"},
	{"field of the variable", "{{ for it in items }}{{ it.title }};{{ end }}", "a;b;"},
	{"name of the data in the body", "{{ for x in nums }}{{ name }}{{ x }} {{ end }}", "n1 n2 n3 "},
	{"without spaces", "{{for x in nums}}{{x}}{{end}}", "123"},
	{"across lines", "{{ for\nx\nin\nnums }}{{ x }}{{ end }}", "123"},
	{"with comments", "{{ for /* c */ x /* c */ in /* c */ nums }}{{ x }}{{ end /* c */ }}", "123"},
	{"trim", "a {{- for x in nums -}} {{ x }} {{- end -}} b", "a123b"},
	{"if in for", "{{ for x in nums }}{{ if x > 1 }}{{ x }}{{ else }}-{{ end }}{{ end }}", "-23"},
	{"for in if", "{{ if true }}{{ for x in nums }}{{ x }}{{ end }}{{ end }}", "123"},
	{"for after for", "{{ for x in nums }}{{ x }}{{ end }}{{ for x in single }}{{ x }}{{ end }}", "1230"},
	{"nested", "{{ for l in lists }}[{{ for x in l }}{{ x }}{{ end }}]{{ end }}", "[12][3]"},
	{"inner loop sees the outer variable", "{{ for a in nums }}{{ for b in single }}{{ a + b }}{{ end }}{{ end }}", "123"},
	{"variable shadows a name of the data", "{{ for name in nums }}{{ name }}{{ end }}{{ name }}", "123n"},
	{"variable shadows the type of a name of the data", "{{ for name in nums }}{{ name + 1 }}{{ end }}", "234"},
	{"variable shadows an outer variable", "{{ for x in nums }}{{ for x in single }}{{ x }}{{ end }}{{ x }}{{ end }}", "010203"},
	{"list of an inner loop is the outer variable", "{{ for x in lists }}{{ for x in x }}{{ x }}{{ end }}{{ end }}", "123"},
	{"variable named as a function", "{{ for len in nums }}{{ len }}{{ end }}{{ len(nums) }}", "1233"},
	{"embedded field in the body", "{{ for x in single }}{{ id }}{{ end }}", "7"},
	{"field of an unexported embedded struct in the body", "{{ for x in single }}{{ Secret }}{{ end }}", "s"},
	{"field without a tag in the body", "{{ for x in single }}{{ Plain }}{{ end }}", "p"},
	{"interface field in the body", "{{ for x in single }}{{ str.String() }}{{ end }}", "S"},
	{"for is a name in parentheses", "{{ for x in single }}{{ (x) }}{{ end }}", "0"},
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
		t.Run(setup.name, func(t *testing.T) {
			if setup.methods {
				got, err := execute(t, "{{ for x in nums }}{{ Greet(name) }}{{ x }} {{ end }}", setup.data, setup.opts...)
				if err != nil {
					t.Fatal(err)
				}
				if want := "hi n1 hi n2 hi n3 "; got != want {
					t.Errorf("got %q, want %q", got, want)
				}
			}
			if setup.methods {
				got, err := execute(t, "{{ for x in single }}{{ Take(First()) }}{{ end }}", setup.data, setup.opts...)
				if err != nil {
					t.Fatal(err)
				}
				if want := "a"; got != want {
					t.Errorf("got %q, want %q", got, want)
				}
			}
			if setup.pointer {
				got, err := execute(t, "{{ for x in single }}{{ for y in single }}{{ Count() }}{{ end }}{{ end }}", setup.data, setup.opts...)
				if err != nil {
					t.Fatal(err)
				}
				if want := "2"; got != want {
					t.Errorf("got %q, want %q", got, want)
				}
			}
		})
	}
}

// TestLoopEnv checks that an expression gives the same result in the body of a
// loop as outside a loop: the loop env has the names of the env, with the same
// types.
func TestLoopEnv(t *testing.T) {
	exprs := []string{
		"name", "id", "Secret", "Plain", "nums", "len(items)", "items[0].title", "mixed", "str", "str.String()",
		"empty == nil", "name + Plain", "id + 1", "Greet(name)", "Count()",
		// These are errors with an env. They must be the same errors.
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
					// Without an env, expr finds a wrong name only when it
					// runs, and only if the data is a struct. In a loop the
					// data is a map, and the name is nil.
					t.Skip(want)
				}
				if setup.name == "struct env and pointer data" && strings.HasPrefix(e, "Greet") {
					// expr calls a method by its index in the type of the env
					// option. The data is a pointer and has one more method, so
					// expr calls the wrong method outside a loop. A loop binds
					// the method by its name.
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

func TestLoopParseError(t *testing.T) {
	tests := []struct {
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
			"list is an int",
			"a {{ for x in id }}{{ end }}",
			"cannot loop over int (t:1:15)\n | a {{ for x in id }}{{ end }}\n | ..............^",
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
			"wrong field of the variable",
			"a {{ for it in items }}{{ it.titel }}{{ end }}",
			"type template.item has no field titel (t:1:30)\n | a {{ for it in items }}{{ it.titel }}{{ end }}\n | .............................^",
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
			"two variables",
			"a {{ for x, y in nums }}b{{ end }}",
			"unexpected token Operator(\",\") (t:1:11)\n | a {{ for x, y in nums }}b{{ end }}\n | ..........^",
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
			"variable is a number",
			"a {{ for 1 in nums }}b{{ end }}",
			"unexpected token Number(\"1\") (t:1:10)\n | a {{ for 1 in nums }}b{{ end }}\n | .........^",
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
	for _, tt := range tests {
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
			"list is an int",
			"a {{ for x in mixed }}b{{ end }}",
			loopData{Mixed: 5},
			"a ",
			"cannot loop over int (t:1:15)\n | a {{ for x in mixed }}b{{ end }}\n | ..............^",
		},
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
			"list is a map",
			"a {{ for x in mixed }}b{{ end }}",
			loopData{Mixed: map[string]int{}},
			"a ",
			"cannot loop over map[string]int (t:1:15)\n | a {{ for x in mixed }}b{{ end }}\n | ..............^",
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

func TestLoopOptions(t *testing.T) {
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
			"map with another value type",
			"{{ for x in nums }}{{ x }}{{ end }}",
			map[string][]int{"nums": {1, 2}},
			[]expr.Option{expr.Env(map[string][]int{"nums": nil})},
			"12",
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

func TestLoopOptionsError(t *testing.T) {
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
