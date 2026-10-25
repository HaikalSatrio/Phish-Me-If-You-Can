package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/identity"
)

func TestReviewCardsShowPublicNamesWithLiteralSource(t *testing.T) {
	var buf bytes.Buffer
	printReviewCards(&buf, []identity.ReviewCard{{CandidateID: "a", Name: "Awan", Decision: "PENDING", URL: "https://example.org/awan", Names: []identity.NameCard{{NameClaim: domain.NameClaim{Alias: "Awan", Name: "Sari Utami", Kind: "known_as", Quote: "Sari Utami, known as Awan, is"}, Sources: []domain.Source{{URL: "https://source.example/awan", ObservedAt: "2026-10-26T00:00:00Z"}}}}}})
	for _, want := range []string{"Awan", "Sari Utami", "unverified", "Sari Utami, known as Awan, is", "https://source.example/awan", "PENDING"} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("missing review context %q: %s", want, buf.String())
		}
	}
}
