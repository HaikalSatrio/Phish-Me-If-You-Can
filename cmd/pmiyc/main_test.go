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
