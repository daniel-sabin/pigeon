package engine

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBuildURL(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		params []KeyValue
		want   string
	}{
		{"adds scheme", "example.com/a", nil, "http://example.com/a"},
		{"keeps query without params", "https://x.io/?a=1&b=2", nil, "https://x.io/?a=1&b=2"},
		{"params replace query", "https://x.io/p?old=1", []KeyValue{
			{Key: "q", Value: "a b&c", Enabled: true},
			{Key: "off", Value: "1", Enabled: false},
			{Key: "", Value: "ignored", Enabled: true},
		}, "https://x.io/p?q=a+b%26c"},
		{"drops fragment", "https://x.io/p#frag", nil, "https://x.io/p"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildURL(tt.raw, tt.params)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}

	for _, bad := range []string{"", "   ", "http://"} {
		if _, err := buildURL(bad, nil); err == nil {
			t.Errorf("buildURL(%q): expected error", bad)
		}
	}
}

func TestSend(t *testing.T) {
	var gotMethod, gotBody, gotCT, gotAuth, gotCustom string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotMethod, gotBody = r.Method, string(b)
		gotCT, gotAuth, gotCustom = r.Header.Get("Content-Type"), r.Header.Get("Authorization"), r.Header.Get("X-Custom")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":12345678901234567890,"ok":true}`))
	}))
	defer srv.Close()

	resp := Send(context.Background(), srv.Client(), Request{
		Method:  "post",
		URL:     srv.URL + "/items",
		Headers: []KeyValue{{Key: "x-custom", Value: "yes", Enabled: true}, {Key: "X-Off", Value: "no"}},
		Body:    Body{Type: "json", Raw: `{"name":"a"}`},
		Auth:    Auth{Type: "bearer", Token: "tok"},
	})

	if resp.Error != "" {
		t.Fatal(resp.Error)
	}
	if gotMethod != "POST" || gotBody != `{"name":"a"}` || gotCT != "application/json" || gotAuth != "Bearer tok" || gotCustom != "yes" {
		t.Errorf("server saw method=%q body=%q ct=%q auth=%q custom=%q", gotMethod, gotBody, gotCT, gotAuth, gotCustom)
	}
	if resp.Status != 201 || resp.StatusText != "Created" {
		t.Errorf("status = %d %q", resp.Status, resp.StatusText)
	}
	if !strings.Contains(resp.BodyPretty, "12345678901234567890") || !strings.Contains(resp.BodyPretty, "\n  \"ok\": true") {
		t.Errorf("pretty body not indented or lost precision: %q", resp.BodyPretty)
	}
	if resp.Size != int64(len(resp.Body)) {
		t.Errorf("size = %d, body len = %d", resp.Size, len(resp.Body))
	}
}

func TestSendFormAndHeaderOverride(t *testing.T) {
	var gotBody, gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody, gotCT = string(b), r.Header.Get("Content-Type")
	}))
	defer srv.Close()

	resp := Send(context.Background(), srv.Client(), Request{
		Method:  "POST",
		URL:     srv.URL,
		Headers: []KeyValue{{Key: "Content-Type", Value: "application/vnd.custom", Enabled: true}},
		Body:    Body{Type: "form", Form: []KeyValue{{Key: "a", Value: "1 2", Enabled: true}, {Key: "b", Value: "x", Enabled: false}}},
	})
	if resp.Error != "" {
		t.Fatal(resp.Error)
	}
	if gotBody != "a=1+2" || gotCT != "application/vnd.custom" {
		t.Errorf("body=%q ct=%q", gotBody, gotCT)
	}
}

func TestSendBinaryAndCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slow" {
			select {
			case <-r.Context().Done():
			case <-time.After(5 * time.Second):
			}
			return
		}
		w.Write([]byte{0xff, 0x00, 0x01})
	}))
	defer srv.Close()

	resp := Send(context.Background(), srv.Client(), Request{URL: srv.URL + "/bin"})
	if !resp.IsBinary || resp.Body != "" || resp.Size != 3 {
		t.Errorf("binary response: %+v", resp)
	}

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	resp = Send(ctx, srv.Client(), Request{URL: srv.URL + "/slow"})
	if resp.Error != "Request cancelled" {
		t.Errorf("error = %q", resp.Error)
	}
}

func TestSendConnectionError(t *testing.T) {
	resp := Send(context.Background(), http.DefaultClient, Request{URL: "http://127.0.0.1:1"})
	if resp.Error == "" || resp.Status != 0 {
		t.Errorf("expected connection error, got %+v", resp)
	}
}
