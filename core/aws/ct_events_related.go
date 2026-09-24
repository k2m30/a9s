// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ct_events_related.go contains CloudTrail related-resource checker functions.
package aws

import (
	"cmp"
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkCtEventsUser matches the event username against the iam-user cache.
func checkCtEventsUser(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	username := res.Fields["user"]
	if username == "" {
		return foundNone("iam-user", "username")
	}

	userList, truncated, err := ctEventsRelatedResources(ctx, clients, cache, "iam-user")
	if err != nil {
		return ReadFailed("iam-user", err)
	}
	if userList == nil {
		return NotRead("iam-user")
	}

	var ids []string
	for _, userRes := range userList {
		if userRes.Name == username || userRes.ID == username {
			ids = append(ids, userRes.ID)
		}
	}
	// The event names a user that was there when it was recorded; it does not
	// prove the user is there now, so only the list answers. A truncated list
	// that confirmed none gives a lower bound, not a question mark.
	return relatedResultTrunc("iam-user", ids, truncated)
}

// checkCtEventsRole is the role the event was performed under: the
// sessionIssuer of an AssumedRole or Role identity, "the source (account, IAM
// user, or role) that was used to get temporary security credentials"
// (https://docs.aws.amazon.com/awscloudtrail/latest/userguide/cloudtrail-event-reference-user-identity.html).
// A role an event acts on (an AssumeRole's requestParameters.roleArn, a
// resource of the event) is not the caller.
func checkCtEventsRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	ref := res.Fields["role_name"]
	if event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct); ok {
		name, roleARN := extractRoleNameFromCTEventJSON(event.CloudTrailEvent)
		ref = cmp.Or(roleARN, name)
	}
	if ref == "" {
		return unreadZero(res, foundNone("role", "sessionIssuer"))
	}
	return unreadZero(res, ctEventsMatchTarget(ctx, clients, cache, "role", res, []string{ref}))
}

// ctEventsRelatedResources reads the target list from the session cache ONLY —
// it never triggers a fetch. The event body names the ids; this list is the only
// thing that can confirm any of them still exists, and returning nil on a cache
// miss is what makes an unconfirmable id read as Unknown rather than a count.
// Zero-fetch is the point: no ListRoles/DescribeInstances/… from opening a row.
func ctEventsRelatedResources(_ context.Context, _ any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	if resources, truncated, ok := cachedRelatedList(cache, target); ok {
		// A cache hit is authoritative — even a proven-EMPTY one. Normalize a nil
		// Resources slice to non-nil so the checkers' `resourceList == nil`
		// (cold-miss) branch doesn't misread a proven-zero target as absent and
		// synthesize a fake event-derived row.
		if resources == nil {
			resources = []resource.Resource{}
		}
		return resources, truncated, nil
	}
	return nil, false, nil
}

// ctEventsMatchTarget reads the references an event names through target's
// resolver, in the session's account (the one the event was recorded in
// while the session's identity is unread) and Region, and keeps the ids target's cached list holds. A reference is a
// claim about the past — the resource existed when the call was recorded,
// which is not evidence it exists now — so only the list can turn it into a
// count:
//
//   - nil list (nothing cached, nothing to call): Unknown. Trusting the ids
//     here would offer a row that navigates to a resource that may be long gone.
//   - proven list: the ids the list holds.
//   - truncated list: still only the ids the list holds, because an unread
//     page cannot confirm anything, and the truncation flag carries the rest —
//     none confirmed among the pages read is a lower bound, rendered "(0+)".
func ctEventsMatchTarget(ctx context.Context, clients any, cache resource.ResourceCache, target string, res resource.Resource, refs []string) resource.RelatedCheckResult {
	list, truncated, err := ctEventsRelatedResources(ctx, clients, cache, target)
	if err != nil {
		return ReadFailed(target, err)
	}
	if list == nil {
		return NotRead(target)
	}
	rc := refContext(clients, cache, target)
	rc.AccountID = cmp.Or(rc.AccountID, res.Fields["_ct.recipient_account"])
	ids, _ := listedRefs(target, refs, rc, list)
	return relatedResultTrunc(target, ids, truncated)
}

