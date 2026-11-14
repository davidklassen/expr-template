package template

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/file"
	"github.com/expr-lang/expr/parser/lexer"
	"github.com/expr-lang/expr/parser/utils"
	"github.com/expr-lang/expr/vm"
)

// A node is a *textNode, an *actionNode, or an *ifNode.
type node any

type textNode struct {
	text string
}

// at is the byte offset of the "{{" in the text of the template. start
// is the byte offset of the expression.
type actionNode struct {
	at    int
	start int
	prog  *vm.Program
}

type ifNode struct {
	at       int
	start    int
	prog     *vm.Program
	body     []node
	elseBody []node
}

const (
	leftDelim  = "{{"
	rightDelim = "}}"
)

type parser struct {
	name string
	text string
	opts []expr.Option
	pos  int
}

func parse(name, text string, opts []expr.Option) ([]node, error) {
	p := &parser{name: name, text: text, opts: opts}
	nodes, stop, err := p.list()
	if err != nil {
		return nil, err
	}
	if stop != nil {
		return nil, p.error(stop.at, fmt.Sprintf("unexpected {{%s}}", stop.tokens[0].Value))
	}
	return nodes, nil
}

// list parses nodes up to the next else action or end action, or up to
// the end of the text. It returns the action that stopped it, or nil at
// the end of the text.
func (p *parser) list() ([]node, *action, error) {
	var nodes []node
	for {
		a, ok, err := p.next()
		if err != nil {
			return nil, nil, err
		}
		if a.before != "" {
			nodes = append(nodes, &textNode{text: a.before})
		}
		if !ok {
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
		case tok.Is(lexer.Operator, "else"):
			return nodes, &a, nil
		case tok.Is(lexer.Identifier, "end"):
			if tok := a.tokens[1]; !tok.Is(lexer.EOF) {
				return nil, nil, p.unexpected(a, tok)
			}
			return nodes, &a, nil
		default:
			prog, err := expr.Compile(a.body, p.opts...)
			if err != nil {
				return nil, nil, exprError(p.name, p.text, a.at, a.start, err)
			}
			nodes = append(nodes, &actionNode{at: a.at, start: a.start, prog: prog})
		}
	}
}

// parseIf parses the if action a and the nodes up to its end action.
// cond is the first token of the condition.
func (p *parser) parseIf(a action, cond lexer.Token) (*ifNode, error) {
	if cond.Is(lexer.EOF) {
		return nil, p.error(a.at, "missing condition")
	}
	offset := byteOffset(a.body, cond.From)
	start := a.start + offset
	// The options of the template can set the type of the result, as
	// expr.AsInt does. That type is for the values that the template
	// prints. The condition must be a boolean with any options, so the
	// last option here removes the type that the options before it set.
	prog, err := expr.Compile(a.body[offset:], append(slices.Clip(p.opts), expr.AsKind(reflect.Invalid))...)
	if err != nil {
		return nil, exprError(p.name, p.text, a.at, start, err)
	}
	// expr reports the type interface{} when it cannot tell the type of
	// the condition at compile time. Parse accepts such a condition, and
	// Execute checks that its value is a boolean.
	if t := prog.Node().Type(); t != reflect.TypeFor[bool]() && t.Kind() != reflect.Interface {
		return nil, p.error(start, conditionMessage(t.String()))
	}
	body, stop, err := p.list()
	if err != nil {
		return nil, err
	}
	if stop == nil {
		return nil, p.error(a.at, "unclosed if")
	}
	n := &ifNode{at: a.at, start: start, prog: prog, body: body}
	if stop.tokens[0].Is(lexer.Operator, "else") {
		n.elseBody, err = p.parseElse(*stop)
		if err != nil {
			return nil, err
		}
	}
	return n, nil
}

