package graph

import (
	"fmt"
	"strings"
)

var categoryColor = map[string]string{
	"identity": "#4C72B0", "contact": "#C44E52", "affiliation": "#55A868",
	"routine": "#DD8452", "social": "#8172B3", "other": "#8C8C8C",
}

func quote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", " ").Replace(s) + `"`
}

// DOT renders the graph for Graphviz. Facts are coloured by category; unverified
// facts and unconfirmed accounts are dashed so the figure shows uncertainty.
func DOT(g Graph) string {
	var b strings.Builder
	fmt.Fprintf(&b, "digraph exposure {\n  rankdir=LR;\n  node [fontname=\"Helvetica\", fontsize=10];\n  edge [fontname=\"Helvetica\", fontsize=8, color=\"#888888\"];\n")
	for _, n := range g.Nodes {
		attrs := []string{"label=" + quote(n.Label)}
		switch n.Kind {
		case "subject":
			attrs = append(attrs, "shape=doublecircle", "style=filled", `fillcolor="#222222"`, `fontcolor="white"`)
		case "account":
			style := "rounded,filled"
			if n.IdentityState != "ACCEPTED" {
				style += ",dashed"
			}
			attrs = append(attrs, "shape=box", "style="+quote(style), `fillcolor="#EEEEEE"`)
		default:
			style := "filled"
			if n.Status != "OBSERVED" && n.Status != "ACCEPTED" {
				style += ",dashed"
			}
			attrs = append(attrs, "shape=ellipse", "style="+quote(style), "fillcolor="+quote(categoryColor[n.Category]), `fontcolor="white"`)
			if len(n.Accounts) > 1 {
				attrs = append(attrs, "penwidth=3")
			}
		}
		fmt.Fprintf(&b, "  %s [%s];\n", quote(n.ID), strings.Join(attrs, ", "))
	}
	for _, e := range g.Edges {
		label := e.Kind
		if e.Kind == "identity_link" {
			label = fmt.Sprintf("%s %.2f", e.Basis, e.Confidence)
		}
		fmt.Fprintf(&b, "  %s -> %s [label=%s];\n", quote(e.From), quote(e.To), quote(label))
	}
	b.WriteString("}\n")
	return b.String()
}
