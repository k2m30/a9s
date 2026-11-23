package unit

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/iampolicy"
)

const (
	t3aTopicARN    = "arn:aws:sns:us-east-1:123456789012:acme-alerts"
	t3aFunctionARN = "arn:aws:lambda:us-east-1:123456789012:function:acme-api"
	t3aFileSystem  = "arn:aws:elasticfilesystem:us-east-1:123456789012:file-system/fs-0123456789abcdef0"
	t3aOwnAccount  = "123456789012"
)

type t3aGrant struct {
	actions  []string
	resource string
	op, key  string
	values   []string
}

func (g t3aGrant) doc(t *testing.T) iampolicy.Document {
	t.Helper()
	st := map[string]any{
		"Sid":       "AcmeGrant",
		"Effect":    "Allow",
		"Principal": "*",
		"Action":    g.actions,
	}
	if g.resource != "" {
		st["Resource"] = g.resource
	}
	if g.op != "" {
		st["Condition"] = map[string]any{g.op: map[string]any{g.key: g.values}}
	}
	raw, err := json.Marshal(map[string]any{"Version": "2012-10-17", "Statement": []any{st}})
	if err != nil {
		t.Fatal(err)
	}
	return mustParse(t, string(raw))
}

func t3aCheck(t *testing.T, ex iampolicy.Exposure, actions []string, public bool) {
	t.Helper()
	if public {
		want := slices.Sorted(slices.Values(actions))
		if !ex.Public || ex.Conditioned || !slices.Equal(ex.PublicActions, want) {
			t.Errorf("want Public with PublicActions %v, Conditioned false; got %+v", want, ex)
		}
		return
	}
	if ex.Public || !ex.Conditioned || ex.PublicActions != nil {
		t.Errorf("want Conditioned, not Public, no PublicActions; got %+v", ex)
	}
}

var (
	t3aPublish   = []string{"sns:Publish"}
	t3aURLInvoke = []string{"lambda:InvokeFunctionUrl"}
	t3aEFSClient = []string{"elasticfilesystem:ClientMount", "elasticfilesystem:ClientWrite", "elasticfilesystem:ClientRootAccess"}
)

// AWS: "To be considered non-public, a bucket policy must grant access only
// to fixed values (values that don't contain a wildcard ...)" and lists
// "aws:userid, outside the pattern AROLEID:*" (S3 User Guide, Blocking public
// access, The meaning of "public",
// https://docs.aws.amazon.com/AmazonS3/latest/userguide/access-control-block-public-access.html;
// the EFS page,
// https://docs.aws.amazon.com/efs/latest/ug/access-control-block-public-access.html,
// uses the same rule). A '*' or '?' anywhere in the value makes it not fixed,
// except the documented AROLEID:* shape: one role ID followed by ":*" names
// every session of that one role ("AROAXROLE1:* represents the unique ID for
// all sessions associated with the IAM role", IAM User Guide, Disabling
// permissions for temporary security credentials).
func TestT3a_FixedValueRule(t *testing.T) {
	cases := []struct {
		name   string
		grant  t3aGrant
		public bool
	}{
		{"row1_source_arn_any_bucket_any_account", t3aGrant{t3aPublish, t3aTopicARN, "ArnLike", "aws:SourceArn", []string{"arn:aws:s3:*:*:*"}}, true},
		{"row1_source_arn_name_prefix_wildcard", t3aGrant{t3aPublish, t3aTopicARN, "ArnLike", "aws:SourceArn", []string{"arn:aws:s3:::acme-*"}}, true},
		{"row1_principal_arn_any_account", t3aGrant{t3aPublish, t3aTopicARN, "ArnLike", "aws:PrincipalArn", []string{"arn:aws:iam::*:role/acme-deployer"}}, true},
		{"row1_one_wildcard_value_among_fixed", t3aGrant{t3aPublish, t3aTopicARN, "ArnLike", "aws:SourceArn", []string{"arn:aws:s3:::acme-bucket", "arn:aws:s3:::*"}}, true},
		{"row1_source_vpc_question_mark", t3aGrant{t3aPublish, t3aTopicARN, "StringLike", "aws:SourceVpc", []string{"vpc-0a1b2c3d4e5f6071?"}}, true},
		{"row2_source_arn_fixed_bucket", t3aGrant{t3aPublish, t3aTopicARN, "ArnLike", "aws:SourceArn", []string{"arn:aws:s3:::acme-bucket"}}, false},
		{"row2_principal_arn_fixed_role", t3aGrant{t3aPublish, t3aTopicARN, "ArnEquals", "aws:PrincipalArn", []string{"arn:aws:iam::123456789012:role/acme-deployer"}}, false},
		{"row2_source_vpc_fixed", t3aGrant{t3aPublish, t3aTopicARN, "StringEquals", "aws:SourceVpc", []string{"vpc-0a1b2c3d4e5f60718"}}, false},
		{"row3_userid_any_session_of_role", t3aGrant{t3aPublish, t3aTopicARN, "StringLike", "aws:userid", []string{"AROAEXAMPLEID:*"}}, false},
		{"row3_userid_one_session_of_role", t3aGrant{t3aPublish, t3aTopicARN, "StringEquals", "aws:userid", []string{"AROAEXAMPLEID:acme-session"}}, false},
		{"row3_userid_iam_user", t3aGrant{t3aPublish, t3aTopicARN, "StringEquals", "aws:userid", []string{"AIDAEXAMPLEUSERID"}}, false},
		{"row4_principal_account_prefix", t3aGrant{t3aPublish, t3aTopicARN, "StringEquals", "aws:PrincipalAccount", []string{"1234*"}}, true},
		{"row4_principal_account_fixed", t3aGrant{t3aPublish, t3aTopicARN, "StringEquals", "aws:PrincipalAccount", []string{"123456789012"}}, false},
		{"row4_principal_org_id_prefix", t3aGrant{t3aPublish, t3aTopicARN, "StringLike", "aws:PrincipalOrgID", []string{"o-acme*"}}, true},
		{"row4_principal_org_id_fixed", t3aGrant{t3aPublish, t3aTopicARN, "StringEquals", "aws:PrincipalOrgID", []string{"o-acme123456"}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t3aCheck(t, iampolicy.Evaluate(c.grant.doc(t), t3aOwnAccount), c.grant.actions, c.public)
		})
	}
}

