package unit

// parse_rows4to8_test.go — an ARN AWS returned is matched on its parsed
// fields, an ARN a9s builds carries the session's partition, and a finding is
// raised only from what the evidence proves.
//
// The five rows share one failure mode: a value is judged by a shape someone
// assumed rather than by what it says. A partition string in a prefix match
// discards every resource outside the commercial partition and renders the
// pivot as a proven zero. An address missing from an inventory is read as a
// released address. A bucket missing from a cache is read as a deleted bucket.
// A TCP listener on 443 is read as plaintext. Each is a confident wrong
// answer, which costs an operator more than an admitted unknown.

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	mwaatypes "github.com/aws/aws-sdk-go-v2/service/mwaa/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
)

// ── Row 4: an ARN is matched on its parsed fields ─────────────────────────

// TestARNForService_MatchesOnServiceNotOnPartition pins the predicate the
// fourteen matching sites share. AWS returns these ARNs; a9s only decides
// whether each names the service it is looking for. The partition is a
// property of the account's region, not of the resource's kind, so comparing
// it discards every resource in China and GovCloud while proving nothing.
func TestARNForService_MatchesOnServiceNotOnPartition(t *testing.T) {
	tests := []struct {
		name        string
		arn         string
		service     string
		wantOK      bool
		wantResourc string
	}{
		{name: "commercial partition", arn: "arn:aws:sns:us-east-1:123456789012:acme-alerts", service: "sns", wantOK: true, wantResourc: "acme-alerts"},
		{name: "China partition", arn: "arn:aws-cn:sns:cn-north-1:123456789012:acme-alerts", service: "sns", wantOK: true, wantResourc: "acme-alerts"},
		{name: "GovCloud partition", arn: "arn:aws-us-gov:sns:us-gov-west-1:123456789012:acme-alerts", service: "sns", wantOK: true, wantResourc: "acme-alerts"},
		{name: "a partition AWS has not published yet", arn: "arn:aws-iso:sns:us-iso-east-1:123456789012:acme-alerts", service: "sns", wantOK: true, wantResourc: "acme-alerts"},
		{name: "another service in the same partition", arn: "arn:aws:sqs:us-east-1:123456789012:acme-queue", service: "sns", wantOK: false},
		{name: "another service in another partition", arn: "arn:aws-cn:sqs:cn-north-1:123456789012:acme-queue", service: "sns", wantOK: false},
		{name: "a service whose name merely starts the same", arn: "arn:aws:snsx:us-east-1:123456789012:x", service: "sns", wantOK: false},
		{name: "an s3 ARN has no region or account", arn: "arn:aws:s3:::acme-assets", service: "s3", wantOK: true, wantResourc: "acme-assets"},
		{name: "a China s3 ARN", arn: "arn:aws-cn:s3:::acme-assets", service: "s3", wantOK: true, wantResourc: "acme-assets"},
		{name: "too few segments", arn: "arn:aws:sns", service: "sns", wantOK: false},
		{name: "not an ARN at all", arn: "acme-alerts", service: "sns", wantOK: false},
		{name: "empty", arn: "", service: "sns", wantOK: false},
		{name: "a bare prefix and nothing else", arn: "arn:aws:sns:", service: "sns", wantOK: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := awsclient.ARNForService(tc.arn, tc.service)
			if ok != tc.wantOK {
				t.Fatalf("ARNForService(%q,%q) ok = %v, want %v", tc.arn, tc.service, ok, tc.wantOK)
			}
			if ok && got.Resource != tc.wantResourc {
				t.Errorf("Resource = %q, want %q", got.Resource, tc.wantResourc)
			}
			if ok && got.Service != tc.service {
				t.Errorf("Service = %q, want %q", got.Service, tc.service)
			}
		})
	}
}

// TestAlarmSNS_TargetsInEveryPartitionAreCounted drives the alarm→sns pivot,
// which reads the topic ARNs straight off the alarm AWS returned. A partition
// prefix drops all three action lists at once, and the panel then says zero
// rather than "I could not tell".
func TestAlarmSNS_TargetsInEveryPartitionAreCounted(t *testing.T) {
	checker := parseCheckerFor(t, "alarm", "sns")
	tests := []struct {
		name    string
		actions []string
		wantIDs int
	}{
		{name: "commercial topic", actions: []string{"arn:aws:sns:us-east-1:123456789012:acme-alerts"}, wantIDs: 1},
		{name: "China topic", actions: []string{"arn:aws-cn:sns:cn-north-1:123456789012:acme-alerts"}, wantIDs: 1},
		{name: "GovCloud topic", actions: []string{"arn:aws-us-gov:sns:us-gov-west-1:123456789012:acme-alerts"}, wantIDs: 1},
		{name: "an autoscaling action is not a topic", actions: []string{"arn:aws-cn:autoscaling:cn-north-1:123456789012:scalingPolicy:abc"}, wantIDs: 0},
		{name: "a malformed action is not a topic", actions: []string{"acme-alerts"}, wantIDs: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := resource.Resource{ID: "acme-cpu-high", RawStruct: cwtypes.MetricAlarm{
				AlarmName:    aws.String("acme-cpu-high"),
				AlarmActions: tc.actions,
			}}
			got := checker(context.Background(), parseClients("us-east-1"), res, resource.ResourceCache{})
			if got.Count() != tc.wantIDs {
				t.Errorf("Count = %d, want %d (ids %v)", got.Count(), tc.wantIDs, got.ResourceIDs())
			}
		})
	}
}

