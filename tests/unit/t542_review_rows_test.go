package unit_test

// A value AWS hands back names one resource in one account and one Region.
// Every surface that turns it into a row — a CloudTrail pivot, a TARGET row,
// a checker reading a profile or a grant — reads it through the target type's
// resolver with the account and Region the value lives in, and a session
// with no clients at all answers "unknown", never an AWS failure.

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsarn "github.com/aws/aws-sdk-go-v2/aws/arn"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/aws/smithy-go"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

// ─── row 8: a CloudTrail pivot counts what its resolver reads ──────────────

// t542EventRow is event as the ct-events list fetcher emits its row.
func t542EventRow(t *testing.T, event cloudtrailtypes.Event) resource.Resource {
	t.Helper()
	page, err := awsclient.FetchCloudTrailEventsPage(t.Context(), &ctOneEventAPI{event: event}, "")
	if err != nil || len(page.Resources) != 1 {
		t.Fatalf("building the event: %v (%d rows)", err, len(page.Resources))
	}
	return page.Resources[0]
}

// An event recorded in this account names each resource by an ARN carrying
// its own account and Region. The pivot counts the row that ARN names in the
// session's list: another account's resource, or one in another Region, is
// not the local row of the same name, as the TARGET rows of the same event
// already say.
func TestCTEventsPivots_CountOnlyTheRowTheResolverReads(t *testing.T) {
	b := newRefBench(t)
	secret := b.row(t, "secrets", "prod/database/primary")
	const stackUUID = "5b0e2f9a-8c1d-11ef-9a2b-0a1b2c3d4e5f"

	secretARN, err := awsarn.Parse(secret.Fields["arn"])
	if err != nil {
		t.Fatalf("demo secret ARN %q: %v", secret.Fields["arn"], err)
	}
	id := func(typ string) string { return b.byType[typ][0].ID }

	targets := []struct {
		typ, awsType, eventName, service, res string
		id                                    string
	}{
		{"ec2", "AWS::EC2::Instance", "StopInstances", "ec2", "instance/" + id("ec2"), id("ec2")},
		{"lambda", "AWS::Lambda::Function", "UpdateFunctionCode20150331v2", "lambda", "function:" + id("lambda"), id("lambda")},
		{"dbi", "AWS::RDS::DBInstance", "ModifyDBInstance", "rds", "db:" + id("dbi"), id("dbi")},
		{"kms", "AWS::KMS::Key", "DisableKeyRotation", "kms", "key/" + id("kms"), id("kms")},
		{"secrets", "AWS::SecretsManager::Secret", "PutSecretValue", "secretsmanager", secretARN.Resource, secret.ID},
		{"sg", "AWS::EC2::SecurityGroup", "AuthorizeSecurityGroupIngress", "ec2", "security-group/" + id("sg"), id("sg")},
		{"ddb", "AWS::DynamoDB::Table", "UpdateTable", "dynamodb", "table/" + id("ddb"), id("ddb")},
		{"trail", "AWS::CloudTrail::Trail", "StopLogging", "cloudtrail", "trail/" + id("trail"), id("trail")},
		{"cfn", "AWS::CloudFormation::Stack", "UpdateStack", "cloudformation", "stack/" + id("cfn") + "/" + stackUUID, id("cfn")},
	}
	for i, tg := range targets {
		arnIn := func(region, account string) string {
			return "arn:aws:" + tg.service + ":" + region + ":" + account + ":" + tg.res
		}
		event := func(suffix, arn, account string) resource.Resource {
			return t542EventRow(t, t542Event("e-t542-r8-"+strconv.Itoa(i)+suffix, tg.eventName, tg.service+".amazonaws.com",
				[]t542Resource{{arn, account, tg.awsType}}, nil))
		}
		check := refChecker(t, "ct-events", tg.typ)
		t.Run(tg.typ+"/recorded account", func(t *testing.T) {
			got := check(context.Background(), refClients(), event("a", arnIn(refRegion, refAccount), refAccount), b.cache)
			if ids := sortedIDs(got); !slices.Equal(ids, []string{tg.id}) {
				t.Errorf("ct-events → %s for %s = %v, want [%s]", tg.typ, arnIn(refRegion, refAccount), ids, tg.id)
			}
		})
		t.Run(tg.typ+"/another account", func(t *testing.T) {
			arn := arnIn(refRegion, refForeignAccount)
			got := check(context.Background(), refClients(), event("b", arn, refForeignAccount), b.cache)
			if ids := got.ResourceIDs(); len(ids) != 0 {
				t.Errorf("ct-events → %s for %s counts %v: account %s's resource is not this account's row", tg.typ, arn, ids, refForeignAccount)
			}
		})
		t.Run(tg.typ+"/another Region", func(t *testing.T) {
			arn := arnIn("us-west-2", refAccount)
			got := check(context.Background(), refClients(), event("c", arn, refAccount), b.cache)
			if slices.Contains(got.ResourceIDs(), tg.id) && got.Region() != "us-west-2" {
				t.Errorf("ct-events → %s for %s counts %s read in Region %q: the %s row of that name is another resource", tg.typ, arn, tg.id, got.Region(), refRegion)
			}
		})
	}

	t.Run("s3/recorded bucket", func(t *testing.T) {
		bucket := b.byType["s3"][0].ID
		ev := t542EventRow(t, t542Event("e-t542-r8-s3", "PutBucketVersioning", "s3.amazonaws.com",
			[]t542Resource{{"arn:aws:s3:::" + bucket, refAccount, "AWS::S3::Bucket"}}, nil))
		got := refChecker(t, "ct-events", "s3")(context.Background(), refClients(), ev, b.cache)
		if ids := sortedIDs(got); !slices.Equal(ids, []string{bucket}) {
			t.Errorf("ct-events → s3 = %v, want [%s]", ids, bucket)
		}
	})
}

