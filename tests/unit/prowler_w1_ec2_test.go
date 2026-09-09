package unit

// prowler_w1_ec2_test.go — behavioural pins for the ec2 posture signals of
// batch w1 (Prowler gap closure): imdsv1-allowed, public-ip,
// internet-exposed and user-data-secret.
//
// Wave 1 (imdsv1-allowed, public-ip) is asserted through
// FetchEC2InstancesPage so the tests exercise the shape the real
// DescribeInstances response has. Wave 2 (internet-exposed,
// user-data-secret) is asserted through EnrichEC2InstanceStatus, the type's
// single Wave-2 enricher.

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	pw1EC2CodeIMDSv1          = domain.FindingCode("ec2.imdsv1-allowed")
	pw1EC2CodePublicIP        = domain.FindingCode("ec2.public-ip")
	pw1EC2CodeInternetExposed = domain.FindingCode("ec2.internet-exposed")
	pw1EC2CodeUserDataSecret  = domain.FindingCode("ec2.user-data-secret")
)

// ─── shared helpers ─────────────────────────────────────────────────────────

// pw1FindFinding returns the finding carrying code, or (zero, false).
func pw1FindFinding(findings []domain.Finding, code domain.FindingCode) (domain.Finding, bool) {
	for _, f := range findings {
		if f.Code == code {
			return f, true
		}
	}
	return domain.Finding{}, false
}

