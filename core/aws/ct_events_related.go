// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ct_events_related.go contains CloudTrail related-resource checker functions.
package aws

import (
	"context"
	"strings"

	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkCtEventsUser matches the event username against the iam-user cache.
// Pattern C — cache lookup by name/ID.
func checkCtEventsUser(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	username := res.Fields["user"]
	if username == "" {
		return resource.KnownRelated("iam-user", nil, false)
	}

	userList, truncated, err := ctEventsRelatedResources(ctx, clients, cache, "iam-user")
	if err != nil {
		return resource.ErrorRelated("iam-user", err)
	}
	if userList == nil {
		return resource.UnknownRelated("iam-user")
	}

	var ids []string
	for _, userRes := range userList {
		if userRes.Name == username || userRes.ID == username {
			ids = append(ids, userRes.ID)
		}
	}
	// The event names a user that was there when it was recorded; it does not
	// prove the user is there now. A first page that did not list them is no
	// answer either way.
	if len(ids) == 0 && truncated {
		return resource.UnknownRelated("iam-user")
	}
	return relatedResult("iam-user", ids)
}

// checkCtEventsRole extracts role information from the CloudTrail event's
// Resources slice (AWS::IAM::Role) and matches against the role cache.
// Pattern C — cache lookup by name extracted from ARN.
func checkCtEventsRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	roleName := ctEventsExtractRoleName(res)
	if roleName == "" {
		return resource.KnownRelated("role", nil, false)
	}

	roleList, truncated, err := ctEventsRelatedResources(ctx, clients, cache, "role")
	if err != nil {
		return resource.ErrorRelated("role", err)
	}

	var ids []string
	for _, roleRes := range roleList {
		if roleRes.Name == roleName || roleRes.ID == roleName {
			ids = append(ids, roleRes.ID)
		}
	}
	// The event names this exact role. When the cache can't disprove it — a
	// truncated page or a cold/nil cache — resolve by identity so the row is a
	// navigable (1), never a scoreless Unknown that renders actionable but
	// dead-ends on Enter. A complete (non-nil, non-truncated) cache with no
	// match stays (0): the role genuinely isn't one of ours.
	if len(ids) == 0 && (truncated || roleList == nil) {
		ids = []string{roleName}
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
	// Authoritative for AssumeRole* events: requestParameters.roleArn is the
	// TARGET role being assumed. Prefer it over Resources[]/sessionIssuer, which
	// carry the assumed-role session ARN (trailing session name) or the CALLER's
	// role — neither is the pivot target.
	if ok {
		if parsed := parseCTEventJSON(event.CloudTrailEvent); parsed != nil {
			if req, _ := parsed["requestParameters"].(map[string]any); req != nil {
				if arn, _ := req["roleArn"].(string); arn != "" {
					return roleNameFromARN(arn)
				}
			}
		}
	}
	if ok {
		for _, r := range event.Resources {
			if r.ResourceType != nil && strings.Contains(*r.ResourceType, "Role") {
				if r.ResourceName != nil && *r.ResourceName != "" {
					return roleNameFromARN(*r.ResourceName)
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

// ctEventsRelatedResources reads the target list from the session cache ONLY —
// it never triggers a fetch. A CloudTrail event names its related resources in
// the event body, so the checkers resolve from that (identity) and use the
// cache only to canonicalize/confirm an id when it happens to be warm already.
// Returning nil on a cache miss keeps the ct-event related panel zero-fetch: no
// ListRoles/DescribeInstances/… just to match ids the event already carries.
func ctEventsRelatedResources(_ context.Context, _ any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	if entry, ok := cache[target]; ok {
		// A cache hit is authoritative — even a proven-EMPTY one. Normalize a nil
		// Resources slice to non-nil so the checkers' `resourceList == nil`
		// (cold-miss) branch doesn't misread a proven-zero target as absent and
		// synthesize a fake event-derived row.
		resources := entry.Resources
		if resources == nil {
			resources = []resource.Resource{}
		}
		return resources, entry.IsTruncated, nil
	}
	return nil, false, nil
}

// ctEventsMatchTarget resolves an event-derived id list against target's
// related-resource cache. An id in an event body is a claim about the past —
// the resource existed when the call was recorded, which is not evidence it
// exists now — so only the list can turn it into a count:
//
//   - nil list (nothing cached, nothing to call): Unknown. Trusting the ids
//     here offered a row that navigates to a resource that may be long gone.
//   - proven list: the ids the list confirms, by ID or Name.
//   - truncated list: still only the confirmed ids, because an unread page
//     cannot confirm anything; Unknown when it confirmed none, since a zero
//     off a partial list is a guess either way.
func ctEventsMatchTarget(ctx context.Context, clients any, cache resource.ResourceCache, target string, ids []string) resource.RelatedCheckResult {
	resourceList, truncated, err := ctEventsRelatedResources(ctx, clients, cache, target)
	if err != nil {
		return resource.ErrorRelated(target, err)
	}
	if resourceList == nil {
		return resource.UnknownRelated(target)
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
	if truncated && len(matched) == 0 {
		return resource.UnknownRelated(target)
	}
	return relatedResultTrunc(target, matched, truncated)
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
		// Emit BOTH the name as written and its last slash segment, and let
		// the target list decide. The trim is right for an ARN, where the id
		// is the last segment ("...:instance/i-abc"), and wrong for a plain
		// name that legitimately contains slashes — a Secrets Manager secret
		// is named "prod/database/primary", and trimming it to "primary"
		// matched nothing, so that pivot read Unknown forever. Which form is
		// the id varies per target type, so neither is preferred here.
		// Offering both costs nothing: ctEventsMatchTarget resolves only the
		// ids the list confirms, so a candidate that names no resource is
		// dropped rather than counted.
		name := *r.ResourceName
		ids = append(ids, name)
		if idx := strings.LastIndex(name, "/"); idx >= 0 && idx < len(name)-1 {
			ids = append(ids, name[idx+1:])
		}
	}
	return ids
}

// cfnStackNameFromResourceName extracts the stack NAME from a CloudTrail
// CloudFormation resource name. Stack ARNs are ".../stack/<name>/<uuid>"; the
// generic last-segment trim (extractCTResourceIDs) would keep the uuid, but cfn
// resources are keyed by stack name. A bare name (no "stack/" segment) passes
// through unchanged.
func cfnStackNameFromResourceName(s string) string {
	if _, rest, ok := strings.Cut(s, ":stack/"); ok {
		if name, _, ok := strings.Cut(rest, "/"); ok {
			return name
		}
		return rest
	}
	return s
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
		return resource.KnownRelated("ec2", nil, false)
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
		return resource.KnownRelated("ec2", nil, false)
	}

	return ctEventsMatchTarget(ctx, clients, cache, "ec2", ids)
}

// checkCtEventsS3 extracts S3 bucket names from the CloudTrail event.
func checkCtEventsS3(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return resource.KnownRelated("s3", nil, false)
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
		return resource.KnownRelated("s3", nil, false)
	}

	return ctEventsMatchTarget(ctx, clients, cache, "s3", ids)
}

// checkCtEventsLambda extracts Lambda function names from the CloudTrail event.
func checkCtEventsLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return resource.KnownRelated("lambda", nil, false)
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
		return resource.KnownRelated("lambda", nil, false)
	}

	return ctEventsMatchTarget(ctx, clients, cache, "lambda", ids)
}

// checkCtEventsRDS extracts RDS instance/cluster identifiers from the CloudTrail event.
// TargetType is "dbi" — the canonical short name for RDS DB instances. FetchRelatedTarget
// and the related-resource cache are keyed by canonical short names, so this checker
// MUST use "dbi" for every cache lookup and result construction (not "rds", which is an
// alias without a registered paginated fetcher).
func checkCtEventsRDS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return resource.KnownRelated("dbi", nil, false)
	}

	var ids []string
	// Resources slice: DB INSTANCES only. AWS::RDS::DBCluster / DBSnapshot /
	// DBClusterSnapshot identifiers are NOT dbi ids — emitting them here would
	// fake an RDS Instance relation that can never resolve (clusters are a
	// separate pivot).
	for _, r := range event.Resources {
		if r.ResourceType == nil || *r.ResourceType != "AWS::RDS::DBInstance" {
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
		}
	}

	if len(ids) == 0 {
		return resource.KnownRelated("dbi", nil, false)
	}

	return ctEventsMatchTarget(ctx, clients, cache, "dbi", ids)
}

