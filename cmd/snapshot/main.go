// Command snapshot downloads raw resource facts for EVERY resource type a9s
// can display, from a real read-only AWS account, into one big JSON file. It
// uses nothing but the AWS SDK — no a9s packages, no a9s binary involvement.
// The output is the comparison baseline the checklist generator (cmd/checklist) turns into
// expected screen checklists; refresh it any time by re-running this command.
//
// The profile name is a runtime flag and never stored in code; the output
// directory is gitignored (real resource names).
//
//	go run ./cmd/snapshot --profile my-real-profile --region eu-west-2
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
)

// captureFunc downloads the raw facts for one resource type. Implementations
// live in per-family files (ec2_network.go, databases.go, …), use plain SDK
// clients only, and return a JSON-marshalable struct of raw AWS facts.
type captureFunc func(ctx context.Context, cfg aws.Config) (any, error)

// snapshotFile is the single big JSON written per profile+region.
type snapshotFile struct {
	Region      string            `json:"region"`
	CollectedAt string            `json:"collected_at"`
	Types       map[string]any    `json:"types"`
	Errors      map[string]string `json:"errors,omitempty"`
}

func main() {
	var (
		profile = flag.String("profile", "", "AWS profile to read (required; never persisted)")
		region  = flag.String("region", "", "AWS region (defaults to the profile's region)")
		out     = flag.String("out", "tests/e2e/testdata/snapshot", "output root")
		types   = flag.String("types", "all", `comma-separated resource short names, or "all"`)
	)
	flag.Parse()

	if *profile == "" {
		log.Fatal("snapshot: --profile is required")
	}

	ctx := context.Background()
	cfg, err := awscfg.LoadDefaultConfig(ctx, awscfg.WithSharedConfigProfile(*profile))
	if err != nil {
		log.Fatalf("snapshot: loading profile: %v", err)
	}
	if *region != "" {
		cfg.Region = *region
	}
	if cfg.Region == "" {
		log.Fatal("snapshot: no region — pass --region or set one in the profile")
	}

	selected := captureOrder
	if *types != "all" {
		selected = nil
		for t := range strings.SplitSeq(*types, ",") {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			if _, ok := captures[t]; !ok {
				log.Fatalf("snapshot: unknown type %q", t)
			}
			selected = append(selected, t)
		}
	}

	dir := filepath.Join(*out, *profile+"--"+cfg.Region)
	path := filepath.Join(dir, "snapshot.json")

	snap := snapshotFile{
		Region:      cfg.Region,
		CollectedAt: time.Now().UTC().Format(time.RFC3339),
		Types:       make(map[string]any, len(selected)),
		Errors:      make(map[string]string),
	}
	// A partial run (--types x,y) refreshes just those types in the existing
	// file instead of clobbering the other 60+ — the file stays the one big
	// snapshot of everything. Carried sections stay json.RawMessage so their
	// numbers never round-trip through float64 (int64 facts above 2^53 would
	// silently lose precision otherwise).
	if *types != "all" {
		carried, carriedErrs := mergeSnapshotFile(path, selected)
		for k, v := range carried {
			snap.Types[k] = v
		}
		maps.Copy(snap.Errors, carriedErrs)
	}

	// One type failing (missing permission, unavailable service) must not sink
	// the other 65 — record the error and keep going.
	failed := 0
	for _, name := range selected {
		start := time.Now()
		data, err := captures[name](ctx, cfg)
		if err != nil {
			failed++
			snap.Errors[name] = err.Error()
			fmt.Fprintf(os.Stderr, "  %-14s ERROR %v\n", name, err)
			continue
		}
		snap.Types[name] = data
		delete(snap.Errors, name)
		fmt.Fprintf(os.Stderr, "  %-14s ok (%s)\n", name, time.Since(start).Round(time.Millisecond))
	}
	if len(snap.Errors) == 0 {
		snap.Errors = nil
	}

	if err := os.MkdirAll(dir, 0o750); err != nil {
		log.Fatalf("snapshot: %v", err)
	}
	if err := writeJSON(path, snap); err != nil {
		log.Fatalf("snapshot: %v", err)
	}
	fmt.Printf("snapshot: wrote %s (%d types, %d errors)\n", path, len(snap.Types), failed)
}

// mergeSnapshotFile loads the existing snapshot at path (if any) and returns
// the sections to carry over for a partial run: every type section EXCEPT
// those in selected, preserved byte-for-byte, plus prior errors minus selected.
// A missing or malformed file degrades to empty maps — a partial run on a
// fresh directory simply produces a partial snapshot.
func mergeSnapshotFile(path string, selected []string) (map[string]json.RawMessage, map[string]string) {
	types := make(map[string]json.RawMessage)
	errs := make(map[string]string)
	data, err := os.ReadFile(path)
	if err != nil {
		return types, errs
	}
	var old struct {
		Types  map[string]json.RawMessage `json:"types"`
		Errors map[string]string          `json:"errors"`
	}
	if err := json.Unmarshal(data, &old); err != nil {
		return types, errs
	}
	sel := make(map[string]bool, len(selected))
	for _, s := range selected {
		sel[s] = true
	}
	for k, v := range old.Types {
		if !sel[k] {
			types[k] = v
		}
	}
	for k, v := range old.Errors {
		if !sel[k] {
			errs[k] = v
		}
	}
	return types, errs
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
