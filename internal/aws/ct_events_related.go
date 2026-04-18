// ct_events_related.go contains CloudTrail related-resource checker functions.
package aws

import (
	"context"
	"strings"

	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkCtEventsUser matches the event username against the iam-user cache.
// Pattern C — cache lookup by name/ID.
func checkCtEventsUser(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	username := res.Fields["user"]
	if username == "" {
		return resource.RelatedCheckResult{TargetType: "iam-user", Count: 0}
	}

	userList, truncated, err := ctEventsRelatedResources(ctx, clients, cache, "iam-user")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "iam-user", Count: -1, Err: err}
	}
	if userList == nil {
		return resource.RelatedCheckResult{TargetType: "iam-user", Count: -1}
	}

	var ids []string
	for _, userRes := range userList {
		if userRes.Name == username || userRes.ID == username {
			ids = append(ids, userRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("iam-user")
	}
	return relatedResult("iam-user", ids)
}

// checkCtEventsRole extracts role information from the CloudTrail event's
// Resources slice (AWS::IAM::Role) and matches against the role cache.
// Pattern C — cache lookup by name extracted from ARN.
func checkCtEventsRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	roleName := ctEventsExtractRoleName(res)
	if roleName == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}

	roleList, truncated, err := ctEventsRelatedResources(ctx, clients, cache, "role")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1, Err: err}
	}
	if roleList == nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}

	var ids []string
	for _, roleRes := range roleList {
		if roleRes.Name == roleName || roleRes.ID == roleName {
			ids = append(ids, roleRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("role")
	}
	return relatedResult("role", ids)
}

// ctEventsExtractRoleName attempts to find a role name from the CloudTrail event.
// It first inspects the event's Resources slice for AWS::IAM::Role entries and
// extracts the name from the ResourceName ARN (last segment after "/"). If no
// role resource is found, it falls back to the Username field — some role-based
// events encode the role as "AWSServiceRole/RoleName".
func ctEventsExtractRoleName(res resource.Resource) string {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if ok {
		for _, r := range event.Resources {
			if r.ResourceType != nil && strings.Contains(*r.ResourceType, "Role") {
				if r.ResourceName != nil && *r.ResourceName != "" {
					name := *r.ResourceName
					if idx := strings.LastIndex(name, "/"); idx >= 0 && idx < len(name)-1 {
						return name[idx+1:]
					}
					return name
				}
			}
		}
	}

	// Fallback: check if Username encodes a service role path (e.g. "AWSServiceRole/RoleName").
	username := res.Fields["user"]
	if strings.Contains(username, "/") {
		return username[strings.LastIndex(username, "/")+1:]
	}

	// Third path: AssumedRole events store role info in the CloudTrailEvent JSON string.
	if ok {
		if name := extractRoleNameFromCTEventJSON(event.CloudTrailEvent); name != "" {
			return name
		}
	}

	return ""
}

// ctEventsRelatedResources returns the resource list for target from cache or
// fetches the first page via the registered paginated fetcher.
func ctEventsRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return []resource.Resource{}, false, nil
		}
	}
	return resources, isTruncated, err
}

// extractCTResourceIDs scans the event's Resources slice for entries matching
// awsResourceType (e.g. "AWS::EC2::Instance") and returns the bare identifiers
// (ResourceName with any "/" prefix trimmed to the last segment).
func extractCTResourceIDs(event cloudtrailtypes.Event, awsResourceType string) []string {
	var ids []string
	for _, r := range event.Resources {
		if r.ResourceType == nil || !strings.EqualFold(*r.ResourceType, awsResourceType) {
			continue
		}
		if r.ResourceName == nil || *r.ResourceName == "" {
			continue
		}
		name := *r.ResourceName
		if idx := strings.LastIndex(name, "/"); idx >= 0 && idx < len(name)-1 {
			name = name[idx+1:]
		}
		ids = append(ids, name)
	}
	return ids
}

