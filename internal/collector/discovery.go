package collector

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type DuckDuckGo struct {
	client   *http.Client
	endpoint string
}

func NewDuckDuckGo(timeout time.Duration) *DuckDuckGo {
	return &DuckDuckGo{publicClient(timeout), "https://html.duckduckgo.com/html/"}
}

func (d *DuckDuckGo) Collect(ctx context.Context, q Query) Result {
	r := Result{Status: "FAILED", Incomplete: true}
	if searchWords(q.Name+" "+q.Username) == "" || q.Budget < 1 || q.Limit < 1 || q.Limit > 20 {
		r.Events = append(r.Events, Event{"duckduckgo:input", "FAILED", "Name or username and valid limits required"})
		return r
	}
	platforms := []Platform{{Name: "web"}}
	if q.Platform != "" {
		platforms = nil
		for _, p := range Platforms() {
			if q.Platform == "all" || q.Platform == p.Name {
				platforms = append(platforms, p)
			}
		}
	}
	if len(platforms) == 0 {
		r.Events = append(r.Events, Event{"duckduckgo:input", "FAILED", "Unknown platform"})
		return r
	}
	seen := map[string]bool{}
	blocked := false
	for _, p := range platforms {
		op := "duckduckgo:" + p.Name
		if blocked || ctx.Err() != nil || r.Requests >= q.Budget || len(r.Profiles) >= q.Limit {
			r.Events = append(r.Events, Event{op, "NOT_CHECKED", "Restriction, cancellation, or collection limit reached"})
			continue
		}
		if r.Requests > 0 {
			timer := time.NewTimer(2 * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				r.Events = append(r.Events, Event{op, "NOT_CHECKED", "Cancelled"})
				continue
			case <-timer.C:
			}
		}
		query := discoveryPhrase(q)
		if len(p.Domains) > 0 {
			query += " (site:" + strings.Join(p.Domains, " OR site:") + ")"
		}
		raw := d.endpoint + "?" + url.Values{"q": {query}}.Encode()
		body, status, detail, _ := readPublic(ctx, d.client, raw, nil)
		r.Requests++
		if status != "SUCCESS" {
			r.Status = status
			blocked = status == "BLOCKED"
			r.Events = append(r.Events, Event{op, status, detail})
			continue
		}
		profiles, status := parseDuckDuckGo(body, raw, q, p.Name)
		if status == "BLOCKED" {
			cooldownAccess(ctx, raw, 15*time.Minute, "search challenge page")
		}
		r.Status = status
		blocked = status == "BLOCKED"
		rankResults(profiles, q)
		for _, profile := range profiles {
			if !seen[profile.URL] && len(r.Profiles) < q.Limit {
				seen[profile.URL] = true
				r.Profiles = append(r.Profiles, profile)
			}
		}
		r.Events = append(r.Events, Event{op, status, "Public HTML index; matched links require identity review. Challenges and unknown layouts are not account absence."})
	}
	if len(r.Profiles) > 0 {
		r.Status = "SUCCESS"
	}
	return r
}

