package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("ct-events", []string{"event_name", "time", "event_time", "event_time_raw", "user", "source", "resource_type", "resource_name", "read_only", "role_name", "_ct.target_raw"})

	// Paginated fetcher for resource list browsing (M key load-more).
	resource.RegisterPaginated("ct-events", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchCloudTrailEventsPage(ctx, c.CloudTrail, continuationToken)
	})

	// Filtered paginated fetcher for related navigation (e.g., IAM User → ct-events via Username).
	resource.RegisterFilteredPaginated("ct-events", func(ctx context.Context, clients any, filter map[string]string, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchCloudTrailEventsPageFiltered(ctx, c.CloudTrail, filter, continuationToken)
	})

	resource.RegisterRelated("ct-events", []resource.RelatedDef{
		{TargetType: "role", DisplayName: "IAM Roles", Checker: checkCtEventsRole, NeedsTargetCache: true},
		{TargetType: "iam-user", DisplayName: "IAM Users", Checker: checkCtEventsUser, NeedsTargetCache: true},
		{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkCtEventsEC2, NeedsTargetCache: true},
		{TargetType: "s3", DisplayName: "S3 Buckets", Checker: checkCtEventsS3, NeedsTargetCache: true},
		{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkCtEventsLambda, NeedsTargetCache: true},
		{TargetType: "dbi", DisplayName: "RDS Instances", Checker: checkCtEventsRDS, NeedsTargetCache: true},
		{TargetType: "kms", DisplayName: "KMS Keys", Checker: checkCtEventsKMS, NeedsTargetCache: true},
		{TargetType: "secrets", DisplayName: "Secrets", Checker: checkCtEventsSecrets, NeedsTargetCache: true},
		{TargetType: "vpce", DisplayName: "VPC Endpoints", Checker: checkCtEventsVPCE, NeedsTargetCache: true},
		{TargetType: "sg", DisplayName: "Security Groups", Checker: checkCtEventsSG, NeedsTargetCache: true},
		{TargetType: "ddb", DisplayName: "DynamoDB Tables", Checker: checkCtEventsDDB, NeedsTargetCache: true},
		{TargetType: "cfn", DisplayName: "CloudFormation Stacks", Checker: checkCtEventsCFN, NeedsTargetCache: true},
		// Self-pivot entries (ct-events → ct-events): navigate to events filtered by attribute.
		{TargetType: "ct-events", DisplayName: "CT events by AccessKeyId", Checker: checkCtEventsPivotByAccessKeyId, NeedsTargetCache: false},
		{TargetType: "ct-events", DisplayName: "CT events by Username", Checker: checkCtEventsPivotByUsername, NeedsTargetCache: false},
		{TargetType: "ct-events", DisplayName: "CT events by EventName", Checker: checkCtEventsPivotByEventName, NeedsTargetCache: false},
		{TargetType: "ct-events", DisplayName: "CT events by SharedEventId", Checker: checkCtEventsPivotBySharedEventId, NeedsTargetCache: false},
	})

	resource.RegisterNavigableFields("ct-events", []resource.NavigableField{
		{FieldPath: "user", TargetType: "iam-user"},
		{FieldPath: "role_name", TargetType: "role"},
	})
}

// FetchCloudTrailEvents fetches all CloudTrail LookupEvents pages and returns
// the combined resources. Used by related-resource cold-cache checks and tests.
func FetchCloudTrailEvents(ctx context.Context, api CloudTrailLookupEventsAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchCloudTrailEventsPage(ctx, api, token)
		if err != nil {
			return nil, err
		}
		all = append(all, result.Resources...)
		if result.Pagination == nil || !result.Pagination.IsTruncated {
			break
		}
		token = result.Pagination.NextToken
	}
	return all, nil
}

// FetchCloudTrailEventsPage calls the CloudTrail LookupEvents API and returns
// a single page of events. Pass an empty continuationToken for the first page.
func FetchCloudTrailEventsPage(ctx context.Context, api CloudTrailLookupEventsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &cloudtrail.LookupEventsInput{
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.LookupEvents(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching CloudTrail events: %w", err)
	}

	resources := make([]resource.Resource, 0, len(output.Events))
	for _, event := range output.Events {
		r := buildCTResource(event)
		resources = append(resources, r)
	}

	// Build pagination metadata
	nextToken := ""
	isTruncated := false
	if output.NextToken != nil {
		nextToken = *output.NextToken
		isTruncated = true
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   -1,
		},
	}, nil
}