// ctJSONString walks a parsed CT event JSON map along the given keys and
// returns the string value at the leaf, or "" if any step fails.
func ctJSONString(m map[string]any, keys ...string) string {
	var cur any = m
	for _, k := range keys {
		mm, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = mm[k]
	}
	s, _ := cur.(string)
	return s
}

// ctJSONStringSlice walks to keys[0..n-2] then collects string values from
// the []any slice at keys[n-1] by reading itemKey from each element.
func ctJSONStringSlice(m map[string]any, itemKey string, keys ...string) []string {
	var cur any = m
	for _, k := range keys {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = mm[k]
	}
	items, ok := cur.([]any)
	if !ok {
		return nil
	}
	var out []string
	for _, it := range items {
		mm, ok := it.(map[string]any)
		if !ok {
			continue
		}
		if v, ok := mm[itemKey].(string); ok && v != "" {
			out = append(out, v)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// §7b.10 typed related-resource checkers
// ---------------------------------------------------------------------------

// checkCtEventsEC2 extracts EC2 instance IDs from the CloudTrail event.
func checkCtEventsEC2(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: 0}
	}

	// Primary: Resources slice
	ids := extractCTResourceIDs(event, "AWS::EC2::Instance")

	// Fallback: requestParameters / responseElements instancesSet
	if len(ids) == 0 {
		parsed := parseCTEventJSON(event.CloudTrailEvent)
		if parsed != nil {
			req, _ := parsed["requestParameters"].(map[string]any)
			ids = append(ids, ctJSONStringSlice(req, "instanceId", "instancesSet", "items")...)
			resp, _ := parsed["responseElements"].(map[string]any)
			ids = append(ids, ctJSONStringSlice(resp, "instanceId", "instancesSet", "items")...)
		}
	}

	if len(ids) == 0 {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: 0}
	}

	resourceList, truncated, err := ctEventsRelatedResources(ctx, clients, cache, "ec2")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1, Err: err}
	}
	if resourceList == nil {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1}
	}

	wantSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wantSet[id] = struct{}{}
	}
	var matched []string
	for _, r := range resourceList {
		if _, ok := wantSet[r.ID]; ok {
			matched = append(matched, r.ID)
		} else if _, ok := wantSet[r.Name]; ok {
			matched = append(matched, r.ID)
		}
	}
	if len(matched) == 0 && truncated {
		return resource.ApproximateZero("ec2")
	}
	return relatedResult("ec2", matched)
}

// checkCtEventsS3 extracts S3 bucket names from the CloudTrail event.
func checkCtEventsS3(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "s3", Count: 0}
	}

	ids := extractCTResourceIDs(event, "AWS::S3::Bucket")

	if len(ids) == 0 {
		parsed := parseCTEventJSON(event.CloudTrailEvent)
		if parsed != nil {
			req, _ := parsed["requestParameters"].(map[string]any)
			if b := ctJSONString(req, "bucketName"); b != "" {
				ids = append(ids, b)
			}
		}
	}

	if len(ids) == 0 {
		return resource.RelatedCheckResult{TargetType: "s3", Count: 0}
	}

	resourceList, truncated, err := ctEventsRelatedResources(ctx, clients, cache, "s3")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "s3", Count: -1, Err: err}
	}
	if resourceList == nil {
		return resource.RelatedCheckResult{TargetType: "s3", Count: -1}
	}

	wantSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wantSet[id] = struct{}{}
	}
	var matched []string
	for _, r := range resourceList {
		if _, ok := wantSet[r.ID]; ok {
			matched = append(matched, r.ID)
		} else if _, ok := wantSet[r.Name]; ok {
			matched = append(matched, r.ID)
		}
	}
	if len(matched) == 0 && truncated {
		return resource.ApproximateZero("s3")
	}
	return relatedResult("s3", matched)
}