// A key answers to every alias that targets it. A Decrypt that names the key
// by an alias other than the one the list shows first is a call on that key,
// as the event's TARGET Key row opens it; another account's alias of the same
// name is another account's key.
func TestCTEventsKMS_AnyAliasOfTheKeyCountsTheKey(t *testing.T) {
	b := newRefBench(t)
	key := b.row(t, "kms", fixtures.OrdersProdKMSKeyID)
	var secondary string
	for _, a := range strings.Split(key.Fields["aliases"], ",") {
		if a != "" && a != key.Fields["alias"] {
			secondary = a
		}
	}
	if secondary == "" {
		t.Fatalf("demo key %s carries one alias (%q); the test needs a second", key.ID, key.Fields["aliases"])
	}
	check := refChecker(t, "ct-events", "kms")

	got := check(context.Background(), refClients(), t542EventRow(t, t542Event("e-t542-r8-kms-alias", "Decrypt", "kms.amazonaws.com", nil,
		map[string]any{"keyId": secondary, "encryptionAlgorithm": "SYMMETRIC_DEFAULT"})), b.cache)
	if ids := sortedIDs(got); !slices.Equal(ids, []string{key.ID}) {
		t.Errorf("ct-events → kms for Decrypt by %s = %v, want [%s], the key the alias targets", secondary, ids, key.ID)
	}

	foreign := "arn:aws:kms:" + refRegion + ":" + refForeignAccount + ":" + key.Fields["alias"]
	got = check(context.Background(), refClients(), t542EventRow(t, t542Event("e-t542-r8-kms-foreign", "Decrypt", "kms.amazonaws.com", nil,
		map[string]any{"keyId": foreign, "encryptionAlgorithm": "SYMMETRIC_DEFAULT"})), b.cache)
	if ids := got.ResourceIDs(); len(ids) != 0 {
		t.Errorf("ct-events → kms for Decrypt by %s counts %v: another account's alias names another account's key", foreign, ids)
	}
}

// ─── row 9: a value in another Region counts and opens there ───────────────

