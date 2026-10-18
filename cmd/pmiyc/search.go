package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/collector"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/search"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/storage"
)

func runSearch(args []string) error {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	name := fs.String("name", "", "subject name")
	username := fs.String("username", "", "known account username or Bluesky handle")
	source := fs.String("source", "", "direct (HTML, no API), web/bing, github (API), google (browser), brave, google-api, url, or bluesky")
	platform := fs.String("platform", "", "platform name or all; direct uses implemented adapters, web uses indexed discovery")
	pageURL := fs.String("url", "", "explicit public page URL")
	company := fs.String("company", "", "company clue for comparison")
	location := fs.String("location", "", "location clue for comparison")
	input := fs.String("input", "", "saved search session to refine")
	out := fs.String("out", "", "new output file, default data/<session>-<attempt>.json")
	config := fs.String("config", "configs/collector-v1.json", "collector configuration")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments %q; supply one value after -username, for example: search -source github -username HaikalSatrio", fs.Args())
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
		if *pageURL != "" {
			provider = "url"
		} else if *username != "" {
			provider = "direct"
		} else if *name != "" || *platform != "" {
			provider = "web"
		} else {
			provider = "github"
		}
	}
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	var client collector.Collector
	switch provider {
	case "direct":
		client = collector.NewDirect(timeout)
	case "web", "bing":
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
	if *platform != "" && provider != "direct" && provider != "web" && provider != "bing" && provider != "google" && provider != "brave" && provider != "google-api" {
		return fmt.Errorf("-platform is supported by direct collection and search providers")
	}
	if *pageURL != "" && provider != "url" {
		return fmt.Errorf("use -source url with -url")
	}
	project, err := search.Run(ctx, client, search.Input{Name: *name, Username: *username, Company: *company, Location: *location, Provider: provider, Platform: *platform, URL: *pageURL}, cfg, previous)
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
	fmt.Printf("Saved project: %s\nStatus: %s\nCollection attempts used: %d/%d\n", path, project.Search.Status, project.Search.RequestsUsed, cfg.RequestBudget)
	fmt.Printf("Collected: %d candidates, %d observations\n", len(project.Candidates), len(project.Evidence))
	if provider == "web" || provider == "bing" {
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
	if len(project.Candidates) == 0 && (project.Search.Status == "BLOCKED" || project.Search.Status == "FAILED") {
		return fmt.Errorf("collection did not succeed; diagnostics saved to %s", path)
	}
	return nil
}

func printCandidates(c domain.Case) {
	if len(c.Candidates) == 0 {
		fmt.Println("No candidate profiles collected. Check source status before drawing conclusions.")
	}
	for _, p := range c.Candidates {
		if p.Label != "" {
			fmt.Printf("Page title: %q\n", p.Label)
		}
		fmt.Printf("\n%s | user=%q name=%q decision=%s\n  URL: %q\n  Company: %q Location: %q\n  Matches: %q\n  Contradictions: %q\n  Missing: %q\n", p.ID, p.Username, p.DisplayName, p.Decision, p.ProfileURL, p.Company, p.Location, p.Matches, p.Contradictions, p.Missing)
		fmt.Println("  Public observations:")
		for _, e := range c.Evidence {
			if e.CandidateID == p.ID {
				fmt.Printf("    %s: %q\n", e.Type, e.Value)
			}
		}
		if p.Username != "" && p.DisplayName == "" && p.Company == "" && p.Location == "" {
			fmt.Println("  The account was retrieved, but public name, company, and location were not provided by this source.")
		}
	}
}
