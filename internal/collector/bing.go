package collector

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
)

// Bing reads the public search RSS feed without an API key or browser session.
type Bing struct {
	client   *http.Client
	endpoint string
}

func NewBing(timeout time.Duration) *Bing {
	return &Bing{client: publicClient(timeout), endpoint: "https://www.bing.com/search"}
}

type searchFeed struct {
	XMLName xml.Name `xml:"rss"`
	Channel struct {
		Title string `xml:"title"`
		Items []struct {
			Title       string `xml:"title"`
			Link        string `xml:"link"`
			Description string `xml:"description"`
		} `xml:"item"`
	} `xml:"channel"`
}

func searchWords(s string) string {
	return strings.Join(strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	}), " ")
}

// This checks textual relevance only, never ownership of a result.
func relevantResult(q Query, text string) bool {
	if username := strings.TrimPrefix(strings.TrimSpace(q.Username), "@"); username != "" {
		return regexp.MustCompile(`(?i)(?:^|[^\p{L}\p{N}_.-])` + regexp.QuoteMeta(username) + `(?:$|[^\p{L}\p{N}_.-])`).MatchString(text)
	}
	haystack := " " + searchWords(text) + " "
	for _, clue := range []string{q.Name, q.Username} {
		words := strings.Fields(searchWords(clue))
		matched := len(words) > 0
		for _, word := range words {
			if !strings.Contains(haystack, " "+word+" ") {
				matched = false
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func discoveryPhrase(q Query) string {
	if q.Purpose == "biography" {
		phrases := searchPhrases(q)
		if len(phrases) > 0 {
			return phrases[0]
		}
	}
	if username := strings.TrimPrefix(strings.TrimSpace(q.Username), "@"); username != "" {
		return searchPhrase(Query{Username: username})
	}
	return searchPhrase(Query{Name: q.Name, Username: q.Username})
}

func rankResults(profiles []Profile, q Query) {
	score := func(p Profile) int {
		text := p.URL
		for _, f := range p.Facts {
			text += " " + f.Value
		}
		haystack := " " + searchWords(text) + " "
		n := 0
		if q.Platform == "" && q.Username == "" && len(q.Accounts) == 0 && hasPublicName(q.Name, p) {
			n += 8
		}
		clues := append([]domain.AccountClue(nil), q.Accounts...)
		if q.Username != "" {
			clues = append(clues, domain.AccountClue{Platform: q.Platform, Username: q.Username})
		}
		for _, clue := range clues {
			username := strings.TrimPrefix(strings.TrimSpace(clue.Username), "@")
			if u, err := url.Parse(p.URL); err == nil {
				if username == "" || clue.Platform != "" && clue.Platform != "all" && platformFor(u.Hostname()) != clue.Platform {
					continue
				}
				parts := strings.Split(strings.Trim(u.Path, "/"), "/")
				for _, part := range parts {
					if strings.EqualFold(strings.TrimPrefix(part, "@"), username) {
						n += 4
						break
					}
				}
			}
		}
		for _, clue := range []string{q.Company, q.Location} {
			if words := searchWords(clue); words != "" && strings.Contains(haystack, " "+words+" ") {
				n++
			}
		}
		return n
	}
	sort.SliceStable(profiles, func(i, j int) bool { return score(profiles[i]) > score(profiles[j]) })
}

func (b *Bing) Collect(ctx context.Context, q Query) Result {
	r := Result{Profiles: []Profile{}, Events: []Event{}, Status: "SUCCESS", Incomplete: true}
	if searchWords(q.Name+" "+q.Username) == "" || q.Limit < 1 || q.Limit > 20 || q.Budget < 1 {
		r.Status = "FAILED"
		r.Events = append(r.Events, Event{"bing:input", "FAILED", "Name or username and valid budget required"})
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
			r.Events = append(r.Events, Event{"bing:input", "FAILED", "Unknown platform"})
			return r
		}
	}
	seen := map[string]bool{}
	stopped := false
	for _, p := range platforms {
		op := "bing:" + p.Name
		if stopped || ctx.Err() != nil || r.Requests >= q.Budget || len(r.Profiles) >= q.Limit {
			r.Events = append(r.Events, Event{op, "NOT_CHECKED", "Stopped after restriction, cancellation, or request budget"})
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
		raw := b.endpoint + "?" + url.Values{"q": {query}, "format": {"rss"}}.Encode()
		body, status, detail, _ := readPublic(ctx, b.client, raw, nil)
		r.Requests++
		if status != "SUCCESS" {
			r.Status = status
			r.Events = append(r.Events, Event{op, status, detail})
			stopped = status == "BLOCKED"
			continue
		}
		var feed searchFeed
		if xml.Unmarshal(body, &feed) != nil || feed.Channel.Title == "" {
			r.Status = "FAILED"
			r.Events = append(r.Events, Event{op, "FAILED", "Expected search RSS; received unsupported content"})
			stopped = true
			continue
		}
		accepted, rejected := 0, 0
		matches := []Profile{}
		invalidURLs, wrongDomains, unrelated := 0, 0, 0
		for _, item := range feed.Channel.Items {
			u, err := cleanURL(item.Link)
			if err != nil {
				invalidURLs++
				rejected++
				continue
			}
			if p.Name != "web" && platformFor(u.Hostname()) != p.Name {
				wrongDomains++
				rejected++
				continue
			}
			if !relevantSearchResult(q, item.Link, item.Title, item.Description) {
				unrelated++
				rejected++
				continue
			}
			if seen[u.String()] {
				continue
			}
			accepted++
			seen[u.String()] = true
			matches = append(matches, indexedProfile(q, u.String(), raw, item.Title, item.Description))
		}
		rankResults(matches, q)
		matches = matches[:min(len(matches), q.Limit-len(r.Profiles))]
		r.Profiles = append(r.Profiles, matches...)
		accepted = len(matches)
		if accepted == 0 {
			r.Status = "FAILED"
			r.Events = append(r.Events, Event{op, "FAILED", fmt.Sprintf("No usable matching links: %d invalid URLs, %d outside the requested platform, %d without the supplied name or username. The feed is insufficient for this query; this does not establish that the account or public data is absent.", invalidURLs, wrongDomains, unrelated)})
		} else {
			r.Events = append(r.Events, Event{op, "SUCCESS", fmt.Sprintf("%d indexed links; %d unrelated or invalid entries rejected. Ownership requires review; destination pages not fetched.", accepted, rejected)})
		}
	}
	if len(r.Profiles) == 0 && r.Status == "SUCCESS" {
		r.Status = "FAILED"
	}
	return r
}