// The key encrypting a bucket lives in the bucket's Region. The pivot counts
// that key, marked with that Region so its drill reads it there — even when
// the session's own Region holds a key under the same alias name, which is
// another key.
func TestS3KMS_TheBucketsKeyCountsInTheBucketsRegion(t *testing.T) {
	w := newRegionWorld()
	clients := w.clients("us-east-1")
	cache := resource.ResourceCache{"kms": {Resources: rwFetch(t, clients, "kms")}}
	buckets := rwS3Resources(t, w, clients)

	for _, bucket := range []string{rwAssetsBucket, rwReportsBucket} {
		t.Run(bucket, func(t *testing.T) {
			got := rwChecker(t, "s3", "kms")(context.Background(), clients, rwByID(t, buckets, bucket), cache)
			if ids := sortedIDs(got); !slices.Equal(ids, []string{rwEUAssetsKeyID}) || got.Region() != "eu-west-1" {
				t.Errorf("s3 → kms for %s = %v in Region %q (state %v), want [%s] in eu-west-1", bucket, ids, got.Region(), got.EffectiveState(), rwEUAssetsKeyID)
			}
		})
	}
}

// A multi-Region trail is listed in every Region, and its SNS topic and KMS
// key live in its home Region. Browsed from another Region, the trail's
// panel counts them there, as its detail fields already open them there. A
// trail homed in the session's Region keeps the session's.
func TestTrail_MultiRegionTrailsTopicAndKeyCountInItsHomeRegion(t *testing.T) {
	w := newRegionWorld()
	clients := w.clients("eu-west-1")
	trails := rwFetch(t, clients, "trail")
	cache := resource.ResourceCache{
		"sns": {Resources: rwFetch(t, clients, "sns")},
		"kms": {Resources: rwFetch(t, clients, "kms")},
	}
	org := rwByID(t, trails, rwOrgTrail)
	eu := rwByID(t, trails, rwEUTrail)

	cases := []struct {
		name, target string
		trail        resource.Resource
		want         []string
		region       string
	}{
		{"org trail topic", "sns", org, []string{rwOrgTrailTopic}, "us-west-2"},
		{"org trail key", "kms", org, []string{rwOrgTrailKMSKeyID}, "us-west-2"},
		{"eu trail key", "kms", eu, []string{rwEUTrailKMSKeyID}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rwChecker(t, "trail", tc.target)(context.Background(), clients, tc.trail, cache)
			if ids := sortedIDs(got); !slices.Equal(ids, tc.want) || got.Region() != tc.region || got.Truncated() {
				t.Errorf("trail → %s = %v in Region %q (state %v, truncated %v), want exactly %v in Region %q",
					tc.target, ids, got.Region(), got.EffectiveState(), got.Truncated(), tc.want, tc.region)
			}
		})
	}
}

// A TARGET row whose ARN names another Region opens the resource there: the
// session Region's row of the same name is another resource.
func TestCTTarget_ARNInAnotherRegionOpensThere(t *testing.T) {
	b := newRefBench(t)
	c := refDetailController(t, b)
	fn := b.byType["lambda"][0].ID
	arn := "arn:aws:lambda:us-west-2:" + refAccount + ":function:" + fn

	fields := t542EventFields(t, c, t542Event("e-t542-r9-01", "UpdateFunctionConfiguration20150331v2", "lambda.amazonaws.com",
		[]t542Resource{{arn, refAccount, "AWS::Lambda::Function"}}, nil))
	idx := slices.IndexFunc(fields, t542IsTargetRow)
	if idx < 0 {
		t.Fatalf("the event has no TARGET row; fields: %+v", fields)
	}
	if f := fields[idx]; !f.IsNavigable || f.TargetType != "lambda" {
		t.Fatalf("TARGET %s %q: navigable=%v type=%q, want a lambda link", f.Key, f.Value, f.IsNavigable, f.TargetType)
	}

	_, tasks := c.Apply(app.Action{Kind: app.ActionFieldSelect, Arg: strconv.Itoa(idx)})
	fetched := false
	for _, task := range tasks {
		switch p := task.Payload.(type) {
		case runtime.FetchResourcesPayload:
			fetched = true
			if p.Region != "us-west-2" {
				t.Errorf("the TARGET drill lists Region %q, want us-west-2", p.Region)
			}
		case runtime.FetchByIDDetailPayload:
			fetched = true
			if p.Region != "us-west-2" {
				t.Errorf("the TARGET drill looks the function up in Region %q, want us-west-2", p.Region)
			}
		case runtime.EnrichDetailPayload:
			if got := p.Op.Resource.Fields["arn"]; got != arn {
				t.Errorf("Enter on the TARGET row opened %s, another function than %s", got, arn)
			}
		}
	}
	if !fetched {
		t.Errorf("Enter on the TARGET row for %s fetched nothing from us-west-2", arn)
	}
}