// TestLambdaSecrets_SecretARNsInEveryPartitionAreCounted drives the
// lambda→secrets pivot, which scans environment-variable values for secret
// ARNs. The secret has to be in the cache for the pivot to resolve it, so the
// cache carries it under the name the ARN's resource segment ends with.
func TestLambdaSecrets_SecretARNsInEveryPartitionAreCounted(t *testing.T) {
	checker := parseCheckerFor(t, "lambda", "secrets")
	const secretID = "acme/db-AbCdEf"
	tests := []struct {
		name      string
		value     string
		wantCount int
	}{
		{name: "commercial secret", value: "arn:aws:secretsmanager:us-east-1:123456789012:secret:" + secretID, wantCount: 1},
		{name: "China secret", value: "arn:aws-cn:secretsmanager:cn-north-1:123456789012:secret:" + secretID, wantCount: 1},
		{name: "GovCloud secret", value: "arn:aws-us-gov:secretsmanager:us-gov-west-1:123456789012:secret:" + secretID, wantCount: 1},
		{name: "an ssm parameter is not a secret", value: "arn:aws-cn:ssm:cn-north-1:123456789012:parameter/acme/db", wantCount: 0},
		{name: "a plain value is not a secret", value: "postgres://localhost", wantCount: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := resource.Resource{ID: "acme-fn", RawStruct: lambdatypes.FunctionConfiguration{
				FunctionName: aws.String("acme-fn"),
				Environment:  &lambdatypes.EnvironmentResponse{Variables: map[string]string{"DB_SECRET": tc.value}},
			}}
			// The secrets cache carries the secret under the ARN of the
			// partition it was fetched from, which is the session's own.
			cache := resource.ResourceCache{"secrets": resource.ResourceCacheEntry{
				Resources: []resource.Resource{{
					ID: secretID, Name: "acme/db",
					Fields: map[string]string{"arn": tc.value},
				}},
			}}
			got := checker(context.Background(), parseClients("us-east-1"), res, cache)
			if got.Count() != tc.wantCount {
				t.Errorf("Count = %d, want %d (ids %v)", got.Count(), tc.wantCount, got.ResourceIDs())
			}
		})
	}
}

// TestMWAAS3_BucketNameIsUnchangedAndEveryPartitionResolves pins the second
// of the two sites that extract a NAME from a matched ARN. The extracted
// value must not move for an arn:aws: input — it is the id the pivot drills
// into — and a China ARN must yield the same bare bucket rather than the
// whole ARN, which is what a failed prefix trim leaves behind.
func TestMWAAS3_BucketNameIsUnchangedAndEveryPartitionResolves(t *testing.T) {
	checker := parseCheckerFor(t, "mwaa", "s3")
	tests := []struct {
		name    string
		arn     string
		wantIDs []string
	}{
		{name: "commercial bucket", arn: "arn:aws:s3:::acme-airflow-dags", wantIDs: []string{"acme-airflow-dags"}},
		{name: "China bucket", arn: "arn:aws-cn:s3:::acme-airflow-dags", wantIDs: []string{"acme-airflow-dags"}},
		{name: "GovCloud bucket", arn: "arn:aws-us-gov:s3:::acme-airflow-dags", wantIDs: []string{"acme-airflow-dags"}},
		// A value that is not an S3 ARN names no bucket. Trimming a prefix
		// that is not there leaves the whole string and drills into nothing.
		{name: "not an S3 ARN", arn: "arn:aws:sqs:us-east-1:123456789012:acme-queue", wantIDs: nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := resource.Resource{ID: "acme-airflow", RawStruct: mwaatypes.Environment{
				Name:            aws.String("acme-airflow"),
				SourceBucketArn: aws.String(tc.arn),
			}}
			got := checker(context.Background(), parseClients("us-east-1"), res, resource.ResourceCache{})
			if got.Count() != len(tc.wantIDs) {
				t.Fatalf("Count = %d, want %d (ids %v)", got.Count(), len(tc.wantIDs), got.ResourceIDs())
			}
			for i, want := range tc.wantIDs {
				if got.ResourceIDs()[i] != want {
					t.Errorf("ResourceIDs()[%d] = %q, want %q", i, got.ResourceIDs()[i], want)
				}
			}
		})
	}
}

