package enrich

import (
	"context"
	"reflect"
	"testing"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/evidence"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/extract"
)

func TestArticleExpansionIsReplayableAndCannotBorrowAnotherCandidateTitle(t *testing.T) {
	c := fixture("Mira Rosa Solis (born 1985) is a software engineer.")
	c.Target.Name = "Mira Solis"
	c.Candidates[0].DisplayName = "Mira Solis"
	c.Candidates[0].ProfileURL = "https://en.wikipedia.org/wiki/Mira_Solis"
	c.Evidence[0].Type = "page_description"
	title := c.Evidence[0]
	title.ID, title.Type, title.Value = "title", "page_title", "Mira Solis - Wikipedia"
	c.Evidence = append(c.Evidence, title)
	got, err := Run(context.Background(), c, extract.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Relations) != 1 || got.Relations[0].Method != extract.ArticleBindingVersion || got.Candidates[0].Role == "" || got.Candidates[0].Decision != "PENDING" || len(got.NameClaims) != 0 {
		t.Fatalf("unexpected article binding: %+v", got)
	}
	again, err := Run(context.Background(), got, extract.Config{})
	if err != nil || !reflect.DeepEqual(again.Relations, got.Relations) {
		t.Fatalf("unstable replay: %v", err)
	}
	got.Relations[0].SubjectEvidenceID = "missing"
	if _, err := evidence.Normalize(got); err == nil {
		t.Fatal("forged heading reference accepted")
	}
}
