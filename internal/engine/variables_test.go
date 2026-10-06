package engine

import (
	"reflect"
	"testing"
)

func TestExpand(t *testing.T) {
	vars := map[string]string{"token": "abc", "host": "api.io"}
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"no variable", "plain", "plain"},
		{"single variable", "{{token}}", "abc"},
		{"inside text", "https://{{host}}/v1", "https://api.io/v1"},
		{"several variables", "{{host}}:{{token}}", "api.io:abc"},
		{"spaces around name", "{{ token }}", "abc"},
		{"unknown variable kept", "{{missing}}", "{{missing}}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expand(tt.in, vars); got != tt.want {
				t.Errorf("expand(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestApplyVariables(t *testing.T) {
	vars := []KeyValue{
		{Key: "v", Value: "X", Enabled: true},
		{Key: "off", Value: "nope", Enabled: false},
	}
	in := Request{
		Name:    "{{v}}",
		URL:     "https://{{v}}.io",
		Params:  []KeyValue{{Key: "{{v}}", Value: "{{v}}", Enabled: true}},
		Headers: []KeyValue{{Key: "X-{{v}}", Value: "{{v}}", Enabled: true}},
		Body:    Body{Type: "form", Raw: "{{v}}", Form: []KeyValue{{Key: "{{v}}", Value: "{{v}}", Enabled: true}}},
		Auth:    Auth{Type: "bearer", Token: "{{v}}", Username: "{{v}}", Password: "{{off}}"},
	}
	original := cloneForTest(in)

	got := ApplyVariables(in, vars)

	want := Request{
		Name:    "{{v}}", // display only, never sent
		URL:     "https://X.io",
		Params:  []KeyValue{{Key: "X", Value: "X", Enabled: true}},
		Headers: []KeyValue{{Key: "X-X", Value: "X", Enabled: true}},
		Body:    Body{Type: "form", Raw: "X", Form: []KeyValue{{Key: "X", Value: "X", Enabled: true}}},
		Auth:    Auth{Type: "bearer", Token: "X", Username: "X", Password: "{{off}}"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
	if !reflect.DeepEqual(in, original) {
		t.Errorf("input was mutated: %+v", in)
	}
}

func cloneForTest(r Request) Request {
	r.Params = append([]KeyValue(nil), r.Params...)
	r.Headers = append([]KeyValue(nil), r.Headers...)
	r.Body.Form = append([]KeyValue(nil), r.Body.Form...)
	return r
}
