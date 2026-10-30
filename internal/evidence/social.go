package evidence

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	ig "github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/instagram"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/profile"
)

var explicitCaptionLabel = regexp.MustCompile(`(?im)^\s*(lokasi|location|acara|event|kegiatan|organisasi|organization)\s*:\s*([^\r\n]+)`)
var socialPostPath = regexp.MustCompile(`^/p/[A-Za-z0-9_-]{1,30}/$`)

// Clues are literal statements in a caption. They never become current location,
// employment, companionship, or accepted PES evidence automatically.
func captionClues(caption string, mentions, hashtags []string) []domain.CaptionClue {
	var out []domain.CaptionClue
	for _, entry := range []struct {
		kind   string
		values []string
	}{{"mention", mentions}, {"hashtag", hashtags}} {
		kind, values := entry.kind, entry.values
		seen := map[string]bool{}
		for _, value := range values {
			if seen[value] {
				continue
			}
			seen[value] = true
			// The entire caption is an exact quote, including original case/Unicode.
			out = append(out, domain.CaptionClue{Kind: kind, Value: value, Quote: caption, Status: "UNVERIFIED"})
		}
	}
	for _, match := range explicitCaptionLabel.FindAllStringSubmatch(caption, -1) {
		kind := "event_label"
		switch strings.ToLower(match[1]) {
		case "lokasi", "location":
			kind = "location_label"
		case "organisasi", "organization":
			kind = "organization_label"
		}
		out = append(out, domain.CaptionClue{Kind: kind, Value: strings.TrimSpace(match[2]), Quote: match[0], Status: "UNVERIFIED"})
	}
	return out
}

func normalizeSocialPosts(c *domain.Case) error {
	c.SocialPosts = nil // Ignore stale or edited derived views; rebuild from evidence.
	accounts := map[string]domain.Candidate{}
	accountIDs := map[string]string{}
	webIDs := map[string]string{}
	for _, candidate := range c.Candidates {
		accounts[candidate.ID] = candidate
	}
	for _, e := range c.Evidence {
		if e.Type != "instagram_collection" || e.Status != "OBSERVED" {
			continue
		}
		var report ig.Report
		if err := json.Unmarshal([]byte(e.Value), &report); err != nil || report.Profile == nil || report.Profile.UserID == "" {
			return fmt.Errorf("invalid Instagram profile evidence %s", e.ID)
		}
		platform, handle, ok := profile.Account(accounts[e.CandidateID].ProfileURL)
		if !ok || platform != "instagram" || !strings.EqualFold(handle, report.Profile.Username) {
			return fmt.Errorf("Instagram profile evidence does not match candidate %s", e.CandidateID)
		}
		if id := accountIDs[e.CandidateID]; id != "" && id != report.Profile.UserID {
			return fmt.Errorf("conflicting Instagram account IDs for %s", e.CandidateID)
		}
		accountIDs[e.CandidateID] = report.Profile.UserID
		if report.Profile.WebID != "" {
			if previous := webIDs[e.CandidateID]; previous != "" && previous != report.Profile.WebID {
				return fmt.Errorf("conflicting Instagram web IDs for %s", e.CandidateID)
			}
			webIDs[e.CandidateID] = report.Profile.WebID
		}
	}
	for _, e := range c.Evidence {
		if e.Type != "instagram_post" || e.Status != "OBSERVED" || e.SupersededBy != "" {
			continue
		}
		var post ig.Post
		var fields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(e.Value), &fields); err != nil || fields["caption"] == nil {
			return fmt.Errorf("missing Instagram caption field for %s", e.ID)
		}
		if err := json.Unmarshal([]byte(e.Value), &post); err != nil {
			return fmt.Errorf("invalid Instagram post %s: %w", e.ID, err)
		}
		candidate := accounts[e.CandidateID]
		expectedID := accountIDs[e.CandidateID]
		switch post.PublisherIDType {
		case "", "user_id":
		case "web_id":
			if post.SourceKind != "timeline" {
				return fmt.Errorf("unexpected web publisher ID for %s", e.ID)
			}
			expectedID = webIDs[e.CandidateID]
		default:
			return fmt.Errorf("unknown publisher ID type for %s", e.ID)
		}
		platform, handle, ok := profile.Account(candidate.ProfileURL)
		u, err := url.Parse(post.URL)
		if !ok || platform != "instagram" || !strings.EqualFold(handle, post.PublisherUsername) || post.PublisherID == "" || post.PublisherID != expectedID || err != nil || u.Scheme != "https" || u.Host != "www.instagram.com" || !socialPostPath.MatchString(u.Path) || u.RawQuery != "" || u.Fragment != "" || post.ClaimStatus != "UNVERIFIED" {
			return fmt.Errorf("invalid Instagram post/account binding for %s", e.ID)
		}
		if post.PublishedAt == "" {
			if post.SourceKind != "timeline" || post.TimestampStatus != "UNAVAILABLE" {
				return fmt.Errorf("missing Instagram publication time for %s", e.ID)
			}
		} else {
			if _, err := time.Parse(time.RFC3339, post.PublishedAt); err != nil || post.TimestampStatus != "" && post.TimestampStatus != "KNOWN" {
				return fmt.Errorf("invalid Instagram publication time for %s", e.ID)
			}
		}
		if post.SourceKind != "" && post.SourceKind != "detail_api" && post.SourceKind != "timeline" {
			return fmt.Errorf("unknown Instagram source kind for %s", e.ID)
		}
		for _, source := range e.Sources {
			rule := "instagram-post-v1"
			requestOK := source.RequestURL == "https://www.instagram.com/api/graphql"
			if post.SourceKind == "timeline" {
				rule = "instagram-timeline-v1"
				requestOK = requestOK || strings.EqualFold(source.RequestURL, "https://www.instagram.com/"+post.PublisherUsername+"/")
			}
			if source.URL != post.URL || !requestOK || source.ResponseSHA256 == "" || source.Rule != rule {
				return fmt.Errorf("missing Instagram post response provenance for %s", e.ID)
			}
		}
		view := domain.SocialPost{EvidenceID: e.ID, CandidateID: e.CandidateID, Platform: "instagram", AccountURL: candidate.ProfileURL, PublisherUsername: post.PublisherUsername, PublisherID: post.PublisherID, URL: post.URL, PublishedAt: post.PublishedAt, Caption: post.Caption, Mentions: []string{}, Hashtags: []string{}, Status: "UNVERIFIED", Sources: e.Sources}
		view.PublisherIDType, view.TimestampStatus = post.PublisherIDType, post.TimestampStatus
		if post.Caption != nil {
			// Derive tags again, rather than trusting editable metadata in the JSON.
			view.Mentions, view.Hashtags = ig.CaptionTags(*post.Caption)
			view.Clues = captionClues(*post.Caption, view.Mentions, view.Hashtags)
		}
		c.SocialPosts = append(c.SocialPosts, view)
	}
	return nil
}
