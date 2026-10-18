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
	source := fs.String("source", "", "github, google (no API), brave, google-api, url, or bluesky")
	platform := fs.String("platform", "", "social platform name or all for indexed discovery")
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
		return fmt.Errorf("unexpected positional arguments")
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
		} else if *name != "" || *platform != "" {
			provider = "google"
		} else {
			provider = "github"
		}
	}
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	var client collector.Collector
	switch provider {
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
	if *platform != "" && provider != "google" && provider != "brave" && provider != "google-api" {
		return fmt.Errorf("-platform is only supported by search providers")
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
	printCandidates(project)
	fmt.Println("Scope: " + project.Search.Scope)
	if len(project.Candidates) > 0 {
		fmt.Println("Review: pmiyc review -input <project.json> -candidate <candidate-id> -decision CONFIRM -out <reviewed.json>")
	}
	if project.Search.Status != "BLOCKED" && provider != "url" && project.Search.Attempts < cfg.MaxAttempts && project.Search.RequestsUsed < cfg.RequestBudget {
		fmt.Println("Need a closer match? Refine with search -input <project.json> -username <known-username>, -company <company>, or -location <location>.")
	}
	if project.Search.Status == "BLOCKED" {
		fmt.Println("Access was blocked. Check the diagnostic below; a saved project does not mean collection succeeded.")
	}
	for _, ev := range project.Search.Events {
		if ev.Status != "SUCCESS" {
			fmt.Printf("%s: %s (%s)\n", ev.Operation, ev.Status, ev.Detail)
		}
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
	}
}
