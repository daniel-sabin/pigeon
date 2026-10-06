package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/daniel-sabin/pigeon/internal/engine"
	"github.com/daniel-sabin/pigeon/internal/openapi"
	"github.com/daniel-sabin/pigeon/internal/storage"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is bound to the frontend: its exported methods are callable from JS.
type App struct {
	ctx    context.Context
	store  *storage.Store
	client *http.Client

	mu       sync.Mutex
	inflight map[string]context.CancelFunc
}

func NewApp(store *storage.Store) *App {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	return &App{
		store:    store,
		client:   &http.Client{Transport: transport},
		inflight: map[string]context.CancelFunc{},
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// SendRequest resolves {{variables}} in req, executes it and records it in the
// history. runID identifies the call so it can be aborted with CancelRequest.
func (a *App) SendRequest(runID string, req engine.Request) engine.Response {
	ctx, cancel := context.WithCancel(a.ctx)
	a.mu.Lock()
	a.inflight[runID] = cancel
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		delete(a.inflight, runID)
		a.mu.Unlock()
		cancel()
	}()

	vars, err := a.store.Variables()
	if err != nil {
		runtime.LogErrorf(a.ctx, "loading variables: %v", err)
	}
	resp := engine.Send(ctx, a.client, engine.ApplyVariables(req, vars))

	entry := storage.HistoryEntry{
		ID:         newID(),
		Request:    req, // placeholders, not the resolved secrets
		Status:     resp.Status,
		Error:      resp.Error,
		DurationMs: resp.DurationMs,
		Timestamp:  time.Now().UnixMilli(),
	}
	if err := a.store.AddHistory(entry); err != nil {
		runtime.LogErrorf(a.ctx, "saving history: %v", err)
	}
	return resp
}

func (a *App) CancelRequest(runID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if cancel, ok := a.inflight[runID]; ok {
		cancel()
	}
}

func (a *App) GetCollections() ([]storage.Collection, error) {
	return a.store.Collections()
}

func (a *App) SaveCollections(cols []storage.Collection) error {
	return a.store.SaveCollections(cols)
}

func (a *App) GetVariables() ([]engine.KeyValue, error) {
	return a.store.Variables()
}

func (a *App) SaveVariables(vars []engine.KeyValue) error {
	return a.store.SaveVariables(vars)
}

func (a *App) GetHistory() ([]storage.HistoryEntry, error) {
	return a.store.History()
}

func (a *App) ClearHistory() error {
	return a.store.ClearHistory()
}

// ImportOpenAPIURL builds a collection from a Swagger/OpenAPI document URL,
// or from a Swagger UI page that references one.
func (a *App) ImportOpenAPIURL(rawURL string) (storage.Collection, error) {
	ctx, cancel := context.WithTimeout(a.ctx, time.Minute)
	defer cancel()
	return openapi.Fetch(ctx, a.client, rawURL)
}

// ImportOpenAPIFile asks for a local spec file and builds a collection from
// it. It returns nil when the dialog is cancelled.
func (a *App) ImportOpenAPIFile() (*storage.Collection, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Import an OpenAPI / Swagger file",
		Filters: []runtime.FileFilter{
			{DisplayName: "OpenAPI / Swagger (*.json, *.yaml, *.yml)", Pattern: "*.json;*.yaml;*.yml"},
		},
	})
	if err != nil || path == "" {
		return nil, err
	}
	col, err := a.ImportOpenAPIPath(path)
	if err != nil {
		return nil, err
	}
	return &col, nil
}

// ImportOpenAPIPath builds a collection from a spec file on disk (used for
// files dropped on the window).
func (a *App) ImportOpenAPIPath(path string) (storage.Collection, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return storage.Collection{}, fmt.Errorf("reading %s: %w", path, err)
	}
	return openapi.Parse(data, path)
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
