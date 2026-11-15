// Package template implements text templates with [expr] expressions.
//
// A template is text with actions. An action is written between "{{" and "}}".
// Execute copies the text to the output and runs the actions:
//
//	t := template.Must(template.New("hello").Parse("Hello, {{ name }}!\n"))
//	err := t.Execute(os.Stdout, map[string]any{"name": "Alice"})
//
// The expressions of a template read their names from the data that Execute
// receives. The data is a map or a struct, as the env of expr is. The options
// of [New] are options of expr. With [expr.Env], Parse knows the type of the
// data and type-checks the template.
//
// The package does not escape the output.
//
// # Actions
//
// Here is the list of actions.
//
//	{{ expression }}
//		The value of the expression is printed as fmt.Print prints
//		it, so nil is printed as <nil>.
//
//	{{ /* a comment */ }}
//		A comment in the syntax of expr: /* text */, or // text up
//		to the end of the line. An action that contains only a
//		comment prints nothing. After a // comment, the closing "}}"
//		must be on the next line.
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
//	{{ for x in list }} T1 {{ end }}
//		The list is an expression, and its value must be a slice, an
//		array, or a map. Any other value, including nil, is an error.
//		T1 is executed for each element of the list. In T1, the
//		variable x has the value of the element. If the list is
//		empty, no output is generated.
//
//	{{ for i, x in list }} T1 {{ end }}
//		This form has a second variable, i. For a slice or an array,
//		i has the index of the element, from 0. For a map, i has the
//		key of the element.
//
// An action that starts with if, else, end, or for is always that kind of
// action. To start an expression with one of these words, put it in
// parentheses: "{{ (end) }}" prints the value of the name end, and
// "{{ (if a { 1 } else { 2 }) }}" is the if expression of expr.
//
// # Loops
//
// For a map, T1 is executed for the elements in sorted key order. The keys must
// all be strings, all be integers, or all be floats.
//
// The variables of a for action exist only in T1. A variable shadows a name of
// the data, a function, or a variable of an enclosing for action with the same
// name. If Parse knows the type of the list, it type-checks the uses of the
// variables.
//
// Inside a for action, expressions read a copy of the data that is made when
// the loop starts. If a method called in the loop changes a field of the data,
// the expressions in the loop still read the old value. Without [expr.Env], a
// name that the data does not have is nil inside a for action, even when it is
// an error outside one.
//
// # Text and spaces
//
// Trim markers remove the white space around an action. "{{- " removes all
// white space before the action, and " -}}" removes all white space after it:
//
//	"a  {{- 1 -}}  b" prints "a1b"
//
// The minus sign must have white space between it and the expression. Otherwise
// it is part of the expression: "{{-3}}" prints "-3". White space is any
// character that expr treats as white space, such as a space, a tab, or a
// newline.
//
// [expr]: https://expr-lang.org/docs/language-definition
package template
