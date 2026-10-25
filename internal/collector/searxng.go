package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type SearXNGOptions struct {
	Language   string
	Engines    []string
	TimeRange  string
	Pages      int
	SafeSearch int
}

func validSearchToken(s, allowed string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune(allowed, r) {
			continue
		}
		return false
	}
	return true
}

func (o SearXNGOptions) Validate() error {
	if o.Pages < 0 || o.Pages > 3 {
		return fmt.Errorf("search_pages must be between 0 and 3")
	}
	if o.SafeSearch < 0 || o.SafeSearch > 2 {
		return fmt.Errorf("searxng_safesearch must be 0, 1, or 2")
	}
	if o.Language != "" && !validSearchToken(o.Language, "-_") {
		return fmt.Errorf("search_language must be a language code such as id-ID or all")
	}
	switch o.TimeRange {
	case "", "day", "week", "month", "year":
	default:
		return fmt.Errorf("search_time_range must be day, week, month, or year")
	}
	for _, engine := range o.Engines {
		if !validSearchToken(engine, "_-.") {
			return fmt.Errorf("searxng_engines entries must be engine names")
		}
	}
	return nil
}

func (o SearXNGOptions) normalized() SearXNGOptions {
	if o.Pages < 1 {
		o.Pages = 1
	}
	if o.Pages > 3 {
		o.Pages = 3
	}
	if o.SafeSearch < 0 || o.SafeSearch > 2 {
		o.SafeSearch = 0
	}
	o.Language = strings.TrimSpace(o.Language)
	o.TimeRange = strings.TrimSpace(o.TimeRange)
	o.Engines = append([]string(nil), o.Engines...)
	return o
}

type SearXNG struct {
	client   *http.Client
	endpoint string
	opts     SearXNGOptions
}

// The trusted service URL comes from operator configuration, never a search result.
func NewSearXNG(raw string, timeout time.Duration) (*SearXNG, error) {
	return NewSearXNGWithOptions(raw, timeout, SearXNGOptions{})
}

