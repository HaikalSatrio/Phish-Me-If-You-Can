package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"

	ig "github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/instagram"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/profile"
)

type InstagramOptions struct {
	MaxPosts int `json:"max_posts"`
}

func (o InstagramOptions) Validate() error {
	if o.MaxPosts < 1 || o.MaxPosts > 12 {
		return fmt.Errorf("instagram.max_posts must be 1..12")
	}
	return nil
}

type Instagram struct {
	client  *http.Client
	options InstagramOptions
}

func NewInstagram(timeout time.Duration, options *InstagramOptions) *Instagram {
	o := InstagramOptions{MaxPosts: 3}
	if options != nil {
		o = *options
	}
	if timeout <= 0 || timeout > 20*time.Second {
		timeout = 20 * time.Second
	}
	client := publicClient(timeout)
	// Match the prototype's standard HTTP/2 negotiation while retaining the
	// public-address dial guard. A custom DialContext otherwise disables it.
	client.Transport.(*http.Transport).ForceAttemptHTTP2 = true
	client.Jar, _ = cookiejar.New(nil)
	return &Instagram{client: client, options: o}
}

func (i *Instagram) Collect(ctx context.Context, q Query) Result {
	r := Result{Status: "BLOCKED", Incomplete: true}
	username := strings.TrimPrefix(strings.TrimSpace(q.Username), "@")
	if q.URL != "" {
		platform, handle, ok := profile.Account(q.URL)
		if !ok || platform != "instagram" {
			r.Events = []Event{{"instagram:input", "BLOCKED", "An Instagram profile URL is required"}}
			return r
		}
		username = handle
	}
	if !ig.UsernamePattern.MatchString(username) || q.Budget < 1 || i.options.Validate() != nil {
		r.Events = []Event{{"instagram:input", "BLOCKED", "Valid exact username, budget, and max_posts (1..12) required"}}
		return r
	}
	ctx, cancel := context.WithTimeout(ctx, 75*time.Second)
	defer cancel()
	if accessFrom(ctx) == nil {
		ctx = WithAccess(ctx, &AccessOptions{StateDir: "data/access-state", MinIntervalMS: 3000}, q.Budget)
	}
	ctx = context.WithValue(ctx, noAccessRetryKey{}, true)
	before, previousEvents, _ := AccessSnapshot(ctx)
	// Isolate anonymous cookies per collection, even when the adapter is reused.
	client := *i.client
	client.Jar, _ = cookiejar.New(nil)
	collect := ig.New(&client, func(req *http.Request) (*http.Response, error) {
		used, _, _ := AccessSnapshot(ctx)
		if used-before >= q.Budget {
			return nil, &accessError{"Instagram request budget exhausted; no request sent"}
		}
		return doAccess(&client, req)
	})
	report := collect.Collect(ctx, username, i.options.MaxPosts)
	after, accessEvents, _ := AccessSnapshot(ctx)
	r.Access = accessEvents[len(previousEvents):]
	r.Requests = after - before
	r.Status = "FAILED"
	detail := report.StopReason
	if report.Error != nil {
		detail = report.Error.Message
	}
	if len(report.Requests) > 0 {
		code := report.Requests[len(report.Requests)-1].Status
		if code == 401 || code == 403 || code == 429 || code >= 300 && code < 400 || strings.Contains(detail, "cooldown") {
			r.Status = "BLOCKED"
		}
	}
	postStatus := report.PostsStatus
	if postStatus == "PRIVATE_PROFILE_NOT_REQUESTED" || postStatus == "NOT_ATTEMPTED" {
		postStatus = "NOT_CHECKED"
	}
	if postStatus == "PARTIAL" {
		postStatus = "FAILED"
	}
	profileStatus := report.ProfileStatus
	profileDetail := "Profile metadata retrieved; account ownership remains unverified"
	if report.Profile == nil {
		profileStatus = r.Status
		profileDetail = detail
	} else if r.Status == "BLOCKED" {
		postStatus = "BLOCKED"
	}
	r.Events = []Event{{"instagram:profile", profileStatus, profileDetail}, {"instagram:posts", postStatus, fmt.Sprintf("status=%s collected=%d limit=%d stop=%s; %s", report.PostsStatus, len(report.Posts), report.MaxPosts, report.StopReason, detail)}}
	if report.Profile == nil {
		return r
	}
	r.Status = "SUCCESS" // Profile success never implies complete post collection.
	p := report.Profile
	url := "https://www.instagram.com/" + p.Username + "/"
	first := report.Requests[0]
	source := FactSource{URL: url, RequestURL: first.URL, ObservedAt: first.ObservedAt, ResponseSHA256: first.SHA256, Rule: "instagram-profile-v1"}
	result := Profile{Key: keyFor("web", url), Platform: "instagram", URL: url, SourceURL: url, Login: p.Username, Name: p.FullName, Bio: p.Biography, ResponseSHA256: first.SHA256, Rule: source.Rule, FactSources: map[string]FactSource{"name": source, "username": source}}
	add := func(kind, value string, proof FactSource) {
		result.Facts = append(result.Facts, Fact{Type: kind, Value: value})
		result.FactSources[kind+"\x00"+value] = proof
	}
	add("page_title", fmt.Sprintf("%s (@%s) | Instagram", p.FullName, p.Username), source)
	if p.Biography != "" {
		add("profile_description", p.Biography, source)
	}
	for _, post := range report.Posts {
		value, _ := json.Marshal(post)
		rule := "instagram-post-v1"
		if post.SourceKind == "timeline" {
			rule = "instagram-timeline-v1"
		}
		add("instagram_post", string(value), FactSource{URL: post.URL, RequestURL: post.Receipt.URL, ObservedAt: post.Receipt.ObservedAt, ResponseSHA256: post.Receipt.SHA256, Rule: rule})
	}
	// Keep bounded coverage and request diagnostics even when a later post fails.
	report.Posts = nil
	metadata, _ := json.Marshal(report)
	add("instagram_collection", string(metadata), source)
	r.Profiles = []Profile{result}
	return r
}

type publicPage struct {
	page      Collector
	instagram Collector
}

func NewPublicPage(timeout time.Duration, options *InstagramOptions) Collector {
	return &publicPage{page: NewPage(timeout), instagram: NewInstagram(timeout, options)}
}
func (p *publicPage) Collect(ctx context.Context, q Query) Result {
	platform, _, ok := profile.Account(q.URL)
	if ok && platform == "instagram" {
		return p.instagram.Collect(ctx, q)
	}
	return p.page.Collect(ctx, q)
}
func (d *Discovery) SetInstagramOptions(timeout time.Duration, options *InstagramOptions) {
	d.page = NewPublicPage(timeout, options)
}

// HTML usually needs one fetch. An Instagram profile additionally needs an
// anonymous bootstrap and one metadata request per post, within the same budget.
func (d *Discovery) inspectionBudget(raw string, remaining, htmlLimit int) int {
	platform, _, ok := profile.Account(raw)
	if ok && platform == "instagram" {
		posts := 3
		if page, ok := d.page.(*publicPage); ok {
			if ig, ok := page.instagram.(*Instagram); ok {
				posts = ig.options.MaxPosts
			}
		}
		return min(remaining, posts+2)
	}
	return min(remaining, htmlLimit)
}