// extractCTResourceIDs returns the name of every entry of the event's
// Resources slice whose type is awsResourceType (e.g. "AWS::EC2::Instance"),
// as the event wrote it: an id, a name or an ARN, for the target's resolver
// to read.
func extractCTResourceIDs(event cloudtrailtypes.Event, awsResourceType string) []string {
	var refs []string
	for _, r := range event.Resources {
		if r.ResourceType != nil && strings.EqualFold(*r.ResourceType, awsResourceType) && aws.ToString(r.ResourceName) != "" {
			refs = append(refs, *r.ResourceName)
		}
	}
	return refs
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
// Typed related-resource checkers
// ---------------------------------------------------------------------------

// checkCtEventsEC2 extracts EC2 instance IDs from the CloudTrail event.
func checkCtEventsEC2(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return NotRead("ec2")
	}

	// Primary: Resources slice
	ids := extractCTResourceIDs(event, "AWS::EC2::Instance")

	// Fallback: requestParameters / responseElements instancesSet
	if len(ids) == 0 {
		parsed := parseCTEventJSON(event.CloudTrailEvent)
		if parsed != nil {
			req, _ := parsed["requestParameters"].(map[string]any)
			resp, _ := parsed["responseElements"].(map[string]any)
			fromBody := append(ctJSONStringSlice(req, "instanceId", "instancesSet", "items"),
				ctJSONStringSlice(resp, "instanceId", "instancesSet", "items")...)
			ids = append(ids, fromBody...)
		}
	}

	if len(ids) == 0 {
		return foundNone("ec2", "ids")
	}

	return ctEventsMatchTarget(ctx, clients, cache, "ec2", res, ids)
}

// checkCtEventsS3 extracts S3 bucket names from the CloudTrail event.
func checkCtEventsS3(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return NotRead("s3")
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
		return foundNone("s3", "ids")
	}

	return ctEventsMatchTarget(ctx, clients, cache, "s3", res, ids)
}

// checkCtEventsLambda extracts Lambda function names from the CloudTrail event.
func checkCtEventsLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return NotRead("lambda")
	}

	ids := extractCTResourceIDs(event, "AWS::Lambda::Function")

	if len(ids) == 0 {
		parsed := parseCTEventJSON(event.CloudTrailEvent)
		if parsed != nil {
			req, _ := parsed["requestParameters"].(map[string]any)
			if fn := ctJSONString(req, "functionName"); fn != "" {
				ids = append(ids, fn)
			}
		}
	}

	if len(ids) == 0 {
		return foundNone("lambda", "ids")
	}

	return ctEventsMatchTarget(ctx, clients, cache, "lambda", res, ids)
}

// checkCtEventsRDS extracts RDS instance/cluster identifiers from the CloudTrail event.
// TargetType is "dbi" — the canonical short name for RDS DB instances. FetchRelatedTarget
// and the related-resource cache are keyed by canonical short names, so this checker
// MUST use "dbi" for every cache lookup and result construction (not "rds", which is an
// alias without a registered paginated fetcher).
func checkCtEventsRDS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return NotRead("dbi")
	}

	// DB INSTANCES only. AWS::RDS::DBCluster / DBSnapshot / DBClusterSnapshot
	// identifiers are NOT dbi ids — emitting them here would fake an RDS
	// Instance relation that can never resolve (clusters are a separate pivot).
	ids := extractCTResourceIDs(event, "AWS::RDS::DBInstance")

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
		return foundNone("dbi", "ids")
	}

	return ctEventsMatchTarget(ctx, clients, cache, "dbi", res, ids)
}

// checkCtEventsKMS extracts KMS key IDs from the CloudTrail event.
func checkCtEventsKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return NotRead("kms")
	}

	ids := extractCTResourceIDs(event, "AWS::KMS::Key")

	if len(ids) == 0 {
		parsed := parseCTEventJSON(event.CloudTrailEvent)
		if parsed != nil {
			req, _ := parsed["requestParameters"].(map[string]any)
			if id := ctJSONString(req, "keyId"); id != "" {
				ids = append(ids, id)
			}
			svcDetails, _ := parsed["serviceEventDetails"].(map[string]any)
			if id := ctJSONString(svcDetails, "keyId"); id != "" {
				ids = append(ids, id)
			}
		}
	}

	if len(ids) == 0 {
		return foundNone("kms", "ids")
	}

	return ctEventsMatchTarget(ctx, clients, cache, "kms", res, ids)
}

// checkCtEventsSecrets extracts Secrets Manager secret IDs from the CloudTrail event.
func checkCtEventsSecrets(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return NotRead("secrets")
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
		return foundNone("secrets", "ids")
	}

	return ctEventsMatchTarget(ctx, clients, cache, "secrets", res, ids)
}

// checkCtEventsVPCE extracts VPC Endpoint IDs from the CloudTrail event.
func checkCtEventsVPCE(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return NotRead("vpce")
	}

	var ids []string
	parsed := parseCTEventJSON(event.CloudTrailEvent)
	if parsed != nil {
		if id := ctJSONString(parsed, "vpcEndpointId"); id != "" {
			ids = append(ids, id)
		}
	}

	if len(ids) == 0 {
		return foundNone("vpce", "ids")
	}

	return ctEventsMatchTarget(ctx, clients, cache, "vpce", res, ids)
}

