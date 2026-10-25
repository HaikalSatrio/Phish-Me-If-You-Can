package collector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPartialNamesRequireIndividualProfileAndOrderedSubset(t *testing.T) {
	q := Query{Name: "Raka Aditya Pranata"}
	for _, tc := range []struct {
		raw, title, snippet string
		want                bool
	}{
		{"https://id.linkedin.com/in/raka-aditya", "Raka Aditya - Student", "", true},
		{"https://linkedin.com/in/raka", "Raka Pranata - Student", "", true},
		{"https://linkedin.com/in/raka", "Raka - Student", "", false},
		{"https://linkedin.com/in/other", "Muhammad Raka Aditya Wibowo - Student", "", false},
		{"https://linkedin.com/in/other", "Aditya Raka - Student", "", false},
		{"https://example.org/story", "Raka Aditya - Student", "", false},
		{"https://linkedin.com/pub/dir/raka/aditya", "Raka Aditya Pranata profiles", "", false},
		{"https://facebook.com/public/Raka-Aditya-Pranata", "Raka Aditya Pranata", "", false},
	} {
		if got := relevantSearchResult(q, tc.raw, tc.title, tc.snippet); got != tc.want {
			t.Fatalf("%s %q: got %v", tc.raw, tc.title, got)
		}
	}
	q.Username = "raka_known"
	if relevantSearchResult(q, "https://linkedin.com/in/raka-aditya", "Raka Aditya - Student", "") {
		t.Fatal("partial name weakened an exact account clue")
	}
}

func TestSearchRetainsPartialProfileAndDistinctQueryEvidence(t *testing.T) {
	queries := []string{}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		query := req.URL.Query().Get("q")
		queries = append(queries, query)
		content := "Undergraduate Cyber Security Student"
		if strings.Contains(query, "site:linkedin.com") {
			content += " | Example University · Pendidikan: Example University · Lokasi: Jakarta"
		}
		json.NewEncoder(w).Encode(map[string]any{"results": []map[string]string{
			{"url": "https://linkedin.com/in/raka-aditya", "title": "Raka Aditya - Undergraduate Cyber Security Student", "content": content},
			{"url": "https://linkedin.com/in/other", "title": "Muhammad Raka Aditya Wibowo - Student", "content": "Other person"},
			{"url": "https://linkedin.com/directory/people/r", "title": "Raka Aditya Pranata", "content": "Directory"},
		}})
	}))
	defer s.Close()
	c, _ := NewSearXNG(s.URL, time.Second)
	r := c.Collect(context.Background(), Query{Name: "Raka Aditya Pranata", Limit: 5, Budget: 3})
	if len(r.Profiles) != 1 || len(queries) != 3 || !strings.Contains(r.Profiles[0].DiscoveryBasis, "partial") {
		t.Fatalf("unexpected discovery: %+v queries=%v", r, queries)
	}
	sources := map[string]string{}
	for _, fact := range r.Profiles[0].Facts {
		if fact.Type == "search_snippet" {
			sources[fact.Value] = SourceForFact(r.Profiles[0], fact).URL
		}
	}
	if len(sources) != 2 {
		t.Fatalf("lost alternate snippets: %v", sources)
	}
	for value, source := range sources {
		if strings.Contains(value, "Pendidikan") != strings.Contains(source, "site%3Alinkedin.com") {
			t.Fatalf("wrong query provenance: %q %q", value, source)
		}
	}
}
