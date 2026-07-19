// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
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
// partitions.json via a multi-return loader (no package init()).
//
//nolint:gochecknoglobals // process-scope region catalogue: parsed once at package load
var (
	allRegionsCache = loadCommercialPartition()
)

// loadCommercialPartition parses the embedded partitions.json and returns the
// commercial-partition region slice (sorted by code, with each AWSRegion's
// DisplayName already populated from the SDK description). Panics on
// malformed input — the embedded JSON is vendored at build time so any parse
// failure is a build-time bug.
//
// AllRegions() copies the returned slice on every call (caller-mutable). Gov-cloud
// (`aws-us-gov`) and China (`aws-cn`) partitions are skipped intentionally —
// `TestAllRegions_NoGovOrChinaLeaks` pins the behavior.
func loadCommercialPartition() []AWSRegion {
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
	return regions
}

// AllRegions returns the list of commercial-partition AWS regions in a stable
// alphabetical order. Data comes from the SDK's partitions.json (embedded at
// build time) so the catalogue stays in sync with the SDK release cycle.
func AllRegions() []AWSRegion {
	out := make([]AWSRegion, len(allRegionsCache))
	copy(out, allRegionsCache)
	return out
}

// maxSourceProfileDepth bounds the source_profile chain walk in
// resolveProfileRegion so a cyclical or accidentally-long chain in a
// malformed config file cannot loop or recurse unbounded.
const maxSourceProfileDepth = 5

// GetDefaultRegion resolves the effective region for a given profile the same
// way the AWS SDK's static config chain does (no network calls, no SSO/STS):
//
//  1. AWS_REGION, then AWS_DEFAULT_REGION environment variables.
//  2. The profile's own `region` key in the AWS config file.
//  3. Following `source_profile` links recursively (cycle-guarded, capped at
//     maxSourceProfileDepth hops), taking the first `region` found along the
//     chain — this is what lets an assume-role profile (role_arn +
//     source_profile, no region of its own) inherit its source profile's
//     region.
//  4. The `[default]` section's `region` key.
//  5. "us-east-1" as a last-resort fallback.
func GetDefaultRegion(configPath, profile string) string {
	const fallback = "us-east-1"

	if region := strings.TrimSpace(os.Getenv("AWS_REGION")); region != "" {
		return region
	}
	if region := strings.TrimSpace(os.Getenv("AWS_DEFAULT_REGION")); region != "" {
		return region
	}

	if configPath == "" {
		return fallback
	}

	sections, err := parseINISections(configPath)
	if err != nil {
		return fallback
	}

	lookupProfile := profile
	if lookupProfile == "" {
		lookupProfile = "default"
	}

	if region, ok := resolveProfileRegion(sections, lookupProfile, make(map[string]bool)); ok {
		return region
	}

	if region, ok := sectionRegion(sections, "default"); ok {
		return region
	}
	if region, ok := sectionRegion(sections, "DEFAULT"); ok {
		return region
	}

	return fallback
}

// resolveProfileRegion looks up the region for profile directly, then —
// when absent — follows the profile's source_profile link recursively.
// visited guards against cycles (A -> B -> A) and, via its growing size,
// bounds the walk to maxSourceProfileDepth hops.
func resolveProfileRegion(sections []iniSection, profile string, visited map[string]bool) (string, bool) {
	if profile == "" || visited[profile] || len(visited) >= maxSourceProfileDepth {
		return "", false
	}
	visited[profile] = true

	sectionName := "profile " + profile
	if profile == "default" {
		sectionName = "default"
	}

	section, ok := findINISection(sections, sectionName)
	if !ok {
		if profile == "default" {
			section, ok = findINISection(sections, "DEFAULT")
		}
		if !ok {
			return "", false
		}
	}

	if region := strings.TrimSpace(section.keys["region"]); region != "" {
		return region, true
	}

	if sourceProfile := strings.TrimSpace(section.keys["source_profile"]); sourceProfile != "" {
		return resolveProfileRegion(sections, sourceProfile, visited)
	}

	return "", false
}

// sectionRegion reads the region key from a named section, reporting ok=false
// when the section or the key is missing/empty.
func sectionRegion(sections []iniSection, sectionName string) (string, bool) {
	section, ok := findINISection(sections, sectionName)
	if !ok {
		return "", false
	}
	region := strings.TrimSpace(section.keys["region"])
	if region == "" {
		return "", false
	}
	return region, true
}
