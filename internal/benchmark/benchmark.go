package benchmark

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/collector"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/enrich"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/extract"
)

type Corpus struct {
	Version     string `json:"version"`
	Description string `json:"description"`
	ObservedAt  string `json:"observed_at"`
	Cases       []Case `json:"cases"`
}

type Case struct {
	ID       string   `json:"id"`
	Category string   `json:"category"`
	Name     string   `json:"name"`
	Results  []Result `json:"results"`
}

type Result struct {
	ID        string             `json:"id"`
	URL       string             `json:"url"`
	Title     string             `json:"title"`
	Snippet   string             `json:"snippet"`
	Documents []extract.Document `json:"documents,omitempty"`
	Keep      bool               `json:"keep"`
	Expected  []Claim            `json:"expected"`
}

type Claim struct {
	CandidateID  string `json:"candidate_id,omitempty"`
	Subject      string `json:"subject"`
	Kind         string `json:"kind"`
	Role         string `json:"role,omitempty"`
	Organization string `json:"organization,omitempty"`
	Temporal     string `json:"temporal"`
	StartYear    int    `json:"start_year,omitempty"`
	EndYear      int    `json:"end_year,omitempty"`
}

type Counts struct {
	TP        int      `json:"true_positive"`
	FP        int      `json:"false_positive"`
	FN        int      `json:"false_negative"`
	Precision *float64 `json:"precision"`
	Recall    *float64 `json:"recall"`
}

func (c *Counts) add(want, got map[string]bool) {
	for k := range got {
		if want[k] {
			c.TP++
		} else {
			c.FP++
		}
	}
	for k := range want {
		if !got[k] {
			c.FN++
		}
	}
	c.Precision, c.Recall = ratio(c.TP, c.TP+c.FP), ratio(c.TP, c.TP+c.FN)
}

func ratio(n, d int) *float64 {
	if d == 0 {
		return nil
	}
	v := float64(n) / float64(d)
	return &v
}

type Metrics struct {
	Cases                    int            `json:"cases"`
	Completed                int            `json:"completed"`
	Claims                   Counts         `json:"claims"`
	Role                     Counts         `json:"role"`
	Company                  Counts         `json:"company"`
	Education                Counts         `json:"education"`
	NegativeCases            int            `json:"negative_cases"`
	CorrectAbstentions       int            `json:"correct_abstentions"`
	MissedPositiveCases      int            `json:"missed_positive_cases"`
	UnrequestedConfirmations int            `json:"unrequested_confirmations"`
	ModelSuccess             int            `json:"model_success"`
	ModelFailed              int            `json:"model_failed"`
	ModelSkipped             int            `json:"model_skipped"`
	ModelStatuses            map[string]int `json:"model_status_counts"`
	RejectedProposals        int            `json:"rejected_proposals"`
	DurationMS               int64          `json:"duration_ms"`
}

type FilterDecision struct {
	CandidateID string `json:"candidate_id"`
	Expected    bool   `json:"expected"`
	Kept        bool   `json:"kept"`
}

type Run struct {
	CaseID       string           `json:"case_id"`
	Category     string           `json:"category"`
	Mode         string           `json:"mode"`
	InputSHA256  string           `json:"input_sha256"`
	Status       string           `json:"status"`
	Error        string           `json:"error,omitempty"`
	DurationMS   int64            `json:"duration_ms"`
	Filters      []FilterDecision `json:"filters"`
	Missing      []Claim          `json:"missing"`
	Unexpected   []Claim          `json:"unexpected"`
	FailureCodes []string         `json:"failure_codes"`
	Output       domain.Case      `json:"output"`
}

type Report struct {
	Version          string              `json:"version"`
	CreatedAt        string              `json:"created_at"`
	CorpusSHA256     string              `json:"corpus_sha256"`
	Corpus           Corpus              `json:"corpus"`
	ExtractorVersion string              `json:"extractor_version"`
	GoVersion        string              `json:"go_version"`
	Revision         string              `json:"revision,omitempty"`
	Modified         bool                `json:"modified"`
	ModelConfig      extract.Config      `json:"model_config"`
	Limitations      []string            `json:"limitations"`
	CandidateFilter  Counts              `json:"candidate_filter"`
	Metrics          map[string]*Metrics `json:"metrics"`
	Failures         map[string]int      `json:"failures"`
	Runs             []Run               `json:"runs"`
}

