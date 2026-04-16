package aws

import (
	"context"
	"strings"

	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkDocdbSnapDBC reads DBClusterIdentifier from the DBClusterSnapshot RawStruct.
// Pattern F — no cache needed.
func checkDocdbSnapDBC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	snap, ok := assertStruct[docdbtypes.DBClusterSnapshot](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "dbc", Count: -1}
	}
	if snap.DBClusterIdentifier == nil || *snap.DBClusterIdentifier == "" {
		return resource.RelatedCheckResult{TargetType: "dbc", Count: 0}
	}
	return relatedResult("dbc", []string{*snap.DBClusterIdentifier})
}

// checkDocdbSnapKMS reads KmsKeyId from the DBClusterSnapshot RawStruct.
// Extracts UUID after last '/' from the ARN.
// Pattern F — no cache needed.
func checkDocdbSnapKMS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	snap, ok := assertStruct[docdbtypes.DBClusterSnapshot](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	if snap.KmsKeyId == nil || *snap.KmsKeyId == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	keyID := *snap.KmsKeyId
	if idx := strings.LastIndex(keyID, "/"); idx >= 0 {
		keyID = keyID[idx+1:]
	}
	if keyID == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	return relatedResult("kms", []string{keyID})
}

func checkDocdbSnapVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	snap, ok := assertStruct[docdbtypes.DBClusterSnapshot](res.RawStruct)
	if !ok || snap.VpcId == nil || *snap.VpcId == "" {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	return relatedResult("vpc", []string{*snap.VpcId})
}

