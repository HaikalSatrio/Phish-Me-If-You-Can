package collector

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestWikipediaBoundedReferenceFallback(t *testing.T) {
	w := NewWikipedia(time.Second)
	w.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		q := req.URL.Query()
		if q.Get("titles") != "Mira" || q.Get("maxlag") != "5" || q.Get("redirects") != "1" || !strings.Contains(req.UserAgent(), "github.com/") {
			t.Fatalf("invalid reference request: %s", req.URL)
		}
		body := `{"query":{"pages":[{"ns":0,"title":"Mira Solis","fullurl":"https://en.wikipedia.org/wiki/Mira_Solis","extract":"Mira Solis, known as Mira, is a singer."}]}}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}, Request: req}, nil
	})}
	r := w.Collect(context.Background(), Query{Name: "Mira", Budget: 1, Limit: 1})
	if r.Status != "SUCCESS" || r.Requests != 1 || len(r.Profiles) != 1 || len(r.Profiles[0].ResponseSHA256) != 64 || r.Profiles[0].Name != "" || r.Profiles[0].Login != "" {
		t.Fatalf("lost provenance or invented observed account: %+v", r)
	}
}

func TestWikipediaAbstainsOnMissingAmbiguousOrUnsafeArticles(t *testing.T) {
	for _, body := range []string{
		`{"query":{"pages":[{"ns":0,"missing":true}]}}`,
		`{"query":{"pages":[{"ns":0,"title":"Mira","fullurl":"https://en.wikipedia.org/wiki/Mira","extract":"Mira may refer to...","pageprops":{"disambiguation":""}}]}}`,
		`{"query":{"pages":[{"ns":0,"title":"Mira","fullurl":"http://127.0.0.1/private","extract":"Mira is an actor."}]}}`,
		`{"query":{"pages":[{"ns":1,"title":"Mira","fullurl":"https://en.wikipedia.org/wiki/Talk:Mira","extract":"Mira is an actor."}]}}`,
		`{"query":{"pages":[{"ns":0,"title":"Mira","fullurl":"https://attacker.example/wiki/Mira","extract":"Mira is an actor."}]}}`,
	} {
		w := NewWikipedia(time.Second)
		w.client = mockClient(body, "application/json", 200)
		r := w.Collect(context.Background(), Query{Name: "Mira", Budget: 1, Limit: 1})
		if r.Status != "EMPTY" || len(r.Profiles) != 0 {
			t.Fatalf("invalid reference accepted: %+v", r)
		}
	}
	w := NewWikipedia(time.Second)
	w.client = mockClient(`{"error":{"code":"maxlag"}}`, "application/json", 200)
	r := w.Collect(context.Background(), Query{Name: "Mira", Budget: 3, Limit: 1})
	if r.Status != "FAILED" || r.Requests != 1 {
		t.Fatalf("API failure hidden or retried: %+v", r)
	}
	for _, q := range []Query{{Name: "A|B", Budget: 1, Limit: 1}, {Name: "Mira", Username: "mira", Platform: "instagram", Budget: 1, Limit: 1}, {Name: "Mira", Budget: 0, Limit: 1}} {
		if r := w.Collect(context.Background(), q); r.Requests != 0 || r.Status != "NOT_CHECKED" {
			t.Fatalf("out of scope fetch: %+v", r)
		}
	}
}

func TestSearXNGUnavailableEnginesAreNotEmptyResults(t *testing.T) {
	s, _ := NewSearXNG("http://127.0.0.1:8888", time.Second)
	s.client = mockClient(`{"results":[],"unresponsive_engines":[["engine","Suspended: too many requests"]]}`, "application/json", 200)
	r := s.Collect(context.Background(), Query{Name: "Mira Solis Hart", Budget: 4, Limit: 5})
	if r.Status != "FAILED" || r.Requests != 1 || !operationalFailure(r) || !strings.Contains(r.Events[0].Detail, "too many requests") {
		t.Fatalf("upstream failure hidden: %+v", r)
	}
	s.client = mockClient(`{"results":[],"unresponsive_engines":[]}`, "application/json", 200)
	r = s.Collect(context.Background(), Query{Name: "Mira", Budget: 1, Limit: 5})
	if r.Status != "EMPTY" || operationalFailure(r) {
		t.Fatalf("healthy empty response misclassified: %+v", r)
	}
}
