package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/iampolicy"
)

// A service trust is protected against the confused deputy only by a value
// that names the account, organization or resource the service acts for. A
// value made of wildcards, a wildcard account inside an ARN, a wildcard
// organization and an S3 bucket ARN with a wildcard name match every
// customer. aws:SourceOrgPaths is one of the keys AWS gives for that control
// (IAM User Guide, AWS global condition context keys: "This key provides a
// uniform mechanism for enforcing cross-service confused deputy control").
func TestT3a_Dev_ConfusedDeputyValueNamesTheSource(t *testing.T) {
	cases := []struct {
		name     string
		cond     string
		unscoped bool
	}{
		{"account_all_question_marks", `{"StringLike":{"aws:SourceAccount":"????????????"}}`, true},
		{"account_prefix", `{"StringLike":{"aws:SourceAccount":"1*"}}`, true},
		{"org_wildcard", `{"StringLike":{"aws:SourceOrgID":"o-*"}}`, true},
		{"bucket_wildcard", `{"ArnLike":{"aws:SourceArn":"arn:aws:s3:::*"}}`, true},
		{"arn_account_prefix", `{"ArnLike":{"aws:SourceArn":"arn:aws:lambda:us-east-1:1234*:function:acme-fn"}}`, true},
		{"org_path_wildcard_org", `{"ForAnyValue:StringLike":{"aws:SourceOrgPaths":"o-*/*"}}`, true},
		{"account_fixed", `{"StringEquals":{"aws:SourceAccount":"123456789012"}}`, false},
		{"org_fixed", `{"StringEquals":{"aws:SourceOrgID":"o-acme123456"}}`, false},
		{"bucket_fixed", `{"ArnLike":{"aws:SourceArn":"arn:aws:s3:::acme-bucket"}}`, false},
		{"arn_fixed_account_any_function", `{"ArnLike":{"aws:SourceArn":"arn:aws:lambda:us-east-1:123456789012:function:*"}}`, false},
		{"org_path_ou_subtree", `{"ForAnyValue:StringLike":{"aws:SourceOrgPaths":"o-acme123456/r-ab12/ou-ab12-11111111/*"}}`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doc := `{"Statement":{"Effect":"Allow","Principal":{"Service":"lambda.amazonaws.com"},"Action":"sts:AssumeRole","Condition":` + c.cond + `}}`
			got := mustParse(t, doc).HasServicePrincipalWithoutSourceScope()
			var want []string
			if c.unscoped {
				want = []string{"lambda"}
			}
			if !strSliceEqual(got, want) {
				t.Errorf("HasServicePrincipalWithoutSourceScope() = %v, want %v", got, want)
			}
		})
	}
}

// aws:userid counts toward a non-public grant "outside the pattern
// AROLEID:*" (S3 User Guide, The meaning of "public"), and "AROAXROLE1:*
// represents the unique ID for all sessions associated with the IAM role"
// (IAM User Guide, Disabling permissions for temporary security
// credentials): one role ID followed by ":*" names one role. Any other
// wildcard, including a wildcard role ID or every federated session of an
// account, names no fixed principal. An organization path may end in a "*"
// inside its last segment, which AWS documents "for only the children (and
// any children of those children)" (IAM User Guide, aws:PrincipalOrgPaths).
func TestT3a_Dev_UserIDRoleSessionsAndOrgPathChildren(t *testing.T) {
	cases := []struct {
		name   string
		op     string
		key    string
		value  string
		public bool
	}{
		{"userid_one_role_any_session", "StringLike", "aws:userid", "AROAEXAMPLEID:*", false},
		{"userid_any_role", "StringLike", "aws:userid", "AROA*:*", true},
		{"userid_role_session_prefix", "StringLike", "aws:userid", "AROAEXAMPLEID:acme-*", true},
		{"userid_any_federated_session", "StringLike", "aws:userid", "123456789012:*", true},
		{"userid_user_id_with_suffix", "StringLike", "aws:userid", "AIDAEXAMPLEUSERID:*", true},
		{"userid_two_colons", "StringLike", "aws:userid", "AROAEXAMPLEID:x:*", true},
		{"org_path_children_only", "ForAnyValue:StringLike", "aws:PrincipalOrgPaths", "o-a1b2c3d4e5/r-ab12/ou-ab12-11111111/ou-ab12-22222222/ou-*", false},
		{"org_path_children_wildcard_org", "ForAnyValue:StringLike", "aws:PrincipalOrgPaths", "o-*/r-ab12/ou-*", true},
		{"org_path_slash_only", "ForAnyValue:StringLike", "aws:PrincipalOrgPaths", "/*", true},
		{"org_path_not_an_org", "ForAnyValue:StringLike", "aws:PrincipalOrgPaths", "r-ab12/*", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := t3aGrant{t3aPublish, t3aTopicARN, c.op, c.key, []string{c.value}}
			t3aCheck(t, iampolicy.Evaluate(g.doc(t), t3aOwnAccount), g.actions, c.public)
		})
	}
}
