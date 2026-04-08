package ctdetail

import (
	"maps"
	"sort"
	"strings"

	aws "github.com/k2m30/a9s/v3/internal/aws"
)

// ExtractTarget derives the TARGET section rows for a CloudTrail event.
// It implements the #246 §4 fallback algorithm:
//  1. resources[] envelope → one Row per ResourceRef (ARN-stripped)
//  2. Per-event-name lookup table (per-service heuristics on requestParameters)
//  3. Catch-all: scan top-level requestParameters for *Id / *Name / *Arn keys
//
// Fields lifted into TARGET rows are removed from the returned cleanedParams map
// so the REQUEST summarizer does not duplicate them (TARGET-vs-REQUEST de-dup rule).
//
// Guarantees:
//   - Returns non-nil cleanedParams (never mutates the input params map).
//   - When params is nil, cleanedParams is an empty non-nil map.
func ExtractTarget(eventName string, eventSource string, recipientAccountID string, resources []ResourceRef, params map[string]any) (rows []Row, cleanedParams map[string]any) {
	// Guarantee: cleanedParams is always non-nil.
	if params == nil {
		cleanedParams = map[string]any{}
	} else {
		cleanedParams = cloneMap(params)
	}

	// §1: resources[] envelope wins — one Row per ResourceRef.
	if len(resources) > 0 {
		for _, ref := range resources {
			rows = append(rows, resourceRefToRow(ref, recipientAccountID))
		}
		return rows, cleanedParams
	}

	// §2: Per-event-name fallback table.
	rows, cleanedParams = extractByEventName(eventName, params, cleanedParams)
	if len(rows) > 0 {
		return rows, cleanedParams
	}

	// §3: Catch-all — scan top-level params for *Id / *Name / *Arn keys.
	rows, cleanedParams = catchAllScan(params, cleanedParams)
	return rows, cleanedParams
}

// cloneMap returns a shallow copy of m.
func cloneMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	maps.Copy(out, m)
	return out
}

// removeKeys returns a copy of m with the given keys deleted.
func removeKeys(m map[string]any, keys ...string) map[string]any {
	out := cloneMap(m)
	for _, k := range keys {
		delete(out, k)
	}
	return out
}

// navFromLabel returns (IsNavigable, TargetType) for a TARGET row label.
// Only labels whose target types are confirmed registered resource short names
// are marked navigable. Returns (false, "") for unknown or unregistered types.
func navFromLabel(label string) (bool, string) {
	switch label {
	case "Bucket", "Object":
		return true, "s3"
	case "Instance", "Instances":
		return true, "ec2"
	case "Role":
		return true, "role"
	case "User":
		return true, "iam-user"
	case "Key":
		return true, "kms"
	case "Secret":
		return true, "secrets"
	case "Function":
		return true, "lambda"
	case "VPC":
		return true, "vpc"
	case "SG":
		return true, "sg"
	case "Subnet":
		return true, "subnet"
	}
	return false, ""
}

// resourceRefToRow converts a ResourceRef to a Row, deriving the label from the
// resource type or ARN and stripping the ARN to its resource portion.
//
// Cross-account detection: recipientAccountID is the caller's (recipient) account.
// When the ARN's account segment matches recipientAccountID, the account is stripped.
// When it differs, the account is retained (FormatCTTarget prefix logic).
// For S3 bucket ARNs (empty account segment), the resource portion is returned as-is.
func resourceRefToRow(ref ResourceRef, recipientAccountID string) Row {
	key := labelFromType(ref.Type, ref.ARN)
	val := aws.FormatCTTarget(ref.ARN, recipientAccountID)
	if val == "" {
		val = ref.ARN
	}
	isNav, target := navFromLabel(key)
	return Row{Key: key, Value: val, IsNavigable: isNav, TargetType: target}
}

// labelFromType derives the Row.Key label from an AWS resource type string.
// Falls back to labelFromARN for unrecognized types.
func labelFromType(resType string, arn string) string {
	switch resType {
	case "AWS::S3::Bucket":
		return "Bucket"
	case "AWS::S3::Object":
		return "Object"
	case "AWS::EC2::Instance":
		return "Instance"
	case "AWS::IAM::Role":
		return "Role"
	case "AWS::IAM::User":
		return "User"
	case "AWS::KMS::Key":
		return "Key"
	case "AWS::SecretsManager::Secret":
		return "Secret"
	}
	return labelFromARN(arn)
}