// pw1RequireFinding fails unless exactly one finding carries code with the
// given phrase, severity and source, and a non-empty Detail sentence.
func pw1RequireFinding(t *testing.T, findings []domain.Finding, code domain.FindingCode, phrase string, sev domain.Severity, source string) domain.Finding {
	t.Helper()
	n := 0
	for _, f := range findings {
		if f.Code == code {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("want exactly 1 finding with code %q, got %d (all: %+v)", code, n, findings)
	}
	f, _ := pw1FindFinding(findings, code)
	if f.Phrase != phrase {
		t.Errorf("Phrase = %q, want %q", f.Phrase, phrase)
	}
	if f.Severity != sev {
		t.Errorf("Severity = %v, want %v", f.Severity, sev)
	}
	if f.Source != source {
		t.Errorf("Source = %q, want %q", f.Source, source)
	}
	if strings.TrimSpace(f.Detail) == "" {
		t.Errorf("Detail is empty; every posture finding stamps an operator sentence")
	}
	return f
}

// pw1RequireNoFinding fails when any finding carries code.
func pw1RequireNoFinding(t *testing.T, findings []domain.Finding, code domain.FindingCode) {
	t.Helper()
	if f, ok := pw1FindFinding(findings, code); ok {
		t.Fatalf("unexpected finding %q emitted: %+v", code, f)
	}
}

// pw1Rows returns the AttentionDetail rows recorded for (id, code).
func pw1Rows(res awsclient.IssueEnricherResult, id string, code domain.FindingCode) []domain.DetailRow {
	byCode, ok := res.AttentionDetails[id]
	if !ok {
		return nil
	}
	return byCode[code].Rows
}

// pw1RequireRow fails unless the rows carry exactly one Label:Value pair.
func pw1RequireRow(t *testing.T, rows []domain.DetailRow, label, value string) {
	t.Helper()
	for _, r := range rows {
		if r.Label == label && r.Value == value {
			return
		}
	}
	t.Fatalf("missing AttentionDetail row %q: %q (rows: %+v)", label, value, rows)
}

// ─── ec2 fakes ──────────────────────────────────────────────────────────────

// pw1EC2ListFake serves DescribeInstances/DescribeInstanceStatus for the
// wave-1 fetcher path.
type pw1EC2ListFake struct {
	instances []ec2types.Instance
}

func (f *pw1EC2ListFake) DescribeInstances(_ context.Context, _ *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	return &ec2.DescribeInstancesOutput{
		Reservations: []ec2types.Reservation{{Instances: f.instances}},
	}, nil
}

func (f *pw1EC2ListFake) DescribeInstanceStatus(_ context.Context, _ *ec2.DescribeInstanceStatusInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstanceStatusOutput, error) {
	return &ec2.DescribeInstanceStatusOutput{}, nil
}

// pw1EC2EnrichFake serves the Wave-2 enricher: no status findings, and a
// per-instance user-data attribute keyed by instance ID.
type pw1EC2EnrichFake struct {
	awsclient.EC2API
	userData map[string]string // instance ID → raw (undecoded) script
	attrErr  map[string]error  // instance ID → DescribeInstanceAttribute error

	// The enricher fans this fake out through ForEachParallel, so every
	// recorded call is written from a different goroutine.
	mu          sync.Mutex
	attrCalls   []string
	statusPages int
}

func (f *pw1EC2EnrichFake) DescribeInstanceStatus(_ context.Context, _ *ec2.DescribeInstanceStatusInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstanceStatusOutput, error) {
	f.mu.Lock()
	f.statusPages++
	f.mu.Unlock()
	return &ec2.DescribeInstanceStatusOutput{}, nil
}

func (f *pw1EC2EnrichFake) DescribeInstanceAttribute(_ context.Context, in *ec2.DescribeInstanceAttributeInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstanceAttributeOutput, error) {
	id := aws.ToString(in.InstanceId)
	f.mu.Lock()
	f.attrCalls = append(f.attrCalls, id)
	f.mu.Unlock()
	if err, ok := f.attrErr[id]; ok {
		return nil, err
	}
	script, ok := f.userData[id]
	if !ok {
		return &ec2.DescribeInstanceAttributeOutput{InstanceId: in.InstanceId}, nil
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(script))
	return &ec2.DescribeInstanceAttributeOutput{
		InstanceId: in.InstanceId,
		UserData:   &ec2types.AttributeValue{Value: aws.String(encoded)},
	}, nil
}

// pw1Instance builds a realistic DescribeInstances entry.
func pw1Instance(id, state, publicIP string, tokens ec2types.HttpTokensState, sgIDs ...string) ec2types.Instance {
	inst := ec2types.Instance{
		InstanceId:   aws.String(id),
		InstanceType: ec2types.InstanceTypeT3Micro,
		State:        &ec2types.InstanceState{Name: ec2types.InstanceStateName(state)},
		MetadataOptions: &ec2types.InstanceMetadataOptionsResponse{
			HttpTokens:   tokens,
			HttpEndpoint: ec2types.InstanceMetadataEndpointStateEnabled,
		},
	}
	if publicIP != "" {
		inst.PublicIpAddress = aws.String(publicIP)
	}
	for _, sg := range sgIDs {
		inst.SecurityGroups = append(inst.SecurityGroups, ec2types.GroupIdentifier{GroupId: aws.String(sg)})
	}
	return inst
}

func pw1FetchEC2(t *testing.T, instances ...ec2types.Instance) []resource.Resource {
	t.Helper()
	out, err := awsclient.FetchEC2InstancesPage(context.Background(), &pw1EC2ListFake{instances: instances}, "")
	if err != nil {
		t.Fatalf("FetchEC2InstancesPage: %v", err)
	}
	return out.Resources
}

func pw1ResourceByID(t *testing.T, rs []resource.Resource, id string) resource.Resource {
	t.Helper()
	for _, r := range rs {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("resource %q not found among %d rows", id, len(rs))
	return resource.Resource{}
}

// ─── row 1: ec2.imdsv1-allowed ──────────────────────────────────────────────

// TestEC2_IMDSv1Allowed_Optional pins that HttpTokens=optional on a running
// instance is a warning: IMDSv1 lets any process that can reach the link-local
// address read the instance's role credentials without a session token.
func TestEC2_IMDSv1Allowed_Optional(t *testing.T) {
	rs := pw1FetchEC2(t, pw1Instance("i-0imdsv1aaaaaaaa1", "running", "", ec2types.HttpTokensStateOptional))
	r := pw1ResourceByID(t, rs, "i-0imdsv1aaaaaaaa1")
	pw1RequireFinding(t, r.Findings, pw1EC2CodeIMDSv1, "IMDSv1 allowed", domain.SevWarn, "wave1")
}

// TestEC2_IMDSv1Allowed_RequiredIsHealthy pins the negative case: the
// hardened configuration emits nothing.
func TestEC2_IMDSv1Allowed_RequiredIsHealthy(t *testing.T) {
	rs := pw1FetchEC2(t, pw1Instance("i-0imdsv2aaaaaaaa1", "running", "", ec2types.HttpTokensStateRequired))
	pw1RequireNoFinding(t, pw1ResourceByID(t, rs, "i-0imdsv2aaaaaaaa1").Findings, pw1EC2CodeIMDSv1)
}

// TestEC2_IMDSv1Allowed_NilMetadataOptions pins that an absent
// MetadataOptions block is unknown, not misconfigured. AWS omits the block on
// instances whose metadata state has not been reported yet.
func TestEC2_IMDSv1Allowed_NilMetadataOptions(t *testing.T) {
	inst := pw1Instance("i-0imdsnil0aaaaaa1", "running", "", "")
	inst.MetadataOptions = nil
	rs := pw1FetchEC2(t, inst)
	pw1RequireNoFinding(t, pw1ResourceByID(t, rs, "i-0imdsnil0aaaaaa1").Findings, pw1EC2CodeIMDSv1)
}

// TestEC2_IMDSv1Allowed_SkipsTerminatedAndShuttingDown pins that a machine on
// its way out is not an open posture item — there is nothing left to harden.
func TestEC2_IMDSv1Allowed_SkipsTerminatedAndShuttingDown(t *testing.T) {
	for _, state := range []string{"terminated", "shutting-down"} {
		id := "i-0dead" + strings.ReplaceAll(state, "-", "")
		rs := pw1FetchEC2(t, pw1Instance(id, state, "203.0.113.9", ec2types.HttpTokensStateOptional))
		r := pw1ResourceByID(t, rs, id)
		pw1RequireNoFinding(t, r.Findings, pw1EC2CodeIMDSv1)
		pw1RequireNoFinding(t, r.Findings, pw1EC2CodePublicIP)
	}
}

// ─── row 2: ec2.public-ip ───────────────────────────────────────────────────

// TestEC2_PublicIP_Present pins the warning and the row that names the
// address the operator has to look up in their SG rules.
func TestEC2_PublicIP_Present(t *testing.T) {
	rs := pw1FetchEC2(t, pw1Instance("i-0pubip00aaaaaaa1", "running", "203.0.113.42", ec2types.HttpTokensStateRequired))
	r := pw1ResourceByID(t, rs, "i-0pubip00aaaaaaa1")
	pw1RequireFinding(t, r.Findings, pw1EC2CodePublicIP, "public address", domain.SevWarn, "wave1")
	pw1RequireRow(t, r.AttentionDetails[pw1EC2CodePublicIP].Rows, "Public address", "203.0.113.42")
}

// TestEC2_PublicIP_PrivateOnlyIsHealthy pins the negative case.
func TestEC2_PublicIP_PrivateOnlyIsHealthy(t *testing.T) {
	rs := pw1FetchEC2(t, pw1Instance("i-0privonly0aaaaa1", "running", "", ec2types.HttpTokensStateRequired))
	pw1RequireNoFinding(t, pw1ResourceByID(t, rs, "i-0privonly0aaaaa1").Findings, pw1EC2CodePublicIP)
}

// TestEC2_PublicIP_EmptyStringIsNotAnAddress pins that an empty
// PublicIpAddress string is the same as a nil pointer — some SDK paths
// populate the field with "" rather than leaving it unset.
func TestEC2_PublicIP_EmptyStringIsNotAnAddress(t *testing.T) {
	inst := pw1Instance("i-0emptyip0aaaaaa1", "running", "", ec2types.HttpTokensStateRequired)
	inst.PublicIpAddress = aws.String("")
	rs := pw1FetchEC2(t, inst)
	pw1RequireNoFinding(t, pw1ResourceByID(t, rs, "i-0emptyip0aaaaaa1").Findings, pw1EC2CodePublicIP)
}

// TestEC2_LifecycleAndPostureFindingsCoexist pins independence: a stopped
// instance that also allows IMDSv1 carries both findings. The lifecycle
// switch in the fetcher assigns the Findings slice, so a posture rule that
// writes rather than appends silently erases the lifecycle signal.
func TestEC2_LifecycleAndPostureFindingsCoexist(t *testing.T) {
	rs := pw1FetchEC2(t, pw1Instance("i-0stopimds0aaaaa1", "stopped", "", ec2types.HttpTokensStateOptional))
	r := pw1ResourceByID(t, rs, "i-0stopimds0aaaaa1")
	pw1RequireFinding(t, r.Findings, pw1EC2CodeIMDSv1, "IMDSv1 allowed", domain.SevWarn, "wave1")
	if _, ok := pw1FindFinding(r.Findings, domain.FindingCode("ec2.state.stopped")); !ok {
		t.Errorf("lifecycle finding ec2.state.stopped lost when a posture finding was added: %+v", r.Findings)
	}
}

// TestEC2_TwoPostureConditionsOnOneInstance pins that IMDSv1 and a public IP
// on the same machine are two independently-actionable findings, not one.
func TestEC2_TwoPostureConditionsOnOneInstance(t *testing.T) {
	rs := pw1FetchEC2(t, pw1Instance("i-0both000aaaaaaa1", "running", "198.51.100.7", ec2types.HttpTokensStateOptional))
	r := pw1ResourceByID(t, rs, "i-0both000aaaaaaa1")
	pw1RequireFinding(t, r.Findings, pw1EC2CodeIMDSv1, "IMDSv1 allowed", domain.SevWarn, "wave1")
	pw1RequireFinding(t, r.Findings, pw1EC2CodePublicIP, "public address", domain.SevWarn, "wave1")
}

// ─── row 3: ec2.internet-exposed ────────────────────────────────────────────

// pw1SGCache builds a cache["sg"] entry through the real security-group
// fetcher, so the risk fields the ec2 enricher reads are produced by sg.go's
// own sensitive-port list rather than hand-written by the test.
func pw1SGCache(t *testing.T, groups ...ec2types.SecurityGroup) resource.ResourceCache {
	t.Helper()
	out, err := awsclient.FetchSecurityGroupsPage(context.Background(), &pw1SGFake{groups: groups}, "")
	if err != nil {
		t.Fatalf("FetchSecurityGroupsPage: %v", err)
	}
	return resource.ResourceCache{"sg": resource.ResourceCacheEntry{Resources: out.Resources}}
}

type pw1SGFake struct {
	groups []ec2types.SecurityGroup
}

func (f *pw1SGFake) DescribeSecurityGroups(_ context.Context, _ *ec2.DescribeSecurityGroupsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	return &ec2.DescribeSecurityGroupsOutput{SecurityGroups: f.groups}, nil
}

// pw1SG builds a security group with one internet-facing TCP rule on
// [from,to]; from<0 means an all-protocols (-1) rule.
func pw1SG(id string, from, to int32, allProtocols bool) ec2types.SecurityGroup {
	perm := ec2types.IpPermission{
		IpRanges: []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}},
	}
	if allProtocols {
		perm.IpProtocol = aws.String("-1")
	} else {
		perm.IpProtocol = aws.String("tcp")
		perm.FromPort = aws.Int32(from)
		perm.ToPort = aws.Int32(to)
	}
	return ec2types.SecurityGroup{
		GroupId:       aws.String(id),
		GroupName:     aws.String("acme-" + id),
		VpcId:         aws.String("vpc-0aaaa1111bbbb2222"),
		Description:   aws.String("test"),
		IpPermissions: []ec2types.IpPermission{perm},
	}
}

