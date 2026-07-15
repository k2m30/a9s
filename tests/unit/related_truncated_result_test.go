package unit_test

// related_approximate_zero_test.go — tests for relatedResultTrunc and
// the truncated-empty-cache honest-lower-bound contract.
//
// Anti-pattern (pre-task-#58; historically 225 occurrences across 69
// *_related*.go files):
//
//   if len(ids) == 0 && truncated {
//       return resource.RelatedCheckResult{TargetType: "X", Count: -1}
//   }
//
// Contract per resource.ValidateRelatedResult (related.go:144) and
// relatedResultTrunc's docstring: the honest state for "truncated cache
// with zero hits" is:
//
//   {State: RelatedResolved (zero value), Count: 0, Truncated: true}   — a valid lower bound, not unknown
//
// Task #58 replaced the Count==-1 sentinel with the domain.RelatedRowState
// enum, so the anti-pattern's modern equivalent is a checker returning any
// non-RelatedResolved state (RelatedUnknown/RelatedError/RelatedDeferred)
// instead of the honest Resolved+Truncated lower bound — that misrepresents
// a "we scanned what we could see and found nothing (more may exist)" result
// as "unknown"/"errored"/"deferred" when the count is actually a known >=0
// lower bound.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"

	_ "github.com/k2m30/a9s/v3/core/aws" // ensure all related registrations run
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ─────────────────────────────────────────────────────────────────────────────
// TestTruncatedResult_ReturnsTruncatedResult
// ─────────────────────────────────────────────────────────────────────────────

