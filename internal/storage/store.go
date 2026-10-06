package storage

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/daniel-sabin/pigeon/internal/engine"
)

const maxHistory = 300

type Collection struct {
	ID       string           `json:"id"`
	Name     string           `json:"name"`
	Source   string           `json:"source,omitempty"` // URL or file it was imported from
	Requests []engine.Request `json:"requests"`
}

type HistoryEntry struct {
	ID         string         `json:"id"`
	Request    engine.Request `json:"request"`
	Status     int            `json:"status"`
	Error      string         `json:"error"`
	DurationMs int64          `json:"durationMs"`
	Timestamp  int64          `json:"timestamp"` // unix milliseconds
}

// Store persists collections and history as JSON files in a directory.
type Store struct {
	dir string
	mu  sync.Mutex
}

func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

func DefaultDataDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "Pigeon"), nil
}

func (s *Store) Collections() ([]Collection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cols := []Collection{}
	if err := s.read("collections.json", &cols); err != nil {
		return nil, err
	}
	return cols, nil
}

func (s *Store) SaveCollections(cols []Collection) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.write("collections.json", cols)
}

// Variables are global: {{name}} placeholders resolve against them in every
// request, wherever it was opened from.
func (s *Store) Variables() ([]engine.KeyValue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	vars := []engine.KeyValue{}
	if err := s.read("variables.json", &vars); err != nil {
		return nil, err
	}
	return vars, nil
}

func (s *Store) SaveVariables(vars []engine.KeyValue) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.write("variables.json", vars)
}

func (s *Store) History() ([]HistoryEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.history()
}

func (s *Store) AddHistory(e HistoryEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.history()
	if err != nil {
		return err
	}
	entries = append([]HistoryEntry{e}, entries...)
	if len(entries) > maxHistory {
		entries = entries[:maxHistory]
	}
	return s.write("history.json", entries)
}

func (s *Store) ClearHistory() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.write("history.json", []HistoryEntry{})
}

func (s *Store) history() ([]HistoryEntry, error) {
	entries := []HistoryEntry{}
	if err := s.read("history.json", &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func (s *Store) read(name string, v any) error {
	data, err := os.ReadFile(filepath.Join(s.dir, name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// write replaces the file atomically so a crash never leaves it half-written.
func (s *Store) write(name string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.dir, name+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(s.dir, name))
}
