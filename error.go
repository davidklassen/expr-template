package template

import (
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/expr-lang/expr/file"
)

// An Error records a problem in a template and its position.
type Error struct {
	Name    string // name of the template
	Line    int    // line in the template, from 1
	Column  int    // column in the line, in characters, from 1
	Message string

	snippet string
	cause   error
}

// Error returns the message, the position, and the snippet.
func (e *Error) Error() string {
	return fmt.Sprintf("%s (%s:%d:%d)%s", e.Message, e.Name, e.Line, e.Column, e.snippet)
}

// Unwrap returns the cause of e, or nil if it has none. A cause is an
// error from outside the template, such as an error that a function
// returns.
func (e *Error) Unwrap() error {
	return e.cause
}

// newError returns an error at the byte offset pos of text. expr
// computes the line, the column, and the snippet, so that they have the
// same form as in an expr error.
func newError(name, text string, pos int, message string) *Error {
	fe := &file.Error{Location: file.Location{From: utf8.RuneCountInString(text[:pos])}}
	fe.Bind(file.NewSource(text))
	return &Error{
		Name:    name,
		Line:    fe.Line,
		Column:  fe.Column + 1,
		Message: message,
		snippet: fe.Snippet,
	}
}

// exprError returns an error for the action at the byte offset at of
// text, whose expression starts at start. It points to the position
// that err has in the expression, or to the action if err has none.
func exprError(name, text string, at, start int, err error) *Error {
	fe, ok := errors.AsType[*file.Error](err)
	if !ok {
		return newError(name, text, at, err.Error())
	}
	e := newError(name, text, start+byteOffset(text[start:], fe.From), fe.Message)
	e.cause = fe.Prev
	return e
}

func exprMessage(err error) string {
	if fe, ok := errors.AsType[*file.Error](err); ok {
		return fe.Message
	}
	return err.Error()
}
