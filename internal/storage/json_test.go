package storage

import (
	"path/filepath"
	"testing"
)

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
