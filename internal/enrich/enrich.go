package enrich

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/evidence"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/extract"
)

// Run replays saved observations without collecting new web data.
func Run(ctx context.Context, input domain.Case, cfg extract.Config) (domain.Case, error) {
	if err := cfg.Validate(); err != nil {
		return input, err
	}
	cfg = cfg.Effective()
	encoded, err := json.Marshal(input)
	if err != nil {
		return input, err
	}
	var c domain.Case
	if err = json.Unmarshal(encoded, &c); err != nil {
		return input, err
	}
	c, err = evidence.Normalize(c)
	if err != nil {
		return input, err
	}
	reviewed := map[string]domain.Relation{}
	reviewedList := []domain.Relation{}
	for _, r := range c.Relations {
		if len(r.Reviews) > 0 {
			reviewed[relationKey(r)] = r
			reviewedList = append(reviewedList, r)
		}
	}
	c.Relations = nil
	c.Extraction = nil
	for i := range c.Evidence {
		e := &c.Evidence[i]
		if e.Status == "INFERRED" && e.InferenceRule != "" && (e.Type == "workplace" || e.Type == "role") {
			e.SupersededBy = extract.RelationVersion
		}
	}
	requests := 0
	for i := range c.Candidates {
		p := &c.Candidates[i]
		if p.CompanyInferred {
			p.Company = ""
		}
		if p.RoleInferred {
			p.Role = ""
		}
		p.CompanyInferred = false
		p.RoleInferred = false
		p.CompanyTemporal = ""
		p.RoleTemporal = ""
		p.CompanyRelationIDs = nil
		p.RoleRelationIDs = nil
		p.CompanyAmbiguous = false
		p.CompanyAlternatives = nil
		p.RoleAlternatives = nil
		notes := []string{}
		for _, n := range p.Missing {
			if !strings.HasPrefix(n, "Extraction:") && !strings.HasPrefix(n, "Company is inferred") && !strings.HasPrefix(n, "Company inference") && !strings.HasPrefix(n, "Role is inferred") {
				notes = append(notes, n)
			}
		}
		p.Missing = notes
		subject := c.Target.Name
		if c.Search != nil && c.Search.Name != "" {
			subject = c.Search.Name
		} else if p.DisplayName != "" {
			subject = p.DisplayName
		}
		docs := []extract.Document{}
		for _, e := range c.Evidence {
			if e.CandidateID == p.ID && e.Status == "OBSERVED" && extract.RelationTextType(e.Type) {
				docs = append(docs, extract.Document{ID: e.ID, Type: e.Type, Text: e.Value})
			}
		}
		asOf := time.Now().UTC()
		relations := extract.Relations(subject, docs, asOf)
		encoded, _ := json.Marshal(docs)
		trace := domain.ExtractionRun{CandidateID: p.ID, Version: extract.RelationVersion, Mode: "rules", Status: "SUCCESS", ObservedAt: asOf.Format(time.RFC3339), InputSHA256: fmt.Sprintf("%x", sha256.Sum256(encoded)), Accepted: len(relations)}
		if len(relations) == 0 {
			trace.Notes = []string{"No subject-bound relationship extracted. Missing fields do not establish absence; aliases and unstructured tables may require review."}
		}
		c.Extraction = append(c.Extraction, trace)
		if cfg.Mode == "hybrid" {
			// Prioritize subject-bearing sentences and bound the complete model input.
			bounded := boundedDocuments(subject, docs)
			if len(bounded) == 0 || requests >= cfg.MaxRequests || ctx.Err() != nil {
				c.Extraction = append(c.Extraction, domain.ExtractionRun{CandidateID: p.ID, Version: extract.RelationVersion, Mode: "hybrid", Model: cfg.Model, Status: "SKIPPED", ObservedAt: asOf.Format(time.RFC3339), InputSHA256: trace.InputSHA256, Notes: []string{"No matching context, local request budget exhausted, or session cancelled"}})
			} else {
				requests++
				proposals, run := extract.LocalRelations(ctx, cfg, subject, bounded, asOf)
				run.CandidateID = p.ID
				c.Extraction = append(c.Extraction, run)
				relations = append(relations, proposals...)
			}
		}
		for _, r := range reviewedList {
			if r.CandidateID == p.ID {
				relations = append(relations, r)
			}
		}
		seen := map[string]bool{}
		for _, r := range relations {
			r.CandidateID = p.ID
			key := relationKey(r)
			if seen[key] {
				continue
			}
			seen[key] = true
			if old, ok := reviewed[key]; ok {
				r = old
			}
			digest := sha256.Sum256([]byte(key))
			r.ID = fmt.Sprintf("%s-relation-%x", p.ID, digest[:8])
			c.Relations = append(c.Relations, r)
		}
		projectSummary(p, c.Relations)
	}
	return evidence.Normalize(c)
}

func relationKey(r domain.Relation) string {
	data, _ := json.Marshal([]any{r.CandidateID, r.EvidenceID, r.Subject, r.Kind, r.Role, r.Organization, r.StartYear, r.EndYear, r.Temporal})
	return string(data)
}

func boundedDocuments(subject string, docs []extract.Document) []extract.Document {
	out := []extract.Document{}
	remaining := 12000
	for _, d := range docs {
		if remaining <= 0 {
			break
		}
		// Keep whole contiguous observations so quotes remain auditable.
		i := strings.Index(strings.ToLower(d.Text), strings.ToLower(subject))
		if i < 0 {
			continue
		}
		start := max(0, i-200)
		for start > 0 && d.Text[start]&0xc0 == 0x80 {
			start--
		}
		runes := []rune(d.Text[start:])
		if len(runes) > remaining {
			runes = runes[:remaining]
		}
		d.Text = string(runes)
		remaining -= len(runes)
		out = append(out, d)
	}
	return out
}