// ─── row 10: an instance profile names its roles by its own list ──────────

// t542ProfileIAM answers the instance-profile reads as IAM does, from one
// table of profile name → the roles it holds.
type t542ProfileIAM struct {
	awsclient.IAMAPI
	profiles map[string][]string
}

func (f *t542ProfileIAM) profile(name string) iamtypes.InstanceProfile {
	p := iamtypes.InstanceProfile{
		InstanceProfileName: aws.String(name),
		InstanceProfileId:   aws.String("AIPAEXAMPL" + strings.ToUpper(strings.ReplaceAll(name, "-", ""))),
		Arn:                 aws.String("arn:aws:iam::" + refAccount + ":instance-profile/" + name),
		Path:                aws.String("/"),
	}
	for _, r := range f.profiles[name] {
		p.Roles = append(p.Roles, iamtypes.Role{RoleName: aws.String(r), Arn: aws.String("arn:aws:iam::" + refAccount + ":role/" + r), Path: aws.String("/")})
	}
	return p
}

func (f *t542ProfileIAM) GetInstanceProfile(_ context.Context, in *iam.GetInstanceProfileInput, _ ...func(*iam.Options)) (*iam.GetInstanceProfileOutput, error) {
	name := aws.ToString(in.InstanceProfileName)
	if _, ok := f.profiles[name]; !ok {
		return nil, &smithy.GenericAPIError{Code: "NoSuchEntity", Message: "Instance Profile " + name + " cannot be found."}
	}
	p := f.profile(name)
	return &iam.GetInstanceProfileOutput{InstanceProfile: &p}, nil
}

func (f *t542ProfileIAM) ListInstanceProfilesForRole(_ context.Context, in *iam.ListInstanceProfilesForRoleInput, _ ...func(*iam.Options)) (*iam.ListInstanceProfilesForRoleOutput, error) {
	out := &iam.ListInstanceProfilesForRoleOutput{InstanceProfiles: []iamtypes.InstanceProfile{}}
	for name, roles := range f.profiles {
		if slices.Contains(roles, aws.ToString(in.RoleName)) {
			out.InstanceProfiles = append(out.InstanceProfiles, f.profile(name))
		}
	}
	return out, nil
}

func t542Instance(id, profile string) resource.Resource {
	return resource.Resource{ID: id, Name: id, Type: "ec2", RawStruct: ec2types.Instance{
		InstanceId:         aws.String(id),
		IamInstanceProfile: &ec2types.IamInstanceProfile{Arn: aws.String("arn:aws:iam::" + refAccount + ":instance-profile/" + profile), Id: aws.String("AIPAEXAMPLE" + strconv.Itoa(len(profile)))},
	}}
}

// A role runs on the instances whose profile holds it. The profile "app"
// holds "app-v2": its instance runs under app-v2, and the role "app", named
// like the profile, is in no profile at all.
func TestRoleEC2_InstancesAreThoseWhoseProfileHoldsTheRole(t *testing.T) {
	iamFake := &t542ProfileIAM{profiles: map[string][]string{"app": {"app-v2"}, "batch": {"batch"}}}
	clients := &awsclient.ServiceClients{IAM: iamFake}
	cache := resource.ResourceCache{"ec2": {Resources: []resource.Resource{
		t542Instance("i-0a1b2c3d4e5f60001", "app"),
		t542Instance("i-0a1b2c3d4e5f60002", "batch"),
	}}}
	check := refChecker(t, "role", "ec2")

	for _, tc := range []struct {
		role string
		want []string
	}{
		{"app-v2", []string{"i-0a1b2c3d4e5f60001"}},
		{"app", nil},
		{"batch", []string{"i-0a1b2c3d4e5f60002"}},
	} {
		t.Run(tc.role, func(t *testing.T) {
			role := resource.Resource{ID: tc.role, Name: tc.role, Type: "role", RawStruct: iamtypes.Role{
				RoleName: aws.String(tc.role), Arn: aws.String("arn:aws:iam::" + refAccount + ":role/" + tc.role),
			}}
			got := check(context.Background(), clients, role, cache)
			if ids := sortedIDs(got); !slices.Equal(ids, tc.want) || got.EffectiveState() != domain.RelatedResolved || got.Truncated() {
				t.Errorf("role %s → ec2 = %v (state %v, truncated %v), want exactly %v", tc.role, ids, got.EffectiveState(), got.Truncated(), tc.want)
			}
		})
	}
}

