package template

import (
	"fmt"
	"io"
	"reflect"
	"slices"

	"github.com/expr-lang/expr"
)

// Template is a parsed template.
type Template struct {
	name   string
	opts   []expr.Option
	print  func(any) (string, error)
	parsed bool
	text   string
	nodes  []Node
}

// New returns a new template with the given name. The options apply to every
// expression of the template when [Template.Parse] compiles it.
//
// Use [expr.Env] to declare the type of the data that [Template.Execute]
// receives. Parse then reports type errors and unknown names. Without expr.Env,
// Parse does not check the names.
func New(name string, opts ...expr.Option) *Template {
	return &Template{name: name, opts: slices.Clone(opts)}
}

func printValue(v any) (string, error) {
	return fmt.Sprint(v), nil
}

// Name returns the name of the template.
func (t *Template) Name() string {
	return t.name
}

// Print overrides the function that converts the value of an action to text.
// The default is [fmt.Sprint]. If f returns an error, Execute stops and returns
// an [*Error] that wraps it. If the template is executed in parallel, f must be
// safe for concurrent use.
func (t *Template) Print(f func(v any) (string, error)) *Template {
	t.print = f
	return t
}

// Parse parses text as the body of the template and compiles its expressions.
//
// If an error occurs, Parse returns nil and the error. The error is an [*Error]
// with the position of the problem in text.
func (t *Template) Parse(text string) (*Template, error) {
	nodes, err := parse(t.name, text, t.opts)
	if err != nil {
		return nil, err
	}
	t.parsed = true
	t.text = text
	t.nodes = nodes
	return t, nil
}

// Nodes returns the parse tree of the template: its top-level nodes in order.
// The tree must not be modified.
func (t *Template) Nodes() []Node {
	return t.nodes
}

// Execute applies the template to data and writes the output to w. The
// expressions are evaluated with data as their env. Execute prints the value of
// each expression as [fmt.Print] does. To override the printing function, use
// [Template.Print].
//
// If an error occurs, Execute stops, but it can have written part of the output
// to w. An error in an action is an [*Error] with the position of the problem
// in the template. If a write to w fails, Execute returns that error unchanged.
//
// A template may be executed safely in parallel. If parallel executions share a
// Writer, the output may be interleaved.
func (t *Template) Execute(w io.Writer, data any) error {
	if !t.parsed {
		return fmt.Errorf("template %q is not parsed", t.name)
	}
	return t.exec(w, t.nodes, data)
}

func (t *Template) exec(w io.Writer, nodes []Node, data any) error {
	for _, n := range nodes {
		switch n := n.(type) {
		case *TextNode:
			if _, err := io.WriteString(w, n.Text); err != nil {
				return err
			}
		case *ActionNode:
			v, err := expr.Run(n.Expr.Program, data)
			if err != nil {
				return exprError(t.name, t.text, n.loc.From, n.Expr.loc.From, err)
			}
			print := t.print
			if print == nil {
				print = printValue
			}
			s, err := print(v)
			if err != nil {
				e := newError(t.name, t.text, n.loc.From, err.Error())
				e.cause = err
				return e
			}
			if _, err := io.WriteString(w, s); err != nil {
				return err
			}
		case *IfNode:
			v, err := expr.Run(n.Cond.Program, data)
			if err != nil {
				return exprError(t.name, t.text, n.loc.From, n.Cond.loc.From, err)
			}
			cond, ok := v.(bool)
			if !ok {
				return newError(t.name, t.text, n.Cond.loc.From, conditionMessage(typeName(v)))
			}
			body := n.ElseBody
			if cond {
				body = n.Body
			}
			if err := t.exec(w, body, data); err != nil {
				return err
			}
		case *ForNode:
			if err := t.execFor(w, n, data); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *Template) execFor(w io.Writer, n *ForNode, data any) error {
	v, err := expr.Run(n.Collection.Program, data)
	if err != nil {
		return exprError(t.name, t.text, n.loc.From, n.Collection.loc.From, err)
	}
	switch coll := reflect.ValueOf(v); coll.Kind() {
	case reflect.Slice, reflect.Array:
		loop := newLoopEnv(data)
		for i := range coll.Len() {
			if n.Key != "" {
				loop[n.Key] = i
			}
			loop[n.Elem] = coll.Index(i).Interface()
			if err := t.exec(w, n.Body, loop); err != nil {
				return err
			}
		}
		return nil
	case reflect.Map:
		entries, ok := sortedEntries(coll)
		if !ok {
			return newError(t.name, t.text, n.Collection.loc.From, sortMessage(typeName(v)))
		}
		loop := newLoopEnv(data)
		for _, e := range entries {
			if n.Key != "" {
				loop[n.Key] = e.key.Interface()
			}
			loop[n.Elem] = e.value.Interface()
			if err := t.exec(w, n.Body, loop); err != nil {
				return err
			}
		}
		return nil
	}
	return newError(t.name, t.text, n.Collection.loc.From, loopMessage(typeName(v)))
}

// Must is a helper that wraps a call to a function returning ([*Template],
// error) and panics if the error is non-nil. It is intended for use in variable
// initializations such as
//
//	var t = template.Must(template.New("name").Parse("text"))
func Must(t *Template, err error) *Template {
	if err != nil {
		panic(err)
	}
	return t
}
