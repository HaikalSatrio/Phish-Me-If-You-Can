package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

func nodeText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style") {
			return
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}
func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func blockedPage(doc *html.Node) bool {
	text := strings.ToLower(nodeText(doc))
	for _, s := range []string{"unusual traffic from your computer network", "verify you are human", "enable javascript to continue", "before you continue to google", "access denied"} {
		if strings.Contains(text, s) {
			return true
		}
	}
	blocked := false
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" && strings.Contains(attr(n, "href"), "/httpservice/retry/enablejs") {
			blocked = true
		}
		if n.Type == html.ElementNode && n.Data == "input" && strings.EqualFold(attr(n, "type"), "password") {
			blocked = true
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return blocked
}

type Page struct{ client *http.Client }

func NewPage(timeout time.Duration) *Page { return &Page{publicClient(timeout)} }
func (p *Page) Collect(ctx context.Context, q Query) Result {
	r := Result{Profiles: []Profile{}, Events: []Event{}, Status: "FAILED"}
	u, err := cleanURL(q.URL)
	if err != nil || q.Budget < 1 {
		r.Events = append(r.Events, Event{"url:input", "BLOCKED", "Valid public URL and request budget required"})
		r.Status = "BLOCKED"
		return r
	}
	b, status, detail, content := readPublic(ctx, p.client, u.String(), nil)
	r.Requests = 1
	r.Status = status
	if status != "SUCCESS" {
		r.Events = append(r.Events, Event{"url:page", status, detail})
		return r
	}
	if !strings.Contains(strings.ToLower(content), "text/html") {
		r.Status = "FAILED"
		r.Events = append(r.Events, Event{"url:page", "FAILED", "Only HTML metadata is supported"})
		return r
	}
	doc, err := html.Parse(bytes.NewReader(b))
	if err != nil {
		r.Status = "FAILED"
		r.Events = append(r.Events, Event{"url:page", "FAILED", "Invalid HTML"})
		return r
	}
	if blockedPage(doc) {
		r.Status = "BLOCKED"
		r.Events = append(r.Events, Event{"url:page", "BLOCKED", "Login, consent, or challenge page detected"})
		return r
	}
	title, description := "", ""
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "title" && title == "" {
			title = nodeText(n)
		}
		if n.Type == html.ElementNode && n.Data == "meta" {
			name := strings.ToLower(attr(n, "name"))
			property := strings.ToLower(attr(n, "property"))
			if description == "" && (name == "description" || property == "og:description") {
				description = attr(n, "content")
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if title == "" && description == "" {
		r.Status = "FAILED"
		r.Events = append(r.Events, Event{"url:page", "FAILED", "No extractable page metadata; content may require JavaScript"})
		return r
	}
	r.Profiles = append(r.Profiles, Profile{Key: keyFor("page", u.String()), Platform: platformFor(u.Hostname()), URL: u.String(), SourceURL: u.String(), Facts: []Fact{{"page_title", title}, {"page_description", description}}})
	r.Events = append(r.Events, Event{"url:page", "SUCCESS", "HTML title/description only; no personal attributes inferred"})
	return r
}

type Bluesky struct {
	client   *http.Client
	endpoint string
}

func NewBluesky(timeout time.Duration) *Bluesky {
	return &Bluesky{publicClient(timeout), "https://public.api.bsky.app/xrpc/app.bsky.actor.getProfile"}
}
func (b *Bluesky) Collect(ctx context.Context, q Query) Result {
	r := Result{Profiles: []Profile{}, Events: []Event{}, Status: "FAILED"}
	if q.Username == "" || len(q.Username) > 250 || q.Budget < 1 {
		r.Events = append(r.Events, Event{"bluesky:input", "FAILED", "Supply a Bluesky handle with -username"})
		return r
	}
	raw := b.endpoint + "?" + url.Values{"actor": {q.Username}}.Encode()
	body, status, detail, _ := readPublic(ctx, b.client, raw, nil)
	r.Requests = 1
	r.Status = status
	if status != "SUCCESS" {
		r.Events = append(r.Events, Event{"bluesky:profile", status, detail})
		return r
	}
	var p struct {
		DID         string `json:"did"`
		Handle      string `json:"handle"`
		DisplayName string `json:"displayName"`
		Description string `json:"description"`
	}
	if json.Unmarshal(body, &p) != nil || p.DID == "" || !strings.EqualFold(p.Handle, q.Username) {
		r.Status = "FAILED"
		r.Events = append(r.Events, Event{"bluesky:profile", "FAILED", "Invalid or mismatched public profile"})
		return r
	}
	r.Profiles = append(r.Profiles, Profile{Key: keyFor("bluesky", p.DID), Platform: "bluesky", URL: "https://bsky.app/profile/" + url.PathEscape(p.Handle), SourceURL: raw, Login: p.Handle, Name: p.DisplayName, Facts: []Fact{{"profile_description", p.Description}}})
	r.Events = append(r.Events, Event{"bluesky:profile", "SUCCESS", fmt.Sprintf("Public profile retrieved for %s", p.Handle)})
	return r
}
