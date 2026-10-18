package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var loginPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}$`)

func ValidLogin(s string) bool { return loginPattern.MatchString(s) }

type GitHub struct {
	client *http.Client
	base   string
	token  string
}

func NewGitHub(timeout time.Duration, token string) *GitHub {
	return &GitHub{client: &http.Client{Timeout: timeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}, base: "https://api.github.com", token: token}
}

func (g *GitHub) get(ctx context.Context, path string, out any) (string, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.base+path, nil)
	if err != nil {
		return "FAILED", "Unable to construct API request"
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2026-03-10")
	req.Header.Set("User-Agent", "Phish-Me-If-You-Can/0.2.0")
	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return "FAILED", "Request failed, timed out, or was cancelled"
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return "EMPTY", "Profile was not found"
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 429 {
		return "BLOCKED", fmt.Sprintf("GitHub HTTP %d; authentication or rate limit restriction. No automatic retry.", resp.StatusCode)
	}
	if resp.StatusCode != 200 {
		return "FAILED", fmt.Sprintf("GitHub HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, (2<<20)+1))
	if err != nil || len(body) > 2<<20 {
		return "FAILED", "Invalid or oversized API response"
	}
	if err := json.Unmarshal(body, out); err != nil {
		return "FAILED", "Malformed API JSON"
	}
	return "SUCCESS", "Public GitHub API response received"
}

func (g *GitHub) Collect(ctx context.Context, q Query) Result {
	r := Result{Profiles: []Profile{}, Events: []Event{}, Status: "SUCCESS"}
	if q.Limit < 1 || q.Limit > 20 || q.Budget < 1 || (q.Username != "" && !ValidLogin(q.Username)) || (q.Username == "" && strings.TrimSpace(q.Name) == "") {
		r.Status = "FAILED"
		r.Events = append(r.Events, Event{"input", "FAILED", "Invalid collector query or budget"})
		return r
	}
	logins := []string{}
	if q.Username != "" {
		logins = append(logins, q.Username)
	} else {
		var response struct {
			Total      int       `json:"total_count"`
			Incomplete bool      `json:"incomplete_results"`
			Items      []Profile `json:"items"`
		}
		// Name is a literal phrase; qualifiers come only from this collector.
		name := strings.NewReplacer("\"", " ", "\\", " ", "\r", " ", "\n", " ").Replace(q.Name)
		path := "/search/users?" + url.Values{"q": {`"` + name + `" in:fullname type:user`}, "per_page": {strconv.Itoa(q.Limit)}}.Encode()
		status, detail := g.get(ctx, path, &response)
		r.Requests++
		r.Events = append(r.Events, Event{"github:search", status, detail})
		if status != "SUCCESS" {
			r.Status = status
			return r
		}
		r.Incomplete = response.Incomplete || response.Total > len(response.Items) || len(response.Items) > q.Limit
		for _, p := range response.Items {
			if len(logins) >= q.Limit {
				break
			}
			if p.Type == "User" && ValidLogin(p.Login) {
				logins = append(logins, p.Login)
			} else {
				r.Incomplete = true
			}
		}
	}
	seen := map[string]bool{}
	for _, login := range logins {
		if seen[strings.ToLower(login)] {
			continue
		}
		seen[strings.ToLower(login)] = true
		if r.Requests >= q.Budget || ctx.Err() != nil {
			r.Incomplete = true
			r.Events = append(r.Events, Event{"github:profiles", "NOT_CHECKED", "Request budget or cancellation stopped collection"})
			break
		}
		var p Profile
		status, detail := g.get(ctx, "/users/"+url.PathEscape(login), &p)
		r.Requests++
		if status == "SUCCESS" && (p.ID <= 0 || !ValidLogin(p.Login) || p.Type != "User" || !strings.EqualFold(login, p.Login)) {
			status = "FAILED"
			detail = "Response is not the requested user profile"
		}
		r.Events = append(r.Events, Event{"github:user:" + login, status, detail})
		if status == "SUCCESS" {
			if p.Bio != "" {
				p.Facts = append(p.Facts, Fact{"profile_description", p.Bio})
			}
			r.Profiles = append(r.Profiles, p)
		} else {
			if status == "EMPTY" && q.Username != "" {
				r.Status = "EMPTY"
				return r
			}
			r.Incomplete = true
			if status == "BLOCKED" {
				r.Status = "BLOCKED"
				break
			}
			if status == "FAILED" {
				r.Status = "FAILED"
			}
		}
	}
	if len(r.Profiles) == 0 && r.Status == "SUCCESS" {
		if r.Incomplete {
			r.Status = "FAILED"
		} else {
			r.Status = "EMPTY"
		}
	}
	return r
}
