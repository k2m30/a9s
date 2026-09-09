package unit

// parse_partition_cidr_test.go — a partition, a region and an address range
// are read out of the value, never guessed from its spelling.
//
// Three facts share one shape: a string AWS returned carries the answer, and
// every site that needs it derives it the same way. A site that instead
// matches one literal spelling ("0.0.0.0/0", "arn:aws:") or slices a string by
// position (the bucket up to the first dot, the region as the zone minus its
// trailing letter) is right only for the one input it was written against and
// silently wrong for every other — and silently wrong here means a resource
// open to the internet renders restricted, a China distribution's origin
// checks never run, and a GovCloud volume reports itself unprotected.
//
// Each test drives the fetcher, enricher or related checker the app itself
// calls, so what is pinned is what an operator sees.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
)

const (
	parseAccount   = "123456789012"
	parseCNRegion  = "cn-north-1"
	parseGovRegion = "us-gov-west-1"
)

// parseCheckerFor returns the registered related checker the app runs, so the
// test enters through the catalog rather than a hand-picked function.
func parseCheckerFor(t *testing.T, shortName, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated(shortName) {
		if def.TargetType == target && def.Checker != nil {
			return def.Checker
		}
	}
	t.Fatalf("no %s related checker for target %q", shortName, target)
	return nil
}

// parseClients builds a session pinned to one region with its account already
// resolved, which is what every ARN-building site reads.
func parseClients(region string) *awsclient.ServiceClients {
	c := &awsclient.ServiceClients{Region: region}
	store := session.NewIdentityStore()
	store.Set(parseAccount, nil)
	c.SetIdentityStore(store)
	return c
}

// ── Row 1: a /0 prefix is everyone, in either family ──────────────────────

// TestCIDROpenToEveryone_ZeroLengthPrefixInEitherFamily pins the rule the
// helper owns: reachability is a property of the prefix length, not of the
// string. An IPv6 default route reaches every IPv6 client on the internet
// exactly as 0.0.0.0/0 reaches every IPv4 one, so a dual-stack resource
// carrying only "::/0" is open. Anything longer than /0 names a subset and is
// not, and a value that is not a prefix at all names no range to judge.
func TestCIDROpenToEveryone_ZeroLengthPrefixInEitherFamily(t *testing.T) {
	tests := []struct {
		name  string
		cidrs []string
		want  bool
	}{
		{name: "IPv4 default route", cidrs: []string{"0.0.0.0/0"}, want: true},
		{name: "IPv6 default route", cidrs: []string{"::/0"}, want: true},
		{name: "both families open", cidrs: []string{"0.0.0.0/0", "::/0"}, want: true},
		{name: "IPv6 default route beside a narrow IPv4 range", cidrs: []string{"10.0.0.0/8", "::/0"}, want: true},
		// A /0 covers the whole space whatever base address is written in
		// front of it, so the answer cannot depend on AWS echoing back one
		// canonical spelling.
		{name: "zero-length prefix on a non-zero base", cidrs: []string{"10.0.0.0/0"}, want: true},
		{name: "expanded IPv6 default route", cidrs: []string{"0000:0000:0000:0000:0000:0000:0000:0000/0"}, want: true},
		{name: "private IPv4 range", cidrs: []string{"10.0.0.0/8"}, want: false},
		{name: "documentation IPv6 range", cidrs: []string{"2001:db8::/32"}, want: false},
		{name: "half the IPv4 space is still not all of it", cidrs: []string{"0.0.0.0/1"}, want: false},
		{name: "half the IPv6 space is still not all of it", cidrs: []string{"::/1"}, want: false},
		{name: "no ranges at all", cidrs: nil, want: false},
		{name: "a value that is not a prefix", cidrs: []string{"everyone"}, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := awsclient.CIDROpenToEveryone(tc.cidrs); got != tc.want {
				t.Errorf("CIDROpenToEveryone(%v) = %v, want %v", tc.cidrs, got, tc.want)
			}
		})
	}
}

