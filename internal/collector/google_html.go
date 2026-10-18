package collector

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type GoogleHTML struct {
	browser  string
	render   bool
	timeout  time.Duration
	client   *http.Client
	endpoint string
}

func NewGoogleHTML(timeout time.Duration) *GoogleHTML {
	return &GoogleHTML{client: publicClient(timeout), endpoint: "https://www.google.com/search", timeout: timeout}
}

func NewGoogleBrowser(timeout time.Duration) *GoogleHTML {
	g := NewGoogleHTML(timeout)
	g.render = true
	g.browser = FindBrowser()
	return g
}

func (g *GoogleHTML) Collect(ctx context.Context, q Query) Result {
	r := Result{Status: "SUCCESS", Profiles: []Profile{}, Events: []Event{}}
	term := q.Name
	if term == "" {
		term = q.Username
	}
	if strings.TrimSpace(term) == "" || q.Limit < 1 || q.Limit > 20 || q.Budget < 1 {
		r.Status = "FAILED"
		r.Events = append(r.Events, Event{"google:input", "FAILED", "Name or username and valid budget required"})
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
			r.Events = append(r.Events, Event{"google:input", "FAILED", "Unknown platform"})
			return r
		}
	}
	seen := map[string]bool{}
	blocked := false
	for _, p := range platforms {
		op := "google:" + p.Name
		if blocked || r.Requests >= q.Budget || ctx.Err() != nil {
			r.Incomplete = true
			r.Events = append(r.Events, Event{op, "NOT_CHECKED", "Stopped after access restriction, cancellation, or budget limit"})
			continue
		}
		if r.Requests > 0 {
			timer := time.NewTimer(2 * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				r.Incomplete = true
				r.Events = append(r.Events, Event{op, "NOT_CHECKED", "Cancelled"})
				continue
			case <-timer.C:
			}
		}
		query := searchPhrase(q)
		if len(p.Domains) > 0 {
			sites := []string{}
			for _, d := range p.Domains {
				sites = append(sites, "site:"+d)
			}
			query += " (" + strings.Join(sites, " OR ") + ")"
		}
		raw := g.endpoint + "?" + url.Values{"q": {query}, "num": {strconv.Itoa(q.Limit)}, "hl": {"en"}}.Encode()
		var body []byte
		var status, detail string
		if g.render {
			body, status, detail = renderGoogle(ctx, g.browser, raw, g.timeout)
		} else {
			body, status, detail, _ = readPublic(ctx, g.client, raw, nil)
		}
		r.Requests++
		if status != "SUCCESS" {
			r.Status = status
			r.Incomplete = true
			r.Events = append(r.Events, Event{op, status, detail})
			if status == "BLOCKED" {
				blocked = true
			}
			continue
		}
		profiles, parseStatus := parseGoogle(body, raw, q.Limit, p.Name)
		if parseStatus != "SUCCESS" {
			r.Status = parseStatus
			r.Incomplete = true
			blocked = true
			r.Events = append(r.Events, Event{op, parseStatus, "Google returned a challenge or unrecognized HTML; no empty-exposure conclusion"})
			continue
		}
		r.Events = append(r.Events, Event{op, "SUCCESS", "First-page indexed links only; destination pages not fetched"})
		// A single HTML page cannot establish exhaustive index coverage.
		r.Incomplete = true
		for _, profile := range profiles {
			if !seen[profile.URL] {
				seen[profile.URL] = true
				r.Profiles = append(r.Profiles, profile)
			}
		}
	}
	return r
}

func parseGoogle(body []byte, source string, limit int, platform string) ([]Profile, string) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, "FAILED"
	}
	if blockedPage(doc) {
		return nil, "BLOCKED"
	}
	profiles := []Profile{}
	seen := map[string]bool{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if len(profiles) >= limit {
			return
		}
		if n.Type == html.ElementNode && n.Data == "a" {
			title := ""
			var heading func(*html.Node)
			heading = func(h *html.Node) {
				if h.Type == html.ElementNode && h.Data == "h3" {
					title = nodeText(h)
				}
				for c := h.FirstChild; c != nil; c = c.NextSibling {
					heading(c)
				}
			}
			heading(n)
			if title != "" {
				link := attr(n, "href")
				if strings.HasPrefix(link, "/url?") {
					if u, e := url.Parse(link); e == nil {
						link = u.Query().Get("q")
						if link == "" {
							link = u.Query().Get("url")
						}
					}
				}
				u, e := cleanURL(link)
				if e == nil && !strings.HasSuffix(u.Hostname(), "google.com") && !seen[u.String()] && (platform == "web" || platformFor(u.Hostname()) == platform) {
					seen[u.String()] = true
					profiles = append(profiles, Profile{Key: keyFor("web", u.String()), Platform: platformFor(u.Hostname()), URL: u.String(), SourceURL: source, Facts: []Fact{{"search_title", title}, {"indexed_url", u.String()}}})
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if len(profiles) == 0 {
		return nil, "FAILED"
	}
	return profiles, "SUCCESS"
}
