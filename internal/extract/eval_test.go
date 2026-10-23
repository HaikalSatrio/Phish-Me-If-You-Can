package extract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type evalCorpus struct {
	Version          string     `json:"version"`
	MinimumPrecision float64    `json:"minimum_precision"`
	MinimumRecall    float64    `json:"minimum_recall"`
	Cases            []evalCase `json:"cases"`
}

type evalCase struct {
	ID    string `json:"id"`
	Input Input  `json:"input"`
	Want  string `json:"want"`
}

func TestCompanyEvaluation(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "company_eval.json"))
	if err != nil {
		t.Fatal(err)
	}
	var corpus evalCorpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.Version != "company-eval-v1" || len(corpus.Cases) < 20 {
		t.Fatalf("invalid evaluation corpus: %+v", corpus)
	}
	truePositive, falsePositive, falseNegative, trueNegative := 0, 0, 0, 0
	for _, test := range corpus.Cases {
		got, ok := ExtractCompany(test.Input)
		predicted := ""
		if ok {
			predicted = got.Value
			if got.Rule == "" || got.Quote == "" || got.QuoteType == "" {
				t.Errorf("%s: missing audit trace: %+v", test.ID, got)
			}
		}
		want := strings.Join(strings.Fields(test.Want), " ")
		switch {
		case want != "" && predicted != "" && sameCompany(want, predicted):
			truePositive++
		case want == "" && predicted == "":
			trueNegative++
		case want == "":
			falsePositive++
			t.Errorf("%s: unexpected inference %q", test.ID, predicted)
		case predicted == "":
			falseNegative++
			t.Errorf("%s: missing inference, want %q", test.ID, want)
		default:
			falsePositive++
			falseNegative++
			t.Errorf("%s: got %q, want %q", test.ID, predicted, want)
		}
	}
	precision := ratio(truePositive, truePositive+falsePositive)
	recall := ratio(truePositive, truePositive+falseNegative)
	t.Logf("cases=%d tp=%d fp=%d fn=%d tn=%d precision=%.3f recall=%.3f", len(corpus.Cases), truePositive, falsePositive, falseNegative, trueNegative, precision, recall)
	if precision < corpus.MinimumPrecision {
		t.Fatalf("precision %.3f below %.3f", precision, corpus.MinimumPrecision)
	}
	if recall < corpus.MinimumRecall {
		t.Fatalf("recall %.3f below %.3f", recall, corpus.MinimumRecall)
	}
}

func sameCompany(a, b string) bool {
	return strings.EqualFold(strings.Join(strings.Fields(a), " "), strings.Join(strings.Fields(b), " "))
}

func ratio(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}
