// Anonymous read-only subset adapted from Instaloader 4.16 (MIT).
// See third_party/instaloader-LICENSE. Not an official Instagram API or a full port.
package instagram

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/norm"
)

const userAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36"
const maxBody = 8 << 20

type Profile struct {
	WebID     string `json:"web_id,omitempty"`
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	FullName  string `json:"full_name"`
	Biography string `json:"biography"`
	IsPrivate bool   `json:"is_private"`
	PostCount *int64 `json:"post_count"`
}
type Post struct {
	SourceKind        string   `json:"source_kind,omitempty"`
	PublisherIDType   string   `json:"publisher_id_type,omitempty"`
	TimestampStatus   string   `json:"timestamp_status,omitempty"`
	PublisherUsername string   `json:"publisher_username"`
	PublisherID       string   `json:"publisher_id"`
	Receipt           Receipt  `json:"-"`
	URL               string   `json:"url"`
	PublishedAt       string   `json:"published_at"`
	Caption           *string  `json:"caption"`
	Mentions          []string `json:"caption_mentions"`
	Hashtags          []string `json:"caption_hashtags"`
	ClaimStatus       string   `json:"claim_status"`
}
type Failure struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
type Receipt struct {
	Protocol   string  `json:"http_protocol,omitempty"`
	ObservedAt string  `json:"observed_at"`
	URL        string  `json:"url"`
	Method     string  `json:"method"`
	Status     int     `json:"http_status"`
	Bytes      int     `json:"bytes"`
	SHA256     string  `json:"response_sha256,omitempty"`
	RetryAfter string  `json:"retry_after,omitempty"`
	Duration   float64 `json:"seconds"`
}
type Report struct {
	CollectedPosts  int       `json:"collected_posts"`
	Tool            string    `json:"tool"`
	Version         string    `json:"version"`
	Username        string    `json:"username"`
	AccessMode      string    `json:"access_mode"`
	StartedAt       string    `json:"started_at"`
	FinishedAt      string    `json:"finished_at"`
	Elapsed         float64   `json:"elapsed_seconds"`
	MaxPosts        int       `json:"max_posts"`
	ProfileStatus   string    `json:"profile_status"`
	PostsStatus     string    `json:"posts_status"`
	StopReason      string    `json:"stop_reason"`
	Scope           string    `json:"scope"`
	MediaDownloaded bool      `json:"media_downloaded"`
	Profile         *Profile  `json:"profile,omitempty"`
	Posts           []Post    `json:"posts"`
	Requests        []Receipt `json:"requests"`
	Error           *Failure  `json:"error,omitempty"`
}
type Client struct {
	do       func(*http.Request) (*http.Response, error)
	http     *http.Client
	base     string
	interval time.Duration
	last     time.Time
	requests []Receipt
	tokens   url.Values
}

// New uses the caller's guarded transport and budget/cooldown implementation.
// Each client belongs to one collection; only anonymous cookies are created.
func New(client *http.Client, do func(*http.Request) (*http.Response, error)) *Client {
	c := newClient()
	c.http = client
	c.do = do
	return c
}

func newClient() *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		http: &http.Client{Timeout: 20 * time.Second, Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		base: "https://www.instagram.com", interval: 3 * time.Second, requests: []Receipt{},
	}
}

// No automatic retries, redirects, credentials, cookie imports, or TLS overrides.
func (c *Client) request(ctx context.Context, method, path string, form url.Values) ([]byte, error) {
	if len(c.requests) >= 20 {
		return nil, fmt.Errorf("request budget exceeded")
	}
	if wait := time.Until(c.last.Add(c.interval)); wait > 0 {
		t := time.NewTimer(wait)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-t.C:
		}
	}
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept-Language", "en-US,en;q=0.8")
	req.Header.Set("Referer", c.base+"/")
	req.Header.Set("X-IG-App-ID", "936619743392459")
	if method == http.MethodGet {
		req.Header.Set("Accept", "text/html")
		req.Header.Set("Sec-Fetch-Dest", "document")
		req.Header.Set("Sec-Fetch-Mode", "navigate")
		req.Header.Set("Sec-Fetch-Site", "none")
	} else {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "*/*")
		req.Header.Set("Sec-Fetch-Dest", "empty")
		req.Header.Set("Sec-Fetch-Mode", "cors")
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		req.Header.Set("X-FB-LSD", form.Get("lsd"))
		for _, cookie := range c.http.Jar.Cookies(req.URL) {
			if cookie.Name == "csrftoken" {
				req.Header.Set("X-CSRFToken", cookie.Value)
			}
		}
	}
	c.last = time.Now()
	receipt := Receipt{URL: req.URL.String(), Method: method, ObservedAt: c.last.UTC().Format(time.RFC3339Nano)}
	defer func() { receipt.Duration = time.Since(c.last).Seconds(); c.requests = append(c.requests, receipt) }()
	do := c.do
	if do == nil {
		do = c.http.Do
	}
	resp, err := do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	receipt.Protocol = resp.Proto
	receipt.Status, receipt.RetryAfter = resp.StatusCode, resp.Header.Get("Retry-After")
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d; stopped without retry (Retry-After=%q)", resp.StatusCode, receipt.RetryAfter)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, err
	}
	receipt.Bytes = len(data)
	receipt.ObservedAt = time.Now().UTC().Format(time.RFC3339Nano)
	receipt.SHA256 = fmt.Sprintf("%x", sha256.Sum256(data))
	if len(data) > maxBody {
		return nil, fmt.Errorf("response exceeds 8 MiB limit")
	}
	return data, nil
}

