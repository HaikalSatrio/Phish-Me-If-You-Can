package collector

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"golang.org/x/net/html"
)

type Adapter struct {
	Name            string
	BaseURL         string
	UsernamePattern string
	Rule            string
}

func DirectAdapters() []Adapter {
	return []Adapter{
		{"github", "https://github.com/", `^[A-Za-z0-9][A-Za-z0-9-]{0,38}$`, "github-html-v2"},
		{"devto", "https://dev.to/", `^[A-Za-z0-9_]{1,30}$`, "devto-html-v2"},
	}
}

type Direct struct{ client *http.Client }

func NewDirect(timeout time.Duration) *Direct { return &Direct{client: publicClient(timeout)} }

func (d *Direct) Collect(ctx context.Context, q Query) Result {
	r := Result{Status: "SUCCESS", Profiles: []Profile{}, Events: []Event{}}
	if q.Username == "" || q.Budget < 1 || q.Limit < 1 {
		r.Status = "FAILED"
		r.Events = append(r.Events, Event{"direct:input", "FAILED", "Supply one known username and a valid request budget"})
		return r
	}
	selected := []Adapter{}
	for _, a := range DirectAdapters() {
		if q.Platform == "" || q.Platform == "all" || q.Platform == a.Name {
			selected = append(selected, a)
		}
	}
	if len(selected) == 0 {
		r.Status = "FAILED"
		r.Events = append(r.Events, Event{"direct:input", "FAILED", "No direct adapter for this platform; use adapters to list implemented rules"})
		return r
	}
	for _, a := range selected {
		raw := a.BaseURL + url.PathEscape(q.Username)
		receipt := domain.CollectionReceipt{Platform: a.Name, URL: raw, Rule: a.Rule, ObservedAt: time.Now().UTC().Format(time.RFC3339)}
		if !regexp.MustCompile(a.UsernamePattern).MatchString(q.Username) {
			receipt.Status, receipt.Reason = "NOT_CHECKED", "Username does not match this site's syntax"
		} else if ctx.Err() != nil || r.Requests >= q.Budget || len(r.Profiles) >= q.Limit {
			receipt.Status, receipt.Reason = "NOT_CHECKED", "Cancellation, request budget, or candidate limit reached"
		} else {
			response := fetchPublic(ctx, d.client, raw, q.Budget-r.Requests, func(next *url.URL) bool { return sameProfileURL(next.String(), raw) })
			body, status, content := response.Body, response.Status, response.Content
			r.Requests += response.Requests
			receipt.Status, receipt.Reason = status, response.Detail
			if status == "SUCCESS" {
				receipt.URL = response.URL
				receipt.Bytes = len(body)
				receipt.ResponseSHA256 = fmt.Sprintf("%x", sha256.Sum256(body))
				if !strings.Contains(strings.ToLower(content), "text/html") {
					receipt.Status, receipt.Reason = "FAILED", "Expected public HTML profile"
				} else {
					p, status, reason := parseDirect(body, a, q.Username, response.URL)
					receipt.Status, receipt.Reason = status, reason
					if status == "SUCCESS" {
						p.ResponseSHA256, p.Rule = receipt.ResponseSHA256, a.Rule
						r.Profiles = append(r.Profiles, p)
					}
				}
			} else if status == "EMPTY" {
				receipt.Reason = "HTTP 404 at the requested profile URL; this does not establish absence on other sites"
			}
		}
		r.Receipts = append(r.Receipts, receipt)
		r.Events = append(r.Events, Event{"direct:" + a.Name, receipt.Status, receipt.Reason})
		if receipt.Status != "SUCCESS" && receipt.Status != "EMPTY" {
			r.Incomplete = true
			if receipt.Status == "BLOCKED" {
				r.Status = "BLOCKED"
			} else if r.Status != "BLOCKED" {
				r.Status = "FAILED"
			}
		}
	}
	if len(r.Profiles) == 0 && r.Status == "SUCCESS" {
		r.Status = "EMPTY"
	}
	return r
}

func sameProfileURL(raw, expected string) bool {
	u, err := cleanURL(raw)
	v, wantErr := cleanURL(expected)
	return err == nil && wantErr == nil && strings.EqualFold(u.Host, v.Host) && strings.EqualFold(strings.TrimRight(u.Path, "/"), strings.TrimRight(v.Path, "/")) && u.RawQuery == ""
}

func hasClass(n *html.Node, want string) bool {
	for _, c := range strings.Fields(attr(n, "class")) {
		if c == want {
			return true
		}
	}
	return false
}

func parseDirect(body []byte, a Adapter, username, raw string) (Profile, string, string) {
	p := Profile{Key: keyFor(a.Name, strings.ToLower(raw)), Platform: a.Name, URL: raw, SourceURL: raw}
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return p, "FAILED", "Unable to parse HTML"
	}
	if blockedPage(doc) {
		return p, "BLOCKED", "Login or challenge detected; no account conclusion"
	}
	meta := map[string]string{}
	canonical, nickname, displayName, heading, bio := "", "", "", "", ""
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if n.Data == "meta" {
				meta[attr(n, "property")] = attr(n, "content")
			}
			if n.Data == "link" && attr(n, "rel") == "canonical" {
				canonical = attr(n, "href")
			}
			if a.Name == "github" {
				if attr(n, "itemprop") == "additionalName" && hasClass(n, "p-nickname") {
					nickname = nodeText(n)
				}
				if attr(n, "itemprop") == "name" && hasClass(n, "p-name") {
					displayName = nodeText(n)
				}
				if hasClass(n, "p-note") {
					bio = nodeText(n)
				}
			}
			if a.Name == "devto" && n.Data == "h1" && hasClass(n, "crayons-title") {
				heading = nodeText(n)
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	if (canonical == "" && meta["og:url"] == "") || (canonical != "" && !sameProfileURL(canonical, raw)) || (meta["og:url"] != "" && !sameProfileURL(meta["og:url"], raw)) {
		return p, "FAILED", "Expected matching profile URL metadata; missing or conflicting URLs require review"
	}
	switch a.Name {
	case "github":
		if meta["og:type"] != "profile" || !strings.EqualFold(nickname, username) {
			return p, "FAILED", "Expected profile type and exact visible username were not both found"
		}
		p.Login, p.Name = nickname, displayName
	case "devto":
		if heading == "" || meta["og:site_name"] != "DEV Community" || !strings.HasSuffix(meta["og:title"], "DEV Community Profile") {
			return p, "FAILED", "Expected DEV profile heading and profile metadata were not both found"
		}
		p.Login, p.Name, bio = username, heading, meta["og:description"]
	default:
		return p, "FAILED", "Unsupported extraction rule"
	}
	p.Facts = []Fact{{"page_title", meta["og:title"]}, {"profile_description", bio}}
	return p, "SUCCESS", "Matching profile URL and site-specific profile markers observed; person ownership still requires review"
}
