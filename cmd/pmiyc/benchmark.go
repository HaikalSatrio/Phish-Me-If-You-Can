package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/benchmark"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/extract"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/storage"
)

func runBenchmark(args []string) error {
	fs := flag.NewFlagSet("benchmark", flag.ContinueOnError)
	input := fs.String("corpus", "fixtures/benchmark-exploratory-v1.json", "labeled frozen index results and observations")
	out := fs.String("out", "", "new JSON report path")
	model := fs.String("model", "", "installed Ollama model; adds a hybrid run on identical inputs")
	endpoint := fs.String("ollama-url", "http://127.0.0.1:11434", "local Ollama origin")
	timeout := fs.Int("timeout", 30, "seconds per local model request")
	deadline := fs.Int("deadline", 600, "total seconds for the benchmark")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *out == "" || *deadline < 1 || *deadline > 3600 {
		return fmt.Errorf("benchmark requires -out and a deadline of 1-3600 seconds")
	}
	if _, err := os.Stat(*out); err == nil {
		return fmt.Errorf("output already exists: %s", *out)
	} else if !os.IsNotExist(err) {
		return err
	}
	var corpus benchmark.Corpus
	if err := storage.Read(*input, &corpus); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, time.Duration(*deadline)*time.Second)
	defer cancel()
	report, err := benchmark.Execute(ctx, corpus, extract.Config{Model: *model, URL: *endpoint, TimeoutSeconds: *timeout, MaxRequests: 20}, func(s string) { fmt.Fprintln(os.Stderr, s) })
	if err != nil {
		return err
	}
	if err := storage.WriteNew(*out, report); err != nil {
		return err
	}
	fmt.Printf("Saved benchmark: %s\nCorpus SHA256: %s\nSynthetic exploratory results; live web recall and identity accuracy are not measured.\n", *out, report.CorpusSHA256)
	fmt.Printf("Candidate filter: TP=%d FP=%d FN=%d\n", report.CandidateFilter.TP, report.CandidateFilter.FP, report.CandidateFilter.FN)
	for _, mode := range []string{"rules", "hybrid"} {
		m := report.Metrics[mode]
		if m == nil {
			continue
		}
		fmt.Printf("%s: %d/%d cases completed, claims TP=%d FP=%d FN=%d, correct abstentions=%d/%d, time=%dms\n", mode, m.Completed, m.Cases, m.Claims.TP, m.Claims.FP, m.Claims.FN, m.CorrectAbstentions, m.NegativeCases, m.DurationMS)
		if mode == "hybrid" {
			fmt.Printf("  Model calls: success=%d failed=%d skipped=%d; rejected proposals=%d\n", m.ModelSuccess, m.ModelFailed, m.ModelSkipped, m.RejectedProposals)
		}
	}
	codes := []string{}
	for code := range report.Failures {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	for _, code := range codes {
		fmt.Printf("  %s: %d cases\n", code, report.Failures[code])
	}
	if ctx.Err() != nil {
		return fmt.Errorf("benchmark interrupted; partial report saved: %w", ctx.Err())
	}
	return nil
}
