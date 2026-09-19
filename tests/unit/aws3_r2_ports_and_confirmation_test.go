// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit

// Three facts each read from their own field rather than computed from a
// rendered sentence or spelled twice: the port list a security group leaves
// open, the number agreement in a phrase declared with a bare "(s)", and
// whether an SNS endpoint confirmed its subscription.

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

type aws3SGFake struct{ groups []ec2types.SecurityGroup }

func (f *aws3SGFake) DescribeSecurityGroups(_ context.Context, _ *ec2.DescribeSecurityGroupsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	return &ec2.DescribeSecurityGroupsOutput{SecurityGroups: f.groups}, nil
}

// aws3SG opens [from,to] to the whole internet over TCP.
func aws3SG(id string, from, to int32) ec2types.SecurityGroup {
	return ec2types.SecurityGroup{
		GroupId:     aws.String(id),
		GroupName:   aws.String("acme-" + id),
		VpcId:       aws.String("vpc-0aaaa1111bbbb2222"),
		Description: aws.String("test"),
		IpPermissions: []ec2types.IpPermission{{
			IpProtocol: aws.String("tcp"),
			FromPort:   aws.Int32(from),
			ToPort:     aws.Int32(to),
			IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}},
		}},
	}
}

func aws3FetchSGs(t *testing.T, groups ...ec2types.SecurityGroup) []resource.Resource {
	t.Helper()
	out, err := awsclient.FetchSecurityGroupsPage(context.Background(), &aws3SGFake{groups: groups}, "")
	if err != nil {
		t.Fatalf("FetchSecurityGroupsPage: %v", err)
	}
	return out.Resources
}

// The port list a group leaves open is a field of its own; parsing it back out
// of the rendered risk_summary would make the wording a parse key.
func TestSGPublishesItsOpenPortsAsAField(t *testing.T) {
	rows := aws3FetchSGs(t, aws3SG("sg-0aaa111bbb222ccc1", 22, 22))
	if got := rows[0].Fields["open_ports"]; got != "22" {
		t.Errorf("Fields[open_ports] = %q, want %q", got, "22")
	}
	rows = aws3FetchSGs(t, aws3SG("sg-0aaa111bbb222ccc2", 22, 22), aws3SG("sg-0aaa111bbb222ccc6", 3306, 3306))
	if got := rows[0].Fields["open_ports"] + "|" + rows[1].Fields["open_ports"]; got != "22|3306" {
		t.Errorf("Fields[open_ports] across two groups = %q, want %q", got, "22|3306")
	}
	rows = aws3FetchSGs(t, aws3SG("sg-0aaa111bbb222ccc3", 443, 443))
	if got := rows[0].Fields["open_ports"]; got != "" {
		t.Errorf("Fields[open_ports] = %q for a group opening nothing sensitive, want empty", got)
	}
}

// The EC2 exposure cross-ref reads the group's port list, not its risk_summary
// sentence, so rewording the phrase cannot answer "no ports" and silence the
// instance.
func TestEC2ExposureReadsThePortListNotTheSentence(t *testing.T) {
	const id = "i-0exposed0aaaaaa2"
	sgRows := aws3FetchSGs(t, aws3SG("sg-0mongo000aaaaaa2", 27017, 27017))
	sgRows[0].Fields["risk_summary"] = "reachable by anyone"
	cache := resource.ResourceCache{"sg": resource.ResourceCacheEntry{Resources: sgRows}}

	res, err := pw1EnrichEC2(t, &pw1EC2EnrichFake{}, cache,
		pw1Instance(id, "running", "203.0.113.11", ec2types.HttpTokensStateRequired, "sg-0mongo000aaaaaa2"))
	if err != nil {
		t.Fatalf("EnrichEC2InstanceStatus: %v", err)
	}
	findings := res.Findings[id]
	if len(findings) == 0 {
		t.Fatal("no exposure finding — the cross-ref lost the ports when the sentence was reworded")
	}
	var phrase string
	for _, f := range findings {
		if f.Code == pw1EC2CodeInternetExposed {
			phrase = f.Phrase
		}
	}
	if !strings.Contains(phrase, "27017") {
		t.Errorf("exposure phrase = %q, want it to name port 27017 off the group's own field", phrase)
	}
}

