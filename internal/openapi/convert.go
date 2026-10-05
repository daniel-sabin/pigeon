// Package openapi turns Swagger 2.0 and OpenAPI 3.x documents into Pigeon
// collections.
package openapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/daniel-sabin/pigeon/internal/engine"
	"github.com/daniel-sabin/pigeon/internal/storage"
)

var methods = []string{"get", "post", "put", "patch", "delete", "head", "options"}

// maxExampleDepth bounds generated examples for deeply nested schemas.
const maxExampleDepth = 8

type converter struct {
	root    *Map
	v2      bool
	baseURL string
}

// Parse converts a JSON or YAML spec into a collection. source is where the
// document came from (a URL or a file path); a URL is used to resolve
// relative server addresses.
func Parse(data []byte, source string) (storage.Collection, error) {
	doc, err := decode(data)
	if err != nil {
		return storage.Collection{}, err
	}
	root := asMap(doc)
	if root == nil {
		return storage.Collection{}, errors.New("not an OpenAPI document: expected an object at the top level")
	}
	c := &converter{root: root}
	switch {
	case strings.HasPrefix(scalarString(root.Get("swagger")), "2"):
		c.v2 = true
	case strings.HasPrefix(scalarString(root.Get("openapi")), "3"):
	default:
		return storage.Collection{}, errors.New("not an OpenAPI document: missing \"openapi: 3.x\" or \"swagger: 2.0\"")
	}
	c.baseURL = c.serverURL(source)

	name := strings.TrimSpace(asString(asMap(root.Get("info")).Get("title")))
	if name == "" {
		name = "Imported API"
	}
	col := storage.Collection{ID: newID(), Name: name, Source: source, Requests: []engine.Request{}}

	paths := asMap(root.Get("paths"))
	for _, path := range paths.Keys() {
		item := c.resolve(paths.Get(path))
		for _, m := range methods {
			op := asMap(item.Get(m))
			if op == nil {
				continue
			}
			col.Requests = append(col.Requests, c.request(path, m, item, op))
		}
	}
	if len(col.Requests) == 0 {
		return storage.Collection{}, errors.New("the document doesn't define any operation")
	}
	c.groupByTag(col.Requests)
	return col, nil
}

// groupByTag orders requests by folder: tags declared at the top level come
// first in their declared order, then the others as they appear. Untagged
// requests go last; the spec order is kept within a folder.
func (c *converter) groupByTag(reqs []engine.Request) {
	rank := map[string]int{}
	add := func(tag string) {
		if _, ok := rank[tag]; !ok && tag != "" {
			rank[tag] = len(rank)
		}
	}
	for _, t := range asSlice(c.root.Get("tags")) {
		add(strings.TrimSpace(asString(asMap(t).Get("name"))))
	}
	for _, r := range reqs {
		add(r.Folder)
	}
	key := func(r engine.Request) int {
		if r.Folder == "" {
			return len(rank)
		}
		return rank[r.Folder]
	}
	sort.SliceStable(reqs, func(i, j int) bool { return key(reqs[i]) < key(reqs[j]) })
}

// serverURL returns the base URL requests are relative to, without a
// trailing slash.
func (c *converter) serverURL(source string) string {
	src, err := url.Parse(source)
	if err != nil || src.Host == "" || (src.Scheme != "http" && src.Scheme != "https") {
		src = nil
	}

	if c.v2 {
		scheme, host := "https", "localhost"
		if src != nil {
			scheme, host = src.Scheme, src.Host
		} else {
			scheme = "http"
		}
		if schemes := asSlice(c.root.Get("schemes")); len(schemes) > 0 {
			scheme = asString(schemes[0])
		}
		if h := asString(c.root.Get("host")); h != "" {
			host = h
		}
		return strings.TrimRight(scheme+"://"+host+asString(c.root.Get("basePath")), "/")
	}

	server := "/"
	if servers := asSlice(c.root.Get("servers")); len(servers) > 0 {
		s := asMap(servers[0])
		server = asString(s.Get("url"))
		vars := asMap(s.Get("variables"))
		for _, name := range vars.Keys() {
			def := scalarString(asMap(vars.Get(name)).Get("default"))
			server = strings.ReplaceAll(server, "{"+name+"}", def)
		}
	}
	if !strings.Contains(server, "://") {
		// Relative server URLs are relative to where the document was served.
		base := "http://localhost"
		if src != nil {
			base = src.Scheme + "://" + src.Host
		}
		if strings.HasPrefix(server, "//") {
			server = "https:" + server
		} else {
			server = base + "/" + strings.TrimLeft(server, "/")
		}
	}
	return strings.TrimRight(server, "/")
}