// ─── row 11: a resolver refuses by shape what its list cannot hold ─────────

// An S3 ARN names a bucket only when it carries no Region and no account; an
// access point, a Multi-Region Access Point, a Batch Operations job or a
// Storage Lens configuration is no bucket. An object ARN names its bucket.
// An EC2 instance id is i-…; an SSM managed node registered from outside
// EC2 is mi-… and no instance.
func TestResolveRef_S3AndEC2RefuseWhatTheirListCannotHold(t *testing.T) {
	b := newRefBench(t)
	bucket := b.byType["s3"][0].ID
	inst := b.byType["ec2"][0].ID
	runRefCases(t, b, []refCase{
		{"bucket ARN", "s3", "arn:aws:s3:::" + bucket, bucket, true},
		{"object ARN", "s3", "arn:aws:s3:::" + bucket + "/reports/2026/q3.csv", bucket, true},
		{"access point", "s3", "arn:aws:s3:" + refRegion + ":" + refAccount + ":accesspoint/acme-analytics-ap", "", false},
		{"multi-region access point", "s3", "arn:aws:s3::" + refAccount + ":accesspoint/mfzwi23gnjvgw.mrap", "", false},
		{"batch job", "s3", "arn:aws:s3:" + refRegion + ":" + refAccount + ":job/0f1e2d3c-4b5a-4978-8a9b-0c1d2e3f4a5b", "", false},
		{"storage lens", "s3", "arn:aws:s3:" + refRegion + ":" + refAccount + ":storage-lens/acme-org-lens", "", false},
		{"instance id", "ec2", inst, inst, true},
		{"managed node id", "ec2", "mi-0123456789abcdef0", "", false},
	})
}

// A TARGET row links only a value its type's resolver reads as a row: an SSM
// heartbeat from an on-premises managed node, and an access point's ARN, name
// nothing Enter could open.
func TestCTTarget_ManagedNodeAndAccessPointAreNotLinks(t *testing.T) {
	b := newRefBench(t)
	c := refDetailController(t, b)
	inst := b.byType["ec2"][0].ID

	cases := []struct {
		name     string
		event    cloudtrailtypes.Event
		wantType string
		wantID   string
	}{
		{"managed node", t542Event("e-t542-r11-01", "UpdateInstanceInformation", "ssm.amazonaws.com", nil,
			map[string]any{"instanceId": "mi-0123456789abcdef0", "agentVersion": "3.3.1142.0", "platformType": "Linux"}), "", ""},
		{"EC2 instance", t542Event("e-t542-r11-02", "UpdateInstanceInformation", "ssm.amazonaws.com", nil,
			map[string]any{"instanceId": inst, "agentVersion": "3.3.1142.0", "platformType": "Linux"}), "ec2", inst},
		{"access point", t542Event("e-t542-r11-03", "PutAccessPointPolicy", "s3.amazonaws.com",
			[]t542Resource{{"arn:aws:s3:" + refRegion + ":" + refAccount + ":accesspoint/acme-analytics-ap", refAccount, "AWS::S3::AccessPoint"}}, nil), "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows := t542TargetRows(t, c, tc.event)
			if len(rows) != 1 {
				t.Fatalf("TARGET rows = %+v, want exactly one", rows)
			}
			r := rows[0]
			if tc.wantType == "" {
				if r.IsNavigable {
					t.Errorf("TARGET %s %q links to %s %q, a row its list can never hold", r.Key, r.Value, r.TargetType, navTarget(r))
				}
				return
			}
			if !r.IsNavigable || r.TargetType != tc.wantType || navTarget(r) != tc.wantID {
				t.Errorf("TARGET %s %q: navigable=%v type=%q opens %q, want %s %q", r.Key, r.Value, r.IsNavigable, r.TargetType, navTarget(r), tc.wantType, tc.wantID)
			}
		})
	}
}