// TestAdminAttachedPolicy_IsRecognisedInEveryPartition pins the admin-policy
// match through the IAM role enricher. The attached policy ARN is AWS's, so
// in China it reads arn:aws-cn:iam::aws:policy/AdministratorAccess and a
// literal list of commercial ARNs silently reports the role unprivileged.
func TestAdminAttachedPolicy_IsRecognisedInEveryPartition(t *testing.T) {
	tests := []struct {
		name        string
		policyARN   string
		wantFinding bool
	}{
		{name: "commercial AdministratorAccess", policyARN: "arn:aws:iam::aws:policy/AdministratorAccess", wantFinding: true},
		{name: "China AdministratorAccess", policyARN: "arn:aws-cn:iam::aws:policy/AdministratorAccess", wantFinding: true},
		{name: "GovCloud PowerUserAccess", policyARN: "arn:aws-us-gov:iam::aws:policy/PowerUserAccess", wantFinding: true},
		{name: "a customer policy of the same name is not the AWS one", policyARN: "arn:aws:iam::123456789012:policy/AdministratorAccess", wantFinding: false},
		{name: "a read-only AWS policy", policyARN: "arn:aws:iam::aws:policy/ReadOnlyAccess", wantFinding: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := awsclient.AdminAttachedPolicyNameForTest([]iamtypes.AttachedPolicy{{
				PolicyArn:  aws.String(tc.policyARN),
				PolicyName: aws.String("AdministratorAccess"),
			}})
			if (got != "") != tc.wantFinding {
				t.Errorf("AdminAttachedPolicyName(%q) = %q, want match = %v", tc.policyARN, got, tc.wantFinding)
			}
		})
	}
}

// ── Row 5: the managed-policy ARN a9s builds ──────────────────────────────

// parseIAMPolicyFake records every ARN GetPolicy is asked about and resolves
// exactly one of them, so the test can assert both what was tried and that
// resolution still works.
type parseIAMPolicyFake struct {
	awsclient.IAMAPI
	asked   []string
	resolve string
}

func (f *parseIAMPolicyFake) ListPolicies(_ context.Context, _ *iam.ListPoliciesInput, _ ...func(*iam.Options)) (*iam.ListPoliciesOutput, error) {
	return &iam.ListPoliciesOutput{}, nil
}

func (f *parseIAMPolicyFake) ListGroups(_ context.Context, _ *iam.ListGroupsInput, _ ...func(*iam.Options)) (*iam.ListGroupsOutput, error) {
	return &iam.ListGroupsOutput{}, nil
}

func (f *parseIAMPolicyFake) GetPolicy(_ context.Context, in *iam.GetPolicyInput, _ ...func(*iam.Options)) (*iam.GetPolicyOutput, error) {
	got := aws.ToString(in.PolicyArn)
	f.asked = append(f.asked, got)
	if got != f.resolve {
		return nil, errors.New("NoSuchEntity: policy does not exist")
	}
	return &iam.GetPolicyOutput{Policy: &iamtypes.Policy{
		PolicyName: aws.String("AdministratorAccess"),
		Arn:        aws.String(got),
		Path:       aws.String("/"),
	}}, nil
}

