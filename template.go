package template

import (
	"fmt"
	"io"
	"slices"

	"github.com/expr-lang/expr"
)

// Template is a parsed template.
type Template struct {
	name   string
	opts   []expr.Option
	parsed bool
	nodes  []node
}

// New returns a new template with the given name. The options apply to
// every expression of the template when [Template.Parse] compiles it.
//
// Use [expr.Env] to declare the type of the data that [Template.Execute]
// receives. Parse then reports unknown names and type errors. Without
// expr.Env, an unknown name has the value nil.
func New(name string, opts ...expr.Option) *Template {
	return &Template{name: name, opts: slices.Clone(opts)}
}

// Name returns the name of the template.
func (t *Template) Name() string {
	return t.name
}

// Parse parses text as the body of the template and compiles its
// expressions. It replaces the body from an earlier call.
//
// If an error occurs, Parse returns nil and the error, and the template
// keeps its earlier body. The error is an [*Error]. It has the position
// of the problem in text.
func (t *Template) Parse(text string) (*Template, error) {
	nodes, err := parse(t.name, text, t.opts)
	if err != nil {
		return nil, err
	}
	t.parsed = true
	t.nodes = nodes
	return t, nil
}

// Execute applies the template to data and writes the output to w.
// The expressions see data as their env. Execute prints the value of
// each action as [fmt.Print] does.
//
// If an error occurs, Execute stops, but it can have written part of
// the output to w. Execute returns an error if the template is not
// parsed.
//
// A template may be executed safely in parallel. If parallel executions
// share a Writer, the output may be interleaved.
func (t *Template) Execute(w io.Writer, data any) error {
	if !t.parsed {
		return fmt.Errorf("template %q is not parsed", t.name)
	}
	for _, n := range t.nodes {
		switch n := n.(type) {
		case *textNode:
			if _, err := io.WriteString(w, n.text); err != nil {
				return err
			}
		case *actionNode:
			v, err := expr.Run(n.prog, data)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprint(w, v); err != nil {
				return err
			}
		}
	}
	return nil
}

// Must is a helper that wraps a call to a function returning
// ([*Template], error) and panics if the error is non-nil. It is
// intended for use in variable initializations such as
//
//	var t = template.Must(template.New("name").Parse("text"))
func Must(t *Template, err error) *Template {
	if err != nil {
		panic(err)
	}
	return t
}
