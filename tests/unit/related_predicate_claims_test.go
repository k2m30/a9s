package unit_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// claimCell is the text the related panel shows for a checker's answer: a
// count only when the lookup resolved, a lower bound when it stopped short,
// and nothing at all when it has no count to offer.
func claimCell(r resource.RelatedCheckResult) string {
	return resource.FormatRelatedCount(r.EffectiveState(), r.Count(), r.Truncated())
}

// --- log groups -------------------------------------------------------------

func claimLogGroups(ids ...string) []resource.Resource {
	out := make([]resource.Resource, 0, len(ids))
	for _, id := range ids {
		out = append(out, resource.Resource{ID: id, Name: id, Fields: map[string]string{"log_group_name": id}})
	}
	return out
}

// --- SQS queue ↔ SNS subscriptions -----------------------------------------

func claimQueue(name string) resource.Resource {
	arn := "arn:aws:sqs:us-east-1:123456789012:" + name
	return resource.Resource{
		ID:     name,
		Name:   name,
		Fields: map[string]string{"queue_name": name, "arn": arn},
		RawStruct: awsclient.SQSQueueAttributesRow{
			QueueURL:   "https://sqs.us-east-1.amazonaws.com/123456789012/" + name,
			QueueName:  name,
			Attributes: map[string]string{"QueueArn": arn},
		},
	}
}

func claimSubscription(id, queue, topic string) resource.Resource {
	return resource.Resource{
		ID:   id,
		Name: id,
		Fields: map[string]string{
			"protocol":  "sqs",
			"endpoint":  "arn:aws:sqs:us-east-1:123456789012:" + queue,
			"topic_arn": "arn:aws:sns:us-east-1:123456789012:" + topic,
		},
	}
}

// TestSQSSubscriptionsMatchTheQueueARNExactly pins the identity an sqs-protocol
// SNS subscription is matched on: its Endpoint is the queue's ARN, whole. An
// ARN ending in ":orders" is a prefix of one ending in ":orders-dlq", so a
// queue and its dead-letter queue each list their own subscription and their
// own topic, and the subscription lists back the queue it delivers to.
func TestSQSSubscriptionsMatchTheQueueARNExactly(t *testing.T) {
	subs := []resource.Resource{
		claimSubscription("arn:aws:sns:us-east-1:123456789012:orders-topic:11111111-2222-3333-4444-555555555555", "orders", "orders-topic"),
		claimSubscription("arn:aws:sns:us-east-1:123456789012:dlq-topic:66666666-7777-8888-9999-000000000000", "orders-dlq", "dlq-topic"),
	}
	cache := resource.ResourceCache{
		"sns-sub": {Resources: subs},
		"sqs":     {Resources: []resource.Resource{claimQueue("orders"), claimQueue("orders-dlq")}},
	}
	ctx := context.Background()

	for _, tc := range []struct{ queue, sub, topic string }{
		{"orders", subs[0].ID, "arn:aws:sns:us-east-1:123456789012:orders-topic"},
		{"orders-dlq", subs[1].ID, "arn:aws:sns:us-east-1:123456789012:dlq-topic"},
	} {
		q := claimQueue(tc.queue)
		assertSameIDs(t, "sqs "+tc.queue+" -> sns-sub",
			checkerByTarget(t, "sqs", "sns-sub")(ctx, nil, q, cache).ResourceIDs(), []string{tc.sub})
		assertSameIDs(t, "sqs "+tc.queue+" -> sns",
			checkerByTarget(t, "sqs", "sns")(ctx, nil, q, cache).ResourceIDs(), []string{tc.topic})
	}

	for i, sub := range subs {
		want := []string{"orders", "orders-dlq"}[i]
		assertSameIDs(t, "sns-sub "+sub.ID+" -> sqs",
			checkerByTarget(t, "sns-sub", "sqs")(ctx, nil, sub, cache).ResourceIDs(), []string{want})
	}
}

// --- ACM validation record → hosted zone ------------------------------------

type claimACMFake struct{ recordNames []string }

func (f claimACMFake) ListCertificates(_ context.Context, _ *acm.ListCertificatesInput, _ ...func(*acm.Options)) (*acm.ListCertificatesOutput, error) {
	return &acm.ListCertificatesOutput{}, nil
}