var scriptPattern = regexp.MustCompile(`(?is)<script\b([^>]*)>(.*?)</script\s*>`)
var jsonType = regexp.MustCompile(`(?i)\btype\s*=\s*["']application/json["']`)

// Decode in source order: Go's map iteration order cannot define query precedence.
func orderedValue(dec *json.Decoder, queries *[]map[string]any, depth int) (any, error) {
	if depth > 200 {
		return nil, fmt.Errorf("JSON nesting limit exceeded")
	}
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if delim, ok := tok.(json.Delim); ok {
		switch delim {
		case '{':
			obj := map[string]any{}
			for dec.More() {
				key, err := dec.Token()
				if err != nil {
					return nil, err
				}
				v, err := orderedValue(dec, queries, depth+1)
				if err != nil {
					return nil, err
				}
				obj[key.(string)] = v
			}
			if _, err = dec.Token(); err != nil {
				return nil, err
			}
			if data := object(object(object(obj["__bbox"])["result"])["data"]); data != nil {
				*queries = append(*queries, data)
			}
			return obj, nil
		case '[':
			items := []any{}
			for dec.More() {
				v, err := orderedValue(dec, queries, depth+1)
				if err != nil {
					return nil, err
				}
				items = append(items, v)
			}
			_, err = dec.Token()
			return items, err
		default:
			return nil, fmt.Errorf("unexpected JSON delimiter")
		}
	}
	return tok, nil
}

func pageQueries(html []byte) []map[string]any {
	result := []map[string]any{}
	for _, match := range scriptPattern.FindAllSubmatch(html, -1) {
		if !jsonType.Match(match[1]) {
			continue
		}
		dec := json.NewDecoder(strings.NewReader(string(match[2])))
		dec.UseNumber()
		found := []map[string]any{}
		if _, err := orderedValue(dec, &found, 0); err == nil {
			if _, err = dec.Token(); err == io.EOF {
				result = append(result, found...)
			}
		}
	}
	return result
}
func object(v any) map[string]any { m, _ := v.(map[string]any); return m }
func stringValue(v any) string    { s, _ := v.(string); return s }
func array(v any) []any           { a, _ := v.([]any); return a }
func integer(v any) *int64 {
	n, ok := v.(json.Number)
	if !ok {
		return nil
	}
	i, err := n.Int64()
	if err != nil {
		return nil
	}
	return &i
}

func parseProfile(html []byte, username string) (*Profile, map[string]any, string, error) {
	user := map[string]any{}
	for _, data := range pageQueries(html) {
		for k, v := range object(data["xig_user_by_username"]) {
			user[k] = v
		}
	}
	if user["pk"] == nil || !strings.EqualFold(stringValue(user["username"]), username) {
		return nil, nil, "", fmt.Errorf("matching profile data missing; login, challenge, unavailable profile or schema change possible")
	}
	private, ok := user["is_private"].(bool)
	if !ok {
		return nil, nil, "", fmt.Errorf("profile privacy field missing; refusing post lookup")
	}
	count, found := user["media_count"]
	if !found {
		count = user["all_media_count"]
	}
	p := &Profile{UserID: scalarID(user["pk"]), WebID: scalarID(user["id"]), Username: stringValue(user["username"]), FullName: stringValue(user["full_name"]), Biography: stringValue(user["biography"]), IsPrivate: private, PostCount: integer(count)}
	return p, object(user["polaris_ordered_timeline_connection"]), p.WebID, nil
}

