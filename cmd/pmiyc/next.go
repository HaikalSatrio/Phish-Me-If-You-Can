package main

import (
	"flag"
	"fmt"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/evidence"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/identity"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/storage"
)

func printNextQuestions(c domain.Case) {
	questions := identity.NextQuestions(c)
	if len(questions) == 0 {
		fmt.Println("No targeted follow-up suggested; this does not establish completeness.")
	}
	for _, q := range questions {
		fmt.Printf("Next [%s]: %s\n  Why: %s\n  Action: pmiyc %s\n", q.Code, q.Prompt, q.Reason, q.Action)
	}
}

func runNext(args []string) error {
	fs := flag.NewFlagSet("next", flag.ContinueOnError)
	input := fs.String("input", "", "saved project to inspect without new collection")
	out := fs.String("out", "", "optional new JSON file containing follow-up questions")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *input == "" || fs.NArg() != 0 {
		return fmt.Errorf("next requires -input")
	}
	var c domain.Case
	if err := storage.Read(*input, &c); err != nil {
		return err
	}
	c, err := evidence.Normalize(c)
	if err != nil {
		return err
	}
	if *out != "" {
		if err := storage.WriteNew(*out, identity.NextQuestions(c)); err != nil {
			return err
		}
	}
	printNextQuestions(c)
	return nil
}
