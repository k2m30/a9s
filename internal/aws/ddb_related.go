package aws

import (
	"context"
	"strings"

	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkDdbKMS reads SSEDescription.KMSMasterKeyArn from the TableDescription RawStruct.
// Pattern F — no cache needed.
func checkDdbKMS(_ context.Context, _ interface{}, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	table, ok := assertStruct[ddbtypes.TableDescription](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	if table.SSEDescription == nil || table.SSEDescription.KMSMasterKeyArn == nil {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	arn := *table.SSEDescription.KMSMasterKeyArn
	idx := strings.LastIndex(arn, "/")
	if idx < 0 || idx == len(arn)-1 {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	keyID := arn[idx+1:]
	return relatedResult("kms", []string{keyID})
}