// Secrets Manager accepts a partial ARN — the name without the six random
// characters — and a name may itself end in "-" and six characters, so only
// the list tells "acme-app-config" from "acme-app" plus a suffix. A reader
// without that secret on its list never cuts the name down to another
// secret's.
func TestResolveRef_SecretSuffixIsDecidedByTheList(t *testing.T) {
	const prefix = "arn:aws:secretsmanager:" + refRegion + ":" + refAccount + ":secret:"
	app := resource.Resource{ID: "acme-app", Name: "acme-app", Type: "secrets", Fields: map[string]string{"arn": prefix + "acme-app-Xy12Zq"}}
	appConfig := resource.Resource{ID: "acme-app-config", Name: "acme-app-config", Type: "secrets", Fields: map[string]string{"arn": prefix + "acme-app-config-Ab34Cd"}}

	cases := []struct {
		name    string
		ref     string
		targets []resource.Resource
		wantID  string
	}{
		{"partial ARN, both listed", prefix + "acme-app-config", []resource.Resource{app, appConfig}, "acme-app-config"},
		{"full ARN, both listed", prefix + "acme-app-Xy12Zq", []resource.Resource{app, appConfig}, "acme-app"},
		{"partial ARN, only acme-app listed", prefix + "acme-app-config", []resource.Resource{app}, ""},
		{"partial ARN, no list", prefix + "acme-app-config", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id, ok := resource.ResolveRef("secrets", tc.ref, domain.RefContext{AccountID: refAccount, Region: refRegion, Targets: tc.targets})
			if tc.wantID != "" {
				if !ok || id != tc.wantID {
					t.Errorf("ResolveRef(secrets, %q) = (%q, %v), want %q", tc.ref, id, ok, tc.wantID)
				}
				return
			}
			if ok && id == "acme-app" {
				t.Errorf("ResolveRef(secrets, %q) = %q: the name was cut to another secret's", tc.ref, id)
			}
		})
	}
}

// secretsListCache is a loaded secrets list holding the secret each full ARN
// names: its row ID is the name, the ARN less the "-" and six characters
// Secrets Manager appends.
func secretsListCache(arns ...string) resource.ResourceCache {
	rows := make([]resource.Resource, 0, len(arns))
	for _, arn := range arns {
		_, name, _ := strings.Cut(arn, ":secret:")
		name = name[:len(name)-7]
		rows = append(rows, resource.Resource{ID: name, Name: name, Type: "secrets", Fields: map[string]string{"arn": arn, "name": name}})
	}
	return resource.ResourceCache{"secrets": {Resources: rows}}
}

// t542GrantsKMS answers a key's policy and grants as KMS does.
type t542GrantsKMS struct {
	awsclient.KMSAPI
	grants []kmstypes.GrantListEntry
}

func (f *t542GrantsKMS) GetKeyPolicy(_ context.Context, _ *kms.GetKeyPolicyInput, _ ...func(*kms.Options)) (*kms.GetKeyPolicyOutput, error) {
	return &kms.GetKeyPolicyOutput{PolicyName: aws.String("default"), Policy: aws.String(`{"Version":"2012-10-17","Id":"key-default-1","Statement":[{"Sid":"Enable IAM User Permissions","Effect":"Allow","Principal":{"AWS":"arn:aws:iam::` + refAccount + `:root"},"Action":"kms:*","Resource":"*"}]}`)}, nil
}

func (f *t542GrantsKMS) ListGrants(_ context.Context, _ *kms.ListGrantsInput, _ ...func(*kms.Options)) (*kms.ListGrantsOutput, error) {
	return &kms.ListGrantsOutput{Grants: f.grants, Truncated: false}, nil
}

