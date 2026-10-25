package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/collector"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/search"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/storage"
)

type accountFlags []domain.AccountClue

func (a *accountFlags) String() string { return fmt.Sprint([]domain.AccountClue(*a)) }
func (a *accountFlags) Set(value string) error {
	platform, username, ok := strings.Cut(value, ":")
	if !ok {
		return fmt.Errorf("account must use platform:username, for example instagram:@haikal_satrio")
	}
	normalized, err := collector.NormalizeAccounts(append(append([]domain.AccountClue(nil), *a...), domain.AccountClue{Platform: platform, Username: username}))
	if err != nil {
		return err
	}
	*a = normalized
	return nil
}

func runSearch(args []string) error {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	name := fs.String("name", "", "subject name")
	username := fs.String("username", "", "exact account username or full Bluesky handle; use -name for people search")
	var accounts accountFlags
	fs.Var(&accounts, "account", "known platform:username clue; repeat for multiple platforms (web source)")
	source := fs.String("source", "", "auto (free public profiles), direct (HTML), web (free search and pages), searxng, duckduckgo, bing, github, devto, google, brave, google-api, url, or bluesky")
	platform := fs.String("platform", "", "platform name or all; web tries scoped account hints and indexed discovery")
	pageURL := fs.String("url", "", "explicit public page URL")
	company := fs.String("company", "", "company clue for comparison")
	location := fs.String("location", "", "location clue for comparison")
	input := fs.String("input", "", "saved search session to refine")
	interactive := fs.Bool("interactive", false, "review collected identities interactively and save a separate reviewed snapshot")
	out := fs.String("out", "", "new output file, default data/<session>-<attempt>.json")
	config := fs.String("config", "configs/collector-v1.json", "collector configuration")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments %q; quote values containing spaces, for example: search -source web -name \"Haikal Satrio Dewandaru\". Use -username only for an exact account username", fs.Args())
	}
	if *out != "" {
		if _, err := os.Stat(*out); err == nil {
			return fmt.Errorf("output already exists: %s", *out)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	var cfg search.Config
	if err := storage.Read(*config, &cfg); err != nil {
		return err
	}
	var previous *domain.Case
	if *input != "" {
		previous = &domain.Case{}
		if err := storage.Read(*input, previous); err != nil {
			return err
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	provider := *source
	if provider == "" && previous != nil && previous.Search != nil {
		provider = previous.Search.Provider
		if provider == "" {
			provider = "github"
		}
	}
	if provider == "" {
		if len(accounts) > 0 {
			provider = "web"
		} else if *pageURL != "" {
			provider = "url"
		} else if *username != "" {
			provider = "auto"
		} else if *name != "" || *platform != "" {
			provider = "web"
		} else {
			provider = "github"
		}
	}
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	var client collector.Collector
	switch provider {
	case "auto":
		client = collector.NewPublicProfiles(timeout, os.Getenv("GITHUB_TOKEN"))
	case "devto":
		client = collector.NewDevTo(timeout)
	case "direct":
		client = collector.NewDirect(timeout)
	case "web":
		configured, err := collector.NewConfiguredDiscoveryWithOptions(timeout, cfg.SearXNGURL, cfg.SearXNGPreferences())
		if err != nil {
			return err
		}
		client = configured
	case "searxng":
		configured, err := collector.NewSearXNGWithOptions(cfg.SearXNGURL, timeout, cfg.SearXNGPreferences())
		if err != nil {
			return err
		}
		client = configured
	case "duckduckgo":
		client = collector.NewDuckDuckGo(timeout)
	case "bing":
		client = collector.NewBing(timeout)
	case "github":
		client = collector.NewGitHub(timeout, os.Getenv("GITHUB_TOKEN"))
	case "google":
		client = collector.NewGoogleBrowser(timeout)
	case "brave":
		client = collector.NewWeb("brave", os.Getenv("BRAVE_API_KEY"), "", timeout)
	case "google-api":
		client = collector.NewWeb("google", os.Getenv("GOOGLE_API_KEY"), os.Getenv("GOOGLE_CSE_ID"), timeout)
	case "url":
		client = collector.NewPage(timeout)
	case "bluesky":
		client = collector.NewBluesky(timeout)
	default:
		return fmt.Errorf("unknown source %q", provider)
	}
	if *platform != "" && provider != "auto" && provider != "direct" && provider != "web" && provider != "searxng" && provider != "duckduckgo" && provider != "bing" && provider != "google" && provider != "brave" && provider != "google-api" {
		return fmt.Errorf("-platform is supported by direct collection and search providers")
	}
	if *pageURL != "" && provider != "url" && provider != "web" {
		return fmt.Errorf("use -source url or -source web with -url")
	}
	lookupUsername := *username
	if lookupUsername == "" && previous != nil && previous.Search != nil {
		lookupUsername = previous.Search.Username
	}
	if (provider == "devto" || provider == "bluesky" || provider == "direct" || provider == "auto") && lookupUsername == "" {
		return fmt.Errorf("-source %s retrieves exact usernames and requires -username; to find people by name on DEV use: search -source web -platform devto -name \"Ben\"", provider)
	}
	project, err := search.Run(ctx, client, search.Input{Name: *name, Username: *username, Accounts: accounts, Company: *company, Location: *location, Provider: provider, Platform: *platform, URL: *pageURL}, cfg, previous)
	if err != nil {
		return err
	}
	path := *out
	if path == "" {
		path = filepath.Join("data", fmt.Sprintf("%s-%d.json", project.Search.ID, project.Search.Attempts))
	}
	if err := storage.WriteNew(path, project); err != nil {
		return err
	}
	fmt.Printf("Saved project: %s\nStatus: %s\nRequests used: %d/%d\n", path, project.Search.Status, project.Search.RequestsUsed, cfg.RequestBudget)
	fmt.Printf("Collected: %d candidates, %d observations\n", len(project.Candidates), len(project.Evidence))
	if provider == "web" {
		fmt.Println("Source: public account/URL clues, linked biographies, SearXNG/DuckDuckGo/Bing, and bounded follow-up name searches. No API key required.")
		if cfg.SearXNGURL != "" {
			fmt.Println("Configured SearXNG service is tried before the public search fallbacks.")
		} else {
			fmt.Println("SearXNG default: http://127.0.0.1:8888. Start it with: powershell -File scripts/start-local-search.ps1")
		}
	}
	if provider == "auto" {
		fmt.Println("Source: free public profile APIs for GitHub, DEV Community, and Bluesky. Matching usernames do not establish shared ownership.")
	}
	if provider == "devto" || provider == "bluesky" || provider == "direct" || provider == "auto" || (provider == "github" && project.Search.Username != "") {
		fmt.Printf("Mode: exact username lookup for %q, not a search for everyone with that display name.\n", project.Search.Username)
	}
	if provider == "bing" {
		fmt.Println("Source: Bing public search RSS, no API key. Social results are indexed links, not full account contents.")
	}
	printCandidates(project)
	for _, receipt := range project.Receipts {
		fmt.Printf("Receipt: %s | %s | rule=%s | response_sha256=%s\n", receipt.Platform, receipt.Status, receipt.Rule, receipt.ResponseSHA256)
	}
	fmt.Println("Scope: " + project.Search.Scope)
	if len(project.Candidates) > 0 {
		fmt.Println("Review: pmiyc review -input <project.json> -candidate <candidate-id> -decision CONFIRM -out <reviewed.json>")
	}
	if len(project.Candidates) > 0 && project.Search.Status != "BLOCKED" && project.Search.Status != "FAILED" && provider != "url" && project.Search.Attempts < cfg.MaxAttempts && project.Search.RequestsUsed < cfg.RequestBudget {
		fmt.Println("Need a closer match? Refine with search -input <project.json> -username <known-username>, -company <company>, or -location <location>.")
	}
	if project.Search.Status == "BLOCKED" {
		fmt.Println("Access was blocked. Check the diagnostic below; a saved project does not mean collection succeeded.")
	}
	for _, ev := range project.Search.Events {
		fmt.Printf("%s: %s (%s)\n", ev.Operation, ev.Status, ev.Detail)
	}
	printNextQuestions(project)
	if len(project.Candidates) == 0 && (project.Search.Status == "BLOCKED" || project.Search.Status == "FAILED") {
		return fmt.Errorf("collection did not succeed; diagnostics saved to %s", path)
	}
	if *interactive && len(project.Candidates) > 0 {
		return runChallenge([]string{"-input", path, "-out", strings.TrimSuffix(path, filepath.Ext(path)) + "-reviewed.json"})
	}
	return nil
}

func printCandidates(c domain.Case) {
	if c.Search != nil && len(c.Search.Research) > 0 {
		fmt.Println("Research trail (retrieval clues, not verified identity links):")
		for _, step := range c.Search.Research {
			fmt.Printf("  %s [%s] requests=%d query=%q url=%q from=%q | %s\n", step.Action, step.Status, step.Requests, step.Query, step.URL, step.FromURL, step.Reason)
		}
	}
	if len(c.Candidates) == 0 {
		fmt.Println("No candidate profiles collected. Check source status before drawing conclusions.")
	}
	for _, p := range c.Candidates {
		if p.DiscoveryBasis != "" {
			fmt.Printf("Discovery: %s | linked from=%q | identity needs review\n", p.DiscoveryBasis, p.LinkedFrom)
		}
		if p.Label != "" {
			fmt.Printf("Page title: %q\n", p.Label)
		}
		companyNote := ""
		if p.Company == "" {
			reasons := map[string]string{
				"ambiguous":           "multiple company claims; inspect alternatives",
				"period_conflict":     "source claims disagree on the period",
				"pending_review":      "model employment proposal needs relationship review",
				"historical_only":     "only historical employers; inspect company details",
				"non_current_claims":  "only non-current company claims; inspect periods",
				"no_employment_claim": "no eligible explicit employment claim in collected sources",
			}
			reason := reasons[p.CompanyStatus]
			if reason == "" {
				reason = "not established; inspect source evidence and relationships"
			}
			companyNote = " (" + reason + ")"
		}
		if p.CompanyInferred {
			companyNote = " (inferred, unverified)"
			if p.CompanyTemporal != "" {
				companyNote += " [" + p.CompanyTemporal + "]"
			}
			if p.CompanyAmbiguous {
				companyNote = " (inferred, unverified, ambiguous)"
			}
		}
		roleNote := ""
		if p.RoleInferred {
			roleNote = " (inferred, unverified)"
			if p.RoleTemporal != "" {
				roleNote += " [" + p.RoleTemporal + "]"
			}
		}
		fmt.Printf("\n%s | user=%q name=%q decision=%s\n  URL: %q\n  Company: %q%s Role: %q%s Location: %q\n  Matches: %q\n  Contradictions: %q\n  Missing: %q\n", p.ID, p.Username, p.DisplayName, p.Decision, p.ProfileURL, p.Company, companyNote, p.Role, roleNote, p.Location, p.Matches, p.Contradictions, p.Missing)
		if len(p.CompanyAlternatives) > 0 {
			fmt.Printf("  Company alternatives (audit): %q\n", p.CompanyAlternatives)
		}
		if len(p.RoleAlternatives) > 0 {
			fmt.Printf("  Role claims (not necessarily concurrent): %q\n", p.RoleAlternatives)
		}
		for _, claim := range p.RoleClaims {
			fmt.Printf("  Role detail: %q | %s | %s | relations=%q\n", claim.Value, claim.Kind, claim.Temporal, claim.RelationIDs)
		}
		for _, claim := range p.CompanyClaims {
			fmt.Printf("  Company detail: %q | %s | %s | relations=%q\n", claim.Value, claim.Kind, claim.Temporal, claim.RelationIDs)
		}
		if len(p.EducationClaims) == 0 {
			fmt.Println("  Education: not established from collected sources")
		}
		for _, claim := range p.EducationClaims {
			fmt.Printf("  Education: %q (unverified source claim) | %s | relations=%q\n", claim.Value, claim.Temporal, claim.RelationIDs)
		}
		for _, r := range c.Relations {
			if r.CandidateID != p.ID {
				continue
			}
			fmt.Printf("  Relationship %s [%s, %s, %s, review=%s]: role=%q organization=%q", r.ID, r.Status, r.Temporal, r.Kind, domain.RelationDecision(r), r.Role, r.Organization)
			if r.StartYear != 0 {
				fmt.Printf(" start=%d", r.StartYear)
			}
			if r.EndYear != 0 {
				fmt.Printf(" end=%d", r.EndYear)
			}
			fmt.Printf("\n    method=%s evidence=%s quote=%q\n", r.Method, r.EvidenceID, r.Quote)
			if r.SubjectEvidenceID != "" {
				fmt.Printf("    Subject evidence=%s title=%q\n", r.SubjectEvidenceID, r.SubjectQuote)
			}
			for _, e := range c.Evidence {
				if e.ID == r.EvidenceID || e.ID == r.SubjectEvidenceID {
					for _, s := range e.Sources {
						fmt.Printf("    Source: %s (observed %s)\n", s.URL, s.ObservedAt)
					}
				}
			}
		}
		for _, run := range c.Extraction {
			if run.CandidateID == p.ID {
				fmt.Printf("  Extraction: %s %s accepted=%d rejected=%d model=%q notes=%q\n", run.Mode, run.Status, run.Accepted, run.Rejected, run.Model, run.Notes)
				for _, rejection := range run.Rejections {
					fmt.Printf("    Rejected proposal %d: %s | evidence=%s | %s\n", rejection.Index, rejection.Code, rejection.Proposal.EvidenceID, rejection.Detail)
				}
			}
		}
		fmt.Println("  Public observations:")
		for _, e := range c.Evidence {
			if e.CandidateID == p.ID {
				value := []rune(e.Value)
				status := ""
				if e.Status != "OBSERVED" {
					status = " [" + e.Status + "]"
				}
				if e.SupersededBy != "" {
					status += " [superseded by " + e.SupersededBy + "]"
				}
				audit := ""
				if e.InferenceRule != "" {
					audit = " (rule=" + e.InferenceRule
					if e.InferenceQuote != "" {
						audit += fmt.Sprintf(", quote=%q", e.InferenceQuote)
					}
					if e.InferenceQuoteType != "" {
						audit += ", quote_type=" + e.InferenceQuoteType
					}
					audit += ")"
				}
				if (e.Type == "page_text" || e.Type == "page_links" || e.Type == "person_jsonld") && len(value) > 400 {
					fmt.Printf("    %s%s%s: %q (preview; full observation in saved JSON)\n", e.Type, status, audit, string(value[:400]))
				} else {
					fmt.Printf("    %s%s%s: %q\n", e.Type, status, audit, e.Value)
				}
			}
		}
		if p.Username != "" && p.DisplayName == "" && p.Company == "" && p.Role == "" && p.Location == "" {
			fmt.Println("  The account was retrieved, but public name, company, role, and location were not provided by this source.")
		}
	}
}
