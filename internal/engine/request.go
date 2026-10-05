package engine

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// maxBodySize caps how much of a response body is kept for display.
const maxBodySize = 20 << 20

type KeyValue struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Enabled bool   `json:"enabled"`
}

type Auth struct {
	Type     string `json:"type"` // none, bearer, basic
	Token    string `json:"token"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type Body struct {
	Type string     `json:"type"` // none, json, text, xml, form
	Raw  string     `json:"raw"`
	Form []KeyValue `json:"form"`
}

type Request struct {
	ID      string     `json:"id"`
	Name    string     `json:"name"`
	Folder  string     `json:"folder,omitempty"` // group inside its collection
	Method  string     `json:"method"`
	URL     string     `json:"url"`
	Params  []KeyValue `json:"params"`
	Headers []KeyValue `json:"headers"`
	Body    Body       `json:"body"`
	Auth    Auth       `json:"auth"`
}

type Response struct {
	Status      int        `json:"status"`
	StatusText  string     `json:"statusText"`
	Proto       string     `json:"proto"`
	Headers     []KeyValue `json:"headers"`
	ContentType string     `json:"contentType"`
	Body        string     `json:"body"`
	BodyPretty  string     `json:"bodyPretty"`
	BodyBase64  string     `json:"bodyBase64"`
	IsBinary    bool       `json:"isBinary"`
	Truncated   bool       `json:"truncated"`
	Size        int64      `json:"size"`
	DurationMs  int64      `json:"durationMs"`
	URL         string     `json:"url"`
	Error       string     `json:"error"`
}

var defaultContentTypes = map[string]string{
	"json": "application/json",
	"text": "text/plain; charset=utf-8",
	"xml":  "application/xml",
	"form": "application/x-www-form-urlencoded",
}

// buildURL normalizes the raw URL and, when params are provided, replaces its
// query string with the enabled params (the UI keeps both in sync).
func buildURL(raw string, params []KeyValue) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("URL is empty")
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	if i := strings.IndexByte(raw, '#'); i >= 0 {
		raw = raw[:i]
	}
	if len(params) > 0 {
		if i := strings.IndexByte(raw, '?'); i >= 0 {
			raw = raw[:i]
		}
		var pairs []string
		for _, p := range params {
			if p.Enabled && p.Key != "" {
				pairs = append(pairs, url.QueryEscape(p.Key)+"="+url.QueryEscape(p.Value))
			}
		}
		if len(pairs) > 0 {
			raw += "?" + strings.Join(pairs, "&")
		}
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	if u.Host == "" {
		return "", errors.New("invalid URL: missing host")
	}
	return u.String(), nil
}

func buildHTTPRequest(ctx context.Context, r Request) (*http.Request, error) {
	method := strings.ToUpper(strings.TrimSpace(r.Method))
	if method == "" {
		method = http.MethodGet
	}
	target, err := buildURL(r.URL, r.Params)
	if err != nil {
		return nil, err
	}

	var body io.Reader
	switch r.Body.Type {
	case "json", "text", "xml":
		body = strings.NewReader(r.Body.Raw)
	case "form":
		var pairs []string
		for _, f := range r.Body.Form {
			if f.Enabled && f.Key != "" {
				pairs = append(pairs, url.QueryEscape(f.Key)+"="+url.QueryEscape(f.Value))
			}
		}
		body = strings.NewReader(strings.Join(pairs, "&"))
	}

	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Pigeon/0.1")
	if ct, ok := defaultContentTypes[r.Body.Type]; ok {
		req.Header.Set("Content-Type", ct)
	}
	// Explicit headers override the defaults above.
	seen := map[string]bool{}
	for _, h := range r.Headers {
		if !h.Enabled || strings.TrimSpace(h.Key) == "" {
			continue
		}
		key := http.CanonicalHeaderKey(strings.TrimSpace(h.Key))
		if !seen[key] {
			req.Header.Del(key)
			seen[key] = true
		}
		if key == "Host" {
			req.Host = h.Value
			continue
		}
		req.Header.Add(key, h.Value)
	}

	switch r.Auth.Type {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+r.Auth.Token)
	case "basic":
		req.SetBasicAuth(r.Auth.Username, r.Auth.Password)
	}
	return req, nil
}

// Send executes the request and always returns a Response; failures are
// reported in Response.Error rather than as a Go error so the UI can show them.
func Send(ctx context.Context, client *http.Client, r Request) Response {
	req, err := buildHTTPRequest(ctx, r)
	if err != nil {
		return Response{Error: err.Error()}
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return Response{Error: describeError(ctx, err), DurationMs: time.Since(start).Milliseconds(), URL: req.URL.String()}
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
	if err != nil {
		return Response{Error: describeError(ctx, err), DurationMs: time.Since(start).Milliseconds(), URL: req.URL.String()}
	}
	size := int64(len(data))
	rest, _ := io.Copy(io.Discard, resp.Body)
	size += rest

	out := Response{
		Status:      resp.StatusCode,
		StatusText:  strings.TrimSpace(strings.TrimPrefix(resp.Status, fmt.Sprint(resp.StatusCode))),
		Proto:       resp.Proto,
		Headers:     flattenHeaders(resp.Header),
		ContentType: resp.Header.Get("Content-Type"),
		Size:        size,
		Truncated:   rest > 0,
		DurationMs:  time.Since(start).Milliseconds(),
		URL:         resp.Request.URL.String(),
	}

	switch {
	case strings.HasPrefix(out.ContentType, "image/"):
		out.IsBinary = true
		out.BodyBase64 = base64.StdEncoding.EncodeToString(data)
	case !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0:
		out.IsBinary = true
	default:
		out.Body = string(data)
		if json.Valid(data) {
			var buf bytes.Buffer
			if json.Indent(&buf, data, "", "  ") == nil {
				out.BodyPretty = buf.String()
			}
		}
	}
	return out
}

func flattenHeaders(h http.Header) []KeyValue {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []KeyValue
	for _, k := range keys {
		for _, v := range h[k] {
			out = append(out, KeyValue{Key: k, Value: v, Enabled: true})
		}
	}
	return out
}

func describeError(ctx context.Context, err error) string {
	if errors.Is(ctx.Err(), context.Canceled) {
		return "Request cancelled"
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err.Error()
	}
	return err.Error()
}
