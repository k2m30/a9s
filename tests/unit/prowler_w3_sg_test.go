package unit

// prowler_w3_sg_test.go — security-group posture signals.
//
// Row 1: the sensitive-port set behind sg.ingress.dangerous-ports must cover
// the plaintext / datastore / admin ports Prowler flags, and must NOT cover
// the application ports (8080/8443) that are ordinarily internet-facing on
// purpose — a finding operators learn to ignore is worse than no finding.
// Row 2: an AWS-created "default" group is supposed to carry no usable rules;
// anything beyond the create-time allow-all egress means it is in service.
// Row 3: sg.unused is a cache-only join against the ENI list, so it must
// refuse to conclude "unused" from an ENI list it knows is incomplete.

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	w3CodeSGDangerousPorts   = domain.FindingCode("sg.ingress.dangerous-ports")
	w3CodeSGWideOpen         = domain.FindingCode("sg.ingress.wide-open")
	w3CodeSGDefaultWithRules = domain.FindingCode("sg.default-with-rules")
	w3CodeSGUnused           = domain.FindingCode("sg.unused")

	w3PhraseSGDefaultWithRules = "default group allows traffic"
	w3PhraseSGUnused           = "not attached to anything"
)

// w3SGAPI serves one DescribeSecurityGroups page.
type w3SGAPI struct {
	groups []ec2types.SecurityGroup
	err    error
}

func (f w3SGAPI) DescribeSecurityGroups(_ context.Context, _ *ec2.DescribeSecurityGroupsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &ec2.DescribeSecurityGroupsOutput{SecurityGroups: f.groups}, nil
}

// w3TCPFromInternet is a realistic DescribeSecurityGroups ingress rule: a
// single TCP port opened to 0.0.0.0/0 with the description AWS echoes back.
func w3TCPFromInternet(port int32) ec2types.IpPermission {
	return ec2types.IpPermission{
		IpProtocol: aws.String("tcp"),
		FromPort:   aws.Int32(port),
		ToPort:     aws.Int32(port),
		IpRanges: []ec2types.IpRange{
			{CidrIp: aws.String("0.0.0.0/0"), Description: aws.String("open to the world")},
		},
	}
}

// w3DefaultEgress is the egress rule AWS creates on every new security group.
// Its presence alone is not evidence the group is in use.
func w3DefaultEgress() ec2types.IpPermission {
	return ec2types.IpPermission{
		IpProtocol: aws.String("-1"),
		IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}},
	}
}

func w3FetchSGs(t *testing.T, groups ...ec2types.SecurityGroup) []resource.Resource {
	t.Helper()
	res, err := awsclient.FetchSecurityGroupsPage(context.Background(), w3SGAPI{groups: groups}, "")
	if err != nil {
		t.Fatalf("FetchSecurityGroupsPage: %v", err)
	}
	return res.Resources
}

// w3FindingByCode returns the finding carrying code, or false.
func w3FindingByCode(fs []domain.Finding, code domain.FindingCode) (domain.Finding, bool) {
	for _, f := range fs {
		if f.Code == code {
			return f, true
		}
	}
	return domain.Finding{}, false
}

// w3AssertRows compares an AttentionDetail's rows to the expected label/value
// pairs in order.
func w3AssertRows(t *testing.T, got []domain.DetailRow, want [][2]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("attention rows = %v, want %d rows %v", got, len(want), want)
	}
	for i, w := range want {
		if got[i].Label != w[0] || got[i].Value != w[1] {
			t.Errorf("row %d = %q: %q, want %q: %q", i, got[i].Label, got[i].Value, w[0], w[1])
		}
	}
}

// ── Row 1: sensitive port set ─────────────────────────────────────────────

// w3SensitivePortCases are the ports both the batch spec and the dispatch
// agree must produce sg.ingress.dangerous-ports when open to 0.0.0.0/0.
var w3SensitivePortCases = []struct {
	port    int32
	service string
}{
	{20, "FTP data"},
	{21, "FTP"},
	{23, "Telnet"},
	{1521, "Oracle"},
	{2483, "Oracle"},
	{5601, "Kibana"},
	{7199, "Cassandra JMX"},
	{8888, "Cassandra"},
	{9092, "Kafka"},
	{9160, "Cassandra Thrift"},
	{11211, "Memcached"},
	// Ports already covered before this batch — they must not regress.
	{22, "SSH"},
	{3389, "RDP"},
	{3306, "MySQL"},
	{5432, "PostgreSQL"},
	{1433, "MSSQL"},
	{6379, "Redis"},
	{9200, "Elasticsearch"},
	{27017, "MongoDB"},
}

