package evidence

import (
	"encoding/hex"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
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
	if c.Search != nil && confirmed > 1 {
		return c, fmt.Errorf("a search session can confirm only one candidate")
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
		e.Value = strings.Join(strings.Fields(e.Value), " ")
		key := e.CandidateID + "\x00" + e.Type + "\x00" + canonical(e.Value)
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
	return c, nil
}
