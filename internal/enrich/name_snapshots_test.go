package enrich

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/extract"
)

// These excerpts are regression data, not an independently labeled accuracy set.
func TestLivePublicNameSnapshots(t *testing.T) {
	data, err := os.ReadFile("../../fixtures/public-name-snapshots-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	type expectedName struct {
		CandidateID string `json:"candidate_id"`
		Alias       string `json:"alias"`
		Name        string `json:"name"`
	}
	var corpus struct {
		Cases []struct {
			Query    string         `json:"query"`
			Expected []expectedName `json:"expected"`
			Input    domain.Case    `json:"input"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) != 17 {
		t.Fatal("incomplete public name regression corpus")
	}
	for _, tc := range corpus.Cases {
		t.Run(tc.Query, func(t *testing.T) {
			got, err := Run(context.Background(), tc.Input, extract.Config{})
			if err != nil {
				t.Fatal(err)
			}
			want, actual := map[expectedName]bool{}, map[expectedName]bool{}
			for _, n := range tc.Expected {
				want[n] = true
			}
			for _, n := range got.NameClaims {
				actual[expectedName{n.CandidateID, n.Alias, n.Name}] = true
			}
			if !reflect.DeepEqual(want, actual) {
				t.Fatalf("source assertions changed\nwant: %+v\ngot: %+v", want, actual)
			}
			for _, p := range got.Candidates {
				if p.Decision != "PENDING" {
					t.Fatal("source name claim confirmed an identity")
				}
			}
		})
	}
}
