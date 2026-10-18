package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/changes"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/storage"
)

func runCompare(args []string) error {
	fs := flag.NewFlagSet("compare", flag.ContinueOnError)
	beforePath := fs.String("before", "", "earlier direct snapshot")
	afterPath := fs.String("after", "", "later direct snapshot")
	out := fs.String("out", "", "new report path; otherwise print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *beforePath == "" || *afterPath == "" {
		return fmt.Errorf("compare requires -before and -after snapshot paths")
	}
	var before, after domain.Case
	if err := storage.Read(*beforePath, &before); err != nil {
		return err
	}
	if err := storage.Read(*afterPath, &after); err != nil {
		return err
	}
	report, err := changes.Compare(before, after)
	if err != nil {
		return err
	}
	if *out != "" {
		return storage.WriteNew(*out, report)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}