// pw1ClosedSG is a security group with no internet-facing ingress.
func pw1ClosedSG(id string) ec2types.SecurityGroup {
	return ec2types.SecurityGroup{
		GroupId:     aws.String(id),
		GroupName:   aws.String("acme-" + id),
		VpcId:       aws.String("vpc-0aaaa1111bbbb2222"),
		Description: aws.String("test"),
		IpPermissions: []ec2types.IpPermission{{
			IpProtocol: aws.String("tcp"),
			FromPort:   aws.Int32(443),
			ToPort:     aws.Int32(443),
			IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}},
		}},
	}
}

// pw1EnrichEC2 runs the ec2 Wave-2 enricher over resources built by the
// wave-1 fetcher, so RawStruct and Fields carry exactly what production holds.
func pw1EnrichEC2(t *testing.T, fake *pw1EC2EnrichFake, cache resource.ResourceCache, instances ...ec2types.Instance) (awsclient.IssueEnricherResult, error) {
	t.Helper()
	rs := pw1FetchEC2(t, instances...)
	return awsclient.EnrichEC2InstanceStatus(context.Background(), &awsclient.ServiceClients{EC2: fake}, rs, cache)
}

// TestEC2_InternetExposed_SensitivePortFromSGCache pins that an instance with
// a public IP whose security group opens a sensitive port to 0.0.0.0/0 is
// Broken, and that the port list is the one sg.go derived. Port 27017 is in
// sg.go's sensitivePorts set but is not one of the obvious two (22/3389), so
// a locally re-implemented port list fails here.
func TestEC2_InternetExposed_SensitivePortFromSGCache(t *testing.T) {
	const id = "i-0exposed0aaaaaa1"
	cache := pw1SGCache(t, pw1SG("sg-0mongo000aaaaaa1", 27017, 27017, false))
	fake := &pw1EC2EnrichFake{}

	res, err := pw1EnrichEC2(t, fake, cache,
		pw1Instance(id, "running", "203.0.113.10", ec2types.HttpTokensStateRequired, "sg-0mongo000aaaaaa1"))
	if err != nil {
		t.Fatalf("EnrichEC2InstanceStatus: %v", err)
	}
	pw1RequireFinding(t, res.Findings[id], pw1EC2CodeInternetExposed,
		"port 27017 reachable from the internet", domain.SevBroken, "wave2")
	rows := pw1Rows(res, id, pw1EC2CodeInternetExposed)
	pw1RequireRow(t, rows, "Public address", "203.0.113.10")
	pw1RequireRow(t, rows, "Security groups", "sg-0mongo000aaaaaa1")
	pw1RequireRow(t, rows, "Ports", "27017")
}

