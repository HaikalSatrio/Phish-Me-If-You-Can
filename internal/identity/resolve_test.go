package identity

import (
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"testing"
)

func TestIdentityStates(t *testing.T) {
	p := domain.Policy{AcceptThreshold: 0.9, ReviewThreshold: 0.6}
	for _, tt := range []struct {
		name      string
		candidate domain.Candidate
		want      string
	}{
		{"name only insufficient confidence", domain.Candidate{Confidence: 0.3, Matches: []string{"same name"}, Decision: "PENDING"}, "ABSTAIN"},
		{"review", domain.Candidate{Confidence: 0.7, Matches: []string{"identifier"}, Decision: "PENDING"}, "REVIEW"},
		{"high confidence", domain.Candidate{Confidence: 0.95, Matches: []string{"identifier"}, Decision: "PENDING"}, "ACCEPTED"},
		{"no support", domain.Candidate{Confidence: 0.99, Decision: "PENDING"}, "ABSTAIN"},
		{"explicit rejection", domain.Candidate{Confidence: 0.99, Decision: "REJECT"}, "REJECTED"},
		{"contradiction overrides confirmation", domain.Candidate{Confidence: 0.99, Decision: "CONFIRM", Contradictions: []string{"different identifier"}}, "REVIEW"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := Resolve(tt.candidate, p); got != tt.want {
				t.Fatalf("got %s, want %s", got, tt.want)
			}
		})
	}
}
