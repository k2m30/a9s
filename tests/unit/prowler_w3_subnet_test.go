package unit

// prowler_w3_subnet_test.go — subnet.auto-public-ip.
//
// A subnet with MapPublicIpOnLaunch set gives every instance launched into it
// a routable address without anyone asking for one, so the posture signal has
// to be independent of the subnet's lifecycle state and must stay silent when
// AWS omits the flag (unknown is not misconfigured).

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	w3CodeSubnetAutoPublicIP = domain.FindingCode("subnet.auto-public-ip")
	w3CodeSubnetPending      = domain.FindingCode("subnet.state.pending")

	w3PhraseSubnetAutoPublicIP = "auto-assigns public IPs"
)

type w3SubnetAPI struct{ subnets []ec2types.Subnet }

func (f w3SubnetAPI) DescribeSubnets(_ context.Context, _ *ec2.DescribeSubnetsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	return &ec2.DescribeSubnetsOutput{Subnets: f.subnets}, nil
}

func w3FetchSubnets(t *testing.T, subnets ...ec2types.Subnet) []resource.Resource {
	t.Helper()
	res, err := awsclient.FetchSubnetsPage(context.Background(), w3SubnetAPI{subnets: subnets}, "")
	if err != nil {
		t.Fatalf("FetchSubnetsPage: %v", err)
	}
	return res.Resources
}

// w3Subnet is a realistic DescribeSubnets row; mapPublicIP nil means AWS did
// not report the flag.
func w3Subnet(id, state string, mapPublicIP *bool) ec2types.Subnet {
	return ec2types.Subnet{
		SubnetId:                aws.String(id),
		VpcId:                   aws.String("vpc-0abc123"),
		CidrBlock:               aws.String("10.0.1.0/24"),
		AvailabilityZone:        aws.String("us-east-1a"),
		AvailableIpAddressCount: aws.Int32(250),
		State:                   ec2types.SubnetState(state),
		MapPublicIpOnLaunch:     mapPublicIP,
		Tags:                    []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("acme-public-1a")}},
	}
}

func TestW3SubnetAutoPublicIP_Flagged(t *testing.T) {
	rows := w3FetchSubnets(t, w3Subnet("subnet-0aa11bb22cc33dd44", "available", aws.Bool(true)))

	f, ok := w3FindingByCode(rows[0].Findings, w3CodeSubnetAutoPublicIP)
	if !ok {
		t.Fatalf("MapPublicIpOnLaunch=true produced no %s; findings=%+v", w3CodeSubnetAutoPublicIP, rows[0].Findings)
	}
	if f.Phrase != w3PhraseSubnetAutoPublicIP {
		t.Errorf("Phrase = %q, want %q", f.Phrase, w3PhraseSubnetAutoPublicIP)
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
	w3AssertRows(t, rows[0].AttentionDetails[w3CodeSubnetAutoPublicIP].Rows, [][2]string{
		{"MapPublicIpOnLaunch", "true"},
	})
}

func TestW3SubnetAutoPublicIP_HealthyAndUnknown(t *testing.T) {
	for _, tc := range []struct {
		name        string
		mapPublicIP *bool
	}{
		{"explicitly false", aws.Bool(false)},
		{"field absent", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := w3FetchSubnets(t, w3Subnet("subnet-0aa11bb22cc33dd44", "available", tc.mapPublicIP))
			if f, ok := w3FindingByCode(rows[0].Findings, w3CodeSubnetAutoPublicIP); ok {
				t.Errorf("got %s (%q), want no finding", f.Code, f.Phrase)
			}
		})
	}
}

// TestW3SubnetAutoPublicIP_IndependentOfState pins that the posture signal and
// the lifecycle signal are separate findings — collapsing them would hide
// whichever one loses the switch.
func TestW3SubnetAutoPublicIP_IndependentOfState(t *testing.T) {
	rows := w3FetchSubnets(t, w3Subnet("subnet-0aa11bb22cc33dd44", "pending", aws.Bool(true)))

	if _, ok := w3FindingByCode(rows[0].Findings, w3CodeSubnetPending); !ok {
		t.Errorf("missing %s; findings=%+v", w3CodeSubnetPending, rows[0].Findings)
	}
	if _, ok := w3FindingByCode(rows[0].Findings, w3CodeSubnetAutoPublicIP); !ok {
		t.Errorf("missing %s; findings=%+v", w3CodeSubnetAutoPublicIP, rows[0].Findings)
	}
	if len(rows[0].Findings) != 2 {
		t.Errorf("got %d findings %+v, want exactly 2", len(rows[0].Findings), rows[0].Findings)
	}
}

// TestW3SubnetAutoPublicIP_PerRowIsolation pins that the flag is read per
// subnet: one public subnet in a page must not paint its healthy neighbours.
func TestW3SubnetAutoPublicIP_PerRowIsolation(t *testing.T) {
	rows := w3FetchSubnets(t,
		w3Subnet("subnet-0private0000000", "available", aws.Bool(false)),
		w3Subnet("subnet-0public00000000", "available", aws.Bool(true)),
		w3Subnet("subnet-0private1111111", "available", aws.Bool(false)),
	)
	for _, r := range rows {
		_, got := w3FindingByCode(r.Findings, w3CodeSubnetAutoPublicIP)
		want := r.ID == "subnet-0public00000000"
		if got != want {
			t.Errorf("%s: has %s = %v, want %v", r.ID, w3CodeSubnetAutoPublicIP, got, want)
		}
	}
}
