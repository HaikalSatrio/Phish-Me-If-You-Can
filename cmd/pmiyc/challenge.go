package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/evidence"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/identity"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/storage"
)

func printReviewCards(w io.Writer, cards []identity.ReviewCard) {
	for i, card := range cards {
		fmt.Fprintf(w, "\n[%d] name=%q | %s\n  Title: %q\n  Profile: %q\n", i+1, card.Name, card.Decision, card.Title, card.URL)
		if len(card.Contradictions) > 0 {
			fmt.Fprintf(w, "  Conflicting clues: %q\n", card.Contradictions)
		}
		if len(card.Claims) == 0 {
			fmt.Fprintln(w, "  No employment or education relationship extracted from this page.")
		}
		for _, claim := range card.Claims {
			fmt.Fprintf(w, "  Source claim (unverified): role=%q organization=%q | %s | %s | review=%s\n    Quote: %q\n", claim.Role, claim.Organization, claim.Kind, claim.Temporal, claim.Review, claim.Quote)
			for _, source := range claim.Sources {
				fmt.Fprintf(w, "    Source: %q (observed %s)\n", source.URL, source.ObservedAt)
			}
			if claim.SubjectEvidenceID != "" {
				fmt.Fprintf(w, "    Subject title: %q\n", claim.SubjectQuote)
				for _, source := range claim.SubjectSources {
					fmt.Fprintf(w, "    Subject source: %q\n", source.URL)
				}
			}
		}
	}
}

func selectionDecisions(c domain.Case, selection string) ([]domain.CandidateDecision, error) {
	selection = strings.TrimSpace(selection)
	choices := map[int]bool{}
	other := "UNSURE"
	switch strings.ToLower(selection) {
	case "none":
		other = "REJECT"
	case "unsure":
	case "":
		return nil, fmt.Errorf("choose profile numbers, none, or unsure")
	default:
		for _, part := range strings.Split(selection, ",") {
			n, err := strconv.Atoi(strings.TrimSpace(part))
			if err != nil || n < 1 || n > len(c.Candidates) || choices[n] {
				return nil, fmt.Errorf("invalid or duplicate profile number %q", part)
			}
			choices[n] = true
		}
	}
	out := []domain.CandidateDecision{}
	for i, p := range c.Candidates {
		decision := other
		if choices[i+1] {
			decision = "CONFIRM"
		}
		out = append(out, domain.CandidateDecision{CandidateID: p.ID, Decision: decision})
	}
	return out, nil
}

func runChallenge(args []string) error { return challenge(args, os.Stdin, os.Stdout) }

func challenge(args []string, reader io.Reader, writer io.Writer) error {
	fs := flag.NewFlagSet("challenge", flag.ContinueOnError)
	input := fs.String("input", "", "saved search or enriched case")
	out := fs.String("out", "", "new reviewed case path")
	selection := fs.String("select", "", "comma-separated profile numbers, none, or unsure; omit for interactive selection")
	reason := fs.String("reason", "", "reason for the identity decision; required with -select")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *input == "" || *out == "" || fs.NArg() != 0 {
		return fmt.Errorf("challenge requires -input and -out")
	}
	if _, err := os.Stat(*out); err == nil {
		return fmt.Errorf("output already exists: %s", *out)
	} else if !os.IsNotExist(err) {
		return err
	}
	var c domain.Case
	if err := storage.Read(*input, &c); err != nil {
		return err
	}
	var err error
	if c, err = evidence.Normalize(c); err != nil {
		return err
	}
	if len(c.Candidates) == 0 {
		return fmt.Errorf("no candidates to review; add a public account or URL clue")
	}
	fmt.Fprintf(writer, "Identity review for %q. Source claims may be outdated or incorrect.\n", c.Target.Name)
	printReviewCards(writer, identity.Cards(c))
	fmt.Fprintln(writer, "\nWhich profiles belong to the person you mean? Choose numbers such as 1,4; none; or unsure. Unselected profiles become UNSURE. Identity confirmation does not accept relationship claims.")
	interactive := *selection == ""
	scanner := bufio.NewScanner(reader)
	var decisions []domain.CandidateDecision
	for {
		if interactive {
			fmt.Fprint(writer, "Selection (q to cancel): ")
			if !scanner.Scan() {
				return fmt.Errorf("identity review cancelled before selection; no output written")
			}
			*selection = scanner.Text()
			if strings.EqualFold(strings.TrimSpace(*selection), "q") {
				return fmt.Errorf("identity review cancelled; no output written")
			}
		}
		decisions, err = selectionDecisions(c, *selection)
		if err == nil {
			break
		}
		if !interactive {
			return err
		}
		fmt.Fprintln(writer, err)
	}
	if interactive && strings.TrimSpace(*reason) == "" {
		fmt.Fprint(writer, "Reason for this identity decision: ")
		if !scanner.Scan() {
			return fmt.Errorf("identity review cancelled before reason; no output written")
		}
		*reason = scanner.Text()
	}
	result, err := identity.Review(c, decisions, *reason, time.Now())
	if err != nil {
		return err
	}
	if err := storage.WriteNew(*out, result); err != nil {
		return err
	}
	fmt.Fprintf(writer, "Saved identity review: %s\n", *out)
	confirmed := 0
	for _, p := range result.Candidates {
		if p.Decision == "CONFIRM" {
			confirmed++
			fmt.Fprintf(writer, "Confirmed profile: %q | %q\n  Source summaries (unverified): role=%q company=%q\n", p.DisplayName, p.ProfileURL, p.Role, p.Company)
		}
	}
	fmt.Fprintf(writer, "Associated %d profiles with this target. Original observations and relationship review states are retained.\n", confirmed)
	for _, q := range identity.NextQuestions(result) {
		fmt.Fprintf(writer, "Next [%s]: %s\n  Action: pmiyc %s\n", q.Code, q.Prompt, q.Action)
	}
	return nil
}