func (c *converter) request(path, method string, item, op *Map) engine.Request {
	name := strings.TrimSpace(asString(op.Get("summary")))
	if name == "" {
		name = asString(op.Get("operationId"))
	}
	if name == "" {
		name = strings.ToUpper(method) + " " + path
	}
	r := engine.Request{
		ID:      newID(),
		Name:    name,
		Folder:  firstTag(op),
		Method:  strings.ToUpper(method),
		Params:  []engine.KeyValue{},
		Headers: []engine.KeyValue{},
		Body:    engine.Body{Type: "none", Form: []engine.KeyValue{}},
		Auth:    engine.Auth{Type: "none"},
	}

	var formParams []*Map
	var bodySchema any
	for _, p := range c.parameters(item, op) {
		pname := asString(p.Get("name"))
		required := asBool(p.Get("required"))
		switch asString(p.Get("in")) {
		case "path":
			if v := c.paramExample(p); v != "" {
				path = strings.ReplaceAll(path, "{"+pname+"}", url.PathEscape(v))
			}
		case "query":
			r.Params = append(r.Params, engine.KeyValue{Key: pname, Value: c.paramExample(p), Enabled: required})
		case "header":
			r.Headers = append(r.Headers, engine.KeyValue{Key: pname, Value: c.paramExample(p), Enabled: required})
		case "body": // Swagger 2.0
			bodySchema = p.Get("schema")
		case "formData": // Swagger 2.0
			if asString(p.Get("type")) != "file" {
				formParams = append(formParams, p)
			}
		}
	}

	if c.v2 {
		switch {
		case bodySchema != nil:
			r.Body = c.v2Body(op, bodySchema)
		case len(formParams) > 0:
			r.Body.Type = "form"
			for _, p := range formParams {
				r.Body.Form = append(r.Body.Form, engine.KeyValue{
					Key: asString(p.Get("name")), Value: c.paramExample(p), Enabled: asBool(p.Get("required")),
				})
			}
		}
	} else if rb := c.resolve(op.Get("requestBody")); rb != nil {
		r.Body = c.v3Body(asMap(rb.Get("content")))
	}

	c.applyAuth(&r, op)
	r.URL = c.baseURL + path
	r.URL = appendQuery(r.URL, r.Params)
	return r
}

// firstTag returns the operation's first tag, used as its folder.
func firstTag(op *Map) string {
	for _, t := range asSlice(op.Get("tags")) {
		if tag := strings.TrimSpace(scalarString(t)); tag != "" {
			return tag
		}
	}
	return ""
}

// parameters merges path-level and operation-level parameters; the latter
// override the former when they share a name and location.
func (c *converter) parameters(item, op *Map) []*Map {
	var out []*Map
	index := map[string]int{}
	for _, list := range [][]any{asSlice(item.Get("parameters")), asSlice(op.Get("parameters"))} {
		for _, raw := range list {
			p := c.resolve(raw)
			if p == nil {
				continue
			}
			key := asString(p.Get("in")) + ":" + asString(p.Get("name"))
			if i, ok := index[key]; ok {
				out[i] = p
				continue
			}
			index[key] = len(out)
			out = append(out, p)
		}
	}
	return out
}

func (c *converter) paramExample(p *Map) string {
	if p.Has("example") {
		return scalarString(p.Get("example"))
	}
	if ex := asMap(p.Get("examples")); ex != nil && len(ex.Keys()) > 0 {
		if v := c.resolve(ex.Get(ex.Keys()[0])); v != nil {
			return scalarString(v.Get("value"))
		}
	}
	// Swagger 2.0 keeps the schema fields on the parameter itself.
	schema := p.Get("schema")
	if c.v2 && schema == nil {
		schema = p
	}
	s := c.resolve(schema)
	switch {
	case s.Has("example"):
		return scalarString(s.Get("example"))
	case s.Has("default"):
		return scalarString(s.Get("default"))
	case len(asSlice(s.Get("enum"))) > 0:
		return scalarString(asSlice(s.Get("enum"))[0])
	}
	return ""
}

