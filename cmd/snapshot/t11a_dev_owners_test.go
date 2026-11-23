package main

import (
	"context"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
)

func TestT11aDev_PageAll_WalksRoute53RecordSetsByStartRecord(t *testing.T) {
	var starts []string
	cfg := answeringConfig(func(service, op string, in any) (any, error) {
		switch op {
		case "ListHostedZones":
			return &route53.ListHostedZonesOutput{HostedZones: []r53types.HostedZone{{Id: aws.String("/hostedzone/Z0EXAMPLE"), Name: aws.String("example.com.")}}}, nil
		case "ListResourceRecordSets":
			l := in.(*route53.ListResourceRecordSetsInput)
			starts = append(starts, aws.ToString(l.StartRecordName)+"/"+string(l.StartRecordType))
			if l.StartRecordName == nil {
				return &route53.ListResourceRecordSetsOutput{ResourceRecordSets: []r53types.ResourceRecordSet{{Name: aws.String("a.example.com."), Type: r53types.RRTypeA}},
					IsTruncated: true, NextRecordName: aws.String("b.example.com."), NextRecordType: r53types.RRTypeCname}, nil
			}
			return &route53.ListResourceRecordSetsOutput{ResourceRecordSets: []r53types.ResourceRecordSet{{Name: aws.String("b.example.com."), Type: r53types.RRTypeCname,
				AliasTarget: &r53types.AliasTarget{DNSName: aws.String("lb-1.eu-west-1.elb.amazonaws.com.")}}}}, nil
		}
		return t11aDefault(service, in)
	})
	data, err := captureR53(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	zone := data.(r53Data).HostedZones[0]
	if !slices.Equal(starts, []string{"/", "b.example.com./CNAME"}) || len(zone.Records) != 2 || zone.Records[1].AliasDNSName != "lb-1.eu-west-1.elb.amazonaws.com." || zone.RecordsOutcome != "ok" {
		t.Errorf("requests %v, zone %+v", starts, zone)
	}
}

func TestT11aDev_DescribeListed_ABatchTheServiceAnswersNotFoundHasNoRows(t *testing.T) {
	rows := describeListed([]string{"a", "b"}, 10, func([]string) (map[string]string, []string, error) {
		return nil, nil, t11aAPIError("ClusterNotFoundException", "Cluster not found.")
	})
	if len(rows) != 0 {
		t.Errorf("rows = %+v, want none", rows)
	}
}

func TestT11aDev_DescribeListed_KeepsAnItemTheAnswerNamesUnderItsOwnID(t *testing.T) {
	rows := describeListed([]string{"asked"}, 1, func([]string) (map[string]string, []string, error) {
		return map[string]string{"answered": "x"}, nil, nil
	})
	var got []string
	for _, r := range rows {
		got = append(got, r.id+":"+r.read.Outcome)
	}
	if !slices.Equal(got, []string{"asked:" + outcomeNotFound, "answered:ok"}) {
		t.Errorf("rows = %v", got)
	}
}
