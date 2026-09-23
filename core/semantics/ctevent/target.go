// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package ctevent

import (
	"cmp"
	"maps"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws/arn"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ExtractTarget derives the TARGET section rows for a CloudTrail event.
// Fallback order:
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
	return ExtractTargetInRegion(eventName, eventSource, recipientAccountID, "", resources, params)
}

// recording is where an event was recorded: its recipient account and its
// Region ("" when unknown).
type recording struct{ account, region string }

// ExtractTargetInRegion is ExtractTarget for an event recorded in eventRegion:
// a TARGET row whose ARN names another Region shows the whole ARN, which is
// what carries that Region to navigation.
func ExtractTargetInRegion(eventName, eventSource, recipientAccountID, eventRegion string, resources []ResourceRef, params map[string]any) (rows []Row, cleanedParams map[string]any) {
	rec := recording{account: recipientAccountID, region: eventRegion}
	if params == nil {
		cleanedParams = map[string]any{}
	} else {
		cleanedParams = cloneMap(params)
	}

	// resources[] envelope wins — one Row per ResourceRef.
	if len(resources) > 0 {
		for _, ref := range resources {
			rows = append(rows, resourceRefToRow(ref, rec))
		}
		return rows, cleanedParams
	}

	rows, cleanedParams = extractByEventName(eventName, rec, params, cleanedParams)
	if len(rows) > 0 {
		return rows, cleanedParams
	}

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

// navRow is the TARGET row for ref, labelled key. It opens a row only when
// navLink reads ref as one; a ref naming another Region than the event's
// shows whole, since its Region is part of what it names.
func navRow(key, ref string, rec recording) Row {
	val := cmp.Or(FormatCTTarget(ref, rec.account), ref)
	if named := resource.RefRegion(ref); named != "" && rec.region != "" && named != rec.region {
		val = ref
	}
	row := Row{Key: key, Value: val}
	_, target := navFromLabel(key)
	if id, ok := navLink(target, ref, rec.account); ok {
		row.IsNavigable, row.TargetType, row.NavID = true, target, id
	}
	return row
}

// navLink reads ref through the target type's own resolver
// (resource.ResolveRef, the reading the detail and the related panel use),
// held to the account the event was recorded in, since another account's
// resource is a different row from this account's row of that name, and in
// the Region ref itself names.
func navLink(target, ref, recipientAccountID string) (string, bool) {
	if target == "" {
		return "", false
	}
	return resource.ResolveRef(target, ref, domain.RefContext{AccountID: recipientAccountID, Region: resource.RefRegion(ref)})
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

// resourceRefToRow converts a ResourceRef to a Row, labelled by the
// resource type or ARN.
func resourceRefToRow(ref ResourceRef, rec recording) Row {
	return navRow(labelFromType(ref.Type, ref.ARN), ref.ARN, rec)
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
	case "AWS::Lambda::Function":
		return "Function"
	case "AWS::EC2::VPC":
		return "VPC"
	case "AWS::EC2::SecurityGroup":
		return "SG"
	case "AWS::EC2::Subnet":
		return "Subnet"
	}
	return labelFromARN(arn)
}

// labelFromARN derives a Row.Key label from the ARN's service and the
// resource type that leads its resource part.
func labelFromARN(ref string) string {
	a, err := arn.Parse(ref)
	if err != nil {
		return "Resource"
	}
	switch a.Service {
	case "s3":
		if strings.Contains(a.Resource, "/") {
			return "Object"
		}
		return "Bucket"
	case "kms":
		return "Key"
	case "secretsmanager":
		return "Secret"
	}
	typ, _, _ := strings.Cut(a.Resource, "/")
	if a.Service == "lambda" {
		typ, _, _ = strings.Cut(a.Resource, ":")
	}
	switch a.Service + ":" + typ {
	case "ec2:instance":
		return "Instance"
	case "ec2:vpc":
		return "VPC"
	case "ec2:security-group":
		return "SG"
	case "ec2:subnet":
		return "Subnet"
	case "iam:role":
		return "Role"
	case "iam:user":
		return "User"
	case "lambda:function":
		return "Function"
	}
	return "Resource"
}

// extractByEventName implements the per-event-name fallback lookup table.
// params is the requestParameters map (may be nil).
// cleanedParams is a clone of the input params; this function removes lifted keys.
func extractByEventName(eventName string, rec recording, params map[string]any, cleanedParams map[string]any) ([]Row, map[string]any) {
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
			return []Row{navRow("Secret", id, rec)}, removeKeys(cleanedParams, "secretId")
		}

	case "Decrypt":
		if id, _ := params["keyId"].(string); id != "" {
			return []Row{navRow("Key", id, rec)}, removeKeys(cleanedParams, "keyId")
		}
		isNav, target := navFromLabel("Key")
		return []Row{{Key: "Key", Value: "(by alias)", IsNavigable: isNav, TargetType: target}}, cleanedParams

	case "AssumeRole", "AssumeRoleWithSAML", "AssumeRoleWithWebIdentity":
		if roleARN, _ := params["roleArn"].(string); roleARN != "" {
			return []Row{navRow("Role", roleARN, rec)}, removeKeys(cleanedParams, "roleArn")
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
			row := navRow("Key", id, rec)
			row.Value = cmp.Or(row.NavID, row.Value)
			row.FieldPath = "TARGET.Key"
			return []Row{row}, removeKeys(cleanedParams, "keyId")
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

	_, instanceTarget := navFromLabel("Instance")

	if params == nil {
		if isDescribe {
			// "(all)" is a display-only placeholder — not a real instance ID, so not navigable.
			return []Row{{Key: "Instances", Value: "(all)"}}, cleanedParams
		}
		return nil, cleanedParams
	}

	set, _ := params["instancesSet"].(map[string]any)
	if set == nil {
		if isDescribe {
			return []Row{{Key: "Instances", Value: "(all)"}}, cleanedParams
		}
		return nil, cleanedParams
	}

	items, _ := set["items"].([]any)
	if len(items) == 0 {
		if isDescribe {
			return []Row{{Key: "Instances", Value: "(all)"}}, cleanedParams
		}
		return nil, cleanedParams
	}

	var rows []Row
	for _, it := range items {
		m, _ := it.(map[string]any)
		if id, _ := m["instanceId"].(string); id != "" {
			rows = append(rows, Row{Key: "Instance", Value: id, IsNavigable: true, TargetType: instanceTarget})
		}
	}
	if len(rows) == 0 {
		if isDescribe {
			return []Row{{Key: "Instances", Value: "(all)"}}, cleanedParams
		}
		return nil, cleanedParams
	}

	return rows, removeKeys(cleanedParams, "instancesSet")
}

// catchAllScan scans top-level params for keys ending in Arn, Id or Name and
// returns at most one Row for the most specific of them: an ARN names the
// subject exactly, an id next, a name least. Ties inside a rank go to the key
// that sorts first, so two renders of one event pick the same key — a Go map
// yields its keys in a different order every time. ARN values are stripped.
func catchAllScan(params map[string]any, cleanedParams map[string]any) ([]Row, map[string]any) {
	bestKey, bestVal, bestRank := "", "", 0
	for k, v := range params {
		s, ok := v.(string)
		if !ok || s == "" {
			continue
		}
		rank := 0
		switch {
		case strings.HasSuffix(k, "Arn"):
			rank = 3
		case strings.HasSuffix(k, "Id"):
			rank = 2
		case strings.HasSuffix(k, "Name"):
			rank = 1
		default:
			continue
		}
		if rank > bestRank || (rank == bestRank && k < bestKey) {
			bestKey, bestVal, bestRank = k, s, rank
		}
	}
	if bestRank == 0 {
		return nil, cleanedParams
	}
	val := FormatCTTarget(bestVal, "")
	if val == "" {
		val = bestVal
	}
	return []Row{{Key: "Resource", Value: val}}, removeKeys(cleanedParams, bestKey)
}
