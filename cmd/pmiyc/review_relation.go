package main

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/enrich"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/evidence"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/storage"
)

func runReviewRelation(args []string) error {
	fs := flag.NewFlagSet("review-relation", flag.ContinueOnError)
	input := fs.String("input", "", "saved project")
	out := fs.String("out", "", "new snapshot; never overwrites")
	id := fs.String("relation", "", "relationship ID shown by candidates")
	decision := fs.String("decision", "", "ACCEPT, REJECT, or PENDING")
	reason := fs.String("reason", "", "reason for the operator decision")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *input == "" || *out == "" || *id == "" || strings.TrimSpace(*reason) == "" || fs.NArg() != 0 {
		return fmt.Errorf("review-relation requires -input, -out, -relation, -decision and -reason")
	}
	if *decision != "ACCEPT" && *decision != "REJECT" && *decision != "PENDING" {
		return fmt.Errorf("relationship decision must be ACCEPT, REJECT, or PENDING")
	}
	var c domain.Case
	if err := storage.Read(*input, &c); err != nil {
		return err
	}
	c, err := evidence.Normalize(c)
	if err != nil {
		return err
	}
	found := false
	for i := range c.Relations {
		if c.Relations[i].ID == *id {
			c.Relations[i].Reviews = append(c.Relations[i].Reviews, domain.RelationReview{Decision: *decision, At: time.Now().UTC().Format(time.RFC3339), Reason: strings.TrimSpace(*reason)})
			found = true
		}
	}
	if !found {
		return fmt.Errorf("relationship not found: %s", *id)
	}
	enrich.Reproject(&c)
	c, err = evidence.Normalize(c)
	if err != nil {
		return err
	}
	if err := storage.WriteNew(*out, c); err != nil {
		return err
	}
	fmt.Printf("Saved relationship review: %s\nOperator acceptance does not verify identity, source truth, or eligibility for PES.\n", *out)
	printCandidates(c)
	return nil
}