// TestEC2_InternetExposed_WideOpenSGReportsAll pins that an all-protocols
// rule reports every port rather than enumerating a list.
//
// The wide-open case is ec2.internet-exposed-all with its own sentence,
// because "port(s) all reachable from the internet" reads the word "all" as
// a port list and carries an unresolved hedge; the port-list code stays
// silent here.
func TestEC2_InternetExposed_WideOpenSGReportsAll(t *testing.T) {
	const id = "i-0wideopen0aaaaa1"
	cache := pw1SGCache(t, pw1SG("sg-0wide0000aaaaaa1", 0, 0, true))

	res, err := pw1EnrichEC2(t, &pw1EC2EnrichFake{}, cache,
		pw1Instance(id, "running", "203.0.113.11", ec2types.HttpTokensStateRequired, "sg-0wide0000aaaaaa1"))
	if err != nil {
		t.Fatalf("EnrichEC2InstanceStatus: %v", err)
	}
	pw1RequireFinding(t, res.Findings[id], misc4CodeExposedAll,
		"every port reachable from the internet", domain.SevBroken, "wave2")
	pw1RequireRow(t, pw1Rows(res, id, misc4CodeExposedAll), "Ports", "all")
}

// TestEC2_InternetExposed_NoPublicIPIsHealthy pins that a private instance
// behind an open security group is not internet-exposed — reachability needs
// both an address and an open rule.
func TestEC2_InternetExposed_NoPublicIPIsHealthy(t *testing.T) {
	const id = "i-0privopen0aaaaa1"
	cache := pw1SGCache(t, pw1SG("sg-0ssh00000aaaaaa1", 22, 22, false))

	res, err := pw1EnrichEC2(t, &pw1EC2EnrichFake{}, cache,
		pw1Instance(id, "running", "", ec2types.HttpTokensStateRequired, "sg-0ssh00000aaaaaa1"))
	if err != nil {
		t.Fatalf("EnrichEC2InstanceStatus: %v", err)
	}
	pw1RequireNoFinding(t, res.Findings[id], pw1EC2CodeInternetExposed)
}

