// Package template implements text templates whose actions are [expr]
// expressions.
//
// A template is text with actions. An action is written between "{{"
// and "}}". Execute copies the text to the output and runs the actions.
// Most actions are an expression, and Execute prints its value:
//
//	t := template.Must(template.New("hello").Parse("Hello, {{ name }}!\n"))
//	err := t.Execute(os.Stdout, map[string]any{"name": "Alice"})
//
// An if action and an end action enclose a part of the template. The
// expression after the word if is the condition. Execute runs the
// enclosed part only when the condition is true:
//
//	{{ if len(items) > 0 }}You have {{ len(items) }} items.{{ end }}
//
// If actions can be nested.
//
// The condition must be a boolean. Any other value, including nil, is
// an error. Parse reports the error when it knows the type of the
// condition. Otherwise Execute reports it. To test whether a value is
// set, compare it with nil: "{{ if name != nil }}".
//
// An action that starts with the word if or end is always an if action
// or an end action. To start an expression with one of these words, put
// the expression in parentheses. "{{ (end) }}" prints the value of the
// name end. "{{ (if a { 1 } else { 2 }) }}" is the if expression of
// expr.
//
// A trim marker removes the white space around an action. A minus
// sign and white space after "{{" remove all white space before the
// action. White space and a minus sign before "}}" remove all white
// space after it:
//
//	"a  {{- 1 -}}  b" prints "a1b"
//
// White space is any character that expr treats as white space, such as
// a space, a tab, or a newline. Without the white space, the minus sign
// is part of the expression: "{{-3}}" prints "-3".
//
// An action can contain a comment, in the comment syntax of expr:
// /* text */, or // text up to the end of the line. An action that
// contains only a comment prints nothing. It can have trim markers:
//
//	"a {{- /* a comment */ -}} b" prints "ab"
//
// A // comment runs to the end of the line, so the closing "}}" must be
// on the next line.
//
// An action with no expression and no comment, such as "{{ }}", is an
// error.
//
// The package does not escape the output.
//
// [expr]: https://expr-lang.org/docs/language-definition
package template