// parseEKSCluster drives the real EKS fetcher over one cluster whose public
// endpoint is reachable from the given ranges.
func parseEKSCluster(t *testing.T, publicAccess bool, cidrs []string) resource.Resource {
	t.Helper()
	list := &mockEKSListClustersClient{output: &eks.ListClustersOutput{Clusters: []string{"acme-prod"}}}
	describe := &mockEKSDescribeClusterClient{outputs: map[string]*eks.DescribeClusterOutput{
		"acme-prod": {Cluster: &ekstypes.Cluster{
			Name:            aws.String("acme-prod"),
			Arn:             aws.String("arn:aws:eks:us-east-1:123456789012:cluster/acme-prod"),
			Version:         aws.String("1.30"),
			Status:          ekstypes.ClusterStatusActive,
			Endpoint:        aws.String("https://ABCDEF1234567890.gr7.us-east-1.eks.amazonaws.com"),
			PlatformVersion: aws.String("eks.5"),
			ResourcesVpcConfig: &ekstypes.VpcConfigResponse{
				EndpointPublicAccess:  publicAccess,
				EndpointPrivateAccess: true,
				PublicAccessCidrs:     cidrs,
				SubnetIds:             []string{"subnet-0a1b2c3d4e5f60001", "subnet-0a1b2c3d4e5f60002"},
				VpcId:                 aws.String("vpc-0a1b2c3d4e5f60001"),
			},
		}},
	}}
	clients := &awsclient.ServiceClients{EKS: newMockEKSFull(list, describe, nil, nil)}
	res, err := awsclient.FetchEKSClustersPage(context.Background(), clients, "")
	if err != nil {
		t.Fatalf("FetchEKSClustersPage: %v", err)
	}
	if len(res.Resources) != 1 {
		t.Fatalf("resources = %d, want 1", len(res.Resources))
	}
	return res.Resources[0]
}

// TestEKSPublicEndpoint_OpenIsDecidedByThePrefixNotTheSpelling pins the
// severity an operator reads off the list. A cluster whose control plane
// accepts connections from every IPv6 address on the internet is as exposed as
// one open to every IPv4 address, and reporting it as "restricted" tells the
// operator a narrowing exists that does not.
func TestEKSPublicEndpoint_OpenIsDecidedByThePrefixNotTheSpelling(t *testing.T) {
	const openCode = domain.FindingCode("eks.public-endpoint")
	const openPhrase = "cluster endpoint reachable from the internet"
	// Inverted for the spec row that gave severity one owner: the scoped case
	// is its own code, because it is its own tier and a code declares one.
	// Do not restore a second severity under eks.public-endpoint.
	const scopedCode = domain.FindingCode("eks.public-endpoint-restricted")
	const scopedPhrase = "cluster endpoint reachable from listed networks"

	tests := []struct {
		name         string
		cidrs        []string
		wantWord     string
		wantCode     domain.FindingCode
		wantPhrase   string
		wantSeverity domain.Severity
	}{
		{name: "IPv6 default route", cidrs: []string{"::/0"}, wantWord: "open", wantCode: openCode, wantPhrase: openPhrase, wantSeverity: domain.SevBroken},
		{name: "IPv4 default route", cidrs: []string{"0.0.0.0/0"}, wantWord: "open", wantCode: openCode, wantPhrase: openPhrase, wantSeverity: domain.SevBroken},
		{name: "both families open", cidrs: []string{"0.0.0.0/0", "::/0"}, wantWord: "open", wantCode: openCode, wantPhrase: openPhrase, wantSeverity: domain.SevBroken},
		{name: "IPv6 open beside a narrowed IPv4 range", cidrs: []string{"10.0.0.0/8", "::/0"}, wantWord: "open", wantCode: openCode, wantPhrase: openPhrase, wantSeverity: domain.SevBroken},
		{name: "narrowed in both families", cidrs: []string{"10.0.0.0/8", "2001:db8::/32"}, wantWord: "restricted", wantCode: scopedCode, wantPhrase: scopedPhrase, wantSeverity: domain.SevWarn},
		// AWS omits the list when it was never narrowed, which is the open
		// case and is decided at the site, not by the range rule.
		{name: "AWS omitted the list", cidrs: nil, wantWord: "open", wantCode: openCode, wantPhrase: openPhrase, wantSeverity: domain.SevBroken},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := parseEKSCluster(t, true, tc.cidrs)
			if r.Fields["public_endpoint"] != tc.wantWord {
				t.Errorf("Fields[public_endpoint] = %q, want %q", r.Fields["public_endpoint"], tc.wantWord)
			}
			w4AssertFinding(t, r.Findings, tc.wantCode, tc.wantPhrase, tc.wantSeverity, "wave1")
		})
	}

	// A cluster with no public endpoint at all carries no exposure finding,
	// so the rule above is reporting a real difference between clusters.
	private := parseEKSCluster(t, false, nil)
	if private.Fields["public_endpoint"] != "no" {
		t.Errorf("Fields[public_endpoint] = %q, want %q", private.Fields["public_endpoint"], "no")
	}
	w4AssertNoCode(t, private.Findings, openCode)
	w4AssertNoCode(t, private.Findings, scopedCode)
}