// TestEC2_InternetExposed_ClosedSGIsHealthy pins that a public instance whose
// only open port is 443 is not flagged.
func TestEC2_InternetExposed_ClosedSGIsHealthy(t *testing.T) {
	const id = "i-0pub443000aaaaa1"
	cache := pw1SGCache(t, pw1ClosedSG("sg-0https000aaaaaa1"))

	res, err := pw1EnrichEC2(t, &pw1EC2EnrichFake{}, cache,
		pw1Instance(id, "running", "203.0.113.12", ec2types.HttpTokensStateRequired, "sg-0https000aaaaaa1"))
	if err != nil {
		t.Fatalf("EnrichEC2InstanceStatus: %v", err)
	}
	pw1RequireNoFinding(t, res.Findings[id], pw1EC2CodeInternetExposed)
}

// TestEC2_InternetExposed_NoSGCacheIsSilent pins that an unloaded sg list is
// unknown, not safe and not exposed: absence of evidence must not produce a
// Broken row.
func TestEC2_InternetExposed_NoSGCacheIsSilent(t *testing.T) {
	const id = "i-0nosgcache0aaaa1"
	res, err := pw1EnrichEC2(t, &pw1EC2EnrichFake{}, resource.ResourceCache{},
		pw1Instance(id, "running", "203.0.113.13", ec2types.HttpTokensStateRequired, "sg-0missing00aaaaa1"))
	if err != nil {
		t.Fatalf("EnrichEC2InstanceStatus: %v", err)
	}
	pw1RequireNoFinding(t, res.Findings[id], pw1EC2CodeInternetExposed)
}

// TestEC2_InternetExposed_TerminatedInstanceIsSilent pins that a terminated
// machine is not a posture item even while its ENI record still names a
// public address.
func TestEC2_InternetExposed_TerminatedInstanceIsSilent(t *testing.T) {
	const id = "i-0termexposed0aa1"
	cache := pw1SGCache(t, pw1SG("sg-0ssh10000aaaaaa1", 22, 22, false))

	res, err := pw1EnrichEC2(t, &pw1EC2EnrichFake{}, cache,
		pw1Instance(id, "terminated", "203.0.113.14", ec2types.HttpTokensStateRequired, "sg-0ssh10000aaaaaa1"))
	if err != nil {
		t.Fatalf("EnrichEC2InstanceStatus: %v", err)
	}
	pw1RequireNoFinding(t, res.Findings[id], pw1EC2CodeInternetExposed)
}

// TestEC2_InternetExposed_EvaluatesEveryInputInstance pins that the exposure
// rule iterates the input resources. The existing enricher body iterates the
// DescribeInstanceStatus response instead, and an instance AWS omits from
// that response (a freshly launched machine) would silently escape the check.
func TestEC2_InternetExposed_EvaluatesEveryInputInstance(t *testing.T) {
	cache := pw1SGCache(t, pw1SG("sg-0rdp00000aaaaaa1", 3389, 3389, false))
	res, err := pw1EnrichEC2(t, &pw1EC2EnrichFake{}, cache,
		pw1Instance("i-0batch00aaaaaaa1", "running", "203.0.113.21", ec2types.HttpTokensStateRequired, "sg-0rdp00000aaaaaa1"),
		pw1Instance("i-0batch00aaaaaaa2", "running", "203.0.113.22", ec2types.HttpTokensStateRequired, "sg-0rdp00000aaaaaa1"),
	)
	if err != nil {
		t.Fatalf("EnrichEC2InstanceStatus: %v", err)
	}
	for _, id := range []string{"i-0batch00aaaaaaa1", "i-0batch00aaaaaaa2"} {
		pw1RequireFinding(t, res.Findings[id], pw1EC2CodeInternetExposed,
			"port 3389 reachable from the internet", domain.SevBroken, "wave2")
	}
}

