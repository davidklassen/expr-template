// Package template implements text templates whose actions are [expr]
// expressions.
//
// A template is text with actions. An action is an expression between
// "{{" and "}}". Execute copies the text to the output and replaces
// each action with the value of its expression:
//
//	t := template.Must(template.New("hello").Parse("Hello, {{ name }}!\n"))
//	err := t.Execute(os.Stdout, map[string]any{"name": "Alice"})
//
// A trim marker removes the white space around an action. A minus
// sign and white space after "{{" remove all white space before the
// action. White space and a minus sign before "}}" remove all white
// space after it:
//
//	"a  {{- 1 -}}  b" prints "a1b"
//
// White space is a space, a tab, a carriage return, or a newline.
// Without the white space, the minus sign is part of the expression:
// "{{-3}}" prints "-3".
//
// The package does not escape the output.
//
// [expr]: https://expr-lang.org/docs/language-definition
package template
