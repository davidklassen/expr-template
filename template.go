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
	parsed bool
	text   string
	nodes  []node
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

// Name returns the name of the template.
func (t *Template) Name() string {
	return t.name
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

// Execute applies the template to data and writes the output to w. The
// expressions are evaluated with data as their env. Execute prints the value of
// each expression as [fmt.Print] does.
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

func (t *Template) exec(w io.Writer, nodes []node, data any) error {
	for _, n := range nodes {
		switch n := n.(type) {
		case *textNode:
			if _, err := io.WriteString(w, n.text); err != nil {
				return err
			}
		case *actionNode:
			v, err := expr.Run(n.prog, data)
			if err != nil {
				return exprError(t.name, t.text, n.at, n.start, err)
			}
			if _, err := fmt.Fprint(w, v); err != nil {
				return err
			}
		case *ifNode:
			v, err := expr.Run(n.prog, data)
			if err != nil {
				return exprError(t.name, t.text, n.at, n.start, err)
			}
			cond, ok := v.(bool)
			if !ok {
				return newError(t.name, t.text, n.start, conditionMessage(typeName(v)))
			}
			body := n.elseBody
			if cond {
				body = n.body
			}
			if err := t.exec(w, body, data); err != nil {
				return err
			}
		case *forNode:
			if err := t.execFor(w, n, data); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *Template) execFor(w io.Writer, n *forNode, data any) error {
	v, err := expr.Run(n.prog, data)
	if err != nil {
		return exprError(t.name, t.text, n.at, n.start, err)
	}
	switch coll := reflect.ValueOf(v); coll.Kind() {
	case reflect.Slice, reflect.Array:
		loop := newLoopEnv(data)
		for i := range coll.Len() {
			if n.key != "" {
				loop[n.key] = i
			}
			loop[n.elem] = coll.Index(i).Interface()
			if err := t.exec(w, n.body, loop); err != nil {
				return err
			}
		}
		return nil
	case reflect.Map:
		entries, ok := sortedEntries(coll)
		if !ok {
			return newError(t.name, t.text, n.start, sortMessage(typeName(v)))
		}
		loop := newLoopEnv(data)
		for _, e := range entries {
			if n.key != "" {
				loop[n.key] = e.key.Interface()
			}
			loop[n.elem] = e.value.Interface()
			if err := t.exec(w, n.body, loop); err != nil {
				return err
			}
		}
		return nil
	}
	return newError(t.name, t.text, n.start, loopMessage(typeName(v)))
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
