package engine

import "regexp"

var varPattern = regexp.MustCompile(`\{\{\s*([^{}\s]+)\s*\}\}`)

// expand replaces {{name}} with its value. Unknown names are left as-is so a
// typo shows up in the sent request instead of silently becoming empty.
func expand(s string, vars map[string]string) string {
	return varPattern.ReplaceAllStringFunc(s, func(m string) string {
		name := varPattern.FindStringSubmatch(m)[1]
		if v, ok := vars[name]; ok {
			return v
		}
		return m
	})
}

// ApplyVariables returns a copy of r with {{name}} replaced in every field that
// is sent. Disabled variables are ignored. r itself is left untouched so the
// history keeps the placeholders rather than the secrets.
func ApplyVariables(r Request, vars []KeyValue) Request {
	m := map[string]string{}
	for _, v := range vars {
		if v.Enabled && v.Key != "" {
			m[v.Key] = v.Value
		}
	}
	r.URL = expand(r.URL, m)
	r.Params = expandAll(r.Params, m)
	r.Headers = expandAll(r.Headers, m)
	r.Body.Raw = expand(r.Body.Raw, m)
	r.Body.Form = expandAll(r.Body.Form, m)
	r.Auth.Token = expand(r.Auth.Token, m)
	r.Auth.Username = expand(r.Auth.Username, m)
	r.Auth.Password = expand(r.Auth.Password, m)
	return r
}

// expandAll returns a new slice: writing into kvs would modify the caller's.
func expandAll(kvs []KeyValue, m map[string]string) []KeyValue {
	if kvs == nil {
		return nil
	}
	out := make([]KeyValue, len(kvs))
	for i, kv := range kvs {
		out[i] = KeyValue{Key: expand(kv.Key, m), Value: expand(kv.Value, m), Enabled: kv.Enabled}
	}
	return out
}