// TestManagedPolicyLookup_ARNCarriesTheSessionPartition pins the ARN the
// lazy-add builds when a checker names an AWS-managed policy. It is a
// construction, not a match: GetPolicy against a partition the session cannot
// see returns NoSuchEntity for every one of the four paths, and the related
// panel then shows the failure instead of the policy.
func TestManagedPolicyLookup_ARNCarriesTheSessionPartition(t *testing.T) {
	tests := []struct {
		region  string
		wantARN string
	}{
		{region: "us-east-1", wantARN: "arn:aws:iam::aws:policy/AdministratorAccess"},
		{region: parseCNRegion, wantARN: "arn:aws-cn:iam::aws:policy/AdministratorAccess"},
		{region: parseGovRegion, wantARN: "arn:aws-us-gov:iam::aws:policy/AdministratorAccess"},
	}
	for _, tc := range tests {
		t.Run(tc.region, func(t *testing.T) {
			fake := &parseIAMPolicyFake{resolve: tc.wantARN}
			clients := parseClients(tc.region)
			clients.IAM = fake
			clients.SetIAMPolicies(session.NewPolicyStore())

			got, err := awsclient.FetchIAMPoliciesByIDsFull(
				context.Background(), clients.IAM, []string{"AdministratorAccess"},
				clients.IAMPolicies(), awsclient.PartitionForRegion(tc.region))
			if err != nil {
				t.Fatalf("FetchIAMPoliciesByIDsFull: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("resolved %d policies, want 1 (asked %v)", len(got), fake.asked)
			}
			if got[0].Fields["arn"] != tc.wantARN {
				t.Errorf("Fields[arn] = %q, want %q", got[0].Fields["arn"], tc.wantARN)
			}
			for _, asked := range fake.asked {
				if _, err := arn.Parse(asked); err != nil {
					t.Errorf("GetPolicy asked about %q, which does not parse: %v", asked, err)
				}
			}
		})
	}
}

// ── Row 6: only an unassociated elastic IP proves a record dangles ────────

// parseR53Fake serves one zone's records.
type parseR53Fake struct {
	awsclient.Route53API
	records []r53types.ResourceRecordSet
}

func (f *parseR53Fake) GetHostedZone(_ context.Context, in *route53.GetHostedZoneInput, _ ...func(*route53.Options)) (*route53.GetHostedZoneOutput, error) {
	return &route53.GetHostedZoneOutput{HostedZone: &r53types.HostedZone{
		Id:     in.Id,
		Name:   aws.String("acme-corp.com."),
		Config: &r53types.HostedZoneConfig{PrivateZone: false},
	}}, nil
}

func (f *parseR53Fake) ListQueryLoggingConfigs(_ context.Context, _ *route53.ListQueryLoggingConfigsInput, _ ...func(*route53.Options)) (*route53.ListQueryLoggingConfigsOutput, error) {
	return &route53.ListQueryLoggingConfigsOutput{QueryLoggingConfigs: []r53types.QueryLoggingConfig{{
		Id: aws.String("qlc-0000000000000000"),
	}}}, nil
}

func (f *parseR53Fake) ListResourceRecordSets(_ context.Context, _ *route53.ListResourceRecordSetsInput, _ ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error) {
	return &route53.ListResourceRecordSetsOutput{ResourceRecordSets: f.records}, nil
}

// parseAddrCache builds the eip and ec2 caches the ownership verdict reads.
func parseAddrCache(eips []resource.Resource, instances []resource.Resource) resource.ResourceCache {
	return resource.ResourceCache{
		"eip": resource.ResourceCacheEntry{Resources: eips},
		"ec2": resource.ResourceCacheEntry{Resources: instances},
	}
}

func parseEIP(ip, status string) resource.Resource {
	return resource.Resource{ID: "eipalloc-" + ip, Fields: map[string]string{"public_ip": ip, "status": status}}
}

func parseInstance(ip string) resource.Resource {
	return resource.Resource{ID: "i-0a1b2c3d4e5f60001", Fields: map[string]string{"public_ip": ip}}
}

// TestR53AddressOwnership_AnswersOnlyWhatTheInventoryProves pins the closed
// vocabulary. The account's own inventory can say an address is held and can
// say an elastic IP it holds has nothing behind it. It cannot say an address
// it has never seen was once this account's, so that address is outside the
// account's knowledge and not a released one.
func TestR53AddressOwnership_AnswersOnlyWhatTheInventoryProves(t *testing.T) {
	const attached = "203.0.113.10"
	const unattached = "203.0.113.20"
	const onInstance = "203.0.113.30"
	const foreign = "198.51.100.50"

	full := parseAddrCache(
		[]resource.Resource{parseEIP(attached, "ATTACHED"), parseEIP(unattached, "UNATTACHED")},
		[]resource.Resource{parseInstance(onInstance)},
	)

	tests := []struct {
		name  string
		addr  string
		cache resource.ResourceCache
		want  string
	}{
		{name: "an attached elastic IP", addr: attached, cache: full, want: awsclient.R53AddrHeld},
		{name: "an address on a running instance", addr: onInstance, cache: full, want: awsclient.R53AddrHeld},
		{name: "an unassociated elastic IP", addr: unattached, cache: full, want: awsclient.R53AddrUnattached},
		{name: "an address this account has never held", addr: foreign, cache: full, want: awsclient.R53AddrOutside},
		{
			name:  "the elastic IP inventory is missing",
			addr:  attached,
			cache: resource.ResourceCache{"ec2": resource.ResourceCacheEntry{}},
			want:  awsclient.R53AddrUnknown,
		},
		{
			name:  "the elastic IP inventory is truncated",
			addr:  attached,
			cache: resource.ResourceCache{"eip": resource.ResourceCacheEntry{IsTruncated: true}, "ec2": resource.ResourceCacheEntry{}},
			want:  awsclient.R53AddrUnknown,
		},
		{
			// The eni cache is no longer consulted, so its absence must not
			// void a verdict the eip and ec2 caches already prove.
			name:  "the network interface inventory is absent",
			addr:  unattached,
			cache: full,
			want:  awsclient.R53AddrUnattached,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := awsclient.R53AddressOwnership(tc.addr, tc.cache); got != tc.want {
				t.Errorf("R53AddressOwnership(%q) = %q, want %q", tc.addr, got, tc.want)
			}
		})
	}
}