func NewSearXNGWithOptions(raw string, timeout time.Duration, opts SearXNGOptions) (*SearXNG, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("searxng_url must be a service base URL without credentials, query, or fragment")
	}
	client := publicClient(timeout)
	local := u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"
	if local {
		if u.Scheme != "http" && u.Scheme != "https" {
			return nil, fmt.Errorf("SearXNG requires HTTP(S)")
		}
		// Only this fixed loopback service may use non-public addressing.
		client = &http.Client{Timeout: timeout, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	} else if _, err := cleanURL(raw); err != nil || u.Scheme != "https" {
		return nil, fmt.Errorf("remote SearXNG requires a public HTTPS URL on a standard port")
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/search"
	return &SearXNG{client: client, endpoint: u.String(), opts: opts.normalized()}, nil
}

func (s *SearXNG) Collect(ctx context.Context, q Query) Result {
	r := Result{Status: "FAILED", Incomplete: true}
	op := "searxng:" + q.Platform
	if q.Platform == "" {
		op = "searxng:web"
	}
	if q.Budget < 1 || q.Limit < 1 || q.Limit > 20 || searchWords(q.Name+" "+q.Username) == "" {
		r.Events = append(r.Events, Event{op, "FAILED", "Name or username and valid limits required"})
		return r
	}
	suffix := ""
	if q.Platform != "" && q.Platform != "all" {
		found := false
		for _, platform := range Platforms() {
			if platform.Name == q.Platform {
				found = true
				suffix = " (site:" + strings.Join(platform.Domains, " OR site:") + ")"
			}
		}
		if !found {
			r.Events = append(r.Events, Event{op, "FAILED", "Unknown platform"})
			return r
		}
	}
	phrases := searchPhrases(q)
	if len(phrases) == 0 {
		r.Events = append(r.Events, Event{op, "FAILED", "Name or username and valid limits required"})
		return r
	}
	pages := s.opts.Pages
	if pages < 1 {
		pages = 1
	}
	collectCap := q.Limit * 2
	if collectCap < 6 {
		collectCap = 6
	}
	seen := map[string]int{}
	parsedAny := false
	for index, phrase := range phrases {
		for page := 1; page <= pages; page++ {
			if ctx.Err() != nil || r.Requests >= q.Budget || len(r.Profiles) >= collectCap {
				break
			}
			values := url.Values{
				"q":          {phrase + suffix},
				"format":     {"json"},
				"categories": {"general"},
				"safesearch": {strconv.Itoa(s.opts.SafeSearch)},
			}
			if s.opts.Language != "" {
				values.Set("language", s.opts.Language)
			}
			if len(s.opts.Engines) > 0 {
				values.Set("engines", strings.Join(s.opts.Engines, ","))
			}
			if s.opts.TimeRange != "" {
				values.Set("time_range", s.opts.TimeRange)
			}
			if page > 1 {
				values.Set("pageno", strconv.Itoa(page))
			}
			raw := s.endpoint + "?" + values.Encode()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
			if err != nil {
				r.Events = append(r.Events, Event{op, "FAILED", "Invalid service URL"})
				return r
			}
			req.Header.Set("Accept", "application/json")
			response, err := s.client.Do(req)
			r.Requests++
			if err != nil {
				r.Events = append(r.Events, Event{op, "FAILED", "SearXNG service unavailable or timed out; start the configured service first. For the bundled local service run: powershell -File scripts/start-local-search.ps1"})
				return r
			}
			if response.StatusCode != 200 {
				if response.StatusCode == 401 || response.StatusCode == 403 || response.StatusCode == 429 {
					r.Status = "BLOCKED"
				}
				response.Body.Close()
				r.Events = append(r.Events, Event{op, r.Status, fmt.Sprintf("SearXNG HTTP %d; enable JSON output in search.formats, and check service access limits", response.StatusCode)})
				return r
			}
			body, err := io.ReadAll(io.LimitReader(response.Body, (2<<20)+1))
			response.Body.Close()
			var data struct {
				Results      []struct{ URL, Title, Content string }
				Unresponsive []json.RawMessage `json:"unresponsive_engines"`
			}
			if err != nil || len(body) > 2<<20 || json.Unmarshal(body, &data) != nil {
				r.Events = append(r.Events, Event{op, "FAILED", "Expected bounded SearXNG JSON response; enable JSON output"})
				return r
			}
			parsedAny = true
			added := 0
			for _, item := range data.Results {
				u, err := cleanURL(item.URL)
				if err != nil || !relevantSearchResult(q, item.URL, item.Title, item.Content) {
					continue
				}
				platform := platformFor(u.Hostname())
				if q.Platform == "all" && platform == "web" {
					continue
				}
				if q.Platform != "" && q.Platform != "all" && platform != q.Platform {
					continue
				}
				profile := indexedProfile(q, u.String(), raw, item.Title, item.Content)
				if at, exists := seen[u.String()]; exists {
					mergeProfileFacts(&r.Profiles[at], profile)
					continue
				}
				if len(r.Profiles) >= collectCap {
					break
				}
				seen[u.String()] = len(r.Profiles)
				r.Profiles = append(r.Profiles, profile)
				added++
			}
			r.Events = append(r.Events, Event{op, "SUCCESS", fmt.Sprintf("%d relevant links; %d upstream engines unavailable (phrase %d/%d, page %d). No matches do not establish account absence.", added, len(data.Unresponsive), index+1, len(phrases), page)})
		}
	}
	rankResults(r.Profiles, q)
	r.Profiles = r.Profiles[:min(q.Limit, len(r.Profiles))]
	switch {
	case len(r.Profiles) > 0:
		r.Status = "SUCCESS"
	case parsedAny:
		r.Status = "EMPTY"
	}
	return r
}

func NewConfiguredDiscovery(timeout time.Duration, endpoint string) (*Discovery, error) {
	return NewConfiguredDiscoveryWithOptions(timeout, endpoint, SearXNGOptions{})
}

func NewConfiguredDiscoveryWithOptions(timeout time.Duration, endpoint string, opts SearXNGOptions) (*Discovery, error) {
	d := NewDiscovery(timeout)
	if endpoint == "" {
		endpoint = "http://127.0.0.1:8888"
	}
	s, err := NewSearXNGWithOptions(endpoint, timeout, opts)
	if err != nil {
		return nil, err
	}
	d.providers[0] = s
	return d, nil
}
