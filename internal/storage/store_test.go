package storage

import (
	"testing"

	"github.com/daniel-sabin/pigeon/internal/engine"
)

func TestStore(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	cols, err := s.Collections()
	if err != nil || len(cols) != 0 {
		t.Fatalf("empty store: %v %v", cols, err)
	}
	want := []Collection{{ID: "c1", Name: "API", Requests: []engine.Request{{ID: "r1", Name: "List", Method: "GET", URL: "x.io"}}}}
	if err := s.SaveCollections(want); err != nil {
		t.Fatal(err)
	}
	cols, err = s.Collections()
	if err != nil || len(cols) != 1 || cols[0].Requests[0].URL != "x.io" {
		t.Fatalf("collections round trip: %+v %v", cols, err)
	}

	for i := 0; i < maxHistory+5; i++ {
		if err := s.AddHistory(HistoryEntry{ID: "h", Status: i}); err != nil {
			t.Fatal(err)
		}
	}
	h, err := s.History()
	if err != nil || len(h) != maxHistory || h[0].Status != maxHistory+4 {
		t.Fatalf("history: len=%d first=%+v err=%v", len(h), h[0], err)
	}
	if err := s.ClearHistory(); err != nil {
		t.Fatal(err)
	}
	if h, _ := s.History(); len(h) != 0 {
		t.Fatalf("history not cleared: %d", len(h))
	}
}