// TestR53DanglingRecord_OnlyAnUnassociatedElasticIPRaisesIt drives the
// enricher. A broken finding is a claim that something is unreachable, and the
// only address this account can prove unreachable is one it holds with nothing
// behind it. A record pointing at a CDN or a partner account is not evidence
// of anything, so it renders nothing at all rather than a warning that would
// light the badge on every real public zone.
func TestR53DanglingRecord_OnlyAnUnassociatedElasticIPRaisesIt(t *testing.T) {
	const zoneID = "Z0PARSEEXAMPLE00001"
	const code = awsclient.CodeR53DanglingRecord
	const unattached = "203.0.113.20"
	const attached = "203.0.113.10"
	const foreign = "198.51.100.50"

	cache := parseAddrCache(
		[]resource.Resource{parseEIP(attached, "ATTACHED"), parseEIP(unattached, "UNATTACHED")},
		nil,
	)

	rec := func(name, addr string) r53types.ResourceRecordSet {
		return r53types.ResourceRecordSet{
			Name: aws.String(name), Type: r53types.RRTypeA, TTL: aws.Int64(300),
			ResourceRecords: []r53types.ResourceRecord{{Value: aws.String(addr)}},
		}
	}

	tests := []struct {
		name        string
		records     []r53types.ResourceRecordSet
		wantFinding bool
	}{
		{name: "an unassociated elastic IP", records: []r53types.ResourceRecordSet{rec("gone.acme-corp.com.", unattached)}, wantFinding: true},
		{name: "an attached elastic IP", records: []r53types.ResourceRecordSet{rec("api.acme-corp.com.", attached)}},
		{name: "a content delivery network", records: []r53types.ResourceRecordSet{rec("cdn.acme-corp.com.", foreign)}},
		{name: "a partner account's address", records: []r53types.ResourceRecordSet{rec("partner.acme-corp.com.", "198.51.100.77")}},
		{name: "no records at all", records: nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clients := parseClients("us-east-1")
			clients.Route53 = &parseR53Fake{records: tc.records}
			res, err := awsclient.EnrichRoute53Zone(context.Background(), clients,
				[]resource.Resource{{ID: zoneID, Fields: map[string]string{"zone_id": zoneID, "type": "public"}}},
				cache)
			if err != nil {
				t.Fatalf("EnrichRoute53Zone: %v", err)
			}
			if !tc.wantFinding {
				w4AssertNoCode(t, res.Findings[zoneID], code)
				return
			}
			w4AssertFinding(t, res.Findings[zoneID], code,
				"record points at an unassociated elastic IP", domain.SevBroken, "wave2")
		})
	}
}

// ── Row 7: a bucket is missing only when something authoritative says so ──

// parseHeadBucketFake answers HeadBucket per bucket name: nil for a bucket
// that exists, NotFound for one that does not, and a permissions error for a
// bucket that exists in an account this session cannot read.
type parseHeadBucketFake struct {
	awsclient.S3API
	notFound map[string]bool
	denied   map[string]bool
	calls    int
}

func (f *parseHeadBucketFake) HeadBucket(_ context.Context, in *s3.HeadBucketInput, _ ...func(*s3.Options)) (*s3.HeadBucketOutput, error) {
	f.calls++
	name := aws.ToString(in.Bucket)
	if f.notFound[name] {
		return nil, &s3types.NotFound{}
	}
	if f.denied[name] {
		return nil, errors.New("AccessDenied: User is not authorized to perform s3:ListBucket")
	}
	return &s3.HeadBucketOutput{}, nil
}

// TestCloudFrontOriginBucket_MissingIsAssertedOnlyOnProof pins when the
// missing-origin finding may fire. Absence from this account's bucket list is
// not evidence about a bucket in another account, and a cross-account origin
// with a bucket policy is the ordinary way to serve a shared bucket through
// CloudFront. Reporting it deleted sends an operator to look for a bucket
// that is working.
func TestCloudFrontOriginBucket_MissingIsAssertedOnlyOnProof(t *testing.T) {
	const distID = "E1PARSEEXAMPLE"
	const missing = awsclient.CodeCFOriginBucketMissing

	tests := []struct {
		name        string
		bucket      string
		inCache     bool
		notFound    bool
		denied      bool
		wantMissing bool
	}{
		{name: "a bucket in our account", bucket: "acme-ours", inCache: true},
		{name: "a bucket in our account that no longer exists", bucket: "acme-gone", notFound: true, wantMissing: true},
		{name: "a cross-account bucket this session may not read", bucket: "partner-assets", denied: true},
		{name: "a cross-account bucket this session may read", bucket: "partner-open"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buckets []resource.Resource
			if tc.inCache {
				buckets = []resource.Resource{{ID: tc.bucket, Name: tc.bucket}}
			}
			cache := resource.ResourceCache{"s3": resource.ResourceCacheEntry{Resources: buckets}}
			head := &parseHeadBucketFake{
				notFound: map[string]bool{tc.bucket: tc.notFound},
				denied:   map[string]bool{tc.bucket: tc.denied},
			}
			host := tc.bucket + ".s3.us-east-1.amazonaws.com"
			fake := &cfGetDistributionConfigFake{results: map[string]*cftypes.DistributionConfig{
				distID: {
					Comment: aws.String("acme cdn"),
					Origins: &cftypes.Origins{Quantity: aws.Int32(1), Items: []cftypes.Origin{parseCFOrigin(host, true)}},
					DefaultCacheBehavior: &cftypes.DefaultCacheBehavior{
						TargetOriginId:       aws.String("origin-" + host),
						ViewerProtocolPolicy: cftypes.ViewerProtocolPolicyRedirectToHttps,
					},
				},
			}}
			clients := parseClients("us-east-1")
			clients.CloudFront = fake
			clients.S3 = head

			res, err := awsclient.EnrichCloudFrontDistribution(context.Background(), clients, cfDistroResources(distID), cache)
			if err != nil {
				t.Fatalf("EnrichCloudFrontDistribution: %v", err)
			}
			if tc.wantMissing {
				w4AssertFinding(t, res.Findings[distID], missing,
					"S3 origin bucket does not exist", domain.SevBroken, "wave2")
			} else {
				w4AssertNoCode(t, res.Findings[distID], missing)
			}
			// A bucket the cache already accounts for needs no call.
			if tc.inCache && head.calls != 0 {
				t.Errorf("HeadBucket called %d times for a cached bucket, want 0", head.calls)
			}
		})
	}
}

