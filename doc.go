// Package template implements text templates whose actions are [expr]
// expressions.
//
// A template is text with actions. An action is written between "{{" and "}}".
// Execute copies the text to the output and runs the actions. Most actions are
// an expression, and Execute prints its value:
//
//	t := template.Must(template.New("hello").Parse("Hello, {{ name }}!\n"))
//	err := t.Execute(os.Stdout, map[string]any{"name": "Alice"})
//
// An if action runs a part of the template only when a condition is true. It
// has the forms below. T1 and T0 stand for parts of the template: text and
// actions.
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
// A for action runs a part of the template for each element of a list.
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
// For a map, T1 is executed for the elements in the order of their keys. The
// keys must all be strings, all be integers, or all be floats. Numbers are in
// the order of their values. Strings are in the order of their bytes, so "B"
// comes before "a".
//
// The variables of a for action exist only in T1. There a variable shadows a
// name of the data, a function, or the variable of an enclosing for action with
// the same name. If Parse knows the type of the list, it knows the types of the
// variables, and it reports a wrong use of a variable.
//
// In T1 of a for action, the expressions read a copy of the data. If a method
// of the data changes a field of the data, an expression in T1 that reads this
// field gets the value from before the change. Without [expr.Env], a name that
// the data does not have is nil in T1.
//
// An action that starts with the word if, else, end, or for is always an action
// of that kind. An expression that starts with the word if, end, or for must be
// in parentheses. "{{ (end) }}" prints the value of the name end. "{{ (if a { 1
// } else { 2 }) }}" is the if expression of expr.
//
// A trim marker removes the white space around an action. A minus sign and
// white space after "{{" remove all white space before the action. White space
// and a minus sign before "}}" remove all white space after it:
//
//	"a  {{- 1 -}}  b" prints "a1b"
//
// White space is any character that expr treats as white space, such as a
// space, a tab, or a newline. Without the white space, the minus sign is part
// of the expression: "{{-3}}" prints "-3".
//
// An action can contain a comment, in the comment syntax of expr: /* text */,
// or // text up to the end of the line. An action that contains only a comment
// prints nothing. It can have trim markers:
//
//	"a {{- /* a comment */ -}} b" prints "ab"
//
// A // comment runs to the end of the line, so the closing "}}" must be on the
// next line.
//
// The package does not escape the output.
//
// [expr]: https://expr-lang.org/docs/language-definition
package template
