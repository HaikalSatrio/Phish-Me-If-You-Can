package collector

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Web struct {
	client                      *http.Client
	provider, key, cx, endpoint string
}

func NewWeb(provider, key, cx string, timeout time.Duration) *Web {
	endpoint := "https://api.search.brave.com/res/v1/web/search"
	if provider == "google" {
		endpoint = "https://customsearch.googleapis.com/customsearch/v1"
	}
	return &Web{client: publicClient(timeout), provider: provider, key: key, cx: cx, endpoint: endpoint}
}

func keyFor(prefix, value string) string {
	return fmt.Sprintf("%s-%x", prefix, sha256.Sum256([]byte(value)))
}

func searchPhrase(q Query) string {
	parts := []string{}
	for _, s := range []string{q.Name, q.Username, q.Company, q.Location} {
		s = strings.TrimSpace(strings.NewReplacer(`"`, " ", "\n", " ", "\r", " ", "\\", " ").Replace(s))
		if s != "" {
			parts = append(parts, `"`+s+`"`)
		}
	}
	return strings.Join(parts, " ")
}

func (w *Web) Collect(ctx context.Context, q Query) Result {
	r := Result{Status: "SUCCESS", Profiles: []Profile{}, Events: []Event{}}
	if w.key == "" || (w.provider == "google" && w.cx == "") {
		r.Status = "BLOCKED"
		r.Events = append(r.Events, Event{w.provider + ":configuration", "BLOCKED", "Search API credentials are not configured. No request made."})
		return r
	}
	term := q.Name
	if term == "" {
		term = q.Username
	}
	if strings.TrimSpace(term) == "" || q.Limit < 1 || q.Limit > 20 || q.Budget < 1 {
		r.Status = "FAILED"
		r.Events = append(r.Events, Event{"web:input", "FAILED", "Name or username and valid limits are required"})
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
		if len(platforms) == 0 {
			r.Status = "FAILED"
			r.Events = append(r.Events, Event{"web:input", "FAILED", "Unknown platform"})
			return r
		}
	}
	seen := map[string]bool{}
	blocked := false
	for _, platform := range platforms {
		op := w.provider + ":" + platform.Name
		if blocked || r.Requests >= q.Budget || ctx.Err() != nil {
			r.Incomplete = true
			r.Events = append(r.Events, Event{op, "NOT_CHECKED", "Access restriction or session request/time budget stopped collection"})
			continue
		}
		if r.Requests > 0 {
			timer := time.NewTimer(time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				r.Incomplete = true
				r.Events = append(r.Events, Event{op, "NOT_CHECKED", "Session cancelled"})
				continue
			case <-timer.C:
			}
		}
		query := searchPhrase(q)
		if len(platform.Domains) > 0 {
			terms := []string{}
			for _, d := range platform.Domains {
				terms = append(terms, "site:"+d)
			}
			query += " (" + strings.Join(terms, " OR ") + ")"
		}
		params := url.Values{"q": {query}}
		headers := map[string]string{"Accept": "application/json"}
		limit := q.Limit
		if w.provider == "google" {
			if limit > 10 {
				limit = 10
			}
			params.Set("num", strconv.Itoa(limit))
			params.Set("cx", w.cx)
			headers["X-Goog-Api-Key"] = w.key
		} else {
			params.Set("count", strconv.Itoa(limit))
			headers["X-Subscription-Token"] = w.key
		}
		raw := w.endpoint + "?" + params.Encode()
		body, status, detail, _ := readPublic(ctx, w.client, raw, headers)
		r.Requests++
		if status != "SUCCESS" {
			if status == "BLOCKED" {
				blocked = true
			}
			r.Events = append(r.Events, Event{op, status, detail})
			r.Incomplete = true
			r.Status = status
			continue
		}
		var response struct {
			Web struct {
				Results []struct{ Title, URL, Description string } `json:"results"`
			} `json:"web"`
			Items []struct{ Title, Link, Snippet string } `json:"items"`
		}
		if err := json.Unmarshal(body, &response); err != nil {
			r.Events = append(r.Events, Event{op, "FAILED", "Malformed search response"})
			r.Status = "FAILED"
			r.Incomplete = true
			continue
		}
		type hit struct{ title, link, snippet string }
		hits := []hit{}
		if w.provider == "google" {
			for _, i := range response.Items {
				hits = append(hits, hit{i.Title, i.Link, i.Snippet})
			}
		} else {
			for _, i := range response.Web.Results {
				hits = append(hits, hit{i.Title, i.URL, i.Description})
			}
		}
		status = "SUCCESS"
		if len(hits) == 0 {
			status = "EMPTY"
		}
		r.Events = append(r.Events, Event{op, status, "Bounded index results; original pages have not been fetched"})
		if len(hits) >= limit {
			r.Incomplete = true
		}
		for i, h := range hits {
			if i >= limit {
				break
			}
			u, err := cleanURL(h.link)
			if err != nil {
				r.Incomplete = true
				continue
			}
			if platform.Name != "web" && platformFor(u.Hostname()) != platform.Name {
				r.Incomplete = true
				continue
			}
			link := u.String()
			if seen[link] {
				continue
			}
			seen[link] = true
			// The source is the search API, not an unvisited destination page.
			r.Profiles = append(r.Profiles, Profile{Key: keyFor("web", link), Platform: platformFor(u.Hostname()), URL: link, SourceURL: raw, Facts: []Fact{{"search_title", h.title}, {"search_snippet", h.snippet}, {"indexed_url", link}}})
		}
	}
	if len(r.Profiles) == 0 && r.Status == "SUCCESS" && !r.Incomplete {
		r.Status = "EMPTY"
	}
	return r
}
