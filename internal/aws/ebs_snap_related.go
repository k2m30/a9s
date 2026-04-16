package aws

import (
	"context"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

var ebsSnapCreateImageRe = regexp.MustCompile(`Created by CreateImage\((i-[a-zA-Z0-9]+)\)`)

// checkEBSSnapAMI scans the AMI cache for AMIs whose block device mappings reference this snapshot (Pattern C).
func checkEBSSnapAMI(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	snapID := res.ID
	if snapID == "" {
		return resource.RelatedCheckResult{TargetType: "ami", Count: 0}
	}

	amiList, truncated, err := ebsSnapRelatedResources(ctx, clients, cache, "ami")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ami", Count: -1, Err: err}
	}
	if amiList == nil {
		return resource.RelatedCheckResult{TargetType: "ami", Count: -1}
	}

	var ids []string
	for _, r := range amiList {
		img, ok := assertStruct[ec2types.Image](r.RawStruct)
		if !ok {
			continue
		}
		for _, bdm := range img.BlockDeviceMappings {
			if bdm.Ebs != nil && bdm.Ebs.SnapshotId != nil && *bdm.Ebs.SnapshotId == snapID {
				ids = append(ids, r.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "ami", Count: -1}
	}
	return relatedResult("ami", ids)
}

// checkEBSSnapEBS reads the source volume ID from Fields["volume_id"] (Pattern F).
func checkEBSSnapEBS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	volumeID := res.Fields["volume_id"]
	if volumeID == "" {
		return resource.RelatedCheckResult{TargetType: "ebs", Count: 0}
	}
	return relatedResult("ebs", []string{volumeID})
}

// checkEBSSnapEC2 parses the snapshot Description for "Created by CreateImage(i-xxx)" (Pattern F).
func checkEBSSnapEC2(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	description := res.Fields["description"]
	matches := ebsSnapCreateImageRe.FindStringSubmatch(description)
	if len(matches) < 2 {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: 0}
	}
	return relatedResult("ec2", []string{matches[1]})
}

// checkEBSSnapKMS extracts the KMS key ID from RawStruct.KmsKeyId (Pattern F).
// Handles both full ARN format (arn:aws:kms:…/key-id) and bare key ID.
func checkEBSSnapKMS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	snap, ok := assertStruct[ec2types.Snapshot](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	if snap.KmsKeyId == nil || *snap.KmsKeyId == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	val := *snap.KmsKeyId
	keyID := val
	if idx := strings.LastIndex(val, "/"); idx >= 0 && idx < len(val)-1 {
		keyID = val[idx+1:]
	}
	if keyID == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	return relatedResult("kms", []string{keyID})
}


// checkEBSSnapBackup calls backup:ListRecoveryPointsByResource with the
// snapshot's ARN and returns the recovery-point ARNs. Pattern C.
// Snapshot ARN: arn:aws:ec2:REGION::snapshot/SNAP-ID (no account segment).
func checkEBSSnapBackup(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	snapID := res.ID
	if snapID == "" {
		return resource.RelatedCheckResult{TargetType: "backup", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Backup == nil {
		return resource.RelatedCheckResult{TargetType: "backup", Count: -1}
	}
	region := regionFromEnv()
	if region == "" {
		return resource.RelatedCheckResult{TargetType: "backup", Count: -1}
	}
	snapARN := "arn:aws:ec2:" + region + "::snapshot/" + snapID
	api, ok := c.Backup.(BackupListRecoveryPointsByResourceAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "backup", Count: -1}
	}
	out, err := api.ListRecoveryPointsByResource(ctx, &backup.ListRecoveryPointsByResourceInput{
		ResourceArn: aws.String(snapARN),
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "backup", Count: -1, Err: err}
	}
	var ids []string
	for _, rp := range out.RecoveryPoints {
		if rp.RecoveryPointArn == nil {
			continue
		}
		arn := *rp.RecoveryPointArn
		if idx := strings.Index(arn, ":recovery-point:"); idx >= 0 {
			ids = append(ids, arn[idx+len(":recovery-point:"):])
		}
	}
	return relatedResult("backup", ids)
}

// ebsSnapRelatedResources returns cached resources for the target type, or fetches the first page.
func ebsSnapRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
