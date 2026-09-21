// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

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

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/semantics/ctevent"
)

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

// ctAltPageToken marks a continuation token as belonging to the lookup under
// the alternate spelling, so every later page of a list is asked for with the
// spelling that answered its first page. CloudTrail rejects a token presented
// with different attributes.
const ctAltPageToken = "alt|"

// ctServerFilter returns the LookupAttributes to send for filter. A key
// beginning with "_" is a9s's own — the endpoint's Region, the row's other
// spelling, the parent an event has to name — and no LookupAttributeKey
// begins with one, so those are left out.
func ctServerFilter(filter map[string]string) map[string]string {
	server := make(map[string]string, len(filter))
	for k, v := range filter {
		if strings.HasPrefix(k, "_") {
			continue
		}
		server[k] = v
	}
	return server
}

// FetchCloudTrailEventsPageFiltered calls the CloudTrail LookupEvents API with
// server-side attribute filters and returns a single page of matching events.
// filter keys must be valid CloudTrail LookupAttributeKey values (e.g.
// "Username", "ResourceName").
//
// CloudTrail records a resource under its bare name or under its ARN per API
// call, and an account can hold both spellings for one resource. A first page
// that comes back empty is therefore asked once more under the spelling on
// resource.CTAltNameFilterKey, and the list then pages whichever spelling
// answered. Where both spellings occur in one account the list is what the
// answering one holds — see docs/related-resources.md §4.
func FetchCloudTrailEventsPageFiltered(ctx context.Context, api CloudTrailLookupEventsAPI, filter map[string]string, continuationToken string) (resource.FetchResult, error) {
	server := ctServerFilter(filter)
	alt := filter[resource.CTAltNameFilterKey]

	parent := filter[resource.CTQualifierFilterKey]
	var paths []string
	if p := filter[resource.CTQualifierPathsKey]; p != "" {
		paths = strings.Split(p, ",")
	}

	if token, isAlt := strings.CutPrefix(continuationToken, ctAltPageToken); isAlt {
		server["ResourceName"] = alt
		return ctLookupPage(ctx, api, server, token, ctAltPageToken, parent, paths)
	}

	page, err := ctLookupPage(ctx, api, server, continuationToken, "", parent, paths)
	if err != nil || alt == "" || continuationToken != "" || len(page.Resources) > 0 {
		return page, err
	}
	server["ResourceName"] = alt
	return ctLookupPage(ctx, api, server, "", ctAltPageToken, parent, paths)
}