// CreateGrant accepts an assumed-role session as grantee; the grant is the
// role's. A grant to a user or to an AWS service principal names no role,
// and leaves the count exact: every grant was read.
func TestKMSRole_GrantToAnAssumedRoleSessionCountsTheRole(t *testing.T) {
	const keyID = "8d7c6b5a-4f3e-4d2c-9b1a-0e9f8d7c6b5a"
	keyARN := "arn:aws:kms:" + refRegion + ":" + refAccount + ":key/" + keyID
	grant := func(id, grantee, retiring string) kmstypes.GrantListEntry {
		g := kmstypes.GrantListEntry{
			GrantId: aws.String(id), KeyId: aws.String(keyARN), GranteePrincipal: aws.String(grantee),
			IssuingAccount: aws.String("arn:aws:iam::" + refAccount + ":root"),
			Operations:     []kmstypes.GrantOperation{kmstypes.GrantOperationDecrypt, kmstypes.GrantOperationGenerateDataKey},
		}
		if retiring != "" {
			g.RetiringPrincipal = aws.String(retiring)
		}
		return g
	}
	kmsFake := &t542GrantsKMS{grants: []kmstypes.GrantListEntry{
		grant("0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d", "arn:aws:sts::"+refAccount+":assumed-role/acme-deployer/ci-run-42", ""),
		grant("1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e", "arn:aws:iam::"+refAccount+":role/acme-app", "arn:aws:iam::"+refAccount+":user/ops-admin"),
		grant("2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f", "rds."+refRegion+".amazonaws.com", ""),
	}}
	clients := &awsclient.ServiceClients{KMS: kmsFake, Region: refRegion}
	key := resource.Resource{ID: keyID, Name: "alias/acme-deploy-key", Type: "kms", RawStruct: kmstypes.KeyMetadata{KeyId: aws.String(keyID), Arn: aws.String(keyARN)}}

	got := refChecker(t, "kms", "role")(context.Background(), clients, key, resource.ResourceCache{})
	if ids := sortedIDs(got); !slices.Equal(ids, []string{"acme-app", "acme-deployer"}) || got.Truncated() || got.EffectiveState() != domain.RelatedResolved {
		t.Errorf("kms → role = %v (state %v, truncated %v), want exactly [acme-app acme-deployer]", ids, got.EffectiveState(), got.Truncated())
	}
}

// ─── row 12: no client set at all reads unknown ────────────────────────────

// Before a connect settles, or after it fails, the session holds no client set
// while its disk rows are browsable. Every list fetcher reached then answers
// the one missing-client signal, typed nil or untyped.
func TestFetchRelatedTarget_NoClientSetIsClientMissing(t *testing.T) {
	for _, clients := range []struct {
		name string
		c    any
	}{{"typed nil", (*awsclient.ServiceClients)(nil)}, {"untyped nil", nil}} {
		for _, td := range resource.AllResourceTypes() {
			if resource.GetPaginatedFetcher(td.ShortName) == nil {
				continue
			}
			t.Run(clients.name+"/"+td.ShortName, func(t *testing.T) {
				list, _, err := awsclient.FetchRelatedTarget(context.Background(), clients.c, nil, td.ShortName)
				if list != nil || err == nil || !strings.Contains(err.Error(), "AWS service client not initialized") {
					t.Errorf("FetchRelatedTarget(%q) with no client set = (%d rows, %v), want no list and the client-missing error", td.ShortName, len(list), err)
				}
				if missing := awsclient.TargetClientMissing(clients.c, td.ShortName); missing == nil {
					t.Errorf("TargetClientMissing(%q) with no client set = nil, want the client-missing error", td.ShortName)
				}
			})
		}
	}
}

