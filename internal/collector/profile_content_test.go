package collector

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/extract"
	"golang.org/x/net/html"
)

func TestProfileHeaderSurvivesCollectionAndExtraction(t *testing.T) {
	for _, wrapper := range []string{"main", "article", "section"} {
		body := `<header role="banner"><h1>Unrelated Corp</h1></header><` + wrapper + `><header><h1>Ayu Lestari</h1><div>Role: Engineer</div><div>Company: Orbit Labs</div><nav>Navigation</nav><span hidden>Hidden Corp</span></header></` + wrapper + `><footer>Footer Corp</footer>`
		p := NewPage(time.Second)
		p.client = mockClient(body, "text/html", 200)
		result := p.Collect(context.Background(), Query{URL: "https://example.org/people/ayu", Budget: 1})
		if len(result.Profiles) != 1 {
			t.Fatalf("no profile: %+v", result)
		}
		found := false
		for _, f := range result.Profiles[0].Facts {
			if f.Type != "page_text" {
				continue
			}
			found = true
			for _, unwanted := range []string{"Unrelated Corp", "Hidden Corp", "Footer Corp", "Navigation"} {
				if strings.Contains(f.Value, unwanted) {
					t.Fatalf("navigation or hidden content leaked: %s", f.Value)
				}
			}
			relations := extract.Relations("Ayu Lestari", []extract.Document{{ID: "page", Type: f.Type, Text: f.Value}}, time.Now())
			if len(relations) != 1 || relations[0].Organization != "Orbit Labs" || relations[0].Role != "Engineer" {
				t.Fatalf("header fields lost: %s %+v", f.Value, relations)
			}
		}
		if !found {
			t.Fatal("page text absent")
		}
	}
}

func TestStructuredEmployerReferencesStayWithinPage(t *testing.T) {
	for _, tc := range []struct{ name, objects, want string }{
		{"same page", `{"@id":"#employer","@type":"Organization","name":"Orbit Labs"}`, "Orbit Labs"},
		{"unresolved", `{"@id":"#other","@type":"Organization","name":"Other Corp"}`, ""},
		{"ambiguous", `[{"@id":"#employer","name":"Orbit Labs"},{"@id":"#employer","name":"Other Corp"}]`, ""},
		{"cycle", `{"@id":"#employer","@type":"Organization","mainEntity":{"@id":"#employer"}}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := `<script type="application/ld+json">{"@type":"Person","name":"Ayu Lestari","jobTitle":"Engineer","worksFor":{"@id":"#employer"},"alumniOf":{"name":"Other University"}}</script><script type="application/ld+json">` + tc.objects + `</script>`
			doc, err := html.Parse(strings.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			var people []domain.PersonRecord
			if err := json.Unmarshal([]byte(structuredPeople(doc)), &people); err != nil || len(people) != 1 {
				t.Fatalf("invalid people: %+v %v", people, err)
			}
			if strings.Join(people[0].Employers, "") != tc.want || len(people[0].Schools) != 1 {
				t.Fatalf("wrong employer or education merged: %+v", people)
			}
		})
	}
}
