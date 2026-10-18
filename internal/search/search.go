package search

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/collector"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/evidence"
)

type Config struct {
	Version               string  `json:"version"`
	CandidateLimit        int     `json:"candidate_limit"`
	RequestBudget         int     `json:"request_budget"`
	MaxAttempts           int     `json:"max_attempts"`
	TimeoutSeconds        int     `json:"timeout_seconds"`
	SessionTimeoutSeconds int     `json:"session_timeout_seconds"`
	NameConfidence        float64 `json:"name_confidence"`
	UsernameConfidence    float64 `json:"username_confidence"`
	AttributeBoost        float64 `json:"attribute_boost"`
	EvidenceConfidence    float64 `json:"evidence_confidence"`
	Discoverability       float64 `json:"discoverability"`
}

func (c Config) Validate() error {
	if c.Version == "" || c.CandidateLimit < 1 || c.CandidateLimit > 20 || c.RequestBudget < 1 || c.RequestBudget > 100 || c.MaxAttempts < 1 || c.MaxAttempts > 10 || c.TimeoutSeconds < 1 || c.TimeoutSeconds > 120 || c.SessionTimeoutSeconds < 1 || c.SessionTimeoutSeconds > 600 {
		return fmt.Errorf("invalid collector limits")
	}
	for _, v := range []float64{c.NameConfidence, c.UsernameConfidence, c.AttributeBoost, c.EvidenceConfidence, c.Discoverability} {
		if !evidence.Unit(v) {
			return fmt.Errorf("invalid collector confidence annotation")
		}
	}
	return nil
}

type Input struct{ Name, Username, Company, Location, Provider, Platform, URL string }

func norm(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }

func Run(ctx context.Context, c collector.Collector, in Input, cfg Config, previous *domain.Case) (domain.Case, error) {
	if err := cfg.Validate(); err != nil {
		return domain.Case{}, err
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Username = strings.TrimSpace(in.Username)
	if len(in.Name) > 150 || len(in.Company) > 150 || len(in.Location) > 150 || len(in.Username) > 250 || len(in.URL) > 4096 {
		return domain.Case{}, fmt.Errorf("search clues must be at most 150 bytes")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	configJSON, _ := json.Marshal(cfg)
	s := domain.SearchSession{StartedAt: now, Name: in.Name, Username: in.Username, Company: in.Company, Location: in.Location, Scope: "GitHub public profile scope only. Coverage is request completion, not coverage of the person's complete public footprint.", ConfigJSON: string(configJSON), Events: []domain.SearchEvent{}}
	s.Provider = in.Provider
	if s.Provider == "" {
		s.Provider = "github"
	}
	s.Platform = in.Platform
	s.URL = in.URL
	if previous != nil {
		if _, err := evidence.Normalize(*previous); err != nil {
			return domain.Case{}, fmt.Errorf("invalid saved session: %w", err)
		}
		if previous.Search == nil {
			return domain.Case{}, fmt.Errorf("input is not a saved search session")
		}
		s = *previous.Search
		if s.Provider == "" {
			s.Provider = "github"
		}
		if in.Provider != "" && in.Provider != s.Provider {
			return domain.Case{}, fmt.Errorf("start a new session to change provider")
		}
		if in.Platform != "" && in.Platform != s.Platform {
			return domain.Case{}, fmt.Errorf("start a new session to change platform")
		}
		if in.URL != "" && in.URL != s.URL {
			return domain.Case{}, fmt.Errorf("start a new session to change URL")
		}
		s.Events = append([]domain.SearchEvent(nil), s.Events...)
		if s.ConfigJSON != string(configJSON) {
			return domain.Case{}, fmt.Errorf("resume requires the original collector configuration")
		}
		if in.Name != "" && norm(in.Name) != norm(s.Name) {
			return domain.Case{}, fmt.Errorf("start a new session to change the subject name")
		}
		if in.Username != "" {
			s.Username = in.Username
		}
		if in.Company != "" {
			s.Company = in.Company
		}
		if in.Location != "" {
			s.Location = in.Location
		}
	} else {
		var id [8]byte
		if _, err := rand.Read(id[:]); err != nil {
			return domain.Case{}, err
		}
		s.ID = "search-" + hex.EncodeToString(id[:])
	}
	if s.Provider != "github" {
		s.Scope = "Public " + s.Provider + " observations only. Indexed snippets and page metadata are not verified personal facts. Coverage is request completion, not completeness across the internet."
	}
	if s.Name == "" && s.Username == "" && s.URL == "" {
		return domain.Case{}, fmt.Errorf("provide -name or -username")
	}
	if s.Provider == "github" && s.Username != "" && !collector.ValidLogin(s.Username) {
		return domain.Case{}, fmt.Errorf("invalid GitHub username")
	}
	if s.Attempts < 0 || s.RequestsUsed < 0 || s.Attempts >= cfg.MaxAttempts || s.RequestsUsed >= cfg.RequestBudget {
		return domain.Case{}, fmt.Errorf("session attempt or request budget exhausted")
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(cfg.SessionTimeoutSeconds)*time.Second)
	defer cancel()
	r := c.Collect(ctx, collector.Query{Name: s.Name, Username: s.Username, Company: s.Company, Location: s.Location, URL: s.URL, Platform: s.Platform, Limit: cfg.CandidateLimit, Budget: cfg.RequestBudget - s.RequestsUsed})
	now = time.Now().UTC().Format(time.RFC3339)
	s.Attempts++
	s.RequestsUsed += r.Requests
	s.UpdatedAt = now
	s.Status = r.Status
	if len(r.Profiles) > 0 {
		s.Status = "REVIEW_REQUIRED"
	}
	if r.Incomplete && (len(r.Profiles) > 0 || r.Status == "SUCCESS" || r.Status == "EMPTY") {
		s.Status = "PARTIAL"
	}
	for _, event := range r.Events {
		s.Events = append(s.Events, domain.SearchEvent{At: now, Operation: event.Operation, Status: event.Status, Detail: event.Detail})
	}
	name := s.Name
	if name == "" {
		name = s.Username
	}
	if name == "" {
		name = s.URL
	}
	project := domain.Case{SchemaVersion: "1", Target: domain.Target{ID: s.ID, Name: name}, Search: &s, Candidates: []domain.Candidate{}, Evidence: []domain.Evidence{}, Coverage: []domain.Coverage{}}
	for _, ev := range r.Events {
		project.Coverage = append(project.Coverage, domain.Coverage{Source: ev.Operation, Status: ev.Status})
	}
	if r.Incomplete {
		project.Coverage = append(project.Coverage, domain.Coverage{Source: s.Provider + ":remaining-results", Status: "NOT_CHECKED"})
	}
	for _, p := range r.Profiles {
		id := fmt.Sprintf("github-%d", p.ID)
		profileURL := "https://github.com/" + p.Login
		sourceURL := "https://api.github.com/users/" + p.Login
		collectorName := "github-public-profile"
		if p.Key != "" {
			id = p.Key
			profileURL = p.URL
			sourceURL = p.SourceURL
			collectorName = s.Provider
		}
		candidate := domain.Candidate{ID: id, RequiresReview: true, Username: p.Login, DisplayName: p.Name, Company: p.Company, Location: p.Location, ProfileURL: "https://github.com/" + p.Login, Decision: "PENDING", Matches: []string{}, Contradictions: []string{}, Missing: []string{}}
		candidate.ProfileURL = profileURL
		for _, f := range p.Facts {
			if f.Type == "search_title" || f.Type == "page_title" {
				candidate.Label = f.Value
				break
			}
		}
		if s.Name != "" && norm(s.Name) == norm(p.Name) {
			candidate.Confidence = cfg.NameConfidence
			candidate.Matches = append(candidate.Matches, "Exact normalized public display name")
		} else if s.Name != "" {
			candidate.Missing = append(candidate.Missing, "Exact display name match")
		}
		if s.Username != "" && strings.EqualFold(s.Username, p.Login) {
			candidate.Confidence = cfg.UsernameConfidence
			candidate.Matches = append(candidate.Matches, "Exact supplied username")
		}
		for _, a := range []struct{ kind, want, got string }{{"company", s.Company, p.Company}, {"location", s.Location, p.Location}} {
			if a.want == "" {
				candidate.Missing = append(candidate.Missing, "No operator "+a.kind+" clue")
				continue
			}
			if strings.TrimSpace(a.got) == "" {
				candidate.Missing = append(candidate.Missing, "Profile has no public "+a.kind)
				continue
			}
			if norm(a.want) == norm(a.got) {
				candidate.Confidence += cfg.AttributeBoost
				candidate.Matches = append(candidate.Matches, "Exact normalized "+a.kind)
			} else {
				candidate.Contradictions = append(candidate.Contradictions, "Supplied "+a.kind+" differs from public profile")
			}
		}
		if candidate.Confidence > 1 {
			candidate.Confidence = 1
		}
		project.Candidates = append(project.Candidates, candidate)
		facts := []collector.Fact{{Type: "username", Value: p.Login}, {Type: "name", Value: p.Name}, {Type: "workplace", Value: p.Company}, {Type: "location", Value: p.Location}, {Type: "email", Value: p.Email}, {Type: "website", Value: p.Blog}}
		facts = append(facts, p.Facts...)
		for _, fact := range facts {
			if strings.TrimSpace(fact.Value) == "" {
				continue
			}
			project.Evidence = append(project.Evidence, domain.Evidence{ID: id + "-" + fact.Type, CandidateID: id, Type: fact.Type, Value: fact.Value, Status: "OBSERVED", Confidence: cfg.EvidenceConfidence, Discoverability: cfg.Discoverability, Sources: []domain.Source{{URL: sourceURL, ObservedAt: now, Collector: collectorName + "/" + cfg.Version}}})
		}
	}
	return evidence.Normalize(project)
}