func TestW3SGDangerousPorts_FlagsSensitivePortOpenToInternet(t *testing.T) {
	for _, tc := range w3SensitivePortCases {
		t.Run(strconv.Itoa(int(tc.port)), func(t *testing.T) {
			rows := w3FetchSGs(t, ec2types.SecurityGroup{
				GroupId:       aws.String("sg-0aa11bb22cc33dd44"),
				GroupName:     aws.String("acme-edge"),
				VpcId:         aws.String("vpc-0abc123"),
				Description:   aws.String("edge tier"),
				IpPermissions: []ec2types.IpPermission{w3TCPFromInternet(tc.port)},
			})
			f, ok := w3FindingByCode(rows[0].Findings, w3CodeSGDangerousPorts)
			if !ok {
				t.Fatalf("port %d (%s) open to 0.0.0.0/0 produced no %s; findings=%+v",
					tc.port, tc.service, w3CodeSGDangerousPorts, rows[0].Findings)
			}
			wantPhrase := "ports " + strconv.Itoa(int(tc.port)) + " open to 0.0.0.0/0"
			if f.Phrase != wantPhrase {
				t.Errorf("Phrase = %q, want %q", f.Phrase, wantPhrase)
			}
			if f.Severity != domain.SevBroken {
				t.Errorf("Severity = %v, want SevBroken", f.Severity)
			}
			if f.Source != "wave1" {
				t.Errorf("Source = %q, want %q", f.Source, "wave1")
			}
			if got := rows[0].Fields["dangerous_open_count"]; got != "1" {
				t.Errorf("dangerous_open_count = %q, want %q", got, "1")
			}
			if got := rows[0].Fields["risk_summary"]; got != wantPhrase {
				t.Errorf("risk_summary = %q, want %q", got, wantPhrase)
			}
		})
	}
}

// TestW3SGDangerousPorts_SkipsApplicationPorts pins the deliberate exclusion:
// 8080 and 8443 are ordinary reverse-proxy ports and are internet-facing on
// purpose in most accounts, so flagging them trains operators to ignore the
// column.
func TestW3SGDangerousPorts_SkipsApplicationPorts(t *testing.T) {
	for _, port := range []int32{8080, 8443} {
		t.Run(strconv.Itoa(int(port)), func(t *testing.T) {
			rows := w3FetchSGs(t, ec2types.SecurityGroup{
				GroupId:       aws.String("sg-0aa11bb22cc33dd44"),
				GroupName:     aws.String("acme-web"),
				IpPermissions: []ec2types.IpPermission{w3TCPFromInternet(port)},
			})
			if f, ok := w3FindingByCode(rows[0].Findings, w3CodeSGDangerousPorts); ok {
				t.Errorf("port %d must not be treated as sensitive; got %q", port, f.Phrase)
			}
			if got := rows[0].Fields["dangerous_open_count"]; got != "0" {
				t.Errorf("dangerous_open_count = %q, want %q", got, "0")
			}
		})
	}
}

// TestW3SGDangerousPorts_SMBAndSMTP covers the two ports where the batch spec
// (which lists 445 SMB and 25 SMTP among the ports to add) and the dispatch
// note (which groups them with 8080/8443 as excluded) disagree. Kept separate
// so a ruling flips exactly one test.
func TestW3SGDangerousPorts_SMBAndSMTP(t *testing.T) {
	for _, tc := range []struct {
		port    int32
		service string
	}{{445, "SMB"}, {25, "SMTP"}} {
		t.Run(strconv.Itoa(int(tc.port)), func(t *testing.T) {
			rows := w3FetchSGs(t, ec2types.SecurityGroup{
				GroupId:       aws.String("sg-0aa11bb22cc33dd44"),
				GroupName:     aws.String("acme-edge"),
				IpPermissions: []ec2types.IpPermission{w3TCPFromInternet(tc.port)},
			})
			if _, ok := w3FindingByCode(rows[0].Findings, w3CodeSGDangerousPorts); !ok {
				t.Fatalf("port %d (%s) open to 0.0.0.0/0 produced no %s", tc.port, tc.service, w3CodeSGDangerousPorts)
			}
		})
	}
}