// checkCtEventsLambda extracts Lambda function names from the CloudTrail event.
func checkCtEventsLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
	}

	ids := extractCTResourceIDs(event, "AWS::Lambda::Function")

	if len(ids) == 0 {
		parsed := parseCTEventJSON(event.CloudTrailEvent)
		if parsed != nil {
			req, _ := parsed["requestParameters"].(map[string]any)
			if fn := ctJSONString(req, "functionName"); fn != "" {
				// Strip ARN if present — extract just the function name
				if idx := strings.LastIndex(fn, ":"); idx >= 0 && idx < len(fn)-1 {
					fn = fn[idx+1:]
				}
				ids = append(ids, fn)
			}
		}
	}

	if len(ids) == 0 {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
	}

	resourceList, truncated, err := ctEventsRelatedResources(ctx, clients, cache, "lambda")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1, Err: err}
	}
	if resourceList == nil {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1}
	}

	wantSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wantSet[id] = struct{}{}
	}
	var matched []string
	for _, r := range resourceList {
		if _, ok := wantSet[r.ID]; ok {
			matched = append(matched, r.ID)
		} else if _, ok := wantSet[r.Name]; ok {
			matched = append(matched, r.ID)
		}
	}
	if len(matched) == 0 && truncated {
		return resource.ApproximateZero("lambda")
	}
	return relatedResult("lambda", matched)
}

// checkCtEventsRDS extracts RDS instance/cluster identifiers from the CloudTrail event.
// TargetType is "rds" which is an alias for the "dbi" resource type.
func checkCtEventsRDS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "rds", Count: 0}
	}

	var ids []string
	// Resources slice: match any AWS::RDS::DB* type
	for _, r := range event.Resources {
		if r.ResourceType == nil || !strings.HasPrefix(*r.ResourceType, "AWS::RDS::DB") {
			continue
		}
		if r.ResourceName == nil || *r.ResourceName == "" {
			continue
		}
		name := *r.ResourceName
		if idx := strings.LastIndex(name, "/"); idx >= 0 && idx < len(name)-1 {
			name = name[idx+1:]
		}
		ids = append(ids, name)
	}

	if len(ids) == 0 {
		parsed := parseCTEventJSON(event.CloudTrailEvent)
		if parsed != nil {
			req, _ := parsed["requestParameters"].(map[string]any)
			if id := ctJSONString(req, "dBInstanceIdentifier"); id != "" {
				ids = append(ids, id)
			}
			if id := ctJSONString(req, "dBClusterIdentifier"); id != "" {
				ids = append(ids, id)
			}
		}
	}

	if len(ids) == 0 {
		return resource.RelatedCheckResult{TargetType: "rds", Count: 0}
	}

	resourceList, truncated, err := ctEventsRelatedResources(ctx, clients, cache, "rds")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "rds", Count: -1, Err: err}
	}
	if resourceList == nil {
		return resource.RelatedCheckResult{TargetType: "rds", Count: -1}
	}

	wantSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wantSet[id] = struct{}{}
	}
	var matched []string
	for _, r := range resourceList {
		if _, ok := wantSet[r.ID]; ok {
			matched = append(matched, r.ID)
		} else if _, ok := wantSet[r.Name]; ok {
			matched = append(matched, r.ID)
		}
	}
	if len(matched) == 0 && truncated {
		return resource.ApproximateZero("rds")
	}
	return relatedResult("rds", matched)
}

// checkCtEventsKMS extracts KMS key IDs from the CloudTrail event.
func checkCtEventsKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}

	ids := extractCTResourceIDs(event, "AWS::KMS::Key")

	if len(ids) == 0 {
		parsed := parseCTEventJSON(event.CloudTrailEvent)
		if parsed != nil {
			req, _ := parsed["requestParameters"].(map[string]any)
			if id := ctJSONString(req, "keyId"); id != "" {
				ids = append(ids, stripKMSKeyID(id))
			}
			svcDetails, _ := parsed["serviceEventDetails"].(map[string]any)
			if id := ctJSONString(svcDetails, "keyId"); id != "" {
				ids = append(ids, stripKMSKeyID(id))
			}
		}
	}

	if len(ids) == 0 {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}

	resourceList, truncated, err := ctEventsRelatedResources(ctx, clients, cache, "kms")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1, Err: err}
	}
	if resourceList == nil {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}

	wantSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wantSet[id] = struct{}{}
	}
	var matched []string
	for _, r := range resourceList {
		if _, ok := wantSet[r.ID]; ok {
			matched = append(matched, r.ID)
		} else if _, ok := wantSet[r.Name]; ok {
			matched = append(matched, r.ID)
		}
	}
	if len(matched) == 0 && truncated {
		return resource.ApproximateZero("kms")
	}
	return relatedResult("kms", matched)
}

