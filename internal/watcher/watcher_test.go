package watcher

import (
	"sync"
	"testing"
	"time"
)

func TestDebouncer_FiresAfterInterval(t *testing.T) {
	var mu sync.Mutex
	var got []string
	done := make(chan struct{})

	d := NewDebouncer(50*time.Millisecond, func(paths []string) {
		mu.Lock()
		got = append(got, paths...)
		mu.Unlock()
		close(done)
	})
	defer d.Stop()

	d.Add("file1.go")
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for debouncer to fire")
	}

	mu.Lock()
	if len(got) != 1 || got[0] != "file1.go" {
		t.Errorf("expected [file1.go], got %v", got)
	}
	mu.Unlock()
}

func TestDebouncer_CoalescesMultiple(t *testing.T) {
	var mu sync.Mutex
	var got []string
	done := make(chan struct{})

	d := NewDebouncer(50*time.Millisecond, func(paths []string) {
		mu.Lock()
		got = append(got, paths...)
		mu.Unlock()
		close(done)
	})
	defer d.Stop()

	d.Add("file1.go")
	time.Sleep(5 * time.Millisecond)
	d.Add("file2.go")
	time.Sleep(5 * time.Millisecond)
	d.Add("file1.go") // duplicate

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for debouncer to fire")
	}

	mu.Lock()
	if len(got) != 2 {
		t.Errorf("expected 2 unique paths, got %d: %v", len(got), got)
	}
	mu.Unlock()
}

func TestDebouncer_StopCancels(t *testing.T) {
	fired := make(chan struct{}, 1)

	d := NewDebouncer(100*time.Millisecond, func(paths []string) {
		fired <- struct{}{}
	})

	d.Add("file.go")
	d.Stop()

	select {
	case <-fired:
		t.Error("debouncer fired after stop")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestDebouncer_EmptyFire(t *testing.T) {
	// fire() with empty set should not panic
	d := NewDebouncer(50*time.Millisecond, func(paths []string) {})
	d.fire()
}

func TestShouldSkip(t *testing.T) {
	tests := []struct {
		path string
		skip bool
	}{
		{"main.go", false},
		{".git/config", true},
		{"src/.hidden", true},
		{"node_modules/pkg/index.js", true},
		{"vendor/pkg/main.go", true},
		{"src/util.go", false},
	}

	w := &Watcher{
		repoPath: "/test",
		indexer:  nil,
	}

	for _, tc := range tests {
		got := w.shouldSkip(tc.path)
		if got != tc.skip {
			t.Errorf("shouldSkip(%q) = %v, want %v", tc.path, got, tc.skip)
		}
	}
}
