package template

import (
	"github.com/expr-lang/expr/file"
	"github.com/expr-lang/expr/vm"
)

// A Node is an element of the parse tree of a template. It is a [*TextNode], an
// [*ActionNode], an [*IfNode], or a [*ForNode].
type Node interface {
	// Location returns the position of the node in the template text, in
	// characters.
	Location() file.Location
}

type base struct {
	loc file.Location
}

// Location returns the position in the template text, in characters.
func (n *base) Location() file.Location {
	return n.loc
}

// A TextNode is text between actions.
type TextNode struct {
	base
	Text string
}

// An ActionNode is an action that prints the value of an expression.
type ActionNode struct {
	base
	Expr *Expr
}

// An IfNode is an if action with its branches. Its location is the location of
// the if action itself, from "{{" to "}}".
type IfNode struct {
	base
	Cond     *Expr
	Body     []Node
	ElseBody []Node // for an else if, one IfNode
}

// A ForNode is a for action with its body. Its location is the location of the
// for action itself, from "{{" to "}}".
type ForNode struct {
	base
	Key        string // variable for the index or the map key, "" if not declared
	Elem       string // variable for the element
	Collection *Expr
	Body       []Node
}

// An Expr is a compiled expression of a template. Its location is the location
// of the source of Program.
type Expr struct {
	base
	Program *vm.Program
}