func (f claimACMFake) DescribeCertificate(_ context.Context, in *acm.DescribeCertificateInput, _ ...func(*acm.Options)) (*acm.DescribeCertificateOutput, error) {
	var opts []acmtypes.DomainValidation
	for _, n := range f.recordNames {
		opts = append(opts, acmtypes.DomainValidation{
			DomainName:       aws.String("app.notexample.com"),
			ValidationMethod: acmtypes.ValidationMethodDns,
			ValidationStatus: acmtypes.DomainStatusPendingValidation,
			ResourceRecord: &acmtypes.ResourceRecord{
				Name:  aws.String(n),
				Type:  acmtypes.RecordTypeCname,
				Value: aws.String("_c1a2b3c4.acm-validations.aws."),
			},
		})
	}
	return &acm.DescribeCertificateOutput{Certificate: &acmtypes.CertificateDetail{
		CertificateArn:          in.CertificateArn,
		DomainName:              aws.String("app.notexample.com"),
		Status:                  acmtypes.CertificateStatusPendingValidation,
		Type:                    acmtypes.CertificateTypeAmazonIssued,
		DomainValidationOptions: opts,
	}}, nil
}

func claimZone(id, name string, private bool) resource.Resource {
	p := "false"
	if private {
		p = "true"
	}
	return resource.Resource{
		ID:     id,
		Name:   name,
		Fields: map[string]string{"zone_id": id, "name": name, "private_zone": p},
		RawStruct: r53types.HostedZone{
			Id:     aws.String("/hostedzone/" + id),
			Name:   aws.String(name),
			Config: &r53types.HostedZoneConfig{PrivateZone: private},
		},
	}
}

func claimCertificate() resource.Resource {
	const arn = "arn:aws:acm:us-east-1:123456789012:certificate/c1a2b3c4-d5e6-f708-1920-a1b2c3d4e5f6"
	return resource.Resource{
		ID:     arn,
		Name:   "app.notexample.com",
		Fields: map[string]string{"domain_name": "app.notexample.com"},
		RawStruct: acmtypes.CertificateSummary{
			CertificateArn: aws.String(arn),
			DomainName:     aws.String("app.notexample.com"),
		},
	}
}

// TestACMValidationRecordPicksItsZoneOnALabelBoundary pins which hosted zone
// hosts a certificate's DNS validation record. A zone contains a record only
// when its name is the record's parent at a label boundary:
// "app.notexample.com" ends in the characters of "example.com" while being a
// name in an entirely different domain, and nothing can be written into a
// zone the account does not host. Among the zones that do contain it the
// longest wins, and a private zone is never the answer — ACM validates a
// public certificate against public DNS.
func TestACMValidationRecordPicksItsZoneOnALabelBoundary(t *testing.T) {
	ctx := context.Background()
	cert := claimCertificate()
	clients := &awsclient.ServiceClients{
		ACM:    claimACMFake{recordNames: []string{"_c1a2b3c4.app.notexample.com."}},
		Region: "us-east-1",
	}

	for _, tc := range []struct {
		name  string
		zones []resource.Resource
		want  []string
	}{
		{
			"only an unrelated zone ending in the same characters",
			[]resource.Resource{claimZone("Z1EXAMPLE0000A", "example.com.", false)},
			nil,
		},
		{
			"the longest zone containing the record",
			[]resource.Resource{
				claimZone("Z1NOTEXAMPLE00", "notexample.com.", false),
				claimZone("Z1APPNOTEXAMPL", "app.notexample.com.", false),
			},
			[]string{"Z1APPNOTEXAMPL"},
		},
		{
			"a private zone of the record's own name",
			[]resource.Resource{
				claimZone("Z1NOTEXAMPLE00", "notexample.com.", false),
				claimZone("Z1PRIVATEAPP00", "app.notexample.com.", true),
			},
			[]string{"Z1NOTEXAMPLE00"},
		},
	} {
		cache := resource.ResourceCache{"r53": {Resources: tc.zones}}
		got := checkerByTarget(t, "acm", "r53")(ctx, clients, cert, cache)
		assertSameIDs(t, "acm app.notexample.com -> r53 ("+tc.name+")", got.ResourceIDs(), tc.want)
	}
}

// --- CloudFront ↔ hosted zone, over a zone whose records were truncated -----

type claimTruncatedR53Fake struct {
	nilCacheR53Fake
}

func (f *claimTruncatedR53Fake) ListResourceRecordSets(_ context.Context, _ *route53.ListResourceRecordSetsInput, _ ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error) {
	return &route53.ListResourceRecordSetsOutput{ResourceRecordSets: f.records, IsTruncated: true}, nil
}

func claimZoneWithAliases(id, name, aliasTargets string, recordsTruncated bool) resource.Resource {
	z := claimZone(id, name, false)
	z.Fields["alias_targets"] = aliasTargets
	z.Fields["records_truncated"] = "false"
	if recordsTruncated {
		z.Fields["records_truncated"] = "true"
	}
	return z
}