func (c *converter) v2Body(op *Map, schema any) engine.Body {
	body := engine.Body{Type: "json", Form: []engine.KeyValue{}}
	consumes := asSlice(op.Get("consumes"))
	if consumes == nil {
		consumes = asSlice(c.root.Get("consumes"))
	}
	for _, ct := range consumes {
		if strings.Contains(asString(ct), "json") {
			break
		}
		if strings.Contains(asString(ct), "xml") {
			body.Type = "xml"
		}
	}
	if body.Type == "json" {
		body.Raw = prettyJSON(c.example(schema, 0, nil))
	}
	return body
}

func (c *converter) v3Body(content *Map) engine.Body {
	body := engine.Body{Type: "none", Form: []engine.KeyValue{}}
	var mediaType string
	for _, ct := range content.Keys() {
		if strings.Contains(ct, "json") {
			mediaType = ct
			break
		}
		if mediaType == "" {
			mediaType = ct
		}
	}
	media := c.resolve(content.Get(mediaType))
	if media == nil {
		return body
	}

	switch {
	case strings.Contains(mediaType, "json"):
		body.Type = "json"
		body.Raw = prettyJSON(c.mediaExample(media))
	case mediaType == "application/x-www-form-urlencoded":
		body.Type = "form"
		schema := c.resolve(media.Get("schema"))
		required := map[string]bool{}
		for _, r := range asSlice(schema.Get("required")) {
			required[asString(r)] = true
		}
		props := asMap(schema.Get("properties"))
		for _, k := range props.Keys() {
			body.Form = append(body.Form, engine.KeyValue{
				Key: k, Value: scalarString(c.example(props.Get(k), 1, nil)), Enabled: required[k],
			})
		}
	case strings.Contains(mediaType, "xml"):
		body.Type = "xml"
		body.Raw = asString(media.Get("example"))
	case strings.HasPrefix(mediaType, "text/"):
		body.Type = "text"
		body.Raw = scalarString(c.mediaExample(media))
	}
	return body
}

func (c *converter) mediaExample(media *Map) any {
	if media.Has("example") {
		return media.Get("example")
	}
	if ex := asMap(media.Get("examples")); ex != nil && len(ex.Keys()) > 0 {
		if v := c.resolve(ex.Get(ex.Keys()[0])); v != nil && v.Has("value") {
			return v.Get("value")
		}
	}
	return c.example(media.Get("schema"), 0, nil)
}

// example builds a sample value for a schema. refs holds the $refs being
// expanded, to stop on recursive schemas.
func (c *converter) example(schema any, depth int, refs []string) any {
	if depth > maxExampleDepth {
		return nil
	}
	if ref := asString(asMap(schema).Get("$ref")); ref != "" {
		for _, r := range refs {
			if r == ref {
				return nil
			}
		}
		return c.example(c.lookup(ref), depth, append(refs, ref))
	}
	s := asMap(schema)
	if s == nil {
		return nil
	}
	switch {
	case s.Has("example"):
		return s.Get("example")
	case s.Has("default"):
		return s.Get("default")
	case len(asSlice(s.Get("enum"))) > 0:
		return asSlice(s.Get("enum"))[0]
	case s.Has("allOf"):
		merged := newMap()
		for _, sub := range asSlice(s.Get("allOf")) {
			if m := asMap(c.example(sub, depth, refs)); m != nil {
				for _, k := range m.Keys() {
					merged.set(k, m.Get(k))
				}
			}
		}
		return merged
	case len(asSlice(s.Get("oneOf"))) > 0:
		return c.example(asSlice(s.Get("oneOf"))[0], depth, refs)
	case len(asSlice(s.Get("anyOf"))) > 0:
		return c.example(asSlice(s.Get("anyOf"))[0], depth, refs)
	}

	switch schemaType(s) {
	case "object":
		obj := newMap()
		props := asMap(s.Get("properties"))
		for _, k := range props.Keys() {
			if asBool(asMap(props.Get(k)).Get("readOnly")) {
				continue // not sent in requests
			}
			obj.set(k, c.example(props.Get(k), depth+1, refs))
		}
		return obj
	case "array":
		if item := c.example(s.Get("items"), depth+1, refs); item != nil {
			return []any{item}
		}
		return []any{}
	case "string":
		return stringExample(asString(s.Get("format")))
	case "integer", "number":
		return 0
	case "boolean":
		return false
	}
	return nil
}