// TestW3SGDangerousPorts_InternalOnlyIsHealthy is the negative half: the same
// port reachable only from inside the VPC is normal operation, not a finding.
func TestW3SGDangerousPorts_InternalOnlyIsHealthy(t *testing.T) {
	rows := w3FetchSGs(t, ec2types.SecurityGroup{
		GroupId:   aws.String("sg-0aa11bb22cc33dd44"),
		GroupName: aws.String("acme-kafka"),
		IpPermissions: []ec2types.IpPermission{{
			IpProtocol: aws.String("tcp"),
			FromPort:   aws.Int32(9092),
			ToPort:     aws.Int32(9092),
			IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("10.0.0.0/16")}},
		}},
	})
	if len(rows[0].Findings) != 0 {
		t.Errorf("private-CIDR ingress produced findings %+v, want none", rows[0].Findings)
	}
}

// TestW3SGDangerousPorts_RangeEnumeratesEveryNewPort pins that a single wide
// TCP range reports every sensitive port it actually exposes, not just the
// first — the risk_summary is what the operator reads to decide what to close.
func TestW3SGDangerousPorts_RangeEnumeratesEveryNewPort(t *testing.T) {
	rows := w3FetchSGs(t, ec2types.SecurityGroup{
		GroupId:   aws.String("sg-0aa11bb22cc33dd44"),
		GroupName: aws.String("acme-legacy"),
		IpPermissions: []ec2types.IpPermission{{
			IpProtocol: aws.String("tcp"),
			FromPort:   aws.Int32(20),
			ToPort:     aws.Int32(25),
			IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}},
		}},
	})
	f, ok := w3FindingByCode(rows[0].Findings, w3CodeSGDangerousPorts)
	if !ok {
		t.Fatalf("TCP 20-25 open to the internet produced no %s", w3CodeSGDangerousPorts)
	}
	for _, want := range []string{"20", "21", "23"} {
		if !strings.Contains(f.Phrase, want) {
			t.Errorf("Phrase %q does not name port %s covered by the 20-25 range", f.Phrase, want)
		}
	}
}

// ── Row 2: default group carrying rules ───────────────────────────────────

func TestW3SGDefaultWithRules_IngressPresent(t *testing.T) {
	rows := w3FetchSGs(t, ec2types.SecurityGroup{
		GroupId:             aws.String("sg-0default11111111"),
		GroupName:           aws.String("default"),
		VpcId:               aws.String("vpc-0abc123"),
		Description:         aws.String("default VPC security group"),
		IpPermissions:       []ec2types.IpPermission{w3TCPFromInternet(443)},
		IpPermissionsEgress: []ec2types.IpPermission{w3DefaultEgress()},
	})
	f, ok := w3FindingByCode(rows[0].Findings, w3CodeSGDefaultWithRules)
	if !ok {
		t.Fatalf("default group with an ingress rule produced no %s; findings=%+v", w3CodeSGDefaultWithRules, rows[0].Findings)
	}
	if f.Phrase != w3PhraseSGDefaultWithRules {
		t.Errorf("Phrase = %q, want %q", f.Phrase, w3PhraseSGDefaultWithRules)
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Source = %q, want %q", f.Source, "wave1")
	}
	if f.Detail == "" {
		t.Error("Detail is empty; every finding carries an operator sentence")
	}
	w3AssertRows(t, rows[0].AttentionDetails[w3CodeSGDefaultWithRules].Rows, [][2]string{
		{"Ingress rules", "1"},
		{"Egress rules", "1"},
	})
}

// TestW3SGDefaultWithRules_UntouchedDefaultIsHealthy is the negative case: a
// freshly created default group has no ingress and exactly the AWS allow-all
// egress rule. Flagging it would light up every VPC in the account.
func TestW3SGDefaultWithRules_UntouchedDefaultIsHealthy(t *testing.T) {
	rows := w3FetchSGs(t, ec2types.SecurityGroup{
		GroupId:             aws.String("sg-0default11111111"),
		GroupName:           aws.String("default"),
		VpcId:               aws.String("vpc-0abc123"),
		IpPermissionsEgress: []ec2types.IpPermission{w3DefaultEgress()},
	})
	if _, ok := w3FindingByCode(rows[0].Findings, w3CodeSGDefaultWithRules); ok {
		t.Errorf("untouched default group was flagged; findings=%+v", rows[0].Findings)
	}
}

