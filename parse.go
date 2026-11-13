package template

import (
	"errors"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/file"
	"github.com/expr-lang/expr/parser/lexer"
	"github.com/expr-lang/expr/vm"
)

type node interface {
	pos() int
}

type textNode struct {
	at   int
	text string
}

func (n *textNode) pos() int { return n.at }

type actionNode struct {
	at   int
	prog *vm.Program
}

func (n *actionNode) pos() int { return n.at }

const (
	leftDelim  = "{{"
	rightDelim = "}}"
	spaces     = " \t\r\n"
)

func isSpace(c byte) bool {
	return strings.IndexByte(spaces, c) >= 0
}

func parse(text string, opts []expr.Option) ([]node, error) {
	var nodes []node
	i := 0
	for {
		j := strings.Index(text[i:], leftDelim)
		if j < 0 {
			break
		}
		at := i + j
		src := text[at+len(leftDelim):]
		end, err := actionEnd(src)
		if err != nil {
			return nil, err
		}
		before := text[i:at]
		body := src[:end]
		next := at + len(leftDelim) + end + len(rightDelim)
		if len(body) > 1 && body[0] == '-' && isSpace(body[1]) {
			before = strings.TrimRight(before, spaces)
			body = body[1:]
		}
		if n := len(body); n > 1 && body[n-1] == '-' && isSpace(body[n-2]) {
			body = body[:n-1]
			next = len(text) - len(strings.TrimLeft(text[next:], spaces))
		}
		if before != "" {
			nodes = append(nodes, &textNode{at: i, text: before})
		}
		prog, err := expr.Compile(body, opts...)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, &actionNode{at: at, prog: prog})
		i = next
	}
	if i < len(text) {
		nodes = append(nodes, &textNode{at: i, text: text[i:]})
	}
	return nodes, nil
}

// actionEnd returns the byte offset in src of the "}}" that ends the
// action. It scans src with the expr lexer, because "}}" does not end
// the action inside a string or a comment, or when it closes a map.
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
			return 0, errors.New("unclosed action")
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
