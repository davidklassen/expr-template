package template

import (
	"errors"
	"fmt"
	"reflect"
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

// Unwrap returns the cause of e, or nil if it has none. If a function that the
// template calls returns an error, that error is the cause. An error from the
// printing function is the cause too.
func (e *Error) Unwrap() error {
	return e.cause
}

// newError returns an error that points to the byte offset pos in text. It uses
// file.Error of expr to compute the line, the column, and the snippet, so that
// they look the same as in an expr error.
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

// exprError converts err, an error from expr, to an Error. at is the byte
// offset in text of the "{{" of the action, and start is the byte offset of its
// expression. The Error points to the position that err reports inside the
// expression, or to the "{{" if err has no position.
func exprError(name, text string, at, start int, err error) *Error {
	fe, ok := errors.AsType[*file.Error](err)
	if !ok {
		return newError(name, text, at, err.Error())
	}
	// expr uses the zero value of Location when it has no position for a
	// run-time error. The error then points to the action. A real location is
	// never the zero value: its end is after its start.
	pos := at
	if fe.Location != (file.Location{}) {
		pos = start + byteOffset(text[start:], fe.From)
	}
	e := newError(name, text, pos, fe.Message)
	e.cause = fe.Prev
	return e
}

func exprMessage(err error) string {
	if fe, ok := errors.AsType[*file.Error](err); ok {
		return fe.Message
	}
	return err.Error()
}

func conditionMessage(typ string) string {
	return fmt.Sprintf("non-bool expression (type %s) used as condition", typ)
}

func loopMessage(typ string) string {
	return "cannot loop over " + typ
}

func sortMessage(typ string) string {
	return "cannot sort keys of " + typ
}

// typeName returns the name of the type of v for an error message.
func typeName(v any) string {
	if v == nil {
		return "nil"
	}
	return reflect.TypeOf(v).String()
}
