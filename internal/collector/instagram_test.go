package collector

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	ig "github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/instagram"
)

type instagramTransport func(*http.Request) (*http.Response, error)

func (f instagramTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func igResponse(req *http.Request, status int, body string) *http.Response {
	return &http.Response{Request: req, StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
}

const igProfileFixture = `<script type="application/json">{"__bbox":{"result":{"data":{"xig_user_by_username":{"pk":"123","id":"opaque","username":"testuser","full_name":"Test User","is_private":false,"biography":"","polaris_ordered_timeline_connection":{"edges":[{"node":{"code":"POST1"}}],"page_info":{"has_next_page":false}}}}}}}</script>`
const igPostFixture = `{"data":{"xdt_api__v1__media__shortcode__web_info":{"items":[{"code":"POST1","user":{"pk":"123","username":"testuser"},"taken_at":1740322502,"caption":{"text":"Lokasi: Bandung\nBersama @Teman #Acara  kemarin"}}]}}}`

func igTestContext(t *testing.T, budget int) context.Context {
	t.Helper()
	return WithAccess(context.Background(), &AccessOptions{StateDir: t.TempDir(), MaxRetries: 2}, budget)
}

func TestInstagramAdapterPreservesPostProof(t *testing.T) {
	client := NewInstagram(time.Second, nil)
	if !client.client.Transport.(*http.Transport).ForceAttemptHTTP2 {
		t.Fatal("Instagram transport lost HTTP/2 negotiation")
	}
	calls := 0
	client.client.Transport = instagramTransport(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.URL.Host != "www.instagram.com" || req.Header.Get("Authorization") != "" {
			t.Fatal("unexpected destination or credentials")
		}
		switch req.URL.Path {
		case "/testuser/":
			return igResponse(req, 200, igProfileFixture), nil
		case "/":
			return igResponse(req, 200, `"LSD",[],{"token":"fixture"};"IG_USER_EIMU":"0"`), nil
		case "/api/graphql":
			return igResponse(req, 200, igPostFixture), nil
		default:
			t.Fatalf("unexpected request: %s", req.URL)
			return nil, nil
		}
	})
	r := client.Collect(igTestContext(t, 3), Query{Username: "testuser", Budget: 3})
	if r.Status != "SUCCESS" || r.Requests != 3 || calls != 3 || len(r.Access) != 3 || len(r.Profiles) != 1 || !r.Incomplete {
		t.Fatalf("%+v calls=%d", r, calls)
	}
	p := r.Profiles[0]
	posts := 0
	for _, fact := range p.Facts {
		if fact.Type != "instagram_post" {
			continue
		}
		posts++
		var post ig.Post
		if err := json.Unmarshal([]byte(fact.Value), &post); err != nil {
			t.Fatal(err)
		}
		proof := SourceForFact(p, fact)
		if post.PublisherID != "123" || post.Caption == nil || !strings.Contains(*post.Caption, "#Acara  kemarin") || proof.URL != "https://www.instagram.com/p/POST1/" || proof.RequestURL != "https://www.instagram.com/api/graphql" || proof.ResponseSHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte(igPostFixture))) || proof.ObservedAt == "" {
			t.Fatalf("lost proof/caption: %+v %+v", post, proof)
		}
	}
	if posts != 1 || p.Location != "" || p.Company != "" {
		t.Fatalf("missing post or invented fact: %+v", p)
	}
}

func TestInstagramAdapterBudgetAndSharedCooldown(t *testing.T) {
	t.Run("budget", func(t *testing.T) {
		client := NewInstagram(time.Second, nil)
		calls := 0
		client.client.Transport = instagramTransport(func(req *http.Request) (*http.Response, error) {
			calls++
			return igResponse(req, 200, igProfileFixture), nil
		})
		r := client.Collect(igTestContext(t, 5), Query{Username: "testuser", Budget: 1})
		if r.Requests != 1 || calls != 1 || len(r.Profiles) != 1 || r.Events[1].Status != "FAILED" {
			t.Fatalf("budget/partial: %+v calls=%d", r, calls)
		}
	})
	t.Run("429", func(t *testing.T) {
		client := NewInstagram(time.Second, nil)
		calls := 0
		client.client.Transport = instagramTransport(func(req *http.Request) (*http.Response, error) {
			calls++
			resp := igResponse(req, 429, "")
			resp.Header.Set("Retry-After", "900")
			return resp, nil
		})
		opts := &AccessOptions{StateDir: t.TempDir(), MaxRetries: 2}
		for run := 0; run < 2; run++ {
			r := client.Collect(WithAccess(context.Background(), opts, 10), Query{Username: "testuser", Budget: 10})
			want := 1 - run
			if r.Status != "BLOCKED" || r.Requests != want || calls != 1 {
				t.Fatalf("cooldown/retry: %+v calls=%d", r, calls)
			}
		}
	})
	t.Run("503_no_retry", func(t *testing.T) {
		client := NewInstagram(time.Second, nil)
		calls := 0
		client.client.Transport = instagramTransport(func(req *http.Request) (*http.Response, error) { calls++; return igResponse(req, 503, ""), nil })
		r := client.Collect(igTestContext(t, 5), Query{Username: "testuser", Budget: 5})
		if calls != 1 || r.Requests != 1 || r.Status != "FAILED" {
			t.Fatalf("retried: %+v calls=%d", r, calls)
		}
	})
}