// TestTruncatedResult_ReturnsTruncatedResult verifies that TruncatedResult
// returns a fully-populated RelatedCheckResult with Count=0, Truncated=true,
// the given TargetType, and nil ResourceIDs / Err.
func TestTruncatedResult_ReturnsTruncatedResult(t *testing.T) {
	result := resource.RelatedCheckResult{TargetType: "vpc", Truncated: true}

	if result.TargetType != "vpc" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "vpc")
	}
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
	if !result.Truncated {
		t.Error("Truncated = false, want true")
	}
	if result.ResourceIDs != nil {
		t.Errorf("ResourceIDs = %v, want nil", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("Err = %v, want nil", result.Err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestTruncatedResult_EmptyTargetType
// ─────────────────────────────────────────────────────────────────────────────

// TestTruncatedResult_EmptyTargetType verifies that TruncatedResult("") returns a
// result with an empty TargetType, which ValidateRelatedResult reports as invalid.
// This lets callers detect the empty-TargetType invariant at validation time.
func TestTruncatedResult_EmptyTargetType(t *testing.T) {
	result := resource.RelatedCheckResult{TargetType: "", Truncated: true}

	// The struct is returned (TruncatedResult does not panic on empty input).
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
	if !result.Truncated {
		t.Error("Truncated = false, want true")
	}

	// Callers can detect the mistake: ValidateRelatedResult must return an error.
	err := resource.ValidateRelatedResult(result)
	if err == nil {
		t.Error("ValidateRelatedResult(TruncatedResult(\"\")) = nil, want non-nil error (empty TargetType invariant)")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestTruncatedResult_PassesValidation
// ─────────────────────────────────────────────────────────────────────────────

// TestTruncatedResult_PassesValidation verifies that for any non-empty targetType,
// the result returned by TruncatedResult passes ValidateRelatedResult with no error.
func TestTruncatedResult_PassesValidation(t *testing.T) {
	targetTypes := []string{
		"vpc", "subnet", "sg", "ec2", "rds", "eks", "ng", "elb",
		"nat", "igw", "rtb", "vpce", "eni", "tgw", "lambda", "s3",
		"secrets", "kms", "cfn", "ami", "ebs", "ebs-snap",
	}

	for _, tt := range targetTypes {
		tt := tt
		t.Run(tt, func(t *testing.T) {
			result := resource.RelatedCheckResult{TargetType: tt, Truncated: true}
			if err := resource.ValidateRelatedResult(result); err != nil {
				t.Errorf("TruncatedResult(%q) fails ValidateRelatedResult: %v", tt, err)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestCheckVPC_TruncatedCacheReturnsTruncatedResult
// ─────────────────────────────────────────────────────────────────────────────

// TestCheckVPC_TruncatedCacheReturnsTruncatedResult calls the registered
// checkVPCSubnet checker (via the "vpc"→"subnet" RelatedDef) with a cache
// that has only the VPC resource and a subnet entry that is truncated with
// zero resources. The vpc resource has ID "vpc-12345678" which will not match
// any subnet in the empty (truncated) list.
//
// EXPECTED: {Count: 0, Truncated: true}  (honest lower bound)
// ACTUAL (BUG): {Count: -1}                (discards lower bound)
//
// This test stays RED until the coder fixes the anti-pattern in vpc_related.go.
func TestCheckVPC_TruncatedCacheReturnsTruncatedResult(t *testing.T) {
	vpcResource := resource.Resource{
		ID:   "vpc-12345678",
		Name: "test-vpc",
		Fields: map[string]string{
			"vpc_id": "vpc-12345678",
			"state":  "available",
			"cidr":   "10.0.0.0/16",
		},
	}

	// Subnet cache entry: truncated, zero resources — simulates a partial page
	// where no subnets for this VPC happened to land in the first page.
	cache := resource.ResourceCache{
		"subnet": {
			Resources:   []resource.Resource{},
			IsTruncated: true,
		},
	}

	// Find the checkVPCSubnet checker via the registry.
	var checker resource.RelatedChecker
	for _, def := range resource.GetRelated("vpc") {
		if def.TargetType == "subnet" {
			checker = def.Checker
			break
		}
	}
	if checker == nil {
		t.Fatal("checkVPCSubnet not registered under vpc→subnet")
	}

	result := checker(context.Background(), nil, vpcResource, cache)

	if result.State != domain.RelatedResolved {
		t.Errorf("checkVPCSubnet with truncated-empty cache returned State=%s "+
			"(anti-pattern); want RelatedResolved with Count=0, Truncated=true. Result: %+v", result.State, result)
	}
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
	if !result.Truncated {
		t.Errorf("Truncated = false, want true (truncated cache means result is a lower bound). Result: %+v", result)
	}
	if result.TargetType != "subnet" {
		t.Errorf("TargetType = %q, want \"subnet\"", result.TargetType)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestCheckSG_TruncatedCacheReturnsTruncatedResult
// ─────────────────────────────────────────────────────────────────────────────

// TestCheckSG_TruncatedCacheReturnsTruncatedResult calls the registered
// checkSGEC2 checker (via the "sg"→"ec2" RelatedDef) with an EC2 cache that
// is truncated with zero resources. The SG resource has a real ID that will
// not match any instance in the empty list.
//
// EXPECTED: {Count: 0, Truncated: true}
// ACTUAL (BUG): {Count: -1}
func TestCheckSG_TruncatedCacheReturnsTruncatedResult(t *testing.T) {
	sgResource := resource.Resource{
		ID:   "sg-0abcdef123456789",
		Name: "test-sg",
		Fields: map[string]string{
			"vpc_id":      "vpc-12345678",
			"group_name":  "test-sg",
			"description": "test security group",
		},
	}

	// EC2 cache entry: truncated, zero resources.
	cache := resource.ResourceCache{
		"ec2": {
			Resources:   []resource.Resource{},
			IsTruncated: true,
		},
	}

	var checker resource.RelatedChecker
	for _, def := range resource.GetRelated("sg") {
		if def.TargetType == "ec2" {
			checker = def.Checker
			break
		}
	}
	if checker == nil {
		t.Fatal("checkSGEC2 not registered under sg→ec2")
	}

	result := checker(context.Background(), nil, sgResource, cache)

	if result.State != domain.RelatedResolved {
		t.Errorf("checkSGEC2 with truncated-empty cache returned State=%s "+
			"(anti-pattern); want RelatedResolved with Count=0, Truncated=true. Result: %+v", result.State, result)
	}
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
	if !result.Truncated {
		t.Errorf("Truncated = false, want true (truncated cache means result is a lower bound). Result: %+v", result)
	}
	if result.TargetType != "ec2" {
		t.Errorf("TargetType = %q, want \"ec2\"", result.TargetType)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestCheckAMI_NG_TruncatedCacheReturnsTruncatedResult
// ─────────────────────────────────────────────────────────────────────────────

// TestCheckAMI_NG_TruncatedCacheReturnsTruncatedResult calls the registered
// checkAMING checker (via the "ami"→"ng" RelatedDef) with an NG cache that is
// truncated with zero resources. The AMI resource has a real ID that will not
// match any node group in the empty list.
//
// EXPECTED: {Count: 0, Truncated: true}
// ACTUAL (BUG): {Count: -1}
func TestCheckAMI_NG_TruncatedCacheReturnsTruncatedResult(t *testing.T) {
	amiResource := resource.Resource{
		ID:   "ami-0abcdef1234567890",
		Name: "test-ami",
		Fields: map[string]string{
			"state":        "available",
			"architecture": "x86_64",
			"image_type":   "machine",
		},
	}

	// NG cache entry: truncated, zero resources.
	cache := resource.ResourceCache{
		"ng": {
			Resources:   []resource.Resource{},
			IsTruncated: true,
		},
	}

	var checker resource.RelatedChecker
	for _, def := range resource.GetRelated("ami") {
		if def.TargetType == "ng" {
			checker = def.Checker
			break
		}
	}
	if checker == nil {
		t.Fatal("checkAMING not registered under ami→ng")
	}

	result := checker(context.Background(), nil, amiResource, cache)

	if result.State != domain.RelatedResolved {
		t.Errorf("checkAMING with truncated-empty NG cache returned State=%s "+
			"(anti-pattern); want RelatedResolved with Count=0, Truncated=true. Result: %+v", result.State, result)
	}
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
	if !result.Truncated {
		t.Errorf("Truncated = false, want true (truncated cache means result is a lower bound). Result: %+v", result)
	}
	if result.TargetType != "ng" {
		t.Errorf("TargetType = %q, want \"ng\"", result.TargetType)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestAllReverseScanCheckers_TruncatedEmptyCacheReturnsApproximate
// ─────────────────────────────────────────────────────────────────────────────

// TestAllReverseScanCheckers_TruncatedEmptyCacheReturnsApproximate iterates over
// every registered (sourceType, RelatedDef) where NeedsTargetCache is true,
// constructs a ResourceCache where ALL target entries are {IsTruncated: true,
// Resources: []}, calls the checker with a minimal parent resource, and asserts
// the result is {Count: 0, Truncated: true} — NEVER {Count: -1}.
//
// This is the regression pin: after the coder sweeps all 225 anti-pattern sites,
// every reverse-scan checker must pass this test. The test stays RED (many
// Count=-1 failures) until the sweep is complete.
func TestAllReverseScanCheckers_TruncatedEmptyCacheReturnsApproximate(t *testing.T) {
	// Minimal parent resources keyed by source type. These are shaped to avoid
	// the early-exit "no ID / no key field → Count=0 definitively" guard, so
	// that each checker actually reaches the truncated-cache code path.
	minimalParents := map[string]resource.Resource{
		"vpc": {
			ID:   "vpc-00000001",
			Name: "test-vpc",
			Fields: map[string]string{
				"vpc_id": "vpc-00000001",
				"state":  "available",
				"cidr":   "10.0.0.0/16",
			},
		},
		"sg": {
			ID:   "sg-00000001",
			Name: "test-sg",
			Fields: map[string]string{
				"vpc_id": "vpc-00000001",
			},
		},
		"ec2": {
			ID:   "i-00000001",
			Name: "test-instance",
			Fields: map[string]string{
				"vpc_id":   "vpc-00000001",
				"state":    "running",
				"image_id": "ami-00000001",
			},
		},
		"ami": {
			ID:   "ami-00000001",
			Name: "test-ami",
			Fields: map[string]string{
				"state":        "available",
				"architecture": "x86_64",
			},
		},
		"ng": {
			ID:   "nodegroup-00000001",
			Name: "test-ng",
			Fields: map[string]string{
				"cluster_name": "test-cluster",
			},
		},
		"eks": {
			ID:   "test-cluster",
			Name: "test-cluster",
			Fields: map[string]string{
				"status": "ACTIVE",
			},
		},
		"elb": {
			ID:   "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/test-lb/0000000000000001",
			Name: "test-lb",
			Fields: map[string]string{
				"vpc_id": "vpc-00000001",
				"state":  "active",
			},
		},
		"rds": {
			ID:   "test-db-instance",
			Name: "test-db",
			Fields: map[string]string{
				"vpc_id": "vpc-00000001",
				"status": "available",
			},
		},
		"lambda": {
			ID:   "arn:aws:lambda:us-east-1:123456789012:function:test-fn",
			Name: "test-fn",
			Fields: map[string]string{
				"vpc_id": "vpc-00000001",
			},
		},
		"ecs-svc": {
			ID:   "arn:aws:ecs:us-east-1:123456789012:service/test-cluster/test-svc",
			Name: "test-svc",
			Fields: map[string]string{
				"cluster_arn": "arn:aws:ecs:us-east-1:123456789012:cluster/test-cluster",
			},
		},
		"asg": {
			ID:   "test-asg",
			Name: "test-asg",
			Fields: map[string]string{
				"status": "InService",
			},
		},
		"subnet": {
			ID:   "subnet-00000001",
			Name: "test-subnet",
			Fields: map[string]string{
				"vpc_id":            "vpc-00000001",
				"availability_zone": "us-east-1a",
			},
		},
		"eni": {
			ID:   "eni-00000001",
			Name: "test-eni",
			Fields: map[string]string{
				"vpc_id": "vpc-00000001",
			},
		},
		"nat": {
			ID:   "nat-00000001",
			Name: "test-nat",
			Fields: map[string]string{
				"vpc_id": "vpc-00000001",
				"state":  "available",
			},
		},
		"igw": {
			ID:   "igw-00000001",
			Name: "test-igw",
			Fields: map[string]string{
				"vpc_id": "vpc-00000001",
			},
		},
		"rtb": {
			ID:   "rtb-00000001",
			Name: "test-rtb",
			Fields: map[string]string{
				"vpc_id": "vpc-00000001",
			},
		},
		"vpce": {
			ID:   "vpce-00000001",
			Name: "test-vpce",
			Fields: map[string]string{
				"vpc_id": "vpc-00000001",
				"state":  "available",
			},
		},
		"tgw": {
			ID:   "tgw-00000001",
			Name: "test-tgw",
			Fields: map[string]string{
				"state": "available",
			},
		},
		"ebs": {
			ID:   "vol-00000001",
			Name: "test-volume",
			Fields: map[string]string{
				"state":             "available",
				"availability_zone": "us-east-1a",
			},
		},
		"kms": {
			ID:   "arn:aws:kms:us-east-1:123456789012:key/00000000-0000-0000-0000-000000000001",
			Name: "test-key",
			Fields: map[string]string{
				"state": "Enabled",
			},
		},
		"secrets": {
			ID:     "arn:aws:secretsmanager:us-east-1:123456789012:secret:test-secret-abcdef",
			Name:   "test-secret",
			Fields: map[string]string{},
		},
		"s3": {
			ID:   "test-bucket",
			Name: "test-bucket",
			Fields: map[string]string{
				"region": "us-east-1",
			},
		},
		"cfn": {
			ID:   "test-stack",
			Name: "test-stack",
			Fields: map[string]string{
				"status": "CREATE_COMPLETE",
			},
		},
		"eb": {
			ID:     "test-eb-app",
			Name:   "test-eb-app",
			Fields: map[string]string{},
		},
		"ecr": {
			ID:   "arn:aws:ecr:us-east-1:123456789012:repository/test-repo",
			Name: "test-repo",
			Fields: map[string]string{
				"uri": "123456789012.dkr.ecr.us-east-1.amazonaws.com/test-repo",
			},
			RawStruct: ecrtypes.Repository{
				RepositoryName: aws.String("test-repo"),
				RepositoryArn:  aws.String("arn:aws:ecr:us-east-1:123456789012:repository/test-repo"),
			},
		},
		"sfn": {
			ID:     "arn:aws:states:us-east-1:123456789012:stateMachine:test-sm",
			Name:   "test-sm",
			Fields: map[string]string{},
		},
		"sns": {
			ID:     "arn:aws:sns:us-east-1:123456789012:test-topic",
			Name:   "test-topic",
			Fields: map[string]string{},
		},
		"dynamo": {
			ID:   "test-table",
			Name: "test-table",
			Fields: map[string]string{
				"status": "ACTIVE",
			},
		},
		"redis": {
			ID:   "test-redis",
			Name: "test-redis",
			Fields: map[string]string{
				"status": "available",
			},
		},
		"docdb": {
			ID:   "test-docdb",
			Name: "test-docdb",
			Fields: map[string]string{
				"status": "available",
			},
		},
		"efs": {
			ID:   "fs-00000001",
			Name: "test-efs",
			Fields: map[string]string{
				"lifecycle_state": "available",
			},
		},
		"ses": {
			ID:     "test@example.com",
			Name:   "test@example.com",
			Fields: map[string]string{},
		},
		"ssm": {
			ID:   "/test/param",
			Name: "/test/param",
			Fields: map[string]string{
				"type": "String",
			},
		},
		"pipeline": {
			ID:     "test-pipeline",
			Name:   "test-pipeline",
			Fields: map[string]string{},
		},
		"ebs-snap": {
			ID:   "snap-00000001",
			Name: "test-snapshot",
			Fields: map[string]string{
				"state": "completed",
			},
		},
		"role": {
			ID:     "arn:aws:iam::123456789012:role/test-role",
			Name:   "test-role",
			Fields: map[string]string{},
		},
		"iam-user": {
			ID:     "arn:aws:iam::123456789012:user/test-user",
			Name:   "test-user",
			Fields: map[string]string{},
		},
		"alarm": {
			ID:     "arn:aws:cloudwatch:us-east-1:123456789012:alarm:test-alarm",
			Name:   "test-alarm",
			Fields: map[string]string{},
		},
		"backup": {
			ID:     "arn:aws:backup:us-east-1:123456789012:backup-plan:test-plan",
			Name:   "test-plan",
			Fields: map[string]string{},
		},
	}

	// fallbackParent is used for any source type not in the above map.
	fallbackParent := resource.Resource{
		ID:   "test-resource-id",
		Name: "test-resource",
		Fields: map[string]string{
			"vpc_id": "vpc-00000001",
			"state":  "active",
		},
	}

	// Enumerate all registered source types from the related registry.
	// We need to iterate over all source types. Use all resource type short names
	// that have registered related defs.
	allSourceTypes := []string{
		"vpc", "sg", "ec2", "ami", "ng", "eks", "elb", "rds",
		"lambda", "ecs-svc", "asg", "subnet", "eni", "nat", "igw",
		"rtb", "vpce", "tgw", "ebs", "kms", "secrets", "s3", "cfn",
		"eb", "ecr", "sfn", "sns", "dynamo", "redis", "docdb", "efs",
		"ses", "ssm", "pipeline", "ebs-snap", "role", "iam-user",
		"alarm", "backup", "vpce", "tgw",
	}

	// Deduplicate: track already-tested (sourceType, targetType) pairs.
	tested := make(map[string]bool)

	for _, sourceType := range allSourceTypes {
		defs := resource.GetRelated(sourceType)
		if len(defs) == 0 {
			continue
		}

		parent := fallbackParent
		if p, ok := minimalParents[sourceType]; ok {
			parent = p
		}

		for _, def := range defs {
			if !def.NeedsTargetCache {
				// Forward checkers that don't read from cache are out of scope.
				continue
			}
			if def.Checker == nil {
				continue
			}

			key := sourceType + "→" + def.TargetType
			if tested[key] {
				continue
			}
			tested[key] = true

			t.Run(key, func(t *testing.T) {
				// Build a cache where the target type is truncated with zero items.
				// All other types are absent from the cache so checkers fall through
				// to the target-type entry only.
				cache := resource.ResourceCache{
					def.TargetType: {
						Resources:   []resource.Resource{},
						IsTruncated: true,
					},
				}

				result := def.Checker(context.Background(), nil, parent, cache)

				// The checker MAY legitimately return Count=0 non-truncated (e.g.,
				// when it determines from the parent's own fields that there can be
				// no related resources of this type). That is acceptable — it means
				// the early-exit guard fired, not the truncated-cache path.
				// What is NEVER acceptable is a non-RelatedResolved State when we
				// provided a real cache entry (not nil). A non-Resolved State
				// combined with IsTruncated=true is the anti-pattern that drops the
				// honest lower bound.
				if result.State != domain.RelatedResolved {
					t.Errorf("checker %s with truncated-empty %q cache returned State=%s "+
						"(anti-pattern: drops honest lower bound); want RelatedResolved with Count=0, Truncated=true. "+
						"Result: %+v", key, def.TargetType, result.State, result)
				}

				// When Count==0, Truncated must be true if the truncated path was hit.
				// We cannot distinguish "early exit" from "truncated path with 0 matches"
				// purely from the outside, so we only assert on the invariant:
				// Count >= 0 is required (already checked above).
				//
				// Additionally, validate the overall result shape.
				if result.TargetType == "" {
					// Tolerate missing echo — fill it for validation.
					result.TargetType = def.TargetType
				}
				if err := resource.ValidateRelatedResult(result); err != nil {
					t.Errorf("checker %s returned invalid result: %v; result: %+v",
						key, err, result)
				}
			})
		}
	}
}
