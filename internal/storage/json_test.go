package storage

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestConcurrentPublicationKeepsOneCompleteFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "graph.dot")
	first, second := bytes.Repeat([]byte("a"), 65536), bytes.Repeat([]byte("b"), 65536)
	start := make(chan struct{})
	results := make(chan error, 2)
	var workers sync.WaitGroup
	for _, payload := range [][]byte{first, second} {
		workers.Go(func() {
			<-start
			results <- WriteNewBytes(path, payload)
		})
	}
	close(start)
	workers.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if !os.IsExist(err) {
			t.Fatalf("unexpected publication error: %v", err)
		}
	}
	got, err := os.ReadFile(path)
	if err != nil || successes != 1 || (!bytes.Equal(got, first) && !bytes.Equal(got, second)) {
		t.Fatalf("expected one complete winner; successes=%d bytes=%d err=%v", successes, len(got), err)
	}
	leftovers, err := filepath.Glob(filepath.Join(dir, ".pmiyc-*"))
	if err != nil || len(leftovers) != 0 {
		t.Fatalf("temporary files left behind: %v %v", leftovers, err)
	}
}

func TestWriteNewPreservesExistingFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "case.json")
	if err := WriteNew(p, map[string]int{"value": 1}); err != nil {
		t.Fatal(err)
	}
	if err := WriteNew(p, map[string]int{"value": 2}); err == nil {
		t.Fatal("existing file overwritten")
	}
	var got map[string]int
	if err := Read(p, &got); err != nil {
		t.Fatal(err)
	}
	if got["value"] != 1 {
		t.Fatal("original changed")
	}
}