// stripKMSKeyID strips a KMS key ID or ARN down to the bare UUID
// (the last path segment after "/").
func stripKMSKeyID(id string) string {
	if idx := strings.LastIndex(id, "/"); idx >= 0 && idx < len(id)-1 {
		return id[idx+1:]
	}
	return id
}

// checkCtEventsSecrets extracts Secrets Manager secret IDs from the CloudTrail event.
func checkCtEventsSecrets(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: 0}
	}

	ids := extractCTResourceIDs(event, "AWS::SecretsManager::Secret")

	if len(ids) == 0 {
		parsed := parseCTEventJSON(event.CloudTrailEvent)
		if parsed != nil {
			req, _ := parsed["requestParameters"].(map[string]any)
			if id := ctJSONString(req, "secretId"); id != "" {
				ids = append(ids, id)
			}
		}
	}

	if len(ids) == 0 {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: 0}
	}

	resourceList, truncated, err := ctEventsRelatedResources(ctx, clients, cache, "secrets")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: -1, Err: err}
	}
	if resourceList == nil {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: -1}
	}

	wantSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wantSet[id] = struct{}{}
	}
	var matched []string
	for _, r := range resourceList {
		if _, ok := wantSet[r.ID]; ok {
			matched = append(matched, r.ID)
		} else if _, ok := wantSet[r.Name]; ok {
			matched = append(matched, r.ID)
		}
	}
	if len(matched) == 0 && truncated {
		return resource.ApproximateZero("secrets")
	}
	return relatedResult("secrets", matched)
}

// checkCtEventsVPCE extracts VPC Endpoint IDs from the CloudTrail event.
func checkCtEventsVPCE(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "vpce", Count: 0}
	}

	var ids []string
	parsed := parseCTEventJSON(event.CloudTrailEvent)
	if parsed != nil {
		if id := ctJSONString(parsed, "vpcEndpointId"); id != "" {
			ids = append(ids, id)
		}
	}

	if len(ids) == 0 {
		return resource.RelatedCheckResult{TargetType: "vpce", Count: 0}
	}

	resourceList, truncated, err := ctEventsRelatedResources(ctx, clients, cache, "vpce")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "vpce", Count: -1, Err: err}
	}
	if resourceList == nil {
		return resource.RelatedCheckResult{TargetType: "vpce", Count: -1}
	}

	wantSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wantSet[id] = struct{}{}
	}
	var matched []string
	for _, r := range resourceList {
		if _, ok := wantSet[r.ID]; ok {
			matched = append(matched, r.ID)
		} else if _, ok := wantSet[r.Name]; ok {
			matched = append(matched, r.ID)
		}
	}
	if len(matched) == 0 && truncated {
		return resource.ApproximateZero("vpce")
	}
	return relatedResult("vpce", matched)
}

// checkCtEventsSG extracts Security Group IDs from the CloudTrail event.
func checkCtEventsSG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
	}

	ids := extractCTResourceIDs(event, "AWS::EC2::SecurityGroup")

	if len(ids) == 0 {
		parsed := parseCTEventJSON(event.CloudTrailEvent)
		if parsed != nil {
			req, _ := parsed["requestParameters"].(map[string]any)
			if id := ctJSONString(req, "groupId"); id != "" {
				ids = append(ids, id)
			}
		}
	}

	if len(ids) == 0 {
		return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
	}

	resourceList, truncated, err := ctEventsRelatedResources(ctx, clients, cache, "sg")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1, Err: err}
	}
	if resourceList == nil {
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1}
	}

	wantSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wantSet[id] = struct{}{}
	}
	var matched []string
	for _, r := range resourceList {
		if _, ok := wantSet[r.ID]; ok {
			matched = append(matched, r.ID)
		} else if _, ok := wantSet[r.Name]; ok {
			matched = append(matched, r.ID)
		}
	}
	if len(matched) == 0 && truncated {
		return resource.ApproximateZero("sg")
	}
	return relatedResult("sg", matched)
}