// An address range with prefix length 0 matches every address of its family,
// whatever bits its base carries: IAM compares the request address with the
// first prefix-length bits of the range. The evaluator and
// awsclient.CIDROpenToEveryone answer that question the same way.
func TestT3a_OpenRangeSharedWithCIDROpenToEveryone(t *testing.T) {
	cases := []struct {
		name string
		cidr string
		open bool
	}{
		{"row5_ipv4_canonical", "0.0.0.0/0", true},
		{"row5_ipv4_nonzero_base", "10.0.0.0/0", true},
		{"row5_ipv6_canonical", "::/0", true},
		{"row5_ipv6_nonzero_base", "2001:db8::/0", true},
		{"row6_ipv4_documentation_range", "203.0.113.0/24", false},
		{"row6_ipv6_documentation_range", "2001:db8::/32", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := t3aGrant{t3aPublish, t3aTopicARN, "IpAddress", "aws:SourceIp", []string{c.cidr}}
			t3aCheck(t, iampolicy.Evaluate(g.doc(t), t3aOwnAccount), g.actions, c.open)
			if got := awsclient.CIDROpenToEveryone([]string{c.cidr}); got != c.open {
				t.Errorf("CIDROpenToEveryone(%q) = %v, want %v", c.cidr, got, c.open)
			}
		})
	}
}

// AWS_IAM makes Lambda require a signed request, and "If the principal making
// the request is in a different account, then the principal must have both an
// identity-based policy ... and permissions granted to them in a
// resource-based policy on the function" (Lambda Developer Guide, Control
// access to Lambda function URLs,
// https://docs.aws.amazon.com/lambda/latest/dg/urls-auth.html). With
// Principal "*" the resource-based half holds for every AWS account, so the
// auth type scopes nobody; a caller-scoping key beside it still does.
func TestT3a_Row7_FunctionURLAuthTypeScopesNobody(t *testing.T) {
	t.Run("aws_iam_alone", func(t *testing.T) {
		g := t3aGrant{t3aURLInvoke, t3aFunctionARN, "StringEquals", "lambda:FunctionUrlAuthType", []string{"AWS_IAM"}}
		t3aCheck(t, iampolicy.Evaluate(g.doc(t), t3aOwnAccount), g.actions, true)
	})
	t.Run("aws_iam_with_org_id", func(t *testing.T) {
		doc := mustParse(t, `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*",
			"Action":"lambda:InvokeFunctionUrl","Resource":"`+t3aFunctionARN+`",
			"Condition":{"StringEquals":{"lambda:FunctionUrlAuthType":"AWS_IAM","aws:PrincipalOrgID":"o-acme123456"}}}]}`)
		t3aCheck(t, iampolicy.Evaluate(doc, t3aOwnAccount), t3aURLInvoke, false)
	})
}

