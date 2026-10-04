package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/daniel-sabin/pigeon/internal/engine"
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

// SendRequest executes req and records it in the history. runID identifies the
// call so it can be aborted with CancelRequest.
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

	resp := engine.Send(ctx, a.client, req)

	entry := storage.HistoryEntry{
		ID:         newID(),
		Request:    req,
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

func (a *App) GetHistory() ([]storage.HistoryEntry, error) {
	return a.store.History()
}

func (a *App) ClearHistory() error {
	return a.store.ClearHistory()
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