// TestEKSPublicEndpoint_ReachableFromRowNamesTheRangesAWSSent pins the
// supporting row against the ranges the cluster actually carries, because the
// severity alone does not tell the operator which addresses reach the API
// server.
func TestEKSPublicEndpoint_ReachableFromRowNamesTheRangesAWSSent(t *testing.T) {
	const code = domain.FindingCode("eks.public-endpoint")
	r := parseEKSCluster(t, true, []string{"10.0.0.0/8", "::/0"})
	rows := r.AttentionDetails[code].Rows
	if len(rows) != 1 {
		t.Fatalf("attention rows = %v, want exactly one", rows)
	}
	if rows[0].Label != "Reachable from" || rows[0].Value != "10.0.0.0/8, ::/0" {
		t.Errorf("row = %q: %q, want %q: %q", rows[0].Label, rows[0].Value, "Reachable from", "10.0.0.0/8, ::/0")
	}
}

// parseSGIngress is a realistic DescribeSecurityGroups ingress rule opening SSH
// to the given IPv4 and IPv6 ranges.
func parseSGIngress(ipv4, ipv6 []string) ec2types.IpPermission {
	p := ec2types.IpPermission{
		IpProtocol: aws.String("tcp"),
		FromPort:   aws.Int32(22),
		ToPort:     aws.Int32(22),
	}
	for _, c := range ipv4 {
		p.IpRanges = append(p.IpRanges, ec2types.IpRange{CidrIp: aws.String(c), Description: aws.String("admin")})
	}
	for _, c := range ipv6 {
		p.Ipv6Ranges = append(p.Ipv6Ranges, ec2types.Ipv6Range{CidrIpv6: aws.String(c), Description: aws.String("admin")})
	}
	return p
}

// TestSGInternetFacing_AnyZeroLengthPrefixIsTheInternet pins the second site
// that asks the same question. A security group opening SSH to a /0 is open to
// the internet whichever family the range is in and however the range is
// spelled; deciding it by string equality against two literals makes the
// verdict depend on the exact text AWS echoed back rather than on what the
// rule permits.
func TestSGInternetFacing_AnyZeroLengthPrefixIsTheInternet(t *testing.T) {
	const code = domain.FindingCode("sg.ingress.dangerous-ports")
	// Inverted for aws3 round 2's row on the sg port list: the declaration
	// agrees its noun with the list it names, and this fixture opens one port.
	// Do not restore the unconditional plural.
	const phrase = "port 22 open to 0.0.0.0/0"

	tests := []struct {
		name        string
		ipv4        []string
		ipv6        []string
		wantExposed bool
	}{
		{name: "IPv4 default route", ipv4: []string{"0.0.0.0/0"}, wantExposed: true},
		{name: "IPv6 default route", ipv6: []string{"::/0"}, wantExposed: true},
		{name: "zero-length prefix on a non-zero base", ipv4: []string{"10.0.0.0/0"}, wantExposed: true},
		{name: "expanded IPv6 default route", ipv6: []string{"0000:0000:0000:0000:0000:0000:0000:0000/0"}, wantExposed: true},
		{name: "narrow ranges in both families", ipv4: []string{"10.0.0.0/8"}, ipv6: []string{"2001:db8::/32"}},
		{name: "half the IPv6 space", ipv6: []string{"::/1"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rows := w3FetchSGs(t, ec2types.SecurityGroup{
				GroupId:       aws.String("sg-0a1b2c3d4e5f60001"),
				GroupName:     aws.String("acme-bastion"),
				VpcId:         aws.String("vpc-0a1b2c3d4e5f60001"),
				Description:   aws.String("bastion access"),
				IpPermissions: []ec2types.IpPermission{parseSGIngress(tc.ipv4, tc.ipv6)},
			})
			if len(rows) != 1 {
				t.Fatalf("resources = %d, want 1", len(rows))
			}
			if !tc.wantExposed {
				w4AssertNoCode(t, rows[0].Findings, code)
				if rows[0].Fields["dangerous_open_count"] != "0" {
					t.Errorf("Fields[dangerous_open_count] = %q, want %q", rows[0].Fields["dangerous_open_count"], "0")
				}
				return
			}
			w4AssertFinding(t, rows[0].Findings, code, phrase, domain.SevBroken, "wave1")
			if rows[0].Fields["dangerous_open_count"] != "1" {
				t.Errorf("Fields[dangerous_open_count] = %q, want %q", rows[0].Fields["dangerous_open_count"], "1")
			}
		})
	}
}