// labelFromARN derives a Row.Key label by inspecting the ARN's service and resource segments.
func labelFromARN(arn string) string {
	if !strings.HasPrefix(arn, "arn:") {
		return "Resource"
	}
	parts := strings.SplitN(arn, ":", 6)
	if len(parts) < 6 {
		return "Resource"
	}
	service := parts[2]
	resource := parts[5]
	switch service {
	case "s3":
		if strings.Contains(resource, "/") {
			return "Object"
		}
		return "Bucket"
	case "ec2":
		return "Instance"
	case "iam":
		if strings.HasPrefix(resource, "role/") {
			return "Role"
		}
		if strings.HasPrefix(resource, "user/") {
			return "User"
		}
		return "Resource"
	case "kms":
		return "Key"
	case "secretsmanager":
		return "Secret"
	}
	return "Resource"
}

// extractByEventName implements the per-event-name fallback lookup table.
// params is the requestParameters map (may be nil).
// cleanedParams is a clone of the input params; this function removes lifted keys.
func extractByEventName(eventName string, params map[string]any, cleanedParams map[string]any) ([]Row, map[string]any) {
	switch eventName {
	case "PutObject", "GetObject", "DeleteObject", "CopyObject":
		return extractS3ObjectEvent(params, cleanedParams)

	case "DescribeInstances", "TerminateInstances", "StartInstances", "StopInstances",
		"RebootInstances", "MonitorInstances":
		return extractInstancesSetEvent(eventName, params, cleanedParams)

	case "UpdateInstanceInformation":
		if id, _ := params["instanceId"].(string); id != "" {
			isNav, target := navFromLabel("Instance")
			return []Row{{Key: "Instance", Value: id, IsNavigable: isNav, TargetType: target}}, removeKeys(cleanedParams, "instanceId")
		}

	case "GetParameter":
		if n, _ := params["name"].(string); n != "" {
			return []Row{{Key: "Parameter", Value: n}}, removeKeys(cleanedParams, "name")
		}

	case "GetParameters":
		if names, _ := params["names"].([]any); len(names) > 0 {
			var rows []Row
			for _, n := range names {
				if s, ok := n.(string); ok && s != "" {
					rows = append(rows, Row{Key: "Parameter", Value: s})
				}
			}
			if len(rows) > 0 {
				return rows, removeKeys(cleanedParams, "names")
			}
		}

	case "GetParametersByPath":
		if p, _ := params["path"].(string); p != "" {
			return []Row{{Key: "Parameter", Value: p}}, removeKeys(cleanedParams, "path")
		}

	case "GetSecretValue":
		if id, _ := params["secretId"].(string); id != "" {
			val := aws.FormatCTTarget(id, "")
			isNav, target := navFromLabel("Secret")
			return []Row{{Key: "Secret", Value: val, IsNavigable: isNav, TargetType: target}}, removeKeys(cleanedParams, "secretId")
		}

	case "Decrypt":
		if id, _ := params["keyId"].(string); id != "" {
			val := aws.FormatCTTarget(id, "")
			isNav, target := navFromLabel("Key")
			return []Row{{Key: "Key", Value: val, IsNavigable: isNav, TargetType: target}}, removeKeys(cleanedParams, "keyId")
		}
		isNav, target := navFromLabel("Key")
		return []Row{{Key: "Key", Value: "(by alias)", IsNavigable: isNav, TargetType: target}}, cleanedParams

	case "AssumeRole", "AssumeRoleWithSAML", "AssumeRoleWithWebIdentity":
		if arn, _ := params["roleArn"].(string); arn != "" {
			val := aws.FormatCTTarget(arn, "")
			isNav, target := navFromLabel("Role")
			return []Row{{Key: "Role", Value: val, IsNavigable: isNav, TargetType: target}}, removeKeys(cleanedParams, "roleArn")
		}

	case "BatchGetImage":
		if r, _ := params["repositoryName"].(string); r != "" {
			return []Row{{Key: "Repository", Value: r}}, removeKeys(cleanedParams, "repositoryName")
		}

	case "BatchGetItem":
		if items, _ := params["requestItems"].(map[string]any); len(items) > 0 {
			tables := make([]string, 0, len(items))
			for name := range items {
				tables = append(tables, name)
			}
			sort.Strings(tables)
			var rows []Row
			for _, t := range tables {
				rows = append(rows, Row{Key: "Table", Value: t})
			}
			return rows, removeKeys(cleanedParams, "requestItems")
		}

	case "RotateKey":
		if id, _ := params["keyId"].(string); id != "" {
			val := aws.FormatCTTarget(id, "")
			if val == "" {
				val = id
			}
			val = strings.TrimPrefix(val, "key/")
			isNav, target := navFromLabel("Key")
			return []Row{{Key: "Key", Value: val, IsNavigable: isNav, TargetType: target, FieldPath: "TARGET.Key"}}, removeKeys(cleanedParams, "keyId")
		}

	case "PutBucketPolicy", "GetBucketPolicy", "DeleteBucketPolicy", "PutBucketAcl",
		"PutBucketVersioning", "PutBucketEncryption", "PutBucketLogging", "PutBucketNotification":
		if bucket, _ := params["bucketName"].(string); bucket != "" {
			isNav, target := navFromLabel("Bucket")
			return []Row{{Key: "Bucket", Value: bucket, IsNavigable: isNav, TargetType: target, FieldPath: "TARGET.Bucket"}}, removeKeys(cleanedParams, "bucketName")
		}

	case "ListBuckets":
		return []Row{{Key: "Bucket", Value: "(none)"}}, cleanedParams
	}

	return nil, cleanedParams
}

