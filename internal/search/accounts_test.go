package search

import (
	"context"
	"reflect"
	"testing"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/collector"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/storage"
)

func TestAccountCluesSurviveRefinementWithoutMutatingPriorSnapshot(t *testing.T) {
	var cfg Config
	if err := storage.Read("../../configs/collector-v1.json", &cfg); err != nil {
		t.Fatal(err)
	}
	f := &fakeCollector{result: collector.Result{Status: "EMPTY", Requests: 1}}
	clues := []domain.AccountClue{{Platform: "instagram", Username: "@ayu_ig"}}
	first, err := Run(context.Background(), f, Input{Name: "Ayu", Accounts: clues}, cfg, nil)
	if err != nil || first.Search.Provider != "web" || first.Search.Accounts[0].Username != "ayu_ig" {
		t.Fatalf("initial clues failed: %+v %v", first.Search, err)
	}
	second, err := Run(context.Background(), f, Input{Accounts: []domain.AccountClue{{Platform: "github", Username: "ayu-dev"}}}, cfg, &first)
	if err != nil || len(second.Search.Accounts) != 2 || len(first.Search.Accounts) != 1 || !reflect.DeepEqual(clues, []domain.AccountClue{{Platform: "instagram", Username: "@ayu_ig"}}) {
		t.Fatalf("clues lost or prior snapshot mutated: %v", err)
	}
	before := f.calls
	if _, err := Run(context.Background(), f, Input{Provider: "github", Accounts: clues}, cfg, nil); err == nil || f.calls != before {
		t.Fatal("unsupported source silently ignored accounts")
	}
	only, err := Run(context.Background(), f, Input{Accounts: clues}, cfg, nil)
	if err != nil || only.Target.Name != "instagram:ayu_ig" || only.Search.Name != "" {
		t.Fatalf("account-only search needs an invented person name: %+v %v", only, err)
	}
}