// ── Row 2: the S3 origin bucket is the whole prefix ───────────────────────

// TestS3OriginBucket_BucketIsEveryLabelBeforeTheEndpointMarker pins what an S3
// origin domain names. A bucket name may contain dots, so the name is not the
// first label; the endpoint suffix is the partition's, so a China endpoint is
// an S3 endpoint too. Getting either wrong does not merely mislabel the
// origin — it drops the distribution out of both origin checks entirely.
func TestS3OriginBucket_BucketIsEveryLabelBeforeTheEndpointMarker(t *testing.T) {
	tests := []struct {
		name       string
		host       string
		wantBucket string
		wantIsS3   bool
	}{
		{name: "regional endpoint", host: "acme-assets.s3.us-east-1.amazonaws.com", wantBucket: "acme-assets", wantIsS3: true},
		{name: "legacy global endpoint", host: "acme-assets.s3.amazonaws.com", wantBucket: "acme-assets", wantIsS3: true},
		{name: "dotted bucket name", host: "my.bucket.s3.us-east-1.amazonaws.com", wantBucket: "my.bucket", wantIsS3: true},
		{name: "China regional endpoint", host: "acme-assets.s3.cn-north-1.amazonaws.com.cn", wantBucket: "acme-assets", wantIsS3: true},
		{name: "dotted bucket on a China endpoint", host: "my.bucket.s3.cn-northwest-1.amazonaws.com.cn", wantBucket: "my.bucket", wantIsS3: true},
		{name: "website endpoint, dashed region", host: "acme-assets.s3-website-us-east-1.amazonaws.com", wantBucket: "acme-assets", wantIsS3: true},
		{name: "website endpoint, dotted region", host: "acme-assets.s3-website.eu-west-1.amazonaws.com", wantBucket: "acme-assets", wantIsS3: true},
		{name: "dualstack endpoint", host: "acme-assets.s3.dualstack.us-east-1.amazonaws.com", wantBucket: "acme-assets", wantIsS3: true},
		{name: "accelerate endpoint", host: "acme-assets.s3-accelerate.amazonaws.com", wantBucket: "acme-assets", wantIsS3: true},
		{name: "custom origin", host: "origin.acme.example.com"},
		{name: "API Gateway origin", host: "abcdef1234.execute-api.us-east-1.amazonaws.com"},
		{name: "load balancer origin", host: "acme-alb-1234567890.us-east-1.elb.amazonaws.com"},
		{name: "another AWS service on a China endpoint", host: "abcdef1234.execute-api.cn-north-1.amazonaws.com.cn"},
		{name: "empty domain name", host: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bucket, isS3 := awsclient.S3OriginBucket(tc.host)
			if isS3 != tc.wantIsS3 {
				t.Fatalf("S3OriginBucket(%q) isS3 = %v, want %v", tc.host, isS3, tc.wantIsS3)
			}
			if bucket != tc.wantBucket {
				t.Errorf("S3OriginBucket(%q) bucket = %q, want %q", tc.host, bucket, tc.wantBucket)
			}
		})
	}
}

// parseCFOrigin builds one origin, with an origin access control only when
// oac is true.
func parseCFOrigin(domainName string, oac bool) cftypes.Origin {
	o := cftypes.Origin{
		Id:             aws.String("origin-" + domainName),
		DomainName:     aws.String(domainName),
		S3OriginConfig: &cftypes.S3OriginConfig{OriginAccessIdentity: aws.String("")},
	}
	if oac {
		o.OriginAccessControlId = aws.String("E1OACEXAMPLE01")
	}
	return o
}