// AWS: "You can make this file system policy non-public by using the EFS
// condition key elasticfilesystem:AccessedViaMountTarget set to true." The
// list of keys that make an EFS file system policy non-public is aws:SourceArn,
// aws:SourceVpc, aws:SourceVpce, aws:SourceOwner, aws:SourceAccount,
// elasticfilesystem:AccessedViaMountTarget, aws:userid outside AROLEID:*, and a
// principal key such as aws:PrincipalOrgID (EFS User Guide, Blocking public
// access, The meaning of "public",
// https://docs.aws.amazon.com/efs/latest/ug/access-control-block-public-access.html).
// elasticfilesystem:AccessPointArn names the mount path, which every AWS
// account can use under Principal "*".
func TestT3a_Row8_EFSAccessedViaMountTarget(t *testing.T) {
	cases := []struct {
		name   string
		grant  t3aGrant
		public bool
	}{
		{"mount_target_true", t3aGrant{t3aEFSClient, t3aFileSystem, "Bool", "elasticfilesystem:AccessedViaMountTarget", []string{"true"}}, false},
		{"mount_target_false", t3aGrant{t3aEFSClient, t3aFileSystem, "Bool", "elasticfilesystem:AccessedViaMountTarget", []string{"false"}}, true},
		{"no_condition", t3aGrant{t3aEFSClient, t3aFileSystem, "", "", nil}, true},
		{"access_point_arn", t3aGrant{t3aEFSClient, t3aFileSystem, "StringEquals", "elasticfilesystem:AccessPointArn", []string{"arn:aws:elasticfilesystem:us-east-1:123456789012:access-point/fsap-0123456789abcdef0"}}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t3aCheck(t, iampolicy.Evaluate(c.grant.doc(t), t3aOwnAccount), c.grant.actions, c.public)
		})
	}
}

// AWS documents a trailing wildcard below an organization ID as the way to
// name an organization or an OU subtree: for aws:PrincipalOrgPaths, "The
// following condition returns true for principals in an account that is
// attached directly to the OU or any of its child OUs. When you include a
// wildcard, you must use the StringLike condition operator" with
// "o-a1b2c3d4e5/r-ab12/ou-ab12-11111111/ou-ab12-22222222/*", and "The following
// condition allows access for every principal in the o-a1b2c3d4e5
// organization" with "o-a1b2c3d4e5/*"; aws:SourceOrgPaths makes sure "a
// calling service can access your resource only when the request originates
// from a specific organizational unit (OU)" (IAM User Guide, AWS global
// condition context keys,
// https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_condition-keys.html).
// Organization IDs are globally unique; OU and root IDs are unique only
// within an organization, so the organization ID is the part that must be
// fixed.
func TestT3a_Row9_OrgPathsTrailingWildcard(t *testing.T) {
	cases := []struct {
		name   string
		key    string
		path   string
		public bool
	}{
		{"source_ou_subtree", "aws:SourceOrgPaths", "o-acme123456/r-ab12/ou-ab12-11111111/*", false},
		{"source_whole_org", "aws:SourceOrgPaths", "o-acme123456/*", false},
		{"source_org_id_prefix", "aws:SourceOrgPaths", "o-acme123456*", true},
		{"source_any_org", "aws:SourceOrgPaths", "o-*/r-ab12/ou-ab12-11111111/*", true},
		{"source_anything", "aws:SourceOrgPaths", "*", true},
		{"principal_ou_subtree", "aws:PrincipalOrgPaths", "o-acme123456/r-ab12/ou-ab12-11111111/*", false},
		{"principal_org_id_prefix", "aws:PrincipalOrgPaths", "o-acme123456*", true},
		{"principal_org_id_question_mark", "aws:PrincipalOrgPaths", "o-acme12345?/*", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := t3aGrant{t3aPublish, t3aTopicARN, "ForAnyValue:StringLike", c.key, []string{c.path}}
			t3aCheck(t, iampolicy.Evaluate(g.doc(t), t3aOwnAccount), g.actions, c.public)
		})
	}
}

// t3aWitness is a value that scopes a Principal "*" grant to fixed callers and
// one that, under the same operator, does not.
type t3aWitness struct{ op, fixed, open string }

