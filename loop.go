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

// loopVar returns an option for the expressions in the body of a loop. It comes
// after the options of the template. It tells the compiler that the env is a
// loop env: a map with the names of the env of the template and a loop variable
// of the given name. elem is what expr knows about the type of the variable.
// The loop env itself is made by newLoopEnv when the loop runs.
func loopVar(name string, elem nature.Nature) expr.Option {
	return func(c *conf.Config) {
		env := types.Map{}.Nature()
		env.Fields = envNames(&c.NtCache, c.Env)
		env.Fields[name] = elem
		c.Env = env
	}
}

// envNames returns the names of env with their types, as expr sees them: expr
// knows the struct tags, the embedded structs, and the methods.
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
			// In the loop env, a method is a function that is bound to the env.
			// Its type does not have the receiver.
			names[name] = c.FromType(reflect.Zero(env.Type).Method(n.MethodIndex).Type())
		}
	}
	return names
}

// newLoopEnv returns the env for the body of a loop: a map with all names of
// env. The caller adds the loop variables.
//
// The body needs an env of its own, because expr has no way to give a variable
// to a program. A program reads all its names from the env.
func newLoopEnv(env any) map[string]any {
	// A loop env is a map of this type, so a nested loop takes this path.
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
			// FieldByIndexErr fails if an embedded pointer on the way to the
			// field is nil. The loop env then does not have the name.
			if f, err := base.FieldByIndexErr(index); err == nil {
				loop[name] = f.Interface()
			}
		}
	}
	// Each named type can have methods, not only a struct. A method comes
	// after a map key with the same name, because expr also prefers the method.
	if v.IsValid() {
		for i := range v.NumMethod() {
			loop[v.Type().Method(i).Name] = v.Method(i).Interface()
		}
	}
	return loop
}

// keyOrder returns how a for action orders the keys of a map with the key type
// key: as strings, as signed integers, as unsigned integers, or as floats. It
// returns reflect.Invalid for a type whose values have no order.
func keyOrder(key reflect.Type) reflect.Kind {
	switch key.Kind() {
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

// canOrder reports whether a for action can loop through a map with the key
// type key. The keys must have an order, so that the output is the same in each
// run. Keys of an interface type can have an order: sortedEntries checks each
// key when the loop runs.
func canOrder(key reflect.Type) bool {
	return key.Kind() == reflect.Interface || keyOrder(key) != reflect.Invalid
}

// An entry is a key of a map with its value.
type entry struct {
	key, value reflect.Value
}

// sortedEntries returns the entries of the map m in the order of their keys. It
// returns false if the keys have no order.
//
// sortedEntries reads all entries before the loop runs its body. A lookup of
// each key in the body would not find a NaN key, or an entry that the body
// removed.
func sortedEntries(m reflect.Value) ([]entry, bool) {
	if !canOrder(m.Type().Key()) {
		return nil, false
	}
	order := keyOrder(m.Type().Key())
	entries := make([]entry, 0, m.Len())
	for it := m.MapRange(); it.Next(); {
		key := it.Key()
		if key.Kind() == reflect.Interface {
			// expr builtins such as groupBy give a map with keys of the type
			// any. Such keys have an order if they all have the same kind of
			// order. A nil key has none.
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
