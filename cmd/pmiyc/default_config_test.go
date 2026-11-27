package main

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/search"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/storage"
)

func TestDefaultSearchRecordsConservativeAccessConfiguration(t *testing.T) {
	t.Chdir("../..")
	path := filepath.Join(t.TempDir(), "invalid-username.json")
	if err := runSearch([]string{"-username", "!", "-out", path}); err == nil {
		t.Fatal("invalid username reported success")
	}
	var c domain.Case
	if err := storage.Read(path, &c); err != nil {
		t.Fatal(err)
	}
	if c.Search == nil || c.Search.RequestsUsed != 0 {
		t.Fatal("invalid input must not consume network requests")
	}
	var cfg search.Config
	if err := json.Unmarshal([]byte(c.Search.ConfigJSON), &cfg); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if cfg.Version != "public_collection_default_v2" || cfg.Access == nil {
		t.Fatalf("default access configuration missing: %+v", cfg)
	}
	if cfg.Access.DNSResolver != "system" || len(cfg.Access.DNSHosts) != 0 || len(cfg.Access.ClassicalTLSHosts) != 0 || cfg.Access.MinIntervalMS < 1000 || cfg.Access.SearchIntervalMS < 6000 || cfg.Access.MaxRetries != 1 || cfg.Access.MaxCooldownWaitMS != 5000 {
		t.Fatalf("unexpected default transport policy: %+v", cfg.Access)
	}
}
