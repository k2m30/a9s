package unit

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	devR2SubnetNAT = "subnet-0d1111aaaa2222ddd"
	devR2RTBNAT    = "rtb-0d1111aaaa2222ddd"
	devR2SSHv4SG   = "sg-0devr2sshv4aaaaa1"
	devR2ClosedSG  = "sg-0devr2closedaaaa1"
)

// devR2Table is a route table explicitly associated with subnet, carrying the
// given default routes.
func devR2Table(id, subnet string, routes ...ec2types.Route) ec2types.RouteTable {
	return ec2types.RouteTable{
		RouteTableId: aws.String(id),
		VpcId:        aws.String(t562VPC),
		OwnerId:      aws.String("123456789012"),
		Associations: []ec2types.RouteTableAssociation{{
			RouteTableAssociationId: aws.String("rtbassoc-" + id[len("rtb-"):]),
			RouteTableId:            aws.String(id),
			SubnetId:                aws.String(subnet),
			Main:                    aws.Bool(false),
		}},
		Routes: append([]ec2types.Route{
			{DestinationCidrBlock: aws.String("10.0.0.0/16"), GatewayId: aws.String("local"), State: ec2types.RouteStateActive},
		}, routes...),
	}
}

func devR2V4IGW() ec2types.Route {
	return ec2types.Route{DestinationCidrBlock: aws.String("0.0.0.0/0"), GatewayId: aws.String(t562IGW), State: ec2types.RouteStateActive}
}

func devR2V4NAT() ec2types.Route {
	return ec2types.Route{DestinationCidrBlock: aws.String("0.0.0.0/0"), NatGatewayId: aws.String("nat-0a1b2c3d4e5f60718"), State: ec2types.RouteStateActive}
}

// devR2RTB is a complete rtb list: subnet B routes both families to the
// internet gateway, subnet C sends ::/0 to an egress-only gateway, and the
// NAT subnet sends 0.0.0.0/0 to a NAT gateway.
func devR2RTB(t *testing.T) resource.ResourceCacheEntry {
	t.Helper()
	return t562RTBEntry(t, false,
		devR2Table(t562RTBB, t562SubnetB, devR2V4IGW(), t562IGWRoute()),
		devR2Table(t562RTBC, t562SubnetC, devR2V4NAT(), t562EIGWRoute()),
		devR2Table(devR2RTBNAT, devR2SubnetNAT, devR2V4NAT()),
	)
}

func devR2SSHv4(id string) ec2types.SecurityGroup {
	return ec2types.SecurityGroup{
		GroupId: aws.String(id), GroupName: aws.String("acme-ssh-v4"), VpcId: aws.String(t562VPC),
		OwnerId: aws.String("123456789012"), Description: aws.String("ssh over ipv4"),
		IpPermissions: []ec2types.IpPermission{{
			IpProtocol: aws.String("tcp"), FromPort: aws.Int32(22), ToPort: aws.Int32(22),
			IpRanges: []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}},
		}},
	}
}

func devR2Cache(t *testing.T, groups ...ec2types.SecurityGroup) resource.ResourceCache {
	t.Helper()
	cache := pw1SGCache(t, groups...)
	cache["rtb"] = devR2RTB(t)
	return cache
}

func devR2Group(id string) []ec2types.GroupIdentifier {
	return []ec2types.GroupIdentifier{{GroupId: aws.String(id), GroupName: aws.String("acme-" + id)}}
}

// devR2Instance is a running instance with the given interfaces; its
// instance-level fields mirror the primary (first) interface the way EC2
// reports them.
func devR2Instance(id string, enis ...ec2types.InstanceNetworkInterface) ec2types.Instance {
	inst := pw1Instance(id, "running", "", ec2types.HttpTokensStateRequired)
	inst.VpcId = aws.String(t562VPC)
	inst.NetworkInterfaces = enis
	if len(enis) > 0 {
		inst.SubnetId = enis[0].SubnetId
		inst.SecurityGroups = enis[0].Groups
		inst.PrivateIpAddress = enis[0].PrivateIpAddress
		if enis[0].Association != nil {
			inst.PublicIpAddress = enis[0].Association.PublicIp
		}
	}
	return inst
}

