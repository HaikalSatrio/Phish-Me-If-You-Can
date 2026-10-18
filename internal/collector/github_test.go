package collector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSearchAndProfile(t *testing.T) {
	requests := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing authentication")
		}
		if r.URL.Path == "/search/users" {
			if !strings.Contains(r.URL.Query().Get("q"), "in:fullname type:user") {
				t.Error("unexpected query")
			}
			w.Write([]byte(`{"total_count":1,"items":[{"login":"anton-test","type":"User"}]}`))
			return
		}
		w.Write([]byte(`{"id":42,"login":"anton-test","name":"Anton Test","type":"User","email":null}`))
	}))
	defer s.Close()
	g := NewGitHub(time.Second, "test-token")
	g.base = s.URL
	r := g.Collect(context.Background(), Query{Name: "Anton Test", Limit: 5, Budget: 6})
	if len(r.Profiles) != 1 || r.Profiles[0].ID != 42 || r.Requests != 2 || requests != 2 || r.Incomplete {
		t.Fatalf("unexpected result: %+v", r)
	}
}

func TestFailuresAndBudgets(t *testing.T) {
	for _, tt := range []struct {
		name, body, status string
		code, budget       int
		username           string
	}{
		{"rate limit", `{"message":"secret body must not be saved"}`, "BLOCKED", 429, 5, "anton"},
		{"not found", `{}`, "EMPTY", 404, 5, "anton"},
		{"bad JSON", `{`, "FAILED", 200, 5, "anton"},
		{"wrong account", `{"id":4,"login":"another","type":"User"}`, "FAILED", 200, 5, "anton"},
		{"budget", `{"total_count":2,"items":[{"login":"anton","type":"User"}]}`, "FAILED", 200, 1, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tt.code); w.Write([]byte(tt.body)) }))
			defer s.Close()
			g := NewGitHub(time.Second, "")
			g.base = s.URL
			r := g.Collect(context.Background(), Query{Name: "Anton", Username: tt.username, Limit: 5, Budget: tt.budget})
			if r.Status != tt.status || r.Requests > tt.budget {
				t.Fatalf("unexpected result: %+v", r)
			}
			for _, e := range r.Events {
				if strings.Contains(e.Detail, "secret") {
					t.Fatal("response body leaked")
				}
			}
		})
	}
}

func TestCancellationAndRedirect(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/elsewhere", http.StatusFound) }))
	defer s.Close()
	g := NewGitHub(time.Second, "")
	g.base = s.URL
	r := g.Collect(context.Background(), Query{Username: "anton", Limit: 1, Budget: 1})
	if r.Status != "FAILED" {
		t.Fatal("redirect followed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r = g.Collect(ctx, Query{Username: "anton", Limit: 1, Budget: 1})
	if r.Requests != 0 || !r.Incomplete {
		t.Fatal("cancelled request attempted")
	}
}
