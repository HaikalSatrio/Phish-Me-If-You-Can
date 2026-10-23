package collector

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
)

type DevTo struct {
	client   *http.Client
	endpoint string
}

func NewDevTo(timeout time.Duration) *DevTo {
	return &DevTo{publicClient(timeout), "https://dev.to/api/users/by_username"}
}

func (d *DevTo) Collect(ctx context.Context, q Query) Result {
	r := Result{Status: "FAILED"}
	if !regexp.MustCompile(`^[A-Za-z0-9_]{1,30}$`).MatchString(q.Username) || q.Budget < 1 {
		r.Events = append(r.Events, Event{"devto:input", "FAILED", "Supply a DEV username and request budget"})
		return r
	}
	if ctx.Err() != nil {
		r.Incomplete = true
		r.Events = append(r.Events, Event{"devto:profile", "NOT_CHECKED", "Cancelled"})
		return r
	}
	raw := d.endpoint + "?" + url.Values{"url": {q.Username}}.Encode()
	body, status, detail, _ := readPublic(ctx, d.client, raw, map[string]string{"Accept": "application/vnd.forem.api-v1+json"})
	r.Requests, r.Status = 1, status
	receipt := domain.CollectionReceipt{Platform: "devto", URL: raw, Rule: "devto-api-v1", ObservedAt: time.Now().UTC().Format(time.RFC3339), Status: status, Reason: detail}
	if len(body) > 0 {
		receipt.Bytes, receipt.ResponseSHA256 = len(body), fmt.Sprintf("%x", sha256.Sum256(body))
	}
	if status == "SUCCESS" {
		var p struct {
			ID       int64  `json:"id"`
			Username string `json:"username"`
			Name     string `json:"name"`
			Summary  string `json:"summary"`
			Location string `json:"location"`
			Website  string `json:"website_url"`
		}
		if json.Unmarshal(body, &p) != nil || p.ID < 1 || !strings.EqualFold(p.Username, q.Username) {
			receipt.Status, receipt.Reason = "FAILED", "Invalid or mismatched DEV public profile"
		} else {
			r.Profiles = append(r.Profiles, Profile{Key: fmt.Sprintf("devto-%d", p.ID), Platform: "devto", Login: p.Username, Name: p.Name, Location: p.Location, Blog: p.Website, URL: "https://dev.to/" + url.PathEscape(p.Username), SourceURL: raw, Rule: receipt.Rule, ResponseSHA256: receipt.ResponseSHA256, Facts: []Fact{{"profile_description", p.Summary}}})
			receipt.Reason = "Public DEV profile retrieved; ownership requires review"
		}
	}
	r.Status = receipt.Status
	r.Receipts = append(r.Receipts, receipt)
	r.Events = append(r.Events, Event{"devto:profile", receipt.Status, receipt.Reason})
	return r
}

type PublicProfiles struct{ collectors map[string]Collector }

func NewPublicProfiles(timeout time.Duration, githubToken string) *PublicProfiles {
	return &PublicProfiles{map[string]Collector{"github": NewGitHub(timeout, githubToken), "devto": NewDevTo(timeout), "bluesky": NewBluesky(timeout)}}
}

func (a *PublicProfiles) Collect(ctx context.Context, q Query) Result {
	r := Result{Status: "EMPTY"}
	if strings.TrimSpace(q.Username) == "" || q.Budget < 1 || q.Limit < 1 {
		r.Status = "FAILED"
		r.Events = append(r.Events, Event{"auto:input", "FAILED", "Supply a username and valid limits"})
		return r
	}
	if q.Platform != "" && q.Platform != "all" && a.collectors[q.Platform] == nil {
		r.Status = "FAILED"
		r.Events = append(r.Events, Event{"auto:input", "FAILED", "Supported public profile platforms: github, devto, bluesky. Use -source web for indexed discovery on other platforms."})
		return r
	}
	for _, name := range []string{"github", "devto", "bluesky"} {
		if q.Platform != "" && q.Platform != "all" && q.Platform != name {
			continue
		}
		next := q
		next.Platform, next.Budget, next.Limit = name, q.Budget-r.Requests, q.Limit-len(r.Profiles)
		if name == "bluesky" && !strings.Contains(next.Username, ".") {
			next.Username += ".bsky.social"
		}
		valid := true
		if name == "github" {
			valid = ValidLogin(next.Username)
		}
		if name == "devto" {
			valid = regexp.MustCompile(`^[A-Za-z0-9_]{1,30}$`).MatchString(next.Username)
		}
		if name == "bluesky" {
			valid = validBlueskyHandle(next.Username)
		}
		if !valid || ctx.Err() != nil || next.Budget < 1 || next.Limit < 1 {
			r.Incomplete = true
			r.Events = append(r.Events, Event{"auto:" + name, "NOT_CHECKED", "Username syntax, cancellation, or collection limit prevents this lookup"})
			continue
		}
		part := a.collectors[name].Collect(ctx, next)
		r.Requests += part.Requests
		r.Profiles = append(r.Profiles, part.Profiles...)
		r.Receipts = append(r.Receipts, part.Receipts...)
		for _, event := range part.Events {
			if name == "bluesky" && next.Username != q.Username {
				event.Detail = "Default handle " + next.Username + " only; custom domains require a full handle. " + event.Detail
			}
			r.Events = append(r.Events, event)
		}
		if part.Incomplete || (part.Status != "SUCCESS" && part.Status != "EMPTY") {
			r.Incomplete = true
		}
		if part.Status == "BLOCKED" || (part.Status == "FAILED" && r.Status != "BLOCKED") {
			r.Status = part.Status
		}
	}
	if len(r.Profiles) > 0 {
		r.Status = "SUCCESS"
	} else if r.Incomplete && r.Status == "EMPTY" {
		r.Status = "FAILED"
	}
	return r
}