// FetchCloudTrailEventsPageFiltered calls the CloudTrail LookupEvents API with server-side
// attribute filters and returns a single page of matching events.
// filter keys must be valid CloudTrail LookupAttributeKey values (e.g., "Username", "ResourceName").
func FetchCloudTrailEventsPageFiltered(ctx context.Context, api CloudTrailLookupEventsAPI, filter map[string]string, continuationToken string) (resource.FetchResult, error) {
	input := &cloudtrail.LookupEventsInput{
		MaxResults: aws.Int32(DefaultPageSize),
	}
	for k, v := range filter {
		input.LookupAttributes = append(input.LookupAttributes, cloudtrailtypes.LookupAttribute{
			AttributeKey:   cloudtrailtypes.LookupAttributeKey(k),
			AttributeValue: aws.String(v),
		})
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.LookupEvents(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching filtered CloudTrail events: %w", err)
	}

	resources := make([]resource.Resource, 0, len(output.Events))
	for _, event := range output.Events {
		r := buildCTResource(event)
		resources = append(resources, r)
	}

	// Build pagination metadata
	nextToken := ""
	isTruncated := false
	if output.NextToken != nil {
		nextToken = *output.NextToken
		isTruncated = true
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   -1,
		},
	}, nil
}

// buildCTResource converts a cloudtrailtypes.Event into a resource.Resource,
// parsing the embedded CloudTrailEvent JSON and writing _ct.* fields.
func buildCTResource(event cloudtrailtypes.Event) resource.Resource {
	eventID := ""
	if event.EventId != nil {
		eventID = *event.EventId
	}

	eventName := ""
	if event.EventName != nil {
		eventName = *event.EventName
	}

	eventTimeRaw := ""
	if event.EventTime != nil {
		eventTimeRaw = event.EventTime.Format(time.RFC3339)
	}
	eventTimeDisplay := FormatCTTimestamp(eventTimeRaw)

	user := ""
	if event.Username != nil {
		user = *event.Username
	}

	source := ""
	if event.EventSource != nil {
		source = *event.EventSource
	}

	resourceType, resourceName := cloudTrailResourceFields(event.Resources)

	// ReadOnly is *string ("true" or "false")
	readOnly := ""
	if event.ReadOnly != nil {
		readOnly = *event.ReadOnly
	}

	// Parse the CloudTrailEvent JSON blob once.
	parsed := parseCTEventJSON(event.CloudTrailEvent)

	// Compute _ct.* fields.
	eventCategory := strFromMap(parsed, "eventCategory")
	eventType := strFromMap(parsed, "eventType")
	verb := ClassifyCTVerb(eventName, eventCategory, eventType)
	errorCode := strFromMap(parsed, "errorCode")
	outcome := "OK"
	if errorCode != "" {
		outcome = errorCode
	}
	accountID := ""
	uiType := ""
	if ui, ok := parsed["userIdentity"].(map[string]any); ok {
		accountID, _ = ui["accountId"].(string)
		uiType, _ = ui["type"].(string)
	}
	recipientAccount := strFromMap(parsed, "recipientAccountId")
	isRoot := "false"
	if uiType == "Root" {
		isRoot = "true"
	}
	crossAccount := "false"
	if accountID != "" && recipientAccount != "" && accountID != recipientAccount {
		crossAccount = "true"
	}

	// Only extract roleName for actual role-based identities (not AWSService).
	// AWSService invokedBy values are service principals, not IAM roles.
	roleName := ""
	if uiType == "AssumedRole" || uiType == "Role" {
		roleName = extractRoleNameFromCTEventJSON(event.CloudTrailEvent)
	}

	// Fields["user"] navigates to iam-user — only set it for actual IAM users.
	// For AssumedRole/Root/AWSService/Role events, event.Username is a role/session
	// name, not an IAM username. Use a separate variable so computeCTActor still
	// receives the original SDK username for display purposes.
	// When uiType is empty (no CloudTrailEvent JSON blob), trust event.Username.
	iamUserName := ""
	if uiType == "IAMUser" || uiType == "" {
		iamUserName = user
	}

	actor := computeCTActor(parsed, user, crossAccount == "true", accountID)
	origin := computeCTOrigin(parsed)
	target := ExtractCTTarget(parsed)
	if target == "(none)" || target == "" {
		// Step 1.5: per-event-name fallback table (§4).
		// Only fires when resources[] is empty and event is a management event.
		eventNameForTarget, _ := parsed["eventName"].(string)
		if t := extractTargetByEventName(eventNameForTarget, parsed); t != "" {
			target = t
		}
	}
	if target == "(none)" || target == "" {
		// LookupEvents fallback: use event.Resources from the SDK convenience slice.
		for _, res := range event.Resources {
			if res.ResourceName != nil && *res.ResourceName != "" {
				target = *res.ResourceName
				break
			}
		}
	}
	targetRaw := target
	target = FormatCTTarget(target, recipientAccount)
	sourceIP := strFromMap(parsed, "sourceIPAddress")
	region := strFromMap(parsed, "awsRegion")

	// Set Resource.Status using §1.2 severity ladder.
	status := computeCTStatus(verb, eventName, source, errorCode, uiType, accountID, recipientAccount)

	r := resource.Resource{
		ID:     eventID,
		Name:   eventName,
		Status: status,
		Fields: map[string]string{
			// Existing keys (kept for backwards compat with related-checkers and tests).
			"event_name":    eventName,
			"time":          eventTimeDisplay,
			"event_time":    eventTimeRaw,
			"event_time_raw": eventTimeRaw,
			"user":          iamUserName,
			"source":        source,
			"resource_type": resourceType,
			"resource_name": resourceName,
			"read_only":     readOnly,
			"role_name":     roleName,
			// New _ct.* keys.
			"_ct.verb":              verb,
			"_ct.actor":             actor,
			"_ct.origin":            origin,
			"_ct.target":            target,
			"_ct.target_raw":        targetRaw,
			"_ct.outcome":           outcome,
			"_ct.error_code":        errorCode,
			"_ct.account_id":        accountID,
			"_ct.recipient_account": recipientAccount,
			"_ct.is_root":           isRoot,
			"_ct.cross_account":     crossAccount,
			"_ct.event_category":    eventCategory,
			"_ct.event_type":        eventType,
			"_ct.source_ip":         sourceIP,
			"_ct.region":            region,
		},
		RawStruct: event,
	}
	return r
}

