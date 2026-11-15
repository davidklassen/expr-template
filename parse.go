package template

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/checker/nature"
	"github.com/expr-lang/expr/file"
	"github.com/expr-lang/expr/parser/lexer"
	"github.com/expr-lang/expr/parser/utils"
)

const (
	leftDelim  = "{{"
	rightDelim = "}}"
)

type parser struct {
	name string
	text string
	// opts are the options of the template. Inside a loop body, they also
	// have one withLoopVar option for each loop variable in scope.
	opts []expr.Option
	pos  int // byte offset of the text that is not parsed yet
	char int // pos as a character offset
}

func parse(name, text string, opts []expr.Option) ([]Node, error) {
	p := &parser{name: name, text: text, opts: opts}
	nodes, stop, err := p.parseNodes()
	if err != nil {
		return nil, err
	}
	if stop != nil {
		return nil, p.error(stop.loc.From, fmt.Sprintf("unexpected {{%s}}", stop.tokens[0].Value))
	}
	return nodes, nil
}

// parseNodes parses nodes up to the next else action or end action, or up to
// the end of the text. It returns the action that stopped it, or nil at the end
// of the text.
func (p *parser) parseNodes() ([]Node, *action, error) {
	var nodes []Node
	for {
		from := p.char
		text, a, err := p.next()
		if err != nil {
			return nil, nil, err
		}
		if text != "" {
			loc := file.Location{From: from, To: from + utf8.RuneCountInString(text)}
			nodes = append(nodes, &TextNode{base: base{loc}, Text: text})
		}
		if a == nil {
			return nodes, nil, nil
		}
		switch tok := a.tokens[0]; {
		case tok.Is(lexer.EOF):
			// The action has only comments.
		case tok.Is(lexer.Operator, "if"):
			n, err := p.parseIf(a, a.tokens[1])
			if err != nil {
				return nil, nil, err
			}
			nodes = append(nodes, n)
		case tok.Is(lexer.Identifier, "for"):
			n, err := p.parseFor(a)
			if err != nil {
				return nil, nil, err
			}
			nodes = append(nodes, n)
		case tok.Is(lexer.Operator, "else"):
			return nodes, a, nil
		case tok.Is(lexer.Identifier, "end"):
			if tok := a.tokens[1]; !tok.Is(lexer.EOF) {
				return nil, nil, p.unexpected(a, tok)
			}
			return nodes, a, nil
		default:
			e, err := p.compile(a, tok, p.opts)
			if err != nil {
				return nil, nil, err
			}
			nodes = append(nodes, &ActionNode{base: base{a.loc}, Expr: e})
		}
	}
}

// parseIf parses the if action a and the nodes up to its end action. cond is
// the first token of the condition.
func (p *parser) parseIf(a *action, first lexer.Token) (*IfNode, error) {
	if first.Is(lexer.EOF) {
		return nil, p.error(a.loc.From, "missing condition")
	}
	cond, err := p.compile(a, first, p.plainOpts())
	if err != nil {
		return nil, err
	}
	// expr reports the type interface{} when it cannot tell the type of the
	// condition at compile time. Parse accepts such a condition, and Execute
	// checks that its value is a boolean.
	if t := cond.Program.Node().Type(); t != reflect.TypeFor[bool]() && t.Kind() != reflect.Interface {
		return nil, p.error(cond.loc.From, conditionMessage(t.String()))
	}
	body, stop, err := p.parseNodes()
	if err != nil {
		return nil, err
	}
	if stop == nil {
		return nil, p.error(a.loc.From, "unclosed if")
	}
	n := &IfNode{base: base{a.loc}, Cond: cond, Body: body}
	if stop.tokens[0].Is(lexer.Operator, "else") {
		n.ElseBody, err = p.parseElse(stop)
		if err != nil {
			return nil, err
		}
	}
	return n, nil
}

// compile compiles the expression of the action a that starts at the token
// first and ends at the end of the action.
func (p *parser) compile(a *action, first lexer.Token, opts []expr.Option) (*Expr, error) {
	src := a.src[byteOffset(a.src, first.From):]
	start := a.start + first.From
	prog, err := expr.Compile(src, opts...)
	if err != nil {
		return nil, exprError(p.name, p.text, a.loc.From, start, err)
	}
	loc := file.Location{From: start, To: start + utf8.RuneCountInString(src)}
	return &Expr{base: base{loc}, Program: prog}, nil
}