func schemaType(s *Map) string {
	switch t := s.Get("type").(type) {
	case string:
		return t
	case []any: // OpenAPI 3.1: ["string", "null"]
		for _, v := range t {
			if asString(v) != "null" {
				return asString(v)
			}
		}
	}
	if s.Has("properties") {
		return "object"
	}
	if s.Has("items") {
		return "array"
	}
	return ""
}

func stringExample(format string) string {
	switch format {
	case "date":
		return "2026-01-01"
	case "date-time":
		return "2026-01-01T00:00:00Z"
	case "email":
		return "user@example.com"
	case "uuid":
		return "00000000-0000-0000-0000-000000000000"
	case "uri", "url":
		return "https://example.com"
	case "ipv4":
		return "127.0.0.1"
	}
	return "string"
}

// applyAuth maps the first security scheme required by the operation (or
// the document) onto the request.
func (c *converter) applyAuth(r *engine.Request, op *Map) {
	security := c.root.Get("security")
	if op.Has("security") {
		security = op.Get("security")
	}
	reqs := asSlice(security)
	if len(reqs) == 0 {
		return
	}
	names := asMap(reqs[0]).Keys()
	if len(names) == 0 {
		return
	}
	var schemes *Map
	if c.v2 {
		schemes = asMap(c.root.Get("securityDefinitions"))
	} else {
		schemes = asMap(asMap(c.root.Get("components")).Get("securitySchemes"))
	}
	scheme := c.resolve(schemes.Get(names[0]))

	switch asString(scheme.Get("type")) {
	case "basic":
		r.Auth.Type = "basic"
	case "http":
		if strings.EqualFold(asString(scheme.Get("scheme")), "basic") {
			r.Auth.Type = "basic"
		} else {
			r.Auth.Type = "bearer"
		}
	case "oauth2", "openIdConnect":
		r.Auth.Type = "bearer"
	case "apiKey":
		name := asString(scheme.Get("name"))
		switch asString(scheme.Get("in")) {
		case "header":
			r.Headers = append(r.Headers, engine.KeyValue{Key: name, Enabled: true})
		case "query":
			r.Params = append(r.Params, engine.KeyValue{Key: name, Enabled: true})
		case "cookie":
			r.Headers = append(r.Headers, engine.KeyValue{Key: "Cookie", Value: name + "=", Enabled: true})
		}
	}
}

// resolve follows local $refs and returns the target object.
func (c *converter) resolve(v any) *Map {
	m := asMap(v)
	for i := 0; i < 20 && m != nil; i++ {
		ref := asString(m.Get("$ref"))
		if ref == "" {
			return m
		}
		m = asMap(c.lookup(ref))
	}
	return m
}

// lookup resolves a local JSON pointer such as "#/components/schemas/Pet".
// External references are not supported and resolve to nil.
func (c *converter) lookup(ref string) any {
	if !strings.HasPrefix(ref, "#/") {
		return nil
	}
	var cur any = c.root
	for _, part := range strings.Split(ref[2:], "/") {
		if p, err := url.PathUnescape(part); err == nil {
			part = p
		}
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		cur = asMap(cur).Get(part)
		if cur == nil {
			return nil
		}
	}
	return cur
}

// appendQuery adds the enabled params to the URL, as the UI shows them.
func appendQuery(u string, params []engine.KeyValue) string {
	// Same light escaping as the UI: only what would break re-parsing.
	esc := strings.NewReplacer("%", "%25", "&", "%26", "#", "%23", "+", "%2B")
	var pairs []string
	for _, p := range params {
		if p.Enabled {
			pairs = append(pairs, esc.Replace(p.Key)+"="+esc.Replace(p.Value))
		}
	}
	if len(pairs) == 0 {
		return u
	}
	return u + "?" + strings.Join(pairs, "&")
}

func prettyJSON(v any) string {
	if v == nil {
		return ""
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
