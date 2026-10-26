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
	if strings.TrimSpace(q.Name) != "" {
		next := q
		next.Platform, next.Username, next.Accounts, next.URL = "", "", nil, ""
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
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) == 2 {
			tab := strings.ToLower(parts[1])
			platform := platformFor(u.Hostname())
			reserved := strings.Contains(" p reel reels explore accounts i search ", " "+strings.ToLower(parts[0])+" ")
			if !reserved && (platform == "instagram" && (tab == "reels" || tab == "feed" || tab == "tagged") || platform == "x" && (tab == "media" || tab == "with_replies")) {
				u.Path = "/" + parts[0]
			}
		}
		query := u.Query()
		for key := range query {
			if strings.HasPrefix(strings.ToLower(key), "utm_") || key == "hl" || key == "igsh" || key == "igshid" || key == "fbclid" {
				query.Del(key)
			}
		}
		u.RawQuery = query.Encode()
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
	seen := map[Fact]bool{}
	counts := map[string]int{}
	for _, fact := range dst.Facts {
		seen[fact] = true
		counts[fact.Type]++
		dst.FactSources[fact.Type+"\x00"+fact.Value] = SourceForFact(*dst, fact)
	}
	for _, fact := range src.Facts {
		if seen[fact] || counts[fact.Type] >= 8 {
			continue
		}
		seen[fact] = true
		counts[fact.Type]++
		dst.Facts = append(dst.Facts, fact)
		source := SourceForFact(src, fact)
		dst.FactSources[fact.Type+"\x00"+fact.Value] = source
		if counts[fact.Type] == 1 {
			dst.FactSources[fact.Type] = source
		}
	}
}

// SourceForFact keeps query-specific provenance for distinct values of a field.
func SourceForFact(p Profile, fact Fact) FactSource {
	if source, ok := p.FactSources[fact.Type+"\x00"+fact.Value]; ok {
		return source
	}
	if source, ok := p.FactSources[fact.Type]; ok {
		return source
	}
	return FactSource{URL: p.SourceURL, ResponseSHA256: p.ResponseSHA256, Rule: p.Rule}
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
