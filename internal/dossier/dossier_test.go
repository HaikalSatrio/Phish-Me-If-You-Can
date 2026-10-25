package dossier

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/enrich"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/extract"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/identity"
)

func fixture(t *testing.T, extra string) domain.Case {
	t.Helper()
	c := domain.Case{SchemaVersion: "1", Target: domain.Target{ID: "demo", Name: "Nila Utami Pradana"}, Search: &domain.SearchSession{Provider: "web", Name: "Nila Utami Pradana"}}
	for _, id := range []string{"a", "b", "other"} {
		raw := "https://profiles.example.org/" + id
		c.Candidates = append(c.Candidates, domain.Candidate{ID: id, DisplayName: "Nila Utami", ProfileURL: raw, Decision: "PENDING", RequiresReview: true})
		text := "Nila Utami works as Engineer at Willow Labs."
		if id == "b" {
			text += " " + extra
		}
		if id == "other" {
			text = "Nila Utami works as Director at Other Corporation."
		}
		for typ, value := range map[string]string{"name": "Nila Utami", "page_text": text} {
			c.Evidence = append(c.Evidence, domain.Evidence{ID: id + "-" + typ, CandidateID: id, Type: typ, Value: value, Status: "OBSERVED", Confidence: 1, Sources: []domain.Source{{URL: raw, Collector: "fixture", ObservedAt: "2026-10-25T00:00:00Z"}}})
		}
	}
	c, err := enrich.Run(context.Background(), c, extract.Config{})
	if err != nil {
		t.Fatal(err)
	}
	c, err = identity.Review(c, []domain.CandidateDecision{{CandidateID: "a", Decision: "CONFIRM"}, {CandidateID: "b", Decision: "CONFIRM"}, {CandidateID: "other", Decision: "UNSURE"}}, "Fictional fixture associates two profiles", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestOnlyConfirmedProfilesContributeAndEvidenceIsPreserved(t *testing.T) {
	c := fixture(t, "")
	before, _ := json.Marshal(c)
	r, err := Build(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Profiles) != 2 || len(r.ExcludedProfiles) != 1 || len(r.Employment) != 1 || len(r.Employment[0].Support) != 2 || r.Employment[0].Organization != "Willow Labs" {
		t.Fatalf("incorrect combination: %+v", r)
	}
	if len(r.ObservedNames) != 1 || r.ObservedNames[0].Value != "Nila Utami" || len(r.ObservedNames[0].Evidence) != 2 {
		t.Fatal("search input substituted for observed name or name provenance lost")
	}
	for _, s := range r.Employment[0].Support {
		if s.Quote == "" || len(s.Sources) == 0 || s.CandidateID == "other" {
			t.Fatal("source ownership lost")
		}
	}
	after, _ := json.Marshal(c)
	if string(before) != string(after) {
		t.Fatal("report mutated source case")
	}
}

func TestModelReviewStatesRemainSeparate(t *testing.T) {
	c := fixture(t, "")
	for i := range c.Relations {
		if c.Relations[i].CandidateID == "b" {
			c.Relations[i].Method = "ollama/" + extract.RelationVersion
		}
	}
	r, err := Build(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.PendingRelations) != 1 || len(r.Employment) != 1 || len(r.Employment[0].Support) != 1 {
		t.Fatal("pending model claim promoted")
	}
	for i := range c.Relations {
		if c.Relations[i].CandidateID == "b" {
			c.Relations[i].Reviews = []domain.RelationReview{{Decision: "ACCEPT", At: time.Now().UTC().Format(time.RFC3339), Reason: "Fixture checks literal source relationship"}}
		}
	}
	r, err = Build(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.PendingRelations) != 0 || len(r.Employment[0].Support) != 2 || r.Employment[0].Status != "UNVERIFIED_SOURCE_CLAIM" {
		t.Fatal("accepted relation lost or verified automatically")
	}
	for i := range c.Relations {
		if c.Relations[i].CandidateID == "b" {
			c.Relations[i].Reviews[0].Decision = "REJECT"
		}
	}
	r, err = Build(c)
	if err != nil || len(r.RejectedRelationIDs) != 1 || len(r.Employment[0].Support) != 1 {
		t.Fatalf("rejected relation retained: %v", err)
	}
}

func TestEmployerAlternativesAndPeriodsRemainVisible(t *testing.T) {
	c := fixture(t, "Nila Utami works as Researcher at Cedar Labs. Nila Utami worked as Engineer at Willow Labs from 2018 to 2021.")
	r, err := Build(c)
	if err != nil {
		t.Fatal(err)
	}
	issues := map[string]bool{}
	for _, issue := range r.Issues {
		issues[issue.Code] = true
	}
	if len(r.Employment) != 3 || !issues["MULTIPLE_EMPLOYERS"] || !issues["EMPLOYMENT_PERIOD_REVIEW"] {
		t.Fatalf("conflicting periods or employer alternatives lost: %+v", r)
	}
}

func TestNoConfirmationAndStructuredFields(t *testing.T) {
	c := fixture(t, "")
	decisions := []domain.CandidateDecision{}
	for _, p := range c.Candidates {
		decisions = append(decisions, domain.CandidateDecision{CandidateID: p.ID, Decision: "UNSURE"})
	}
	unconfirmed, err := identity.Review(c, decisions, "Fixture retracts association", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	r, err := Build(unconfirmed)
	if err != nil || r.Status != "IDENTITY_REVIEW_REQUIRED" || len(r.Employment) != 0 || len(r.ObservedNames) != 0 {
		t.Fatal("unconfirmed profiles contributed claims")
	}
	c.Evidence = append(c.Evidence, domain.Evidence{ID: "structured-workplace", CandidateID: "a", Type: "workplace", Value: "Cedar Labs", Status: "OBSERVED", Sources: []domain.Source{{URL: "https://profiles.example.org/a", Collector: "fixture", ObservedAt: "2026-10-25T00:00:00Z"}}})
	r, err = Build(c)
	if err != nil || len(r.ProfileFields) != 1 {
		t.Fatalf("structured profile field discarded: %v", err)
	}
}