func TestInstagramWebAnchorAndURLDispatch(t *testing.T) {
	client := NewInstagram(time.Second, nil)
	calls := 0
	client.client.Transport = instagramTransport(func(req *http.Request) (*http.Response, error) {
		calls++
		return igResponse(req, 200, strings.Replace(igProfileFixture, `"is_private":false`, `"is_private":true`, 1)), nil
	})
	page := &publicPage{page: &scriptedCollector{result: Result{Status: "FAILED"}}, instagram: client}
	d := Discovery{page: page}
	r := d.Collect(igTestContext(t, 1), Query{Accounts: []domain.AccountClue{{Platform: "instagram", Username: "testuser"}}, Budget: 1, Limit: 1})
	if calls != 1 || len(r.Profiles) != 1 || r.Profiles[0].Login != "testuser" || r.Status != "SUCCESS" {
		t.Fatalf("web dispatch failed: %+v", r)
	}
	found := false
	for _, event := range r.Events {
		if event.Operation == "instagram:posts" && event.Status == "NOT_CHECKED" {
			found = true
		}
	}
	if !found {
		t.Fatal("private-post status lost")
	}
	for _, raw := range []string{"https://www.instagram.com/p/POST1/", "https://www.instagram.com@evil.example/testuser/", "https://www.instagram.com/testuser/"} {
		platform := NewPublicPage(time.Second, nil).(*publicPage)
		plain, igFake := &scriptedCollector{result: Result{Status: "EMPTY"}}, &scriptedCollector{result: Result{Status: "EMPTY"}}
		platform.page, platform.instagram = plain, igFake
		platform.Collect(context.Background(), Query{URL: raw, Budget: 1})
		want := 0
		if raw == "https://www.instagram.com/testuser/" {
			want = 1
		}
		if len(igFake.queries) != want {
			t.Fatalf("wrong route: %s", raw)
		}
	}
}

func TestInstagramInspectionUsesRemainingSharedBudget(t *testing.T) {
	d := NewDiscovery(time.Second)
	for _, tc := range []struct {
		url             string
		remaining, want int
	}{{"https://www.instagram.com/testuser/", 20, 5}, {"https://www.instagram.com/testuser/", 2, 2}, {"https://example.org/about", 20, 2}} {
		if got := d.inspectionBudget(tc.url, tc.remaining, 2); got != tc.want {
			t.Fatalf("%s: got %d want %d", tc.url, got, tc.want)
		}
	}
	d.SetInstagramOptions(time.Second, &InstagramOptions{MaxPosts: 12})
	if got := d.inspectionBudget("https://www.instagram.com/testuser/", 20, 2); got != 14 {
		t.Fatalf("configured limit ignored: %d", got)
	}
}

func TestInstagramEmbeddedCaptionAdapter(t *testing.T) {
	client := NewInstagram(time.Second, nil)
	calls := 0
	body := strings.Replace(igProfileFixture, `"code":"POST1"`, `"code":"POST1","user":{"id":"opaque","username":"testuser"},"caption":{"text":"Caption from public profile"}`, 1)
	client.client.Transport = instagramTransport(func(req *http.Request) (*http.Response, error) { calls++; return igResponse(req, 200, body), nil })
	r := client.Collect(igTestContext(t, 1), Query{Username: "testuser", Budget: 1})
	if r.Requests != 1 || calls != 1 || len(r.Profiles) != 1 || r.Events[1].Status != "SUCCESS" {
		t.Fatalf("%+v calls=%d", r, calls)
	}
	posts := 0
	for _, f := range r.Profiles[0].Facts {
		if f.Type != "instagram_post" {
			continue
		}
		posts++
		proof := SourceForFact(r.Profiles[0], f)
		if proof.RequestURL != "https://www.instagram.com/testuser/" || proof.URL != "https://www.instagram.com/p/POST1/" || proof.Rule != "instagram-timeline-v1" || proof.ResponseSHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte(body))) {
			t.Fatalf("wrong source: %+v", proof)
		}
	}
	if posts != 1 {
		t.Fatal("caption not exported")
	}
}
