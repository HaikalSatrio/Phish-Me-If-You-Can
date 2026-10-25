package identity

import (
	"strings"
	"testing"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
)

func TestDifferentPublicNamesTriggerReviewWithoutIdentityChanges(t *testing.T) {
	c := domain.Case{Target: domain.Target{Name: "Awan"}, Search: &domain.SearchSession{Research: []domain.ResearchStep{{Action: "search_biography"}}}, Candidates: []domain.Candidate{{ID: "a", Decision: "PENDING"}, {ID: "b", Decision: "PENDING"}}, NameClaims: []domain.NameClaim{{CandidateID: "a", Alias: "Awan", Name: "Sari Utami"}, {CandidateID: "b", Alias: "Awan", Name: "Tari Murni"}}}
	questions := NextQuestions(c)
	if len(questions) != 1 || questions[0].Code != "IDENTITY_REVIEW" || !strings.Contains(questions[0].Reason, "namesakes") {
		t.Fatalf("wrong follow-up: %+v", questions)
	}
	if len(questions[0].Candidates[0].Names) != 0 {
		t.Fatal("unbacked evidence shown as a sourced name")
	}
	if c.Candidates[0].Decision != "PENDING" || c.Candidates[1].Decision != "PENDING" {
		t.Fatal("review suggestion changed identities")
	}
	if len(NameAmbiguities(c)) != 1 {
		t.Fatal("ambiguous alias hidden")
	}
	c.Candidates[1].Decision = "REJECT"
	if len(NameAmbiguities(c)) != 0 {
		t.Fatal("rejected identity still contributes name ambiguity")
	}
}
