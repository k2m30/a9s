// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package aws — lt_issue_enrichment.go: deprecated-AMI cross-cache signal
// for Launch Templates.
//
// A Wave 2 IssueEnricher that makes zero AWS API calls, scanning the
// already-loaded "ami" ResourceCache entry instead of the fetcher's own
// client, in the same cache-scan layer as snapshot_cross_ref.go.
//
// The deprecated-ami signal (docs/resources/lt.md):
//   - fires only when the "$Default" version's ImageId is ami-prefixed AND
//     present in the loaded "ami" cache AND that AMI's DeprecationTime is
//     past.
//   - NOT-in-cache (a public/marketplace AMI, or a resolve:ssm: reference,
//     which never matches the ami- prefix) is never a signal — absence is
//     non-definitive, not "deregistered".
//   - Idempotent: ApplyWave2ToRow (core/runtime/helpers.go) strips prior
//     wave2: findings before merging fresh ones, so repeated runs never
//     double-append.
package aws

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// EnrichLTDeprecatedAMI cross-references each Launch Template's "$Default"
// ImageId against the already-loaded "ami" cache: an ami-prefixed id whose
// cached AMI has a past DeprecationTime is a "deprecated AMI" Warning. Zero
// AWS API calls. When the "ami" list has not been loaded this session,
// every template on an ami- image is marked not inspected, as
// EnrichSnapshotCrossRef requires its parent list to be loaded.
func EnrichLTDeprecatedAMI(_ context.Context, _ *ServiceClients, resources []resource.Resource, cache resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:         make(map[string][]domain.Finding),
		AttentionDetails: make(map[string]map[domain.FindingCode]domain.AttentionDetail),
		TruncatedIDs:     make(map[string]string),
	}

	// The default version is already in hand from Wave 1 — no API call — so
	// the user-data scan runs whether or not the ami cache is loaded.
	for _, res := range resources {
		raw, ok := assertStruct[LTRaw](res.RawStruct)
		if !ok || raw.DefaultVersion.LaunchTemplateData == nil {
			continue
		}
		userData := aws.ToString(raw.DefaultVersion.LaunchTemplateData.UserData)
		if userData == "" {
			continue
		}
		hitRows := secretScanTextRows(decodeUserData(userData))
		if len(hitRows) == 0 {
			continue
		}
		rows := append([]domain.DetailRow{{
			Label: "Version",
			Value: strconv.FormatInt(aws.ToInt64(raw.DefaultVersion.VersionNumber), 10),
			Tier:  "!",
		}}, hitRows...)
		setWave2Finding(&result, res.ID, ltCodeUserDataSecret, rows)

	}

	amiEntry, amiLoaded := cache["ami"]
	if !amiLoaded {
		for _, res := range resources {
			if raw, ok := assertStruct[LTRaw](res.RawStruct); ok && raw.DefaultVersion.LaunchTemplateData != nil &&
				strings.HasPrefix(aws.ToString(raw.DefaultVersion.LaunchTemplateData.ImageId), "ami-") {
				markUninspected(&result, res.ID, checkListIncomplete("ami"))
			}
		}
		return result, nil
	}

	deprecatedByID := make(map[string]bool, len(amiEntry.Resources))
	for _, amiRes := range amiEntry.Resources {
		img, ok := assertStruct[ec2types.Image](amiRes.RawStruct)
		if !ok || img.DeprecationTime == nil || *img.DeprecationTime == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339, *img.DeprecationTime)
		if err != nil {
			continue
		}
		if time.Now().After(t) {
			deprecatedByID[amiRes.ID] = true
		}
	}
	if len(deprecatedByID) == 0 {
		return result, nil
	}

	for _, res := range resources {
		raw, ok := assertStruct[LTRaw](res.RawStruct)
		if !ok || raw.DefaultVersion.LaunchTemplateData == nil {
			continue
		}
		imageID := aws.ToString(raw.DefaultVersion.LaunchTemplateData.ImageId)
		if !strings.HasPrefix(imageID, "ami-") || !deprecatedByID[imageID] {
			continue
		}
		setWave2Finding(&result, res.ID, ltCodeDeprecatedAMI, []domain.DetailRow{{Label: "AMI", Value: imageID, Tier: tierOf(ltCodeDeprecatedAMI)}})

	}

	return result, nil
}