// TestOnePortReadsAsOnePort pins the singular at the two surfaces that name a
// port list: the group's own status cell and the instance's exposure phrase.
func TestOnePortReadsAsOnePort(t *testing.T) {
	rows := aws3FetchSGs(t, aws3SG("sg-0aaa111bbb222ccc4", 22, 22))
	if got := rows[0].Fields["risk_summary"]; got != "port 22 open to 0.0.0.0/0" {
		t.Errorf("risk_summary = %q, want %q", got, "port 22 open to 0.0.0.0/0")
	}
	// One rule spanning 22-3389 leaves every sensitive port inside it open, so
	// the plural arm is exercised with the list sg.go actually derives.
	rows = aws3FetchSGs(t, aws3SG("sg-0aaa111bbb222ccc5", 3306, 3389))
	if got := rows[0].Fields["risk_summary"]; got != "ports 3306, 3389 open to 0.0.0.0/0" {
		t.Errorf("risk_summary = %q, want %q", got, "ports 3306, 3389 open to 0.0.0.0/0")
	}
}

// aws3SNSSubFake answers both subscription list calls with one subscription
// whose SubscriptionArn AWS never filled in.
type aws3SNSSubFake struct{ awsclient.SNSAPI }

func (f *aws3SNSSubFake) ListSubscriptions(_ context.Context, _ *sns.ListSubscriptionsInput, _ ...func(*sns.Options)) (*sns.ListSubscriptionsOutput, error) {
	return &sns.ListSubscriptionsOutput{Subscriptions: []snstypes.Subscription{aws3ArnlessSub()}}, nil
}

func (f *aws3SNSSubFake) ListSubscriptionsByTopic(_ context.Context, _ *sns.ListSubscriptionsByTopicInput, _ ...func(*sns.Options)) (*sns.ListSubscriptionsByTopicOutput, error) {
	return &sns.ListSubscriptionsByTopicOutput{Subscriptions: []snstypes.Subscription{aws3ArnlessSub()}}, nil
}

func aws3ArnlessSub() snstypes.Subscription {
	return snstypes.Subscription{
		TopicArn: aws.String("arn:aws:sns:us-east-1:123456789012:order-events"),
		Protocol: aws.String("email"),
		Endpoint: aws.String("ops@example.com"),
		Owner:    aws.String("123456789012"),
	}
}

// AWS puts the state word where the ARN goes, so a subscription that came back
// with neither has confirmed nothing, on either surface.
func TestSubscriptionWithoutAnArnIsNotConfirmedOnEitherSurface(t *testing.T) {
	fake := &aws3SNSSubFake{}

	listRes, err := awsclient.FetchSNSSubscriptionsPage(context.Background(), fake, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(listRes.Resources) != 1 {
		t.Fatalf("want one subscription, got %d", len(listRes.Resources))
	}
	if got := cellFor(t, "sns-sub", "Confirmed", listRes.Resources[0]); strings.EqualFold(got, "yes") || strings.EqualFold(got, "confirmed") {
		t.Errorf("subscription list Confirmed = %q for a subscription AWS returned no ARN for", got)
	}

	childRes, err := awsclient.FetchSNSTopicSubscriptions(context.Background(), fake,
		"arn:aws:sns:us-east-1:123456789012:order-events", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(childRes.Resources) != 1 {
		t.Fatalf("want one child subscription, got %d", len(childRes.Resources))
	}
	if got := childRes.Resources[0].Fields["confirmation_status"]; strings.EqualFold(got, "confirmed") {
		t.Errorf("by-topic child confirmation_status = %q for a subscription AWS returned no ARN for", got)
	}
}
