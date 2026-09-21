// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
)

// A fake that answers a filtered DescribeNetworkInterfaces with the whole
// account turns every pivot that narrows by description or security group
// into a list of every interface there is.
func TestDemoEC2FakeHonoursNetworkInterfaceFilters(t *testing.T) {
	f := fakes.NewEC2()
	ctx := context.Background()

	all, err := f.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{})
	if err != nil {
		t.Fatalf("unfiltered DescribeNetworkInterfaces: %v", err)
	}
	if len(all.NetworkInterfaces) == 0 {
		t.Fatal("no ENI fixtures — nothing to filter")
	}

	var sampleDesc string
	var sampleGroup string
	for _, ni := range all.NetworkInterfaces {
		if ni.Description == nil || len(ni.Groups) == 0 || ni.Groups[0].GroupId == nil {
			continue
		}
		sampleDesc = *ni.Description
		sampleGroup = *ni.Groups[0].GroupId
		break
	}
	if sampleDesc == "" {
		t.Fatal("no ENI fixture carries both a description and a security group")
	}

	t.Run("no fixture matches", func(t *testing.T) {
		out, err := f.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{
			Filters: []ec2types.Filter{
				{Name: aws.String("description"), Values: []string{"RDSNetworkInterface"}},
				{Name: aws.String("group-id"), Values: []string{"sg-0000000000000dead"}},
			},
		})
		if err != nil {
			t.Fatalf("DescribeNetworkInterfaces: %v", err)
		}
		if len(out.NetworkInterfaces) != 0 {
			t.Errorf("filters matching nothing returned %d of %d interfaces",
				len(out.NetworkInterfaces), len(all.NetworkInterfaces))
		}
	})

	t.Run("description narrows", func(t *testing.T) {
		out, err := f.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{
			Filters: []ec2types.Filter{{Name: aws.String("description"), Values: []string{sampleDesc}}},
		})
		if err != nil {
			t.Fatalf("DescribeNetworkInterfaces: %v", err)
		}
		if len(out.NetworkInterfaces) == 0 || len(out.NetworkInterfaces) == len(all.NetworkInterfaces) {
			t.Errorf("description=%q returned %d of %d interfaces, want a non-empty subset",
				sampleDesc, len(out.NetworkInterfaces), len(all.NetworkInterfaces))
		}
		for _, ni := range out.NetworkInterfaces {
			if aws.ToString(ni.Description) != sampleDesc {
				t.Errorf("returned %s with description %q, want %q",
					aws.ToString(ni.NetworkInterfaceId), aws.ToString(ni.Description), sampleDesc)
			}
		}
	})

	t.Run("group-id matches any of the interface's groups", func(t *testing.T) {
		out, err := f.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{
			Filters: []ec2types.Filter{{Name: aws.String("group-id"), Values: []string{sampleGroup}}},
		})
		if err != nil {
			t.Fatalf("DescribeNetworkInterfaces: %v", err)
		}
		if len(out.NetworkInterfaces) == 0 || len(out.NetworkInterfaces) == len(all.NetworkInterfaces) {
			t.Errorf("group-id=%q returned %d of %d interfaces, want a non-empty subset",
				sampleGroup, len(out.NetworkInterfaces), len(all.NetworkInterfaces))
		}
		for _, ni := range out.NetworkInterfaces {
			found := false
			for _, g := range ni.Groups {
				if aws.ToString(g.GroupId) == sampleGroup {
					found = true
				}
			}
			if !found {
				t.Errorf("returned %s, which is in no group %q",
					aws.ToString(ni.NetworkInterfaceId), sampleGroup)
			}
		}
	})
}
