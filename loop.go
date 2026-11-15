package template

import (
	"cmp"
	"maps"
	"reflect"
	"slices"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/checker/nature"
	"github.com/expr-lang/expr/conf"
	"github.com/expr-lang/expr/types"
)

// withLoopVar returns an expr option that declares a loop variable with the
// given name and type, so that the compiler can type-check the expressions in
// the loop body. expr can only read variables from the env, so the option
// replaces the env type with a map type that has every name of the original env
// plus the variable.
//
// The option only describes types. newLoopEnv builds the matching map when the
// template is executed.
func withLoopVar(name string, typ nature.Nature) expr.Option {
	return func(c *conf.Config) {
		env := types.Map{}.Nature()
		env.Fields = envNames(&c.NtCache, c.Env)
		env.Fields[name] = typ
		c.Env = env
	}
}

// envNames returns the names that expr resolves in env, with their types. It
// asks expr for them, so that struct tags, embedded structs, and methods follow
// the rules of expr.
func envNames(c *nature.Cache, env nature.Nature) map[string]nature.Nature {
	if env.Kind == reflect.Map {
		names := map[string]nature.Nature{}
		if env.TypeData != nil {
			maps.Copy(names, env.Fields)
		}
		return names
	}
	names := env.All(c)
	for name, n := range names {
		if n.Method {
			// newLoopEnv stores a method as a method value, which is bound to
			// its receiver. So the type here must not have the receiver.
			names[name] = c.FromType(reflect.Zero(env.Type).Method(n.MethodIndex).Type())
		}
	}
	return names
}

// newLoopEnv returns a map with every name of env: its map entries or struct
// fields, and its methods. The caller adds the loop variables to the map.
//
// expr has no API to pass a variable to a program. So the loop body runs with
// this map as its env, and not with the data itself.
func newLoopEnv(env any) map[string]any {
	// A nested loop also takes this path, because its env is the map of the
	// enclosing loop.
	if m, ok := env.(map[string]any); ok && m != nil {
		return maps.Clone(m)
	}
	loop := map[string]any{}
	v := reflect.ValueOf(env)
	base := v
	for base.Kind() == reflect.Pointer {
		base = base.Elem()
	}
	switch base.Kind() {
	case reflect.Map:
		for k, x := range base.Seq2() {
			loop[k.String()] = x.Interface()
		}
	case reflect.Struct:
		var c nature.Cache
		fields := c.FromType(base.Type())
		for name := range nature.StructFields(&c, base.Type()) {
			index, _ := fields.FieldIndex(&c, name)
			// FieldByIndexErr fails if the field is in an embedded struct
			// pointer that is nil. Such a field is left out of the map.
			if f, err := base.FieldByIndexErr(index); err == nil {
				loop[name] = f.Interface()
			}
		}
	}
	// Methods are added for any kind of data, because a named map type can
	// have methods too. They are added last so that a method overwrites a map
	// key with the same name: expr resolves such a name to the method.
	if v.IsValid() {
		for i := range v.NumMethod() {
			loop[v.Type().Method(i).Name] = v.Method(i).Interface()
		}
	}
	return loop
}

// keyOrder returns how map keys of type t are compared when they are sorted: as
// strings, signed integers, unsigned integers, or floats. It returns
// reflect.Invalid if keys of type t cannot be sorted.
func keyOrder(t reflect.Type) reflect.Kind {
	switch t.Kind() {
	case reflect.String:
		return reflect.String
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return reflect.Int64
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return reflect.Uint64
	case reflect.Float32, reflect.Float64:
		return reflect.Float64
	}
	return reflect.Invalid
}

// canOrder reports whether a for action can loop through a map with keys of
// type t. The keys must be sortable, so that the output is the same on every
// run. For an interface type the answer depends on the actual keys, and
// sortedEntries checks them when the loop runs.
func canOrder(t reflect.Type) bool {
	return t.Kind() == reflect.Interface || keyOrder(t) != reflect.Invalid
}

// An entry is one key-value pair of a map.
type entry struct {
	key, value reflect.Value
}

// sortedEntries returns the entries of the map m sorted by key. It returns
// false if the keys cannot be sorted.
func sortedEntries(m reflect.Value) ([]entry, bool) {
	if !canOrder(m.Type().Key()) {
		return nil, false
	}
	order := keyOrder(m.Type().Key())
	entries := make([]entry, 0, m.Len())
	for it := m.MapRange(); it.Next(); {
		key := it.Key()
		if key.Kind() == reflect.Interface {
			// expr builtins such as groupBy return a map[any]V. Its keys can
			// be sorted only if keyOrder is the same for all of them. A nil key
			// cannot be sorted.
			key = key.Elem()
			o := reflect.Invalid
			if key.IsValid() {
				o = keyOrder(key.Type())
			}
			if o == reflect.Invalid || len(entries) > 0 && o != order {
				return nil, false
			}
			order = o
		}
		entries = append(entries, entry{key, it.Value()})
	}
	slices.SortFunc(entries, func(a, b entry) int {
		switch order {
		case reflect.Int64:
			return cmp.Compare(a.key.Int(), b.key.Int())
		case reflect.Uint64:
			return cmp.Compare(a.key.Uint(), b.key.Uint())
		case reflect.Float64:
			return cmp.Compare(a.key.Float(), b.key.Float())
		}
		return cmp.Compare(a.key.String(), b.key.String())
	})
	return entries, true
}
