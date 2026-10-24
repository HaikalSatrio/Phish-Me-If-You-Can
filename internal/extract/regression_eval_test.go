package extract

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
)

type evalCounts struct {
	TP        int     `json:"true_positive"`
	FP        int     `json:"false_positive"`
	FN        int     `json:"false_negative"`
	Precision float64 `json:"precision"`
	Recall    float64 `json:"recall"`
}

func (c *evalCounts) add(want, got map[string]bool) {
	for v := range got {
		if want[v] {
			c.TP++
		} else {
			c.FP++
		}
	}
	for v := range want {
		if !got[v] {
			c.FN++
		}
	}
	if c.TP+c.FP > 0 {
		c.Precision = float64(c.TP) / float64(c.TP+c.FP)
	}
	if c.TP+c.FN > 0 {
		c.Recall = float64(c.TP) / float64(c.TP+c.FN)
	}
}

func evalValues(relations []domain.Relation, field string) map[string]bool {
	out := map[string]bool{}
	for _, r := range relations {
		value := ""
		switch field {
		case "role":
			value = r.Role
		case "company":
			if r.Kind == "employment" {
				value = r.Organization
			}
		default:
			data, _ := json.Marshal([]any{r.Subject, r.Kind, r.Role, r.Organization, r.StartYear, r.EndYear, r.Temporal})
			value = string(data)
		}
		if value != "" {
			out[strings.ToLower(value)] = true
		}
	}
	return out
}

// Set PMIYC_EVAL_MODEL to run the same labeled cases against an installed local model.
func TestRelationRegressionEvaluation(t *testing.T) {
	var corpus struct {
		AsOf        string `json:"as_of"`
		Description string `json:"description"`
		Cases       []struct {
			ID, Subject, Type, Text string
			Expected                []domain.Relation `json:"expected"`
		} `json:"cases"`
	}
	data, err := os.ReadFile("testdata/relation-regression.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	asOf, err := time.Parse(time.RFC3339, corpus.AsOf)
	if err != nil {
		t.Fatal(err)
	}
	type modeReport struct {
		Role, Company, Relationship                              evalCounts
		Cases, Abstentions, WrongSubject, PeriodErrors, Failures int
		DurationMS                                               int64
	}
	type caseReport struct {
		ID        string
		Mode      string
		Relations []domain.Relation
		Trace     domain.ExtractionRun
	}
	report := struct {
		Description string
		Version     string
		AsOf        string
		Metrics     map[string]*modeReport
		Results     []caseReport
	}{Description: corpus.Description, Version: RelationVersion, AsOf: corpus.AsOf, Metrics: map[string]*modeReport{}}
	modes := []string{"rules"}
	model := os.Getenv("PMIYC_EVAL_MODEL")
	if model != "" {
		modes = append(modes, "local_model")
	}
	for _, mode := range modes {
		stats := &modeReport{}
		report.Metrics[mode] = stats
		for _, tc := range corpus.Cases {
			for i := range tc.Expected {
				tc.Expected[i].Subject = tc.Subject
			}
			docs := []Document{{ID: tc.ID, Type: tc.Type, Text: tc.Text}}
			var got []domain.Relation
			var trace domain.ExtractionRun
			started := time.Now()
			if mode == "rules" {
				got = Relations(tc.Subject, docs, asOf)
			} else {
				got, trace = LocalRelations(context.Background(), Config{Mode: "hybrid", Model: model}, tc.Subject, docs, asOf)
			}
			stats.DurationMS += time.Since(started).Milliseconds()
			stats.Cases++
			if len(got) == 0 {
				stats.Abstentions++
			}
			if trace.Status == "FAILED" {
				stats.Failures++
			}
			stats.Role.add(evalValues(tc.Expected, "role"), evalValues(got, "role"))
			stats.Company.add(evalValues(tc.Expected, "company"), evalValues(got, "company"))
			beforeFP, beforeFN := stats.Relationship.FP, stats.Relationship.FN
			stats.Relationship.add(evalValues(tc.Expected, "relationship"), evalValues(got, "relationship"))
			for _, r := range got {
				if r.Subject != tc.Subject {
					stats.WrongSubject++
				}
				for _, want := range tc.Expected {
					if r.Kind == want.Kind && r.Role == want.Role && r.Organization == want.Organization && (r.StartYear != want.StartYear || r.EndYear != want.EndYear || r.Temporal != want.Temporal) {
						stats.PeriodErrors++
						break
					}
				}
			}
			report.Results = append(report.Results, caseReport{ID: tc.ID, Mode: mode, Relations: got, Trace: trace})
			if mode == "rules" && (stats.Relationship.FP != beforeFP || stats.Relationship.FN != beforeFN) {
				t.Errorf("%s: got %+v; want %+v", tc.ID, got, tc.Expected)
			}
			t.Logf("%s %s: relations=%d model_status=%s accepted=%d rejected=%d", mode, tc.ID, len(got), trace.Status, trace.Accepted, trace.Rejected)
		}
		encoded, _ := json.Marshal(stats)
		t.Logf("%s known-regression metrics: %s", mode, encoded)
	}
	if path := os.Getenv("PMIYC_EVAL_OUT"); path != "" {
		encoded, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			t.Fatal(err)
		}
		_, writeErr := f.Write(encoded)
		closeErr := f.Close()
		if writeErr != nil {
			t.Fatal(writeErr)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
	}
}