// checkCtEventsDDB extracts DynamoDB table names from the CloudTrail event.
func checkCtEventsDDB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ddb", Count: 0}
	}

	ids := extractCTResourceIDs(event, "AWS::DynamoDB::Table")

	if len(ids) == 0 {
		parsed := parseCTEventJSON(event.CloudTrailEvent)
		if parsed != nil {
			req, _ := parsed["requestParameters"].(map[string]any)
			if name := ctJSONString(req, "tableName"); name != "" {
				ids = append(ids, name)
			}
		}
	}

	if len(ids) == 0 {
		return resource.RelatedCheckResult{TargetType: "ddb", Count: 0}
	}

	resourceList, truncated, err := ctEventsRelatedResources(ctx, clients, cache, "ddb")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ddb", Count: -1, Err: err}
	}
	if resourceList == nil {
		return resource.RelatedCheckResult{TargetType: "ddb", Count: -1}
	}

	wantSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wantSet[id] = struct{}{}
	}
	var matched []string
	for _, r := range resourceList {
		if _, ok := wantSet[r.ID]; ok {
			matched = append(matched, r.ID)
		} else if _, ok := wantSet[r.Name]; ok {
			matched = append(matched, r.ID)
		}
	}
	if len(matched) == 0 && truncated {
		return resource.ApproximateZero("ddb")
	}
	return relatedResult("ddb", matched)
}

// ---------------------------------------------------------------------------
// §7b.10 self-pivot checkers (ct-events → ct-events)
// ---------------------------------------------------------------------------

// checkCtEventsPivotByAccessKeyId returns a self-pivot FetchFilter for the
// accessKeyId found in the event's userIdentity JSON blob. Returns Count=0 when
// the event has no accessKeyId or the caller is Root (Root has no access key).
func checkCtEventsPivotByAccessKeyId(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: 0}
	}
	parsed := parseCTEventJSON(event.CloudTrailEvent)
	ui, _ := parsed["userIdentity"].(map[string]any)
	uiType, _ := ui["type"].(string)
	if uiType == "Root" {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: 0}
	}
	accessKeyID, _ := ui["accessKeyId"].(string)
	if accessKeyID == "" {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: 0}
	}
	return resource.RelatedCheckResult{
		TargetType:  "ct-events",
		Count:       -1,
		FetchFilter: map[string]string{"AccessKeyId": accessKeyID},
	}
}

// checkCtEventsPivotByUsername returns a self-pivot FetchFilter for the Username
// derived from the event. The Username field is always derivable from any event
// that has a non-empty user.
func checkCtEventsPivotByUsername(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	username := res.Fields["user"]
	if username == "" {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: 0}
	}
	return resource.RelatedCheckResult{
		TargetType:  "ct-events",
		Count:       -1,
		FetchFilter: map[string]string{"Username": username},
	}
}

// checkCtEventsPivotByEventName returns a self-pivot FetchFilter for the EventName.
// Every CloudTrail event has an event name, so this pivot always applies.
func checkCtEventsPivotByEventName(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	eventName := res.Fields["event_name"]
	if eventName == "" {
		eventName = res.Name
	}
	if eventName == "" {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: 0}
	}
	return resource.RelatedCheckResult{
		TargetType:  "ct-events",
		Count:       -1,
		FetchFilter: map[string]string{"EventName": eventName},
	}
}