// parseCTEventJSON parses the raw CloudTrailEvent JSON blob into a map.
// Returns an empty map on nil/empty input or parse errors (never panics).
func parseCTEventJSON(s *string) map[string]any {
	if s == nil || *s == "" {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(*s), &m); err != nil {
		return map[string]any{}
	}
	return m
}

// strFromMap returns a string value from a map, empty string if absent or wrong type.
func strFromMap(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, _ := m[key].(string)
	return v
}

// ClassifyCTVerb classifies a CloudTrail event into one of:
// "R" (read), "W" (write), "D" (destructive), "S" (service event),
// "I" (insight), "N" (network activity), "?" (unknown).
//
// Implements §2.1 of docs/design/ct-event-list-v2.md. Order matters; first match wins.
//  1. eventCategory == "Insight" → "I"
//  2. eventCategory == "NetworkActivity" → "N"
//  3. eventType == "AwsServiceEvent" → "S"
//  4. BatchGet* prefix → "R" (must beat Batch write prefix)
//  5. KMS use-key exact names → "R" (Decrypt, Encrypt, Sign, Verify, ReEncrypt, GenerateDataKey*)
//  6. Destructive prefix table → "D"
//  7. Read prefix table → "R"
//  8. Write prefix table → "W" ("Assume" prefix catches non-STS Assume* events; all AssumeRole* are exact-match R above)
//  9. "?" (no match)
func ClassifyCTVerb(eventName, eventCategory, eventType string) string {
	// Category / type overrides (highest precedence).
	switch eventCategory {
	case "Insight":
		return "I"
	case "NetworkActivity":
		return "N"
	}
	if eventType == "AwsServiceEvent" {
		return "S"
	}

	// Special-case reads BEFORE prefix matching.
	// BatchGet* must beat the Batch write prefix.
	if strings.HasPrefix(eventName, "BatchGet") {
		return "R"
	}
	// BatchDelete* must beat the Batch write prefix.
	if strings.HasPrefix(eventName, "BatchDelete") {
		return "D"
	}
	// KMS use-key ops and STS role-vending — exact matches (§2.1 row 2 additional).
	// All AssumeRole* operations are STS session vending (identity exchange, not state
	// mutation). They are exact-matched here so the "Assume" W-prefix below only catches
	// non-STS Assume* events from other services (if any).
	switch eventName {
	case "Decrypt", "Encrypt", "Sign", "Verify",
		"ReEncrypt", "GenerateDataKey", "GenerateDataKeyWithoutPlaintext",
		"AssumeRole", "AssumeRoleWithSAML", "AssumeRoleWithWebIdentity":
		return "R"
	}

	// Destructive prefix table (§2.1 row 1).
	for _, p := range []string{
		"Delete", "Terminate", "Destroy", "Remove", "Revoke", "Disable",
		"Stop", "Detach", "Cancel", "Reject", "Abort", "Purge",
		"Deregister", "Disassociate",
	} {
		if strings.HasPrefix(eventName, p) {
			return "D"
		}
	}

	// Read prefix table (§2.1 row 2).
	for _, p := range []string{
		"Get", "Describe", "List", "Lookup", "Search", "Query",
		"Scan", "Head", "Test", "Check", "Validate", "Verify",
	} {
		if strings.HasPrefix(eventName, p) {
			return "R"
		}
	}

	// Write prefix table (§2.1 row 4).
	for _, p := range []string{
		"Create", "Put", "Update", "Modify", "Set", "Add",
		"Attach", "Associate", "Register", "Enable", "Start", "Run",
		"Restore", "Restart", "Reboot", "Tag", "Untag", "Activate",
		"Reset", "Replace", "Apply", "Import", "Export", "Copy",
		"Move", "Upload", "Submit", "Send", "Publish", "Invoke",
		"Execute", "Transition", "Issue", "Renew", "Rotate",
		"Batch", "Assume",
	} {
		if strings.HasPrefix(eventName, p) {
			return "W"
		}
	}

	return "?"
}

