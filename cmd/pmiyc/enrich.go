package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/enrich"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/extract"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/storage"
)

func runEnrich(args []string) error {
	fs := flag.NewFlagSet("enrich", flag.ContinueOnError)
	input := fs.String("input", "", "saved project to replay without web requests")
	out := fs.String("out", "", "new output file; original is preserved")
	model := fs.String("model", "", "installed local Ollama model; omit for rules only")
	endpoint := fs.String("ollama-url", "http://127.0.0.1:11434", "local Ollama origin")
	timeout := fs.Int("timeout", 90, "timeout in seconds per local request (1-300)")
	budget := fs.Int("max-requests", 5, "maximum local model requests (1-20)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *input == "" || *out == "" || fs.NArg() != 0 {
		return fmt.Errorf("enrich requires -input and -out; use -model to enable local extraction")
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
	cfg := extract.Config{URL: *endpoint, Model: *model, TimeoutSeconds: *timeout, MaxRequests: *budget}
	if *model != "" {
		cfg.Mode = "hybrid"
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	result, err := enrich.Run(ctx, c, cfg)
	if err != nil {
		return err
	}
	if err := storage.WriteNew(*out, result); err != nil {
		return err
	}
	fmt.Printf("Saved enrichment: %s\n", *out)
	printCandidates(result)
	return nil
}
