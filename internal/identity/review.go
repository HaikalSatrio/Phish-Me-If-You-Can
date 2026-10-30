package identity

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/evidence"
)

type ClaimCard struct {
	RelationID        string          `json:"relation_id"`
	Kind              string          `json:"kind"`
	Role              string          `json:"role,omitempty"`
	Organization      string          `json:"organization,omitempty"`
	Temporal          string          `json:"temporal"`
	Review            string          `json:"review"`
	Quote             string          `json:"quote"`
	EvidenceID        string          `json:"evidence_id"`
	Sources           []domain.Source `json:"sources"`
	SubjectQuote      string          `json:"subject_quote,omitempty"`
	SubjectEvidenceID string          `json:"subject_evidence_id,omitempty"`
	SubjectSources    []domain.Source `json:"subject_sources,omitempty"`
}

type ReviewCard struct {
	SocialPosts    []domain.SocialPost `json:"social_posts,omitempty"`
	Names          []NameCard          `json:"names,omitempty"`
	CandidateID    string              `json:"candidate_id"`
	Name           string              `json:"name,omitempty"`
	Title          string              `json:"title,omitempty"`
	URL            string              `json:"url"`
	Decision       string              `json:"decision"`
	Claims         []ClaimCard         `json:"claims"`
	Contradictions []string            `json:"contradictions,omitempty"`
}

type NameCard struct {
	domain.NameClaim
	Sources []domain.Source `json:"sources"`
}

func NameCards(c domain.Case, candidateID string) []NameCard {
	out := []NameCard{}
	for _, claim := range c.NameClaims {
		if claim.CandidateID != candidateID {
			continue
		}
		for _, e := range c.Evidence {
			if e.ID == claim.EvidenceID {
				out = append(out, NameCard{claim, e.Sources})
			}
		}
	}
	return out
}

func Cards(c domain.Case) []ReviewCard {
	evidence := map[string]domain.Evidence{}
	for _, e := range c.Evidence {
		evidence[e.ID] = e
	}
	cards := []ReviewCard{}
	for _, p := range c.Candidates {
		card := ReviewCard{CandidateID: p.ID, Name: p.DisplayName, Title: p.Label, URL: p.ProfileURL, Decision: p.Decision, Claims: []ClaimCard{}, Contradictions: append([]string(nil), p.Contradictions...)}
		card.Names = NameCards(c, p.ID)
		for _, post := range c.SocialPosts {
			if post.CandidateID == p.ID {
				card.SocialPosts = append(card.SocialPosts, post)
			}
		}
		for _, r := range c.Relations {
			if r.CandidateID != p.ID || domain.RelationDecision(r) == "REJECT" {
				continue
			}
			card.Claims = append(card.Claims, ClaimCard{RelationID: r.ID, Kind: r.Kind, Role: r.Role, Organization: r.Organization, Temporal: r.Temporal, Review: domain.RelationDecision(r), Quote: r.Quote, EvidenceID: r.EvidenceID, Sources: evidence[r.EvidenceID].Sources, SubjectQuote: r.SubjectQuote, SubjectEvidenceID: r.SubjectEvidenceID, SubjectSources: evidence[r.SubjectEvidenceID].Sources})
		}
		cards = append(cards, card)
	}
	return cards
}

// Review applies only explicit operator decisions and retains all source evidence.
func Review(input domain.Case, decisions []domain.CandidateDecision, reason string, at time.Time) (domain.Case, error) {
	if len(decisions) != len(input.Candidates) || len(decisions) == 0 || strings.TrimSpace(reason) == "" {
		return input, fmt.Errorf("review requires a decision for every candidate and a reason")
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return input, err
	}
	var c domain.Case
	if err := json.Unmarshal(encoded, &c); err != nil {
		return input, err
	}
	if c, err = evidence.Normalize(c); err != nil {
		return input, err
	}
	choices := map[string]string{}
	for _, decision := range decisions {
		if choices[decision.CandidateID] != "" {
			return input, fmt.Errorf("duplicate review candidate")
		}
		choices[decision.CandidateID] = decision.Decision
	}
	confirmed := 0
	for i := range c.Candidates {
		p := &c.Candidates[i]
		decision, found := choices[p.ID]
		if !found {
			return input, fmt.Errorf("missing decision for %s", p.ID)
		}
		if decision == "CONFIRM" {
			if len(p.Contradictions) > 0 {
				return input, fmt.Errorf("candidate %s has conflicting clues; resolve them before confirming", p.ID)
			}
			confirmed++
		}
		p.Decision = decision
	}
	timestamp := at.UTC().Format(time.RFC3339)
	c.IdentityReviews = append(c.IdentityReviews, domain.IdentityReview{At: timestamp, Reason: strings.TrimSpace(reason), Decisions: append([]domain.CandidateDecision(nil), decisions...)})
	if c.Search != nil {
		c.Search.UpdatedAt = timestamp
		c.Search.Status = "REVIEW_REQUIRED"
		if confirmed > 0 {
			c.Search.Status = "IDENTITY_CONFIRMED"
		}
		c.Search.Events = append(c.Search.Events, domain.SearchEvent{At: timestamp, Operation: "review:identity", Status: c.Search.Status, Detail: fmt.Sprintf("Operator associated %d profiles with the target; source claims remain unverified", confirmed)})
	}
	return evidence.Normalize(c)
}