// plainOpts returns the options for compiling a condition or a collection. A
// template option such as expr.AsInt sets the result type of every expression.
// It is meant for the values that the template prints, so plainOpts adds
// AsKind(reflect.Invalid), which resets the result type.
func (p *parser) plainOpts() []expr.Option {
	return append(slices.Clip(p.opts), expr.AsKind(reflect.Invalid))
}

// parseFor parses the for action a and the nodes up to its end action.
func (p *parser) parseFor(a *action) (*ForNode, error) {
	var key lexer.Token
	elem := a.tokens[1]
	if err := p.checkVariable(a, elem); err != nil {
		return nil, err
	}
	i := 2
	if a.tokens[i].Is(lexer.Operator, ",") {
		key, elem = elem, a.tokens[i+1]
		if err := p.checkVariable(a, elem); err != nil {
			return nil, err
		}
		if elem.Value == key.Value {
			return nil, p.error(a.start+elem.From, "cannot redeclare "+elem.Value)
		}
		i += 2
	}
	if in := a.tokens[i]; !in.Is(lexer.Operator, "in") {
		return nil, p.unexpected(a, in)
	}
	first := a.tokens[i+1]
	if first.Is(lexer.EOF) {
		return nil, p.error(a.loc.From, "missing collection")
	}
	collection, err := p.compile(a, first, p.plainOpts())
	if err != nil {
		return nil, err
	}
	// The loop variables get their types from the nature of the collection,
	// which is the type information of the expr checker. It has the element
	// type even when the Go type of the collection is []any, as for the result
	// of the map builtin. If expr does not know the type of the collection, the
	// variables are untyped, and Execute checks the value of the collection.
	var c nature.Cache
	var keyType, elemType nature.Nature
	switch coll := collection.Program.Node().Nature(); {
	case coll.IsArray():
		keyType, elemType = c.FromType(reflect.TypeFor[int]()), coll.Elem(&c)
	case coll.IsMap():
		if !canOrder(coll.Type.Key()) {
			return nil, p.error(collection.loc.From, sortMessage(coll.Type.String()))
		}
		keyType, elemType = coll.Key(&c), coll.Elem(&c)
	case coll.Type == nil || coll.Kind == reflect.Interface:
	default:
		return nil, p.error(collection.loc.From, loopMessage(coll.Type.String()))
	}
	opts := p.opts
	p.opts = append(slices.Clip(opts), withLoopVar(elem.Value, elemType))
	if key.Value != "" {
		p.opts = append(p.opts, withLoopVar(key.Value, keyType))
	}
	body, stop, err := p.parseNodes()
	p.opts = opts
	if err != nil {
		return nil, err
	}
	if stop == nil {
		return nil, p.error(a.loc.From, "unclosed for")
	}
	if stop.tokens[0].Is(lexer.Operator, "else") {
		return nil, p.error(stop.loc.From, "unexpected {{else}}")
	}
	return &ForNode{base: base{a.loc}, Key: key.Value, Elem: elem.Value, Collection: collection, Body: body}, nil
}

// checkVariable checks that the token tok of the for action a can be a loop
// variable.
func (p *parser) checkVariable(a *action, tok lexer.Token) error {
	if tok.Is(lexer.EOF) {
		return p.error(a.loc.From, "missing variable")
	}
	if !tok.Is(lexer.Identifier) {
		return p.unexpected(a, tok)
	}
	// These identifiers are keywords of this package or have a fixed meaning in
	// expr. The lexer returns if, else, and in as operators, so the check above
	// rejects them.
	switch tok.Value {
	case "for", "end", "nil", "true", "false", "$env":
		return p.unexpected(a, tok)
	}
	return nil
}

// parseElse parses the else action a and the nodes up to the end action of its
// if action. It returns the nodes of the else part.
func (p *parser) parseElse(a *action) ([]Node, error) {
	switch tok := a.tokens[1]; {
	case tok.Is(lexer.EOF):
		nodes, stop, err := p.parseNodes()
		if err != nil {
			return nil, err
		}
		if stop == nil {
			return nil, p.error(a.loc.From, "unclosed if")
		}
		if stop.tokens[0].Is(lexer.Operator, "else") {
			return nil, p.error(stop.loc.From, "unexpected {{else}}")
		}
		return nodes, nil
	case tok.Is(lexer.Operator, "if"):
		// An "else if" is treated as an if nested in the else branch. parseIf
		// reads up to the end action, so one end action closes the whole chain.
		n, err := p.parseIf(a, a.tokens[2])
		if err != nil {
			return nil, err
		}
		return []Node{n}, nil
	default:
		return nil, p.unexpected(a, tok)
	}
}

