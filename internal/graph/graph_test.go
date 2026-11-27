package graph

import (
	"strings"
	"testing"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/evidence"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/storage"
)

var testPolicy = domain.Policy{AcceptThreshold: 0.9, ReviewThreshold: 0.6}

func nodeByType(g Graph, typ string) []Node {
	var out []Node
	for _, n := range g.Nodes {
		if n.Type == typ {
			out = append(out, n)
		}
	}
	return out
}

func TestKevinFixture(t *testing.T) {
	var c domain.Case
	if err := storage.Read("../../fixtures/kevin.json", &c); err != nil {
		t.Fatal(err)
	}
	c, err := evidence.Normalize(c)
	if err != nil {
		t.Fatal(err)
	}
	var p domain.Policy
	if err := storage.Read("../../configs/experimental-v1.json", &p); err != nil {
		t.Fatal(err)
	}
	g := Build(c, Options{Policy: p})
	s := g.Summary
	if s.Accounts != 1 || s.ExcludedAccounts != 1 || s.Facts != 6 {
		t.Fatalf("unexpected accepted-only summary: %+v", s)
	}
	want := map[string]int{"affiliation": 2, "contact": 1, "routine": 2, "social": 1}
	for cat, n := range want {
		if s.FactsByCategory[cat] != n {
			t.Fatalf("category %s = %d, want %d (%+v)", cat, s.FactsByCategory[cat], n, s.FactsByCategory)
		}
	}
	g = Build(c, Options{Policy: p, IncludeUnconfirmed: true})
	if g.Summary.Accounts != 2 || g.Summary.Facts != 7 {
		t.Fatalf("unconfirmed account not included: %+v", g.Summary)
	}
	for _, n := range g.Nodes {
		if n.Kind == "account" && n.Label == "kevin-profile-other" && n.IdentityState == "ACCEPTED" {
			t.Fatal("low-confidence account labelled as accepted")
		}
	}
}

func TestSharedFactsAndExclusions(t *testing.T) {
	src := []domain.Source{{URL: "https://example.org/a"}}
	c := domain.Case{
		Target: domain.Target{ID: "t", Name: "Test Person"},
		Candidates: []domain.Candidate{
			{ID: "a", ProfileURL: "https://github.com/tester", Confidence: 0.6, Decision: "CONFIRM"},
			{ID: "b", ProfileURL: "https://bsky.app/profile/tester.bsky.social", Confidence: 0.6, Decision: "CONFIRM"},
			{ID: "x", Confidence: 0.9, Decision: "REJECT", Matches: []string{"name"}},
		},
		Evidence: []domain.Evidence{
			{ID: "a1", CandidateID: "a", Type: "email", Value: "Tester@Example.org", Status: "OBSERVED", Confidence: 0.9, Discoverability: 0.5, Sources: src},
			{ID: "b1", CandidateID: "b", Type: "email", Value: " tester@example.org ", Status: "OBSERVED", Confidence: 0.7, Discoverability: 0.8, Sources: src},
			{ID: "a2", CandidateID: "a", Type: "search_snippet", Value: "indexed text", Status: "OBSERVED", Confidence: 1, Sources: src},
			{ID: "a3", CandidateID: "a", Type: "location", Value: "Old City", Status: "OBSERVED", Confidence: 1, SupersededBy: "a4", Sources: src},
			{ID: "x1", CandidateID: "x", Type: "email", Value: "someone@else.org", Status: "OBSERVED", Confidence: 1, Sources: src},
		},
		Relations: []domain.Relation{
			{ID: "r1", CandidateID: "a", Kind: "employment", Organization: "Example Corp", Role: "Analyst", Temporal: "current", EvidenceID: "a1", Reviews: []domain.RelationReview{{Decision: "ACCEPT"}}},
			{ID: "r2", CandidateID: "a", Kind: "employment", Organization: "Wrong Corp", EvidenceID: "a1", Reviews: []domain.RelationReview{{Decision: "REJECT"}}},
		},
	}
	g := Build(c, Options{Policy: testPolicy})
	emails := nodeByType(g, "email")
	if len(emails) != 1 || len(emails[0].Accounts) != 2 || g.Summary.SharedFacts != 1 {
		t.Fatalf("identical email across accounts not merged: %+v", emails)
	}
	if emails[0].Confidence != 0.7 || emails[0].Discoverability != 0.8 {
		t.Fatalf("merged node must keep min confidence and max discoverability: %+v", emails[0])
	}
	jobs := nodeByType(g, "employment")
	if len(jobs) != 1 || jobs[0].Status != "ACCEPTED" || jobs[0].Category != "affiliation" {
		t.Fatalf("relation handling wrong: %+v", jobs)
	}
	ex := g.Summary.Excluded
	if ex["discovery_metadata"] != 1 || ex["superseded_evidence"] != 1 || ex["rejected_relation"] != 1 || ex["evidence_of_excluded_account"] != 1 || g.Summary.ExcludedAccounts != 1 {
		t.Fatalf("exclusions not counted: %+v", g.Summary)
	}
	for _, n := range g.Nodes {
		if n.Label == "someone@else.org" || n.Label == "Old City" {
			t.Fatalf("excluded fact entered the graph: %+v", n)
		}
	}
}