// TestCloudFrontZonePivotIsALowerBoundOverATruncatedZone pins what a
// distribution's hosted-zone row may claim. A zone's alias records are read
// one page at a time and AWS caps that page, so an alias sitting on an unread
// page is a record nobody looked at: the row owes a lower bound, which is
// what the zone's own CloudFront row already reports for the same pair. Only
// a zone read to its end can report a zero.
func TestCloudFrontZonePivotIsALowerBoundOverATruncatedZone(t *testing.T) {
	ctx := context.Background()
	dist := predDistribution("E1WEB0000000AA", "d111111abcdef8.cloudfront.net")

	unread := resource.ResourceCache{"r53": {Resources: []resource.Resource{
		claimZoneWithAliases("Z1TRUNCATED000", "acme.example.", "", true),
	}}}
	fromDist := checkerByTarget(t, "cf", "r53")(ctx, nil, dist, unread)
	if cell := claimCell(fromDist); cell != "(0+)" {
		t.Errorf("cf E1WEB0000000AA -> r53 over a zone whose records were truncated renders %q, want \"(0+)\": the alias record may be on the page nobody read", cell)
	}

	read := resource.ResourceCache{"r53": {Resources: []resource.Resource{
		claimZoneWithAliases("Z1COMPLETE0000", "acme.example.", "d999999abcdef8.cloudfront.net", false),
	}}}
	if cell := claimCell(checkerByTarget(t, "cf", "r53")(ctx, nil, dist, read)); cell != "(0)" {
		t.Errorf("cf E1WEB0000000AA -> r53 over a zone read to its end renders %q, want \"(0)\"", cell)
	}

	aliased := resource.ResourceCache{"r53": {Resources: []resource.Resource{
		claimZoneWithAliases("Z1ALIASED00000", "acme.example.", "d111111abcdef8.cloudfront.net", false),
	}}}
	assertSameIDs(t, "cf E1WEB0000000AA -> r53 over a zone aliasing it",
		checkerByTarget(t, "cf", "r53")(ctx, nil, dist, aliased).ResourceIDs(), []string{"Z1ALIASED00000"})

	zoneClients := &awsclient.ServiceClients{Route53: &claimTruncatedR53Fake{nilCacheR53Fake{
		records: []r53types.ResourceRecordSet{aliasRecord("cdn.acme.example.", "d999999abcdef8.cloudfront.net.")},
	}}}
	fromZone := checkerByTarget(t, "r53", "cf")(ctx, zoneClients, r53Zone(),
		resource.ResourceCache{"cf": {Resources: []resource.Resource{dist}}})
	if cell := claimCell(fromZone); cell != "(0+)" {
		t.Errorf("r53 acme.example -> cf over a truncated record page renders %q, want \"(0+)\"", cell)
	}
}

// --- SSM parameter names that say "credential" ------------------------------

// TestSSMPlaintextCredentialNamesAreTheSecretScanNames pins which parameter
// names say "this holds a credential". The same key name in a Lambda
// environment variable, a CodeBuild environment variable or a task
// definition is reported as a plaintext credential, and an SSM String
// parameter stores its value with no more protection than those do, so the
// name "aws_access_key_id" means the same thing on all of them. A SecureString
// of that name holds its value under a KMS key and is not plaintext.
func TestSSMPlaintextCredentialNamesAreTheSecretScanNames(t *testing.T) {
	const code = domain.FindingCode("ssm.value.plaintext-sensitive")
	phrase := catalog.Phrase(code)
	if phrase == "" {
		t.Fatalf("%s is not a registered finding", code)
	}
	td := catalog.FindAny("ssm")
	if td == nil {
		t.Fatal("ssm is not a registered type")
	}

	credentialNames := []string{
		"/prod/api/aws_access_key_id",
		"/prod/api/client_secret_v2",
		"/prod/db/db_pass",
		"/prod/svc/auth_token",
		"/prod/tls/private_key",
		"/prod/orders/db/password",
		"/prod/orders/db_password",
	}
	plainNames := []string{"/prod/orders/log_level", "/prod/region", "/prod/app/feature_flags"}

	recent := time.Now().Add(-30 * 24 * time.Hour)
	var params []ssmtypes.ParameterMetadata
	for _, n := range append(slices.Clone(credentialNames), plainNames...) {
		params = append(params, factsParam(n, ssmtypes.ParameterTypeString, recent))
	}
	params = append(params, factsParam("/prod/secure/client_secret_v2", ssmtypes.ParameterTypeSecureString, recent))

	out, err := awsclient.FetchSSMParametersPage(context.Background(), factsSSMFake{params: params}, "")
	if err != nil {
		t.Fatalf("FetchSSMParametersPage: %v", err)
	}
	rows := out.Resources
	byID := map[string]resource.Resource{}
	for _, r := range rows {
		byID[r.ID] = r
	}

	for _, n := range credentialNames {
		row := byID[n]
		top, has := domain.TopFinding(row.Findings)
		if !has || top.Code != code {
			t.Errorf("%s: findings = %v, want %s — the name says it holds a credential and the type is String", n, row.Findings, code)
			continue
		}
		cell, ok := listStatusCellFor(t, *td, rows, n)
		if !ok {
			t.Fatalf("%s: no Status cell rendered", n)
		}
		if cell != phrase {
			t.Errorf("%s: Status cell = %q, want %q", n, cell, phrase)
		}
	}

	for _, n := range append(slices.Clone(plainNames), "/prod/secure/client_secret_v2") {
		if slices.ContainsFunc(byID[n].Findings, func(f domain.Finding) bool { return f.Code == code }) {
			t.Errorf("%s: carries %s; it holds no credential a plaintext value would expose", n, code)
		}
	}
}

