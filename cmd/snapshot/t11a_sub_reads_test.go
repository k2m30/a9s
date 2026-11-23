package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
)

const t11aIAMDenial = "AccessDenied"

var t11aIAMTime = time.Date(2026, 10, 2, 7, 15, 0, 0, time.UTC)

// t11aIAMAnswer answers an IAM account with one user (console password, one
// access key, one MFA device), one group with one member, one role used
// recently and one customer-managed policy. subject is the operation under
// test: it fails with AccessDenied when fail is set, and otherwise answers
// with an empty, successful answer.
func t11aIAMAnswer(subject string, fail bool) answerFunc {
	return func(service, op string, in any) (any, error) {
		if op == subject {
			if fail {
				return nil, t11aAPIError(t11aIAMDenial, "User: arn:aws:iam::123456789012:user/example-readonly is not authorized to perform: iam:"+op)
			}
			switch op {
			case "ListAccessKeys":
				return &iam.ListAccessKeysOutput{}, nil
			case "ListMFADevices":
				return &iam.ListMFADevicesOutput{}, nil
			case "GetAccessKeyLastUsed":
				return &iam.GetAccessKeyLastUsedOutput{UserName: aws.String("alice"), AccessKeyLastUsed: &iamtypes.AccessKeyLastUsed{ServiceName: aws.String("N/A"), Region: aws.String("N/A")}}, nil
			case "GetGroup":
				return &iam.GetGroupOutput{Group: &iamtypes.Group{GroupName: aws.String("deployers"), Arn: aws.String("arn:aws:iam::123456789012:group/deployers")}}, nil
			case "GetRole":
				return &iam.GetRoleOutput{Role: &iamtypes.Role{RoleName: aws.String("ci-runner"), Arn: aws.String("arn:aws:iam::123456789012:role/ci-runner"), RoleLastUsed: &iamtypes.RoleLastUsed{}}}, nil
			case "GetPolicyVersion":
				return &iam.GetPolicyVersionOutput{}, nil
			}
		}
		switch op {
		case "ListUsers":
			return &iam.ListUsersOutput{Users: []iamtypes.User{{UserName: aws.String("alice"), Arn: aws.String("arn:aws:iam::123456789012:user/alice"), CreateDate: aws.Time(t11aIAMTime)}}}, nil
		case "GetLoginProfile":
			return &iam.GetLoginProfileOutput{LoginProfile: &iamtypes.LoginProfile{UserName: aws.String("alice"), CreateDate: aws.Time(t11aIAMTime)}}, nil
		case "ListAccessKeys":
			return &iam.ListAccessKeysOutput{AccessKeyMetadata: []iamtypes.AccessKeyMetadata{{UserName: aws.String("alice"), AccessKeyId: aws.String("AKIAIOSFODNN7EXAMPLE"), Status: iamtypes.StatusTypeActive, CreateDate: aws.Time(t11aIAMTime)}}}, nil
		case "GetAccessKeyLastUsed":
			return &iam.GetAccessKeyLastUsedOutput{UserName: aws.String("alice"), AccessKeyLastUsed: &iamtypes.AccessKeyLastUsed{LastUsedDate: aws.Time(t11aIAMTime), ServiceName: aws.String("s3"), Region: aws.String("eu-west-1")}}, nil
		case "ListMFADevices":
			return &iam.ListMFADevicesOutput{MFADevices: []iamtypes.MFADevice{{UserName: aws.String("alice"), SerialNumber: aws.String("arn:aws:iam::123456789012:mfa/alice"), EnableDate: aws.Time(t11aIAMTime)}}}, nil
		case "ListGroups":
			return &iam.ListGroupsOutput{Groups: []iamtypes.Group{{GroupName: aws.String("deployers"), Arn: aws.String("arn:aws:iam::123456789012:group/deployers"), CreateDate: aws.Time(t11aIAMTime)}}}, nil
		case "GetGroup":
			return &iam.GetGroupOutput{Group: &iamtypes.Group{GroupName: aws.String("deployers"), Arn: aws.String("arn:aws:iam::123456789012:group/deployers")},
				Users: []iamtypes.User{{UserName: aws.String("alice"), Arn: aws.String("arn:aws:iam::123456789012:user/alice")}}}, nil
		case "ListRoles":
			return &iam.ListRolesOutput{Roles: []iamtypes.Role{{RoleName: aws.String("ci-runner"), Arn: aws.String("arn:aws:iam::123456789012:role/ci-runner"), CreateDate: aws.Time(t11aIAMTime)}}}, nil
		case "GetRole":
			return &iam.GetRoleOutput{Role: &iamtypes.Role{RoleName: aws.String("ci-runner"), Arn: aws.String("arn:aws:iam::123456789012:role/ci-runner"),
				RoleLastUsed: &iamtypes.RoleLastUsed{LastUsedDate: aws.Time(t11aIAMTime), Region: aws.String("eu-west-1")}}}, nil
		case "ListPolicies":
			return &iam.ListPoliciesOutput{Policies: []iamtypes.Policy{{PolicyName: aws.String("deploy"), Arn: aws.String("arn:aws:iam::123456789012:policy/deploy"),
				DefaultVersionId: aws.String("v3"), AttachmentCount: aws.Int32(1)}}}, nil
		case "GetPolicyVersion":
			return &iam.GetPolicyVersionOutput{PolicyVersion: &iamtypes.PolicyVersion{VersionId: aws.String("v3"), IsDefaultVersion: true,
				Document: aws.String("%7B%22Version%22%3A%222012-10-17%22%2C%22Statement%22%3A%5B%5D%7D")}}, nil
		}
		return t11aDefault(service, in)
	}
}

