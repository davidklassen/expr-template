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
// An if action runs a part of the template only when a condition is
// true. It has the forms below. T1 and T0 stand for parts of the
// template: text and actions.
//
//	{{ if condition }} T1 {{ end }}
//		If the condition is true, T1 is executed. Otherwise no output
//		is generated. The condition is an expression, and its value
//		must be a boolean. Any other value, including nil, is an
//		error. To test whether a value is set, compare it with nil:
//		{{ if name != nil }}.
//
//	{{ if condition }} T1 {{ else }} T0 {{ end }}
//		If the condition is true, T1 is executed. Otherwise T0 is
//		executed.
//
//	{{ if condition }} T1 {{ else if condition }} T0 {{ end }}
//		The else action of an if can include another if directly.
//		The effect is the same as writing
//			{{ if condition }} T1 {{ else }}{{ if condition }} T0 {{ end }}{{ end }}
//
// An action that starts with the word if, else, or end is always an
// action of that kind. An expression that starts with the word if or
// end must be in parentheses. "{{ (end) }}" prints the value of the
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
// The package does not escape the output.
//
// [expr]: https://expr-lang.org/docs/language-definition
package template