// --- API Gateway → CloudFront ----------------------------------------------

// TestAPIGatewayCountsOnlyDistributionsFrontingItsOwnHost pins the identity an
// API's invoke host is read by: the label before ".execute-api.", whole. An
// API id is the leading label of its host, so "xyzabc1234567.execute-api…"
// is another API's host even though the characters of "abc1234567" appear in
// it.
func TestAPIGatewayCountsOnlyDistributionsFrontingItsOwnHost(t *testing.T) {
	const region = ".execute-api.us-east-1.amazonaws.com"
	own := predDistribution("E1API0000000AA", "d111111abcdef8.cloudfront.net", "abc1234567"+region)
	other := predDistribution("E2API0000000BB", "d222222abcdef8.cloudfront.net", "xyzabc1234567"+region)
	cache := resource.ResourceCache{"cf": {Resources: []resource.Resource{own, other}}}
	ctx := context.Background()

	for _, tc := range []struct{ api, dist string }{
		{"abc1234567", "E1API0000000AA"},
		{"xyzabc1234567", "E2API0000000BB"},
	} {
		src := resource.Resource{ID: tc.api, Name: tc.api, Fields: map[string]string{"api_id": tc.api}}
		assertSameIDs(t, "apigw "+tc.api+" -> cf",
			checkerByTarget(t, "apigw", "cf")(ctx, nil, src, cache).ResourceIDs(), []string{tc.dist})
	}
}

// --- Lambda function → API Gateway ------------------------------------------

func claimAPI(id, name string, tags map[string]string) resource.Resource {
	return resource.Resource{
		ID:     id,
		Name:   name,
		Fields: map[string]string{"api_id": id, "name": name},
		RawStruct: apigwtypes.Api{
			ApiId:        aws.String(id),
			Name:         aws.String(name),
			ProtocolType: apigwtypes.ProtocolTypeHttp,
			ApiEndpoint:  aws.String("https://" + id + ".execute-api.us-east-1.amazonaws.com"),
			Tags:         tags,
		},
	}
}

// TestLambdaAPIGatewayRowClaimsOnlyWhatItCanShow pins what a function's API
// Gateway row may assert. An API's Name is free text an operator chooses, so
// an API called "acme-orders-api" says nothing about which function it
// invokes; a tag whose key is the function's name is a fact recorded on the
// API. A count on that row tells the operator the APIs invoking this function
// have been found, so an API related to it only by the characters of its name
// may not be inside a number that says so, and an API sharing nothing with it
// is not a candidate either.
func TestLambdaAPIGatewayRowClaimsOnlyWhatItCanShow(t *testing.T) {
	const byName, byTag, unrelated = "abc1234567", "def8901234", "ghi5678901"
	cache := resource.ResourceCache{"apigw": {Resources: []resource.Resource{
		claimAPI(byName, "acme-orders-api", map[string]string{"Environment": "production"}),
		claimAPI(byTag, "acme-checkout-api", map[string]string{"orders": "invoke"}),
		claimAPI(unrelated, "acme-billing-api", nil),
	}}}
	fn := resource.Resource{ID: "orders", Name: "orders", Fields: map[string]string{"function_name": "orders"}}

	res := checkerByTarget(t, "lambda", "apigw")(context.Background(), nil, fn, cache)
	ids := res.ResourceIDs()

	if !slices.Contains(ids, byTag) {
		t.Errorf("lambda orders -> apigw omits %s, whose tags name the function (lists %v)", byTag, ids)
	}
	if slices.Contains(ids, unrelated) {
		t.Errorf("lambda orders -> apigw lists %s, which names the function nowhere (lists %v)", unrelated, ids)
	}
	if cell := claimCell(res); slices.Contains(ids, byName) && cell != "" {
		t.Errorf("lambda orders -> apigw renders %s over %s: that API's name merely contains the function's, and a count states the API was found to invoke it",
			cell, byName)
	}
}

// --- VPC peering connection → route tables ----------------------------------

func claimPeerRouteTable(id string, routes ...ec2types.Route) resource.Resource {
	return resource.Resource{
		ID:     id,
		Name:   id,
		Fields: map[string]string{"route_table_id": id},
		RawStruct: ec2types.RouteTable{
			RouteTableId: aws.String(id),
			VpcId:        aws.String("vpc-0a1b2c3d4e5f60718"),
			OwnerId:      aws.String("123456789012"),
			Routes:       routes,
		},
	}
}

