package template

import (
	"strings"
	"unicode/utf8"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/file"
	"github.com/expr-lang/expr/parser/lexer"
	"github.com/expr-lang/expr/parser/utils"
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
	at    int
	start int
	prog  *vm.Program
}

func (n *actionNode) pos() int { return n.at }

const (
	leftDelim  = "{{"
	rightDelim = "}}"
)

func parse(name, text string, opts []expr.Option) ([]node, error) {
	var nodes []node
	i := 0
	for {
		j := strings.Index(text[i:], leftDelim)
		if j < 0 {
			break
		}
		at := i + j
		start := at + len(leftDelim)
		src := text[start:]
		end, err := actionEnd(src)
		if err != nil {
			// The lexer can fail in the text after an action that has no
			// "}}", so the position of err can be far from the cause.
			return nil, newError(name, text, at, exprMessage(err))
		}
		if end < 0 {
			return nil, newError(name, text, at, "unclosed action")
		}
		before := text[i:at]
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
				next = len(text) - len(strings.TrimLeftFunc(text[next:], utils.IsSpace))
			}
		}
		if strings.TrimFunc(body, utils.IsSpace) == "" {
			return nil, newError(name, text, at, "empty action")
		}
		if before != "" {
			nodes = append(nodes, &textNode{at: i, text: before})
		}
		i = next
		if isComment(body) {
			continue
		}
		prog, err := expr.Compile(body, opts...)
		if err != nil {
			return nil, exprError(name, text, at, start, err)
		}
		nodes = append(nodes, &actionNode{at: at, start: start, prog: prog})
	}
	if i < len(text) {
		nodes = append(nodes, &textNode{at: i, text: text[i:]})
	}
	return nodes, nil
}

// isComment reports whether body has comments and white space only.
func isComment(body string) bool {
	l := lexer.New()
	l.Reset(file.NewSource(body))
	tok, err := l.Next()
	return err == nil && tok.Is(lexer.EOF)
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
