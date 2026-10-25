package dossier

import (
	"context"
	"testing"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/enrich"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/extract"
)

func TestPublicNameClaimsRequireProfileConfirmation(t *testing.T) {
	c := domain.Case{SchemaVersion: "1", Target: domain.Target{ID: "case", Name: "Awan"}}
	for _, entry := range []struct{ id, text, decision string }{
		{"a", "Sari Utami, known as Awan, is a singer.", "CONFIRM"},
		{"b", "Tari Murni, known as Awan, is a writer.", "PENDING"},
	} {
		c.Candidates = append(c.Candidates, domain.Candidate{ID: entry.id, Decision: entry.decision})
		c.Evidence = append(c.Evidence, domain.Evidence{ID: entry.id + "-doc", CandidateID: entry.id, Type: "search_snippet", Value: entry.text, Status: "OBSERVED", Sources: []domain.Source{{URL: "https://example.org/" + entry.id, Collector: "fixture", ObservedAt: "2026-10-25T00:00:00Z"}}})
	}
	c, err := enrich.Run(context.Background(), c, extract.Config{})
	if err != nil {
		t.Fatal(err)
	}
	r, err := Build(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.PublicNames) != 1 || r.PublicNames[0].Name != "Sari Utami" || len(r.PublicNames[0].Sources) != 1 || len(r.ObservedNames) != 0 {
		t.Fatalf("name ownership or provenance lost: %+v", r)
	}
	c.Candidates[1].Decision = "CONFIRM"
	r, err = Build(c)
	if err != nil {
		t.Fatal(err)
	}
	issue := false
	for _, i := range r.Issues {
		if i.Code == "PUBLIC_NAME_VARIANTS" {
			issue = true
		}
	}
	if len(r.PublicNames) != 2 || !issue {
		t.Fatal("different people silently merged into a canonical name")
	}
}