// checkCtEventsKMS extracts KMS key IDs from the CloudTrail event.
func checkCtEventsKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return resource.KnownRelated("kms", nil, false)
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
		return resource.KnownRelated("kms", nil, false)
	}

	return ctEventsMatchTarget(ctx, clients, cache, "kms", ids)
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
		return resource.KnownRelated("secrets", nil, false)
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
		return resource.KnownRelated("secrets", nil, false)
	}

	return ctEventsMatchTarget(ctx, clients, cache, "secrets", ids)
}

// checkCtEventsVPCE extracts VPC Endpoint IDs from the CloudTrail event.
func checkCtEventsVPCE(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return resource.KnownRelated("vpce", nil, false)
	}

	var ids []string
	parsed := parseCTEventJSON(event.CloudTrailEvent)
	if parsed != nil {
		if id := ctJSONString(parsed, "vpcEndpointId"); id != "" {
			ids = append(ids, id)
		}
	}

	if len(ids) == 0 {
		return resource.KnownRelated("vpce", nil, false)
	}

	return ctEventsMatchTarget(ctx, clients, cache, "vpce", ids)
}

// checkCtEventsSG extracts Security Group IDs from the CloudTrail event.
func checkCtEventsSG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return resource.KnownRelated("sg", nil, false)
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
		return resource.KnownRelated("sg", nil, false)
	}

	return ctEventsMatchTarget(ctx, clients, cache, "sg", ids)
}

