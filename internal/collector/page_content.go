package collector

import (
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

func pageContent(doc *html.Node, base *url.URL) (string, []string) {
	var main, article, body *html.Node
	var find func(*html.Node)
	find = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if (n.Data == "main" || attr(n, "role") == "main") && main == nil {
				main = n
			}
			if n.Data == "article" && article == nil {
				article = n
			}
			if n.Data == "body" {
				body = n
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			find(c)
		}
	}
	find(doc)
	root := main
	if root == nil {
		root = article
	}
	if root == nil {
		root = body
	}
	if root == nil {
		root = doc
	}
	var text strings.Builder
	links := []string{}
	seen := map[string]bool{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "head", "script", "style", "noscript", "nav", "footer", "header", "form", "template", "svg":
				return
			}
			if text.Len() < 32768 {
				switch n.Data {
				case "p", "div", "section", "li", "tr", "h1", "h2", "h3", "br":
					text.WriteByte('\n')
				case "td", "th":
					text.WriteString(" | ")
				}
			}
			for _, a := range n.Attr {
				if a.Key == "hidden" || (a.Key == "aria-hidden" && a.Val == "true") {
					return
				}
			}
			if n.Data == "a" && len(links) < 50 {
				href := strings.TrimSpace(attr(n, "href"))
				if u, err := url.Parse(href); err == nil && href != "" && !strings.HasPrefix(href, "#") {
					if u, err = cleanURL(base.ResolveReference(u).String()); err == nil && !seen[u.String()] {
						seen[u.String()] = true
						links = append(links, u.String())
					}
				}
			}
		}
		if n.Type == html.TextNode && text.Len() < 32768 {
			value := n.Data
			if len(value) > 32768 {
				value = string([]rune(value)[:min(8192, len([]rune(value)))])
			}
			text.WriteString(value)
			text.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if n.Type == html.ElementNode && text.Len() < 32768 {
			switch n.Data {
			case "p", "div", "section", "li", "tr", "h1", "h2", "h3":
				text.WriteByte('\n')
			}
		}
	}
	walk(root)
	lines := []string{}
	for _, line := range strings.Split(text.String(), "\n") {
		if line = strings.Join(strings.Fields(line), " "); line != "" {
			lines = append(lines, line)
		}
	}
	value := []rune(strings.Join(lines, "\n"))
	if len(value) > 8000 {
		value = append(value[:8000], []rune(" [truncated]")...)
	}
	return string(value), links
}