func (c *Client) graphql(ctx context.Context, docID string, variables map[string]any) (map[string]any, error) {
	if c.tokens == nil {
		page, err := c.request(ctx, "GET", "/", nil)
		if err != nil {
			return nil, err
		}
		c.tokens = url.Values{}
		for _, pair := range [][2]string{{"lsd", `"LSD",\[\],\{[^{}]*"token":"([^"]+)"`}, {"av", `"IG_USER_EIMU":"?(\d+)`}, {"fb_dtsg", `"DTSGInitialData",\[\],\{[^{}]*"token":"([^"]+)"`}} {
			match := regexp.MustCompile(pair[1]).FindSubmatch(page)
			if len(match) > 1 {
				c.tokens.Set(pair[0], string(match[1]))
			} else if pair[0] != "fb_dtsg" {
				return nil, fmt.Errorf("anonymous page token missing; cannot request post metadata")
			}
		}
	}
	form := url.Values{}
	for k, v := range c.tokens {
		form[k] = v
	}
	encoded, err := json.Marshal(variables)
	if err != nil {
		return nil, err
	}
	form.Set("variables", string(encoded))
	form.Set("doc_id", docID)
	form.Set("server_timestamps", "true")
	body, err := c.request(ctx, "POST", "/api/graphql", form)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.UseNumber()
	var data map[string]any
	if err = dec.Decode(&data); err != nil {
		return nil, fmt.Errorf("post response is not JSON: %w", err)
	}
	if len(array(data["errors"])) > 0 || (data["status"] != nil && data["status"] != "ok") {
		return nil, fmt.Errorf("upstream GraphQL error; no retry")
	}
	return object(data["data"]), nil
}

var shortcodePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,30}$`)
var UsernamePattern = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.]{0,29}$`)

func scalarID(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	default:
		return ""
	}
}

func parsePost(data map[string]any, code string) (Post, error) {
	items := array(object(data["xdt_api__v1__media__shortcode__web_info"])["items"])
	if len(items) != 1 {
		return Post{}, fmt.Errorf("post metadata missing or ambiguous")
	}
	return parsePostNode(object(items[0]), code, false)
}

// Timeline nodes often already contain the full caption and publisher. Missing
// publication time is explicitly unknown; never substitute the collection time.
func parsePostNode(node map[string]any, code string, timeline bool) (Post, error) {
	p := Post{Mentions: []string{}, Hashtags: []string{}, ClaimStatus: "UNVERIFIED", SourceKind: "detail_api", PublisherIDType: "user_id", TimestampStatus: "KNOWN"}
	p.PublisherUsername = stringValue(object(node["user"])["username"])
	p.PublisherID = scalarID(object(node["user"])["pk"])
	if timeline {
		p.SourceKind = "timeline"
		if p.PublisherID == "" {
			p.PublisherID = scalarID(object(node["user"])["id"])
			p.PublisherIDType = "web_id"
		}
		if p.PublisherID == "" || p.PublisherUsername == "" {
			return p, fmt.Errorf("timeline publisher missing")
		}
	}
	if stringValue(node["code"]) != code {
		return p, fmt.Errorf("returned post shortcode does not match request")
	}
	taken := integer(node["taken_at"])
	if taken == nil && timeline && node["taken_at"] == nil {
		p.TimestampStatus = "UNAVAILABLE"
	} else if taken == nil || *taken <= 0 {
		return p, fmt.Errorf("post timestamp missing or invalid")
	} else {
		p.PublishedAt = time.Unix(*taken, 0).UTC().Format(time.RFC3339)
	}
	p.URL = "https://www.instagram.com/p/" + code + "/"
	caption, exists := node["caption"]
	if !exists {
		return p, fmt.Errorf("caption field missing; unknown is not an empty caption")
	}
	if caption != nil {
		text, ok := object(caption)["text"].(string)
		if !ok {
			return p, fmt.Errorf("unexpected caption structure")
		}
		text = norm.NFC.String(text)
		p.Caption = &text
		p.Mentions, p.Hashtags = captionTags(text)
	}
	return p, nil
}