// TestCloudFrontOriginBucket_WithoutAnAuthoritativeCheckNothingIsAsserted
// pins the session that cannot ask. A9s reaching no S3 client has no way to
// tell a deleted bucket from a cross-account one, and an unanswerable
// question renders as no finding rather than the worse of the two answers.
func TestCloudFrontOriginBucket_WithoutAnAuthoritativeCheckNothingIsAsserted(t *testing.T) {
	const distID = "E1PARSEEXAMPLE"
	const host = "somebody-elses.s3.us-east-1.amazonaws.com"

	fake := &cfGetDistributionConfigFake{results: map[string]*cftypes.DistributionConfig{
		distID: {
			Comment: aws.String("acme cdn"),
			Origins: &cftypes.Origins{Quantity: aws.Int32(1), Items: []cftypes.Origin{parseCFOrigin(host, true)}},
			DefaultCacheBehavior: &cftypes.DefaultCacheBehavior{
				TargetOriginId:       aws.String("origin-" + host),
				ViewerProtocolPolicy: cftypes.ViewerProtocolPolicyRedirectToHttps,
			},
		},
	}}
	clients := parseClients("us-east-1")
	clients.CloudFront = fake

	res, err := awsclient.EnrichCloudFrontDistribution(context.Background(), clients,
		cfDistroResources(distID),
		resource.ResourceCache{"s3": resource.ResourceCacheEntry{Resources: []resource.Resource{{ID: "acme-ours"}}}})
	if err != nil {
		t.Fatalf("EnrichCloudFrontDistribution: %v", err)
	}
	w4AssertNoCode(t, res.Findings[distID], awsclient.CodeCFOriginBucketMissing)
}

// ── Row 8: TCP on 443 is passthrough, not plaintext ───────────────────────

// TestELBListenerIsPlaintext_TheProtocolAndThePortDecide pins the classifier.
// A network load balancer forwarding TCP does not read what it forwards, so
// what crosses the wire is decided by what the client speaks on that port —
// 443 is the TLS port and the session terminates on the target.
func TestELBListenerIsPlaintext_TheProtocolAndThePortDecide(t *testing.T) {
	tests := []struct {
		proto elbtypes.ProtocolEnum
		port  int32
		want  bool
	}{
		{proto: elbtypes.ProtocolEnumHttp, port: 80, want: true},
		{proto: elbtypes.ProtocolEnumHttp, port: 8080, want: true},
		{proto: elbtypes.ProtocolEnumTcp, port: 80, want: true},
		{proto: elbtypes.ProtocolEnumTcp, port: 8080, want: true},
		{proto: elbtypes.ProtocolEnumTcp, port: 443, want: false},
		{proto: elbtypes.ProtocolEnumTcp, port: 5432, want: false},
		{proto: elbtypes.ProtocolEnumTls, port: 443, want: false},
		{proto: elbtypes.ProtocolEnumHttps, port: 443, want: false},
		{proto: elbtypes.ProtocolEnumUdp, port: 53, want: false},
		{proto: elbtypes.ProtocolEnumTcpUdp, port: 443, want: false},
	}
	for _, tc := range tests {
		t.Run(string(tc.proto)+"/"+strconv.Itoa(int(tc.port)), func(t *testing.T) {
			if got := awsclient.ELBListenerIsPlaintext(tc.proto, tc.port); got != tc.want {
				t.Errorf("ELBListenerIsPlaintext(%s,%d) = %v, want %v", tc.proto, tc.port, got, tc.want)
			}
		})
	}
}

