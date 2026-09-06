// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ct_events_related.go contains CloudTrail related-resource checker functions.
package aws

import (
	"context"
	"slices"
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
	// prove the user is there now, so only the list answers. A truncated list
	// that confirmed none gives a lower bound, not a question mark.
	return relatedResultTrunc("iam-user", ids, truncated)
}

// checkCtEventsRole extracts role information from the CloudTrail event's
// Resources slice (AWS::IAM::Role) and matches against the role cache.
// Pattern C — cache lookup by name extracted from ARN.
func checkCtEventsRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	candidates := ctEventsRoleCandidates(res)
	if len(candidates) == 0 {
		return resource.KnownRelated("role", nil, false)
	}
	return ctEventsMatchTarget(ctx, clients, cache, "role", [][]string{candidates})
}

// ctEventsRoleCandidates returns the candidate ids for the role a CloudTrail
// event names, in the order the four sources are trusted: the target roleArn in
// requestParameters, an AWS::IAM::Role entry in Resources, a Username carrying a
// service-role path, then the event JSON. Each source's raw value goes through
// ctRoleAlternatives, so the candidates follow row 11's id rule rather than any
// role-specific trim.
func ctEventsRoleCandidates(res resource.Resource) []string {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	// Authoritative for AssumeRole* events: requestParameters.roleArn is the
	// TARGET role being assumed. Prefer it over Resources[]/sessionIssuer, which
	// carry the assumed-role session ARN (trailing session name) or the CALLER's
	// role — neither is the pivot target.
	if ok {
		if parsed := parseCTEventJSON(event.CloudTrailEvent); parsed != nil {
			if req, _ := parsed["requestParameters"].(map[string]any); req != nil {
				if arn, _ := req["roleArn"].(string); arn != "" {
					return ctRoleAlternatives(arn)
				}
			}
		}
	}
	if ok {
		for _, r := range event.Resources {
			if r.ResourceType != nil && strings.Contains(*r.ResourceType, "Role") {
				if r.ResourceName != nil && *r.ResourceName != "" {
					return ctRoleAlternatives(*r.ResourceName)
				}
			}
		}
	}

	// Fallback: Username may encode a service role path ("AWSServiceRole/RoleName").
	if username := res.Fields["user"]; strings.Contains(username, "/") {
		return ctRoleAlternatives(username)
	}

	// Third path: AssumedRole events store role info in the CloudTrailEvent JSON string.
	if ok {
		if name := extractRoleNameFromCTEventJSON(event.CloudTrailEvent); name != "" {
			return ctRoleAlternatives(name)
		}
	}

	return nil
}

// ctRoleAlternatives is ctIDAlternatives plus the STS form: an assumed-role
// ARN ends "assumed-role/<role>/<session>", so the role is the segment before
// the session and no general rule can see that.
func ctRoleAlternatives(v string) []string {
	out := ctIDAlternatives(v)
	if name := roleNameFromARN(v); name != "" && !slices.Contains(out, name) {
		out = append(out, name)
	}
	return out
}

// ctEventsRelatedResources reads the target list from the session cache ONLY —
// it never triggers a fetch. The event body names the ids; this list is the only
// thing that can confirm any of them still exists, and returning nil on a cache
// miss is what makes an unconfirmable id read as Unknown rather than a count.
// Zero-fetch is the point: no ListRoles/DescribeInstances/… from opening a row.
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
//     cannot confirm anything, and the truncation flag carries the rest — none
//     confirmed among the pages read is a lower bound, rendered "(0+)".
func ctEventsMatchTarget(ctx context.Context, clients any, cache resource.ResourceCache, target string, groups [][]string) resource.RelatedCheckResult {
	resourceList, truncated, err := ctEventsRelatedResources(ctx, clients, cache, target)
	if err != nil {
		return resource.ErrorRelated(target, err)
	}
	if resourceList == nil {
		return resource.UnknownRelated(target)
	}

	byID := make(map[string]string, len(resourceList)*3)
	for _, r := range resourceList {
		byID[r.ID] = r.ID
		if r.Name != "" {
			if _, taken := byID[r.Name]; !taken {
				byID[r.Name] = r.ID
			}
		}
		// A secret's id is its name while its ARN carries the six random
		// characters AWS appends, so an event that named the ARN can only be
		// confirmed here. Exact equality against the row's own ARN, so no id
		// is guessed and the suffix is never stripped.
		if arn := r.Fields["arn"]; arn != "" {
			if _, taken := byID[arn]; !taken {
				byID[arn] = r.ID
			}
		}
	}
	var matched []string
	seen := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		for _, candidate := range group {
			id, ok := byID[candidate]
			if !ok {
				continue
			}
			if _, dup := seen[id]; !dup {
				seen[id] = struct{}{}
				matched = append(matched, id)
			}
			break
		}
	}
	return relatedResultTrunc(target, matched, truncated)
}