// computeCTActor computes the _ct.actor string from parsed JSON and the top-level Username.
// Never returns blank — falls back to "-" if no identity can be determined.
// When crossAccount is true, the result is prefixed with "<counterpartyAccountID>/" per §1.4
// (except for "-", which indicates no actor was identified). counterpartyAccount is the
// userIdentity.accountId. Note: ROOT actors DO receive the prefix — the counterparty account
// identity is exactly the high-signal information the user needs for cross-account root events.
func computeCTActor(parsed map[string]any, topLevelUser string, crossAccount bool, counterpartyAccount string) string {
	actor := computeCTActorInner(parsed, topLevelUser)
	if crossAccount && actor != "-" && counterpartyAccount != "" {
		return counterpartyAccount + "/" + actor
	}
	return actor
}

// computeCTActorInner resolves the raw actor string without cross-account prefix.
func computeCTActorInner(parsed map[string]any, topLevelUser string) string {
	if parsed == nil {
		if topLevelUser != "" {
			return topLevelUser
		}
		return "-"
	}
	ui, hasUI := parsed["userIdentity"].(map[string]any)
	if !hasUI {
		if topLevelUser != "" {
			return topLevelUser
		}
		return "-"
	}
	uiType, _ := ui["type"].(string)

	switch uiType {
	case "Root":
		return "ROOT"
	case "IAMUser":
		if name, _ := ui["userName"].(string); name != "" {
			return name
		}
		if topLevelUser != "" {
			return topLevelUser
		}
	case "AssumedRole", "Role":
		// Use sessionContext.sessionIssuer.userName / session name.
		if sc, ok := ui["sessionContext"].(map[string]any); ok {
			if si, ok := sc["sessionIssuer"].(map[string]any); ok {
				if roleName, _ := si["userName"].(string); roleName != "" {
					// Append session name if available.
					if arn, _ := ui["arn"].(string); arn != "" {
						// Extract session name from arn: arn:aws:sts::…:assumed-role/<role>/<session>
						parts := strings.Split(arn, "/")
						if len(parts) >= 3 {
							sessionName := parts[len(parts)-1]
							return roleName + "/" + sessionName
						}
					}
					return roleName
				}
			}
		}
		if topLevelUser != "" {
			return topLevelUser
		}
	case "AWSService":
		if invokedBy, _ := ui["invokedBy"].(string); invokedBy != "" {
			return invokedBy
		}
		if src, _ := parsed["eventSource"].(string); src != "" {
			return src
		}
	case "FederatedUser":
		if principalID, _ := ui["principalId"].(string); principalID != "" {
			return principalID
		}
		if topLevelUser != "" {
			return topLevelUser
		}
	case "WebIdentityUser":
		if name, _ := ui["userName"].(string); name != "" {
			return name
		}
		if topLevelUser != "" {
			return topLevelUser
		}
	case "SAMLUser":
		if name, _ := ui["userName"].(string); name != "" {
			return name
		}
		if topLevelUser != "" {
			return topLevelUser
		}
	}

	if topLevelUser != "" {
		return topLevelUser
	}
	return "-"
}

