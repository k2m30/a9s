package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	opensearchtypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	sestypes "github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

// t570VPCsWithSubnets returns n demo VPC ids, each with one of its subnets.
func t570VPCsWithSubnets(t *testing.T, c *awsclient.ServiceClients, n int) (vpcs, subnets []string) {
	t.Helper()
	subnetOf := map[string]string{}
	for _, s := range t570List(t, c, "subnet") {
		raw := s.RawStruct.(ec2types.Subnet)
		if v := aws.ToString(raw.VpcId); subnetOf[v] == "" {
			subnetOf[v] = s.ID
		}
	}
	for _, v := range t570List(t, c, "vpc") {
		if sub := subnetOf[v.ID]; sub != "" {
			vpcs = append(vpcs, v.ID)
			subnets = append(subnets, sub)
			if len(vpcs) == n {
				return vpcs, subnets
			}
		}
	}
	t.Fatalf("demo account holds fewer than %d VPCs with subnets", n)
	return nil, nil
}

// A transit gateway's VPCs and subnets are those of its live attachments.
// An attachment AWS reports deleted, failed or rejected
// (TransitGatewayAttachmentState) connects nothing.
func TestT570_TGWVPCAndSubnet_CountOnlyLiveAttachments(t *testing.T) {
	c := t570Demo()
	tgw := t570List(t, c, "tgw")[0]
	vpcs, subnets := t570VPCsWithSubnets(t, c, 4)
	att := func(id, vpc, subnet string, state ec2types.TransitGatewayAttachmentState) ec2types.TransitGatewayVpcAttachment {
		return ec2types.TransitGatewayVpcAttachment{
			TransitGatewayAttachmentId: aws.String(id),
			TransitGatewayId:           aws.String(tgw.ID),
			VpcId:                      aws.String(vpc),
			VpcOwnerId:                 aws.String(t570Account),
			SubnetIds:                  []string{subnet},
			State:                      state,
		}
	}
	c.EC2 = &t570EC2{EC2API: c.EC2, tgwVPC: []ec2types.TransitGatewayVpcAttachment{
		att("tgw-attach-0live00000000001", vpcs[0], subnets[0], ec2types.TransitGatewayAttachmentStateAvailable),
		att("tgw-attach-0gone00000000002", vpcs[1], subnets[1], ec2types.TransitGatewayAttachmentStateDeleted),
		att("tgw-attach-0fail00000000003", vpcs[2], subnets[2], ec2types.TransitGatewayAttachmentStateFailed),
		att("tgw-attach-0rjct00000000004", vpcs[3], subnets[3], ec2types.TransitGatewayAttachmentStateRejected),
	}}
	t570Exact(t, t570Pivot(t, c, tgw, "tgw", "vpc"), vpcs[0])
	t570Exact(t, t570Pivot(t, c, tgw, "tgw", "subnet"), subnets[0])
}

// A VPC's transit gateways are those it is attached to through a live
// attachment.
func TestT570_VPCTGW_CountsOnlyLiveAttachments(t *testing.T) {
	c := t570Demo()
	tgws := t570List(t, c, "tgw")
	if len(tgws) < 3 {
		t.Fatalf("demo account holds %d transit gateways, want 3", len(tgws))
	}
	vpc := t570List(t, c, "vpc")[0]
	att := func(id, tgw string, state ec2types.TransitGatewayAttachmentState) ec2types.TransitGatewayAttachment {
		return ec2types.TransitGatewayAttachment{
			TransitGatewayAttachmentId: aws.String(id),
			TransitGatewayId:           aws.String(tgw),
			TransitGatewayOwnerId:      aws.String(t570Account),
			ResourceId:                 aws.String(vpc.ID),
			ResourceOwnerId:            aws.String(t570Account),
			ResourceType:               ec2types.TransitGatewayAttachmentResourceTypeVpc,
			State:                      state,
		}
	}
	c.EC2 = &t570EC2{EC2API: c.EC2, tgwAtt: []ec2types.TransitGatewayAttachment{
		att("tgw-attach-0live00000000001", tgws[0].ID, ec2types.TransitGatewayAttachmentStateAvailable),
		att("tgw-attach-0gone00000000002", tgws[1].ID, ec2types.TransitGatewayAttachmentStateDeleted),
		att("tgw-attach-0rjct00000000004", tgws[2].ID, ec2types.TransitGatewayAttachmentStateRejected),
	}}
	t570Exact(t, t570Pivot(t, c, vpc, "vpc", "tgw"), tgws[0].ID)
}