// checkCtEventsPivotBySharedEventId returns a self-pivot FetchFilter for the
// SharedEventId. This only applies to cross-account events where accountId differs
// from recipientAccountId. The SharedEventId links events across accounts for the
// same API call.
func checkCtEventsPivotBySharedEventId(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if res.Fields["_ct.cross_account"] != "true" {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: 0}
	}
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: 0}
	}
	parsed := parseCTEventJSON(event.CloudTrailEvent)
	sharedEventID, _ := parsed["sharedEventID"].(string)
	if sharedEventID == "" {
		// Cross-account event without a sharedEventID in the JSON — use the eventID as
		// a best-effort fallback so the pivot is still offered to the user.
		if event.EventId != nil && *event.EventId != "" {
			sharedEventID = *event.EventId
		}
	}
	if sharedEventID == "" {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: 0}
	}
	return resource.RelatedCheckResult{
		TargetType:  "ct-events",
		Count:       -1,
		FetchFilter: map[string]string{"SharedEventId": sharedEventID},
	}
}

// checkCtEventsTrail extracts CloudTrail trail identifiers from the CloudTrail
// event. Trail resources appear either in the event's Resources slice as
// AWS::CloudTrail::Trail entries or inline in the CloudTrailEvent JSON
// requestParameters as "name"/"trailName"/"trailARN" for API calls that act
// on a trail (e.g. CreateTrail, UpdateTrail, PutEventSelectors, StartLogging).
// The extracted names/ARNs are then matched against the trail cache.
func checkCtEventsTrail(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "trail", Count: 0}
	}

	ids := extractCTResourceIDs(event, "AWS::CloudTrail::Trail")

	if len(ids) == 0 {
		parsed := parseCTEventJSON(event.CloudTrailEvent)
		if parsed != nil {
			req, _ := parsed["requestParameters"].(map[string]any)
			for _, key := range []string{"name", "trailName", "trailARN", "trailArn"} {
				if v := ctJSONString(req, key); v != "" {
					// If this looks like a full ARN, extract the trail name suffix.
					name := v
					if idx := strings.LastIndex(v, "/"); idx >= 0 && idx < len(v)-1 {
						name = v[idx+1:]
					}
					ids = append(ids, name)
				}
			}
		}
	}

	if len(ids) == 0 {
		return resource.RelatedCheckResult{TargetType: "trail", Count: 0}
	}

	resourceList, truncated, err := ctEventsRelatedResources(ctx, clients, cache, "trail")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "trail", Count: -1, Err: err}
	}
	if resourceList == nil {
		return resource.RelatedCheckResult{TargetType: "trail", Count: -1}
	}

	wantSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wantSet[id] = struct{}{}
	}
	var matched []string
	for _, r := range resourceList {
		if _, ok := wantSet[r.ID]; ok {
			matched = append(matched, r.ID)
			continue
		}
		if _, ok := wantSet[r.Name]; ok {
			matched = append(matched, r.ID)
		}
	}
	if len(matched) == 0 && truncated {
		return resource.ApproximateZero("trail")
	}
	return relatedResult("trail", matched)
}

// checkCtEventsCFN extracts CloudFormation stack names from the CloudTrail event.
func checkCtEventsCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}

	ids := extractCTResourceIDs(event, "AWS::CloudFormation::Stack")

	if len(ids) == 0 {
		parsed := parseCTEventJSON(event.CloudTrailEvent)
		if parsed != nil {
			req, _ := parsed["requestParameters"].(map[string]any)
			if name := ctJSONString(req, "stackName"); name != "" {
				ids = append(ids, name)
			}
		}
	}

	if len(ids) == 0 {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}

	resourceList, truncated, err := ctEventsRelatedResources(ctx, clients, cache, "cfn")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1, Err: err}
	}
	if resourceList == nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}

	wantSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wantSet[id] = struct{}{}
	}
	var matched []string
	for _, r := range resourceList {
		if _, ok := wantSet[r.ID]; ok {
			matched = append(matched, r.ID)
		} else if _, ok := wantSet[r.Name]; ok {
			matched = append(matched, r.ID)
		}
	}
	if len(matched) == 0 && truncated {
		return resource.ApproximateZero("cfn")
	}
	return relatedResult("cfn", matched)
}

