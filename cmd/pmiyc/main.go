package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/collector"
	"os"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/evidence"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/scoring"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/storage"
)

func run(args []string) error {
	if len(args) > 0 && args[0] == "next" {
		return runNext(args[1:])
	}
	if len(args) > 0 && args[0] == "review-relation" {
		return runReviewRelation(args[1:])
	}
	if len(args) > 0 && args[0] == "enrich" {
		return runEnrich(args[1:])
	}
	if len(args) == 1 && args[0] == "adapters" {
		for _, a := range collector.DirectAdapters() {
			fmt.Printf("%s | %s{username} | rule=%s | HTML without API | experimental, live reliability not established\n", a.Name, a.BaseURL, a.Rule)
		}
		return nil
	}
	if len(args) > 0 && args[0] == "compare" {
		return runCompare(args[1:])
	}
	if len(args) == 1 && args[0] == "platforms" {
		for _, p := range collector.Platforms() {
			fmt.Printf("%s: %s\n", p.Name, p.Mode)
		}
		return nil
	}
	if len(args) > 0 && args[0] == "search" {
		return runSearch(args[1:])
	}
	if len(args) == 0 {
		return fmt.Errorf("usage: pmiyc search -source auto -username <username> | pmiyc search -source web -name <name> | pmiyc enrich -input case.json -out enriched.json [-model local-model] | pmiyc review-relation -input case.json -out reviewed.json -relation <id> -decision <ACCEPT|REJECT|PENDING> -reason <reason> | pmiyc next -input case.json | pmiyc adapters | pmiyc compare -before before.json -after after.json | pmiyc <import|analyze|review|candidates> -input case.json [-out new-file.json]")
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	input := fs.String("input", "", "case JSON path")
	policyPath := fs.String("policy", "configs/experimental-v1.json", "scoring policy")
	out := fs.String("out", "", "new output file (never overwrites)")
	candidate := fs.String("candidate", "", "candidate ID for review")
	decision := fs.String("decision", "", "CONFIRM, REJECT, UNSURE, or PENDING")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *input == "" || fs.NArg() != 0 {
		return fmt.Errorf("input is required; positional arguments are not supported")
	}
	var c domain.Case
	if err := storage.Read(*input, &c); err != nil {
		return err
	}
	var result any
	switch args[0] {
	case "candidates":
		printCandidates(c)
		return nil
	case "import":
		if *out == "" {
			return fmt.Errorf("import requires -out")
		}
		normalized, err := evidence.Normalize(c)
		if err != nil {
			return err
		}
		result = normalized
	case "review":
		if *out == "" {
			return fmt.Errorf("review requires -out to preserve the original project")
		}
		found := false
		for i := range c.Candidates {
			if c.Candidates[i].ID == *candidate {
				if c.Search != nil && *decision == "CONFIRM" && len(c.Candidates[i].Contradictions) > 0 {
					return fmt.Errorf("candidate has conflicting clues; refine the search before confirming")
				}
				c.Candidates[i].Decision = *decision
				found = true
			}
		}
		if !found {
			return fmt.Errorf("candidate not found")
		}
		if c.Search != nil {
			if *decision == "CONFIRM" {
				for i := range c.Candidates {
					if c.Candidates[i].ID != *candidate && c.Candidates[i].Decision == "CONFIRM" {
						c.Candidates[i].Decision = "PENDING"
					}
				}
			}
			c.Search.Status = "REVIEW_REQUIRED"
			for _, v := range c.Candidates {
				if v.Decision == "CONFIRM" {
					c.Search.Status = "IDENTITY_CONFIRMED"
				}
			}
			c.Search.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			c.Search.Events = append(c.Search.Events, domain.SearchEvent{At: c.Search.UpdatedAt, Operation: "review:" + *candidate, Status: *decision, Detail: "Operator decision; confirmation does not independently verify public profile claims"})
		}
		normalized, err := evidence.Normalize(c)
		if err != nil {
			return err
		}
		result = normalized
	case "analyze":
		var p domain.Policy
		if err := storage.Read(*policyPath, &p); err != nil {
			return err
		}
		r, err := scoring.Calculate(c, p)
		if err != nil {
			return err
		}
		result = r
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
	if *out != "" {
		return storage.WriteNew(*out, result)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