// TestCloudFrontS3Origin_DottedAndChinaOriginsReachBothChecks pins the
// consequence of the parse for the operator. The origin checks answer two
// questions about the bucket behind a distribution — does it exist, and is it
// reachable only through CloudFront — and an origin the parser fails to
// recognise is not judged as healthy, it is not judged at all.
func TestCloudFrontS3Origin_DottedAndChinaOriginsReachBothChecks(t *testing.T) {
	const missing = awsclient.CodeCFOriginBucketMissing
	const noOAC = awsclient.CodeCFS3OriginNoOAC

	tests := []struct {
		name        string
		host        string
		oac         bool
		wantMissing bool
		wantNoOAC   bool
	}{
		{name: "dotted bucket that exists, no origin access control", host: "my.bucket.s3.us-east-1.amazonaws.com", wantNoOAC: true},
		{name: "China bucket that exists, no origin access control", host: "acme-cn.s3.cn-north-1.amazonaws.com.cn", wantNoOAC: true},
		{name: "dualstack bucket that exists, no origin access control", host: "acme-dual.s3.dualstack.us-east-1.amazonaws.com", wantNoOAC: true},
		// A website endpoint takes neither an origin access control nor an
		// identity, so their absence is not a finding there.
		{name: "website bucket that exists, no origin access control", host: "acme-site.s3-website-us-east-1.amazonaws.com", wantNoOAC: false},
		{name: "dotted bucket that does not exist", host: "gone.away.s3.us-east-1.amazonaws.com", oac: true, wantMissing: true},
		{name: "China bucket that does not exist", host: "gone-cn.s3.cn-north-1.amazonaws.com.cn", oac: true, wantMissing: true},
		{name: "bucket that exists behind an origin access control", host: "acme-ok.s3.us-east-1.amazonaws.com", oac: true},
		{name: "custom origin is not an S3 origin", host: "origin.acme.example.com"},
	}

	// Every bucket the table expects to exist, and nothing else: a name the
	// parser gets wrong will not be in here, which is what makes the
	// "no missing-origin finding" assertions discriminating.
	buckets := []resource.Resource{
		{ID: "my.bucket", Name: "my.bucket"},
		{ID: "acme-cn", Name: "acme-cn"},
		{ID: "acme-dual", Name: "acme-dual"},
		{ID: "acme-site", Name: "acme-site"},
		{ID: "acme-ok", Name: "acme-ok"},
	}
	cache := resource.ResourceCache{"s3": resource.ResourceCacheEntry{Resources: buckets}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			const distID = "E1PARSEEXAMPLE"
			fake := &cfGetDistributionConfigFake{results: map[string]*cftypes.DistributionConfig{
				distID: {
					Comment: aws.String("acme cdn"),
					Origins: &cftypes.Origins{
						Quantity: aws.Int32(1),
						Items:    []cftypes.Origin{parseCFOrigin(tc.host, tc.oac)},
					},
					DefaultCacheBehavior: &cftypes.DefaultCacheBehavior{
						TargetOriginId:       aws.String("origin-" + tc.host),
						ViewerProtocolPolicy: cftypes.ViewerProtocolPolicyRedirectToHttps,
					},
				},
			}}
			// Row 7 of this spec made HeadBucket the authority for "gone",
			// so the session needs one; here every bucket outside the cache
			// is genuinely absent, which is what the table's
			// does-not-exist rows mean.
			head := &w6aHeadBucketFake{exists: map[string]bool{}}
			for _, b := range buckets {
				head.exists[b.ID] = true
			}
			clients := &awsclient.ServiceClients{CloudFront: fake, S3: head}
			res, err := awsclient.EnrichCloudFrontDistribution(
				context.Background(), clients, cfDistroResources(distID), cache)
			if err != nil {
				t.Fatalf("EnrichCloudFrontDistribution: %v", err)
			}

			if tc.wantMissing {
				w4AssertFinding(t, res.Findings[distID], missing,
					"S3 origin bucket does not exist", domain.SevBroken, "wave2")
			} else {
				w4AssertNoCode(t, res.Findings[distID], missing)
			}
			if tc.wantNoOAC {
				w4AssertFinding(t, res.Findings[distID], noOAC,
					"S3 origin without origin access control", domain.SevWarn, "wave2")
			} else {
				w4AssertNoCode(t, res.Findings[distID], noOAC)
			}
		})
	}
}

// ── Row 3: the partition comes from the region ────────────────────────────