type t570SESv1 struct {
	awsclient.SESV1API
	rules []sestypes.ReceiptRule
}

func (f *t570SESv1) DescribeActiveReceiptRuleSet(context.Context, *ses.DescribeActiveReceiptRuleSetInput, ...func(*ses.Options)) (*ses.DescribeActiveReceiptRuleSetOutput, error) {
	return &ses.DescribeActiveReceiptRuleSetOutput{
		Metadata: &sestypes.ReceiptRuleSetMetadata{Name: aws.String("inbound")},
		Rules:    f.rules,
	}, nil
}

type t570SESv2 struct {
	awsclient.SESv2API
	identity, configSet string
	destinations        []sesv2types.EventDestination
}

func (f *t570SESv2) GetEmailIdentity(ctx context.Context, in *sesv2.GetEmailIdentityInput, opt ...func(*sesv2.Options)) (*sesv2.GetEmailIdentityOutput, error) {
	out, err := f.SESv2API.GetEmailIdentity(ctx, in, opt...)
	if err != nil || aws.ToString(in.EmailIdentity) != f.identity {
		return out, err
	}
	cp := *out
	cp.ConfigurationSetName = aws.String(f.configSet)
	return &cp, nil
}

func (f *t570SESv2) GetConfigurationSetEventDestinations(ctx context.Context, in *sesv2.GetConfigurationSetEventDestinationsInput, opt ...func(*sesv2.Options)) (*sesv2.GetConfigurationSetEventDestinationsOutput, error) {
	if aws.ToString(in.ConfigurationSetName) == f.configSet {
		return &sesv2.GetConfigurationSetEventDestinationsOutput{EventDestinations: f.destinations}, nil
	}
	return f.SESv2API.GetConfigurationSetEventDestinations(ctx, in, opt...)
}

func t570ReceiptRule(name string, enabled bool, fnARN, bucket string) sestypes.ReceiptRule {
	return sestypes.ReceiptRule{
		Name:        aws.String(name),
		Enabled:     enabled,
		ScanEnabled: true,
		TlsPolicy:   sestypes.TlsPolicyOptional,
		Actions: []sestypes.ReceiptAction{
			{S3Action: &sestypes.S3Action{BucketName: aws.String(bucket), ObjectKeyPrefix: aws.String("inbound/")}},
			{LambdaAction: &sestypes.LambdaAction{FunctionArn: aws.String(fnARN), InvocationType: sestypes.InvocationTypeEvent}},
		},
	}
}

// A receipt rule with Enabled=false processes no mail, so its Lambda and S3
// actions are not the identity's.
func TestT570_SESLambdaAndS3_CountOnlyEnabledReceiptRules(t *testing.T) {
	c := t570Demo()
	b := t570Buckets(t, c, 2)
	c.SES = &t570SESv1{SESV1API: c.SES, rules: []sestypes.ReceiptRule{
		t570ReceiptRule("store-and-process", true, "arn:aws:lambda:us-east-1:123456789012:function:audit-logger", b[0]),
		t570ReceiptRule("legacy-archive", false, "arn:aws:lambda:us-east-1:123456789012:function:email-sender", b[1]),
	}}
	id := t570Row(t, c, "ses", "acme-corp.com")
	t570Exact(t, t570Pivot(t, c, id, "ses", "lambda"), "audit-logger")
	t570Exact(t, t570Pivot(t, c, id, "ses", "s3"), b[0])
}