// parseElse parses the else action a and the nodes up to the end action
// of its if action. It returns the nodes of the else part.
func (p *parser) parseElse(a action) ([]node, error) {
	switch tok := a.tokens[1]; {
	case tok.Is(lexer.EOF):
		nodes, stop, err := p.list()
		if err != nil {
			return nil, err
		}
		if stop == nil {
			return nil, p.error(a.at, "unclosed if")
		}
		if stop.tokens[0].Is(lexer.Operator, "else") {
			return nil, p.error(stop.at, "unexpected {{else}}")
		}
		return nodes, nil
	case tok.Is(lexer.Operator, "if"):
		// An "else if" is treated as an if nested in the else branch.
		// parseIf reads up to the end action, so one end action closes
		// the whole chain.
		n, err := p.parseIf(a, a.tokens[2])
		if err != nil {
			return nil, err
		}
		return []node{n}, nil
	default:
		return nil, p.unexpected(a, tok)
	}
}

func (p *parser) unexpected(a action, tok lexer.Token) *Error {
	return p.error(a.start+byteOffset(a.body, tok.From), fmt.Sprintf("unexpected token %v", tok))
}

func (p *parser) error(pos int, message string) *Error {
	return newError(p.name, p.text, pos, message)
}

func conditionMessage(typ string) string {
	return fmt.Sprintf("non-bool expression (type %s) used as condition", typ)
}

// An action is the source of one action. at is the byte offset of its
// "{{" in the text of the template. body is the text between the
// delimiters without the trim markers, and start is its byte offset.
// tokens are the tokens of body. The last token is always the EOF
// token. before is the text between the action before it and this
// action.
type action struct {
	before string
	at     int
	start  int
	body   string
	tokens []lexer.Token
}

// next returns the next action and moves p after it. After the last
// action, it returns false, and the action has only the field before:
// the rest of the text.
func (p *parser) next() (action, bool, error) {
	j := strings.Index(p.text[p.pos:], leftDelim)
	if j < 0 {
		a := action{before: p.text[p.pos:]}
		p.pos = len(p.text)
		return a, false, nil
	}
	at := p.pos + j
	start := at + len(leftDelim)
	src := p.text[start:]
	end, err := actionEnd(src)
	if err != nil {
		// The lexer can fail in the text after an action that has no
		// "}}", so the position of err can be far from the cause.
		return action{}, false, p.error(at, exprMessage(err))
	}
	if end < 0 {
		return action{}, false, p.error(at, "unclosed action")
	}
	before := p.text[p.pos:at]
	body := src[:end]
	next := start + end + len(rightDelim)
	if rest, ok := strings.CutPrefix(body, "-"); ok {
		if r, _ := utf8.DecodeRuneInString(rest); utils.IsSpace(r) {
			before = strings.TrimRightFunc(before, utils.IsSpace)
			body = rest
			start++
		}
	}
	if rest, ok := strings.CutSuffix(body, "-"); ok {
		if r, _ := utf8.DecodeLastRuneInString(rest); utils.IsSpace(r) {
			body = rest
			next = len(p.text) - len(strings.TrimLeftFunc(p.text[next:], utils.IsSpace))
		}
	}
	if strings.TrimFunc(body, utils.IsSpace) == "" {
		return action{}, false, p.error(at, "empty action")
	}
	tokens, err := lexer.Lex(file.NewSource(body))
	if err != nil {
		return action{}, false, p.error(at, exprMessage(err))
	}
	p.pos = next
	return action{before: before, at: at, start: start, body: body, tokens: tokens}, true, nil
}

// actionEnd returns the byte offset in src of the "}}" that ends the
// action, or -1 if src has none. It scans src with the expr lexer,
// because "}}" does not end the action inside a string or a comment, or
// when it closes a map.
func actionEnd(src string) (int, error) {
	// Do not reuse one lexer for all actions. Reset has a bug in expr: it
	// keeps the position from the previous source.
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

// byteOffset returns the byte offset in s of its nth rune, counting
// from 0. The expr lexer counts positions in runes.
func byteOffset(s string, n int) int {
	for i := range s {
		if n == 0 {
			return i
		}
		n--
	}
	return len(s)
}
