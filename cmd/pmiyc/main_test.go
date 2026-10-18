package main

import (
	"path/filepath"
	"testing"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/storage"
)

func TestImportReviewAnalyze(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "project.json")
	reviewed := filepath.Join(dir, "reviewed.json")
	report := filepath.Join(dir, "report.json")
	commands := [][]string{
		{"import", "-input", "../../fixtures/kevin.json", "-out", project},
		{"review", "-input", project, "-candidate", "kevin-profile-other", "-decision", "REJECT", "-out", reviewed},
		{"analyze", "-input", reviewed, "-policy", "../../configs/experimental-v1.json", "-out", report},
	}
	for _, args := range commands {
		if err := run(args); err != nil {
			t.Fatal(err)
		}
	}
	var r domain.Report
	if err := storage.Read(report, &r); err != nil {
		t.Fatal(err)
	}
	if r.PES == nil || r.IdentityStates["kevin-profile-other"] != "REJECTED" || len(r.InputSHA256) != 64 {
		t.Fatal("incomplete report")
	}
	if err := run(commands[0]); err == nil {
		t.Fatal("overwrote saved project")
	}
}

func TestSearchReviewSelectsOnlyOneCandidate(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "search.json")
	out := filepath.Join(dir, "reviewed.json")
	c := domain.Case{SchemaVersion: "1", Target: domain.Target{ID: "test", Name: "Anton"}, Search: &domain.SearchSession{}, Candidates: []domain.Candidate{
		{ID: "one", Decision: "CONFIRM", RequiresReview: true, Confidence: 0.6},
		{ID: "two", Decision: "PENDING", RequiresReview: true, Confidence: 0.6},
	}}
	if err := storage.WriteNew(input, c); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"review", "-input", input, "-candidate", "two", "-decision", "CONFIRM", "-out", out}); err != nil {
		t.Fatal(err)
	}
	var saved domain.Case
	if err := storage.Read(out, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Candidates[0].Decision != "PENDING" || saved.Candidates[1].Decision != "CONFIRM" || len(saved.Search.Events) != 1 {
		t.Fatal("review did not preserve exclusive selection and history")
	}
}