// TestELBPlainListener_TLSPassthroughIsNotAnExposure drives the enricher. A
// TLS-passthrough network load balancer is a deliberate design — the
// certificate lives on the target — and warning about it trains an operator
// to ignore the row that matters.
func TestELBPlainListener_TLSPassthroughIsNotAnExposure(t *testing.T) {
	const code = domain.FindingCode("elb.plain-http-listener")

	tests := []struct {
		name        string
		lbType      string
		listeners   []elbtypes.Listener
		wantFinding bool
	}{
		{
			name:      "a network balancer passing TLS through on 443",
			lbType:    "network",
			listeners: []elbtypes.Listener{w3Listener("aa", 443, elbtypes.ProtocolEnumTcp, "", nil)},
		},
		{
			name:        "a network balancer carrying plain traffic on 80",
			lbType:      "network",
			listeners:   []elbtypes.Listener{w3Listener("bb", 80, elbtypes.ProtocolEnumTcp, "", nil)},
			wantFinding: true,
		},
		{
			name:        "a network balancer carrying plain traffic on 8080",
			lbType:      "network",
			listeners:   []elbtypes.Listener{w3Listener("cc", 8080, elbtypes.ProtocolEnumTcp, "", nil)},
			wantFinding: true,
		},
		{
			name:      "a network balancer forwarding a database port",
			lbType:    "network",
			listeners: []elbtypes.Listener{w3Listener("dd", 5432, elbtypes.ProtocolEnumTcp, "", nil)},
		},
		{
			name:      "a network balancer terminating TLS",
			lbType:    "network",
			listeners: []elbtypes.Listener{w3Listener("ee", 443, elbtypes.ProtocolEnumTls, "ELBSecurityPolicy-TLS13-1-2-2021-06", nil)},
		},
		{
			name:        "an application balancer serving plain HTTP",
			lbType:      "application",
			listeners:   []elbtypes.Listener{w3Listener("ff", 80, elbtypes.ProtocolEnumHttp, "", nil)},
			wantFinding: true,
		},
		{
			name:   "an application balancer redirecting to HTTPS",
			lbType: "application",
			listeners: []elbtypes.Listener{w3Listener("gg", 80, elbtypes.ProtocolEnumHttp, "", []elbtypes.Action{{
				Type:           elbtypes.ActionTypeEnumRedirect,
				RedirectConfig: &elbtypes.RedirectActionConfig{Protocol: aws.String("HTTPS")},
			}})},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lb := w3ELBRes("acme-lb", tc.lbType)
			fake := w3NewELBFake()
			fake.attrs[lb.Fields["load_balancer_arn"]] = w3ELBAttrs(nil)
			fake.listeners[lb.Fields["load_balancer_arn"]] = tc.listeners

			clients := parseClients("us-east-1")
			clients.ELBv2 = fake
			res, err := awsclient.EnrichELBAttributes(context.Background(), clients, []resource.Resource{lb}, nil)
			if err != nil {
				t.Fatalf("EnrichELBAttributes: %v", err)
			}
			if tc.wantFinding {
				if !hasCode(res.Findings[lb.ID], code) {
					t.Errorf("no %s finding; findings = %v", code, res.Findings[lb.ID])
				}
				return
			}
			w4AssertNoCode(t, res.Findings[lb.ID], code)
		})
	}
}

func hasCode(fs []domain.Finding, code domain.FindingCode) bool {
	for _, f := range fs {
		if f.Code == code {
			return true
		}
	}
	return false
}

// ── Row 4, the two sites the first sweep missed ───────────────────────────

// parseBucketPolicyFake serves one bucket policy.
type parseBucketPolicyFake struct {
	awsclient.S3API
	policy string
}

func (f *parseBucketPolicyFake) GetBucketPolicy(_ context.Context, _ *s3.GetBucketPolicyInput, _ ...func(*s3.Options)) (*s3.GetBucketPolicyOutput, error) {
	return &s3.GetBucketPolicyOutput{Policy: aws.String(f.policy)}, nil
}