// An event destination with Enabled=false publishes nothing, so its SNS topic
// and EventBridge bus are not the identity's.
func TestT570_SESSnsAndEbRule_CountOnlyEnabledEventDestinations(t *testing.T) {
	c := t570Demo()
	topics := t570List(t, c, "sns")
	c.SESv2 = &t570SESv2{SESv2API: c.SESv2, identity: "acme-corp.com", configSet: "transactional", destinations: []sesv2types.EventDestination{
		{
			Name:               aws.String("bounces-to-ops"),
			Enabled:            true,
			MatchingEventTypes: []sesv2types.EventType{sesv2types.EventTypeBounce, sesv2types.EventTypeComplaint},
			SnsDestination:     &sesv2types.SnsDestination{TopicArn: aws.String(topics[0].ID)},
		},
		{
			Name:               aws.String("old-deliveries"),
			Enabled:            false,
			MatchingEventTypes: []sesv2types.EventType{sesv2types.EventTypeDelivery},
			SnsDestination:     &sesv2types.SnsDestination{TopicArn: aws.String(topics[1].ID)},
		},
		{
			Name:                   aws.String("old-bus"),
			Enabled:                false,
			MatchingEventTypes:     []sesv2types.EventType{sesv2types.EventTypeSend},
			EventBridgeDestination: &sesv2types.EventBridgeDestination{EventBusArn: aws.String("arn:aws:events:us-east-1:123456789012:event-bus/default")},
		},
	}}
	id := t570Row(t, c, "ses", "acme-corp.com")
	t570Exact(t, t570Pivot(t, c, id, "ses", "sns"), topics[0].ID)
	t570Exact(t, t570Pivot(t, c, id, "ses", "eb-rule"))
}

type t570OpenSearch struct {
	awsclient.OpenSearchAPI
	domain  string
	edit    func(*opensearchtypes.DomainStatus)
	cfgErr  error
	cfgCall int
}

func (f *t570OpenSearch) DescribeDomains(ctx context.Context, in *opensearch.DescribeDomainsInput, opt ...func(*opensearch.Options)) (*opensearch.DescribeDomainsOutput, error) {
	out, err := f.OpenSearchAPI.DescribeDomains(ctx, in, opt...)
	if err != nil {
		return out, err
	}
	for i := range out.DomainStatusList {
		if aws.ToString(out.DomainStatusList[i].DomainName) == f.domain {
			f.edit(&out.DomainStatusList[i])
		}
	}
	return out, nil
}

func (f *t570OpenSearch) DescribeDomainConfig(ctx context.Context, in *opensearch.DescribeDomainConfigInput, opt ...func(*opensearch.Options)) (*opensearch.DescribeDomainConfigOutput, error) {
	f.cfgCall++
	if f.cfgErr != nil {
		return nil, f.cfgErr
	}
	return f.OpenSearchAPI.(interface {
		DescribeDomainConfig(context.Context, *opensearch.DescribeDomainConfigInput, ...func(*opensearch.Options)) (*opensearch.DescribeDomainConfigOutput, error)
	}).DescribeDomainConfig(ctx, in, opt...)
}

// A log-publishing option with Enabled=false publishes nothing to its group.
func TestT570_OpenSearchLogs_CountsOnlyEnabledPublishingOptions(t *testing.T) {
	c := t570Demo()
	domain := t570List(t, c, "opensearch")[0].ID
	const slow = "/aws/opensearch/acme-logs/search-slow"
	const audit = "/aws/opensearch/acme-logs/audit"
	c.OpenSearch = &t570OpenSearch{OpenSearchAPI: c.OpenSearch, domain: domain, edit: func(d *opensearchtypes.DomainStatus) {
		d.LogPublishingOptions = map[string]opensearchtypes.LogPublishingOption{
			"SEARCH_SLOW_LOGS": {Enabled: aws.Bool(true), CloudWatchLogsLogGroupArn: aws.String("arn:aws:logs:us-east-1:123456789012:log-group:" + slow)},
			"AUDIT_LOGS":       {Enabled: aws.Bool(false), CloudWatchLogsLogGroupArn: aws.String("arn:aws:logs:us-east-1:123456789012:log-group:" + audit)},
		}
	}}
	d := t570Row(t, c, "opensearch", domain)
	t570Exact(t, t570Pivot(t, c, d, "opensearch", "logs"), slow)
}