func devR2ENI(n int, subnet, privateIP string, groups []ec2types.GroupIdentifier) ec2types.InstanceNetworkInterface {
	return ec2types.InstanceNetworkInterface{
		NetworkInterfaceId: aws.String("eni-0devr2" + strings.Repeat(string(rune('a'+n)), 10)),
		SubnetId:           aws.String(subnet),
		VpcId:              aws.String(t562VPC),
		PrivateIpAddress:   aws.String(privateIP),
		Groups:             groups,
		Attachment:         &ec2types.InstanceNetworkInterfaceAttachment{DeviceIndex: aws.Int32(int32(n))},
		PrivateIpAddresses: []ec2types.InstancePrivateIpAddress{{PrivateIpAddress: aws.String(privateIP), Primary: aws.Bool(true)}},
	}
}

func devR2Row(res awsclient.IssueEnricherResult, id string, code domain.FindingCode, label string) string {
	for _, r := range res.AttentionDetails[id][code].Rows {
		if r.Label == label {
			return r.Value
		}
	}
	return ""
}

// Security groups and routes belong to the interface: an IPv6 address on a
// secondary interface in an internet-routed subnet, behind that interface's
// open group, is exposed although the primary interface sits behind an
// egress-only gateway and a closed group.
func TestDevT562_ExposureJudgedPerInterface_SecondaryIPv6(t *testing.T) {
	const id = "i-0devr2eni6000aaa1"
	primary := devR2ENI(0, t562SubnetC, "10.0.3.10", devR2Group(devR2ClosedSG))
	primary.Ipv6Addresses = []ec2types.InstanceIpv6Address{{Ipv6Address: aws.String(t562IPv6C)}}
	secondary := devR2ENI(1, t562SubnetB, "10.0.1.10", devR2Group(t562SSHv6SG))
	secondary.Ipv6Addresses = []ec2types.InstanceIpv6Address{{Ipv6Address: aws.String(t562IPv6B)}}

	res := t562EnrichEC2(t, devR2Cache(t, pw1ClosedSG(devR2ClosedSG), t562SSHv6Only(t562SSHv6SG)), devR2Instance(id, primary, secondary))
	pw1RequireFinding(t, res.Findings[id], pw1EC2CodeInternetExposed, "port 22 reachable from the internet", domain.SevBroken, "wave2")
	if got := devR2Row(res, id, pw1EC2CodeInternetExposed, "Public address"); got != t562IPv6B {
		t.Errorf("Public address = %q, want the secondary interface's %s", got, t562IPv6B)
	}
	if reason, marked := res.TruncatedIDs[id]; marked {
		t.Errorf("marked not inspected (%q); every list was loaded", reason)
	}
}

// An Elastic IP on a secondary interface's private address is a public IPv4
// address of the instance, judged against that interface's groups.
func TestDevT562_ExposureJudgedPerInterface_ElasticIPOnSecondary(t *testing.T) {
	const id = "i-0devr2eip0000aaa1"
	primary := devR2ENI(0, t562SubnetB, "10.0.1.20", devR2Group(devR2ClosedSG))
	secondary := devR2ENI(1, t562SubnetB, "10.0.1.21", devR2Group(devR2SSHv4SG))
	secondary.PrivateIpAddresses[0].Association = &ec2types.InstanceNetworkInterfaceAssociation{PublicIp: aws.String("203.0.113.60")}

	res := t562EnrichEC2(t, devR2Cache(t, pw1ClosedSG(devR2ClosedSG), devR2SSHv4(devR2SSHv4SG)), devR2Instance(id, primary, secondary))
	pw1RequireFinding(t, res.Findings[id], pw1EC2CodeInternetExposed, "port 22 reachable from the internet", domain.SevBroken, "wave2")
	if got := devR2Row(res, id, pw1EC2CodeInternetExposed, "Public address"); got != "203.0.113.60" {
		t.Errorf("Public address = %q, want the Elastic IP 203.0.113.60", got)
	}
}

// A group on an interface that holds no public address exposes nothing, even
// when another interface of the instance holds one.
func TestDevT562_ExposureJudgedPerInterface_OpenGroupOnPrivateInterface(t *testing.T) {
	const id = "i-0devr2priv000aaa1"
	primary := devR2ENI(0, t562SubnetB, "10.0.1.30", devR2Group(devR2ClosedSG))
	primary.Association = &ec2types.InstanceNetworkInterfaceAssociation{PublicIp: aws.String("203.0.113.61")}
	primary.PrivateIpAddresses[0].Association = primary.Association
	secondary := devR2ENI(1, t562SubnetB, "10.0.1.31", devR2Group(devR2SSHv4SG))

	res := t562EnrichEC2(t, devR2Cache(t, pw1ClosedSG(devR2ClosedSG), devR2SSHv4(devR2SSHv4SG)), devR2Instance(id, primary, secondary))
	t562RequireNoExposure(t, res, id)
}