// TestPartitionForRegion_EveryRegionNamesItsPartition pins the one place the
// package decides a partition. An ARN built with the wrong one is not
// malformed, it simply never matches anything AWS returned, so the failure
// mode is a confident zero rather than an error.
func TestPartitionForRegion_EveryRegionNamesItsPartition(t *testing.T) {
	tests := []struct {
		region string
		want   string
	}{
		{region: "cn-north-1", want: "aws-cn"},
		{region: "cn-northwest-1", want: "aws-cn"},
		{region: "us-gov-west-1", want: "aws-us-gov"},
		{region: "us-gov-east-1", want: "aws-us-gov"},
		// The rest of the partitions the SDK's own catalogue names
		// (core/aws/data/partitions.json). A region in one of these built a
		// commercial ARN, which matches nothing AWS returned there.
		{region: "us-iso-east-1", want: "aws-iso"},
		{region: "us-isob-east-1", want: "aws-iso-b"},
		{region: "eu-isoe-west-1", want: "aws-iso-e"},
		{region: "us-isof-south-1", want: "aws-iso-f"},
		{region: "eusc-de-east-1", want: "aws-eusc"},
		{region: "us-east-1", want: "aws"},
		{region: "eu-west-1", want: "aws"},
		{region: "ap-southeast-2", want: "aws"},
		// A region whose name merely starts with the same letters is a
		// commercial region: the prefix is a whole segment, not a substring.
		{region: "us-west-2", want: "aws"},
		{region: "cnorth-1", want: "aws"},
		{region: "", want: "aws"},
	}
	for _, tc := range tests {
		t.Run(tc.region, func(t *testing.T) {
			if got := awsclient.PartitionForRegion(tc.region); got != tc.want {
				t.Errorf("PartitionForRegion(%q) = %q, want %q", tc.region, got, tc.want)
			}
		})
	}
}

// parseVolume is an EC2 volume row the way FetchEBSVolumesPage builds it.
func parseVolume(az string) resource.Resource {
	const id = "vol-0a1b2c3d4e5f60001"
	return resource.Resource{
		ID:   id,
		Name: "acme-data",
		Fields: map[string]string{
			"volume_id":   id,
			"name":        "acme-data",
			"state":       "in-use",
			"attached_to": "i-0a1b2c3d4e5f60001",
			"az":          az,
		},
		RawStruct: ec2types.Volume{
			VolumeId:         aws.String(id),
			State:            ec2types.VolumeStateInUse,
			AvailabilityZone: aws.String(az),
			Tags:             []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("acme-data")}},
		},
	}
}

// TestEBSBackupARN_PartitionAndRegionComeFromTheSession pins the ARN the
// backup join matches on. The volume row carries no ARN, so one is built: the
// partition belongs to the session's region, and the region is the session's
// too. An availability zone is not a region — a Local Zone's zone name carries
// an extra segment — so deriving one from the other reports a covered volume
// as unprotected.
func TestEBSBackupARN_PartitionAndRegionComeFromTheSession(t *testing.T) {
	const id = "vol-0a1b2c3d4e5f60001"
	const code = awsclient.CodeEBSNotInBackupPlan

	tests := []struct {
		name          string
		region        string
		az            string
		planARN       string
		wantUncovered bool
	}{
		{
			name:    "China volume named by a China plan",
			region:  parseCNRegion,
			az:      "cn-north-1a",
			planARN: "arn:aws-cn:ec2:cn-north-1:" + parseAccount + ":volume/" + id,
		},
		{
			name:    "GovCloud volume named by a GovCloud plan",
			region:  parseGovRegion,
			az:      "us-gov-west-1a",
			planARN: "arn:aws-us-gov:ec2:us-gov-west-1:" + parseAccount + ":volume/" + id,
		},
		{
			// A Local Zone volume belongs to its parent region; the zone name
			// carries a segment the region does not.
			name:    "Local Zone volume named by its parent region's plan",
			region:  "us-west-2",
			az:      "us-west-2-lax-1a",
			planARN: "arn:aws:ec2:us-west-2:" + parseAccount + ":volume/" + id,
		},
		{
			name:    "commercial volume named by a commercial plan",
			region:  "us-east-1",
			az:      "us-east-1a",
			planARN: "arn:aws:ec2:us-east-1:" + parseAccount + ":volume/" + id,
		},
		{
			// The whole ARN has to match, so a plan naming another region's
			// volume of the same id leaves this one uncovered.
			name:          "plan naming the same volume in another region",
			region:        "us-east-1",
			az:            "us-east-1a",
			planARN:       "arn:aws:ec2:eu-west-1:" + parseAccount + ":volume/" + id,
			wantUncovered: true,
		},
		{
			// Nor does a plan in another partition cover a commercial volume.
			name:          "plan naming the same volume in another partition",
			region:        "us-east-1",
			az:            "us-east-1a",
			planARN:       "arn:aws-cn:ec2:us-east-1:" + parseAccount + ":volume/" + id,
			wantUncovered: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := awsclient.EnrichEBSVolumeStatus(
				context.Background(), parseClients(tc.region),
				[]resource.Resource{parseVolume(tc.az)},
				w7CacheWith(w7Plan(tc.planARN, "", "")))
			if err != nil {
				t.Fatalf("EnrichEBSVolumeStatus: %v", err)
			}
			if tc.wantUncovered {
				w4AssertFinding(t, res.Findings[id], code,
					"not covered by a backup plan", domain.SevWarn, "wave2")
				return
			}
			w4AssertNoCode(t, res.Findings[id], code)
		})
	}
}

