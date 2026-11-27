package collector

import (
	"net/mail"
	"net/url"
	"sort"
	"strings"

	"golang.org/x/net/html"
)

func ignoredProfileNode(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	for _, a := range n.Attr {
		if a.Key == "hidden" || a.Key == "aria-hidden" && strings.EqualFold(a.Val, "true") {
			return true
		}
	}
	style := strings.ToLower(strings.Join(strings.Fields(attr(n, "style")), ""))
	if strings.Contains(style, "display:none") || strings.Contains(style, "visibility:hidden") {
		return true
	}
	switch n.Data {
	case "script", "style", "template", "form", "footer":
		return true
	}
	return false
}

func profileFieldText(n *html.Node) string {
	var parts []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if ignoredProfileNode(n) {
			return
		}
		if n.Type == html.TextNode {
			parts = append(parts, n.Data)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(strings.Join(parts, " ")), " ")
}

func githubDetailFields(doc *html.Node) (company, location, email, website string, conflicts []Fact) {
	values := map[string]map[string]bool{}
	add := func(kind, value string) {
		value = strings.TrimSpace(value)
		if value == "" || len(value) > 2048 {
			return
		}
		if values[kind] == nil {
			values[kind] = map[string]bool{}
		}
		values[kind][value] = true
	}
	var walk func(*html.Node, bool, string)
	walk = func(n *html.Node, inDetails bool, property string) {
		if ignoredProfileNode(n) {
			return
		}
		if n.Type == html.ElementNode {
			inDetails = inDetails || hasClass(n, "vcard-details")
			if inDetails {
				if prop := attr(n, "itemprop"); prop != "" {
					property = prop
				}
				switch attr(n, "itemprop") {
				case "worksFor":
					add("workplace", profileFieldText(n))
					return
				case "homeLocation":
					add("location", profileFieldText(n))
					return
				}
				if n.Data == "a" {
					if property == "url" {
						if u, err := cleanURL(attr(n, "href")); err == nil {
							add("website", u.String())
						}
					}
					if property == "email" {
						if u, err := url.Parse(attr(n, "href")); err == nil && u.Scheme == "mailto" && u.RawQuery == "" {
							if address, err := mail.ParseAddress(u.Opaque); err == nil && address.Address == u.Opaque {
								add("email", address.Address)
							}
						}
					}
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child, inDetails, property)
		}
	}
	walk(doc, false, "")
	for _, field := range []struct {
		kind   string
		target *string
	}{
		{"workplace", &company}, {"location", &location}, {"email", &email}, {"website", &website},
	} {
		ordered := make([]string, 0, len(values[field.kind]))
		for value := range values[field.kind] {
			ordered = append(ordered, value)
		}
		sort.Strings(ordered)
		if len(ordered) == 1 {
			*field.target = ordered[0]
		} else {
			for _, value := range ordered {
				conflicts = append(conflicts, Fact{Type: field.kind, Value: value})
			}
		}
	}
	return
}