// TestEC2_PublicIPAndInternetExposedBothFire pins the interim behaviour the
// batch spec mandates while the suppression question is unresolved: an
// exposed instance carries both the wave-1 public-IP warning and the wave-2
// exposure finding, and worst severity decides the row colour.
//
// This test encodes the interim contract deliberately. If a suppression
// mechanism is later added to ApplyWave2ToRow, invert this test rather than
// deleting it, and say so here.
func TestEC2_PublicIPAndInternetExposedBothFire(t *testing.T) {
	const id = "i-0bothexposed0aa1"
	cache := pw1SGCache(t, pw1SG("sg-0ssh20000aaaaaa1", 22, 22, false))
	inst := pw1Instance(id, "running", "203.0.113.31", ec2types.HttpTokensStateRequired, "sg-0ssh20000aaaaaa1")

	wave1 := pw1ResourceByID(t, pw1FetchEC2(t, inst), id)
	pw1RequireFinding(t, wave1.Findings, pw1EC2CodePublicIP, "public address", domain.SevWarn, "wave1")

	res, err := pw1EnrichEC2(t, &pw1EC2EnrichFake{}, cache, inst)
	if err != nil {
		t.Fatalf("EnrichEC2InstanceStatus: %v", err)
	}
	pw1RequireFinding(t, res.Findings[id], pw1EC2CodeInternetExposed,
		"port 22 reachable from the internet", domain.SevBroken, "wave2")
}

// ─── row 4: ec2.user-data-secret ────────────────────────────────────────────

const pw1UserDataWithSecret = `#!/bin/bash
yum install -y awscli
export DB_PASSWORD=hunter2hunter2
/opt/app/start.sh
`

const pw1UserDataClean = `#!/bin/bash
yum install -y awscli
export DB_PASSWORD_ARN=arn:aws:secretsmanager:us-east-1:123456789012:secret:acme/db-AbCdEf
/opt/app/start.sh
`

// TestEC2_UserDataSecret_PlaintextCredential pins the Broken finding and the
// Where:Kind row. The row must name the location and the kind only — the
// credential value never reaches a phrase, a row or a log.
func TestEC2_UserDataSecret_PlaintextCredential(t *testing.T) {
	const id = "i-0udsecret0aaaaa1"
	fake := &pw1EC2EnrichFake{userData: map[string]string{id: pw1UserDataWithSecret}}

	res, err := pw1EnrichEC2(t, fake, resource.ResourceCache{},
		pw1Instance(id, "running", "", ec2types.HttpTokensStateRequired))
	if err != nil {
		t.Fatalf("EnrichEC2InstanceStatus: %v", err)
	}
	f := pw1RequireFinding(t, res.Findings[id], pw1EC2CodeUserDataSecret,
		"credential in user data", domain.SevBroken, "wave2")
	pw1RequireRow(t, pw1Rows(res, id, pw1EC2CodeUserDataSecret), "line 3", "keyword")

	for _, text := range []string{f.Phrase, f.Detail} {
		if strings.Contains(text, "hunter2hunter2") {
			t.Errorf("secret value leaked into finding text: %q", text)
		}
	}
	for _, row := range pw1Rows(res, id, pw1EC2CodeUserDataSecret) {
		if strings.Contains(row.Value, "hunter2hunter2") || strings.Contains(row.Label, "hunter2hunter2") {
			t.Errorf("secret value leaked into an AttentionDetail row: %+v", row)
		}
	}
}

// TestEC2_UserDataSecret_SecretsManagerReferenceIsHealthy pins the negative
// case that separates good practice from a leak: a script that resolves the
// credential from Secrets Manager carries an ARN, not a secret. secretscan's
// isRealValue rejects it; a bare "password=" regex would not.
func TestEC2_UserDataSecret_SecretsManagerReferenceIsHealthy(t *testing.T) {
	const id = "i-0udclean00aaaaa1"
	fake := &pw1EC2EnrichFake{userData: map[string]string{id: pw1UserDataClean}}

	res, err := pw1EnrichEC2(t, fake, resource.ResourceCache{},
		pw1Instance(id, "running", "", ec2types.HttpTokensStateRequired))
	if err != nil {
		t.Fatalf("EnrichEC2InstanceStatus: %v", err)
	}
	pw1RequireNoFinding(t, res.Findings[id], pw1EC2CodeUserDataSecret)
}

