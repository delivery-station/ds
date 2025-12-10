package types

import (
	"strconv"
	"strings"
)

// PluginArgs represents a set of plugin arguments parsed into key/value pairs.
type PluginArgs struct {
	raw         []string
	values      map[string][]string
	positionals []string
}

// NewPluginArgs constructs a PluginArgs helper from a key=value list.
func NewPluginArgs(pairs []string) PluginArgs {
	values := make(map[string][]string, len(pairs))
	positionalMap := make(map[int]string)
	maxIndex := -1

	for _, pair := range pairs {
		if pair == "" {
			continue
		}

		key := pair
		val := ""
		if idx := strings.Index(pair, "="); idx >= 0 {
			key = pair[:idx]
			val = pair[idx+1:]
		}

		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if key == "" {
			continue
		}

		values[key] = append(values[key], val)

		if strings.HasPrefix(key, "arg") {
			if pos, err := strconv.Atoi(key[3:]); err == nil && pos >= 0 {
				positionalMap[pos] = val
				if pos > maxIndex {
					maxIndex = pos
				}
			}
		}
	}

	positionals := make([]string, 0)
	if maxIndex >= 0 {
		positionals = make([]string, maxIndex+1)
		for i := 0; i <= maxIndex; i++ {
			positionals[i] = positionalMap[i]
		}
	}

	return PluginArgs{
		raw:         append([]string(nil), pairs...),
		values:      values,
		positionals: positionals,
	}
}

// Raw returns a copy of the underlying key=value list.
func (a PluginArgs) Raw() []string {
	return append([]string(nil), a.raw...)
}

// Keys returns the set of keys present in the argument list.
func (a PluginArgs) Keys() []string {
	keys := make([]string, 0, len(a.values))
	for k := range a.values {
		keys = append(keys, k)
	}
	return keys
}

// Has reports whether the given key exists.
func (a PluginArgs) Has(key string) bool {
	_, ok := a.values[key]
	return ok
}

// All returns all values associated with the key.
func (a PluginArgs) All(key string) []string {
	vals := a.values[key]
	return append([]string(nil), vals...)
}

// First returns the first value associated with the key.
func (a PluginArgs) First(key string) (string, bool) {
	vals := a.values[key]
	if len(vals) == 0 {
		return "", false
	}
	return vals[0], true
}

// FirstAny returns the first value for the first key in the list that exists.
func (a PluginArgs) FirstAny(keys ...string) (string, bool) {
	for _, key := range keys {
		if val, ok := a.First(key); ok {
			return val, true
		}
	}
	return "", false
}

// Bool interprets the first value for the key as a boolean.
func (a PluginArgs) Bool(key string) (bool, bool) {
	val, ok := a.First(key)
	if !ok {
		return false, false
	}
	if val == "" {
		return true, true
	}

	lowered := strings.ToLower(val)
	switch lowered {
	case "true", "t", "1", "yes", "y", "on":
		return true, true
	case "false", "f", "0", "no", "n", "off":
		return false, true
	}

	parsed, err := strconv.ParseBool(lowered)
	if err != nil {
		return false, false
	}
	return parsed, true
}

// BoolAny returns the first boolean value for any of the provided keys.
func (a PluginArgs) BoolAny(keys ...string) (bool, bool) {
	for _, key := range keys {
		if val, ok := a.Bool(key); ok {
			return val, true
		}
	}
	return false, false
}

// Positional returns the positional argument at the provided index.
func (a PluginArgs) Positional(index int) (string, bool) {
	if index < 0 || index >= len(a.positionals) {
		return "", false
	}
	val := strings.TrimSpace(a.positionals[index])
	if val == "" {
		return "", false
	}
	return val, true
}

// Positionals returns a copy of all positional arguments.
func (a PluginArgs) Positionals() []string {
	out := make([]string, len(a.positionals))
	copy(out, a.positionals)
	return out
}

// Count returns the number of values tracked for the key.
func (a PluginArgs) Count(key string) int {
	return len(a.values[key])
}
