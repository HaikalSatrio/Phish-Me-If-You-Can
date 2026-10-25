package identity

import (
	"reflect"
	"testing"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
)

func TestFollowUpQuestionsTrackUnresolvedEvidence(t *testing.T) {
	if got := NextQuestions(domain.Case{}); len(got) != 1 || got[0].Code != "PUBLIC_PROFILE_CLUE" {
		t.Fatalf("missing collection follow-up: %+v", got)
	}
	c := domain.Case{Candidates: []domain.Candidate{{ID: "a", Decision: "PENDING", CompanyStatus: "no_employment_claim"}}, Relations: []domain.Relation{{ID: "r", CandidateID: "a", Method: "ollama/relations-v6"}}}
	before := c.Candidates[0]
	got := NextQuestions(c)
	if len(got) != 2 || got[0].Code != "IDENTITY_REVIEW" || got[1].Code != "RELATION_REVIEW" || !reflect.DeepEqual(before, c.Candidates[0]) {
		t.Fatalf("review question missing or decision mutated: %+v", got)
	}
	c.Candidates[0].Decision = "CONFIRM"
	c.Relations[0].Reviews = []domain.RelationReview{{Decision: "ACCEPT"}}
	got = NextQuestions(c)
	if len(got) != 1 || got[0].Code != "EMPLOYER_SOURCE" {
		t.Fatalf("resolved questions repeated: %+v", got)
	}
	c.Candidates[0].CompanyStatus = "historical_only"
	if got = NextQuestions(c); len(got) != 1 || got[0].Code != "EMPLOYMENT_PERIOD" {
		t.Fatalf("history treated as missing data: %+v", got)
	}
	c.Candidates[0].Decision = "REJECT"
	c.Relations[0].Reviews = nil
	if got = NextQuestions(c); len(got) != 1 || got[0].Code != "NEW_IDENTITY_CLUE" {
		t.Fatalf("rejected identities need a new clue, not relation review: %+v", got)
	}
}

func TestResearchWithoutUsefulClaimsAsksForBiographyEvidence(t *testing.T) {
	c := domain.Case{Search: &domain.SearchSession{Research: []domain.ResearchStep{{Action: "search_handle", Status: "EMPTY"}}}, Candidates: []domain.Candidate{{ID: "a", Decision: "PENDING"}}}
	got := NextQuestions(c)
	if len(got) != 2 || got[0].Code != "PUBLIC_BIO_SOURCE" || c.Candidates[0].Decision != "PENDING" {
		t.Fatalf("missing evidence gap question: %+v", got)
	}
	c.Candidates[0].EducationClaims = []domain.SummaryClaim{{Value: "Example University"}}
	if got = NextQuestions(c); got[0].Code == "PUBLIC_BIO_SOURCE" {
		t.Fatal("education claim ignored")
	}
}
