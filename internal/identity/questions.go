package identity

import (
	"strings"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
)

type Question struct {
	Code         string   `json:"code"`
	Prompt       string   `json:"prompt"`
	Reason       string   `json:"reason"`
	Action       string   `json:"action"`
	CandidateIDs []string `json:"candidate_ids,omitempty"`
	RelationIDs  []string `json:"relation_ids,omitempty"`
}

// NextQuestions suggests missing evidence or review steps without changing decisions.
func NextQuestions(c domain.Case) []Question {
	out := []Question{}
	if len(c.Candidates) == 0 {
		return []Question{{Code: "PUBLIC_PROFILE_CLUE", Prompt: "Which public profile URL or platform:username can identify the intended person?", Reason: "No usable candidate was collected; failed sources do not establish absence.", Action: "search -source web -name <full-name> -account <platform:username> or -url <public-url>"}}
	}
	pending := []string{}
	confirmed := false
	rejected := map[string]bool{}
	for _, p := range c.Candidates {
		confirmed = confirmed || p.Decision == "CONFIRM"
		rejected[p.ID] = p.Decision == "REJECT"
		if p.Decision == "PENDING" || p.Decision == "UNSURE" {
			pending = append(pending, p.ID)
		}
	}
	if !confirmed && len(pending) > 0 {
		useful := false
		for _, p := range c.Candidates {
			if !rejected[p.ID] && (p.DisplayName != "" || p.Company != "" || p.Role != "" || len(p.EducationClaims) > 0) {
				useful = true
			}
		}
		if !useful && c.Search != nil && len(c.Search.Research) > 0 {
			out = append(out, Question{Code: "PUBLIC_BIO_SOURCE", Prompt: "Which public biography, portfolio, or institutional profile links this account to a full name or affiliation?", Reason: "Research collected pages but has not established a public name, education, or employment claim. More pages alone do not resolve identity.", Action: "search -source web -name <name> -account <platform:username> -url <public-biography-url>", CandidateIDs: pending})
		}
		out = append(out, Question{Code: "IDENTITY_REVIEW", Prompt: "Which candidate can you establish as the intended person? Leave uncertain candidates pending.", Reason: "Names and supplied handles do not verify ownership or link accounts across platforms.", Action: "review -input <project> -candidate <id> -decision CONFIRM|REJECT|UNSURE -out <new-project>", CandidateIDs: pending})
	}
	if !confirmed && len(pending) == 0 {
		return []Question{{Code: "NEW_IDENTITY_CLUE", Prompt: "Which additional public account or URL can distinguish the intended person?", Reason: "All collected candidates were rejected.", Action: "search -source web -name <full-name> -account <platform:username> or -url <public-url>"}}
	}
	modelRelations := []string{}
	for _, r := range c.Relations {
		if !rejected[r.CandidateID] && strings.HasPrefix(r.Method, "ollama/") && domain.RelationDecision(r) == "PENDING" {
			modelRelations = append(modelRelations, r.ID)
		}
	}
	if len(modelRelations) > 0 {
		out = append(out, Question{Code: "RELATION_REVIEW", Prompt: "Does each quoted passage actually support the proposed employer, role, and period?", Reason: "Passing literal validation does not establish the meaning or current validity of a model proposal.", Action: "review-relation -input <project> -relation <id> -decision ACCEPT|REJECT|PENDING -reason <reason> -out <new-project>", RelationIDs: modelRelations})
	}
	for _, p := range c.Candidates {
		if p.Decision != "CONFIRM" {
			continue
		}
		switch p.CompanyStatus {
		case "ambiguous", "period_conflict", "historical_only", "non_current_claims":
			out = append(out, Question{Code: "EMPLOYMENT_PERIOD", Prompt: "Which dated public source distinguishes current employment from past or competing claims?", Reason: "The available claims do not establish one current employer.", Action: "search -source url -name <full-name> -url <dated-public-source>", CandidateIDs: []string{p.ID}})
		case "no_employment_claim":
			out = append(out, Question{Code: "EMPLOYER_SOURCE", Prompt: "Do you have a public source that explicitly links this person to an employer?", Reason: "The collected sources have not supplied an eligible employment claim; entering -company alone is not evidence.", Action: "search -source url -name <full-name> -url <public-source>", CandidateIDs: []string{p.ID}})
		}
	}
	return out[:min(len(out), 3)]
}