// A found exposure whose other group went unread is a partial verdict and
// reads as one: the finding stands and the row carries the incomplete mark.
func TestDevT562_PartialVerdictCarriesTheMark(t *testing.T) {
	const id = "i-0devr2part000aaa1"
	eni := devR2ENI(0, t562SubnetB, "10.0.1.40", append(devR2Group(devR2SSHv4SG), devR2Group("sg-0devr2unreadaaaa1")...))
	eni.Association = &ec2types.InstanceNetworkInterfaceAssociation{PublicIp: aws.String("203.0.113.62")}
	eni.PrivateIpAddresses[0].Association = eni.Association
	cache := devR2Cache(t, devR2SSHv4(devR2SSHv4SG))
	cache["sg"] = resource.ResourceCacheEntry{Resources: cache["sg"].Resources, IsTruncated: true}

	res := t562EnrichEC2(t, cache, devR2Instance(id, eni))
	pw1RequireFinding(t, res.Findings[id], pw1EC2CodeInternetExposed, "port 22 reachable from the internet", domain.SevBroken, "wave2")
	if got := res.TruncatedIDs[id]; got != "sg list incomplete" {
		t.Errorf("TruncatedIDs[%s] = %q, want \"sg list incomplete\" beside the partial finding", id, got)
	}
}

// A public IPv4 address reaches the internet only through a 0.0.0.0/0 route
// to an internet gateway in its subnet's table; behind a NAT gateway nothing
// arrives, and an unread route table decides nothing.
func TestDevT562_IPv4RouteRule(t *testing.T) {
	mk := func(id, subnet string) ec2types.Instance {
		eni := devR2ENI(0, subnet, "10.0.9.10", devR2Group(devR2SSHv4SG))
		eni.Association = &ec2types.InstanceNetworkInterfaceAssociation{PublicIp: aws.String("203.0.113.70")}
		eni.PrivateIpAddresses[0].Association = eni.Association
		return devR2Instance(id, eni)
	}
	routed, natted, unread := "i-0devr2v4igw00aaa1", "i-0devr2v4nat00aaa1", "i-0devr2v4unk00aaa1"

	res := t562EnrichEC2(t, devR2Cache(t, devR2SSHv4(devR2SSHv4SG)), mk(routed, t562SubnetB), mk(natted, devR2SubnetNAT))
	pw1RequireFinding(t, res.Findings[routed], pw1EC2CodeInternetExposed, "port 22 reachable from the internet", domain.SevBroken, "wave2")
	t562RequireNoExposure(t, res, natted)
	if reason, marked := res.TruncatedIDs[natted]; marked {
		t.Errorf("%s marked not inspected (%q); its route table was read", natted, reason)
	}

	noRTB := pw1SGCache(t, devR2SSHv4(devR2SSHv4SG))
	res = t562EnrichEC2(t, noRTB, mk(unread, t562SubnetB))
	t562RequireNoExposure(t, res, unread)
	if got := res.TruncatedIDs[unread]; got != "rtb list incomplete" {
		t.Errorf("TruncatedIDs[%s] = %q, want \"rtb list incomplete\"", unread, got)
	}
}

// ─── S3: RestrictPublicBuckets ────────────────────────────────────────────

type devR2PABDenied struct{ *t562S3Fake }

func (devR2PABDenied) GetPublicAccessBlock(_ context.Context, _ *s3.GetPublicAccessBlockInput, _ ...func(*s3.Options)) (*s3.GetPublicAccessBlockOutput, error) {
	return nil, &smithy.GenericAPIError{Code: "AccessDenied", Message: "Access Denied", Fault: smithy.FaultClient}
}

// With RestrictPublicBuckets on, S3 serves a public policy only to AWS service
// principals and the owner's own account: the internet is not let in.
func TestDevT562_RestrictPublicBucketsClosesAPublicPolicy(t *testing.T) {
	trailPublic, s3Public := t562Verdicts(t, &t562S3Fake{
		policyPublic: true,
		pab: &s3types.PublicAccessBlockConfiguration{
			BlockPublicAcls: aws.Bool(true), IgnorePublicAcls: aws.Bool(true),
			BlockPublicPolicy: aws.Bool(false), RestrictPublicBuckets: aws.Bool(true),
		},
	})
	if trailPublic || s3Public {
		t.Errorf("trail public = %v, s3 public = %v, want both false under RestrictPublicBuckets", trailPublic, s3Public)
	}
}