// TestW3SGDefaultWithRules_ExtraEgressOnly pins that egress beyond the
// create-time allow-all rule counts as "in service" even with no ingress.
func TestW3SGDefaultWithRules_ExtraEgressOnly(t *testing.T) {
	rows := w3FetchSGs(t, ec2types.SecurityGroup{
		GroupId:   aws.String("sg-0default11111111"),
		GroupName: aws.String("default"),
		IpPermissionsEgress: []ec2types.IpPermission{
			w3DefaultEgress(),
			{
				IpProtocol: aws.String("tcp"),
				FromPort:   aws.Int32(443),
				ToPort:     aws.Int32(443),
				IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("10.0.0.0/8")}},
			},
		},
	})
	if _, ok := w3FindingByCode(rows[0].Findings, w3CodeSGDefaultWithRules); !ok {
		t.Fatalf("default group with an extra egress rule produced no %s", w3CodeSGDefaultWithRules)
	}
	w3AssertRows(t, rows[0].AttentionDetails[w3CodeSGDefaultWithRules].Rows, [][2]string{
		{"Ingress rules", "0"},
		{"Egress rules", "2"},
	})
}

// TestW3SGDefaultWithRules_NamedGroupNotFlagged pins that the check keys on
// the reserved "default" name only — a user-created group with rules is doing
// its job.
func TestW3SGDefaultWithRules_NamedGroupNotFlagged(t *testing.T) {
	rows := w3FetchSGs(t, ec2types.SecurityGroup{
		GroupId:       aws.String("sg-0aa11bb22cc33dd44"),
		GroupName:     aws.String("acme-api"),
		IpPermissions: []ec2types.IpPermission{w3TCPFromInternet(443)},
	})
	if _, ok := w3FindingByCode(rows[0].Findings, w3CodeSGDefaultWithRules); ok {
		t.Errorf("non-default group was flagged as a default group; findings=%+v", rows[0].Findings)
	}
}

// TestW3SGDefaultWithRules_IndependentOfExposure pins independence: a default
// group that is also wide open reports both conditions, each with its own
// code, phrase and severity.
func TestW3SGDefaultWithRules_IndependentOfExposure(t *testing.T) {
	rows := w3FetchSGs(t, ec2types.SecurityGroup{
		GroupId:   aws.String("sg-0default11111111"),
		GroupName: aws.String("default"),
		IpPermissions: []ec2types.IpPermission{{
			IpProtocol: aws.String("-1"),
			IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}},
		}},
		IpPermissionsEgress: []ec2types.IpPermission{w3DefaultEgress()},
	})
	if _, ok := w3FindingByCode(rows[0].Findings, w3CodeSGWideOpen); !ok {
		t.Errorf("missing %s; findings=%+v", w3CodeSGWideOpen, rows[0].Findings)
	}
	if _, ok := w3FindingByCode(rows[0].Findings, w3CodeSGDefaultWithRules); !ok {
		t.Errorf("missing %s; findings=%+v", w3CodeSGDefaultWithRules, rows[0].Findings)
	}
	if len(rows[0].Findings) != 2 {
		t.Errorf("got %d findings %+v, want exactly 2", len(rows[0].Findings), rows[0].Findings)
	}
}

// ── Row 3: unused group (cache-only join against ENIs) ────────────────────

// w3SGRes builds the shape FetchSecurityGroupsPage hands the enricher.
func w3SGRes(id, name string) resource.Resource {
	return resource.Resource{
		ID:     id,
		Name:   name,
		Fields: map[string]string{"group_id": id, "group_name": name},
	}
}

// w3ENIRes mirrors the eni fetcher: security groups are comma-joined in one
// field.
func w3ENIRes(id string, sgIDs ...string) resource.Resource {
	return resource.Resource{
		ID:     id,
		Fields: map[string]string{"eni_id": id, "security_groups": strings.Join(sgIDs, ",")},
	}
}

func w3ENICache(truncated bool, enis ...resource.Resource) resource.ResourceCache {
	return resource.ResourceCache{"eni": resource.ResourceCacheEntry{
		Resources:   enis,
		IsTruncated: truncated,
	}}
}

