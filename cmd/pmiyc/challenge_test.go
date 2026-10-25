package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/enrich"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/evidence"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/extract"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/identity"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/scoring"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/storage"
)

func challengeFixture(t *testing.T) domain.Case {
	t.Helper()
	c := domain.Case{SchemaVersion: "1", Target: domain.Target{ID: "case", Name: "Sari Sari"}, Search: &domain.SearchSession{Provider: "web", Name: "Sari Sari"}, Candidates: []domain.Candidate{
		{ID: "ig", RequiresReview: true, ProfileURL: "https://instagram.com/sari_dev", Decision: "PENDING"},
		{ID: "li", RequiresReview: true, ProfileURL: "https://linkedin.com/in/sari-dev", Decision: "PENDING"},
	}}
	c.Coverage = []domain.Coverage{{Source: "fixture", Status: "SUCCESS"}}
	for _, item := range []struct{ candidate, typ, text string }{
		{"ig", "page_title", "Sari (@sari_dev) | Instagram"},
		{"ig", "search_snippet", `100 followers · @sari_dev: “Sharing thoughts Cyber Security Lecturer ❣ Mom ...`},
		{"li", "search_title", "Sari Sari - CHFI | Deputy Head of Cyber Security Program at ..."},
		{"li", "search_snippet", "CHFI | Deputy Head of Cyber Security Program at Example University · Experience: Example University"},
	} {
		c.Evidence = append(c.Evidence, domain.Evidence{ID: item.candidate + "-" + item.typ, CandidateID: item.candidate, Type: item.typ, Value: item.text, Status: "OBSERVED", Confidence: 1, Sources: []domain.Source{{URL: "https://search.example/", Collector: "fixture", ObservedAt: "2026-10-25T00:00:00Z"}}})
	}
	got, err := enrich.Run(context.Background(), c, extract.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Candidates[0].DisplayName != "Sari" || got.Candidates[0].Role != "Cyber Security Lecturer" || got.Candidates[1].DisplayName != "Sari Sari" || got.Candidates[1].Company != "Example University" || len(got.Relations) != 2 {
		t.Fatalf("profile extraction failed: %+v", got.Candidates)
	}
	return got
}

func TestInteractiveIdentityChallengePreservesEvidenceAndClaims(t *testing.T) {
	c := challengeFixture(t)
	dir := t.TempDir()
	input, out := filepath.Join(dir, "input.json"), filepath.Join(dir, "reviewed.json")
	if err := storage.WriteNew(input, c); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := challenge([]string{"-input", input, "-out", out}, strings.NewReader("1,2\nKnown public profiles of the same volunteer\n"), &output); err != nil {
		t.Fatal(err)
	}
	var reviewed domain.Case
	if err := storage.Read(out, &reviewed); err != nil {
		t.Fatal(err)
	}
	if len(reviewed.IdentityReviews) != 1 || reviewed.Candidates[0].Decision != "CONFIRM" || reviewed.Candidates[1].Decision != "CONFIRM" {
		t.Fatal("multi-profile review not saved")
	}
	if !reflect.DeepEqual(reviewed.Evidence, c.Evidence) || !reflect.DeepEqual(reviewed.Relations, c.Relations) {
		t.Fatal("identity confirmation changed source claims")
	}
	var policy domain.Policy
	if err := storage.Read("../../configs/experimental-v1.json", &policy); err != nil {
		t.Fatal(err)
	}
	report, err := scoring.Calculate(reviewed, policy)
	if err != nil || report.PES != nil {
		t.Fatalf("profile metadata or unverified claims entered scoring: %+v %v", report, err)
	}
	if !strings.Contains(output.String(), "Cyber Security Lecturer") || !strings.Contains(output.String(), "Subject title:") {
		t.Fatal("review omitted supporting evidence")
	}
	again, err := enrich.Run(context.Background(), reviewed, extract.Config{})
	if err != nil || !reflect.DeepEqual(again.IdentityReviews, reviewed.IdentityReviews) {
		t.Fatalf("review lost during replay: %v", err)
	}
	if err := run([]string{"review", "-input", out, "-out", filepath.Join(dir, "changed.json"), "-candidate", "li", "-decision", "REJECT"}); err != nil {
		t.Fatalf("single-candidate correction failed: %v", err)
	}
	var original domain.Case
	if err := storage.Read(input, &original); err != nil || original.Candidates[0].Decision != "PENDING" {
		t.Fatal("input overwritten")
	}
}

func TestChallengeCancellationInvalidSelectionAndForgedBinding(t *testing.T) {
	c := challengeFixture(t)
	dir := t.TempDir()
	input := filepath.Join(dir, "input.json")
	if err := storage.WriteNew(input, c); err != nil {
		t.Fatal(err)
	}
	for _, answer := range []string{"", "q\n", "1,2\n"} {
		out := filepath.Join(dir, "cancelled.json")
		if err := challenge([]string{"-input", input, "-out", out}, strings.NewReader(answer), &bytes.Buffer{}); err == nil {
			t.Fatal("incomplete review saved")
		}
		if _, err := os.Stat(out); !os.IsNotExist(err) {
			t.Fatal("partial review file created")
		}
	}
	for _, choice := range []string{"0", "3", "1,1", "", "all"} {
		if _, err := selectionDecisions(c, choice); err == nil {
			t.Fatalf("invalid choice accepted: %s", choice)
		}
	}
	c.Relations[0].SubjectEvidenceID = "li-search_title"
	c.Relations[0].SubjectQuote = c.Evidence[2].Value
	if _, err := evidence.Normalize(c); err == nil {
		t.Fatal("cross-candidate subject binding accepted")
	}
}

func TestIdentityReviewRequiresConsistentExplicitDecisions(t *testing.T) {
	at := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	for _, selection := range []string{"none", "unsure", "1", "1,2"} {
		t.Run(selection, func(t *testing.T) {
			c := challengeFixture(t)
			decisions, err := selectionDecisions(c, selection)
			if err != nil {
				t.Fatal(err)
			}
			reviewed, err := identity.Review(c, decisions, "Operator checked the public profile URLs", at)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(c.Evidence, reviewed.Evidence) || !reflect.DeepEqual(c.Relations, reviewed.Relations) || c.Candidates[0].Decision != "PENDING" {
				t.Fatal("review changed evidence, claims, or input")
			}
			want := map[string][]string{"none": {"REJECT", "REJECT"}, "unsure": {"UNSURE", "UNSURE"}, "1": {"CONFIRM", "UNSURE"}, "1,2": {"CONFIRM", "CONFIRM"}}[selection]
			for i, candidate := range reviewed.Candidates {
				if candidate.Decision != want[i] {
					t.Fatalf("unexpected decision: %s", candidate.Decision)
				}
			}
			reviewed.Candidates[0].Decision = "PENDING"
			if _, err := evidence.Normalize(reviewed); err == nil {
				t.Fatal("decision inconsistent with audit accepted")
			}
		})
	}
	c := challengeFixture(t)
	decisions, _ := selectionDecisions(c, "1,2")
	c.Candidates[0].Contradictions = []string{"Conflicting supplied account"}
	if _, err := identity.Review(c, decisions, "Attempt to confirm conflicting clues", at); err == nil {
		t.Fatal("contradicting candidate confirmed")
	}
	if c.Candidates[0].Decision != "PENDING" || len(c.IdentityReviews) != 0 {
		t.Fatal("failed review mutated its input")
	}
	c = challengeFixture(t)
	for i := range c.Candidates {
		c.Candidates[i].Decision = "CONFIRM"
	}
	if _, err := evidence.Normalize(c); err == nil {
		t.Fatal("multiple profiles confirmed without explicit review")
	}
}