// checkCtEventsSG extracts Security Group IDs from the CloudTrail event.
func checkCtEventsSG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return NotRead("sg")
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
		return foundNone("sg", "ids")
	}

	return ctEventsMatchTarget(ctx, clients, cache, "sg", res, ids)
}

// checkCtEventsDDB extracts DynamoDB table names from the CloudTrail event.
func checkCtEventsDDB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return NotRead("ddb")
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
		return foundNone("ddb", "ids")
	}

	return ctEventsMatchTarget(ctx, clients, cache, "ddb", res, ids)
}

// checkCtEventsECR extracts the repository an ECR record names: its
// AWS::ECR::Repository resource, by ARN or by name, else the request's
// repositoryName
// (https://docs.aws.amazon.com/AmazonECR/latest/userguide/logging-using-cloudtrail.html).
func checkCtEventsECR(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return NotRead("ecr")
	}
	ids := extractCTResourceIDs(event, "AWS::ECR::Repository")
	if len(ids) == 0 {
		if parsed := parseCTEventJSON(event.CloudTrailEvent); parsed != nil {
			req, _ := parsed["requestParameters"].(map[string]any)
			if name := ctJSONString(req, "repositoryName"); name != "" {
				ids = append(ids, name)
			}
		}
	}
	if len(ids) == 0 {
		return foundNone("ecr", "ids")
	}
	return ctEventsMatchTarget(ctx, clients, cache, "ecr", res, ids)
}

// ---------------------------------------------------------------------------
// Self-pivot checkers (ct-events → ct-events)
// ---------------------------------------------------------------------------

// ctSelfPivot completes a ct-events → ct-events filter with the Region the
// event was recorded in: a drill off an event is a lookup for more of that
// event's own history, which lives in the Region the event names, not in the
// one the session is browsing. An event that names no Region cannot say where
// to look, so the pivot answers unknown rather than searching the wrong one.
func ctSelfPivot(res resource.Resource, filter map[string]string) resource.RelatedCheckResult {
	region := res.Fields["_ct.region"]
	if region == "" {
		return NotRead("ct-events")
	}
	filter[resource.CTRegionFilterKey] = region
	return resource.DeferredRelated("ct-events", filter)
}

// checkCtEventsPivotByAccessKeyId returns a self-pivot FetchFilter for the
// accessKeyId found in the event's userIdentity JSON blob. Returns Count=0 when
// the event carries no accessKeyId. The root user can hold access keys, and a
// root key in use is the case this pivot exists to trace.
func checkCtEventsPivotByAccessKeyId(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return NotRead("ct-events")
	}
	parsed := parseCTEventJSON(event.CloudTrailEvent)
	ui, _ := parsed["userIdentity"].(map[string]any)
	accessKeyID, _ := ui["accessKeyId"].(string)
	if accessKeyID == "" {
		return foundNone("ct-events", "accessKeyID")
	}
	return ctSelfPivot(res, map[string]string{"AccessKeyId": accessKeyID})
}

// checkCtEventsPivotByUsername returns a self-pivot FetchFilter for the
// caller CloudTrail recorded, which is the value its Username attribute
// matches for an assumed role, a service and a federated user alike.
func checkCtEventsPivotByUsername(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	username := res.Fields["_ct.username"]
	if username == "" {
		return foundNone("ct-events", "username")
	}
	return ctSelfPivot(res, map[string]string{"Username": username})
}

// checkCtEventsPivotByEventName returns a self-pivot FetchFilter for the EventName.
// Every CloudTrail event has an event name, so this pivot always applies.
func checkCtEventsPivotByEventName(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	eventName := res.Fields["event_name"]
	if eventName == "" {
		eventName = res.Name
	}
	if eventName == "" {
		return keyMissing("ct-events", "eventName")
	}
	return ctSelfPivot(res, map[string]string{"EventName": eventName})
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
		return NotRead("trail")
	}

	ids := extractCTResourceIDs(event, "AWS::CloudTrail::Trail")

	if len(ids) == 0 {
		parsed := parseCTEventJSON(event.CloudTrailEvent)
		if parsed != nil {
			req, _ := parsed["requestParameters"].(map[string]any)
			for _, key := range []string{"name", "trailName", "trailARN", "trailArn"} {
				if v := ctJSONString(req, key); v != "" {
					ids = append(ids, v)
				}
			}
		}
	}

	if len(ids) == 0 {
		return foundNone("trail", "ids")
	}

	return ctEventsMatchTarget(ctx, clients, cache, "trail", res, ids)
}

// checkCtEventsCFN extracts CloudFormation stack names from the CloudTrail event.
func checkCtEventsCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if !ok {
		return NotRead("cfn")
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
		return foundNone("cfn", "ids")
	}

	return ctEventsMatchTarget(ctx, clients, cache, "cfn", res, ids)
}