func TestW3EnrichSGUsage_UnreferencedGroupFlagged(t *testing.T) {
	sgs := []resource.Resource{
		w3SGRes("sg-0unused1111111111", "acme-orphan"),
		w3SGRes("sg-0inuse22222222222", "acme-api"),
	}
	cache := w3ENICache(false,
		w3ENIRes("eni-0aaa", "sg-0inuse22222222222"),
		w3ENIRes("eni-0bbb", "sg-0inuse22222222222", "sg-0other333333333"),
	)

	res, err := awsclient.EnrichSGUsage(context.Background(), &awsclient.ServiceClients{}, sgs, cache)
	if err != nil {
		t.Fatalf("EnrichSGUsage: %v", err)
	}
	f, ok := w3FindingByCode(res.Findings["sg-0unused1111111111"], w3CodeSGUnused)
	if !ok {
		t.Fatalf("unreferenced group produced no %s; findings=%+v", w3CodeSGUnused, res.Findings)
	}
	if f.Phrase != w3PhraseSGUnused {
		t.Errorf("Phrase = %q, want %q", f.Phrase, w3PhraseSGUnused)
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", f.Severity)
	}
	if f.Source != "wave2:sg" {
		t.Errorf("Source = %q, want %q", f.Source, "wave2:sg")
	}
	if f.Detail == "" {
		t.Error("Detail is empty; every finding carries an operator sentence")
	}
	w3AssertRows(t, res.AttentionDetails["sg-0unused1111111111"][w3CodeSGUnused].Rows, [][2]string{
		{"Network interfaces referencing", "0"},
	})
	if len(res.Findings["sg-0inuse22222222222"]) != 0 {
		t.Errorf("attached group was flagged: %+v", res.Findings["sg-0inuse22222222222"])
	}
}

// TestW3EnrichSGUsage_TruncatedENICacheProvesNothing is the load-bearing edge:
// a first-page-only ENI list cannot establish that no interface references the
// group, so the enricher must stay silent rather than report a false orphan.
func TestW3EnrichSGUsage_TruncatedENICacheProvesNothing(t *testing.T) {
	sgs := []resource.Resource{w3SGRes("sg-0unused1111111111", "acme-orphan")}
	cache := w3ENICache(true, w3ENIRes("eni-0aaa", "sg-0somethingelse"))

	res, err := awsclient.EnrichSGUsage(context.Background(), &awsclient.ServiceClients{}, sgs, cache)
	if err != nil {
		t.Fatalf("EnrichSGUsage: %v", err)
	}
	if len(res.Findings["sg-0unused1111111111"]) != 0 {
		t.Errorf("truncated ENI cache produced %+v, want no finding", res.Findings["sg-0unused1111111111"])
	}
}

// TestW3EnrichSGUsage_AbsentENICacheProvesNothing covers the same reasoning
// when the ENI list has not been loaded at all.
func TestW3EnrichSGUsage_AbsentENICacheProvesNothing(t *testing.T) {
	sgs := []resource.Resource{w3SGRes("sg-0unused1111111111", "acme-orphan")}
	for _, tc := range []struct {
		name  string
		cache resource.ResourceCache
	}{
		{"nil cache", nil},
		{"no eni entry", resource.ResourceCache{"ec2": resource.ResourceCacheEntry{}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := awsclient.EnrichSGUsage(context.Background(), &awsclient.ServiceClients{}, sgs, tc.cache)
			if err != nil {
				t.Fatalf("EnrichSGUsage: %v", err)
			}
			if len(res.Findings["sg-0unused1111111111"]) != 0 {
				t.Errorf("got %+v, want no finding", res.Findings["sg-0unused1111111111"])
			}
			if res.Findings == nil || res.TruncatedIDs == nil {
				t.Error("result maps must be non-nil even when nothing is emitted")
			}
		})
	}
}

// TestW3EnrichSGUsage_DefaultGroupExempt pins the exemption: a default group
// cannot be deleted, so reporting it as unused is noise an operator can never
// clear.
func TestW3EnrichSGUsage_DefaultGroupExempt(t *testing.T) {
	sgs := []resource.Resource{w3SGRes("sg-0default11111111", "default")}
	res, err := awsclient.EnrichSGUsage(context.Background(), &awsclient.ServiceClients{}, sgs,
		w3ENICache(false, w3ENIRes("eni-0aaa", "sg-0other333333333")))
	if err != nil {
		t.Fatalf("EnrichSGUsage: %v", err)
	}
	if len(res.Findings["sg-0default11111111"]) != 0 {
		t.Errorf("default group flagged as unused: %+v", res.Findings["sg-0default11111111"])
	}
}