// TestS3BucketPolicyRoles_RolePrincipalsInEveryPartitionAreCounted pins the
// bucket→roles pivot. The principals come out of a policy document AWS
// returned, so their partition is the account's own; testing for a literal
// commercial prefix leaves a China or GovCloud operator looking at a bucket
// whose cross-account grants render as none.
func TestS3BucketPolicyRoles_RolePrincipalsInEveryPartitionAreCounted(t *testing.T) {
	checker := parseCheckerFor(t, "s3", "role")
	const roleName = "acme-reader"
	roleCache := resource.ResourceCache{"role": resource.ResourceCacheEntry{
		Resources: []resource.Resource{{ID: roleName, Name: roleName}},
	}}

	policy := func(principal string) string {
		return `{"Version":"2012-10-17","Statement":[{"Effect":"Allow",` +
			`"Principal":{"AWS":"` + principal + `"},` +
			`"Action":"s3:GetObject","Resource":"arn:aws:s3:::acme-assets/*"}]}`
	}

	tests := []struct {
		name      string
		principal string
		wantCount int
	}{
		{name: "commercial role", principal: "arn:aws:iam::123456789012:role/" + roleName, wantCount: 1},
		{name: "China role", principal: "arn:aws-cn:iam::123456789012:role/" + roleName, wantCount: 1},
		{name: "GovCloud role", principal: "arn:aws-us-gov:iam::123456789012:role/" + roleName, wantCount: 1},
		{name: "a role under a path", principal: "arn:aws-cn:iam::123456789012:role/service-role/" + roleName, wantCount: 1},
		// The role pivot surfaces roles. A user, the account root and a
		// wildcard are principals too, and none of them is one.
		{name: "a user is not a role", principal: "arn:aws-cn:iam::123456789012:user/" + roleName},
		{name: "the account root is not a role", principal: "arn:aws-cn:iam::123456789012:root"},
		{name: "a wildcard is not a role", principal: "*"},
		{name: "an assumed-role session is not an iam role ARN", principal: "arn:aws-cn:sts::123456789012:assumed-role/" + roleName + "/sess"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clients := parseClients("us-east-1")
			clients.S3 = &parseBucketPolicyFake{policy: policy(tc.principal)}
			got := checker(context.Background(), clients,
				resource.Resource{ID: "acme-assets", Name: "acme-assets"}, roleCache)
			if got.Count() != tc.wantCount {
				t.Errorf("Count = %d, want %d (ids %v)", got.Count(), tc.wantCount, got.ResourceIDs())
			}
		})
	}
}

// TestGlueSecrets_SecretARNsInEveryPartitionAreCounted pins the fourth site of
// the secret-ARN question, the one the first sweep left behind while
// converting the other three. A Glue job names its secrets in the job
// arguments AWS returned.
func TestGlueSecrets_SecretARNsInEveryPartitionAreCounted(t *testing.T) {
	checker := parseCheckerFor(t, "glue", "secrets")
	const secretName = "acme/db-AbCdEf"

	tests := []struct {
		name    string
		value   string
		wantIDs []string
	}{
		{name: "commercial secret", value: "arn:aws:secretsmanager:us-east-1:123456789012:secret:" + secretName, wantIDs: []string{secretName}},
		{name: "China secret", value: "arn:aws-cn:secretsmanager:cn-north-1:123456789012:secret:" + secretName, wantIDs: []string{secretName}},
		{name: "GovCloud secret", value: "arn:aws-us-gov:secretsmanager:us-gov-west-1:123456789012:secret:" + secretName, wantIDs: []string{secretName}},
		{name: "an ssm parameter is not a secret", value: "arn:aws-cn:ssm:cn-north-1:123456789012:parameter/acme/db"},
		{name: "a plain argument is not a secret", value: "--enable-metrics"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := resource.Resource{ID: "acme-etl-job", RawStruct: gluetypes.Job{
				Name:             aws.String("acme-etl-job"),
				DefaultArguments: map[string]string{"--db-secret": tc.value},
			}}
			got := checker(context.Background(), parseClients("us-east-1"), res, resource.ResourceCache{})
			if got.Count() != len(tc.wantIDs) {
				t.Fatalf("Count = %d, want %d (ids %v)", got.Count(), len(tc.wantIDs), got.ResourceIDs())
			}
			for i, want := range tc.wantIDs {
				if got.ResourceIDs()[i] != want {
					t.Errorf("ResourceIDs()[%d] = %q, want %q", i, got.ResourceIDs()[i], want)
				}
			}
		})
	}
}

// TestNavIDFromValue_S3BucketARNResolvesInEveryPartition pins the third site
// the mechanical sweep found, in core/resource rather than core/aws. It turns
// a field value into the id Enter navigates to, and an S3 bucket ARN it fails
// to recognise is passed through whole — so the drill-in looks for a bucket
// named by its own ARN and lands nowhere.
func TestNavIDFromValue_S3BucketARNResolvesInEveryPartition(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "commercial bucket ARN", value: "arn:aws:s3:::acme-assets", want: "acme-assets"},
		{name: "China bucket ARN", value: "arn:aws-cn:s3:::acme-assets", want: "acme-assets"},
		{name: "GovCloud bucket ARN", value: "arn:aws-us-gov:s3:::acme-assets", want: "acme-assets"},
		// A bare bucket name is the common case and must pass through.
		{name: "a bare bucket name", value: "acme-assets", want: "acme-assets"},
		// Anything that is not an S3 bucket ARN is not a bucket name to
		// rewrite, so it passes through for the caller to fall back on.
		{name: "another service's ARN", value: "arn:aws-cn:sqs:cn-north-1:123456789012:acme-queue", want: "arn:aws-cn:sqs:cn-north-1:123456789012:acme-queue"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := resource.NavIDFromValue("s3", tc.value); got != tc.want {
				t.Errorf("NavIDFromValue(s3, %q) = %q, want %q", tc.value, got, tc.want)
			}
		})
	}
}