// TestEC2_UserDataSecret_NoUserDataIsHealthy pins that an instance whose
// DescribeInstanceAttribute response carries no UserData value emits nothing.
func TestEC2_UserDataSecret_NoUserDataIsHealthy(t *testing.T) {
	const id = "i-0noud0000aaaaaa1"
	res, err := pw1EnrichEC2(t, &pw1EC2EnrichFake{}, resource.ResourceCache{},
		pw1Instance(id, "running", "", ec2types.HttpTokensStateRequired))
	if err != nil {
		t.Fatalf("EnrichEC2InstanceStatus: %v", err)
	}
	pw1RequireNoFinding(t, res.Findings[id], pw1EC2CodeUserDataSecret)
}

// TestEC2_UserDataSecret_SkipsTerminated pins that no user-data call is made
// for a terminated instance — the API rejects it and the row is not an item.
func TestEC2_UserDataSecret_SkipsTerminated(t *testing.T) {
	const id = "i-0termud000aaaaa1"
	fake := &pw1EC2EnrichFake{userData: map[string]string{id: pw1UserDataWithSecret}}

	res, err := pw1EnrichEC2(t, fake, resource.ResourceCache{},
		pw1Instance(id, "terminated", "", ec2types.HttpTokensStateOptional))
	if err != nil {
		t.Fatalf("EnrichEC2InstanceStatus: %v", err)
	}
	pw1RequireNoFinding(t, res.Findings[id], pw1EC2CodeUserDataSecret)
	for _, called := range fake.attrCalls {
		if called == id {
			t.Errorf("DescribeInstanceAttribute called for terminated instance %s", id)
		}
	}
}

// TestEC2_UserDataSecret_PerItemErrorMarksOnlyThatRow pins the partial-failure
// contract: one instance whose attribute call fails goes to TruncatedIDs so
// its row renders "?", while its neighbour in the same batch is still
// evaluated and still reports its secret.
func TestEC2_UserDataSecret_PerItemErrorMarksOnlyThatRow(t *testing.T) {
	const bad = "i-0udfail000aaaaa1"
	const good = "i-0udok00000aaaaa1"
	fake := &pw1EC2EnrichFake{
		userData: map[string]string{good: pw1UserDataWithSecret},
		attrErr:  map[string]error{bad: errors.New("UnauthorizedOperation: not authorized")},
	}

	res, _ := pw1EnrichEC2(t, fake, resource.ResourceCache{},
		pw1Instance(bad, "running", "", ec2types.HttpTokensStateRequired),
		pw1Instance(good, "running", "", ec2types.HttpTokensStateRequired),
	)
	if _, marked := res.TruncatedIDs[bad]; !marked {
		t.Errorf("TruncatedIDs missing %s; a failed user-data read must render ? not vanish", bad)
	}
	pw1RequireNoFinding(t, res.Findings[bad], pw1EC2CodeUserDataSecret)
	pw1RequireFinding(t, res.Findings[good], pw1EC2CodeUserDataSecret,
		"credential in user data", domain.SevBroken, "wave2")
	if _, marked := res.TruncatedIDs[good]; marked {
		t.Errorf("healthy neighbour %s wrongly marked truncated", good)
	}
}

// TestEC2_UserDataSecret_CapBoundsTheWalk pins that EnrichmentCap+1 instances
// leave the enricher Truncated: the user-data check emits "!" findings, so a
// cut-short walk means the issue count is a lower bound.
func TestEC2_UserDataSecret_CapBoundsTheWalk(t *testing.T) {
	var instances []ec2types.Instance
	for i := 0; i <= awsclient.EnrichmentCap; i++ {
		instances = append(instances, pw1Instance(pw1InstanceID(i), "running", "", ec2types.HttpTokensStateRequired))
	}
	fake := &pw1EC2EnrichFake{}
	res, err := pw1EnrichEC2(t, fake, resource.ResourceCache{}, instances...)
	if err != nil {
		t.Fatalf("EnrichEC2InstanceStatus: %v", err)
	}
	if !res.Truncated {
		t.Errorf("Truncated = false for %d instances; EnrichmentCap is %d", len(instances), awsclient.EnrichmentCap)
	}
	if len(fake.attrCalls) > awsclient.EnrichmentCap {
		t.Errorf("DescribeInstanceAttribute called %d times, cap is %d", len(fake.attrCalls), awsclient.EnrichmentCap)
	}
}

// TestEC2_UserDataSecret_ExactlyCapIsNotTruncated pins the other side of the
// boundary: a batch of exactly EnrichmentCap is fully inspected.
func TestEC2_UserDataSecret_ExactlyCapIsNotTruncated(t *testing.T) {
	var instances []ec2types.Instance
	for i := 0; i < awsclient.EnrichmentCap; i++ {
		instances = append(instances, pw1Instance(pw1InstanceID(i), "running", "", ec2types.HttpTokensStateRequired))
	}
	res, err := pw1EnrichEC2(t, &pw1EC2EnrichFake{}, resource.ResourceCache{}, instances...)
	if err != nil {
		t.Fatalf("EnrichEC2InstanceStatus: %v", err)
	}
	if res.Truncated {
		t.Errorf("Truncated = true for exactly EnrichmentCap (%d) instances", awsclient.EnrichmentCap)
	}
}