// ctIDAlternatives returns the candidate ids for one value an event named,
// derived from what the event wrote and never from the ARN's fixed grammar.
// An ARN contributes the id inside its resource part — everything after the
// type word, as one string — and then itself, so no segment of the grammar and
// no type word can answer for a resource. Anything else contributes itself. A
// fragment of a name is not a form of it.
func ctIDAlternatives(v string) []string {
	if parts := strings.SplitN(v, ":", 6); strings.HasPrefix(v, "arn:") && len(parts) == 6 {
		// The whole ARN is offered last, behind the resource-part forms: a
		// list that carries its rows' own ARNs can answer it exactly, which
		// is the only way to confirm an id whose ARN carries a suffix the
		// name does not. Last because the callers that read a group
		// positionally (cfnStackNameFromResourceName, ctLambdaAlternatives)
		// read the resource part at [0] and its stripped form at [1].
		return append(ctStripTypeWord(parts[5], "/:"), v)
	}
	// Not an ARN: the value IS the id. An event may still write the resource
	// part alone ("instance/i-abc"), so the type-word strip is offered behind
	// it — on "/" only, because a trailing ":qualifier" on a bare name is
	// Lambda's business and its head is the function.
	return ctStripTypeWord(v, "/")
}

// ctStripTypeWord returns res and the form inside it, most specific first:
// res as written, then without its leading "<type><sep>"
// ("instance/i-abc" → "i-abc", "secret:prod/api/key" → "prod/api/key").
// Nothing shorter: a fragment of a name belongs to a different resource.
func ctStripTypeWord(res, seps string) []string {
	out := []string{res}
	i := strings.IndexAny(res, seps)
	if i < 0 || i >= len(res)-1 {
		return out
	}
	return append(out, res[i+1:])
}

// ctLambdaAlternatives is ctIDAlternatives plus the Lambda-only rule: a
// function may be named with a trailing ":<alias>" qualifier, so the head
// before it is a candidate after the as-written form.
func ctLambdaAlternatives(group []string) []string {
	// The type word is what precedes the first separator of the widest
	// candidate; a head that equals it is grammar, not a function name.
	typeWord := ""
	if len(group) > 1 {
		if i := strings.IndexAny(group[0], "/:"); i > 0 && group[0][i+1:] == group[1] {
			typeWord = group[0][:i]
		}
	}
	out := group
	for _, c := range group {
		if i := strings.LastIndex(c, ":"); i > 0 && c[:i] != typeWord {
			out = append(out, c[:i])
		}
	}
	return out
}

// extractCTResourceIDs scans the event's Resources slice for entries matching
// awsResourceType (e.g. "AWS::EC2::Instance") and returns one candidate group
// per entry, built by ctIDAlternatives: at most two forms, the value as written
// and, when it starts with a type word, everything after that word's separator.
// Which form is the id varies per target type — a Secrets Manager name keeps its
// slashes, an ARN's resource part does not — so both are offered and the target
// list picks. They are ALTERNATIVES: one event resource is one resource, so
// ctEventsMatchTarget takes at most one match per group, preferring the value as
// written. No fragment of a name is ever a candidate.
func extractCTResourceIDs(event cloudtrailtypes.Event, awsResourceType string) [][]string {
	var groups [][]string
	for _, r := range event.Resources {
		if r.ResourceType == nil || !strings.EqualFold(*r.ResourceType, awsResourceType) {
			continue
		}
		if r.ResourceName == nil || *r.ResourceName == "" {
			continue
		}
		groups = append(groups, ctIDAlternatives(*r.ResourceName))
	}
	return groups
}

