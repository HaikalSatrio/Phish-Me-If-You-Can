package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/graph"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/storage"
)

func TestGraphRejectsInvalidPolicyBeforePublishing(t *testing.T) {
	var policy domain.Policy
	if err := storage.Read("../../configs/experimental-v1.json", &policy); err != nil {
		t.Fatal(err)
	}
	policy.AcceptThreshold = 0
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "invalid-policy.json")
	if err := storage.WriteNew(policyPath, policy); err != nil {
		t.Fatal(err)
	}
	out, dot := filepath.Join(dir, "graph.json"), filepath.Join(dir, "graph.dot")
	err := runGraph([]string{"-input", "../../fixtures/kevin.json", "-policy", policyPath, "-out", out, "-dot", dot})
	if err == nil || !strings.Contains(err.Error(), "graph policy") {
		t.Fatalf("invalid policy was not rejected: %v", err)
	}
	for _, path := range []string{out, dot} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("invalid policy published output %s: %v", path, err)
		}
	}
}

func TestGraphPublishesJSONAndDOTWithoutOverwrite(t *testing.T) {
	dir := t.TempDir()
	out, dot := filepath.Join(dir, "graph.json"), filepath.Join(dir, "graph.dot")
	args := []string{"-input", "../../fixtures/kevin.json", "-policy", "../../configs/experimental-v1.json", "-out", out, "-dot", dot}
	if err := runGraph(args); err != nil {
		t.Fatal(err)
	}
	var g graph.Graph
	if err := storage.Read(out, &g); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(dot)
	if err != nil || string(b) != graph.DOT(g) {
		t.Fatalf("DOT does not match saved graph: %v", err)
	}
	if err := runGraph(args); err == nil {
		t.Fatal("existing outputs were accepted")
	}
}

func TestGraphRejectsSameOutputPathBeforePublishing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "graph.json")
	err := runGraph([]string{"-input", "../../fixtures/kevin.json", "-out", path, "-dot", path})
	if err == nil || !strings.Contains(err.Error(), "different paths") {
		t.Fatalf("same output paths not rejected: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("output created before validation: %v", err)
	}
}