func claimPeerRoute(cidr, pcx string, state ec2types.RouteState) ec2types.Route {
	return ec2types.Route{
		DestinationCidrBlock:   aws.String(cidr),
		VpcPeeringConnectionId: aws.String(pcx),
		State:                  state,
		Origin:                 ec2types.RouteOriginCreateRoute,
	}
}

// TestPeeringConnectionCountsOnlyRouteTablesThatStillRouteToIt pins that a
// peering connection's route-table row reads the same rule as every other
// route target. AWS leaves the pcx- id on the route after the connection is
// deleted and marks the route blackhole, so a blackhole route names no path
// to the peer, and a table holding both a live and a dead route to it still
// routes to it.
func TestPeeringConnectionCountsOnlyRouteTablesThatStillRouteToIt(t *testing.T) {
	const live, dead = "pcx-0live000000000001", "pcx-0deleted00000001"
	cache := resource.ResourceCache{"rtb": {Resources: []resource.Resource{
		claimPeerRouteTable("rtb-0live000000000001", claimPeerRoute("10.30.0.0/16", live, ec2types.RouteStateActive)),
		claimPeerRouteTable("rtb-0dead000000000002", claimPeerRoute("10.40.0.0/16", dead, ec2types.RouteStateBlackhole)),
		claimPeerRouteTable("rtb-0both000000000003",
			claimPeerRoute("10.50.0.0/16", dead, ec2types.RouteStateBlackhole),
			claimPeerRoute("10.60.0.0/16", live, ec2types.RouteStateActive)),
	}}}
	ctx := context.Background()

	peer := func(id string) resource.Resource {
		return resource.Resource{
			ID:     id,
			Name:   id,
			Fields: map[string]string{"vpc_peering_connection_id": id},
			RawStruct: ec2types.VpcPeeringConnection{
				VpcPeeringConnectionId: aws.String(id),
				AccepterVpcInfo:        &ec2types.VpcPeeringConnectionVpcInfo{VpcId: aws.String("vpc-0peer00000000001"), OwnerId: aws.String("123456789012")},
				RequesterVpcInfo:       &ec2types.VpcPeeringConnectionVpcInfo{VpcId: aws.String("vpc-0a1b2c3d4e5f60718"), OwnerId: aws.String("123456789012")},
			},
		}
	}
	assertSameIDs(t, "vpc-peer "+live+" -> rtb",
		checkerByTarget(t, "vpc-peer", "rtb")(ctx, nil, peer(live), cache).ResourceIDs(),
		[]string{"rtb-0live000000000001", "rtb-0both000000000003"})
	assertSameIDs(t, "vpc-peer "+dead+" -> rtb (every route to it is blackhole)",
		checkerByTarget(t, "vpc-peer", "rtb")(ctx, nil, peer(dead), cache).ResourceIDs(), nil)
}

// --- Launch template → Auto Scaling groups / node groups --------------------

func claimASG(name string, spec *asgtypes.LaunchTemplateSpecification, mixed, override bool) resource.Resource {
	asg := asgtypes.AutoScalingGroup{
		AutoScalingGroupName: aws.String(name),
		AutoScalingGroupARN:  aws.String("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:6f1c2a3b-4d5e-6f70-8192-a3b4c5d6e7f8:autoScalingGroupName/" + name),
		MinSize:              aws.Int32(1),
		MaxSize:              aws.Int32(4),
		DesiredCapacity:      aws.Int32(2),
	}
	switch {
	case override:
		asg.MixedInstancesPolicy = &asgtypes.MixedInstancesPolicy{LaunchTemplate: &asgtypes.LaunchTemplate{
			LaunchTemplateSpecification: &asgtypes.LaunchTemplateSpecification{LaunchTemplateId: aws.String("lt-0other0000000001")},
			Overrides: []asgtypes.LaunchTemplateOverrides{
				{InstanceType: aws.String("m6i.large"), LaunchTemplateSpecification: spec},
			},
		}}
	case mixed:
		asg.MixedInstancesPolicy = &asgtypes.MixedInstancesPolicy{LaunchTemplate: &asgtypes.LaunchTemplate{LaunchTemplateSpecification: spec}}
	default:
		asg.LaunchTemplate = spec
	}
	return resource.Resource{ID: name, Name: name, Fields: map[string]string{}, RawStruct: asg}
}