// cfnStackNameFromResourceName extracts the stack NAME from a CloudTrail
// CloudFormation resource name. Stack ARNs are ".../stack/<name>/<uuid>"; the
// generic last-segment trim (extractCTResourceIDs) would keep the uuid, but cfn
// resources are keyed by stack name. A bare name (no "stack/" segment) passes
// through unchanged.
func cfnStackNameFromResourceName(group []string) []string {
	s := group[0]
	if len(group) > 1 {
		s = group[1] // the id after the "stack/" type word: "<name>/<uuid>"
	}
	name, _, _ := strings.Cut(s, "/")
	return []string{name}
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
			resp, _ := parsed["responseElements"].(map[string]any)
			fromBody := append(ctJSONStringSlice(req, "instanceId", "instancesSet", "items"),
				ctJSONStringSlice(resp, "instanceId", "instancesSet", "items")...)
			for _, id := range fromBody {
				ids = append(ids, ctIDAlternatives(id))
			}
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
				ids = append(ids, ctIDAlternatives(b))
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
	for i, group := range ids {
		ids[i] = ctLambdaAlternatives(group)
	}

	if len(ids) == 0 {
		parsed := parseCTEventJSON(event.CloudTrailEvent)
		if parsed != nil {
			req, _ := parsed["requestParameters"].(map[string]any)
			if fn := ctJSONString(req, "functionName"); fn != "" {
				ids = append(ids, ctLambdaAlternatives(ctIDAlternatives(fn)))
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

	// DB INSTANCES only. AWS::RDS::DBCluster / DBSnapshot / DBClusterSnapshot
	// identifiers are NOT dbi ids — emitting them here would fake an RDS
	// Instance relation that can never resolve (clusters are a separate pivot).
	ids := extractCTResourceIDs(event, "AWS::RDS::DBInstance")

	if len(ids) == 0 {
		parsed := parseCTEventJSON(event.CloudTrailEvent)
		if parsed != nil {
			req, _ := parsed["requestParameters"].(map[string]any)
			if id := ctJSONString(req, "dBInstanceIdentifier"); id != "" {
				ids = append(ids, ctIDAlternatives(id))
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
				ids = append(ids, ctIDAlternatives(id))
			}
			svcDetails, _ := parsed["serviceEventDetails"].(map[string]any)
			if id := ctJSONString(svcDetails, "keyId"); id != "" {
				ids = append(ids, ctIDAlternatives(id))
			}
		}
	}

	if len(ids) == 0 {
		return resource.KnownRelated("kms", nil, false)
	}

	return ctEventsMatchTarget(ctx, clients, cache, "kms", ids)
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
				ids = append(ids, ctIDAlternatives(id))
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

	var ids [][]string
	parsed := parseCTEventJSON(event.CloudTrailEvent)
	if parsed != nil {
		if id := ctJSONString(parsed, "vpcEndpointId"); id != "" {
			ids = append(ids, ctIDAlternatives(id))
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
				ids = append(ids, ctIDAlternatives(id))
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
				ids = append(ids, ctIDAlternatives(name))
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
					ids = append(ids, ctIDAlternatives(v))
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

	// cfn resources are keyed by stack NAME and a stack id is "<name>/<uuid>",
	// so the uuid is dropped from whatever the extractor produced.
	var ids [][]string
	for _, group := range extractCTResourceIDs(event, "AWS::CloudFormation::Stack") {
		ids = append(ids, cfnStackNameFromResourceName(group))
	}

	if len(ids) == 0 {
		parsed := parseCTEventJSON(event.CloudTrailEvent)
		if parsed != nil {
			req, _ := parsed["requestParameters"].(map[string]any)
			if name := ctJSONString(req, "stackName"); name != "" {
				ids = append(ids, cfnStackNameFromResourceName(ctIDAlternatives(name)))
			}
		}
	}

	if len(ids) == 0 {
		return resource.KnownRelated("cfn", nil, false)
	}

	return ctEventsMatchTarget(ctx, clients, cache, "cfn", ids)
}
