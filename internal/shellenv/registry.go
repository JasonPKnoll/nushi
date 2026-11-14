package shellenv

import (
	"strings"
)

// A registryVar is one of the registry's environment values: a REG_SZ,
// or a REG_EXPAND_SZ whose %NAME%s are expanded.
type registryVar struct {
	name, value string
	expand      bool
}

// combine is the environment Windows gives a new process from its
// registry (read_windows.go reads it): the system's variables, then the
// user's over them, except Path, which is the system's then the user's;
// a REG_EXPAND_SZ value has each %NAME% it names replaced by that
// variable's value as it stands, else base's (the per-session ones, such
// as USERPROFILE, are only there), and one named nowhere is kept as it
// is, as Windows does. Nushi's own are left out (Own). It's kept apart
// from the registry so its tests run on every OS.
func combine(system, user []registryVar, base []string) []string {
	at := map[string]int{}
	var out []string
	lookup := func(name string) (string, bool) {
		if i, ok := at[strings.ToUpper(name)]; ok {
			_, v, _ := strings.Cut(out[i], "=")
			return v, true
		}
		for _, kv := range base {
			if n, v, ok := strings.Cut(kv, "="); ok && strings.EqualFold(n, name) {
				return v, true
			}
		}
		return "", false
	}
	set := func(v registryVar, appendPath bool) {
		if v.name == "" || Own(v.name) {
			return
		}
		value := v.value
		if v.expand {
			value = expand(value, lookup)
		}
		key := strings.ToUpper(v.name)
		i, ok := at[key]
		if !ok {
			at[key] = len(out)
			out = append(out, v.name+"="+value)
			return
		}
		name, old, _ := strings.Cut(out[i], "=")
		if appendPath && key == "PATH" && old != "" && value != "" {
			value = strings.TrimSuffix(old, ";") + ";" + value
		}
		out[i] = name + "=" + value
	}
	for _, v := range system {
		set(v, false)
	}
	for _, v := range user {
		set(v, true)
	}
	return out
}

// expand is s with each %NAME% that lookup knows replaced by its value;
// one it doesn't, and a lone %, are kept as they are.
func expand(s string, lookup func(string) (string, bool)) string {
	var b strings.Builder
	for {
		start := strings.IndexByte(s, '%')
		if start < 0 {
			break
		}
		end := strings.IndexByte(s[start+1:], '%')
		if end < 0 {
			break
		}
		end += start + 1
		name := s[start+1 : end]
		if v, ok := lookup(name); ok && name != "" {
			b.WriteString(s[:start])
			b.WriteString(v)
			s = s[end+1:]
			continue
		}
		// Not a variable: the first % is text, and the second may open one.
		b.WriteString(s[:end])
		s = s[end:]
	}
	b.WriteString(s)
	return b.String()
}