// TestLaunchTemplateFindsGroupsThatNameItByName pins that a launch template
// finds the groups launching from it however they name it. The Auto Scaling
// API's LaunchTemplateSpecification requires either LaunchTemplateId or
// LaunchTemplateName, so a group may carry only the name — as an EKS node
// group may — and it launches from the template all the same. A group naming
// a different template does not.
func TestLaunchTemplateFindsGroupsThatNameItByName(t *testing.T) {
	const ltID, ltName = "lt-0a1b2c3d4e5f60718", "acme-web"
	byName := func() *asgtypes.LaunchTemplateSpecification {
		return &asgtypes.LaunchTemplateSpecification{LaunchTemplateName: aws.String(ltName)}
	}
	cache := resource.ResourceCache{"asg": {Resources: []resource.Resource{
		claimASG("acme-web-asg", byName(), false, false),
		claimASG("acme-web-spot-asg", byName(), true, false),
		claimASG("acme-web-override-asg", byName(), false, true),
		claimASG("acme-web-id-asg", &asgtypes.LaunchTemplateSpecification{LaunchTemplateId: aws.String(ltID)}, false, false),
		claimASG("acme-other-asg", &asgtypes.LaunchTemplateSpecification{LaunchTemplateName: aws.String("acme-other")}, false, false),
	}}}
	lt := resource.Resource{ID: ltID, Name: ltName, Fields: map[string]string{"launch_template_id": ltID, "launch_template_name": ltName}}
	ctx := context.Background()

	assertSameIDs(t, "lt "+ltName+" -> asg",
		checkerByTarget(t, "lt", "asg")(ctx, nil, lt, cache).ResourceIDs(),
		[]string{"acme-web-asg", "acme-web-spot-asg", "acme-web-override-asg", "acme-web-id-asg"})

	ngCache := resource.ResourceCache{"ng": {Resources: []resource.Resource{
		{ID: "general-pool", Name: "general-pool", RawStruct: ekstypes.Nodegroup{
			NodegroupName:  aws.String("general-pool"),
			ClusterName:    aws.String("acme-prod"),
			LaunchTemplate: &ekstypes.LaunchTemplateSpecification{Name: aws.String(ltName)},
		}},
		{ID: "other-pool", Name: "other-pool", RawStruct: ekstypes.Nodegroup{
			NodegroupName:  aws.String("other-pool"),
			ClusterName:    aws.String("acme-prod"),
			LaunchTemplate: &ekstypes.LaunchTemplateSpecification{Name: aws.String("acme-other")},
		}},
	}}}
	assertSameIDs(t, "lt "+ltName+" -> ng",
		checkerByTarget(t, "lt", "ng")(ctx, nil, lt, ngCache).ResourceIDs(), []string{"general-pool"})
}

// --- ECS workloads → the log groups their containers write to ---------------