func asciiWord(r rune) bool                        { return r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' }
func CaptionTags(text string) ([]string, []string) { return captionTags(text) }
func captionTags(text string) ([]string, []string) {
	mentions, hashtags := []string{}, []string{}
	r := []rune(cases.Lower(language.Und).String(text))
	for i := 0; i < len(r); i++ {
		if r[i] == '#' {
			j := i + 1
			for j < len(r) && j-i <= 150 && (unicode.IsLetter(r[j]) || unicode.IsNumber(r[j]) || r[j] == '_') {
				j++
			}
			if j > i+1 {
				hashtags = append(hashtags, string(r[i+1:j]))
			}
		}
		if r[i] != '@' || i > 0 && asciiWord(r[i-1]) && r[i-1] != '_' {
			continue
		}
		j := i + 1
		if j >= len(r) || !asciiWord(r[j]) {
			continue
		}
		j++
		for j < len(r) && j-i <= 30 {
			if asciiWord(r[j]) {
				j++
				continue
			}
			if r[j] == '.' && j+1 < len(r) && r[j+1] != '.' {
				j++
				continue
			}
			break
		}
		for j > i+1 && r[j-1] == '.' {
			j--
		}
		mentions = append(mentions, string(r[i+1:j]))
		i = j - 1
	}
	return mentions, hashtags
}

func (c *Client) Collect(ctx context.Context, username string, limit int) (r Report) {
	start := time.Now()
	r = Report{Tool: "pmiyc-instagram-go", Version: "1-instaloader4.16-subset", Username: username, AccessMode: "anonymous", StartedAt: start.UTC().Format(time.RFC3339Nano), MaxPosts: limit, ProfileStatus: "ATTEMPTING", PostsStatus: "NOT_ATTEMPTED", Posts: []Post{}, Scope: "bounded public sample; source order; completeness not verified"}
	defer func() {
		r.CollectedPosts = len(r.Posts)
		r.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
		r.Elapsed = time.Since(start).Seconds()
		r.Requests = c.requests
	}()
	stage := "profile"
	fail := func(err error) {
		if stage == "profile" {
			r.ProfileStatus = "FAILED"
		} else {
			r.PostsStatus = "FAILED"
			if len(r.Posts) > 0 {
				r.PostsStatus = "PARTIAL"
			}
		}
		r.StopReason = "error"
		r.Error = &Failure{Type: "CollectionError", Message: err.Error()}
	}
	if !UsernamePattern.MatchString(username) || limit < 1 || limit > 12 {
		fail(fmt.Errorf("invalid username or post limit (1..12)"))
		return
	}
	body, err := c.request(ctx, "GET", "/"+username+"/", nil)
	if err != nil {
		fail(err)
		return
	}
	profile, timeline, pageID, err := parseProfile(body, username)
	if err != nil {
		fail(err)
		return
	}
	r.Profile, r.ProfileStatus = profile, "SUCCESS"
	timelineReceipt := c.requests[len(c.requests)-1]
	if profile.IsPrivate {
		r.PostsStatus, r.StopReason = "PRIVATE_PROFILE_NOT_REQUESTED", "private_profile"
		return
	}
	stage, r.PostsStatus = "posts", "ATTEMPTING"
	if timeline == nil {
		fail(fmt.Errorf("public profile has no embedded timeline; not evidence of zero posts"))
		return
	}
	seen, cursors := map[string]bool{}, map[string]bool{}
	for {
		edges, ok := timeline["edges"].([]any)
		if !ok {
			fail(fmt.Errorf("timeline edges missing"))
			return
		}
		for _, edge := range edges {
			node := object(object(edge)["node"])
			code := stringValue(node["code"])
			if !shortcodePattern.MatchString(code) {
				fail(fmt.Errorf("invalid post shortcode"))
				return
			}
			if seen[code] {
				continue
			}
			seen[code] = true
			post, err := parsePostNode(node, code, true)
			if err == nil {
				post.Receipt = timelineReceipt
			} else {
				data, err := c.graphql(ctx, "27128499623469141", map[string]any{"shortcode": code, "__relay_internal__pv__PolarisAIGMMediaWebLabelEnabledrelayprovider": false})
				if err != nil {
					fail(err)
					return
				}
				post, err = parsePost(data, code)
				if err != nil {
					fail(err)
					return
				}
				post.Receipt = c.requests[len(c.requests)-1]
			}
			expectedID := profile.UserID
			if post.PublisherIDType == "web_id" {
				expectedID = profile.WebID
			}
			if post.PublisherID == "" || expectedID == "" || post.PublisherID != expectedID || !strings.EqualFold(post.PublisherUsername, profile.Username) {
				fail(fmt.Errorf("post publisher missing or different from profile; account binding not established"))
				return
			}
			r.Posts = append(r.Posts, post)
			if len(r.Posts) >= limit {
				r.PostsStatus, r.StopReason = "SUCCESS", "sample_limit"
				return
			}
		}
		pageInfo := object(timeline["page_info"])
		hasNext, known := pageInfo["has_next_page"].(bool)
		if !known {
			fail(fmt.Errorf("pagination state missing; completeness unknown"))
			return
		}
		if !hasNext {
			r.PostsStatus, r.StopReason = "SUCCESS", "source_end"
			if len(r.Posts) == 0 {
				r.PostsStatus = "EMPTY"
			}
			return
		}
		cursor := stringValue(pageInfo["end_cursor"])
		if cursor == "" || cursors[cursor] || pageID == "" {
			fail(fmt.Errorf("missing or repeated pagination cursor"))
			return
		}
		cursors[cursor] = true
		data, err := c.graphql(ctx, "28904005159232689", map[string]any{"id": pageID, "__relay_internal__pv__PolarisSEOIsCrawlerrelayprovider": false, "after": cursor, "before": nil, "first": 12, "last": nil})
		if err != nil {
			fail(err)
			return
		}
		timeline = object(object(data["node"])["polaris_ordered_timeline_connection"])
		timelineReceipt = c.requests[len(c.requests)-1]
	}
}