// computeCTOrigin derives the _ct.origin label from userAgent and sessionCredentialFromConsole.
// Returns one of: "Console", "CLI", "SDK", "Service", "TF", "Boto", "Browser", "VPCE", "?"
func computeCTOrigin(parsed map[string]any) string {
	if parsed == nil {
		return "?"
	}
	ua, _ := parsed["userAgent"].(string)
	uaLow := strings.ToLower(ua)

	// sessionCredentialFromConsole overrides UA for Console detection.
	// In CloudTrail JSON this lives under userIdentity.sessionContext.
	if ui, ok := parsed["userIdentity"].(map[string]any); ok {
		if sc, ok := ui["sessionContext"].(map[string]any); ok {
			switch v := sc["sessionCredentialFromConsole"].(type) {
			case string:
				if v == "true" {
					return "Console"
				}
			case bool:
				if v {
					return "Console"
				}
			}
		}
	}

	switch {
	case strings.Contains(uaLow, "console"):
		return "Console"
	case strings.Contains(uaLow, "terraform"):
		return "TF"
	case strings.Contains(uaLow, "boto"):
		return "Boto"
	case strings.Contains(uaLow, "aws-cli"):
		return "CLI"
	case strings.Contains(uaLow, "vpce") || strings.Contains(uaLow, "vpcendpoint"):
		return "VPCE"
	case strings.Contains(uaLow, "mozilla") || strings.Contains(uaLow, "chrome") ||
		strings.Contains(uaLow, "safari") || strings.Contains(uaLow, "firefox"):
		return "Browser"
	case ua == "":
		// AwsServiceEvent or internal AWS call
		if t, _ := parsed["eventType"].(string); t == "AwsServiceEvent" {
			return "Service"
		}
		if ui, ok := parsed["userIdentity"].(map[string]any); ok {
			if uiType, _ := ui["type"].(string); uiType == "AWSService" {
				return "Service"
			}
		}
		return "?"
	case strings.Contains(uaLow, "amazonaws.com") || strings.Contains(uaLow, ".internal"):
		return "Service"
	case strings.Contains(uaLow, "aws-sdk"):
		return "SDK"
	default:
		return "SDK"
	}
}

// ExtractCTTarget derives the _ct.target string from the parsed CloudTrailEvent JSON.
// Never returns blank — falls back to "(none)" for management events with no resources.
func ExtractCTTarget(parsed map[string]any) string {
	if parsed == nil {
		return "(none)"
	}

	// 1. resources[] — use first non-empty resource.
	if res, ok := parsed["resources"].([]any); ok && len(res) > 0 {
		if first, ok := res[0].(map[string]any); ok {
			// Prefer ARN, then resourceName.
			if arn, _ := first["ARN"].(string); arn != "" {
				return arn
			}
			if name, _ := first["resourceName"].(string); name != "" {
				return name
			}
		}
	}

	eventCategory, _ := parsed["eventCategory"].(string)
	eventType, _ := parsed["eventType"].(string)

	// 2. Insight category → "<eventName> ×<ratio>"
	if eventCategory == "Insight" {
		eventName, _ := parsed["eventName"].(string)
		ratio := extractInsightRatio(parsed)
		if ratio != "" {
			return eventName + " \u00d7" + ratio
		}
		if eventName != "" {
			return eventName
		}
		return "(insight)"
	}

	// 3. NetworkActivity → "<vpce-id> → <svc>"
	if eventCategory == "NetworkActivity" {
		vpce, _ := parsed["vpcEndpointId"].(string)
		svc := ""
		if src, _ := parsed["eventSource"].(string); src != "" {
			// Strip .amazonaws.com suffix to get short service name.
			svc = strings.TrimSuffix(src, ".amazonaws.com")
			if idx := strings.Index(svc, "."); idx > 0 {
				svc = svc[:idx]
			}
		}
		if vpce != "" && svc != "" {
			return vpce + " \u2192 " + svc
		}
		if vpce != "" {
			return vpce
		}
		if svc != "" {
			return svc
		}
		return "(vpce)"
	}

	// 4. AwsServiceEvent → service principal (eventSource).
	if eventType == "AwsServiceEvent" {
		if src, _ := parsed["eventSource"].(string); src != "" {
			return src
		}
		return "(service)"
	}

	// 5. Management event with no resources → "(none)".
	return "(none)"
}

