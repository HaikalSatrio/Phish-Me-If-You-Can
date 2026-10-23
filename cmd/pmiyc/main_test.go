package main

import (
	"path/filepath"
	"strings"
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

func TestEnrichPreservesInputAndRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "enriched.json")
	args := []string{"enrich", "-input", "../../fixtures/kevin.json", "-out", out}
	if err := run(args); err != nil {
		t.Fatal(err)
	}
	var saved domain.Case
	if err := storage.Read(out, &saved); err != nil {
		t.Fatal(err)
	}
	if len(saved.Extraction) != len(saved.Candidates) {
		t.Fatal("missing extraction audit")
	}
	if err := run(args); err == nil {
		t.Fatal("overwrote enrichment")
	}
}

func TestSearchNameAndUsernameGuidance(t *testing.T) {
	err := run([]string{"search", "-source", "web", "-name", "Haikal", "Satrio", "Dewandaru"})
	if err == nil || !strings.Contains(err.Error(), `-name "Haikal Satrio Dewandaru"`) {
		t.Fatalf("missing multiword guidance: %v", err)
	}
	err = run([]string{"search", "-source", "devto", "-name", "Ben", "-config", "../../configs/collector-v1.json"})
	if err == nil || !strings.Contains(err.Error(), "exact usernames") || !strings.Contains(err.Error(), "-platform devto") {
		t.Fatalf("missing people search guidance: %v", err)
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

func TestDefaultUsernameSourceAndResume(t *testing.T) {
	dir := t.TempDir()
	for _, source := range []string{"", "direct"} {
		input, out := filepath.Join(dir, "case-"+source+".json"), filepath.Join(dir, "resumed-"+source+".json")
		args := []string{"search", "-username", "!", "-config", "../../configs/collector-v1.json", "-out", input}
		if source != "" {
			args = append(args, "-source", source)
		}
		if err := run(args); err == nil {
			t.Fatal("invalid username reported success")
		}
		var saved domain.Case
		if err := storage.Read(input, &saved); err != nil {
			t.Fatal(err)
		}
		want := source
		if want == "" {
			want = "auto"
		}
		if saved.Search.Provider != want || saved.Search.RequestsUsed != 0 {
			t.Fatalf("unexpected routing: %+v", saved.Search)
		}
		if err := run([]string{"search", "-input", input, "-config", "../../configs/collector-v1.json", "-out", out}); err == nil {
			t.Fatal("invalid resumed username reported success")
		}
		if err := storage.Read(out, &saved); err != nil {
			t.Fatal(err)
		}
		if saved.Search.Provider != want {
			t.Fatal("resume changed source")
		}
	}
}
