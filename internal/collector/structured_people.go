package collector

import (
	"encoding/json"
	"strings"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"golang.org/x/net/html"
)

// Only explicit Person objects are retained. No remote contexts or IDs are fetched.
func structuredPeople(doc *html.Node) string {
	people := []domain.PersonRecord{}
	budget := 64 << 10
	values := []any{}
	objects := map[string]map[string]any{}
	var index func(any, int)
	index = func(v any, depth int) {
		if depth > 16 || len(objects) >= 256 {
			return
		}
		switch x := v.(type) {
		case []any:
			for _, item := range x {
				index(item, depth+1)
			}
		case map[string]any:
			if id, ok := x["@id"].(string); ok && id != "" && len(x) > 1 {
				if _, exists := objects[id]; exists {
					objects[id] = nil
				} else {
					objects[id] = x
				}
			}
			for _, key := range []string{"@graph", "mainEntity", "author", "employee", "worksFor", "alumniOf", "memberOf", "birthPlace"} {
				index(x[key], depth+1)
			}
		}
	}
	var names func(any) []string
	names = func(v any) []string {
		out := []string{}
		switch x := v.(type) {
		case string:
			if len([]rune(x)) <= 160 && strings.TrimSpace(x) != "" {
				out = append(out, strings.Join(strings.Fields(x), " "))
			}
		case map[string]any:
			if name, ok := x["name"].(string); ok {
				out = names(name)
			} else if id, ok := x["@id"].(string); ok {
				if name, ok := objects[id]["name"].(string); ok {
					out = names(name)
				}
			}
		case []any:
			for _, item := range x {
				out = append(out, names(item)...)
				if len(out) >= 10 {
					break
				}
			}
		}
		return out[:min(len(out), 10)]
	}
	seenPeople := map[string]bool{}
	remaining := 2048
	var scan func(any, int)
	scan = func(v any, depth int) {
		if depth > 16 || len(people) >= 20 || remaining <= 0 {
			return
		}
		remaining--
		switch x := v.(type) {
		case []any:
			for _, item := range x {
				scan(item, depth+1)
			}
		case map[string]any:
			if id, ok := x["@id"].(string); ok && len(x) == 1 {
				if resolved := objects[id]; resolved != nil {
					scan(resolved, depth+1)
				}
				return
			}
			person := false
			for _, typ := range names(x["@type"]) {
				if typ == "Person" || typ == "https://schema.org/Person" || typ == "http://schema.org/Person" {
					person = true
				}
			}
			if person {
				n := names(x["name"])
				if len(n) == 1 {
					person := domain.PersonRecord{Name: n[0], JobTitles: names(x["jobTitle"]), Employers: names(x["worksFor"]), Schools: names(x["alumniOf"]), Memberships: names(x["memberOf"]), BirthPlaces: names(x["birthPlace"])}
					key, _ := json.Marshal(person)
					if !seenPeople[string(key)] {
						seenPeople[string(key)] = true
						people = append(people, person)
					}
				}
			}
			// Named containers avoid guessing ownership from unrelated nested objects.
			for _, key := range []string{"@graph", "mainEntity", "author", "employee"} {
				if child, ok := x[key]; ok {
					scan(child, depth+1)
				}
			}
		}
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if budget <= 0 || len(people) >= 20 {
			return
		}
		if n.Type == html.ElementNode && n.Data == "script" && strings.EqualFold(attr(n, "type"), "application/ld+json") {
			var b strings.Builder
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				if child.Type == html.TextNode {
					if len(child.Data) > budget {
						budget = 0
						return
					}
					budget -= len(child.Data)
					b.WriteString(child.Data)
				}
			}
			var value any
			if json.Unmarshal([]byte(b.String()), &value) == nil {
				values = append(values, value)
				index(value, 0)
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	for _, value := range values {
		scan(value, 0)
	}
	if len(people) == 0 {
		return ""
	}
	data, _ := json.Marshal(people)
	return string(data)
}