// extractInsightRatio computes the ratio string for Insight events.
// Returns e.g. "4.2" from insightDetails.insightContext.statistics.
func extractInsightRatio(parsed map[string]any) string {
	id, ok := parsed["insightDetails"].(map[string]any)
	if !ok {
		return ""
	}
	ic, ok := id["insightContext"].(map[string]any)
	if !ok {
		return ""
	}
	stats, ok := ic["statistics"].(map[string]any)
	if !ok {
		return ""
	}
	baseline, _ := stats["baseline"].(map[string]any)
	insight, _ := stats["insight"].(map[string]any)
	if baseline == nil || insight == nil {
		return ""
	}
	baseAvg, _ := baseline["average"].(float64)
	insightAvg, _ := insight["average"].(float64)
	if baseAvg == 0 {
		return ""
	}
	ratio := insightAvg / baseAvg
	// Format to 1 decimal place.
	formatted := fmt.Sprintf("%.1f", ratio)
	formatted = strings.TrimSuffix(formatted, ".0")
	return formatted
}

// computeCTStatus implements the §1.2 severity ladder.
// Precedence: danger > attention > info. Highest match wins, top to bottom.
func computeCTStatus(verb, eventName, eventSource, errorCode, userIdentityType, accountID, recipientAccountID string) string {
	// 1. ct-danger
	if errorCode != "" {
		return "ct-danger"
	}
	if verb == "D" {
		return "ct-danger"
	}
	// 2. ct-attention
	if verb == "W" {
		return "ct-attention"
	}
	if userIdentityType == "Root" {
		return "ct-attention"
	}
	if accountID != "" && recipientAccountID != "" && accountID != recipientAccountID {
		return "ct-attention"
	}
	if isSensitiveRead(eventSource, eventName) {
		return "ct-attention"
	}
	// 3. ct-info (default)
	return "ct-info"
}

// isSensitiveRead reports whether an event is in the §1.3 hard-coded
// sensitive-reads allowlist. Match is exact "<service>:<eventName>" where
// service is derived from eventSource by stripping ".amazonaws.com".
// KMS is deliberately excluded.
func isSensitiveRead(eventSource, eventName string) bool {
	svc := eventSource
	if idx := strings.Index(svc, "."); idx > 0 {
		svc = svc[:idx]
	}
	key := svc + ":" + eventName
	switch key {
	// Secrets / parameters
	case "secretsmanager:GetSecretValue",
		"secretsmanager:BatchGetSecretValue",
		"secretsmanager:GetRandomPassword",
		"secretsmanager:ListSecrets",
		"ssm:GetParameter",
		"ssm:GetParameters",
		"ssm:GetParametersByPath",
		"ssm:GetParameterHistory",
		"ssm:DescribeParameters":
		return true

	// STS session vending (AssumeRole* deliberately excluded — too noisy via IRSA/SSO)
	case "sts:GetSessionToken",
		"sts:GetFederationToken":
		return true

	// Cognito admin auth surface
	case "cognito-idp:AdminInitiateAuth",
		"cognito-idp:AdminGetUser":
		return true

	// Code signing
	case "signer:GetSigningProfile":
		return true

	// IAM credential / privilege recon
	case "iam:GetAccessKeyLastUsed",
		"iam:ListAccessKeys",
		"iam:GetCredentialReport",
		"iam:GenerateCredentialReport",
		"iam:GetLoginProfile",
		"iam:GetAccountAuthorizationDetails",
		"iam:SimulatePrincipalPolicy",
		"iam:SimulateCustomPolicy",
		"iam:ListUsers",
		"iam:ListRoles",
		"iam:ListPolicies",
		"iam:ListAttachedRolePolicies",
		"iam:ListRolePolicies",
		"iam:ListMFADevices",
		"iam:ListVirtualMFADevices",
		"iam:ListSSHPublicKeys",
		"iam:ListServiceSpecificCredentials":
		return true

	// Organizations enumeration
	case "organizations:ListAccounts",
		"organizations:DescribeOrganization":
		return true

	// Bulk data exfil via reads
	case "dynamodb:Scan",
		"rds:DownloadDBLogFilePortion":
		return true

	// EC2 instance secret/console exfil
	case "ec2:GetPasswordData",
		"ec2:GetConsoleOutput",
		"ec2:GetConsoleScreenshot":
		return true

	// Account-wide recon
	case "support:DescribeTrustedAdvisorChecks",
		"ce:GetCostAndUsage":
		return true
	}
	return false
}

