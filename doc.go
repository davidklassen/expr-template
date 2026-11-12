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
// The package does not escape the output.
//
// [expr]: https://expr-lang.org/docs/language-definition
package template