// pw1InstanceID builds a syntactically valid 17-hex-char instance ID from n.
func pw1InstanceID(n int) string {
	const hexDigits = "0123456789abcdef"
	suffix := make([]byte, 17)
	for i := range suffix {
		suffix[i] = 'a'
	}
	suffix[15] = hexDigits[(n/16)%16]
	suffix[16] = hexDigits[n%16]
	return "i-" + string(suffix)
}

// ─── demo bench ─────────────────────────────────────────────────────────────

// TestEC2_DemoBench_WitnessRowsCarryExactlyTheirFinding pins the demo fixture
// contract: each ec2 posture code is carried by its named witness row and by
// no other row, so the demo bench shows one example of each signal.
func TestEC2_DemoBench_WitnessRowsCarryExactlyTheirFinding(t *testing.T) {
	out, err := awsclient.FetchEC2InstancesPage(context.Background(), fakes.NewEC2(), "")
	if err != nil {
		t.Fatalf("FetchEC2InstancesPage(demo): %v", err)
	}
	clients := &awsclient.ServiceClients{EC2: fakes.NewEC2()}
	enriched, err := awsclient.EnrichEC2InstanceStatus(context.Background(), clients, out.Resources, pw1DemoSGCache(t))
	if err != nil {
		t.Fatalf("EnrichEC2InstanceStatus(demo): %v", err)
	}

	var imdsv1 []string
	for _, r := range out.Resources {
		if _, ok := pw1FindFinding(r.Findings, pw1EC2CodeIMDSv1); ok {
			imdsv1 = append(imdsv1, r.Name+"/"+r.ID)
		}
	}
	pw1RequireOnlyWitness(t, pw1EC2CodeIMDSv1, fixtures.EC2InstanceIMDSv1, imdsv1)

	// Three rows legitimately carry ec2.public-ip: its own witness, and the
	// two exposure witnesses, which need a public address to be reachable at
	// all. Both signals fire on those rows under the interim no-suppression
	// rule, so the demo contract here is "these three and nobody else".
	publicIP := map[string]bool{}
	for _, r := range out.Resources {
		if _, ok := pw1FindFinding(r.Findings, pw1EC2CodePublicIP); ok {
			publicIP[r.ID] = true
		}
	}
	for _, want := range []string{
		fixtures.EC2InstancePublicIPOnly,
		fixtures.EC2InstanceInternetExposed,
		fixtures.EC2InstanceInternetExposedAll,
	} {
		if !publicIP[want] {
			t.Errorf("ec2.public-ip: witness %s does not carry the finding", want)
		}
		delete(publicIP, want)
	}
	for extra := range publicIP {
		t.Errorf("ec2.public-ip: unexpected demo row %s carries the finding", extra)
	}

	wave2 := map[domain.FindingCode]string{
		pw1EC2CodeInternetExposed: fixtures.EC2InstanceInternetExposed,
		misc4CodeExposedAll:       fixtures.EC2InstanceInternetExposedAll,
		pw1EC2CodeUserDataSecret:  fixtures.EC2InstanceUserDataSecret,
	}
	for code, witness := range wave2 {
		var carriers []string
		for id, fs := range enriched.Findings {
			if _, ok := pw1FindFinding(fs, code); ok {
				carriers = append(carriers, pw1DemoNameFor(out.Resources, id))
			}
		}
		pw1RequireOnlyWitness(t, code, witness, carriers)
	}
}

// pw1DemoSGCache builds the demo sg cache the ec2 exposure rule cross-refs.
func pw1DemoSGCache(t *testing.T) resource.ResourceCache {
	t.Helper()
	out, err := awsclient.FetchSecurityGroupsPage(context.Background(), fakes.NewEC2(), "")
	if err != nil {
		t.Fatalf("FetchSecurityGroupsPage(demo): %v", err)
	}
	return resource.ResourceCache{"sg": resource.ResourceCacheEntry{Resources: out.Resources}}
}

func pw1DemoNameFor(rs []resource.Resource, id string) string {
	for _, r := range rs {
		if r.ID == id {
			return r.Name + "/" + r.ID
		}
	}
	return id
}

// pw1RequireOnlyWitness fails unless exactly one row carries the code and it
// is the fixture named as that code's witness.
func pw1RequireOnlyWitness(t *testing.T, code domain.FindingCode, witness string, carriers []string) {
	t.Helper()
	if len(carriers) != 1 {
		t.Errorf("%s: want exactly 1 demo row, got %d: %v", code, len(carriers), carriers)
		return
	}
	if !strings.Contains(carriers[0], witness) {
		t.Errorf("%s: carried by %q, want the witness %q", code, carriers[0], witness)
	}
}