// checkCtEventsDDB extracts DynamoDB table names from the CloudTrail event.
func checkCtEventsDDB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return resource.KnownRelated("ddb", nil, false)
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
		return resource.KnownRelated("ddb", nil, false)
	}

	return ctEventsMatchTarget(ctx, clients, cache, "ddb", ids)
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
		return resource.KnownRelated("ct-events", nil, false)
	}
	parsed := parseCTEventJSON(event.CloudTrailEvent)
	ui, _ := parsed["userIdentity"].(map[string]any)
	uiType, _ := ui["type"].(string)
	if uiType == "Root" {
		return resource.KnownRelated("ct-events", nil, false)
	}
	accessKeyID, _ := ui["accessKeyId"].(string)
	if accessKeyID == "" {
		return resource.KnownRelated("ct-events", nil, false)
	}
	return resource.DeferredRelated("ct-events", map[string]string{"AccessKeyId": accessKeyID})
}

// checkCtEventsPivotByUsername returns a self-pivot FetchFilter for the Username
// derived from the event. The Username field is always derivable from any event
// that has a non-empty user.
func checkCtEventsPivotByUsername(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	username := res.Fields["user"]
	if username == "" {
		return resource.KnownRelated("ct-events", nil, false)
	}
	return resource.DeferredRelated("ct-events", map[string]string{"Username": username})
}

// checkCtEventsPivotByEventName returns a self-pivot FetchFilter for the EventName.
// Every CloudTrail event has an event name, so this pivot always applies.
func checkCtEventsPivotByEventName(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	eventName := res.Fields["event_name"]
	if eventName == "" {
		eventName = res.Name
	}
	if eventName == "" {
		return resource.KnownRelated("ct-events", nil, false)
	}
	return resource.DeferredRelated("ct-events", map[string]string{"EventName": eventName})
}

// checkCtEventsPivotBySharedEventId returns a self-pivot FetchFilter for the
// SharedEventId. This only applies to cross-account events where accountId differs
// from recipientAccountId. The SharedEventId links events across accounts for the
// same API call.
func checkCtEventsPivotBySharedEventId(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if res.Fields["_ct.cross_account"] != "true" {
		return resource.KnownRelated("ct-events", nil, false)
	}
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return resource.KnownRelated("ct-events", nil, false)
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
		return resource.KnownRelated("ct-events", nil, false)
	}
	return resource.DeferredRelated("ct-events", map[string]string{"SharedEventId": sharedEventID})
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
		return resource.KnownRelated("trail", nil, false)
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
		return resource.KnownRelated("trail", nil, false)
	}

	return ctEventsMatchTarget(ctx, clients, cache, "trail", ids)
}

// checkCtEventsCFN extracts CloudFormation stack names from the CloudTrail event.
func checkCtEventsCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return resource.KnownRelated("cfn", nil, false)
	}

	var ids []string
	for _, r := range event.Resources {
		if r.ResourceType == nil || !strings.EqualFold(*r.ResourceType, "AWS::CloudFormation::Stack") {
			continue
		}
		if r.ResourceName == nil || *r.ResourceName == "" {
			continue
		}
		ids = append(ids, cfnStackNameFromResourceName(*r.ResourceName))
	}

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
		return resource.KnownRelated("cfn", nil, false)
	}

	return ctEventsMatchTarget(ctx, clients, cache, "cfn", ids)
}