var t3aWitnesses = map[string]t3aWitness{
	"aws:SourceAccount":                        {"StringEquals", "123456789012", "1234*"},
	"aws:SourceOwner":                          {"StringEquals", "123456789012", "12345678901?"},
	"aws:SourceArn":                            {"ArnLike", "arn:aws:s3:::acme-bucket", "arn:aws:s3:::*"},
	"aws:SourceVpc":                            {"StringEquals", "vpc-0a1b2c3d4e5f60718", "vpc-*"},
	"aws:SourceVpce":                           {"StringEquals", "vpce-0a1b2c3d4e5f60718", "vpce-*"},
	"aws:SourceOrgID":                          {"StringEquals", "o-acme123456", "o-*"},
	"aws:SourceOrgPaths":                       {"ForAnyValue:StringLike", "o-acme123456/r-ab12/ou-ab12-11111111/*", "o-*/r-ab12/*"},
	"aws:SourceIp":                             {"IpAddress", "203.0.113.0/24", "10.0.0.0/0"},
	"aws:PrincipalAccount":                     {"StringEquals", "123456789012", "1234*"},
	"aws:PrincipalArn":                         {"ArnLike", "arn:aws:iam::123456789012:role/acme-deployer", "arn:aws:iam::*:role/acme-deployer"},
	"aws:PrincipalOrgID":                       {"StringEquals", "o-acme123456", "o-acme*"},
	"aws:PrincipalOrgPaths":                    {"ForAnyValue:StringLike", "o-acme123456/r-ab12/ou-ab12-11111111/*", "o-*/r-ab12/*"},
	"aws:PrincipalIsAWSService":                {"Bool", "true", "false"},
	"aws:userid":                               {"StringLike", "AROAEXAMPLEID:*", "AROA*:*"},
	"aws:username":                             {"StringEquals", "acme-deployer", "acme-*"},
	"kms:CallerAccount":                        {"StringEquals", "123456789012", "1234*"},
	"elasticfilesystem:AccessedViaMountTarget": {"Bool", "true", "false"},
	"sts:ExternalId":                           {"StringEquals", "acme-external-id-7f3a", "acme-*"},
	"aws:ResourceAccount":                      {"StringEquals", "123456789012", "1234*"},
	"aws:ResourceOrgID":                        {"StringEquals", "o-acme123456", "o-*"},
	"aws:ResourceOrgPaths":                     {"ForAnyValue:StringLike", "o-acme123456/*", "o-*/*"},
	"s3:ResourceAccount":                       {"StringEquals", "123456789012", "1234*"},
}

type t3aVariant struct {
	name     string
	keys     []iampolicy.RestrictingKey
	eval     func(iampolicy.Document) iampolicy.Exposure
	actions  []string
	resource string
}

func t3aVariants() []t3aVariant {
	resource, endpoint := iampolicy.RestrictingKeys()
	return []t3aVariant{
		{"resource", resource, func(d iampolicy.Document) iampolicy.Exposure { return iampolicy.Evaluate(d, t3aOwnAccount) }, t3aPublish, t3aTopicARN},
		{"trust", resource, func(d iampolicy.Document) iampolicy.Exposure { return iampolicy.Evaluate(d, t3aOwnAccount) }, []string{"sts:AssumeRole"}, ""},
		{"endpoint", endpoint, iampolicy.EvaluateEndpoint, []string{"s3:GetObject"}, "*"},
	}
}

// Every key a table counts as restricting scopes a Principal "*" grant at a
// fixed value and stops scoping it at a value with a wildcard (or, for a
// range, prefix length 0; for a Bool key, false), and carries the AWS
// documentation page that makes it restricting. The resource table is the
// set AWS documents as making a Principal "*" grant non-public, and a trust
// policy is read by the same table: the third-party access page puts the
// partner's account in Principal and says "The external ID is not a special
// value". The endpoint table adds the resource-side keys.
func TestT3a_Row10_EveryTableKeyHasBothWitnesses(t *testing.T) {
	resourceKeys := []string{
		"aws:SourceAccount", "aws:SourceOwner", "aws:SourceArn", "aws:SourceVpc", "aws:SourceVpce",
		"aws:SourceOrgID", "aws:SourceOrgPaths", "aws:SourceIp", "aws:PrincipalAccount", "aws:PrincipalArn",
		"aws:PrincipalOrgID", "aws:PrincipalOrgPaths", "aws:userid",
		"kms:CallerAccount", "elasticfilesystem:AccessedViaMountTarget",
	}
	wantKeys := map[string][]string{
		"resource": resourceKeys,
		"trust":    slices.Clone(resourceKeys),
		"endpoint": append(slices.Clone(resourceKeys), "aws:ResourceAccount", "aws:ResourceOrgID", "aws:ResourceOrgPaths", "s3:ResourceAccount"),
	}
	for _, v := range t3aVariants() {
		t.Run(v.name, func(t *testing.T) {
			var got []string
			for _, k := range v.keys {
				got = append(got, k.Key)
			}
			if want := slices.Sorted(slices.Values(wantKeys[v.name])); !slices.Equal(slices.Sorted(slices.Values(got)), want) {
				t.Errorf("table keys = %v, want %v", slices.Sorted(slices.Values(got)), want)
			}
			for _, k := range v.keys {
				t.Run(k.Key, func(t *testing.T) {
					if !strings.HasPrefix(k.Citation, "https://docs.aws.amazon.com/") {
						t.Errorf("Citation = %q, want an AWS documentation URL", k.Citation)
					}
					w, ok := t3aWitnesses[k.Key]
					if !ok {
						t.Fatalf("no witness pair for table key %q", k.Key)
					}
					fixed := t3aGrant{v.actions, v.resource, w.op, k.Key, []string{w.fixed}}
					t3aCheck(t, v.eval(fixed.doc(t)), v.actions, false)
					open := t3aGrant{v.actions, v.resource, w.op, k.Key, []string{w.open}}
					t3aCheck(t, v.eval(open.doc(t)), v.actions, true)
				})
			}
		})
	}
}

