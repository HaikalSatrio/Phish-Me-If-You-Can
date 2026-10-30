package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/identity"
)

func TestSocialReviewShowsCaptionAndProof(t *testing.T) {
	caption := "Lokasi: Bandung\n@Teman  kemarin"
	post := domain.SocialPost{EvidenceID: "fixture-post", PublisherUsername: "testuser", URL: "https://www.instagram.com/p/POST1/", PublishedAt: "2025-02-23T14:55:02Z", Caption: &caption, Status: "UNVERIFIED", Sources: []domain.Source{{RequestURL: "https://www.instagram.com/api/graphql", ObservedAt: "2026-10-29T01:02:03Z", ResponseSHA256: strings.Repeat("a", 64)}}}
	var out bytes.Buffer
	printReviewCards(&out, []identity.ReviewCard{{Name: "Test User", Decision: "PENDING", SocialPosts: []domain.SocialPost{post}}})
	for _, want := range []string{"publisher=@testuser", "claims=UNVERIFIED", `Caption: "Lokasi: Bandung\n@Teman  kemarin"`, "Evidence: fixture-post", "observed=2026-10-29T01:02:03Z", "Response SHA256:", "do not establish current location"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in %s", want, out.String())
		}
	}
	post.Caption = nil
	out.Reset()
	printSocialPosts(&out, []domain.SocialPost{post})
	if !strings.Contains(out.String(), "Caption: null") {
		t.Fatal("null caption not explained")
	}
}
