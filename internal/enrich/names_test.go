package enrich

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/evidence"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/extract"
)

func nameFixture() domain.Case {
	c := fixture("Sari Utami, known as Awan, is a singer. Sari Utami works at Orbit Records.")
	c.Target.Name = "Awan"
	c.Candidates[0].DisplayName = "Awan"
	c.Candidates[0].ProfileURL = "https://example.org/awan"
	return c
}

func TestPublicNamesSupportSameDocumentRelationsAndReplay(t *testing.T) {
	c := nameFixture()
	// Another document about a namesake must not acquire this subject automatically.
	e := c.Evidence[0]
	e.ID, e.Value = "other", "Sari Utami works at Unrelated Corporation."
	c.Evidence = append(c.Evidence, e)
	before, _ := json.Marshal(c)
	got, err := Run(context.Background(), c, extract.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.NameClaims) != 1 || got.NameClaims[0].Name != "Sari Utami" || got.Candidates[0].DisplayName != "Awan" || got.Candidates[0].Decision != "PENDING" {
		t.Fatalf("name assertion or identity changed: %+v", got)
	}
	if got.Candidates[0].Company != "Orbit Records" || !strings.EqualFold(got.Candidates[0].Role, "singer") {
		t.Fatalf("same-source claims lost: %+v", got.Candidates[0])
	}
	for _, relation := range got.Relations {
		if relation.Organization == "Unrelated Corporation" {
			t.Fatal("alias escaped its document")
		}
	}
	second, err := Run(context.Background(), got, extract.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.NameClaims, second.NameClaims) || !reflect.DeepEqual(got.Relations, second.Relations) {
		t.Fatal("replay changed name or relation claims")
	}
	after, _ := json.Marshal(c)
	if string(before) != string(after) {
		t.Fatal("input mutated")
	}
}

func TestPublicNamesRejectForgery(t *testing.T) {
	got, err := Run(context.Background(), nameFixture(), extract.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*domain.NameClaim){
		func(n *domain.NameClaim) { n.Name = "Invented Person" },
		func(n *domain.NameClaim) { n.CandidateID = "elsewhere" },
		func(n *domain.NameClaim) { n.Alias = "AnotherAlias" },
		func(n *domain.NameClaim) { n.EvidenceID = "missing" },
		func(n *domain.NameClaim) { n.Quote = "Sari Utami is Awan" },
		func(n *domain.NameClaim) { n.Status = "VERIFIED" },
		func(n *domain.NameClaim) { n.Kind = "legal_identity" },
	} {
		bad := got
		bad.NameClaims = append([]domain.NameClaim(nil), got.NameClaims...)
		mutate(&bad.NameClaims[0])
		if _, err := evidence.Normalize(bad); err == nil {
			t.Fatal("forged public name accepted")
		}
	}
}

func TestFanpageCannotAcquireCopiedBiographyName(t *testing.T) {
	for _, title := range []string{"Awan fan page", "Awan fanpage", "Awan parody", "Awan akun penggemar"} {
		c := nameFixture()
		e := c.Evidence[0]
		e.ID, e.Type, e.Value = "title", "page_title", title
		c.Evidence = append(c.Evidence, e)
		got, err := Run(context.Background(), c, extract.Config{})
		if err != nil {
			t.Fatal(err)
		}
		if len(got.NameClaims) > 0 || got.Candidates[0].Company != "" || got.Candidates[0].Role != "" {
			t.Fatalf("copied biography adopted: %+v", got)
		}
	}
}

func TestConflictingPublicNamesRemainSeparate(t *testing.T) {
	c := nameFixture()
	e := c.Evidence[0]
	e.ID, e.Value = "conflict", "Nila Putri, known as Awan, is a writer."
	c.Evidence = append(c.Evidence, e)
	got, err := Run(context.Background(), c, extract.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.NameClaims) != 2 || got.NameClaims[0].Name == got.NameClaims[1].Name || got.Candidates[0].DisplayName != "Awan" {
		t.Fatal("different names silently collapsed")
	}
}

func TestMergedEvidenceKeepsNameProvenance(t *testing.T) {
	c := nameFixture()
	got, err := Run(context.Background(), c, extract.Config{})
	if err != nil {
		t.Fatal(err)
	}
	e := got.Evidence[0]
	e.ID = "duplicate"
	got.Evidence = append(got.Evidence, e)
	got.NameClaims[0].EvidenceID = e.ID
	got, err = evidence.Normalize(got)
	if err != nil {
		t.Fatal(err)
	}
	if got.NameClaims[0].EvidenceID != "doc" {
		t.Fatal("merged evidence reference lost")
	}
}

func TestReplayAcceptsPriorSummaryLabelsAndLiteralPostnominals(t *testing.T) {
	c := nameFixture()
	c.Evidence[0].Value = "Sari Utami KBE, known as Awan, is a musisi."
	got, err := Run(context.Background(), c, extract.Config{})
	if err != nil {
		t.Fatal(err)
	}
	got.NameClaims[0].Name = "Sari Utami KBE"
	for i := range got.Candidates[0].RoleClaims {
		got.Candidates[0].RoleClaims[i].Value = "musisi"
	}
	replayed, err := Run(context.Background(), got, extract.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if replayed.NameClaims[0].Name != "Sari Utami" || replayed.Candidates[0].Role != "Musician" {
		t.Fatal("derived name or role was not refreshed")
	}
}
