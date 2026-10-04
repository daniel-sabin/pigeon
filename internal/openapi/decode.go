package openapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// Map is a JSON/YAML object that remembers its key order, so imported
// requests and generated examples follow the order of the spec.
type Map struct {
	keys   []string
	values map[string]any
}

func newMap() *Map { return &Map{values: map[string]any{}} }

func (m *Map) set(k string, v any) {
	if _, ok := m.values[k]; !ok {
		m.keys = append(m.keys, k)
	}
	m.values[k] = v
}

// Get returns the value for k; it is safe to call on a nil Map.
func (m *Map) Get(k string) any {
	if m == nil {
		return nil
	}
	return m.values[k]
}

func (m *Map) Has(k string) bool {
	if m == nil {
		return false
	}
	_, ok := m.values[k]
	return ok
}

func (m *Map) Keys() []string {
	if m == nil {
		return nil
	}
	return m.keys
}

func (m *Map) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range m.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		buf.Write(kb)
		buf.WriteByte(':')
		vb, err := json.Marshal(m.values[k])
		if err != nil {
			return nil, err
		}
		buf.Write(vb)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// decode parses a JSON or YAML document into *Map, []any and scalar values.
func decode(data []byte) (any, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, errors.New("document is empty")
	}
	if trimmed[0] == '{' {
		dec := json.NewDecoder(bytes.NewReader(trimmed))
		dec.UseNumber()
		v, err := decodeJSON(dec)
		if err != nil {
			return nil, fmt.Errorf("invalid JSON: %w", err)
		}
		return v, nil
	}
	var node yaml.Node
	if err := yaml.Unmarshal(trimmed, &node); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	return fromYAML(&node)
}

func decodeJSON(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return tok, nil // string, json.Number, bool or nil
	}
	switch delim {
	case '{':
		m := newMap()
		for dec.More() {
			kt, err := dec.Token()
			if err != nil {
				return nil, err
			}
			v, err := decodeJSON(dec)
			if err != nil {
				return nil, err
			}
			m.set(kt.(string), v)
		}
		_, err := dec.Token() // closing '}'
		return m, err
	case '[':
		arr := []any{}
		for dec.More() {
			v, err := decodeJSON(dec)
			if err != nil {
				return nil, err
			}
			arr = append(arr, v)
		}
		_, err := dec.Token() // closing ']'
		return arr, err
	}
	return nil, fmt.Errorf("unexpected %v", delim)
}

func fromYAML(n *yaml.Node) (any, error) {
	switch n.Kind {
	case yaml.DocumentNode:
		if len(n.Content) == 0 {
			return nil, nil
		}
		return fromYAML(n.Content[0])
	case yaml.MappingNode:
		m := newMap()
		for i := 0; i+1 < len(n.Content); i += 2 {
			v, err := fromYAML(n.Content[i+1])
			if err != nil {
				return nil, err
			}
			m.set(n.Content[i].Value, v)
		}
		return m, nil
	case yaml.SequenceNode:
		arr := []any{}
		for _, c := range n.Content {
			v, err := fromYAML(c)
			if err != nil {
				return nil, err
			}
			arr = append(arr, v)
		}
		return arr, nil
	case yaml.AliasNode:
		return fromYAML(n.Alias)
	default:
		var v any
		if err := n.Decode(&v); err != nil {
			return nil, err
		}
		return v, nil
	}
}

func asMap(v any) *Map {
	m, _ := v.(*Map)
	return m
}

func asSlice(v any) []any {
	s, _ := v.([]any)
	return s
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func asBool(v any) bool {
	b, _ := v.(bool)
	return b
}

// scalarString renders a scalar example value as it would appear in a query
// string or header.
func scalarString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case time.Time: // YAML timestamps
		return t.Format(time.RFC3339)
	case *Map, []any:
		b, _ := json.Marshal(t)
		return string(b)
	default:
		return fmt.Sprint(t)
	}
}
