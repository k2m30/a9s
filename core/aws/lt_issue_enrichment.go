// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package aws — lt_issue_enrichment.go: IMDSv1, user-data secret and
// deprecated-AMI signals for Launch Templates.
//
// The deprecated-AMI signal makes no AWS API call: it scans the
// already-loaded "ami" ResourceCache entry, in the same cache-scan layer as
// snapshot_cross_ref.go. The IMDSv1 signal reads the AMIs of templates that
// leave HttpTokens unset in one batched DescribeImages walk.
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
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// EnrichLTDeprecatedAMI raises the IMDSv1 finding (ltIMDSv1Findings) and
// cross-references each Launch Template's "$Default" ImageId against the
// already-loaded "ami" cache: an ami-prefixed id whose cached AMI has a past
// DeprecationTime is a "deprecated AMI" Warning. When the "ami" list has not
// been loaded this session, every template on an ami- image is marked not
// inspected, as EnrichSnapshotCrossRef requires its parent list to be loaded.
func EnrichLTDeprecatedAMI(ctx context.Context, clients *ServiceClients, resources []resource.Resource, cache resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:         make(map[string][]domain.Finding),
		AttentionDetails: make(map[string]map[domain.FindingCode]domain.AttentionDetail),
		TruncatedIDs:     make(map[string]string),
	}
	var imagesAPI EC2DescribeImagesAPI
	if clients != nil && clients.EC2 != nil {
		imagesAPI = clients.EC2
	}
	imdsErr := ltIMDSv1Findings(ctx, imagesAPI, resources, &result)
	ebsErr := ltUnencryptedFindings(ctx, imagesAPI, resources, &result)

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

	// An AMI absent from an unloaded or truncated ami list may be a deprecated
	// one on a page nobody read, so the template reads not inspected.
	amiEntry, amiLoaded := cache["ami"]
	complete := amiLoaded && !amiEntry.IsTruncated
	listed := make(map[string]bool, len(amiEntry.Resources))
	deprecatedByID := make(map[string]bool, len(amiEntry.Resources))
	for _, amiRes := range amiEntry.Resources {
		listed[amiRes.ID] = true
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

	for _, res := range resources {
		raw, ok := assertStruct[LTRaw](res.RawStruct)
		if !ok || raw.DefaultVersion.LaunchTemplateData == nil {
			continue
		}
		imageID := aws.ToString(raw.DefaultVersion.LaunchTemplateData.ImageId)
		switch {
		case !strings.HasPrefix(imageID, "ami-"):
		case deprecatedByID[imageID]:
			setWave2Finding(&result, res.ID, ltCodeDeprecatedAMI, []domain.DetailRow{{Label: "AMI", Value: imageID, Tier: tierOf(ltCodeDeprecatedAMI)}})
		case !listed[imageID] && !complete:
			markUninspected(&result, res.ID, checkListIncomplete("ami"))
		}
	}

	return result, errors.Join(imdsErr, ebsErr)
}

// ltUnencryptedFindings raises ltCodeUnencrypted on every template whose
// "$Default" version maps a block device with Encrypted=false explicitly. A
// nil Encrypted is the account default, not off, and is never flagged. The
// explicit false loses to the Region's EBS encryption by default: "If you
// enable it for a Region, you cannot disable it for individual volumes or
// snapshots in that Region" (Amazon EBS User Guide, "Enable Amazon EBS
// encryption by default"). That setting is one GetEbsEncryptionByDefault
// read, made only when a template sets Encrypted=false; a failed read leaves
// those templates not inspected. With no EC2 client they are not inspected;
// a client without the call reads the AWS default, off.
func ltUnencryptedFindings(ctx context.Context, api EC2DescribeImagesAPI, resources []resource.Resource, result *IssueEnricherResult) error {
	var offIDs []string
	for _, res := range resources {
		raw, ok := assertStruct[LTRaw](res.RawStruct)
		if !ok || raw.DefaultVersion.LaunchTemplateData == nil {
			continue
		}
		if slices.ContainsFunc(raw.DefaultVersion.LaunchTemplateData.BlockDeviceMappings, func(bdm ec2types.LaunchTemplateBlockDeviceMapping) bool {
			return bdm.Ebs != nil && bdm.Ebs.Encrypted != nil && !*bdm.Ebs.Encrypted
		}) {
			offIDs = append(offIDs, res.ID)
		}
	}
	if len(offIDs) == 0 {
		return nil
	}
	if api == nil {
		for _, id := range offIDs {
			markUninspected(result, id, imdsCheckNoClient)
		}
		return nil
	}
	forced := false
	if defaultAPI, ok := api.(EC2GetEbsEncryptionByDefaultAPI); ok {
		out, err := defaultAPI.GetEbsEncryptionByDefault(ctx, &ec2.GetEbsEncryptionByDefaultInput{})
		if err != nil {
			var failures []Failure
			for _, id := range offIDs {
				MarkSkipped(result, id, &failures, err)
			}
			return AggregateFailures("GetEbsEncryptionByDefault", failures, len(offIDs))
		}
		forced = aws.ToBool(out.EbsEncryptionByDefault)
	}
	if forced {
		return nil
	}
	for _, id := range offIDs {
		setWave2Finding(result, id, ltCodeUnencrypted, nil)
	}
	return nil
}

// ltIMDSv1Findings hands every template's "$Default" metadata settings to
// raiseIMDSv1, which owns the IMDSv1 decision.
func ltIMDSv1Findings(ctx context.Context, api EC2DescribeImagesAPI, resources []resource.Resource, result *IssueEnricherResult) error {
	var launches []imdsLaunch
	for _, res := range resources {
		raw, ok := assertStruct[LTRaw](res.RawStruct)
		if !ok || raw.DefaultVersion.LaunchTemplateData == nil {
			continue
		}
		data := raw.DefaultVersion.LaunchTemplateData
		l := imdsLaunch{ID: res.ID, ImageID: aws.ToString(data.ImageId)}
		if md := data.MetadataOptions; md != nil {
			l.Endpoint = string(md.HttpEndpoint)
			l.Tokens = string(md.HttpTokens)
		}
		launches = append(launches, l)
	}
	return raiseIMDSv1(ctx, api, launches, result, ltCodeIMDSv1, func(imdsLaunch) []domain.DetailRow { return nil })
}