// claimTaskDefinition is a family whose first container writes to
// /ecs/acme-api, whose sidecar writes to /ecs/acme-api-sidecar, and whose
// third container has no log driver at all — a shape ECS allows and the
// container's output is then discarded.
func claimTaskDefinition() *ecstypes.TaskDefinition {
	awslogs := func(group string) *ecstypes.LogConfiguration {
		return &ecstypes.LogConfiguration{
			LogDriver: ecstypes.LogDriverAwslogs,
			Options: map[string]string{
				"awslogs-group":         group,
				"awslogs-region":        "us-east-1",
				"awslogs-stream-prefix": "ecs",
			},
		}
	}
	return &ecstypes.TaskDefinition{
		Family:            aws.String(claimECSFamily),
		Revision:          7,
		TaskDefinitionArn: aws.String(claimECSTaskDefARN),
		ContainerDefinitions: []ecstypes.ContainerDefinition{
			{Name: aws.String("app"), Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme-api:v1.4.2"), LogConfiguration: awslogs("/ecs/acme-api")},
			{Name: aws.String("sidecar"), Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/envoy:v1.29"), LogConfiguration: awslogs("/ecs/acme-api-sidecar")},
			{Name: aws.String("init"), Image: aws.String("public.ecr.aws/docker/library/busybox:1.36")},
		},
	}
}

const (
	claimECSFamily     = "acme-api"
	claimECSTaskDefARN = "arn:aws:ecs:us-east-1:123456789012:task-definition/acme-api:7"
)

// claimECSLogCache holds the two groups the definition names plus three that
// only carry the family's characters: another family's group, another
// service's, and a Lambda's.
func claimECSLogCache() resource.ResourceCache {
	return resource.ResourceCache{"logs": {Resources: claimLogGroups(
		"/ecs/acme-api",
		"/ecs/acme-api-sidecar",
		"/ecs/acme-api-worker",
		"/aws/lambda/acme-api-handler",
		"/ecs/other-service",
	)}}
}

func claimECSTask() resource.Resource {
	return resource.Resource{
		ID:   "1a2b3c4d5e6f708192a3b4c5d6e7f809",
		Name: "1a2b3c4d5e6f708192a3b4c5d6e7f809",
		Fields: map[string]string{
			"task_id":         "1a2b3c4d5e6f708192a3b4c5d6e7f809",
			"cluster":         "acme-prod",
			"task_definition": claimECSTaskDefARN,
			"status":          "RUNNING",
		},
		RawStruct: ecstypes.Task{
			TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/acme-prod/1a2b3c4d5e6f708192a3b4c5d6e7f809"),
			ClusterArn:        aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/acme-prod"),
			TaskDefinitionArn: aws.String(claimECSTaskDefARN),
			LastStatus:        aws.String("RUNNING"),
			LaunchType:        ecstypes.LaunchTypeFargate,
		},
	}
}

// TestECSWorkloadsCountTheLogGroupsTheirContainersWriteTo pins which log
// groups an ECS service and an ECS task report. A container's
// LogConfiguration Options["awslogs-group"] names the group its stdout and
// stderr reach: that is the link AWS records, it is readable with one
// DescribeTaskDefinition per family, and every container of the definition
// contributes one. A group whose name only carries the family — another
// family's, a Lambda's — is not one of them, and a container with no log
// driver adds none.
func TestECSWorkloadsCountTheLogGroupsTheirContainersWriteTo(t *testing.T) {
	ctx := context.Background()
	clients := &awsclient.ServiceClients{ECS: newFakeECSWithTaskDefinition(claimTaskDefinition()), Region: "us-east-1"}
	cache := claimECSLogCache()
	want := []string{"/ecs/acme-api", "/ecs/acme-api-sidecar"}

	for _, tc := range []struct {
		label string
		src   resource.Resource
		typ   string
	}{
		{"ecs-svc acme-api-svc", ecsSvcWithTaskDef("acme-api-svc", claimECSTaskDefARN), "ecs-svc"},
		{"ecs-task 1a2b3c4d…", claimECSTask(), "ecs-task"},
	} {
		res := checkerByTarget(t, tc.typ, "logs")(ctx, clients, tc.src, cache)
		assertSameIDs(t, tc.label+" -> logs", res.ResourceIDs(), want)
		if cell := claimCell(res); cell != "(2)" {
			t.Errorf("%s -> logs renders %q, want \"(2)\": the task definition names both groups, so the row has a count to show", tc.label, cell)
		}
		if got := res.Coverage(); got != domain.CoverageComplete {
			t.Errorf("%s -> logs coverage = %v, want %v: every place the link is recorded was read", tc.label, got, domain.CoverageComplete)
		}
	}
}

// TestECSWorkloadsOfferCandidatesWhenTheDefinitionCannotBeRead pins what the
// row falls back to when ecs:DescribeTaskDefinition is refused. Nothing was
// read, so nothing is proven: the groups whose name carries the family are
// candidates the operator may open, never a number saying they were found to
// receive the containers' output.
func TestECSWorkloadsOfferCandidatesWhenTheDefinitionCannotBeRead(t *testing.T) {
	ctx := context.Background()
	refused := &awsclient.ServiceClients{
		ECS: &fakeECSForSvcPivots{describeTaskDefFn: func(_ *ecs.DescribeTaskDefinitionInput) (*ecs.DescribeTaskDefinitionOutput, error) {
			return nil, &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "User is not authorized to perform: ecs:DescribeTaskDefinition"}
		}},
		Region: "us-east-1",
	}
	cache := claimECSLogCache()

	for _, tc := range []struct {
		label string
		src   resource.Resource
		typ   string
	}{
		{"ecs-svc acme-api-svc", ecsSvcWithTaskDef("acme-api-svc", claimECSTaskDefARN), "ecs-svc"},
		{"ecs-task 1a2b3c4d…", claimECSTask(), "ecs-task"},
	} {
		res := checkerByTarget(t, tc.typ, "logs")(ctx, refused, tc.src, cache)
		if cell := claimCell(res); cell != "" {
			t.Errorf("%s -> logs renders %q with the definition unread, want no count", tc.label, cell)
		}
		if got := res.Coverage(); got != domain.CoverageHeuristic {
			t.Errorf("%s -> logs coverage = %v, want %v: the groups carrying the family's name are candidates", tc.label, got, domain.CoverageHeuristic)
		}
		if !slices.Contains(res.ResourceIDs(), "/ecs/acme-api") {
			t.Errorf("%s -> logs offers %v, want the groups carrying the family's name as candidates", tc.label, res.ResourceIDs())
		}
	}
}

// TestEC2InstanceLogGroupsStayCandidates pins that an instance's log-group row
// keeps offering candidates. Nothing AWS returns about an instance names a log
// group: the CloudWatch agent's configuration lives on the instance's disk, so
// a group carrying the instance id is a guess an unrelated group may match too.
func TestEC2InstanceLogGroupsStayCandidates(t *testing.T) {
	const instance = "i-0a1b2c3d4e5f60718"
	cache := resource.ResourceCache{"logs": {Resources: claimLogGroups(
		"/var/log/messages/"+instance,
		"/aws/ssm/"+instance,
		"/ecs/other-service",
	)}}
	src := resource.Resource{ID: instance, Name: instance, Fields: map[string]string{"instance_id": instance}}

	res := checkerByTarget(t, "ec2", "logs")(context.Background(), nil, src, cache)
	if cell := claimCell(res); cell != "" {
		t.Errorf("ec2 %s -> logs renders %q, want no count: nothing AWS returns about an instance names a log group", instance, cell)
	}
	if got := res.Coverage(); got != domain.CoverageHeuristic {
		t.Errorf("ec2 %s -> logs coverage = %v, want %v", instance, got, domain.CoverageHeuristic)
	}
	assertSameIDs(t, "ec2 "+instance+" -> logs",
		res.ResourceIDs(), []string{"/var/log/messages/" + instance, "/aws/ssm/" + instance})
}

// claimCluster is an ECS cluster whose ecs exec sessions are sent to
// execLogGroup, or, when execLogGroup is "", one that configures ecs exec
// without a CloudWatch destination.
func claimCluster(name, execLogGroup string) resource.Resource {
	cluster := ecstypes.Cluster{
		ClusterName: aws.String(name),
		ClusterArn:  aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/" + name),
		Status:      aws.String("ACTIVE"),
		Configuration: &ecstypes.ClusterConfiguration{
			ExecuteCommandConfiguration: &ecstypes.ExecuteCommandConfiguration{
				KmsKeyId: aws.String("a1b2c3d4-5678-90ab-cdef-111111111111"),
				Logging:  ecstypes.ExecuteCommandLoggingDefault,
			},
		},
	}
	if execLogGroup != "" {
		cluster.Configuration.ExecuteCommandConfiguration.Logging = ecstypes.ExecuteCommandLoggingOverride
		cluster.Configuration.ExecuteCommandConfiguration.LogConfiguration = &ecstypes.ExecuteCommandLogConfiguration{
			CloudWatchLogGroupName:      aws.String(execLogGroup),
			CloudWatchEncryptionEnabled: true,
		}
	}
	return resource.Resource{
		ID:        name,
		Name:      name,
		Fields:    map[string]string{"cluster_name": name, "status": "ACTIVE"},
		RawStruct: cluster,
	}
}

// TestECSClusterCountsItsExecuteCommandLogGroup pins the log group an ECS
// cluster reports. A Cluster carries one log-group name of its own —
// Configuration.ExecuteCommandConfiguration.LogConfiguration.CloudWatchLogGroupName,
// where ecs exec session transcripts are written — so the row is that group
// or none, counted. A cluster that sends its exec sessions nowhere has no log
// group of its own, which is a proven zero; and a group whose name merely
// carries the cluster's belongs to whoever named it.
func TestECSClusterCountsItsExecuteCommandLogGroup(t *testing.T) {
	cache := resource.ResourceCache{"logs": {Resources: claimLogGroups(
		"/ecs/exec/acme-prod",
		"/ecs/acme-prod",
		"/aws/lambda/acme-prod-api",
		"/ecs/acme-batch",
		"/ecs/acme-staging",
	)}}
	ctx := context.Background()

	withGroup := checkerByTarget(t, "ecs", "logs")(ctx, nil, claimCluster("acme-prod", "/ecs/exec/acme-prod"), cache)
	assertSameIDs(t, "ecs acme-prod -> logs", withGroup.ResourceIDs(), []string{"/ecs/exec/acme-prod"})
	if cell := claimCell(withGroup); cell != "(1)" {
		t.Errorf("ecs acme-prod -> logs renders %q, want \"(1)\": the cluster names the group its exec sessions are written to", cell)
	}
	if got := withGroup.Coverage(); got != domain.CoverageComplete {
		t.Errorf("ecs acme-prod -> logs coverage = %v, want %v: the one field that records the link was read", got, domain.CoverageComplete)
	}

	for _, src := range []resource.Resource{
		claimCluster("acme-batch", ""),
		{ID: "acme-staging", Name: "acme-staging", Fields: map[string]string{"cluster_name": "acme-staging"},
			RawStruct: ecstypes.Cluster{
				ClusterName: aws.String("acme-staging"),
				ClusterArn:  aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/acme-staging"),
				Status:      aws.String("ACTIVE"),
			}},
	} {
		res := checkerByTarget(t, "ecs", "logs")(ctx, nil, src, cache)
		assertSameIDs(t, "ecs "+src.ID+" -> logs", res.ResourceIDs(), nil)
		if cell := claimCell(res); cell != "(0)" {
			t.Errorf("ecs %s -> logs renders %q, want \"(0)\": the cluster writes no exec session transcripts, and the field saying so was read", src.ID, cell)
		}
	}
}