// A policy status read against an unread block is unknown on both passes.
func TestDevT562_PolicyVerdictNeedsTheBlock(t *testing.T) {
	fake := devR2PABDenied{&t562S3Fake{policyPublic: true}}
	clients := &awsclient.ServiceClients{S3: fake, Region: "us-east-1"}
	trails, err := awsclient.FetchCloudTrailTrails(context.Background(), t562TrailListFake{})
	if err != nil {
		t.Fatal(err)
	}
	tr, _ := awsclient.EnrichTrailLogBucket(context.Background(), clients, trails, nil) //nolint:errcheck // the refusal is the input; the row is under test
	bucket := []resource.Resource{{ID: t562TrailBucket, Name: t562TrailBucket, Fields: map[string]string{"name": t562TrailBucket}}}
	sr, _ := awsclient.EnrichS3Posture(context.Background(), clients, bucket, nil) //nolint:errcheck // same
	if hasCode(tr.Findings[trails[0].ID], awsclient.CodeTrailLogBucketPublic) || hasCode(sr.Findings[t562TrailBucket], t562S3Public) {
		t.Errorf("public verdict reported with the public access block unread: trail %v, s3 %v", tr.Findings, sr.Findings)
	}
	if _, marked := tr.TruncatedIDs[trails[0].ID]; !marked {
		t.Error("trail not marked not inspected")
	}
	if _, marked := sr.TruncatedIDs[t562TrailBucket]; !marked {
		t.Error("bucket not marked not inspected")
	}
}

// ─── Coalescing wrappers ───────────────────────────────────────────────────

type devR2ECSFake struct {
	awsclient.ECSAPI
	errs  []error
	calls atomic.Int64
}

func (f *devR2ECSFake) DescribeTaskDefinition(_ context.Context, in *ecs.DescribeTaskDefinitionInput, _ ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
	n := int(f.calls.Add(1)) - 1
	if n < len(f.errs) && f.errs[n] != nil {
		return nil, f.errs[n]
	}
	return &ecs.DescribeTaskDefinitionOutput{TaskDefinition: &ecstypes.TaskDefinition{TaskDefinitionArn: in.TaskDefinition}}, nil
}

