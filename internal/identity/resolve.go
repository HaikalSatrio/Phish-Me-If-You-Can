package identity

import "github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"

func Resolve(c domain.Candidate, p domain.Policy) string {
	if c.Decision == "REJECT" {
		return "REJECTED"
	}
	if len(c.Contradictions) > 0 {
		return "REVIEW"
	}
	if c.Decision == "CONFIRM" {
		return "ACCEPTED"
	}
	if c.Decision == "UNSURE" {
		return "ABSTAIN"
	}
	if c.RequiresReview {
		return "REVIEW"
	}
	if len(c.Matches) == 0 {
		return "ABSTAIN"
	}
	if c.Confidence >= p.AcceptThreshold {
		return "ACCEPTED"
	}
	if c.Confidence >= p.ReviewThreshold {
		return "REVIEW"
	}
	return "ABSTAIN"
}
