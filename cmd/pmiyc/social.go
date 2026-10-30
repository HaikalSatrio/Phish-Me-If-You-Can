package main

import (
	"fmt"
	"io"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
)

func printSocialPosts(w io.Writer, posts []domain.SocialPost) {
	for _, post := range posts {
		published := post.PublishedAt
		if published == "" {
			published = "unknown (not provided by source)"
		}
		fmt.Fprintf(w, "  Instagram post | publisher=@%s | claims=%s\n    URL: %s\n    Published: %s\n", post.PublisherUsername, post.Status, post.URL, published)
		if post.Caption == nil {
			fmt.Fprintln(w, "    Caption: null (source explicitly returned no caption)")
		} else {
			fmt.Fprintf(w, "    Caption: %q\n", *post.Caption)
		}
		for _, clue := range post.Clues {
			fmt.Fprintf(w, "    Caption clue: %s=%q | %s\n      Quote: %q\n", clue.Kind, clue.Value, clue.Status, clue.Quote)
		}
		for _, source := range post.Sources {
			fmt.Fprintf(w, "    Evidence: %s | observed=%s\n    Request: %s\n    Response SHA256: %s\n", post.EvidenceID, source.ObservedAt, source.RequestURL, source.ResponseSHA256)
		}
	}
	if len(posts) > 0 {
		fmt.Fprintln(w, "  Caption clues do not establish current location, companionship, or account ownership. Posts are a bounded sample in source order.")
	}
}