func Validate(c Corpus) error {
	if c.Version != "1" || strings.TrimSpace(c.Description) == "" || len(c.Cases) == 0 || len(c.Cases) > 200 {
		return fmt.Errorf("corpus requires version 1, description, and 1-200 cases")
	}
	if _, err := time.Parse(time.RFC3339, c.ObservedAt); err != nil {
		return err
	}
	ids := map[string]bool{}
	for _, tc := range c.Cases {
		if tc.ID == "" || ids[tc.ID] || tc.Name == "" || tc.Category == "" || len(tc.Results) == 0 || len(tc.Results) > 20 {
			return fmt.Errorf("invalid or duplicate benchmark case %q", tc.ID)
		}
		ids[tc.ID] = true
		results := map[string]bool{}
		for _, r := range tc.Results {
			u, err := url.Parse(r.URL)
			if err != nil || u.Scheme != "https" || u.Host == "" || r.ID == "" || results[r.ID] || r.Title == "" || r.Expected == nil {
				return fmt.Errorf("case %s: invalid result or missing expected array", tc.ID)
			}
			results[r.ID] = true
			if !r.Keep && len(r.Expected) > 0 {
				return fmt.Errorf("case %s: excluded candidate cannot have expected claims", tc.ID)
			}
			for _, claim := range r.Expected {
				if claim.CandidateID != "" || claim.Subject == "" || claim.Kind == "" || claim.Temporal == "" || claim.Role == "" && claim.Organization == "" {
					return fmt.Errorf("case %s: invalid expected claim", tc.ID)
				}
			}
		}
	}
	return nil
}

func frozenInput(tc Case, at string) (domain.Case, []FilterDecision, []Claim) {
	c := domain.Case{SchemaVersion: "1", Target: domain.Target{ID: tc.ID, Name: tc.Name}, Search: &domain.SearchSession{Provider: "web", Name: tc.Name}}
	filters, expected := []FilterDecision{}, []Claim{}
	for _, r := range tc.Results {
		kept := collector.AcceptIndexedPerson(collector.Query{Name: tc.Name}, r.URL, r.Title, r.Snippet)
		filters = append(filters, FilterDecision{r.ID, r.Keep, kept})
		for _, claim := range r.Expected {
			claim.CandidateID = r.ID
			expected = append(expected, claim)
		}
		if !kept {
			continue
		}
		c.Candidates = append(c.Candidates, domain.Candidate{ID: r.ID, Label: r.Title, ProfileURL: r.URL, RequiresReview: true, Decision: "PENDING"})
		docs := append([]extract.Document{{ID: "title", Type: "search_title", Text: r.Title}, {ID: "snippet", Type: "search_snippet", Text: r.Snippet}}, r.Documents...)
		for i, d := range docs {
			if d.Text == "" {
				continue
			}
			c.Evidence = append(c.Evidence, domain.Evidence{ID: fmt.Sprintf("%s-doc-%d", r.ID, i), CandidateID: r.ID, Type: d.Type, Value: d.Text, Status: "OBSERVED", Confidence: 1, Sources: []domain.Source{{URL: r.URL, Collector: "synthetic-benchmark", ObservedAt: at}}})
		}
	}
	return c, filters, expected
}

func claimOf(r domain.Relation) Claim {
	return Claim{r.CandidateID, r.Subject, r.Kind, r.Role, r.Organization, r.Temporal, r.StartYear, r.EndYear}
}
func key(c Claim) string { data, _ := json.Marshal(c); return strings.ToLower(string(data)) }

func claimsMap(claims []Claim) map[string]Claim {
	out := map[string]Claim{}
	for _, c := range claims {
		out[key(c)] = c
	}
	return out
}
func values(claims []Claim, field string) map[string]bool {
	out := map[string]bool{}
	for _, c := range claims {
		v := ""
		switch field {
		case "role":
			v = c.Role
		case "company":
			if c.Kind == "employment" {
				v = c.Organization
			}
		case "education":
			if c.Kind == "education" {
				v = c.Organization
			}
		default:
			out[key(c)] = true
			continue
		}
		if v != "" {
			out[strings.ToLower(c.CandidateID+"\x00"+c.Subject+"\x00"+v)] = true
		}
	}
	return out
}