// TestW3EnrichSGUsage_MatchesInsideCommaList pins that the join reads every
// group id on an ENI, not just the first — the eni fetcher comma-joins them
// into a single field.
func TestW3EnrichSGUsage_MatchesInsideCommaList(t *testing.T) {
	sgs := []resource.Resource{w3SGRes("sg-0inuse22222222222", "acme-api")}
	res, err := awsclient.EnrichSGUsage(context.Background(), &awsclient.ServiceClients{}, sgs,
		w3ENICache(false, w3ENIRes("eni-0aaa", "sg-0first111111111", "sg-0inuse22222222222", "sg-0last9999999999")))
	if err != nil {
		t.Fatalf("EnrichSGUsage: %v", err)
	}
	if len(res.Findings["sg-0inuse22222222222"]) != 0 {
		t.Errorf("group referenced in the middle of the list was flagged: %+v", res.Findings["sg-0inuse22222222222"])
	}
}

// TestW3EnrichSGUsage_SubstringIsNotAReference pins exact id matching: an ENI
// carrying "sg-0inuse22222222222x" does not reference "sg-0inuse22222222222".
func TestW3EnrichSGUsage_SubstringIsNotAReference(t *testing.T) {
	sgs := []resource.Resource{w3SGRes("sg-0abc123", "acme-orphan")}
	res, err := awsclient.EnrichSGUsage(context.Background(), &awsclient.ServiceClients{}, sgs,
		w3ENICache(false, w3ENIRes("eni-0aaa", "sg-0abc123456789")))
	if err != nil {
		t.Fatalf("EnrichSGUsage: %v", err)
	}
	if _, ok := w3FindingByCode(res.Findings["sg-0abc123"], w3CodeSGUnused); !ok {
		t.Errorf("sg-0abc123 is referenced by nothing (sg-0abc123456789 is a different group) but was not flagged")
	}
}

// TestW3SGFetch_APIErrorSurfaces guards the fetcher contract the posture rows
// ride on: a failed DescribeSecurityGroups reports an error rather than an
// empty, confidently-healthy list.
func TestW3SGFetch_APIErrorSurfaces(t *testing.T) {
	_, err := awsclient.FetchSecurityGroupsPage(context.Background(), w3SGAPI{err: errors.New("AccessDenied")}, "")
	if err == nil {
		t.Fatal("DescribeSecurityGroups failure returned no error")
	}
}

// TestW3EnrichSGUsage_EmptyButCompleteENIListStillProves pins the other side
// of the truncation rule: an ENI list that came back complete and empty is a
// real answer — the account genuinely has no interfaces — so the groups it
// fails to mention really are unattached. Bailing out on len == 0 would make
// the check silently useless in exactly the account that needs it.
func TestW3EnrichSGUsage_EmptyButCompleteENIListStillProves(t *testing.T) {
	sgs := []resource.Resource{w3SGRes("sg-0unused1111111111", "acme-orphan")}
	res, err := awsclient.EnrichSGUsage(context.Background(), &awsclient.ServiceClients{}, sgs, w3ENICache(false))
	if err != nil {
		t.Fatalf("EnrichSGUsage: %v", err)
	}
	if _, ok := w3FindingByCode(res.Findings["sg-0unused1111111111"], w3CodeSGUnused); !ok {
		t.Errorf("complete, empty ENI list produced no %s; findings=%+v", w3CodeSGUnused, res.Findings)
	}
}

// TestW3EnrichSGUsage_ENIWithNoGroupsReferencesNothing pins that an interface
// carrying an empty security-group field is not treated as a reference to
// some group — splitting "" on a comma yields one empty element, and a set
// built from it must not match a real group id.
func TestW3EnrichSGUsage_ENIWithNoGroupsReferencesNothing(t *testing.T) {
	sgs := []resource.Resource{w3SGRes("sg-0unused1111111111", "acme-orphan")}
	res, err := awsclient.EnrichSGUsage(context.Background(), &awsclient.ServiceClients{}, sgs,
		w3ENICache(false, w3ENIRes("eni-0aaa")))
	if err != nil {
		t.Fatalf("EnrichSGUsage: %v", err)
	}
	if _, ok := w3FindingByCode(res.Findings["sg-0unused1111111111"], w3CodeSGUnused); !ok {
		t.Errorf("group unreferenced by a group-less ENI produced no %s; findings=%+v", w3CodeSGUnused, res.Findings)
	}
}
