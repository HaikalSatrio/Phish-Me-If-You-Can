package enrich

import (
	"context"
	"reflect"
	"testing"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/evidence"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/extract"
)

func TestUndatedCorroborationDoesNotEraseSummary(t *testing.T) {
	c := fixture("Ayu Lestari works as Engineer at Orbit Labs.")
	e := c.Evidence[0]
	e.ID, e.Type, e.Value = "structured", "person_jsonld", `[{"name":"Ayu Lestari","job_titles":["Engineer"],"employers":["Orbit Labs"]}]`
	c.Evidence = append(c.Evidence, e)
	for _, structuredOnly := range []bool{false, true} {
		if structuredOnly {
			c.Evidence = c.Evidence[1:]
		}
		got, err := Run(context.Background(), c, extract.Config{})
		if err != nil {
			t.Fatal(err)
		}
		p := got.Candidates[0]
		if p.Company != "Orbit Labs" || p.Role != "Engineer" {
			t.Fatalf("corroboration erased values: %+v", p)
		}
		if structuredOnly {
			if p.RoleTemporal != "unknown" || p.CompanyTemporal != "unknown" {
				t.Fatalf("undated claim became current: %+v", p)
			}
		} else if p.RoleTemporal != "present_assertion" || len(p.RoleRelationIDs) != 2 || len(p.CompanyRelationIDs) != 2 {
			t.Fatalf("lost corroborating references: %+v", p)
		}
	}
}

func TestRelationshipReviewSurvivesReplayWithoutVerifyingIdentity(t *testing.T) {
	c, err := Run(context.Background(), fixture("Ayu Lestari works as Engineer at Orbit Labs."), extract.Config{})
	if err != nil {
		t.Fatal(err)
	}
	originalID := c.Relations[0].ID
	c.Relations[0].Method = "ollama/" + extract.RelationVersion
	for _, decision := range []string{"ACCEPT", "REJECT", "PENDING"} {
		c.Relations[0].Reviews = append(c.Relations[0].Reviews, domain.RelationReview{Decision: decision, At: "2026-10-24T00:00:00Z", Reason: "Checked source wording"})
		Reproject(&c)
		p := c.Candidates[0]
		if (p.Role == "Engineer") != (decision == "ACCEPT") {
			t.Fatalf("review not applied: %s %+v", decision, p)
		}
		if p.Decision != "PENDING" || p.Confidence != 0 || c.Relations[0].Status != "UNVERIFIED" {
			t.Fatal("review promoted identity or evidence")
		}
		again, err := Run(context.Background(), c, extract.Config{})
		if err != nil {
			t.Fatal(err)
		}
		if len(again.Relations) != 1 || again.Relations[0].ID != originalID || !reflect.DeepEqual(c.Relations[0].Reviews, again.Relations[0].Reviews) || again.Candidates[0].Role != p.Role {
			t.Fatalf("review lost during replay: %+v", again)
		}
		c = again
	}
	for _, bad := range []domain.RelationReview{{Decision: "OBSERVED", At: "2026-10-24T00:00:00Z", Reason: "bad"}, {Decision: "ACCEPT", At: "invalid", Reason: "bad"}, {Decision: "ACCEPT", At: "2026-10-24T00:00:00Z"}, {Decision: "ACCEPT", At: "2020-01-01T00:00:00Z", Reason: "backdated"}} {
		copy := c
		copy.Relations = append([]domain.Relation(nil), c.Relations...)
		copy.Relations[0].Reviews = append(append([]domain.RelationReview(nil), c.Relations[0].Reviews...), bad)
		if _, err := evidence.Normalize(copy); err == nil {
			t.Fatalf("invalid review accepted: %+v", bad)
		}
	}
}

func TestHistoricalClaimsRemainVisible(t *testing.T) {
	c := fixture("Ayu Lestari worked as Engineer at Orbit Labs from 1 January 2018 to 31 December 2022.")
	got, err := Run(context.Background(), c, extract.Config{})
	if err != nil {
		t.Fatal(err)
	}
	p := got.Candidates[0]
	if p.Role != "" || p.Company != "" || len(p.RoleClaims) != 1 || len(p.CompanyClaims) != 1 || p.RoleClaims[0].Temporal != "historical" || p.CompanyClaims[0].Value != "Orbit Labs" {
		t.Fatalf("history hidden or misrepresented as current: %+v", p)
	}
}

func TestSummaryClaimsRequireMatchingSourceRelationships(t *testing.T) {
	c, err := Run(context.Background(), fixture("Ayu Lestari works as Engineer at Orbit Labs."), extract.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*domain.SummaryClaim){
		func(c *domain.SummaryClaim) { c.Value = "Invented Corp" },
		func(c *domain.SummaryClaim) { c.RelationIDs = []string{"missing"} },
		func(c *domain.SummaryClaim) { c.Temporal = "historical" },
	} {
		bad := c
		bad.Candidates = append([]domain.Candidate(nil), c.Candidates...)
		bad.Candidates[0].CompanyClaims = append([]domain.SummaryClaim(nil), c.Candidates[0].CompanyClaims...)
		mutate(&bad.Candidates[0].CompanyClaims[0])
		if _, err := evidence.Normalize(bad); err == nil {
			t.Fatal("forged summary claim accepted")
		}
	}
}