// FormatCTTimestamp formats an RFC3339 timestamp as "Jan 02 15:04:05" (15 chars).
// Empty input returns "". Invalid input returns the raw input unchanged.
func FormatCTTimestamp(rfc3339 string) string {
	if rfc3339 == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		return rfc3339
	}
	return t.UTC().Format("Jan 02 15:04:05")
}

func cloudTrailResourceFields(resources []cloudtrailtypes.Resource) (string, string) {
	if len(resources) == 0 {
		return "", ""
	}
	types := make([]string, 0, len(resources))
	names := make([]string, 0, len(resources))
	typeSeen := map[string]struct{}{}
	nameSeen := map[string]struct{}{}
	for _, rr := range resources {
		if rr.ResourceType != nil && *rr.ResourceType != "" {
			if _, ok := typeSeen[*rr.ResourceType]; !ok {
				typeSeen[*rr.ResourceType] = struct{}{}
				types = append(types, *rr.ResourceType)
			}
		}
		if rr.ResourceName != nil && *rr.ResourceName != "" {
			if _, ok := nameSeen[*rr.ResourceName]; !ok {
				nameSeen[*rr.ResourceName] = struct{}{}
				names = append(names, *rr.ResourceName)
			}
		}
	}
	return strings.Join(types, ", "), strings.Join(names, ", ")
}

// ctEventJSONUserIdentity is a minimal struct for parsing the CloudTrailEvent JSON string
// to extract the userIdentity.sessionContext.sessionIssuer.userName for AssumedRole events,
// or userIdentity.invokedBy for AWSService events.
type ctEventJSONUserIdentity struct {
	UserIdentity struct {
		Type      string `json:"type"`
		InvokedBy string `json:"invokedBy"`
		SessionContext struct {
			SessionIssuer struct {
				UserName string `json:"userName"`
			} `json:"sessionIssuer"`
		} `json:"sessionContext"`
	} `json:"userIdentity"`
}

// extractRoleNameFromCTEventJSON parses the raw CloudTrailEvent JSON string and returns
// a human-readable identity string for the event:
//   - AssumedRole/Role: userIdentity.sessionContext.sessionIssuer.userName (e.g., "AccountAccessRole")
//   - AWSService: userIdentity.invokedBy (e.g., "ec2.amazonaws.com")
//
// Returns "" for nil input, parse errors, or unrecognised identity types (e.g., IAMUser — those
// events already have Username set on the CloudTrail Event struct itself).
func extractRoleNameFromCTEventJSON(cloudTrailEvent *string) string {
	if cloudTrailEvent == nil || *cloudTrailEvent == "" {
		return ""
	}
	var parsed ctEventJSONUserIdentity
	if err := json.Unmarshal([]byte(*cloudTrailEvent), &parsed); err != nil {
		return ""
	}
	switch parsed.UserIdentity.Type {
	case "AssumedRole", "Role":
		return parsed.UserIdentity.SessionContext.SessionIssuer.UserName
	}
	return ""
}
