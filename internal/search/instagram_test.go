package search

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/collector"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/dossier"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/evidence"
	ig "github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/instagram"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/scoring"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/storage"
)

const syntheticCaption = "Test User works at Fictional Corp.\nLokasi: Bandung\nKegiatan: Workshop\nBersama @Teman #Acara  kemarin 🥂"

func instagramCase(t *testing.T) domain.Case {
	t.Helper()
	var cfg Config
	if err := storage.Read("../../configs/collector-v1.json", &cfg); err != nil {
		t.Fatal(err)
	}
	caption := syntheticCaption
	post := ig.Post{URL: "https://www.instagram.com/p/POST1/", PublisherUsername: "testuser", PublisherID: "123", PublishedAt: "2025-02-23T14:55:02Z", Caption: &caption, ClaimStatus: "UNVERIFIED", Mentions: []string{"tampered"}, Hashtags: []string{"tampered"}}
	encoded, _ := json.Marshal(post)
	meta, _ := json.Marshal(ig.Report{Profile: &ig.Profile{Username: "testuser", UserID: "123"}, CollectedPosts: 1, ProfileStatus: "SUCCESS", PostsStatus: "SUCCESS", StopReason: "sample_limit"})
	proof := collector.FactSource{URL: post.URL, RequestURL: "https://www.instagram.com/api/graphql", ObservedAt: "2026-10-29T01:02:03Z", ResponseSHA256: strings.Repeat("a", 64), Rule: "instagram-post-v1"}
	p := collector.Profile{Key: "ig-test", URL: "https://www.instagram.com/testuser/", SourceURL: "https://www.instagram.com/testuser/", Name: "Test User", Login: "testuser", Facts: []collector.Fact{{Type: "instagram_post", Value: string(encoded)}, {Type: "instagram_collection", Value: string(meta)}}, FactSources: map[string]collector.FactSource{"instagram_post": proof}}
	f := &fakeCollector{result: collector.Result{Status: "SUCCESS", Incomplete: true, Requests: 3, Profiles: []collector.Profile{p}, Events: []collector.Event{{Operation: "instagram:profile", Status: "SUCCESS"}, {Operation: "instagram:posts", Status: "SUCCESS"}}}}
	c, err := Run(context.Background(), f, Input{Provider: "instagram", Username: "testuser"}, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestInstagramEvidenceToReviewAndDossier(t *testing.T) {
	c := instagramCase(t)
	if c.Search.RequestsUsed != 3 || c.Search.Status != "PARTIAL" || len(c.SocialPosts) != 1 || c.Candidates[0].Decision != "PENDING" || c.Candidates[0].Location != "" || c.Candidates[0].Company != "" || len(c.Relations) != 0 {
		t.Fatalf("caption promoted or pipeline lost data: %+v", c)
	}
	p := c.SocialPosts[0]
	if p.Caption == nil || *p.Caption != syntheticCaption || !reflect.DeepEqual(p.Mentions, []string{"teman"}) || !reflect.DeepEqual(p.Hashtags, []string{"acara"}) || p.Sources[0].ObservedAt != "2026-10-29T01:02:03Z" || p.Sources[0].ResponseSHA256 != strings.Repeat("a", 64) || p.Status != "UNVERIFIED" {
		t.Fatalf("damaged evidence: %+v", p)
	}
	if len(p.Clues) != 4 {
		t.Fatalf("missing literal clues: %+v", p.Clues)
	}
	for _, clue := range p.Clues {
		if clue.Status != "UNVERIFIED" || !strings.Contains(*p.Caption, clue.Quote) {
			t.Fatalf("unsupported clue: %+v", clue)
		}
	}
	again, err := evidence.Normalize(c)
	if err != nil || !reflect.DeepEqual(c, again) {
		t.Fatalf("unstable normalization: %v", err)
	}
	r, err := dossier.Build(c)
	if err != nil || len(r.SocialPosts) != 0 {
		t.Fatalf("unconfirmed posts entered dossier: %+v %v", r, err)
	}
	c.Candidates[0].Decision = "CONFIRM"
	r, err = dossier.Build(c)
	if err != nil || len(r.SocialPosts) != 1 || r.SocialPosts[0].Status != "UNVERIFIED" {
		t.Fatalf("confirmed identity lost/verified caption: %+v %v", r, err)
	}
}

func TestInstagramEvidenceRejectsBrokenBinding(t *testing.T) {
	for _, kind := range []string{"publisher", "missing_caption", "source", "account_id"} {
		t.Run(kind, func(t *testing.T) {
			c := instagramCase(t)
			for i := range c.Evidence {
				e := &c.Evidence[i]
				if e.Type != "instagram_post" {
					continue
				}
				switch kind {
				case "publisher":
					e.Value = strings.Replace(e.Value, `"publisher_username":"testuser"`, `"publisher_username":"another"`, 1)
				case "account_id":
					e.Value = strings.Replace(e.Value, `"publisher_id":"123"`, `"publisher_id":"999"`, 1)
				case "source":
					e.Sources[0].URL = "https://www.instagram.com/p/OTHER/"
				case "missing_caption":
					var v map[string]any
					json.Unmarshal([]byte(e.Value), &v)
					delete(v, "caption")
					b, _ := json.Marshal(v)
					e.Value = string(b)
				}
			}
			if _, err := evidence.Normalize(c); err == nil {
				t.Fatal("invalid provenance accepted")
			}
		})
	}
}

func TestInstagramRawObservationsDoNotIncreasePES(t *testing.T) {
	c := instagramCase(t)
	c.Candidates[0].Decision = "CONFIRM"
	var policy domain.Policy
	if err := storage.Read("../../configs/experimental-v1.json", &policy); err != nil {
		t.Fatal(err)
	}
	with, err := scoring.Calculate(c, policy)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range c.Evidence {
		if strings.HasPrefix(e.Type, "instagram_") && !slices.Contains(with.ExcludedIDs, e.ID) {
			t.Fatalf("raw social evidence scored: %s", e.ID)
		}
	}
	var without []domain.Evidence
	for _, e := range c.Evidence {
		if !strings.HasPrefix(e.Type, "instagram_") {
			without = append(without, e)
		}
	}
	c.Evidence = without
	c.SocialPosts = nil
	base, err := scoring.Calculate(c, policy)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(with.PES, base.PES) || !reflect.DeepEqual(with.Dimensions, base.Dimensions) {
		t.Fatal("caption/diagnostics changed PES")
	}
}

func TestInstagramOptionsCompatibility(t *testing.T) {
	var cfg Config
	if err := storage.Read("../../configs/collector-v1.json", &cfg); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, limit := range []int{0, 13} {
		cfg.Instagram = &collector.InstagramOptions{MaxPosts: limit}
		if cfg.Validate() == nil {
			t.Fatalf("accepted limit %d", limit)
		}
	}
	cfg.Instagram = &collector.InstagramOptions{MaxPosts: 12}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestInstagramTimelineEvidenceHasExplicitUnknownTime(t *testing.T) {
	c := instagramCase(t)
	for i := range c.Evidence {
		e := &c.Evidence[i]
		if e.Type == "instagram_post" {
			var p ig.Post
			json.Unmarshal([]byte(e.Value), &p)
			p.SourceKind = "timeline"
			p.PublisherIDType = "web_id"
			p.PublisherID = "web123"
			p.PublishedAt = ""
			p.TimestampStatus = "UNAVAILABLE"
			b, _ := json.Marshal(p)
			e.Value = string(b)
			e.Sources[0].RequestURL = "https://www.instagram.com/testuser/"
			e.Sources[0].Rule = "instagram-timeline-v1"
		}
		if e.Type == "instagram_collection" {
			var r ig.Report
			json.Unmarshal([]byte(e.Value), &r)
			r.Profile.WebID = "web123"
			b, _ := json.Marshal(r)
			e.Value = string(b)
		}
	}
	n, err := evidence.Normalize(c)
	if err != nil || len(n.SocialPosts) != 1 {
		t.Fatalf("timeline rejected: %v", err)
	}
	p := n.SocialPosts[0]
	if p.PublishedAt != "" || p.TimestampStatus != "UNAVAILABLE" || p.Caption == nil || *p.Caption != syntheticCaption || p.PublisherIDType != "web_id" {
		t.Fatalf("invented missing metadata: %+v", p)
	}
	for i := range c.Evidence {
		if c.Evidence[i].Type == "instagram_post" {
			c.Evidence[i].Sources[0].RequestURL = "https://www.instagram.com/other/"
		}
	}
	if _, err := evidence.Normalize(c); err == nil {
		t.Fatal("another profile accepted as caption source")
	}
}
