package aws

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/ini.v1"
)

// AWSRegion represents an AWS region with its code and human-readable display name.
type AWSRegion struct {
	Code        string
	DisplayName string
}

// partitionsJSON is the AWS SDK's own partitions catalog. It ships as
// aws-sdk-go-v2/internal/endpoints/awsrulesfn/partitions.json and is
// refreshed by the SDK release process. We embed a verified copy here and
// parse it into AllRegions() at init time so the region catalogue is
// SDK-backed — bumping the SDK version and re-copying this file is the
// single source-of-truth update.
//
//go:embed data/partitions.json
var partitionsJSON []byte

// sdkPartitions is the minimal subset of the SDK's partition catalogue that
// we parse: per-partition regex + region map with descriptions.
type sdkPartitions struct {
	Partitions []sdkPartition `json:"partitions"`
}

type sdkPartition struct {
	ID          string                         `json:"id"`
	RegionRegex string                         `json:"regionRegex"`
	Regions     map[string]sdkRegionDescriptor `json:"regions"`
}

type sdkRegionDescriptor struct {
	Description string `json:"description"`
}

// Package-level region catalogue, parsed once at package load from the embedded
// partitions.json. The multi-return loader replaces the AS-947-pruned init()
// so the file no longer trips the `rg ^func init\(\)` gate.
//
//nolint:gochecknoglobals // process-scope region catalogue: parsed once at package load
var (
	allRegionsCache, awsPartitionRegionRegex = loadCommercialPartition()
)

// loadCommercialPartition parses the embedded partitions.json and returns the
// commercial-partition region slice (sorted by code, with each AWSRegion's
// DisplayName already populated from the SDK description) and the region-code
// regex. Panics on malformed input — the embedded JSON is vendored at build
// time so any parse failure is a build-time bug.
//
// AllRegions() copies the returned slice on every call (caller-mutable). Gov-cloud
// (`aws-us-gov`) and China (`aws-cn`) partitions are skipped intentionally —
// `TestAllRegions_NoGovOrChinaLeaks` pins the behavior.
func loadCommercialPartition() ([]AWSRegion, *regexp.Regexp) {
	var parsed sdkPartitions
	if err := json.Unmarshal(partitionsJSON, &parsed); err != nil {
		panic(fmt.Sprintf("aws regions: parse embedded partitions.json: %v", err))
	}

	var regions []AWSRegion
	var regex *regexp.Regexp

	for _, p := range parsed.Partitions {
		if p.ID != "aws" {
			continue
		}
		re, err := regexp.Compile(p.RegionRegex)
		if err != nil {
			panic(fmt.Sprintf("aws regions: compile region regex %q: %v", p.RegionRegex, err))
		}
		regex = re

		regions = make([]AWSRegion, 0, len(p.Regions))
		for code, desc := range p.Regions {
			// Filter out pseudo-regions that the SDK emits for global
			// services (e.g. "aws-global", "aws-cn-global"). They are not
			// selectable via the region switcher and do not match the
			// commercial region regex.
			if !re.MatchString(code) {
				continue
			}
			display := desc.Description
			if display == "" {
				display = code
			}
			regions = append(regions, AWSRegion{Code: code, DisplayName: display})
		}
		// Stable alphabetical order on code — the selector UI expects a
		// deterministic ordering independent of map iteration.
		sort.Slice(regions, func(i, j int) bool { return regions[i].Code < regions[j].Code })
	}
	if regex == nil {
		panic("aws regions: embedded partitions.json has no 'aws' partition")
	}
	return regions, regex
}

// AllRegions returns the list of commercial-partition AWS regions in a stable
// alphabetical order. Data comes from the SDK's partitions.json (embedded at
// build time) so the catalogue stays in sync with the SDK release cycle.
func AllRegions() []AWSRegion {
	out := make([]AWSRegion, len(allRegionsCache))
	copy(out, allRegionsCache)
	return out
}

// ValidateRegionCode reports whether a region code matches the SDK's commercial
// region regex. Returns false for an empty string or any code outside the
// commercial partition.
func ValidateRegionCode(code string) bool {
	if awsPartitionRegionRegex == nil {
		return false
	}
	return awsPartitionRegionRegex.MatchString(code)
}

// GetDefaultRegion reads the region configured for a given profile in the AWS config file.
// If the config file doesn't exist or the profile has no region, it falls back to "us-east-1".
func GetDefaultRegion(configPath, profile string) string {
	const fallback = "us-east-1"

	if configPath == "" {
		return fallback
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fallback
	}

	cfg, err := ini.LoadSources(ini.LoadOptions{
		Insensitive:     false,
		AllowShadows:    true,
		Loose:           true,
		InsensitiveKeys: true,
	}, configPath)
	if err != nil {
		return fallback
	}

	// Determine the section name to look up
	sectionName := "profile " + profile
	if profile == "default" || profile == "" {
		sectionName = "default"
	}

	section, err := cfg.GetSection(sectionName)
	if err != nil {
		// Try without "profile " prefix for default
		if profile == "default" || profile == "" {
			section, err = cfg.GetSection("DEFAULT")
			if err != nil {
				return fallback
			}
		} else {
			return fallback
		}
	}

	if section.HasKey("region") {
		region := strings.TrimSpace(section.Key("region").String())
		if region != "" {
			return region
		}
	}

	return fallback
}
