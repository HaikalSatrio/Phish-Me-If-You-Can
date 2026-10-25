package collector

import (
	"context"
	"strings"
)

type discoveryAnchor struct {
	URL, Platform, Username string
	Derived                 bool
}

func discoveryQueries(q Query) []Query {
	if len(q.Accounts) == 0 {
		if q.Name == "" && q.Username == "" {
			return nil
		}
		return []Query{q}
	}
	out := []Query{}
	for _, account := range q.Accounts {
		next := q
		next.Platform, next.Username = account.Platform, account.Username
		next.Accounts, next.URL = nil, ""
		out = append(out, next)
	}
	return out
}

func discoveryAnchors(q Query) []discoveryAnchor {
	out := []discoveryAnchor{}
	if q.URL != "" {
		out = append(out, discoveryAnchor{URL: q.URL, Platform: q.Platform})
	}
	for _, query := range discoveryQueries(q) {
		if query.Username == "" || query.Platform == "" || query.Platform == "all" {
			continue
		}
		if raw, err := accountURL(query.Platform, query.Username); err == nil {
			out = append(out, discoveryAnchor{URL: raw, Platform: query.Platform, Username: query.Username, Derived: true})
		}
	}
	return out
}

func discoveryURLKey(raw string) string {
	u, err := cleanURL(raw)
	if err != nil {
		return raw
	}
	if platformFor(u.Hostname()) != "web" {
		u.Host = strings.TrimPrefix(u.Host, "www.")
		u.Path = strings.TrimSuffix(u.Path, "/")
	}
	return u.String()
}

func mergeProfileFacts(dst *Profile, src Profile) {
	if dst.FactSources == nil {
		dst.FactSources = map[string]FactSource{}
	}
	if dst.Name == "" && src.Name != "" {
		dst.Name = src.Name
		source, ok := src.FactSources["name"]
		if !ok {
			source = FactSource{URL: src.SourceURL, ResponseSHA256: src.ResponseSHA256, Rule: src.Rule}
		}
		dst.FactSources["name"] = source
	}
	seen := map[string]bool{}
	for _, fact := range dst.Facts {
		seen[fact.Type] = true
	}
	for _, fact := range src.Facts {
		if seen[fact.Type] {
			continue
		}
		seen[fact.Type] = true
		dst.Facts = append(dst.Facts, fact)
		source, ok := src.FactSources[fact.Type]
		if !ok {
			source = FactSource{URL: src.SourceURL, ResponseSHA256: src.ResponseSHA256, Rule: src.Rule}
		}
		dst.FactSources[fact.Type] = source
	}
}

func (d *Discovery) collectAnchor(ctx context.Context, anchor discoveryAnchor, budget int) Result {
	r := Result{Status: "FAILED"}
	op := "web:anchor"
	if anchor.Platform != "" {
		op += ":" + anchor.Platform
	}
	u, err := cleanURL(anchor.URL)
	if err != nil || anchor.Platform != "" && anchor.Platform != "all" && platformFor(u.Hostname()) != anchor.Platform {
		r.Status = "BLOCKED"
		r.Events = []Event{{op, r.Status, "Invalid public URL or URL outside the requested platform"}}
		return r
	}
	r = d.page.Collect(ctx, Query{URL: anchor.URL, Budget: budget, Limit: 1})
	detail := "Public page could not be collected"
	if len(r.Events) > 0 {
		detail = r.Events[len(r.Events)-1].Detail
	}
	if r.Status == "SUCCESS" && len(r.Profiles) > 0 {
		p := r.Profiles[0]
		final, finalErr := cleanURL(p.URL)
		match := finalErr == nil
		if match && anchor.Platform != "" && anchor.Platform != "all" {
			match = platformFor(final.Hostname()) == anchor.Platform
		}
		if match && anchor.Derived {
			match = strings.EqualFold(strings.Trim(final.Path, "/"), strings.Trim(u.Path, "/"))
			metadata := ""
			for _, f := range p.Facts {
				if f.Type == "page_title" || f.Type == "page_description" {
					metadata += " " + f.Value
				}
				if f.Type == "page_title" {
					for _, unavailable := range []string{"page not found", "profile not found", "user not found", "account not found", "account suspended", "account doesn't exist", "page isn't available"} {
						if strings.Contains(strings.ToLower(f.Value), unavailable) {
							match = false
						}
					}
				}
			}
			match = match && relevantResult(Query{Username: anchor.Username}, metadata)
		}
		if match {
			p.Key = keyFor("web", p.URL)
			r.Profiles = []Profile{p}
			detail = "Public page retained as a candidate; account ownership and employment remain unverified"
		} else {
			r.Status, r.Profiles = "EMPTY", nil
			detail = "Page metadata did not identify the supplied handle, or the destination changed; this does not establish account absence"
		}
	} else {
		r.Profiles = nil
		if r.Status == "SUCCESS" {
			r.Status = "EMPTY"
		}
	}
	r.Events = []Event{{op, r.Status, detail}}
	return r
}