// ctLookupPage fetches one LookupEvents page and marks the continuation token
// it returns with tokenPrefix, which tells a later page which spelling this
// one was asked under.
func ctLookupPage(ctx context.Context, api CloudTrailLookupEventsAPI, server map[string]string, continuationToken, tokenPrefix, parent string, paths []string) (resource.FetchResult, error) {
	input := &cloudtrail.LookupEventsInput{
		MaxResults: aws.Int32(DefaultPageSize),
	}
	for k, v := range server {
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
		if !ctEventIsOfParent(event, parent, paths) {
			continue
		}
		resources = append(resources, buildCTResource(event))
	}

	nextToken := ""
	isTruncated := false
	if output.NextToken != nil {
		nextToken = tokenPrefix + *output.NextToken
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

	parsed := parseCTEventJSON(event.CloudTrailEvent)

	eventCategory := strFromMap(parsed, "eventCategory")
	eventType := strFromMap(parsed, "eventType")
	verb := ctevent.ClassifyCTVerb(eventName, eventCategory, eventType)
	errorCode := strFromMap(parsed, "errorCode")
	outcome := "OK"
	if errorCode != "" {
		// The OUTCOME column summarizes; the verbatim code an operator
		// searches for is the detail's own ERROR.errorCode row.
		outcome = domain.HumanizeStatusPhrase(errorCode)
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
		roleName, _ = extractRoleNameFromCTEventJSON(event.CloudTrailEvent)
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
	target := ctTargetCell(event, parsed)
	if target == "(none)" || target == "" {
		// LookupEvents fallback: use event.Resources from the SDK convenience slice.
		for _, res := range event.Resources {
			if res.ResourceName != nil && *res.ResourceName != "" {
				target = *res.ResourceName
				break
			}
		}
	}
	target = ctevent.FormatCTTarget(target, recipientAccount)
	if target == "" {
		target = "(none)"
	}
	sourceIP := strFromMap(parsed, "sourceIPAddress")
	region := strFromMap(parsed, "awsRegion")

	// The severity tier ("ct-info" | "ct-attention" | "ct-danger")
	// and the cause that earned it. The tier is written below to Fields["status"]
	// (branching/color); the cause drives the finding phrase so the operator
	// sees WHY the row is flagged, not just its severity name.
	status, cause := computeCTStatus(verb, eventName, source, errorCode, uiType, accountID, recipientAccount)

	r := resource.Resource{
		ID:       eventID,
		Name:     eventName,
		Findings: ctEventFindings(status, cause, errorCode, eventName),
		Fields: map[string]string{
			"event_name":    eventName,
			"status":        status,
			"time":          eventTimeDisplay,
			"event_time":    eventTimeRaw,
			"user":          iamUserName,
			"source":        source,
			"resource_type": resourceType,
			"resource_name": resourceName,
			"read_only":     readOnly,
			"role_name":     roleName,
			// The caller as CloudTrail's Username attribute records it, which
			// is a session name for an assumed role and a service principal
			// for a service — the value the Username lookup matches, unlike
			// Fields["user"], which names an IAM user or nothing.
			"_ct.username": user,
			// CloudTrail assigns one shared event id to the events a single
			// customer action produced across accounts or services.
			"shared_event_id": strFromMap(parsed, "sharedEventID"),
			// _ct.* keys carry the computed display values.
			"_ct.verb":              verb,
			"_ct.actor":             actor,
			"_ct.origin":            origin,
			"_ct.target":            target,
			"_ct.outcome":           outcome,
			"_ct.cause":             cause,
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

// ctTargetCell is the TARGET column's value: the first row of the same
// extraction the detail's TARGET section renders, so a cell and the detail it
// opens cannot name different resources. A category that names no resource at
// all — an Insight, a network-activity call, a service event — is described
// by ctCategoryTarget instead, ahead of the request parameters, which for
// those events hold the call's arguments rather than its subject.
func ctTargetCell(event cloudtrailtypes.Event, parsed map[string]any) string {
	ev, err := ctevent.Parse(aws.ToString(event.CloudTrailEvent))
	// no finding: an unreadable blob says nothing about the resource the event names.
	if err != nil || ev == nil {
		return ctCategoryTarget(parsed)
	}
	if len(ev.Resources) > 0 {
		// One row per resource the event lists; the cell is as wide as a
		// column, so it names the first and the detail lists the rest.
		if rows, _ := ctevent.ExtractTarget(ev.EventName, ev.EventSource, ev.RecipientAccountID, ev.Resources, nil); len(rows) > 0 {
			return rows[0].Value
		}
	}
	if v := ctCategoryTarget(parsed); v != "" {
		return v
	}
	if ev.RequestParameters == nil {
		return ""
	}
	// A call over several instances or parameters yields one row each, and all
	// of them are its subject.
	rows, _ := ctevent.ExtractTarget(ev.EventName, ev.EventSource, ev.RecipientAccountID, nil, ev.RequestParameters)
	values := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.Value != "" {
			values = append(values, row.Value)
		}
	}
	return strings.Join(values, ",")
}

// ctCategoryTarget describes the events whose subject is not a resource: an
// Insight names the call it found unusual and how far off its baseline it
// ran, a network-activity event names the endpoint and the service reached
// through it, and a service event names the service that acted.
func ctCategoryTarget(parsed map[string]any) string {
	switch {
	case strFromMap(parsed, "eventCategory") == "Insight":
		name := strFromMap(parsed, "eventName")
		if ratio := extractInsightRatio(parsed); ratio != "" {
			return name + " \u00d7" + ratio
		}
		if name != "" {
			return name
		}
		return "(insight)"

	case strFromMap(parsed, "eventCategory") == "NetworkActivity":
		vpce := strFromMap(parsed, "vpcEndpointId")
		svc := strings.TrimSuffix(strFromMap(parsed, "eventSource"), ".amazonaws.com")
		if idx := strings.Index(svc, "."); idx > 0 {
			svc = svc[:idx]
		}
		switch {
		case vpce != "" && svc != "":
			return vpce + " \u2192 " + svc
		case vpce != "":
			return vpce
		case svc != "":
			return svc
		}
		return "(vpce)"

	case strFromMap(parsed, "eventType") == "AwsServiceEvent":
		if src := strFromMap(parsed, "eventSource"); src != "" {
			return src
		}
		return "(service)"
	}
	return ""
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

// computeCTActor computes the _ct.actor string from parsed JSON and the top-level Username.
// Never returns blank — falls back to "-" if no identity can be determined.
// When crossAccount is true, the result is prefixed with "<counterpartyAccountID>/"
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
	formatted := fmt.Sprintf("%.1f", ratio)
	formatted = strings.TrimSuffix(formatted, ".0")
	return formatted
}

// ctEvent cause tags — the reason computeCTStatus assigned a given tier.
// Each maps 1:1 to a branch in computeCTStatus's ladder; ctCauseNone marks the
// ct-info default (no branch matched).
const (
	ctCauseError         = "error"
	ctCauseDestructive   = "destructive"
	ctCauseWrite         = "write"
	ctCauseRoot          = "root"
	ctCauseCrossAccount  = "cross_account"
	ctCauseSensitiveRead = "sensitive_read"
	ctCauseNone          = ""
)

// ctEventFindings builds the wave1 Finding for a CT event. The cause
// computeCTStatus derived picks the code, so the phrase the row shows is that
// code's own registered wording rather than a sentence assembled per event.
func ctEventFindings(status, cause, errorCode, eventName string) []domain.Finding {
	switch status {
	case "ct-danger":
		if cause == ctCauseError {
			return []domain.Finding{wave1Finding(CodeCTEventFailedCall, ctErrorWord(errorCode))}
		}
		return []domain.Finding{wave1Finding(CodeCTEventDanger)}
	case "ct-attention":
		switch cause {
		case ctCauseWrite:
			return []domain.Finding{wave1Finding(CodeCTEventWrite)}
		case ctCauseCrossAccount:
			return []domain.Finding{wave1Finding(CodeCTEventCrossAccount)}
		case ctCauseSensitiveRead:
			return []domain.Finding{wave1Finding(CodeCTEventSensitiveRead, eventName)}
		}
		return []domain.Finding{wave1Finding(CodeCTEventAttention)}
	}
	// ct-info (or any unrecognized tier) — colorCTEvents has no healthy
	// bucket for events, so the routine/no-signal tier still needs a
	// Finding to explain its Dim color on the list/detail surfaces.
	return []domain.Finding{wave1Finding(CodeCTEventInfo)}
}

// ctErrorWord is what the failed-call phrase names as the error.
// computeCTStatus reaches ctCauseError only for a record that carries an
// error code, but colorCTEvents rebuilds the cause from Fields, where a row
// restored without error_code has none — and "failed: " names nothing.
func ctErrorWord(errorCode string) string {
	if word := domain.HumanizeStatusPhrase(errorCode); word != "" {
		return word
	}
	return "unknown error"
}

// computeCTStatus implements the severity ladder, returning the tier
// and the cause that earned it. Precedence: danger > attention > info.
// Highest match wins, top to bottom, within each tier.
func computeCTStatus(verb, eventName, eventSource, errorCode, userIdentityType, accountID, recipientAccountID string) (tier, cause string) {
	// 1. ct-danger
	if errorCode != "" {
		return "ct-danger", ctCauseError
	}
	if verb == "D" {
		return "ct-danger", ctCauseDestructive
	}
	// 2. ct-attention
	if verb == "W" {
		return "ct-attention", ctCauseWrite
	}
	if userIdentityType == "Root" {
		return "ct-attention", ctCauseRoot
	}
	if accountID != "" && recipientAccountID != "" && accountID != recipientAccountID {
		return "ct-attention", ctCauseCrossAccount
	}
	if isSensitiveRead(eventSource, eventName) {
		return "ct-attention", ctCauseSensitiveRead
	}
	// 3. ct-info (default)
	return "ct-info", ctCauseNone
}

// isSensitiveRead reports whether an event is in the hard-coded
// sensitive-reads allowlist. Match is exact "<service>:<eventName>" where
// service is derived from eventSource by stripping ".amazonaws.com".
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

	// STS session vending
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
		Type           string `json:"type"`
		InvokedBy      string `json:"invokedBy"`
		SessionContext struct {
			SessionIssuer struct {
				UserName string `json:"userName"`
				Arn      string `json:"arn"`
			} `json:"sessionIssuer"`
		} `json:"sessionContext"`
	} `json:"userIdentity"`
}

// extractRoleNameFromCTEventJSON parses the raw CloudTrailEvent JSON string and
// returns, for an AssumedRole/Role identity, the issuing role's name
// (userIdentity.sessionContext.sessionIssuer.userName, e.g. "AccountAccessRole")
// and its ARN, which carries the role's account.
//
// Returns "" for nil input, parse errors, or other identity types (e.g., IAMUser — those
// events already have Username set on the CloudTrail Event struct itself).
func extractRoleNameFromCTEventJSON(cloudTrailEvent *string) (name, roleARN string) {
	if cloudTrailEvent == nil || *cloudTrailEvent == "" {
		return "", ""
	}
	var parsed ctEventJSONUserIdentity
	if err := json.Unmarshal([]byte(*cloudTrailEvent), &parsed); err != nil {
		return "", ""
	}
	switch parsed.UserIdentity.Type {
	case "AssumedRole", "Role":
		issuer := parsed.UserIdentity.SessionContext.SessionIssuer
		return issuer.UserName, issuer.Arn
	}
	return "", ""
}

// ctLocalPrincipals clears role_name and user on every event whose principal
// is in another account than the session's: those fields open the local role
// or user of that name, which is not the principal the event names. With the
// session account unknown, nothing is cleared.
func ctLocalPrincipals(c *ServiceClients) func(resource.FetchResult, error) (resource.FetchResult, error) {
	return func(res resource.FetchResult, err error) (resource.FetchResult, error) {
		account := ""
		if store := c.IdentityStore(); store != nil {
			account = store.AccountID()
		}
		for _, r := range res.Resources {
			if a := r.Fields["_ct.account_id"]; account != "" && a != "" && a != account {
				r.Fields["role_name"] = ""
				r.Fields["user"] = ""
			}
		}
		return res, err
	}
}

// ctEventIsOfParent reports whether the event acted on something belonging to
// parent. An event whose body names a different parent belongs to the row of
// that name under that parent, not to this one; an event that names none
// answers nothing either way and is kept, the rule an alarm's qualifier
// dimension already follows. The value a path carries is a bare name or an
// ARN of it.
func ctEventIsOfParent(event cloudtrailtypes.Event, parent string, paths []string) bool {
	if parent == "" || len(paths) == 0 {
		return true
	}
	parsed := parseCTEventJSON(event.CloudTrailEvent)
	if parsed == nil {
		return true
	}
	named := false
	for _, path := range paths {
		value := ctEventPathValue(parsed, path)
		if value == "" {
			continue
		}
		named = true
		if lastSegment(value, "/") == parent {
			return true
		}
	}
	return !named
}

// ctEventPathValue reads a dotted path out of a parsed event body, and ""
// when the path names nothing or names something that is not a string.
func ctEventPathValue(parsed map[string]any, path string) string {
	var node any = parsed
	for key := range strings.SplitSeq(path, ".") {
		m, ok := node.(map[string]any)
		if !ok {
			return ""
		}
		node = m[key]
	}
	s, _ := node.(string)
	return s
}