// Execute measures frozen result filtering and extraction, not search-engine recall.
func Execute(ctx context.Context, corpus Corpus, cfg extract.Config, progress func(string)) (Report, error) {
	if err := Validate(corpus); err != nil {
		return Report{}, err
	}
	if cfg.Model != "" {
		cfg.Mode = "hybrid"
	}
	if err := cfg.Validate(); err != nil {
		return Report{}, err
	}
	data, _ := json.Marshal(corpus)
	report := Report{Version: "1", CreatedAt: time.Now().UTC().Format(time.RFC3339), CorpusSHA256: fmt.Sprintf("%x", sha256.Sum256(data)), Corpus: corpus, ExtractorVersion: extract.RelationVersion, ModelConfig: cfg.Effective(), Metrics: map[string]*Metrics{}, Failures: map[string]int{}, Runs: []Run{}, Limitations: []string{
		"Synthetic exploratory corpus; not independently labeled, not a real-web or held-out accuracy estimate.",
		"Candidate metrics measure filtering of supplied index results; provider availability, ranking, budgets and live retrieval recall are not measured.",
		"Claims include unverified model proposals. Identity and relationship review are not performed; production summaries may differ.",
		"Strict claim matching includes candidate, subject, kind, literal values and period. Equivalent paraphrases may count as errors.",
		"Model failures and skips remain in totals; compare coverage before interpreting hybrid quality. Runtime uses the current clock and records it.",
	}}
	report.GoVersion = runtime.Version()
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				report.Revision = setting.Value
			}
			if setting.Key == "vcs.modified" {
				report.Modified = setting.Value == "true"
			}
		}
	}
	modes := []string{"rules"}
	if cfg.Model != "" {
		modes = append(modes, "hybrid")
	}
	for _, mode := range modes {
		stats := &Metrics{ModelStatuses: map[string]int{}}
		report.Metrics[mode] = stats
		for _, tc := range corpus.Cases {
			input, filters, want := frozenInput(tc, corpus.ObservedAt)
			encoded, _ := json.Marshal(input)
			run := Run{CaseID: tc.ID, Category: tc.Category, Mode: mode, InputSHA256: fmt.Sprintf("%x", sha256.Sum256(encoded)), Filters: filters, Status: "COMPLETE", Missing: []Claim{}, Unexpected: []Claim{}, FailureCodes: []string{}}
			stats.Cases++
			if mode == "rules" {
				wanted, kept := map[string]bool{}, map[string]bool{}
				for _, f := range filters {
					if f.Expected {
						wanted[f.CandidateID] = true
					}
					if f.Kept {
						kept[f.CandidateID] = true
					}
				}
				report.CandidateFilter.add(wanted, kept)
			}
			failure := map[string]bool{}
			for _, f := range filters {
				if f.Expected && !f.Kept {
					failure["candidate_filtered"] = true
				}
				if !f.Expected && f.Kept {
					failure["irrelevant_candidate"] = true
				}
			}
			active := extract.Config{}
			if mode == "hybrid" {
				active = cfg
			}
			started := time.Now()
			var err error
			if ctx.Err() != nil {
				err = ctx.Err()
			} else {
				run.Output, err = enrich.Run(ctx, input, active)
			}
			run.DurationMS = time.Since(started).Milliseconds()
			stats.DurationMS += run.DurationMS
			got := []Claim{}
			if err != nil {
				run.Status, run.Error = "ERROR", err.Error()
				failure["pipeline_error"] = true
			} else {
				stats.Completed++
				for _, r := range run.Output.Relations {
					got = append(got, claimOf(r))
				}
				for _, p := range run.Output.Candidates {
					if p.Decision == "CONFIRM" {
						stats.UnrequestedConfirmations++
						failure["unrequested_confirmation"] = true
					}
				}
				for _, trace := range run.Output.Extraction {
					if trace.Mode != "hybrid" {
						continue
					}
					stats.RejectedProposals += trace.Rejected
					stats.ModelStatuses[trace.Status]++
					if trace.Rejected > 0 {
						failure["model_proposals_rejected"] = true
					}
					switch trace.Status {
					case "SUCCESS", "EMPTY", "PARTIAL", "REJECTED_ALL":
						stats.ModelSuccess++
					case "FAILED":
						stats.ModelFailed++
						failure["model_failed"] = true
						run.Status = "DEGRADED"
					case "SKIPPED":
						stats.ModelSkipped++
						failure["model_skipped"] = true
					}
				}
			}
			stats.Claims.add(values(want, "claims"), values(got, "claims"))
			stats.Role.add(values(want, "role"), values(got, "role"))
			stats.Company.add(values(want, "company"), values(got, "company"))
			stats.Education.add(values(want, "education"), values(got, "education"))
			wantMap, gotMap := claimsMap(want), claimsMap(got)
			for k, c := range wantMap {
				if _, ok := gotMap[k]; !ok {
					run.Missing = append(run.Missing, c)
				}
			}
			for k, c := range gotMap {
				if _, ok := wantMap[k]; !ok {
					run.Unexpected = append(run.Unexpected, c)
				}
			}
			sort.Slice(run.Missing, func(i, j int) bool { return key(run.Missing[i]) < key(run.Missing[j]) })
			sort.Slice(run.Unexpected, func(i, j int) bool { return key(run.Unexpected[i]) < key(run.Unexpected[j]) })
			if len(run.Missing) > 0 {
				failure["missing_claim"] = true
			}
			if len(run.Unexpected) > 0 {
				failure["unexpected_claim"] = true
			}
			if len(want) == 0 {
				stats.NegativeCases++
				if len(got) == 0 && err == nil {
					stats.CorrectAbstentions++
				}
			} else if len(got) == 0 {
				stats.MissedPositiveCases++
			}
			for code := range failure {
				run.FailureCodes = append(run.FailureCodes, code)
				report.Failures[mode+":"+code]++
			}
			sort.Strings(run.FailureCodes)
			report.Runs = append(report.Runs, run)
			if progress != nil {
				progress(fmt.Sprintf("%s %s: %s, missing=%d unexpected=%d", mode, tc.ID, run.Status, len(run.Missing), len(run.Unexpected)))
			}
		}
	}
	return report, nil
}
