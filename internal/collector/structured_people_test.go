package collector

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"golang.org/x/net/html"
)

func TestStructuredPeopleAndBlockBoundaries(t *testing.T) {
	raw := `<head><script type="application/ld+json">{"@context":"https://schema.org","@graph":[{"@type":"Person","name":"Ayu Lestari","jobTitle":"Engineer","worksFor":{"@type":"Organization","name":"Orbit Labs"},"alumniOf":{"name":"Nusantara University"}},{"@type":"Person","name":"Budi Santoso","jobTitle":"Director","worksFor":{"name":"Contoso Ltd"}},{"@type":"Organization","name":"Other","jobTitle":"Unrelated"}]}</script></head><body><main><p>Ayu Lestari interviewed Budi Santoso.</p><p>Budi Santoso works at Contoso Ltd.</p><table><tr><th>Name</th><th>Role</th></tr><tr><td>Ayu Lestari</td><td>Engineer</td></tr></table></main></body>`
	doc, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	var people []domain.PersonRecord
	if json.Unmarshal([]byte(structuredPeople(doc)), &people) != nil || len(people) != 2 {
		t.Fatalf("wrong structured people: %+v", people)
	}
	if people[0].Employers[0] != "Orbit Labs" || people[1].Employers[0] != "Contoso Ltd" {
		t.Fatal("people's fields were merged")
	}
	base, _ := url.Parse("https://example.org")
	text, _ := pageContent(doc, base)
	if !strings.Contains(text, "Santoso.\nBudi") || !strings.Contains(text, "| Name | Role\n| Ayu Lestari | Engineer") {
		t.Fatalf("lost paragraph/table boundaries: %q", text)
	}
	if strings.Contains(text, "Orbit Labs") {
		t.Fatal("script leaked into prose")
	}
}
