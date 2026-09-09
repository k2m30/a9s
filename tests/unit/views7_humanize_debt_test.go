// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// views7_humanize_debt_test.go — the sixteen detail fields that still render an
// AWS constant.
//
// Each one is a fact an operator reads off a detail screen in the vocabulary
// of an SDK enum: ENCRYPT_DECRYPT, GreaterThanOrEqualToThreshold, INELIGIBLE.
// They were recorded rather than fixed when the sweep that finds them was
// written, and this file is the worklist turned into pins: one per field, on a
// named demo row, through the real detail build.
//
// The wording each one must read is the wording a field declared on its type
// (ResourceTypeDef.HumanizeFields) renders: the constant split into words and
// lowercased. It is pinned literally rather than computed, so a change to the
// conversion that quietly reworded sixteen screens is a failure here.
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
)

// views7RawEnumWitness is one demo detail row that must read as words, with
// the constant it shows today.
type views7RawEnumWitness struct {
	shortName  string
	resourceID string
	path       string
	raw        string
	want       string
}

// views7HumanizeDebtWitnesses covers every entry of rawEnumDetailDebt, and
// covers a field with more than one constant on the bench more than once: a
// single witness cannot tell a conversion from a hardcoded string.
var views7HumanizeDebtWitnesses = []views7RawEnumWitness{
	{"acm", "arn:aws:acm:us-east-1:123456789012:certificate/a1b2c3d4-5678-90ab-cdef-111111111111", "RenewalEligibility", "ELIGIBLE", "eligible"},
	{"acm", "arn:aws:acm:us-east-1:123456789012:certificate/a7b8c9d0-1234-56ab-cdef-777777777777", "RenewalEligibility", "INELIGIBLE", "ineligible"},

	{"alarm", "api-high-error-rate", "ComparisonOperator", "GreaterThanThreshold", "greater than threshold"},
	{"alarm", "aurora-prod-cluster-cpu", "ComparisonOperator", "GreaterThanOrEqualToThreshold", "greater than or equal to threshold"},
	{"alarm", "prod-efs-burst-credit-low", "ComparisonOperator", "LessThanThreshold", "less than threshold"},
	{"alarm", "redis-prod-cache-hits", "ComparisonOperator", "LessThanOrEqualToThreshold", "less than or equal to threshold"},

	{"asg", "acme-web-prod-asg", "HealthCheckType", "ELB", "elb"},
	{"asg", "acme-staging-asg", "HealthCheckType", "EC2", "ec2"},

	{"ddb", "orders-prod", "TableStatus", "ACTIVE", "active"},
	{"ddb", "analytics-deleting", "TableStatus", "DELETING", "deleting"},
	{"ddb", "legacy-kms-lost", "TableStatus", "INACCESSIBLE_ENCRYPTION_CREDENTIALS", "inaccessible encryption credentials"},

	{"ecs-svc", "api-gateway", "SchedulingStrategy", "REPLICA", "replica"},
	{"ecs-svc", "log-aggregator", "SchedulingStrategy", "DAEMON", "daemon"},

	{"ecs-task", "a1b2c3d4e5f6a1b2c3d4e5f6", "Connectivity", "CONNECTED", "connected"},

	{"kms", "acme-prod-master-key", "KeyManager", "CUSTOMER", "customer"},
	{"kms", "a1b2c3d4-5678-90ab-cdef-111111111111", "KeySpec", "SYMMETRIC_DEFAULT", "symmetric default"},
	{"kms", "deadbeef-0000-0000-0000-000000000000", "KeyState", "PendingDeletion", "pending deletion"},
	{"kms", "acme-prod-master-key", "KeyUsage", "ENCRYPT_DECRYPT", "encrypt decrypt"},
	{"kms", "acme-prod-master-key", "Origin", "AWS_KMS", "aws kms"},

	{"logs", "/aws/lambda/api-gateway-authorizer", "DataProtectionStatus", "ACTIVATED", "activated"},
	{"logs", "/aws/lambda/api-gateway-authorizer", "LogGroupClass", "STANDARD", "standard"},

	{"mwaa", "prod-airflow-etl", "Status", "AVAILABLE", "available"},

	{"pipeline", "acme-api-deploy", "ExecutionMode", "QUEUED", "queued"},

	{"tg", "acme-grpc-tg", "ProtocolVersion", "GRPC", "grpc"},
}