func TestSocialPostClues(t *testing.T) {
	caption1, caption2 := "Hari ini #lari bareng @teman\nLokasi: Jakarta", "Pagi #lari lagi"
	c := domain.Case{
		Target:     domain.Target{ID: "t", Name: "Test"},
		Candidates: []domain.Candidate{{ID: "ig", ProfileURL: "https://www.instagram.com/tester/", Confidence: 0.6, Decision: "CONFIRM"}},
		Evidence:   []domain.Evidence{{ID: "p1", CandidateID: "ig", Type: "instagram_post", Status: "OBSERVED", Confidence: 0.8}, {ID: "p2", CandidateID: "ig", Type: "instagram_post", Status: "OBSERVED", Confidence: 0.8}},
		SocialPosts: []domain.SocialPost{
			{EvidenceID: "p1", CandidateID: "ig", URL: "https://www.instagram.com/p/AAA/", Caption: &caption1, Clues: []domain.CaptionClue{{Kind: "hashtag", Value: "lari"}, {Kind: "mention", Value: "teman"}, {Kind: "location_label", Value: "Jakarta"}}},
			{EvidenceID: "p2", CandidateID: "ig", URL: "https://www.instagram.com/p/BBB/", Caption: &caption2, Clues: []domain.CaptionClue{{Kind: "hashtag", Value: "lari"}}},
		},
	}
	g := Build(c, Options{Policy: testPolicy})
	if len(nodeByType(g, "post")) != 2 || len(nodeByType(g, "hashtag")) != 1 {
		t.Fatalf("posts or merged hashtag wrong: %+v", g.Nodes)
	}
	loc := nodeByType(g, "location_label")
	if len(loc) != 1 || loc[0].Category != "routine" || nodeByType(g, "mention")[0].Category != "social" {
		t.Fatal("caption clues not categorised")
	}
	reveals := 0
	for _, e := range g.Edges {
		if e.Kind == "reveals" {
			reveals++
		}
	}
	if reveals != 4 {
		t.Fatalf("want 4 reveals edges, got %d", reveals)
	}
	if g.Summary.Excluded["discovery_metadata"] != 2 {
		t.Fatal("raw instagram_post evidence should enter only through SocialPosts")
	}
}

func TestLabelFoldsStyledLetters(t *testing.T) {
	if got := truncate("𝐋𝐄𝐓   𝐓𝐇𝐄 Kingdom", 40); got != "LET THE Kingdom" {
		t.Fatalf("styled letters not folded: %q", got)
	}
	if got := truncate("abcdefghij", 5); got != "abcd…" {
		t.Fatalf("not truncated: %q", got)
	}
}

func TestDOT(t *testing.T) {
	c := domain.Case{
		Target:     domain.Target{ID: "t", Name: `Quote "Name"`},
		Candidates: []domain.Candidate{{ID: "a", Confidence: 0.6, Decision: "CONFIRM"}, {ID: "b", Confidence: 0.3, Matches: []string{"name"}}},
		Evidence:   []domain.Evidence{{ID: "e", CandidateID: "a", Type: "hobby", Value: "chess", Status: "INFERRED", Confidence: 0.5}},
	}
	out := DOT(Build(c, Options{Policy: testPolicy, IncludeUnconfirmed: true}))
	for _, want := range []string{"digraph exposure", `Quote \"Name\"`, "dashed", "ACCEPTED 0.60", "ABSTAIN"} {
		if !strings.Contains(out, want) {
			t.Fatalf("DOT missing %q:\n%s", want, out)
		}
	}
}
