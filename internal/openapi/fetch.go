package openapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/daniel-sabin/pigeon/internal/storage"
)

const maxSpecSize = 50 << 20

// Common locations of the spec, relative to a Swagger UI page or the site root.
var wellKnownPaths = []string{
	"openapi.json", "openapi.yaml", "swagger.json", "swagger.yaml",
	"v3/api-docs", "v2/api-docs", "api-docs", "swagger/v1/swagger.json",
}

var (
	// Spec URLs in a Swagger UI page or its initializer script:
	// url: "...", configUrl: "...", "url":"..."
	specURLPattern = regexp.MustCompile(`["']?(?:url|configUrl)["']?\s*[:=]\s*["']([^"']+)["']`)
	scriptPattern  = regexp.MustCompile(`<script[^>]+src=["']([^"']*initializer[^"']*\.js)["']`)
	// Any other address that looks like a spec, quoted or inside a string.
	specLikePattern = regexp.MustCompile(`(?:https?://|["'\x60]/?)[^\s"'\x60,<>()=]*(?:\.json|\.ya?ml|api-docs)\b[^\s"'\x60,<>()]*`)
)

// Fetch downloads a spec and converts it. rawURL may point at the spec itself
// or at a Swagger UI page, in which case the spec URL is discovered.
func Fetch(ctx context.Context, client *http.Client, rawURL string) (storage.Collection, error) {
	start, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || start.Host == "" {
		if !strings.Contains(rawURL, "://") {
			start, err = url.Parse("https://" + strings.TrimSpace(rawURL))
		}
		if err != nil || start.Host == "" {
			return storage.Collection{}, fmt.Errorf("invalid URL %q", rawURL)
		}
	}

	f := &fetcher{ctx: ctx, client: client, tried: map[string]bool{}}
	col, err := f.try(start, 0)
	if err == nil {
		return col, nil
	}
	if f.lastHTML == nil {
		return storage.Collection{}, err
	}

	// The URL served a web page: look for the spec it refers to, then in the
	// usual places.
	page := f.lastHTML
	var candidates []*url.URL
	if q := page.Query().Get("url"); q != "" { // Swagger UI's ?url= parameter
		if u, err := page.Parse(q); err == nil {
			candidates = append(candidates, u)
		}
	}
	sources := [][]byte{f.lastBody}
	for _, m := range scriptPattern.FindAllSubmatch(f.lastBody, -1) {
		if script, err := page.Parse(string(m[1])); err == nil {
			if body, err := f.get(script); err == nil {
				sources = append(sources, body)
			}
		}
	}
	var found []*url.URL
	for _, src := range sources {
		found = append(found, refsIn(src, page)...)
	}
	// Prefer documents hosted next to the page.
	sort.SliceStable(found, func(i, j int) bool {
		return found[i].Host == page.Host && found[j].Host != page.Host
	})
	candidates = append(candidates, found...)
	dir := *page
	if !strings.HasSuffix(dir.Path, "/") {
		dir.Path = dir.Path[:strings.LastIndex(dir.Path, "/")+1]
	}
	for _, p := range wellKnownPaths {
		if u, err := dir.Parse(p); err == nil {
			candidates = append(candidates, u)
		}
		if u, err := page.Parse("/" + p); err == nil {
			candidates = append(candidates, u)
		}
	}
	for _, u := range candidates {
		if col, err := f.try(u, 0); err == nil {
			return col, nil
		}
	}
	return storage.Collection{}, errors.New("no OpenAPI or Swagger document found at this address; try the URL of the JSON or YAML file itself")
}

type fetcher struct {
	ctx      context.Context
	client   *http.Client
	tried    map[string]bool
	lastHTML *url.URL // first page that returned HTML
	lastBody []byte
}

// try fetches u and parses it as a spec. A Swagger UI config document
// ({"url": ...} or {"urls": [...]}) is followed once.
func (f *fetcher) try(u *url.URL, depth int) (storage.Collection, error) {
	if f.tried[u.String()] {
		return storage.Collection{}, errors.New("already tried")
	}
	f.tried[u.String()] = true

	body, err := f.get(u)
	if err != nil {
		return storage.Collection{}, err
	}
	if isHTML(body) {
		if f.lastHTML == nil {
			f.lastHTML, f.lastBody = u, body
		}
		return storage.Collection{}, errors.New("got an HTML page, not an OpenAPI document")
	}

	col, err := Parse(body, u.String())
	if err == nil || depth > 0 {
		return col, err
	}
	if doc, derr := decode(body); derr == nil {
		cfg := asMap(doc)
		next := asString(cfg.Get("url"))
		if urls := asSlice(cfg.Get("urls")); next == "" && len(urls) > 0 {
			next = asString(asMap(urls[0]).Get("url"))
		}
		if next != "" {
			if nu, perr := u.Parse(next); perr == nil {
				return f.try(nu, depth+1)
			}
		}
	}
	return col, err
}

func (f *fetcher) get(u *url.URL) ([]byte, error) {
	req, err := http.NewRequestWithContext(f.ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json, application/yaml;q=0.9, text/yaml;q=0.9, */*;q=0.5")
	req.Header.Set("User-Agent", "Pigeon/0.1")
	resp, err := f.client.Do(req)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			return nil, urlErr.Err
		}
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%s returned %s", u, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSpecSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxSpecSize {
		return nil, errors.New("document is larger than 50 MB")
	}
	return data, nil
}

// refsIn lists the addresses in a page or script that may point at a spec:
// explicit Swagger UI "url" settings first, then anything spec-like.
func refsIn(body []byte, base *url.URL) []*url.URL {
	var refs []string
	for _, m := range specURLPattern.FindAllSubmatch(body, -1) {
		refs = append(refs, string(m[1]))
	}
	for _, m := range specLikePattern.FindAll(body, -1) {
		refs = append(refs, strings.TrimLeft(string(m), "\"'\x60"))
	}
	var out []*url.URL
	for _, ref := range refs {
		if strings.HasPrefix(ref, "#") || strings.HasPrefix(ref, "javascript:") || strings.HasSuffix(ref, "package.json") {
			continue
		}
		if u, err := base.Parse(ref); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
			out = append(out, u)
		}
	}
	return out
}

func isHTML(body []byte) bool {
	head := strings.ToLower(strings.TrimSpace(string(body[:min(len(body), 1024)])))
	return strings.HasPrefix(head, "<") && (strings.Contains(head, "<html") || strings.Contains(head, "<!doctype html"))
}