// extractS3ObjectEvent handles S3 data-plane events: PutObject, GetObject, DeleteObject, CopyObject.
func extractS3ObjectEvent(params map[string]any, cleanedParams map[string]any) ([]Row, map[string]any) {
	if params == nil {
		return nil, cleanedParams
	}
	bucketName, _ := params["bucketName"].(string)
	key, _ := params["key"].(string)
	var rows []Row
	var toRemove []string
	if bucketName != "" {
		isNav, target := navFromLabel("Bucket")
		rows = append(rows, Row{Key: "Bucket", Value: bucketName, IsNavigable: isNav, TargetType: target})
		toRemove = append(toRemove, "bucketName")
	}
	if key != "" {
		isNav, target := navFromLabel("Object")
		// NavID is set to the bucket name so that pressing Enter on an Object row
		// navigates to the parent bucket in the S3 list, not the object path.
		// The display Value retains the full key for human readability.
		rows = append(rows, Row{Key: "Object", Value: key, IsNavigable: isNav, TargetType: target, NavID: bucketName})
		toRemove = append(toRemove, "key")
	}
	if len(rows) == 0 {
		return nil, cleanedParams
	}
	return rows, removeKeys(cleanedParams, toRemove...)
}

// extractInstancesSetEvent handles DescribeInstances, TerminateInstances, etc.
// Returns one Row per instance ID found in requestParameters.instancesSet.items.
// For DescribeInstances with no IDs, returns a single "(all)" row.
func extractInstancesSetEvent(eventName string, params map[string]any, cleanedParams map[string]any) ([]Row, map[string]any) {
	isDescribe := eventName == "DescribeInstances"

	instancesNav, instancesTarget := navFromLabel("Instances")
	instanceNav, instanceTarget := navFromLabel("Instance")

	if params == nil {
		if isDescribe {
			return []Row{{Key: "Instances", Value: "(all)", IsNavigable: instancesNav, TargetType: instancesTarget}}, cleanedParams
		}
		return nil, cleanedParams
	}

	set, _ := params["instancesSet"].(map[string]any)
	if set == nil {
		if isDescribe {
			return []Row{{Key: "Instances", Value: "(all)", IsNavigable: instancesNav, TargetType: instancesTarget}}, cleanedParams
		}
		return nil, cleanedParams
	}

	items, _ := set["items"].([]any)
	if len(items) == 0 {
		if isDescribe {
			return []Row{{Key: "Instances", Value: "(all)", IsNavigable: instancesNav, TargetType: instancesTarget}}, cleanedParams
		}
		return nil, cleanedParams
	}

	var rows []Row
	for _, it := range items {
		m, _ := it.(map[string]any)
		if id, _ := m["instanceId"].(string); id != "" {
			rows = append(rows, Row{Key: "Instance", Value: id, IsNavigable: instanceNav, TargetType: instanceTarget})
		}
	}
	if len(rows) == 0 {
		if isDescribe {
			return []Row{{Key: "Instances", Value: "(all)", IsNavigable: instancesNav, TargetType: instancesTarget}}, cleanedParams
		}
		return nil, cleanedParams
	}

	return rows, removeKeys(cleanedParams, "instancesSet")
}

// catchAllScan scans top-level params for keys ending in Id, Name, or Arn.
// Returns at most one Row for the first match found. ARN values are stripped.
func catchAllScan(params map[string]any, cleanedParams map[string]any) ([]Row, map[string]any) {
	if params == nil {
		return nil, cleanedParams
	}
	for k, v := range params {
		s, ok := v.(string)
		if !ok || s == "" {
			continue
		}
		if strings.HasSuffix(k, "Id") || strings.HasSuffix(k, "Name") || strings.HasSuffix(k, "Arn") {
			val := aws.FormatCTTarget(s, "")
			if val == "" {
				val = s
			}
			return []Row{{Key: "Resource", Value: val}}, removeKeys(cleanedParams, k)
		}
	}
	return nil, cleanedParams
}
