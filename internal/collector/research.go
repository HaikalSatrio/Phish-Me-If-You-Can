package collector

import (
	"context"
	"strings"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
)

type researchQuery struct{ name, username, from, reason string }

func plannedResearch(q Query, profiles []Profile) []researchQuery {
	out := []researchQuery{}
	seen := map[string]bool{strings.ToLower(q.Name): true}
	add := func(name, from, reason string) {
		key := strings.ToLower(strings.Join(strings.Fields(name), " "))
		if seen[key] || !plausiblePersonName(name) {
			return
		}
		seen[key] = true
		out = append(out, researchQuery{name: name, from: from, reason: reason})
	}
	for _, p := range profiles {
		if p.Name != "" {
			add(p.Name, p.URL, "observed display name on source; cross-page identity unverified")
		}
	}
	for _, anchor := range discoveryAnchors(q) {
		if anchor.Username != "" {
			out = append(out, researchQuery{username: anchor.Username, from: anchor.URL, reason: "search supplied handle across the public web, including external biographies"})
		}
	}
	for _, anchor := range discoveryAnchors(q) {
		if hypothesis := handleHypothesis(anchor.Username, q.Name); hypothesis != "" {
			add(hypothesis, anchor.URL, "name hypothesis from handle segmentation; not an observed name")
		}
	}
	return out
}

func (d *Discovery) followResearchLinks(ctx context.Context, q Query, r *Result, attempted map[string]bool, seen map[string]int, maxSteps int) {
	steps := 0
	for _, step := range r.Research {
		if step.Action == "follow_link" {
			steps++
		}
	}
	// Appended pages may contribute the next hop. The step cap bounds depth and breadth.
	for steps < maxSteps {
		parent, link, ok := nextResearchLink(r.Profiles, attempted)
		if !ok {
			return
		}
		key := discoveryURLKey(link.URL)
		if ctx.Err() != nil || r.Requests >= q.Budget || len(r.Profiles) >= q.Limit || steps >= maxSteps {
			return
		}
		attempted[key] = true
		steps++
		part := d.page.Collect(ctx, Query{URL: link.URL, Limit: 1, Budget: min(3, q.Budget-r.Requests)})
		r.Requests += part.Requests
		r.Research = append(r.Research, domain.ResearchStep{Action: "follow_link", FromURL: parent.URL, URL: link.URL, Reason: link.Reason, Status: part.Status, Requests: part.Requests})
		r.Events = append(r.Events, Event{"web:research:link", part.Status, link.URL + ": " + link.Reason})
		for _, p := range part.Profiles {
			key = discoveryURLKey(p.URL)
			attempted[key] = true
			if at, ok := seen[key]; ok {
				mergeProfileFacts(&r.Profiles[at], p)
				continue
			}
			if len(r.Profiles) >= q.Limit {
				break
			}
			p.Key = keyFor("web", p.URL)
			p.DiscoveryBasis, p.LinkedFrom = link.Reason, parent.URL
			seen[key] = len(r.Profiles)
			r.Profiles = append(r.Profiles, p)
		}
	}
}

func nextResearchLink(profiles []Profile, attempted map[string]bool) (Profile, researchLink, bool) {
	var parent Profile
	var selected researchLink
	best := 0
	for _, p := range profiles {
		for _, link := range researchLinks(p) {
			if attempted[discoveryURLKey(link.URL)] {
				continue
			}
			u, err := cleanURL(link.URL)
			if err != nil {
				continue
			}
			score := 10
			if platformFor(u.Hostname()) == "web" {
				score = 70
			}
			if strings.Contains(link.Reason, "README") {
				score = 80
			}
			if strings.Contains(link.Reason, "biography") {
				score = 100
			}
			if score > best {
				parent, selected, best = p, link, score
			}
		}
	}
	return parent, selected, best > 0
}

func (d *Discovery) researchQueries(ctx context.Context, q Query, r *Result, attempted map[string]bool, seen map[string]int, unavailable map[int]bool) {
	searched := map[string]bool{}
	for round := 0; round < 3; round++ {
		var plan researchQuery
		for _, candidate := range plannedResearch(q, r.Profiles) {
			if !searched[strings.ToLower(candidate.name+"|"+candidate.username)] {
				plan = candidate
				break
			}
		}
		if plan.name == "" && plan.username == "" {
			break
		}
		searched[strings.ToLower(plan.name+"|"+plan.username)] = true
		queryText := plan.name
		if plan.username != "" {
			queryText = plan.username
		}
		for providerIndex, provider := range d.providers {
			if unavailable[providerIndex] {
				continue
			}
			if ctx.Err() != nil || r.Requests >= q.Budget || len(r.Profiles) >= q.Limit {
				return
			}
			// Reserve one request for inspecting a result rather than keeping only snippets.
			if q.Budget-r.Requests < 2 {
				return
			}
			query := Query{Name: plan.name, Username: plan.username, Company: q.Company, Location: q.Location, Budget: 1, Limit: min(2, q.Limit-len(r.Profiles))}
			part := provider.Collect(ctx, query)
			unavailable[providerIndex] = operationalFailure(part)
			r.Requests += part.Requests
			r.Events = append(r.Events, part.Events...)
			action := "search_name"
			if plan.username != "" {
				action = "search_handle"
			}
			r.Research = append(r.Research, domain.ResearchStep{Action: action, FromURL: plan.from, Query: queryText, Reason: plan.reason, Status: part.Status, Requests: part.Requests})
			added := false
			for _, p := range part.Profiles {
				if len(r.Profiles) >= q.Limit {
					break
				}
				if _, err := cleanURL(p.URL); err != nil {
					continue
				}
				key := discoveryURLKey(p.URL)
				if _, exists := seen[key]; exists {
					continue
				}
				p.Key = keyFor("web", p.URL)
				p.DiscoveryBasis, p.LinkedFrom = plan.reason, plan.from
				if !attempted[key] && r.Requests < q.Budget && ctx.Err() == nil {
					attempted[key] = true
					page := d.page.Collect(ctx, Query{URL: p.URL, Limit: 1, Budget: min(2, q.Budget-r.Requests)})
					r.Requests += page.Requests
					r.Research = append(r.Research, domain.ResearchStep{Action: "inspect_result", FromURL: plan.from, URL: p.URL, Query: queryText, Reason: "inspect destination evidence; a name match alone does not link identities", Status: page.Status, Requests: page.Requests})
					for _, observed := range page.Profiles {
						attempted[discoveryURLKey(observed.URL)] = true
						mergeProfileFacts(&p, observed)
					}
				}
				seen[key] = len(r.Profiles)
				r.Profiles = append(r.Profiles, p)
				added = true
			}
			if added {
				break
			}
		}
		d.followResearchLinks(ctx, q, r, attempted, seen, 4)
	}
}

func operationalFailure(r Result) bool {
	if r.Status == "BLOCKED" {
		return true
	}
	if r.Status != "FAILED" {
		return false
	}
	for _, event := range r.Events {
		for _, marker := range []string{"service unavailable", "upstream engines unavailable", "connection failed", "dns lookup failed", "network request failed", "request timed out", "tls certificate"} {
			if strings.Contains(strings.ToLower(event.Detail), marker) {
				return true
			}
		}
	}
	return false
}