// With no client set, a pivot either answers what it answers with clients —
// read from the row itself or from a list already loaded — or reads unknown.
// It never reads as an error, never a count the clients would not give, and
// its result raises no error flash: no AWS call was made to fail.
func TestRelatedPanel_NoClientSetReadsUnknownNeverAnError(t *testing.T) {
	b := newRefBench(t)
	full := make(map[string]struct{}, len(b.cache))
	for typ := range b.cache {
		full[typ] = struct{}{}
	}
	core := runtime.New(session.New(), nil)
	types := make([]string, 0, len(b.byType))
	for typ := range b.byType {
		types = append(types, typ)
	}
	slices.Sort(types)

	var bad []string
	for _, typ := range types {
		rows := b.byType[typ]
		defs := resource.GetRelated(typ)
		if len(rows) == 0 || len(defs) == 0 {
			continue
		}
		src := rows[0]
		disk := resource.ResourceCache{typ: {Resources: rows}}
		for _, def := range defs {
			with := runtime.RunRelatedDef(context.Background(), runtime.DetailOperation{ID: 1, ResourceType: typ, Resource: src, Clients: refClients()}, b.cache, full, def)
			none := runtime.RunRelatedDef(context.Background(), runtime.DetailOperation{ID: 1, ResourceType: typ, Resource: src}, disk, map[string]struct{}{typ: {}}, def)
			pivot := typ + " → " + def.TargetType + " (" + src.ID + ")"
			state := none.Result.EffectiveState()
			switch {
			case state == domain.RelatedError || none.Result.Err() != nil:
				bad = append(bad, pivot+": error "+errText(none.Result.Err()))
			case none.LazyAddError != nil:
				bad = append(bad, pivot+": lazy-add error "+none.LazyAddError.Error())
			case state == domain.RelatedUnknown:
			case state != with.Result.EffectiveState() || !slices.Equal(sortedIDs(none.Result), sortedIDs(with.Result)) || none.Result.Truncated() != with.Result.Truncated():
				bad = append(bad, pivot+": "+state.String()+" "+strings.Join(sortedIDs(none.Result), ",")+
					" without clients, "+with.Result.EffectiveState().String()+" "+strings.Join(sortedIDs(with.Result), ",")+" with them")
			}
			intents, _ := core.HandleRelatedCheckResult(runtime.RelatedCheckResultEvent{
				ResourceType: typ, SourceResourceID: src.ID, DefDisplayName: def.DisplayName,
				Result: none.Result, LazyAddError: none.LazyAddError,
			})
			for _, in := range intents {
				if f, ok := in.(runtime.FlashIntent); ok && f.IsError {
					bad = append(bad, pivot+": flash "+strconv.Quote(f.Text))
				}
			}
		}
	}
	if len(bad) > 0 {
		t.Errorf("%d pivot result(s) with no client set that are not unknown or the clients' own answer:\n  %s", len(bad), strings.Join(bad, "\n  "))
	}
}

func errText(err error) string {
	if err == nil {
		return "<nil>"
	}
	return err.Error()
}

// ─── row 13: a panicking checker is that pivot's error ─────────────────────

// A checker that panics is a defect in that pivot. It is reported as that
// pivot's error — the row fails and the flash names the pivot — not as a
// failed by-ID fetch beside it, which never ran.
func TestRunRelatedDef_APanickingCheckerIsThatPivotsError(t *testing.T) {
	var inst *ec2types.Instance
	def := resource.RelatedDef{
		TargetType:  "lambda",
		DisplayName: "Lambda Functions",
		Checker: func(context.Context, any, resource.Resource, resource.ResourceCache) resource.RelatedCheckResult {
			return resource.KnownRelated("lambda", []string{aws.ToString(inst.InstanceId)}, false)
		},
	}
	src := resource.Resource{ID: "a1b2c3d4e5", Name: "acme-orders-api", Type: "apigw"}
	ev := runtime.RunRelatedDef(context.Background(), runtime.DetailOperation{ID: 1, ResourceType: "apigw", Resource: src}, resource.ResourceCache{}, nil, def)

	if ev.LazyAddError != nil {
		t.Errorf("LazyAddError = %v: no by-ID fetch ran, the checker panicked", ev.LazyAddError)
	}
	if ev.Result.EffectiveState() != domain.RelatedError || ev.Result.TargetType() != "lambda" ||
		ev.Result.Err() == nil || !strings.Contains(ev.Result.Err().Error(), "panicked") {
		t.Fatalf("result = state %v target %q err %v, want the lambda pivot's error naming the panic", ev.Result.EffectiveState(), ev.Result.TargetType(), ev.Result.Err())
	}

	intents, _ := runtime.New(session.New(), nil).HandleRelatedCheckResult(runtime.RelatedCheckResultEvent{
		ResourceType: "apigw", SourceResourceID: src.ID, DefDisplayName: def.DisplayName,
		Result: ev.Result, LazyAddError: ev.LazyAddError,
	})
	var flashes []string
	for _, in := range intents {
		if f, ok := in.(runtime.FlashIntent); ok && f.IsError {
			flashes = append(flashes, f.Text)
		}
	}
	if len(flashes) != 1 || !strings.HasPrefix(flashes[0], "related lambda:") || strings.Contains(flashes[0], "related-fetch") {
		t.Errorf("error flashes = %q, want one naming the pivot (\"related lambda: …\")", flashes)
	}
}
