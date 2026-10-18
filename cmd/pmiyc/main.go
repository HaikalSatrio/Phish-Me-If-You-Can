package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/evidence"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/scoring"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/storage"
)

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: pmiyc <import|analyze|review> -input case.json [-policy configs/experimental-v1.json] [-out new-file.json]")
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
				c.Candidates[i].Decision = *decision
				found = true
			}
		}
		if !found {
			return fmt.Errorf("candidate not found")
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
