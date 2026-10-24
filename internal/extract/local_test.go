package extract

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLocalGroundingAndTransport(t *testing.T) {
	quote := "Ayu Lestari works as Engineer at Orbit Labs."
	valid := proposedRelation{Subject: "Ayu Lestari", Kind: "employment", Role: "Engineer", Organization: "Orbit Labs", Temporal: "present_assertion", EvidenceID: "s1"}
	for _, tc := range []struct {
		name   string
		mutate func(*proposedRelation)
		want   int
	}{
		{"valid", func(*proposedRelation) {}, 1},
		{"invented-employer", func(p *proposedRelation) { p.Organization = "Contoso" }, 0},
		{"invented-year", func(p *proposedRelation) { p.StartYear = 2020 }, 0},
		{"wrong-subject", func(p *proposedRelation) { p.Subject = "Budi Santoso" }, 0},
		{"unmapped-passage", func(p *proposedRelation) { p.EvidenceID = "doc" }, 0},
		{"foreign-evidence", func(p *proposedRelation) { p.EvidenceID = "other" }, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := valid
			tc.mutate(&p)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					w.Write([]byte(`{"models":[]}`))
					return
				}
				if r.URL.Path != "/api/chat" || r.Method != "POST" {
					t.Error("unexpected endpoint")
				}
				var req struct {
					Stream   bool                `json:"stream"`
					Format   map[string]any      `json:"format"`
					Messages []map[string]string `json:"messages"`
				}
				if json.NewDecoder(r.Body).Decode(&req) != nil || req.Stream || req.Format["type"] != "object" || len(req.Messages) != 2 {
					t.Error("missing bounded structured request")
				}
				content, _ := json.Marshal(map[string]any{"relations": []proposedRelation{p}})
				json.NewEncoder(w).Encode(map[string]any{"done": true, "message": map[string]string{"content": string(content)}})
			}))
			defer srv.Close()
			got, run := LocalRelations(context.Background(), Config{Mode: "hybrid", Model: "fixture", URL: srv.URL}, "Ayu Lestari", []Document{{ID: "doc", Text: quote}}, time.Now())
			if len(got) != tc.want || run.Rejected != 1-tc.want || run.InputSHA256 == "" || run.OutputSHA256 == "" {
				t.Fatalf("got %+v trace %+v", got, run)
			}
			if tc.want == 1 && (got[0].EvidenceID != "doc" || !strings.Contains(quote, got[0].Quote) || len(run.Passages) != 1) {
				t.Fatal("lost passage provenance")
			}
		})
	}
}

func TestLocalRejectsRemoteOrigins(t *testing.T) {
	for _, model := range []string{"example:cloud", "example:120b-cloud", "https://example.org/model"} {
		if err := (Config{Mode: "hybrid", Model: model}).Validate(); err == nil {
			t.Errorf("accepted remote model %s", model)
		}
	}
	for _, raw := range []string{"http://example.org", "http://192.168.1.1", "http://127.0.0.1@evil.example", "http://localhost/path", "http://127.0.0.1?proxy=x", "https://127.0.0.1"} {
		if err := (Config{URL: raw}).Validate(); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
}

func TestLocalFailureModes(t *testing.T) {
	for _, mode := range []string{"redirect", "bad-json", "truncated", "oversized", "unknown-field", "extra-json", "missing-array", "missing-field", "null-field", "generated-quote"} {
		t.Run(mode, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					w.Write([]byte(`{"models":[]}`))
					return
				}
				switch mode {
				case "redirect":
					w.Header().Set("Location", "https://example.org")
					w.WriteHeader(302)
				case "bad-json":
					w.Write([]byte("no"))
				case "truncated":
					w.Write([]byte(`{"done":false,"message":{"content":"{\"relations\":[]}"}}`))
				case "oversized":
					w.Write([]byte(strings.Repeat("x", (256<<10)+1)))
				default:
					content := `{"relations":[],"unwanted":true}`
					if mode == "extra-json" {
						content = `{"relations":[]} {}`
					}
					if mode == "missing-array" {
						content = `{}`
					}
					if mode == "missing-field" {
						content = `{"relations":[{"subject":"Ayu"}]}`
					}
					if mode == "null-field" {
						content = `{"relations":[{"subject":null,"kind":"occupation","organization":"","role":"Engineer","start_year":0,"end_year":0,"temporal":"unknown","evidence_id":"doc","quote":"Ayu is Engineer"}]}`
					}
					if mode == "generated-quote" {
						content = `{"relations":[{"subject":"Ayu","kind":"occupation","organization":"","role":"Engineer","start_year":0,"end_year":0,"temporal":"unknown","evidence_id":"s1","quote":"Ayu is Engineer"}]}`
					}
					json.NewEncoder(w).Encode(map[string]any{"done": true, "message": map[string]string{"content": content}})
				}
			}))
			defer srv.Close()
			got, run := LocalRelations(context.Background(), Config{Mode: "hybrid", Model: "fixture", URL: srv.URL}, "Ayu", nil, time.Now())
			if len(got) != 0 || run.Status != "FAILED" {
				t.Fatalf("failure hidden: %+v", run)
			}
		})
	}
}

func TestLocalCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, trace := LocalRelations(ctx, Config{Mode: "hybrid", Model: "fixture"}, "Ayu", nil, time.Now())
	if len(got) != 0 || trace.Status != "FAILED" {
		t.Fatalf("cancelled extraction succeeded: %+v", trace)
	}
}