// views7UntouchedWitnesses are the rows the same fields must leave exactly as
// they are: a value that already reads as words is not reworded a second time,
// and a fact the resource does not carry stays the not-applicable dash rather
// than becoming a word that looks like an answer.
var views7UntouchedWitnesses = []views7RawEnumWitness{
	// Task phrase7 row 2 split this phrase: "deleting" was declared at two
	// severities across the catalog (Dim here, Warn on twelve other types),
	// so the same word carried two colours. The pin still proves the
	// humanizer leaves a declared value alone; it is the declaration that
	// moved, not the rendering.
	{"mwaa", "dim-airflow-deleting", "Status", "", "deleting — environment teardown"},
	{"mwaa", "warn-airflow-maintenance", "Status", "", "maintenance in progress"},
	{"pipeline", "acme-frontend-deploy", "ExecutionMode", "", "-"},
	{"tg", "acme-web-tg", "ProtocolVersion", "", "-"},
}

// views7DetailValue renders one demo row's detail and returns the value at path.
func views7DetailValue(t *testing.T, w views7RawEnumWitness) string {
	t.Helper()
	clients := demo.NewServiceClients()
	byType, cache := buildVisibilityTypeCache(t)

	td := resource.FindResourceType(w.shortName)
	if td == nil {
		t.Fatalf("no resource type %q", w.shortName)
	}
	for _, r := range mergeWave2Findings(t, *td, byType[w.shortName], cache, clients) {
		if r.ID != w.resourceID {
			continue
		}
		got, ok := aws6DetailValueAtPath(t, r, w.shortName, w.path)
		if !ok {
			t.Fatalf("%s %s: the detail has no row at path %q", w.shortName, w.resourceID, w.path)
		}
		return got
	}
	t.Fatalf("no demo %s row %q — the witness this field is pinned on is gone", w.shortName, w.resourceID)
	return ""
}

// TestDemoDetailReadsWordsForEveryDebtField pins each recorded field on the
// demo bench.
func TestDemoDetailReadsWordsForEveryDebtField(t *testing.T) {
	for _, w := range views7HumanizeDebtWitnesses {
		t.Run(w.shortName+"/"+w.path+"/"+w.raw, func(t *testing.T) {
			got := views7DetailValue(t, w)
			if got == w.raw {
				t.Errorf("%s %s: %s still shows the SDK constant %q; want %q",
					w.shortName, w.resourceID, w.path, got, w.want)
				return
			}
			if got != w.want {
				t.Errorf("%s %s: %s = %q, want %q", w.shortName, w.resourceID, w.path, got, w.want)
			}
		})
	}
}

// TestHumanizingLeavesTheseRowsAlone is the other half: the same declaration
// reaches every row of the field, including the ones that were never a
// constant, and none of them may change.
func TestHumanizingLeavesTheseRowsAlone(t *testing.T) {
	for _, w := range views7UntouchedWitnesses {
		t.Run(w.shortName+"/"+w.resourceID+"/"+w.path, func(t *testing.T) {
			if got := views7DetailValue(t, w); got != w.want {
				t.Errorf("%s %s: %s = %q, want %q unchanged", w.shortName, w.resourceID, w.path, got, w.want)
			}
		})
	}
}

// TestRawEnumDebtIsPaidOff closes the list itself. The sixteen fields above
// are the whole of rawEnumDetailDebt, so fixing them empties it — and the
// ceiling comes down with it, or the room they freed is spendable on a new
// raw constant.
func TestRawEnumDebtIsPaidOff(t *testing.T) {
	if len(rawEnumDetailDebt) != 0 {
		t.Errorf("rawEnumDetailDebt still holds %d fields: %v", len(rawEnumDetailDebt), rawEnumDetailDebt)
	}
	if rawEnumDetailDebtCeiling != 0 {
		t.Errorf("rawEnumDetailDebtCeiling still reads %d — lower it to 0 in the change that empties the "+
			"list, so the room the fixed fields freed cannot be spent on a new raw constant", rawEnumDetailDebtCeiling)
	}
}