func (p *parser) unexpected(a *action, tok lexer.Token) *Error {
	return p.error(a.start+tok.From, fmt.Sprintf("unexpected token %v", tok))
}

// error returns an error at the character offset pos in the template text.
func (p *parser) error(pos int, message string) *Error {
	return newError(p.name, p.text, pos, message)
}

// An action holds the source of one action, from "{{" to "}}".
type action struct {
	loc    file.Location // from the "{{" to the "}}" in the template text
	start  int           // character offset of src in the template text
	src    string        // text between the delimiters, without trim markers
	tokens []lexer.Token // tokens of src; the last one is always EOF
}

// next returns the text up to the next action, and the action. At the end of
// the template, it returns the rest of the text and a nil action.
func (p *parser) next() (string, *action, error) {
	j := strings.Index(p.text[p.pos:], leftDelim)
	if j < 0 {
		text := p.text[p.pos:]
		p.pos = len(p.text)
		return text, nil, nil
	}
	before := p.text[p.pos : p.pos+j]
	rest := p.text[p.pos+j+len(leftDelim):]
	at := p.char + utf8.RuneCountInString(before)
	start := at + len(leftDelim)
	n, err := actionEnd(rest)
	if err != nil {
		// If an action has no "}}", the lexer reads on into the text after it
		// and can fail there, far from the cause. So the error points to the
		// "{{" and not to the position of err.
		return "", nil, p.error(at, exprMessage(err))
	}
	if n < 0 {
		return "", nil, p.error(at, "unclosed action")
	}
	src := rest[:n]
	after := rest[n+len(rightDelim):]
	end := start + utf8.RuneCountInString(src) + len(rightDelim)
	if s, ok := strings.CutPrefix(src, "-"); ok {
		if r, _ := utf8.DecodeRuneInString(s); utils.IsSpace(r) {
			before = strings.TrimRightFunc(before, utils.IsSpace)
			src = s
			start++
		}
	}
	// space is the length of the white space that " -}}" removes from after.
	space := 0
	if s, ok := strings.CutSuffix(src, "-"); ok {
		if r, _ := utf8.DecodeLastRuneInString(s); utils.IsSpace(r) {
			src = s
			space = len(after) - len(strings.TrimLeftFunc(after, utils.IsSpace))
		}
	}
	if strings.TrimFunc(src, utils.IsSpace) == "" {
		return "", nil, p.error(at, "empty action")
	}
	tokens, err := lexer.Lex(file.NewSource(src))
	if err != nil {
		return "", nil, p.error(at, exprMessage(err))
	}
	p.pos = len(p.text) - len(after) + space
	p.char = end + utf8.RuneCountInString(after[:space])
	return before, &action{loc: file.Location{From: at, To: end}, start: start, src: src, tokens: tokens}, nil
}

// actionEnd returns the byte offset in src of the "}}" that ends the action, or
// -1 if src has none. It scans src with the expr lexer, because "}}" does not
// end the action inside a string or a comment, or when it closes a map.
func actionEnd(src string) (int, error) {
	// Do not reuse one lexer for all actions. Reset has a bug in expr: it keeps
	// the position from the previous source.
	l := lexer.New()
	l.Reset(file.NewSource(src))
	depth := 0
	// prev is the rune offset of the last "}" outside a map, or -1.
	prev := -1
	for {
		tok, err := l.Next()
		if err != nil {
			return 0, err
		}
		switch {
		case tok.Is(lexer.EOF):
			return -1, nil
		case tok.Is(lexer.Bracket, "{"):
			depth++
		case tok.Is(lexer.Bracket, "}") && depth > 0:
			depth--
		case tok.Is(lexer.Bracket, "}") && prev >= 0 && prev+1 == tok.From:
			return byteOffset(src, prev), nil
		case tok.Is(lexer.Bracket, "}"):
			prev = tok.From
		}
	}
}

// byteOffset returns the byte offset in s of its nth rune, counting from 0. The
// expr lexer counts positions in runes.
func byteOffset(s string, n int) int {
	for i := range s {
		if n == 0 {
			return i
		}
		n--
	}
	return len(s)
}
