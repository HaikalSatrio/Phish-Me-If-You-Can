package collector

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Wikipedia is a bounded exact-title fallback, not a general person directory.
// Redirects supply source assertions; all returned identities still require review.
type Wikipedia struct{ client *http.Client }

func NewWikipedia(timeout time.Duration) *Wikipedia { return &Wikipedia{publicClient(timeout)} }

func (w *Wikipedia) Collect(ctx context.Context, q Query) Result {
	r := Result{Status: "NOT_CHECKED", Incomplete: true}
	if strings.TrimSpace(q.Name) == "" || q.Username != "" || q.Platform != "" || q.Purpose != "" || q.Budget < 1 || q.Limit < 1 {
		return r
	}
	if strings.ContainsAny(q.Name, "|:#") {
		return r
	}
	r.Status = "EMPTY"
	values := url.Values{"action": {"query"}, "titles": {strings.TrimSpace(q.Name)}, "redirects": {"1"},
		"prop": {"extracts|info|pageprops"}, "exintro": {"1"}, "explaintext": {"1"}, "exchars": {"6000"},
		"inprop": {"url"}, "ppprop": {"disambiguation"}, "format": {"json"}, "formatversion": {"2"}, "maxlag": {"5"}}
	raw := "https://en.wikipedia.org/w/api.php?" + values.Encode()
	body, status, detail, _ := readPublic(ctx, w.client, raw, map[string]string{
		"Accept": "application/json", "User-Agent": "PhishMeIfYouCan/0.3 (https://github.com/HaikalSatrio/Phish-Me-If-You-Can)",
	})
	r.Requests = 1
	if status != "SUCCESS" {
		r.Status = status
		r.Events = []Event{{"wikipedia:title", status, detail}}
		return r
	}
	var data struct {
		Error json.RawMessage `json:"error"`
		Query struct {
			Pages []struct {
				NS        int                        `json:"ns"`
				Title     string                     `json:"title"`
				FullURL   string                     `json:"fullurl"`
				Extract   string                     `json:"extract"`
				Missing   bool                       `json:"missing"`
				PageProps map[string]json.RawMessage `json:"pageprops"`
			} `json:"pages"`
		} `json:"query"`
	}
	if json.Unmarshal(body, &data) != nil || len(data.Error) > 0 || data.Query.Pages == nil {
		r.Status = "FAILED"
		r.Events = []Event{{"wikipedia:title", "FAILED", "Reference API unavailable, lagged, or returned an invalid response; no retry in this collection"}}
		return r
	}
	for _, page := range data.Query.Pages {
		_, ambiguous := page.PageProps["disambiguation"]
		u, err := cleanURL(page.FullURL)
		if page.Missing || page.NS != 0 || ambiguous || page.Extract == "" || err != nil || u.Hostname() != "en.wikipedia.org" || !strings.HasPrefix(u.Path, "/wiki/") || wikiNavigation(u) {
			continue
		}
		if len([]rune(page.Extract)) > 6500 || len([]rune(page.Title)) > 200 {
			continue
		}
		p := Profile{Key: keyFor("web", u.String()), Platform: "web", URL: u.String(), SourceURL: raw,
			Rule: "wikipedia-title-v1", ResponseSHA256: fmt.Sprintf("%x", sha256.Sum256(body)),
			DiscoveryBasis: "exact article title or source redirect; not proof of a person or account ownership",
			Facts:          []Fact{{"page_title", page.Title + " - Wikipedia"}, {"page_description", page.Extract}}}
		r.Profiles = append(r.Profiles, p)
		if len(r.Profiles) >= q.Limit {
			break
		}
	}
	if len(r.Profiles) > 0 {
		r.Status = "SUCCESS"
	}
	r.Events = []Event{{"wikipedia:title", r.Status, "English Wikipedia exact-title lead; missing or ambiguous articles are not evidence that a person is absent"}}
	return r
}
