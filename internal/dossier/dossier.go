package dossier

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/evidence"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/identity"
)

type Profile struct {
	ID       string `json:"id"`
	Name     string `json:"name,omitempty"`
	URL      string `json:"url"`
	Decision string `json:"decision"`
}

type ObservedName struct {
	Value    string            `json:"value"`
	Evidence []domain.Evidence `json:"evidence"`
}

type Support struct {
	CandidateID       string          `json:"candidate_id"`
	Subject           string          `json:"subject"`
	RelationID        string          `json:"relation_id"`
	Method            string          `json:"method"`
	Review            string          `json:"review"`
	Quote             string          `json:"quote"`
	EvidenceID        string          `json:"evidence_id"`
	Sources           []domain.Source `json:"sources"`
	SubjectQuote      string          `json:"subject_quote,omitempty"`
	SubjectEvidenceID string          `json:"subject_evidence_id,omitempty"`
	SubjectSources    []domain.Source `json:"subject_sources,omitempty"`
}

type Claim struct {
	Kind         string    `json:"kind"`
	Role         string    `json:"role,omitempty"`
	Organization string    `json:"organization,omitempty"`
	Temporal     string    `json:"temporal"`
	StartYear    int       `json:"start_year,omitempty"`
	EndYear      int       `json:"end_year,omitempty"`
	Status       string    `json:"status"`
	Support      []Support `json:"support"`
}

type Issue struct {
	Code        string   `json:"code"`
	Detail      string   `json:"detail"`
	RelationIDs []string `json:"relation_ids,omitempty"`
}

type Report struct {
	PublicNames         []identity.NameCard     `json:"public_names,omitempty"`
	Version             string                  `json:"version"`
	InputSHA256         string                  `json:"input_sha256"`
	SearchName          string                  `json:"search_name"`
	Status              string                  `json:"status"`
	Profiles            []Profile               `json:"profiles"`
	ExcludedProfiles    []Profile               `json:"excluded_profiles"`
	ObservedNames       []ObservedName          `json:"observed_names"`
	ProfileFields       []domain.Evidence       `json:"profile_fields"`
	Employment          []Claim                 `json:"employment"`
	Education           []Claim                 `json:"education"`
	Occupations         []Claim                 `json:"occupations"`
	Other               []Claim                 `json:"other"`
	PendingRelations    []Claim                 `json:"pending_relations"`
	RejectedRelationIDs []string                `json:"rejected_relation_ids"`
	Issues              []Issue                 `json:"issues"`
	IdentityReviews     []domain.IdentityReview `json:"identity_reviews,omitempty"`
	Limitations         []string                `json:"limitations"`
}

func normalized(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }

// Build combines source claims only after explicit profile confirmation.
func Build(input domain.Case) (Report, error) {
	raw, err := json.Marshal(input)
	if err != nil {
		return Report{}, err
	}
	var c domain.Case
	if err := json.Unmarshal(raw, &c); err != nil {
		return Report{}, err
	}
	c, err = evidence.Normalize(c)
	if err != nil {
		return Report{}, err
	}
	out := Report{Version: "1", InputSHA256: fmt.Sprintf("%x", sha256.Sum256(raw)), SearchName: c.Target.Name, Status: "IDENTITY_REVIEW_REQUIRED", Profiles: []Profile{}, ExcludedProfiles: []Profile{}, ObservedNames: []ObservedName{}, Employment: []Claim{}, Education: []Claim{}, Occupations: []Claim{}, Other: []Claim{}, PendingRelations: []Claim{}, RejectedRelationIDs: []string{}, Issues: []Issue{}, IdentityReviews: c.IdentityReviews, Limitations: []string{
		"Profiles were associated by operator decisions; this does not independently verify account ownership or claims.",
		"Search name is user input, not an observed full name. Observed names retain their source evidence.",
		"Claims remain unverified and may be outdated. Multiple profiles do not necessarily provide independent corroboration.",
		"Historical and undated claims retain their periods. Multiple employers may reflect different periods or concurrent work.",
		"This report does not calculate PES or change identity, evidence, or relationship decisions.",
	}}
	selected := map[string]bool{}
	for _, p := range c.Candidates {
		profile := Profile{p.ID, p.DisplayName, p.ProfileURL, p.Decision}
		if p.Decision != "CONFIRM" {
			out.ExcludedProfiles = append(out.ExcludedProfiles, profile)
			continue
		}
		if len(p.Contradictions) > 0 {
			return Report{}, fmt.Errorf("confirmed profile %s has unresolved identity contradictions", p.ID)
		}
		selected[p.ID] = true
		out.Profiles = append(out.Profiles, profile)
		out.PublicNames = append(out.PublicNames, identity.NameCards(c, p.ID)...)
	}
	if len(selected) == 0 {
		return out, nil
	}
	out.Status = "SOURCE_REVIEW_REQUIRED"
	publicNames := map[string]map[string]bool{}
	for _, claim := range out.PublicNames {
		key := normalized(claim.Alias)
		if publicNames[key] == nil {
			publicNames[key] = map[string]bool{}
		}
		publicNames[key][normalized(claim.Name)] = true
	}
	for _, names := range publicNames {
		if len(names) > 1 {
			out.Issues = append(out.Issues, Issue{Code: "PUBLIC_NAME_VARIANTS", Detail: "Sources associate an alias with different public names. Review possible namesakes and name variants; no single real or legal name has been selected."})
			break
		}
	}
	byID := map[string]domain.Evidence{}
	nameIndex := map[string]int{}
	hasWorkplace := false
	for _, e := range c.Evidence {
		byID[e.ID] = e
		if selected[e.CandidateID] && e.Status == "OBSERVED" {
			switch e.Type {
			case "workplace", "role", "location", "website":
				out.ProfileFields = append(out.ProfileFields, e)
				hasWorkplace = hasWorkplace || e.Type == "workplace"
			}
		}
		if !selected[e.CandidateID] || e.Status != "OBSERVED" || e.Type != "name" && e.Type != "profile_display_name" {
			continue
		}
		key := normalized(e.Value)
		if i, ok := nameIndex[key]; ok {
			out.ObservedNames[i].Evidence = append(out.ObservedNames[i].Evidence, e)
		} else {
			nameIndex[key] = len(out.ObservedNames)
			out.ObservedNames = append(out.ObservedNames, ObservedName{e.Value, []domain.Evidence{e}})
		}
	}
	groups := []Claim{}
	index := map[string]int{}
	for _, r := range c.Relations {
		if !selected[r.CandidateID] {
			continue
		}
		decision := domain.RelationDecision(r)
		if decision == "REJECT" {
			out.RejectedRelationIDs = append(out.RejectedRelationIDs, r.ID)
			continue
		}
		status := "UNVERIFIED_SOURCE_CLAIM"
		if strings.HasPrefix(r.Method, "ollama/") && decision != "ACCEPT" {
			status = "PENDING_RELATION_REVIEW"
		}
		claim := Claim{Kind: r.Kind, Role: r.Role, Organization: r.Organization, Temporal: r.Temporal, StartYear: r.StartYear, EndYear: r.EndYear, Status: status}
		keyData, _ := json.Marshal(claim)
		key := normalized(string(keyData))
		support := Support{CandidateID: r.CandidateID, Subject: r.Subject, RelationID: r.ID, Method: r.Method, Review: decision, Quote: r.Quote, EvidenceID: r.EvidenceID, Sources: byID[r.EvidenceID].Sources, SubjectQuote: r.SubjectQuote, SubjectEvidenceID: r.SubjectEvidenceID, SubjectSources: byID[r.SubjectEvidenceID].Sources}
		if i, ok := index[key]; ok {
			groups[i].Support = append(groups[i].Support, support)
		} else {
			index[key] = len(groups)
			claim.Support = []Support{support}
			groups = append(groups, claim)
		}
	}
	for _, claim := range groups {
		if claim.Status == "PENDING_RELATION_REVIEW" {
			out.PendingRelations = append(out.PendingRelations, claim)
			continue
		}
		switch claim.Kind {
		case "employment":
			out.Employment = append(out.Employment, claim)
		case "education":
			out.Education = append(out.Education, claim)
		case "occupation":
			out.Occupations = append(out.Occupations, claim)
		default:
			out.Other = append(out.Other, claim)
		}
	}
	if len(out.ObservedNames) == 0 && len(out.PublicNames) == 0 {
		out.Issues = append(out.Issues, Issue{Code: "NO_OBSERVED_NAME", Detail: "No sourced name field was collected for the confirmed profiles."})
	}
	if len(out.ObservedNames) > 1 {
		out.Issues = append(out.Issues, Issue{Code: "NAME_VARIANTS", Detail: "Confirmed profiles use different observed names; no full or legal name has been selected automatically."})
	}
	if len(out.Employment) == 0 && !hasWorkplace {
		out.Issues = append(out.Issues, Issue{Code: "NO_EMPLOYMENT_CLAIM", Detail: "Collected sources have not established an eligible employer claim; this does not imply unemployment."})
	}
	if len(out.Education) == 0 {
		out.Issues = append(out.Issues, Issue{Code: "NO_EDUCATION_CLAIM", Detail: "Collected sources have not established an eligible education claim."})
	}
	if len(out.PendingRelations) > 0 {
		out.Issues = append(out.Issues, Issue{Code: "RELATION_REVIEW", Detail: "Model proposals remain separate until their quoted relationships are reviewed."})
	}
	out.Issues = append(out.Issues, employmentIssues(out.Employment)...)
	sort.Strings(out.RejectedRelationIDs)
	return out, nil
}

func employmentIssues(claims []Claim) []Issue {
	current, past := map[string][]string{}, map[string][]string{}
	for _, c := range claims {
		key := normalized(c.Organization)
		for _, support := range c.Support {
			if c.Temporal == "headline" || c.Temporal == "present_assertion" {
				current[key] = append(current[key], support.RelationID)
			}
			if c.Temporal == "historical" {
				past[key] = append(past[key], support.RelationID)
			}
		}
	}
	issues := []Issue{}
	if len(current) > 1 {
		ids := []string{}
		for _, refs := range current {
			ids = append(ids, refs...)
		}
		sort.Strings(ids)
		issues = append(issues, Issue{Code: "MULTIPLE_EMPLOYERS", Detail: "Sources mention multiple employers in present-tense or undated headlines. A single current employer cannot be selected from these claims.", RelationIDs: ids})
	}
	periodIDs := []string{}
	for org, ids := range current {
		if len(past[org]) > 0 {
			periodIDs = append(periodIDs, ids...)
			periodIDs = append(periodIDs, past[org]...)
		}
	}
	if len(periodIDs) > 0 {
		sort.Strings(periodIDs)
		issues = append(issues, Issue{Code: "EMPLOYMENT_PERIOD_REVIEW", Detail: "The same employer appears in historical and present-tense or undated claims; inspect dates and possible re-employment.", RelationIDs: periodIDs})
	}
	return issues
}
