package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/enrich"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/extract"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/storage"
)

func TestReviewRelationCLI(t *testing.T) {
	c := domain.Case{SchemaVersion: "1", Target: domain.Target{ID: "test", Name: "Ayu Lestari"}, Candidates: []domain.Candidate{{ID: "ayu", Decision: "PENDING"}}, Evidence: []domain.Evidence{{ID: "doc", CandidateID: "ayu", Type: "page_text", Value: "Ayu Lestari works as Engineer at Orbit Labs.", Status: "OBSERVED", Sources: []domain.Source{{URL: "https://example.org/ayu", ObservedAt: "2026-10-24T00:00:00Z", Collector: "fixture"}}}}}
	c, err := enrich.Run(context.Background(), c, extract.Config{})
	if err != nil {
		t.Fatal(err)
	}
	c.Relations[0].Method = "ollama/" + extract.RelationVersion
	enrich.Reproject(&c)
	dir := t.TempDir()
	input, out := filepath.Join(dir, "input.json"), filepath.Join(dir, "reviewed.json")
	if err := storage.WriteNew(input, c); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(input)
	args := []string{"review-relation", "-input", input, "-out", out, "-relation", c.Relations[0].ID, "-decision", "ACCEPT", "-reason", "Source explicitly states employment"}
	if err := run(args); err != nil {
		t.Fatal(err)
	}
	var got domain.Case
	if err := storage.Read(out, &got); err != nil {
		t.Fatal(err)
	}
	if got.Candidates[0].Role != "Engineer" || got.Candidates[0].Decision != "PENDING" || got.Relations[0].Status != "UNVERIFIED" || domain.RelationDecision(got.Relations[0]) != "ACCEPT" {
		t.Fatalf("incorrect review: %+v", got)
	}
	if err := run(args); err == nil {
		t.Fatal("overwrote output")
	}
	after, _ := os.ReadFile(input)
	if string(after) != string(before) {
		t.Fatal("modified original snapshot")
	}
}
