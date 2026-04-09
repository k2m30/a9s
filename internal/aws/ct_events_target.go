package aws

import (
	"sort"
	"strings"
)

// FormatCTTarget collapses an ARN to its resource portion per §5 of
// docs/design/ct-event-list-v2.md. When the ARN's account segment differs
// from localAccount, the account ID is retained inline as "<acct>:<resource>".
// Non-ARN input is returned unchanged.
func FormatCTTarget(rawARN, localAccount string) string {
	if rawARN == "" {
		return ""
	}
	if !strings.HasPrefix(rawARN, "arn:") {
		return rawARN
	}
	// arn:aws:<service>:<region>:<account>:<resource>
	// Split on ":" with limit 6 so the resource can itself contain colons.
	parts := strings.SplitN(rawARN, ":", 6)
	if len(parts) < 6 {
		return rawARN
	}
	account := parts[4]
	resource := parts[5]
	// When account is empty (e.g. S3 bucket ARNs like arn:aws:s3:::bucket),
	// return the resource portion — there is no account segment to compare.
	if account == "" {
		return resource
	}
	// When localAccount is unknown (recipientAccountId missing from event),
	// we can't tell if this is cross-account — strip the account so same-account
	// events don't render with a spurious prefix.
	if localAccount == "" {
		return resource
	}
	if account != localAccount {
		return account + ":" + resource
	}
	return resource
}

// extractTargetByEventName implements the §4 per-event-name fallback table.
// Called when resources[] is empty and the event is a management event.
func extractTargetByEventName(eventName string, parsed map[string]any) string {
	req, _ := parsed["requestParameters"].(map[string]any)
	switch eventName {
	case "DescribeInstances":
		// requestParameters.instancesSet.items[*].instanceId → joined "," or "(all)"
		if req == nil {
			return "(all)"
		}
		set, _ := req["instancesSet"].(map[string]any)
		if set == nil {
			return "(all)"
		}
		items, _ := set["items"].([]any)
		if len(items) == 0 {
			return "(all)"
		}
		var ids []string
		for _, it := range items {
			m, _ := it.(map[string]any)
			if id, _ := m["instanceId"].(string); id != "" {
				ids = append(ids, id)
			}
		}
		if len(ids) == 0 {
			return "(all)"
		}
		return strings.Join(ids, ",")
	case "UpdateInstanceInformation":
		if req != nil {
			if id, _ := req["instanceId"].(string); id != "" {
				return id
			}
		}
	case "GetParameter":
		if req != nil {
			if n, _ := req["name"].(string); n != "" {
				return n
			}
		}
	case "GetParameters":
		if req != nil {
			if names, _ := req["names"].([]any); len(names) > 0 {
				var out []string
				for _, n := range names {
					if s, ok := n.(string); ok {
						out = append(out, s)
					}
				}
				return strings.Join(out, ",")
			}
		}
	case "GetParametersByPath":
		if req != nil {
			if p, _ := req["path"].(string); p != "" {
				return p
			}
		}
	case "GetSecretValue":
		if req != nil {
			if id, _ := req["secretId"].(string); id != "" {
				return id
			}
		}
	case "Decrypt":
		if req != nil {
			if id, _ := req["keyId"].(string); id != "" {
				return id
			}
		}
		return "(by alias)"
	case "AssumeRole", "AssumeRoleWithSAML", "AssumeRoleWithWebIdentity":
		if req != nil {
			if arn, _ := req["roleArn"].(string); arn != "" {
				// Return raw ARN; FormatCTTarget in buildCTResource will strip it.
				return arn
			}
		}
	case "BatchGetImage":
		if req != nil {
			if r, _ := req["repositoryName"].(string); r != "" {
				return r
			}
		}
	case "BatchGetItem":
		if req != nil {
			if items, _ := req["requestItems"].(map[string]any); len(items) > 0 {
				tables := make([]string, 0, len(items))
				for tableName := range items {
					tables = append(tables, tableName)
				}
				sort.Strings(tables) // deterministic order
				return strings.Join(tables, ",")
			}
		}
	case "ListBuckets":
		return "(none)"
	}
	// Catch-all: scan for any *Id / *Name / *Arn key at top level of requestParameters.
	// Ranging over a nil map is a no-op in Go, so the nil check is not needed.
	for k, v := range req {
		if s, ok := v.(string); ok && s != "" {
			if strings.HasSuffix(k, "Id") || strings.HasSuffix(k, "Name") || strings.HasSuffix(k, "Arn") {
				return s
			}
		}
	}
	return ""
}
