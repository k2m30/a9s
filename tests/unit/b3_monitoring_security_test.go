// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestAlarmThresholdKeepsItsPrecision pins that a sub-percent threshold
// reaches the Threshold column as the number CloudWatch reports, not as a
// two-decimal rounding of it.
func TestAlarmThresholdKeepsItsPrecision(t *testing.T) {
	mock := &fakeCloudWatchDescribeAlarms{
		Output: &cloudwatch.DescribeAlarmsOutput{
			MetricAlarms: []cwtypes.MetricAlarm{
				{AlarmName: aws.String("error-rate"), StateValue: cwtypes.StateValueOk, Threshold: aws.Float64(0.005)},
				{AlarmName: aws.String("free-storage"), StateValue: cwtypes.StateValueOk, Threshold: aws.Float64(10_000_000_000)},
			},
		},
	}
	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchCloudWatchAlarmsPage(context.Background(), mock, token)
	})
	if err != nil {
		t.Fatalf("FetchCloudWatchAlarmsPage: %v", err)
	}
	for i, want := range []string{"0.005", "10000000000"} {
		if got := resources[i].Fields["threshold"]; got != want {
			t.Errorf("resources[%d] threshold = %q, want %q", i, got, want)
		}
	}
}

// TestAlarmConfiguredActionsRowCountsEveryTransition pins that the row under
// "actions disabled" counts what is wired behind the switch on all three
// transitions, so an alarm paging on OK does not read as wired to nothing.
func TestAlarmConfiguredActionsRowCountsEveryTransition(t *testing.T) {
	mock := &fakeCloudWatchDescribeAlarms{
		Output: &cloudwatch.DescribeAlarmsOutput{
			MetricAlarms: []cwtypes.MetricAlarm{{
				AlarmName:               aws.String("muted"),
				StateValue:              cwtypes.StateValueOk,
				ActionsEnabled:          aws.Bool(false),
				OKActions:               []string{"arn:aws:sns:us-east-1:123456789012:ok-topic"},
				InsufficientDataActions: []string{"arn:aws:sns:us-east-1:123456789012:insuf-topic"},
			}},
		},
	}
	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchCloudWatchAlarmsPage(context.Background(), mock, token)
	})
	if err != nil {
		t.Fatalf("FetchCloudWatchAlarmsPage: %v", err)
	}
	rows := resources[0].AttentionDetails[awsclient.CodeAlarmActionsDisabled].Rows
	i := slices.IndexFunc(rows, func(r domain.DetailRow) bool { return r.Label == "Configured actions" })
	if i < 0 {
		t.Fatalf("no Configured actions row; got %v", rows)
	}
	if rows[i].Value != "2" {
		t.Errorf("Configured actions = %q, want %q", rows[i].Value, "2")
	}
}

// TestACMListAsksForEveryKeyAlgorithm pins the key-type filter. Unfiltered,
// ListCertificates answers with RSA-1024 and RSA-2048 certificates only, so
// an RSA-4096 or EC certificate would be missing from the list entirely.
func TestACMListAsksForEveryKeyAlgorithm(t *testing.T) {
	fake := &fakeACMListCertificates{Output: &acm.ListCertificatesOutput{}}
	if _, err := awsclient.FetchACMCertificatesPage(context.Background(), fake, ""); err != nil {
		t.Fatalf("FetchACMCertificatesPage: %v", err)
	}
	if fake.LastInput.Includes == nil {
		t.Fatal("ListCertificates was sent with no Includes filter")
	}
	for _, alg := range acmtypes.KeyAlgorithm("").Values() {
		if !slices.Contains(fake.LastInput.Includes.KeyTypes, alg) {
			t.Errorf("Includes.KeyTypes is missing %q", alg)
		}
	}
}

// TestIAMGroupOrphanNeedsThirtyDays pins that a group created this week is
// not reported as an orphan for having no members yet.
func TestIAMGroupOrphanNeedsThirtyDays(t *testing.T) {
	fake := &iamGroupFake{
		usersByGroup:            map[string][]iamtypes.User{"fresh-team": {}},
		attachedPoliciesByGroup: map[string][]iamtypes.AttachedPolicy{"fresh-team": {iamAttachedPolicy("arn:aws:iam::aws:policy/ReadOnlyAccess", "ReadOnlyAccess")}},
		inlinePoliciesByGroup:   map[string][]string{},
	}
	rows := iamGroupResources("fresh-team")
	rows[0].RawStruct = iamtypes.Group{
		GroupName:  aws.String("fresh-team"),
		CreateDate: aws.Time(time.Now().Add(-3 * 24 * time.Hour)),
	}
	res, err := awsclient.EnrichIAMGroup(context.Background(), &awsclient.ServiceClients{IAM: fake}, rows, nil)
	if err != nil {
		t.Fatalf("EnrichIAMGroup: %v", err)
	}
	for code, ad := range res.AttentionDetails["fresh-team"] {
		for _, row := range ad.Rows {
			if row.Label == "Members" {
				t.Errorf("a three-day-old group was reported as an orphan under %s: %q", code, row.Value)
			}
		}
	}
}

// TestIAMRoleDormantNeedsNinetyDaysOfAge pins that a role created today, for
// which IAM has recorded no last-used date yet, is not called dormant.
func TestIAMRoleDormantNeedsNinetyDaysOfAge(t *testing.T) {
	fresh := &iamtypes.Role{
		RoleName:     aws.String("new-deploy-role"),
		Path:         aws.String("/"),
		CreateDate:   aws.Time(time.Now().Add(-2 * time.Hour)),
		RoleLastUsed: nil,
	}
	fake := &iamGetRoleFake{results: map[string]*iamtypes.Role{"new-deploy-role": fresh}}
	rows := []resource.Resource{{
		ID:     "new-deploy-role",
		Name:   "new-deploy-role",
		Fields: map[string]string{"role_name": "new-deploy-role", "path": "/"},
	}}
	res, err := awsclient.EnrichIAMRoleLastUsed(context.Background(),
		&awsclient.ServiceClients{IAM: fake}, rows, nil)
	if err != nil {
		t.Fatalf("EnrichIAMRoleLastUsed: %v", err)
	}
	if fs, ok := res.Findings["new-deploy-role"]; ok {
		t.Errorf("a two-hour-old role was reported dormant: %v", fs)
	}
}

// TestIAMUserMFAIsUncheckedForProgrammaticUsers pins that the MFA column does
// not claim coverage for a user ListMFADevices was never called for.
func TestIAMUserMFAIsUncheckedForProgrammaticUsers(t *testing.T) {
	fake := &iamUserMFAFake{
		loginProfileErr:  map[string]error{"batch-runner": noSuchEntityErr()},
		mfaDevicesByUser: map[string][]iamtypes.MFADevice{},
		accessKeysByUser: map[string][]iamtypes.AccessKeyMetadata{},
	}
	res, err := awsclient.EnrichIAMUserMFA(context.Background(),
		&awsclient.ServiceClients{IAM: fake}, iamUserResources("batch-runner"), nil)
	if err != nil {
		t.Fatalf("EnrichIAMUserMFA: %v", err)
	}
	if got := res.FieldUpdates["batch-runner"]["mfa"]; got != "n/a" {
		t.Errorf("mfa = %q, want %q", got, "n/a")
	}
}
