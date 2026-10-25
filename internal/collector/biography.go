package collector

import (
	"context"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/personname"
)

func (d *Discovery) findBiography(ctx context.Context, q Query, r *Result, seen map[string]int, unavailable map[int]bool) {
	for _, p := range r.Profiles {
		if hasPublicName(q.Name, p) {
			return
		}
	}
	for i, provider := range d.providers {
		if unavailable[i] {
			continue
		}
		if ctx.Err() != nil || q.Budget-r.Requests < 2 || len(r.Profiles) >= q.Limit {
			return
		}
		query := Query{Name: q.Name, Purpose: "biography", Budget: 1, Limit: q.Limit - len(r.Profiles)}
		part := provider.Collect(ctx, query)
		r.Requests += part.Requests
		r.Events = append(r.Events, part.Events...)
		unavailable[i] = operationalFailure(part)
		r.Research = append(r.Research, domain.ResearchStep{Action: "search_biography", Query: discoveryPhrase(query), Reason: "initial results lack an explicit public name relationship; inspect biographical sources", Status: part.Status, Requests: part.Requests})
		added := false
		for _, p := range part.Profiles {
			if _, err := cleanURL(p.URL); err != nil {
				continue
			}
			key := discoveryURLKey(p.URL)
			if at, exists := seen[key]; exists {
				mergeProfileFacts(&r.Profiles[at], p)
				continue
			}
			if len(r.Profiles) >= q.Limit {
				break
			}
			p.DiscoveryBasis = "biography query result; public name and identity claims require source review"
			seen[key] = len(r.Profiles)
			r.Profiles = append(r.Profiles, p)
			added = true
		}
		if added {
			return
		}
		for _, p := range r.Profiles {
			if hasPublicName(q.Name, p) {
				return
			}
		}
	}
}

func hasPublicName(subject string, p Profile) bool {
	for _, f := range p.Facts {
		if (f.Type == "search_title" || f.Type == "page_title" || f.Type == "profile_description") && personname.ExcludedContext(f.Value) {
			return false
		}
	}
	for _, f := range p.Facts {
		if personname.TextType(f.Type) && len(personname.Find(subject, f.Value)) > 0 {
			return true
		}
	}
	return false
}
