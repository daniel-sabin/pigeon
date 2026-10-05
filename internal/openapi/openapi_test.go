package openapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/daniel-sabin/pigeon/internal/engine"
)

func load(t *testing.T, name, source string) []engine.Request {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	col, err := Parse(data, source)
	if err != nil {
		t.Fatal(err)
	}
	return col.Requests
}

func TestParseOpenAPI3(t *testing.T) {
	reqs := load(t, "petstore-v3.yaml", "/tmp/petstore.yaml")

	var got []string
	for _, r := range reqs {
		got = append(got, r.Method+" "+r.Name+" "+r.URL+" auth="+r.Auth.Type)
	}
	want := []string{
		"GET List pets https://api.example.com/v1/pets?limit=20 auth=bearer",
		"POST createPet https://api.example.com/v1/pets auth=bearer",
		"GET GET /pets/{petId} https://api.example.com/v1/pets/{petId} auth=none",
		"DELETE Delete a pet https://api.example.com/v1/pets/{petId} auth=none",
		"POST Login https://api.example.com/v1/login auth=basic",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("requests:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	list := reqs[0]
	wantParams := []engine.KeyValue{{Key: "limit", Value: "20", Enabled: true}, {Key: "status", Value: "available"}}
	if !equalKV(list.Params, wantParams) {
		t.Errorf("params = %+v", list.Params)
	}
	if !equalKV(list.Headers, []engine.KeyValue{{Key: "X-Request-Id", Value: "abc-123", Enabled: true}}) {
		t.Errorf("headers = %+v", list.Headers)
	}

	// Properties keep the spec order, readOnly ones are skipped, allOf is
	// merged and the recursive "friends" reference stops.
	wantBody := `{
  "name": "Rex",
  "tag": "string",
  "born": "2026-01-01",
  "owner": {
    "email": "user@example.com",
    "vip": false
  },
  "friends": []
}`
	if create := reqs[1]; create.Body.Type != "json" || create.Body.Raw != wantBody {
		t.Errorf("body (%s) =\n%s", create.Body.Type, create.Body.Raw)
	}

	if del := reqs[3]; !equalKV(del.Headers, []engine.KeyValue{{Key: "X-API-Key", Enabled: true}}) {
		t.Errorf("apiKey header = %+v", del.Headers)
	}

	login := reqs[4]
	wantForm := []engine.KeyValue{{Key: "username", Value: "alice", Enabled: true}, {Key: "password", Value: "string"}}
	if login.Body.Type != "form" || !equalKV(login.Body.Form, wantForm) {
		t.Errorf("form body (%s) = %+v", login.Body.Type, login.Body.Form)
	}
}

func TestParseSwagger2(t *testing.T) {
	reqs := load(t, "petstore-v2.json", "https://petstore.swagger.io/v2/swagger.json")

	var got []string
	for _, r := range reqs {
		got = append(got, r.Method+" "+r.URL+" auth="+r.Auth.Type+" body="+r.Body.Type)
	}
	want := []string{
		"POST https://petstore.swagger.io/v2/pet auth=bearer body=json",
		"GET https://petstore.swagger.io/v2/pet/findByStatus?status=available auth=none body=none",
		"GET https://petstore.swagger.io/v2/pet/42 auth=none body=none",
		"POST https://petstore.swagger.io/v2/pet/{petId} auth=none body=form",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("requests:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	if !strings.Contains(reqs[0].Body.Raw, `"id": 9007199254740993`) {
		t.Errorf("large integer example lost precision:\n%s", reqs[0].Body.Raw)
	}
	if !equalKV(reqs[2].Headers, []engine.KeyValue{{Key: "api_key", Enabled: true}}) {
		t.Errorf("api key header = %+v", reqs[2].Headers)
	}
	if !equalKV(reqs[3].Body.Form, []engine.KeyValue{{Key: "name"}}) {
		t.Errorf("form = %+v (file fields must be skipped)", reqs[3].Body.Form)
	}
}

func TestServerURL(t *testing.T) {
	tests := []struct {
		name, spec, source, want string
	}{
		{"v3 relative server, from URL", `{"openapi":"3.0.0","servers":[{"url":"/api"}]}`, "https://x.io/docs/openapi.json", "https://x.io/api"},
		{"v3 no server, from file", `{"openapi":"3.0.0"}`, "/Users/me/spec.json", "http://localhost"},
		{"v2 no host, from URL", `{"swagger":"2.0","basePath":"/v1/"}`, "http://localhost:8080/swagger.json", "http://localhost:8080/v1"},
		{"v2 no host, from file", `{"swagger":"2.0"}`, "spec.json", "http://localhost"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, _ := decode([]byte(tt.spec))
			c := &converter{root: asMap(doc), v2: strings.Contains(tt.spec, "swagger")}
			if got := c.serverURL(tt.source); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	for _, doc := range []string{"", "not: [valid", `{"foo": 1}`, `{"openapi": "3.0.0", "paths": {}}`, "<html></html>"} {
		if _, err := Parse([]byte(doc), "x"); err == nil {
			t.Errorf("Parse(%q): expected an error", doc)
		}
	}
}

func TestFetchDiscoversSpecFromSwaggerUI(t *testing.T) {
	spec, _ := os.ReadFile("testdata/petstore-v3.yaml")
	mux := http.NewServeMux()
	mux.HandleFunc("/docs/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<!-- HTML for static distribution bundle build -->
<!DOCTYPE html><html><body><div id="swagger-ui"></div>
<script src="./swagger-initializer.js" charset="UTF-8"></script></body></html>`))
	})
	mux.HandleFunc("/docs/swagger-initializer.js", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`window.ui = SwaggerUIBundle({ url: "/specs/main.yaml", dom_id: '#swagger-ui' })`))
	})
	mux.HandleFunc("/specs/main.yaml", func(w http.ResponseWriter, r *http.Request) { w.Write(spec) })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	col, err := Fetch(context.Background(), srv.Client(), srv.URL+"/docs/")
	if err != nil {
		t.Fatal(err)
	}
	if col.Name != "Petstore" || len(col.Requests) != 5 || col.Source != srv.URL+"/specs/main.yaml" {
		t.Errorf("got %q with %d requests from %q", col.Name, len(col.Requests), col.Source)
	}
}

func TestFetchFollowsSwaggerConfigAndWellKnownPaths(t *testing.T) {
	spec, _ := os.ReadFile("testdata/petstore-v2.json")
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/swagger-ui/index.html":
			w.Write([]byte(`<html><body>no hints here</body></html>`))
		case "/v3/api-docs": // springdoc-style config pointing at the spec
			w.Write([]byte(`{"urls":[{"name":"main","url":"/v3/api-docs/main"}]}`))
		case "/v3/api-docs/main":
			w.Write(spec)
		default:
			http.NotFound(w, r)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	col, err := Fetch(context.Background(), srv.Client(), srv.URL+"/swagger-ui/index.html")
	if err != nil {
		t.Fatal(err)
	}
	if col.Name != "Swagger Petstore" {
		t.Errorf("name = %q", col.Name)
	}

	if _, err := Fetch(context.Background(), srv.Client(), srv.URL+"/nothing"); err == nil {
		t.Error("expected an error for a URL without a spec")
	}
}

func equalKV(a, b []engine.KeyValue) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestRefsInScript(t *testing.T) {
	page, _ := url.Parse("https://b.example.com/docs/")
	script := []byte("const def = \"https://a.example.com/v2/swagger.json\";\n" +
		"const services = `a.example.com=https://a.example.com/v2/swagger.json,b.example.com=https://b.example.com/api/v3/openapi.json`;\n" +
		"SwaggerUIBundle({ url: def, configUrl: \"/v3/api-docs/swagger-config\" })")
	var got []string
	for _, u := range refsIn(script, page) {
		got = append(got, u.String())
	}
	want := []string{
		"https://b.example.com/v3/api-docs/swagger-config",
		"https://a.example.com/v2/swagger.json",
		"https://a.example.com/v2/swagger.json",
		"https://b.example.com/api/v3/openapi.json",
		"https://b.example.com/v3/api-docs/swagger-config",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("refs:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