// parseGlueTagsFake records the ARN checkGlueCFN asks GetTags about.
type parseGlueTagsFake struct {
	awsclient.GlueAPI
	gotARN string
}

func (f *parseGlueTagsFake) GetTags(
	_ context.Context,
	in *glue.GetTagsInput,
	_ ...func(*glue.Options),
) (*glue.GetTagsOutput, error) {
	f.gotARN = aws.ToString(in.ResourceArn)
	return &glue.GetTagsOutput{Tags: map[string]string{}}, nil
}

// TestGlueCFN_JobARNCarriesTheSessionPartition pins the ARN a glue job's
// CloudFormation lookup asks about. The region is already in hand; only the
// partition was assumed, and GetTags on an ARN from the wrong partition
// answers about nothing.
func TestGlueCFN_JobARNCarriesTheSessionPartition(t *testing.T) {
	tests := []struct {
		region string
		want   string
	}{
		{region: parseCNRegion, want: "arn:aws-cn:glue:cn-north-1:" + parseAccount + ":job/acme-etl-job"},
		{region: parseGovRegion, want: "arn:aws-us-gov:glue:us-gov-west-1:" + parseAccount + ":job/acme-etl-job"},
		{region: "us-east-1", want: "arn:aws:glue:us-east-1:" + parseAccount + ":job/acme-etl-job"},
	}
	checker := parseCheckerFor(t, "glue", "cfn")
	for _, tc := range tests {
		t.Run(tc.region, func(t *testing.T) {
			fake := &parseGlueTagsFake{}
			clients := parseClients(tc.region)
			clients.Glue = fake
			checker(context.Background(),
				clients,
				resource.Resource{ID: "acme-etl-job", Name: "acme-etl-job"},
				resource.ResourceCache{})
			if fake.gotARN != tc.want {
				t.Errorf("GetTags ResourceArn = %q, want %q", fake.gotARN, tc.want)
			}
		})
	}
}

// parseCodePipelineFake serves one pipeline.
type parseCodePipelineFake struct {
	awsclient.CodePipelineAPI
}

func (f *parseCodePipelineFake) ListPipelines(
	_ context.Context,
	_ *codepipeline.ListPipelinesInput,
	_ ...func(*codepipeline.Options),
) (*codepipeline.ListPipelinesOutput, error) {
	return &codepipeline.ListPipelinesOutput{
		Pipelines: []cptypes.PipelineSummary{{
			Name:         aws.String("acme-release"),
			PipelineType: cptypes.PipelineTypeV2,
			Version:      aws.Int32(3),
		}},
	}, nil
}

// TestCodePipelineARN_CarriesTheSessionPartition pins the ARN the pipeline
// fetcher writes onto every row. It is the value every downstream join and
// console link reads, so a partition it guessed wrong is wrong everywhere at
// once.
func TestCodePipelineARN_CarriesTheSessionPartition(t *testing.T) {
	tests := []struct {
		region string
		want   string
	}{
		{region: parseCNRegion, want: "arn:aws-cn:codepipeline:cn-north-1:" + parseAccount + ":acme-release"},
		{region: parseGovRegion, want: "arn:aws-us-gov:codepipeline:us-gov-west-1:" + parseAccount + ":acme-release"},
		{region: "us-east-1", want: "arn:aws:codepipeline:us-east-1:" + parseAccount + ":acme-release"},
	}
	for _, tc := range tests {
		t.Run(tc.region, func(t *testing.T) {
			clients := parseClients(tc.region)
			clients.CodePipeline = &parseCodePipelineFake{}
			res, err := awsclient.FetchCodePipelinesPageWithClients(context.Background(), clients, "")
			if err != nil {
				t.Fatalf("FetchCodePipelinesPageWithClients: %v", err)
			}
			if len(res.Resources) != 1 {
				t.Fatalf("resources = %d, want 1", len(res.Resources))
			}
			if got := res.Resources[0].Fields["arn"]; got != tc.want {
				t.Errorf("Fields[arn] = %q, want %q", got, tc.want)
			}
		})
	}
}

// parseEBTargetsFake serves one rule's targets. The shared ListTargetsByRule
// mock covers only that one call; the checker reaches it through the aggregate
// client, which needs the rest of the interface present.
type parseEBTargetsFake struct {
	awsclient.EventBridgeAPI
	targets []ebtypes.Target
}

