package evidence

import (
	"encoding/hex"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/extract"
)

func Unit(v float64) bool       { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v <= 1 }
func canonical(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }

func Normalize(c domain.Case) (domain.Case, error) {
	if c.SchemaVersion != "1" || strings.TrimSpace(c.Target.ID) == "" || strings.TrimSpace(c.Target.Name) == "" {
		return c, fmt.Errorf("schema_version 1 and target id/name are required")
	}
	receipts := map[string]bool{}
	for _, receipt := range c.Receipts {
		u, err := url.Parse(receipt.URL)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || receipt.Platform == "" || receipt.Rule == "" || receipt.Reason == "" || receipts[receipt.URL] || receipt.Bytes < 0 {
			return c, fmt.Errorf("invalid or duplicate collection receipt")
		}
		receipts[receipt.URL] = true
		if _, err := time.Parse(time.RFC3339, receipt.ObservedAt); err != nil {
			return c, fmt.Errorf("invalid receipt observation time")
		}
		switch receipt.Status {
		case "SUCCESS", "EMPTY", "BLOCKED", "FAILED", "NOT_CHECKED":
		default:
			return c, fmt.Errorf("invalid receipt status")
		}
		if receipt.ResponseSHA256 != "" || receipt.Status == "SUCCESS" {
			decoded, err := hex.DecodeString(receipt.ResponseSHA256)
			if err != nil || len(decoded) != 32 {
				return c, fmt.Errorf("invalid receipt response digest")
			}
		}
	}
	candidates := map[string]bool{}
	confirmed := 0
	for _, v := range c.Candidates {
		if c.Search != nil {
			if !v.RequiresReview {
				return c, fmt.Errorf("search candidate %s must require review", v.ID)
			}
			if v.Decision == "CONFIRM" {
				confirmed++
			}
		}
		if v.ID == "" || candidates[v.ID] || !Unit(v.Confidence) {
			return c, fmt.Errorf("invalid or duplicate candidate %q", v.ID)
		}
		switch v.Decision {
		case "PENDING", "CONFIRM", "REJECT", "UNSURE":
		default:
			return c, fmt.Errorf("invalid decision for %s", v.ID)
		}
		candidates[v.ID] = true
	}
	lastReview := map[string]string{}
	previousReviewTime := time.Time{}
	for _, review := range c.IdentityReviews {
		at, err := time.Parse(time.RFC3339, review.At)
		if err != nil || at.Before(previousReviewTime) || strings.TrimSpace(review.Reason) == "" || len(review.Reason) > 2000 || len(review.Decisions) != len(c.Candidates) {
			return c, fmt.Errorf("invalid identity review")
		}
		lastReview = map[string]string{}
		for _, d := range review.Decisions {
			if !candidates[d.CandidateID] || lastReview[d.CandidateID] != "" {
				return c, fmt.Errorf("invalid identity review candidate")
			}
			switch d.Decision {
			case "CONFIRM", "REJECT", "UNSURE", "PENDING":
			default:
				return c, fmt.Errorf("invalid identity review decision")
			}
			lastReview[d.CandidateID] = d.Decision
		}
		previousReviewTime = at
	}
	if c.Search != nil && confirmed > 1 {
		for _, p := range c.Candidates {
			if lastReview[p.ID] != p.Decision || p.Decision == "CONFIRM" && len(p.Contradictions) > 0 {
				return c, fmt.Errorf("confirming multiple profiles requires an explicit identity review without unresolved contradictions")
			}
		}
	}
	if len(c.IdentityReviews) > 0 {
		for _, p := range c.Candidates {
			if lastReview[p.ID] != p.Decision {
				return c, fmt.Errorf("identity decisions differ from the latest review")
			}
		}
	}
	ids := map[string]bool{}
	for _, e := range c.Evidence {
		if e.ID == "" || ids[e.ID] {
			return c, fmt.Errorf("invalid or duplicate evidence id %q", e.ID)
		}
		ids[e.ID] = true
	}
	for _, e := range c.Evidence {
		for _, alias := range e.Aliases {
			if alias == "" || ids[alias] {
				return c, fmt.Errorf("invalid or duplicate alias %q", alias)
			}
			ids[alias] = true
		}
	}
	owners := map[string]string{}
	claims := map[string]domain.Evidence{}
	for _, e := range c.Evidence {
		owners[e.ID] = e.CandidateID
		claims[e.ID] = e
		for _, alias := range e.Aliases {
			owners[alias] = e.CandidateID
			claims[alias] = e
		}
	}
	for _, e := range c.Evidence {
		if !candidates[e.CandidateID] || canonical(e.Type) == "" || !Unit(e.Confidence) || !Unit(e.Discoverability) {
			return c, fmt.Errorf("invalid evidence %s", e.ID)
		}
		switch e.Status {
		case "OBSERVED", "INFERRED", "UNKNOWN", "REJECTED":
		default:
			return c, fmt.Errorf("invalid evidence status %s", e.ID)
		}
		if e.Status != "UNKNOWN" && strings.TrimSpace(e.Value) == "" {
			return c, fmt.Errorf("missing value for %s", e.ID)
		}
		if e.Status == "OBSERVED" && len(e.Sources) == 0 {
			return c, fmt.Errorf("missing provenance for %s", e.ID)
		}
		if e.Status == "INFERRED" && len(e.SupportingIDs) == 0 {
			return c, fmt.Errorf("missing supporting evidence for %s", e.ID)
		}
		for _, id := range e.SupportingIDs {
			if !ids[id] || claims[id].ID == e.ID || owners[id] != e.CandidateID {
				return c, fmt.Errorf("invalid supporting reference %s in %s", id, e.ID)
			}
		}
		for _, s := range e.Sources {
			if c.Search != nil && c.Search.Provider == "direct" {
				matched := false
				for _, receipt := range c.Receipts {
					if receipt.Status == "SUCCESS" && receipt.URL == s.URL && receipt.Rule == s.Rule && receipt.ResponseSHA256 == s.ResponseSHA256 && s.ResponseSHA256 != "" {
						matched = true
					}
				}
				if !matched {
					return c, fmt.Errorf("direct evidence %s has no matching collection receipt", e.ID)
				}
			}
			if s.ResponseSHA256 != "" {
				decoded, err := hex.DecodeString(s.ResponseSHA256)
				if err != nil || len(decoded) != 32 || s.Rule == "" {
					return c, fmt.Errorf("invalid response provenance for %s", e.ID)
				}
			}
			u, err := url.Parse(s.URL)
			if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || s.Collector == "" {
				return c, fmt.Errorf("invalid source for %s", e.ID)
			}
			if _, err := time.Parse(time.RFC3339, s.ObservedAt); err != nil {
				return c, fmt.Errorf("invalid observation time for %s", e.ID)
			}
		}
	}
	for _, receipt := range c.Receipts {
		if receipt.Status != "SUCCESS" {
			continue
		}
		matched := false
		for _, e := range c.Evidence {
			if e.Status != "OBSERVED" {
				continue
			}
			for _, source := range e.Sources {
				if source.URL == receipt.URL && source.ResponseSHA256 == receipt.ResponseSHA256 && source.Rule == receipt.Rule {
					matched = true
				}
			}
		}
		if !matched {
			return c, fmt.Errorf("successful receipt has no matching observed evidence")
		}
	}
	visited := map[string]int{}
	var visit func(string) error
	visit = func(id string) error {
		e := claims[id]
		id = e.ID
		if visited[id] == 1 {
			return fmt.Errorf("cyclic supporting evidence at %s", id)
		}
		if visited[id] == 2 {
			return nil
		}
		visited[id] = 1
		for _, parent := range e.SupportingIDs {
			if err := visit(parent); err != nil {
				return err
			}
		}
		visited[id] = 2
		return nil
	}
	for _, e := range c.Evidence {
		if err := visit(e.ID); err != nil {
			return c, err
		}
	}
	coverage := map[string]bool{}
	for _, s := range c.Coverage {
		if s.Source == "" || coverage[s.Source] {
			return c, fmt.Errorf("invalid or duplicate coverage source")
		}
		coverage[s.Source] = true
		switch s.Status {
		case "SUCCESS", "EMPTY", "FAILED", "BLOCKED", "NOT_CHECKED":
		default:
			return c, fmt.Errorf("invalid coverage status")
		}
	}
	// Only identical observed claims are merged. Conflicting claims remain visible.
	out := make([]domain.Evidence, 0, len(c.Evidence))
	seen := map[string]int{}
	for _, e := range c.Evidence {
		e.Type = canonical(e.Type)
		if e.Type == "page_text" {
			lines := strings.Split(e.Value, "\n")
			for i := range lines {
				lines[i] = strings.Join(strings.Fields(lines[i]), " ")
			}
			e.Value = strings.TrimSpace(strings.Join(lines, "\n"))
		} else if e.Type != "instagram_post" && e.Type != "instagram_collection" {
			e.Value = strings.Join(strings.Fields(e.Value), " ")
		}
		key := e.CandidateID + "\x00" + e.Type + "\x00" + canonical(e.Value)
		if e.Type == "instagram_post" || e.Type == "instagram_collection" {
			key = e.CandidateID + "\x00" + e.Type + "\x00" + e.Value
		}
		if i, ok := seen[key]; ok && e.Status == "OBSERVED" {
			x := &out[i]
			x.Aliases = append(x.Aliases, e.ID)
			x.Aliases = append(x.Aliases, e.Aliases...)
			for _, s := range e.Sources {
				found := false
				for _, old := range x.Sources {
					if old == s {
						found = true
					}
				}
				if !found {
					x.Sources = append(x.Sources, s)
				}
			}
			if e.Confidence < x.Confidence {
				x.Confidence = e.Confidence
			}
			if e.Discoverability > x.Discoverability {
				x.Discoverability = e.Discoverability
			}
			continue
		}
		if e.Status == "OBSERVED" {
			seen[key] = len(out)
		}
		out = append(out, e)
	}
	// Preserve references to merged facts through their canonical IDs.
	aliases := map[string]string{}
	for _, e := range out {
		for _, a := range e.Aliases {
			aliases[a] = e.ID
		}
	}
	for i := range out {
		for j, id := range out[i].SupportingIDs {
			if resolved, ok := aliases[id]; ok {
				out[i].SupportingIDs[j] = resolved
			}
		}
	}
	c.Evidence = out
	if err := normalizeSocialPosts(&c); err != nil {
		return c, err
	}
	byID := map[string]domain.Evidence{}
	for _, e := range out {
		byID[e.ID] = e
	}
	relationIDs := map[string]bool{}
	if err := normalizeNames(&c, aliases, byID); err != nil {
		return c, err
	}
	relationsByID := map[string]domain.Relation{}
	for i := range c.Relations {
		r := &c.Relations[i]
		if resolved, ok := aliases[r.EvidenceID]; ok {
			r.EvidenceID = resolved
		}
		if resolved, ok := aliases[r.SubjectEvidenceID]; ok {
			r.SubjectEvidenceID = resolved
		}
		e, ok := byID[r.EvidenceID]
		if !ok || e.Status != "OBSERVED" || e.CandidateID != r.CandidateID || !extract.RelationTextType(e.Type) || r.ID == "" || relationIDs[r.ID] || r.Status != "UNVERIFIED" || r.Method == "" {
			return c, fmt.Errorf("invalid relation provenance %s", r.ID)
		}
		if err := extract.ValidateRelation(*r, r.Subject, extract.Document{ID: e.ID, Type: e.Type, Text: e.Value}); err != nil {
			return c, fmt.Errorf("invalid relation %s: %w", r.ID, err)
		}
		if r.SubjectEvidenceID != "" || r.SubjectQuote != "" {
			title, found := byID[r.SubjectEvidenceID]
			if !found || title.CandidateID != r.CandidateID || title.Status != "OBSERVED" {
				return c, fmt.Errorf("invalid subject provenance %s", r.ID)
			}
			raw := ""
			for _, p := range c.Candidates {
				if p.ID == r.CandidateID {
					raw = p.ProfileURL
					break
				}
			}
			bodyDoc := extract.Document{ID: e.ID, Type: e.Type, Text: e.Value}
			titleDoc := extract.Document{ID: title.ID, Type: title.Type, Text: title.Value}
			var bindingErr error
			if r.Method == extract.ArticleBindingVersion {
				asOf := time.Now().UTC()
				if len(e.Sources) > 0 {
					asOf, _ = time.Parse(time.RFC3339, e.Sources[0].ObservedAt)
				}
				for _, run := range c.Extraction {
					if run.CandidateID == r.CandidateID && run.Mode == "rules" {
						if extractedAt, err := time.Parse(time.RFC3339, run.ObservedAt); err == nil {
							asOf = extractedAt
						}
						break
					}
				}
				bindingErr = extract.ValidateArticleBinding(*r, raw, bodyDoc, titleDoc, asOf)
			} else {
				bindingErr = extract.ValidateProfileBinding(*r, raw, bodyDoc, titleDoc)
			}
			if bindingErr != nil {
				return c, fmt.Errorf("invalid bound relation %s: %w", r.ID, bindingErr)
			}
		}
		previousTime := time.Time{}
		for _, review := range r.Reviews {
			at, err := time.Parse(time.RFC3339, review.At)
			if err != nil || at.Before(previousTime) || strings.TrimSpace(review.Reason) == "" || len(review.Reason) > 2000 || (review.Decision != "ACCEPT" && review.Decision != "REJECT" && review.Decision != "PENDING") {
				return c, fmt.Errorf("invalid relationship review %s", r.ID)
			}
			previousTime = at
		}
		relationIDs[r.ID] = true
		relationsByID[r.ID] = *r
	}
	for _, candidate := range c.Candidates {
		for field, claims := range map[string][]domain.SummaryClaim{"role": candidate.RoleClaims, "company": candidate.CompanyClaims, "education": candidate.EducationClaims} {
			for _, claim := range claims {
				if claim.Value == "" || len(claim.RelationIDs) == 0 {
					return c, fmt.Errorf("invalid %s summary claim for %s", field, candidate.ID)
				}
				for _, id := range claim.RelationIDs {
					r, ok := relationsByID[id]
					value := extract.CanonicalRole(r.Role)
					if field == "role" && canonical(claim.Value) == canonical(extract.PriorRoleSummary(r.Role)) {
						value = claim.Value
					}
					if field == "company" {
						value = r.Organization
						if r.Kind != "employment" {
							value = ""
						}
					}
					if field == "education" {
						value = r.Organization
						if r.Kind != "education" {
							value = ""
						}
					}
					if !ok || r.CandidateID != candidate.ID || !strings.EqualFold(strings.Join(strings.Fields(value), " "), strings.Join(strings.Fields(claim.Value), " ")) || r.Kind != claim.Kind || r.Temporal != claim.Temporal {
						return c, fmt.Errorf("invalid %s summary reference %s", field, id)
					}
				}
			}
		}
	}
	return c, nil
}
