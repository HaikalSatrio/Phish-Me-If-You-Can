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
	c.NameClaims = nil
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
		recoverProfileName(p, &c.Evidence)
		subject := c.Target.Name
		if p.DisplayName != "" {
			subject = p.DisplayName
		} else if c.Search != nil && c.Search.Name != "" {
			subject = c.Search.Name
		}
		docs := []extract.Document{}
		for _, e := range c.Evidence {
			if e.CandidateID == p.ID && e.Status == "OBSERVED" && extract.RelationTextType(e.Type) {
				docs = append(docs, extract.Document{ID: e.ID, Type: e.Type, Text: e.Value})
			}
		}
		asOf := time.Now().UTC()
		names := publicNames(subject, *p, docs)
		c.NameClaims = append(c.NameClaims, names...)
		relations := extract.Relations(subject, docs, asOf)
		// Alternate subjects are used only within the document that explicitly names them.
		for _, claim := range names {
			if strings.EqualFold(claim.Name, subject) {
				continue
			}
			for _, doc := range docs {
				if doc.ID == claim.EvidenceID {
					relations = append(relations, extract.Relations(claim.Name, []extract.Document{doc}, asOf)...)
				}
			}
		}
		relations = append(relations, extract.BoundProfileRelations(subject, p.ProfileURL, docs, asOf)...)
		relations = append(relations, extract.BoundArticleRelations(subject, p.ProfileURL, docs, asOf)...)
		encoded, _ := json.Marshal(docs)
		trace := domain.ExtractionRun{CandidateID: p.ID, Version: extract.RelationVersion, Mode: "rules", Status: "SUCCESS", ObservedAt: asOf.Format(time.RFC3339), InputSHA256: fmt.Sprintf("%x", sha256.Sum256(encoded)), Accepted: len(relations)}
		if len(relations) == 0 {
			trace.Notes = []string{"No subject-bound relationship extracted. Missing fields do not establish absence; aliases and unstructured tables may require review."}
		}
		c.Extraction = append(c.Extraction, trace)
		if cfg.Mode == "hybrid" {
			if len(extract.SelectPassages(subject, docs)) == 0 || requests >= cfg.MaxRequests || ctx.Err() != nil {
				c.Extraction = append(c.Extraction, domain.ExtractionRun{CandidateID: p.ID, Version: extract.RelationVersion, Mode: "hybrid", Model: cfg.Model, Status: "SKIPPED", ObservedAt: asOf.Format(time.RFC3339), InputSHA256: trace.InputSHA256, Notes: []string{"No matching context, local request budget exhausted, or session cancelled"}})
			} else {
				requests++
				proposals, run := extract.LocalRelations(ctx, cfg, subject, docs, asOf)
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
	values := []any{r.CandidateID, r.EvidenceID, r.Subject, r.Kind, r.Role, r.Organization, r.StartYear, r.EndYear, r.Temporal}
	if r.SubjectEvidenceID != "" {
		values = append(values, r.SubjectEvidenceID)
	}
	data, _ := json.Marshal(values)
	return string(data)
}