// A sub-read that AWS refused says nothing about the item: the app marks the
// row not inspected (it reads these through PageAll and records the
// failure). The oracle therefore records a refused sub-read differently from
// an answer that is empty (no keys, no MFA device, no member, never used, no
// document), and names the code AWS returned; a refusal written as an empty
// answer is the "console user without MFA" or "orphan group" signal the app
// never raises for it.
func TestT11a_ARefusedSubReadIsRecordedAsAFailureNotAsEmpty(t *testing.T) {
	cases := []struct {
		typ     string
		capture captureFunc
		subject string
	}{
		{"iam-user", captureIAMUser, "ListAccessKeys"},
		{"iam-user", captureIAMUser, "GetAccessKeyLastUsed"},
		{"iam-user", captureIAMUser, "ListMFADevices"},
		{"iam-group", captureIAMGroup, "GetGroup"},
		{"role", captureRole, "GetRole"},
		{"policy", capturePolicy, "GetPolicyVersion"},
	}
	for _, c := range cases {
		t.Run(c.typ+"/"+c.subject, func(t *testing.T) {
			snap := func(fail bool) string {
				data, err := c.capture(context.Background(), answeringConfig(t11aIAMAnswer(c.subject, fail)))
				if err != nil {
					t.Fatalf("capture (refused=%v) failed as a whole: %v", fail, err)
				}
				b, err := json.Marshal(data)
				if err != nil {
					t.Fatal(err)
				}
				return string(b)
			}
			empty, refused := snap(false), snap(true)
			if empty == refused {
				t.Errorf("a refused %s is recorded exactly as an empty answer:\n  %s", c.subject, refused)
			}
			if !strings.Contains(refused, t11aIAMDenial) {
				t.Errorf("a refused %s does not record the code AWS returned (%s):\n  %s", c.subject, t11aIAMDenial, refused)
			}
		})
	}
}

// GetGroup answers at most MaxItems users per page and sets IsTruncated with
// a Marker while more remain; the app walks every page and shows the total in
// the Members column, so the oracle counts the members of every page.
func TestT11a_CaptureIAMGroup_CountsTheMembersOfEveryPage(t *testing.T) {
	users := func(prefix string, n int) []iamtypes.User {
		out := make([]iamtypes.User, n)
		for i := range out {
			name := fmt.Sprintf("%s-user%03d", prefix, i)
			out[i] = iamtypes.User{UserName: aws.String(name), Arn: aws.String("arn:aws:iam::123456789012:user/" + name)}
		}
		return out
	}
	cfg := answeringConfig(func(service, op string, in any) (any, error) {
		switch op {
		case "ListGroups":
			return &iam.ListGroupsOutput{Groups: []iamtypes.Group{{GroupName: aws.String("engineering"), Arn: aws.String("arn:aws:iam::123456789012:group/engineering")}}}, nil
		case "GetGroup":
			g := &iamtypes.Group{GroupName: aws.String("engineering"), Arn: aws.String("arn:aws:iam::123456789012:group/engineering")}
			if in.(*iam.GetGroupInput).Marker == nil {
				return &iam.GetGroupOutput{Group: g, Users: users("p1", 100), IsTruncated: true, Marker: aws.String("group-page-2")}, nil
			}
			return &iam.GetGroupOutput{Group: g, Users: users("p2", 150)}, nil
		}
		return t11aDefault(service, in)
	})
	data, err := captureIAMGroup(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	groups := data.(iamGroupData).Groups
	if len(groups) != 1 || groups[0].UserCount != 250 {
		t.Errorf("groups = %+v, want engineering with user_count 250", groups)
	}
}