func parseDuckDuckGo(body []byte, source string, q Query, platform string) ([]Profile, string) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, "FAILED"
	}
	if blockedPage(doc) {
		return nil, "BLOCKED"
	}
	profiles := []Profile{}
	challenge := false
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && (attr(n, "id") == "challenge-form" || attr(n, "id") == "anomaly-modal") {
			challenge = true
		}
		if n.Type == html.ElementNode && n.Data == "a" && hasClass(n, "result__a") {
			link := attr(n, "href")
			u, err := url.Parse(link)
			if err == nil {
				if target := u.Query().Get("uddg"); target != "" {
					link = target
				}
				u, err = cleanURL(link)
			}
			title, snippet := nodeText(n), ""
			for parent := n.Parent; parent != nil; parent = parent.Parent {
				if hasClass(parent, "result") {
					snippet = nodeText(parent)
					break
				}
			}
			if err == nil && (platform == "web" || platformFor(u.Hostname()) == platform) && relevantSearchResult(q, u.String(), title, snippet) {
				profiles = append(profiles, indexedProfile(q, u.String(), source, title, snippet))
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if challenge {
		return nil, "BLOCKED"
	}
	if len(profiles) == 0 {
		return nil, "FAILED"
	}
	return profiles, "SUCCESS"
}

type Discovery struct {
	providers []Collector
	page      Collector
}

func NewDiscovery(timeout time.Duration) *Discovery {
	local, _ := NewSearXNG("http://127.0.0.1:8888", timeout)
	return &Discovery{[]Collector{local, NewDuckDuckGo(timeout), NewBing(timeout), NewWikipedia(timeout)}, NewPublicPage(timeout, nil)}
}

func (d *Discovery) Collect(ctx context.Context, q Query) Result {
	r := Result{Status: "FAILED", Incomplete: true}
	if q.Budget < 1 || q.Limit < 1 || q.Limit > 20 {
		r.Events = []Event{{"web:input", "FAILED", "Valid request and candidate budgets required"}}
		return r
	}
	accounts, err := NormalizeAccounts(q.Accounts)
	if err != nil {
		r.Events = []Event{{"web:input", "FAILED", err.Error()}}
		return r
	}
	q.Accounts = accounts
	seen := map[string]int{}
	attempted := map[string]bool{}
	unavailable := map[int]bool{}
	status := "FAILED"
	for _, anchor := range discoveryAnchors(q) {
		key := discoveryURLKey(anchor.URL)
		if attempted[key] {
			continue
		}
		if ctx.Err() != nil || r.Requests >= q.Budget || len(r.Profiles) >= q.Limit {
			r.Events = append(r.Events, Event{"web:anchor", "NOT_CHECKED", "Account or URL clue not fetched: budget, candidate limit, or cancellation"})
			continue
		}
		attempted[key] = true
		part := d.collectAnchor(ctx, anchor, q.Budget-r.Requests)
		r.Requests += part.Requests
		r.Access = append(r.Access, part.Access...)
		r.Events = append(r.Events, part.Events...)
		status = part.Status
		for _, p := range part.Profiles {
			key = discoveryURLKey(p.URL)
			attempted[key] = true
			if i, ok := seen[key]; ok {
				mergeProfileFacts(&r.Profiles[i], p)
				continue
			}
			seen[key] = len(r.Profiles)
			p.DiscoveryBasis = "supplied public account or URL clue; ownership unverified"
			r.Profiles = append(r.Profiles, p)
		}
	}
	research := q.Budget >= 8 && (len(q.Accounts) > 0 || q.URL != "" || q.Username != "")
	biography := q.Budget >= 8 && q.Limit >= 3 && q.Platform == "" && strings.TrimSpace(q.Name) != ""
	if research && strings.TrimSpace(q.Name) == "" {
		d.followResearchLinks(ctx, q, &r, attempted, seen, 3)
	}
	reserve := 0
	if research {
		reserve = min(6, (q.Budget-r.Requests)/2)
	}
	if biography && !research {
		reserve = 2
	}
	for providerIndex, provider := range d.providers {
		for _, query := range discoveryQueries(q) {
			if unavailable[providerIndex] {
				break
			}
			if ctx.Err() != nil || r.Requests >= q.Budget || len(r.Profiles) >= q.Limit {
				break
			}
			next := query
			next.Budget -= r.Requests + reserve
			if research || biography {
				next.Budget = min(2, next.Budget)
			}
			next.Limit = q.Limit - len(r.Profiles)
			if (research || biography) && q.Limit > 2 {
				next.Limit--
			}
			if len(q.Accounts) > 0 && q.Name != "" && next.Username != "" {
				next.Limit = min(1, next.Limit)
			}
			if next.Budget < 1 || next.Limit < 1 {
				break
			}
			part := provider.Collect(ctx, next)
			unavailable[providerIndex] = operationalFailure(part)
			r.Requests += part.Requests
			r.Events = append(r.Events, part.Events...)
			if part.Status == "BLOCKED" {
				status = "BLOCKED"
			} else if part.Status == "EMPTY" && status != "BLOCKED" {
				status = "EMPTY"
			}
			for _, profile := range part.Profiles {
				if len(r.Profiles) >= q.Limit {
					break
				}
				key := discoveryURLKey(profile.URL)
				if key == "" {
					continue
				}
				if i, ok := seen[key]; ok {
					mergeProfileFacts(&r.Profiles[i], profile)
					continue
				}
				seen[key] = len(r.Profiles)
				r.Profiles = append(r.Profiles, profile)
			}
		}
	}
	if biography {
		d.findBiography(ctx, q, &r, seen, unavailable)
	}
	if len(r.Profiles) > 0 {
		status = "SUCCESS"
	}
	r.Status = status
	rankResults(r.Profiles, q)
	for i := range r.Profiles {
		p := &r.Profiles[i]
		op := fmt.Sprintf("web:page:%d", i+1)
		if attempted[discoveryURLKey(p.URL)] {
			continue
		}
		if ctx.Err() != nil || r.Requests >= q.Budget {
			r.Events = append(r.Events, Event{op, "NOT_CHECKED", "Page fetch budget or session time exhausted"})
			continue
		}
		attempted[discoveryURLKey(p.URL)] = true
		page := d.page.Collect(ctx, Query{URL: p.URL, Budget: q.Budget - r.Requests, Limit: 1})
		r.Requests += page.Requests
		r.Access = append(r.Access, page.Access...)
		for _, event := range page.Events {
			if strings.HasPrefix(event.Operation, "instagram:") {
				r.Events = append(r.Events, event)
			}
		}
		detail := "Destination extraction failed; indexed evidence retained"
		if len(page.Events) > 0 {
			detail = page.Events[len(page.Events)-1].Detail
		}
		r.Events = append(r.Events, Event{op, page.Status, detail})
		if len(page.Profiles) == 0 {
			continue
		}
		observed := page.Profiles[0]
		attempted[discoveryURLKey(observed.URL)] = true
		mergeProfileFacts(p, observed)
	}
	// Rebuild after ranking so URL indexes still point at the right documents.
	seen = map[string]int{}
	for i, p := range r.Profiles {
		seen[discoveryURLKey(p.URL)] = i
	}
	if research {
		d.researchQueries(ctx, q, &r, attempted, seen, unavailable)
	}
	d.followResearchLinks(ctx, q, &r, attempted, seen, 4)
	if len(r.Profiles) > 0 {
		r.Status = "SUCCESS"
	} else {
		// An empty fallback cannot establish absence when another source failed.
		for _, event := range r.Events {
			if event.Status == "BLOCKED" {
				r.Status = "BLOCKED"
				break
			}
			if event.Status == "FAILED" {
				r.Status = "FAILED"
			}
		}
	}
	return r
}