// sts:ExternalId scopes a wildcard principal in no document kind: any caller
// that knows or guesses the value can present it, and no AWS page counts it
// as making a Principal "*" grant non-public. The resource-side keys scope
// only a VPC endpoint policy, where they narrow which resources the endpoint
// reaches; on a resource's own policy they hold for every caller. The
// fixed-value rule applies in every document kind.
func TestT3a_Row11_DocumentKindScope(t *testing.T) {
	assumeRole := []string{"sts:AssumeRole"}
	getObject := []string{"s3:GetObject"}
	externalID := t3aGrant{assumeRole, "", "StringEquals", "sts:ExternalId", []string{"acme-external-id-7f3a"}}
	externalIDOpen := t3aGrant{assumeRole, "", "StringLike", "sts:ExternalId", []string{"acme-*"}}
	resourceAccount := t3aGrant{getObject, "*", "StringEquals", "aws:ResourceAccount", []string{"123456789012"}}
	resourceAccountOpen := t3aGrant{getObject, "*", "StringLike", "aws:ResourceAccount", []string{"1234*"}}
	resourceOrgPath := t3aGrant{getObject, "*", "ForAnyValue:StringLike", "aws:ResourceOrgPaths", []string{"o-acme123456/r-ab12/ou-ab12-11111111/*"}}
	trustSourceArnOpen := t3aGrant{assumeRole, "", "ArnLike", "aws:SourceArn", []string{"arn:aws:s3:*:*:*"}}
	endpointOrgOpen := t3aGrant{getObject, "*", "StringLike", "aws:PrincipalOrgID", []string{"o-acme*"}}

	evalResource := func(d iampolicy.Document) iampolicy.Exposure { return iampolicy.Evaluate(d, t3aOwnAccount) }
	evalTrust := func(d iampolicy.Document) iampolicy.Exposure { return iampolicy.Evaluate(d, t3aOwnAccount) }
	cases := []struct {
		name   string
		eval   func(iampolicy.Document) iampolicy.Exposure
		grant  t3aGrant
		public bool
	}{
		{"trust_external_id_fixed", evalTrust, externalID, true},
		{"trust_external_id_wildcard", evalTrust, externalIDOpen, true},
		{"resource_external_id_fixed", evalResource, externalID, true},
		{"endpoint_external_id_fixed", iampolicy.EvaluateEndpoint, externalID, true},
		{"endpoint_resource_account_fixed", iampolicy.EvaluateEndpoint, resourceAccount, false},
		{"endpoint_resource_account_wildcard", iampolicy.EvaluateEndpoint, resourceAccountOpen, true},
		{"endpoint_resource_org_path_subtree", iampolicy.EvaluateEndpoint, resourceOrgPath, false},
		{"resource_resource_account_fixed", evalResource, resourceAccount, true},
		{"trust_resource_account_fixed", evalTrust, resourceAccount, true},
		{"trust_source_arn_wildcard", evalTrust, trustSourceArnOpen, true},
		{"endpoint_principal_org_id_wildcard", iampolicy.EvaluateEndpoint, endpointOrgOpen, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t3aCheck(t, c.eval(c.grant.doc(t)), c.grant.actions, c.public)
		})
	}
}