func (f *parseEBTargetsFake) ListTargetsByRule(
	_ context.Context,
	_ *eventbridge.ListTargetsByRuleInput,
	_ ...func(*eventbridge.Options),
) (*eventbridge.ListTargetsByRuleOutput, error) {
	return &eventbridge.ListTargetsByRuleOutput{Targets: f.targets}, nil
}

// TestEBRuleTargets_TargetsInTheSessionPartitionAreCounted pins the related
// panel's count. The rule's targets are ARNs AWS returned; filtering them
// against a hard-coded partition prefix discards every one of them in China
// and GovCloud, and the panel then reports a confident zero rather than
// admitting it could not tell.
func TestEBRuleTargets_TargetsInTheSessionPartitionAreCounted(t *testing.T) {
	tests := []struct {
		name      string
		region    string
		targetARN string
		wantIDs   []string
	}{
		{
			name:      "China lambda target",
			region:    parseCNRegion,
			targetARN: "arn:aws-cn:lambda:cn-north-1:" + parseAccount + ":function:acme-fn",
			wantIDs:   []string{"acme-fn"},
		},
		{
			name:      "GovCloud lambda target",
			region:    parseGovRegion,
			targetARN: "arn:aws-us-gov:lambda:us-gov-west-1:" + parseAccount + ":function:acme-fn",
			wantIDs:   []string{"acme-fn"},
		},
		{
			name:      "commercial lambda target",
			region:    "us-east-1",
			targetARN: "arn:aws:lambda:us-east-1:" + parseAccount + ":function:acme-fn",
			wantIDs:   []string{"acme-fn"},
		},
		{
			// The service segment still has to match, so the filter is doing
			// real work and not simply accepting everything.
			name:      "an SQS target is not a lambda target",
			region:    parseCNRegion,
			targetARN: "arn:aws-cn:sqs:cn-north-1:" + parseAccount + ":acme-queue",
		},
	}
	checker := parseCheckerFor(t, "eb-rule", "lambda")
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clients := parseClients(tc.region)
			clients.EventBridge = &parseEBTargetsFake{targets: []ebtypes.Target{{
				Id:  aws.String("target-1"),
				Arn: aws.String(tc.targetARN),
			}}}
			result := checker(context.Background(), clients,
				resource.Resource{ID: "acme-nightly", Name: "acme-nightly"},
				resource.ResourceCache{})

			if result.Count() != len(tc.wantIDs) {
				t.Fatalf("Count = %d, want %d (ids %v)", result.Count(), len(tc.wantIDs), result.ResourceIDs())
			}
			for i, want := range tc.wantIDs {
				if result.ResourceIDs()[i] != want {
					t.Errorf("ResourceIDs()[%d] = %q, want %q", i, result.ResourceIDs()[i], want)
				}
			}
		})
	}
}

// TestS3Backup_BucketARNCarriesTheSessionPartition pins the last ARN built at
// runtime. An S3 bucket ARN names no region, but it does name a partition, so
// a China bucket's backup plan is matched against a string the join never
// builds and the panel reports the bucket unprotected.
func TestS3Backup_BucketARNCarriesTheSessionPartition(t *testing.T) {
	tests := []struct {
		name      string
		region    string
		planARN   string
		wantCount int
	}{
		{name: "China bucket named by a China plan", region: parseCNRegion, planARN: "arn:aws-cn:s3:::acme-assets", wantCount: 1},
		{name: "GovCloud bucket named by a GovCloud plan", region: parseGovRegion, planARN: "arn:aws-us-gov:s3:::acme-assets", wantCount: 1},
		{name: "commercial bucket named by a commercial plan", region: "us-east-1", planARN: "arn:aws:s3:::acme-assets", wantCount: 1},
		{name: "China wildcard selection", region: parseCNRegion, planARN: "arn:aws-cn:s3:::*", wantCount: 1},
		{name: "plan naming another bucket", region: parseCNRegion, planARN: "arn:aws-cn:s3:::other-assets"},
	}
	checker := parseCheckerFor(t, "s3", "backup")
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := checker(context.Background(), parseClients(tc.region),
				resource.Resource{ID: "acme-assets", Name: "acme-assets"},
				w7CacheWith(w7Plan(tc.planARN, "", "")))
			if result.Count() != tc.wantCount {
				t.Errorf("Count = %d, want %d (ids %v)", result.Count(), tc.wantCount, result.ResourceIDs())
			}
		})
	}
}
