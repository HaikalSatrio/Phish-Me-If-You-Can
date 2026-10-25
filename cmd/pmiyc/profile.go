package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/dossier"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/storage"
)

func runProfile(args []string) error {
	fs := flag.NewFlagSet("profile", flag.ContinueOnError)
	input := fs.String("input", "", "case with operator identity decisions")
	out := fs.String("out", "", "optional new combined report JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *input == "" || fs.NArg() != 0 {
		return fmt.Errorf("profile requires -input")
	}
	var c domain.Case
	if err := storage.Read(*input, &c); err != nil {
		return err
	}
	r, err := dossier.Build(c)
	if err != nil {
		return err
	}
	if *out != "" {
		if err := storage.WriteNew(*out, r); err != nil {
			return err
		}
		fmt.Printf("Saved profile report: %s\n", *out)
	}
	printProfile(os.Stdout, r)
	return nil
}

func printProfile(w io.Writer, r dossier.Report) {
	fmt.Fprintf(w, "\nProfile summary for search input %q\nStatus: %s\n", r.SearchName, r.Status)
	if len(r.Profiles) == 0 {
		fmt.Fprintln(w, "No profiles have been explicitly confirmed. Run challenge -input <case> -out <reviewed-case> first.")
		return
	}
	fmt.Fprintf(w, "Operator-associated profiles: %d; excluded pending, unsure or rejected profiles: %d\n", len(r.Profiles), len(r.ExcludedProfiles))
	for _, p := range r.Profiles {
		fmt.Fprintf(w, "  %q | %q\n", p.Name, p.URL)
	}
	fmt.Fprintln(w, "Observed names:")
	if len(r.ObservedNames) == 0 {
		fmt.Fprintln(w, "  Not established from collected sources.")
	}
	for _, name := range r.ObservedNames {
		fmt.Fprintf(w, "  %q\n", name.Value)
		for _, e := range name.Evidence {
			for _, s := range e.Sources {
				fmt.Fprintf(w, "    Source: %q (observed %s)\n", s.URL, s.ObservedAt)
			}
		}
	}
	if len(r.ProfileFields) > 0 {
		fmt.Fprintln(w, "Observed profile fields (source self-descriptions):")
		for _, e := range r.ProfileFields {
			fmt.Fprintf(w, "  %s=%q | profile=%q\n", e.Type, e.Value, e.CandidateID)
			for _, s := range e.Sources {
				fmt.Fprintf(w, "    Source: %q (observed %s)\n", s.URL, s.ObservedAt)
			}
		}
	}
	for _, section := range []struct {
		label  string
		claims []dossier.Claim
	}{
		{"Employment source claims", r.Employment}, {"Education source claims", r.Education}, {"Occupation source claims", r.Occupations}, {"Other source claims", r.Other}, {"Model proposals requiring relationship review", r.PendingRelations},
	} {
		fmt.Fprintln(w, section.label+":")
		if len(section.claims) == 0 {
			fmt.Fprintln(w, "  None established in this category.")
			continue
		}
		for _, claim := range section.claims {
			fmt.Fprintf(w, "  role=%q organization=%q | %s | %s\n", claim.Role, claim.Organization, claim.Temporal, claim.Status)
			if claim.StartYear != 0 || claim.EndYear != 0 {
				fmt.Fprintf(w, "    Period: start=%d end=%d (0 means unspecified)\n", claim.StartYear, claim.EndYear)
			}
			for _, support := range claim.Support {
				fmt.Fprintf(w, "    Profile=%q subject=%q relation=%q review=%s\n    Quote: %q\n", support.CandidateID, support.Subject, support.RelationID, support.Review, support.Quote)
				for _, s := range support.Sources {
					fmt.Fprintf(w, "    Source: %q (observed %s)\n", s.URL, s.ObservedAt)
				}
				if support.SubjectQuote != "" {
					fmt.Fprintf(w, "    Subject title: %q\n", support.SubjectQuote)
					for _, s := range support.SubjectSources {
						fmt.Fprintf(w, "    Subject source: %q\n", s.URL)
					}
				}
			}
		}
	}
	for _, issue := range r.Issues {
		fmt.Fprintf(w, "Review [%s]: %s\n", issue.Code, issue.Detail)
	}
	fmt.Fprintln(w, "All source claims remain unverified. Identity confirmation does not verify current employment or education.")
}
