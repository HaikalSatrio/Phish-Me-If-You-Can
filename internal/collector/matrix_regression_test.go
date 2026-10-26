package collector

import (
	"context"
	"strings"
	"testing"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
)

func TestScopedAccountSearchRejectsOtherOwnersMentions(t *testing.T) {
	q := Query{Name: "Mira", Username: "mira_art", Platform: "instagram"}
	for _, raw := range []string{"https://instagram.com/other/", "https://instagram.com/other/reel/123", "https://instagram.com/p/123"} {
		if AcceptIndexedPerson(q, raw, "Mira (@mira_art)", "An interview with Mira @mira_art") {
			t.Fatalf("mention treated as requested account: %s", raw)
		}
	}
	if !AcceptIndexedPerson(q, "https://instagram.com/mira_art/", "Mira (@mira_art)", "") {
		t.Fatal("requested account missing")
	}
	q.Platform = ""
	if !AcceptIndexedPerson(q, "https://example.org/interview", "Mira (@mira_art)", "") {
		t.Fatal("unscoped public reference was lost")
	}
}

func TestAccountViewsDeduplicateWithoutMergingPosts(t *testing.T) {
	want := discoveryURLKey("https://instagram.com/mira_art")
	for _, raw := range []string{"https://www.instagram.com/mira_art/reels/", "https://instagram.com/mira_art/feed/?hl=en", "https://instagram.com/mira_art/?utm_source=index&igsh=abc"} {
		if got := discoveryURLKey(raw); got != want {
			t.Fatalf("duplicate account view: %s", got)
		}
	}
	for _, raw := range []string{"https://instagram.com/mira_art/reel/123", "https://instagram.com/mira_art2", "https://instagram.com/mira_art/?id=123"} {
		if discoveryURLKey(raw) == want {
			t.Fatalf("distinct resource collapsed: %s", raw)
		}
	}
	if discoveryURLKey("https://instagram.com/p/reels") == discoveryURLKey("https://instagram.com/p") {
		t.Fatal("post shortcode mistaken for account view")
	}
}

func TestAccountClueLeavesRoomForMononymBiography(t *testing.T) {
	index := &anchorPage{response: func(q Query) Result {
		profiles := []Profile{}
		if q.Platform == "instagram" {
			for _, suffix := range []string{"", "/reels", "/feed"} {
				profiles = append(profiles, Profile{URL: "https://instagram.com/mira_art" + suffix, Facts: []Fact{{"search_title", "Mira (@mira_art)"}}})
			}
		} else {
			profiles = append(profiles, Profile{URL: "https://example.org/mira", Facts: []Fact{{"search_snippet", "Mira Solis, known as Mira, is a singer."}}})
		}
		return Result{Status: "SUCCESS", Requests: 1, Profiles: profiles[:min(q.Limit, len(profiles))]}
	}}
	page := &anchorPage{response: func(q Query) Result {
		return Result{Status: "SUCCESS", Requests: 1, Profiles: []Profile{{URL: q.URL, Facts: []Fact{{"page_title", "Mira (@mira_art)"}}}}}
	}}
	d := Discovery{providers: []Collector{index}, page: page}
	r := d.Collect(context.Background(), Query{Name: "Mira", Accounts: []domain.AccountClue{{Platform: "instagram", Username: "mira_art"}}, Budget: 12, Limit: 5})
	found := false
	for _, p := range r.Profiles {
		found = found || strings.HasPrefix(p.URL, "https://example.org/")
	}
	if !found || r.Requests > 12 {
		t.Fatalf("account search crowded out biography: %+v", r)
	}
}