// A throttle within one operation is asked again; a definitive refusal is the
// operation's answer and is not.
func TestDevT562_ECSMemoizesOnlyDefinitiveAnswers(t *testing.T) {
	ctx := awsclient.WithDetailOp(context.Background(), domain.Gen(562))
	in := &ecs.DescribeTaskDefinitionInput{TaskDefinition: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/acme-api:7")}

	throttled := &devR2ECSFake{errs: []error{&smithy.GenericAPIError{Code: "ThrottlingException", Message: "Rate exceeded"}}}
	c := awsclient.NewCoalescingECS(throttled)
	_, _ = c.DescribeTaskDefinition(ctx, in) //nolint:errcheck // the throttle is the input
	if _, err := c.DescribeTaskDefinition(ctx, in); err != nil || throttled.calls.Load() != 2 {
		t.Errorf("after a throttle: err = %v, calls = %d, want a second real call that succeeds", err, throttled.calls.Load())
	}

	deadline := &devR2ECSFake{errs: []error{context.DeadlineExceeded}}
	c = awsclient.NewCoalescingECS(deadline)
	_, _ = c.DescribeTaskDefinition(ctx, in) //nolint:errcheck // same
	if _, err := c.DescribeTaskDefinition(ctx, in); err != nil || deadline.calls.Load() != 2 {
		t.Errorf("after a deadline: err = %v, calls = %d, want a second real call that succeeds", err, deadline.calls.Load())
	}

	refused := &devR2ECSFake{errs: []error{&ecstypes.ClientException{Message: aws.String("Unable to describe task definition.")}}}
	c = awsclient.NewCoalescingECS(refused)
	_, _ = c.DescribeTaskDefinition(ctx, in) //nolint:errcheck // same
	if _, err := c.DescribeTaskDefinition(ctx, in); err == nil || refused.calls.Load() != 1 {
		t.Errorf("after a refusal: err = %v, calls = %d, want the refusal served from the operation", err, refused.calls.Load())
	}
}

type devR2LambdaFake struct {
	coalesceLambdaFake
	release chan struct{}
	entered chan struct{}
	once    sync.Once
}

func (f *devR2LambdaFake) GetFunction(ctx context.Context, in *lambda.GetFunctionInput, _ ...func(*lambda.Options)) (*lambda.GetFunctionOutput, error) {
	f.once.Do(func() { close(f.entered) })
	<-f.release
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &lambda.GetFunctionOutput{Configuration: &lambdatypes.FunctionConfiguration{FunctionName: in.FunctionName}}, nil
}

// A caller joining a flight gets the flight's answer on its own terms: the
// caller that started the flight giving up does not fail the flight.
func TestDevT562_FlightOutlivesItsLeadersCancellation(t *testing.T) {
	fake := &devR2LambdaFake{release: make(chan struct{}), entered: make(chan struct{})}
	c := awsclient.NewCoalescingLambda(fake)
	in := &lambda.GetFunctionInput{FunctionName: aws.String("acme-api")}
	op := awsclient.WithDetailOp(context.Background(), domain.Gen(563))

	leaderCtx, cancel := context.WithCancel(op)
	leaderDone := make(chan error, 1)
	go func() {
		_, err := c.GetFunction(leaderCtx, in)
		leaderDone <- err
	}()
	<-fake.entered

	joinerDone := make(chan error, 1)
	go func() {
		_, err := c.GetFunction(op, in)
		joinerDone <- err
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case err := <-leaderDone:
		if err == nil {
			t.Error("the cancelled leader returned no error")
		}
	case <-time.After(2 * time.Second):
		t.Error("the cancelled leader is still waiting on the flight")
	}
	close(fake.release)
	select {
	case err := <-joinerDone:
		if err != nil {
			t.Errorf("joiner err = %v, want the flight's answer: its own context is live", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("joiner never returned")
	}
}

// ─── lt: truncated ami list ───────────────────────────────────────────────

// A launch template whose AMI is absent from a truncated ami list may use a
// deprecated one on an unread page.
func TestDevT562_LTAMIOffATruncatedListReadsNotInspected(t *testing.T) {
	lt := pw1LTResource("lt-0devr2aaaa1111aaa", "acme-web", 3, "")
	other := resource.Resource{ID: "ami-0other000000000001", RawStruct: ec2types.Image{
		ImageId: aws.String("ami-0other000000000001"), DeprecationTime: aws.String("2020-01-01T00:00:00Z"),
	}}
	res := pw1EnrichLT(t, resource.ResourceCache{"ami": {Resources: []resource.Resource{other}, IsTruncated: true}}, lt)
	if got := res.TruncatedIDs[lt.ID]; got != "ami list incomplete" {
		t.Errorf("TruncatedIDs[%s] = %q, want \"ami list incomplete\"", lt.ID, got)
	}

	res = pw1EnrichLT(t, resource.ResourceCache{"ami": {Resources: []resource.Resource{}, IsTruncated: true}}, lt)
	if got := res.TruncatedIDs[lt.ID]; got != "ami list incomplete" {
		t.Errorf("no deprecated AMI loaded: TruncatedIDs[%s] = %q, want \"ami list incomplete\"", lt.ID, got)
	}
}

// ─── sg: the phrase names the range actually open ─────────────────────────

func TestDevT562_SGPhraseNamesTheOpenRange(t *testing.T) {
	both := t562SSHv6Only("sg-0devr2both000aaa1")
	both.IpPermissions[0].IpRanges = []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}
	wide6 := devR2Wide6("sg-0devr2wide600aaa1")
	cases := map[string]struct {
		sg   ec2types.SecurityGroup
		want string
	}{
		"ipv6 only": {t562SSHv6Only("sg-0devr2v6only0aaa1"), "port 22 open to ::/0"},
		"ipv4 only": {devR2SSHv4("sg-0devr2v4only0aaa1"), "port 22 open to 0.0.0.0/0"},
		"both":      {both, "port 22 open to 0.0.0.0/0 and ::/0"},
		"wide ipv6": {wide6, "all ports open to ::/0"},
	}
	for name, tc := range cases {
		r := pw1SGCache(t, tc.sg)["sg"].Resources[0]
		if got := domain.StatusPhrase(r.Findings); got != tc.want {
			t.Errorf("%s: status = %q, want %q", name, got, tc.want)
		}
	}
}

func devR2Wide6(id string) ec2types.SecurityGroup {
	return ec2types.SecurityGroup{
		GroupId: aws.String(id), GroupName: aws.String("acme-wide6"), VpcId: aws.String(t562VPC),
		OwnerId: aws.String("123456789012"), Description: aws.String("all over ipv6"),
		IpPermissions: []ec2types.IpPermission{{IpProtocol: aws.String("-1"), Ipv6Ranges: []ec2types.Ipv6Range{{CidrIpv6: aws.String("::/0")}}}},
	}
}
