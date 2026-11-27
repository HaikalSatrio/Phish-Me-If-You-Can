package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/evidence"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/graph"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/scoring"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/storage"
)

func runGraph(args []string) error {
	fs := flag.NewFlagSet("graph", flag.ContinueOnError)
	input := fs.String("input", "", "reviewed (and optionally enriched) case")
	out := fs.String("out", "", "new exposure graph JSON")
	dot := fs.String("dot", "", "new Graphviz DOT file for a figure")
	policyPath := fs.String("policy", "configs/experimental-v1.json", "scoring policy used to resolve identity states")
	unconfirmed := fs.Bool("include-unconfirmed", false, "also include accounts that are not yet accepted, labelled with their identity state")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *input == "" || fs.NArg() != 0 {
		return fmt.Errorf("graph requires -input")
	}
	if *out != "" && *dot != "" {
		jsonPath, err := filepath.Abs(*out)
		if err != nil {
			return err
		}
		dotPath, err := filepath.Abs(*dot)
		if err != nil {
			return err
		}
		if strings.EqualFold(jsonPath, dotPath) {
			return fmt.Errorf("graph JSON and DOT outputs must use different paths")
		}
	}
	for _, path := range []string{*out, *dot} {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("output already exists: %s", path)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	var c domain.Case
	if err := storage.Read(*input, &c); err != nil {
		return err
	}
	c, err := evidence.Normalize(c)
	if err != nil {
		return err
	}
	var p domain.Policy
	if err := storage.Read(*policyPath, &p); err != nil {
		return err
	}
	if err := scoring.Validate(p); err != nil {
		return fmt.Errorf("graph policy: %w", err)
	}
	g := graph.Build(c, graph.Options{Policy: p, IncludeUnconfirmed: *unconfirmed})
	if *out != "" {
		if err := storage.WriteNew(*out, g); err != nil {
			return err
		}
		fmt.Printf("Saved exposure graph: %s\n", *out)
	}
	if *dot != "" {
		if err := writeNewText(*dot, graph.DOT(g)); err != nil {
			return err
		}
		fmt.Printf("Saved Graphviz figure source: %s (render: dot -Tpng %s -o graph.png)\n", *dot, *dot)
	}
	printGraph(os.Stdout, g)
	return nil
}

// writeNewText follows the same never-overwrite rule as storage.WriteNew.
func writeNewText(path, text string) error {
	return storage.WriteNewBytes(path, []byte(text))
}

func printGraph(w io.Writer, g graph.Graph) {
	s := g.Summary
	fmt.Fprintf(w, "\nExposure graph for %q\nScope: %s\n", g.TargetName, g.Scope)
	fmt.Fprintf(w, "Accounts: %d included, %d excluded | Facts: %d | Shared across accounts: %d\n", s.Accounts, s.ExcludedAccounts, s.Facts, s.SharedFacts)
	if s.Accounts == 0 {
		fmt.Fprintln(w, "No accepted accounts. Confirm identities with challenge, or rerun with -include-unconfirmed to inspect candidates.")
	}
	for _, cat := range graph.Categories {
		if n := s.FactsByCategory[cat]; n > 0 {
			fmt.Fprintf(w, "  %-12s %d\n", cat, n)
		}
	}
	for _, n := range g.Nodes {
		switch n.Kind {
		case "account":
			fmt.Fprintf(w, "Account %s [%s %.2f] %s\n", n.Label, n.IdentityState, n.Confidence, n.URL)
		case "fact":
			shared := ""
			if len(n.Accounts) > 1 {
				shared = fmt.Sprintf(" (shared by %d accounts)", len(n.Accounts))
			}
			fmt.Fprintf(w, "  [%s/%s] %q %s%s\n", n.Category, n.Type, n.Label, n.Status, shared)
		}
	}
	reasons := make([]string, 0, len(s.Excluded))
	for reason := range s.Excluded {
		reasons = append(reasons, reason)
	}
	sort.Strings(reasons)
	for _, reason := range reasons {
		fmt.Fprintf(w, "Excluded %s: %d\n", reason, s.Excluded[reason])
	}
	fmt.Fprintln(w, "Facts remain unverified public claims; the graph organises evidence and adds none.")
}
