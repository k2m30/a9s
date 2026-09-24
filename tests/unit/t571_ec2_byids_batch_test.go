package unit_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// t571EC2DescribeByID answers DescribeInstances for the ids it is given, one
// reservation per instance, and records how many ids each call carried.
type t571EC2DescribeByID struct {
	awsclient.EC2API
	batches []int
}

func (f *t571EC2DescribeByID) DescribeInstances(_ context.Context, in *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	f.batches = append(f.batches, len(in.InstanceIds))
	out := &ec2.DescribeInstancesOutput{}
	for _, id := range in.InstanceIds {
		out.Reservations = append(out.Reservations, ec2types.Reservation{
			OwnerId: aws.String("123456789012"),
			Instances: []ec2types.Instance{{
				InstanceId:       aws.String(id),
				InstanceType:     ec2types.InstanceTypeT3Micro,
				State:            &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning, Code: aws.Int32(16)},
				VpcId:            aws.String("vpc-0a1b2c3d4e5f60718"),
				PrivateIpAddress: aws.String("10.0.1.10"),
				Tags:             []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("web-" + id)}},
			}},
		})
	}
	return out, nil
}

func t571InstanceIDs(n int) []string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("i-0%016x", i+1)
	}
	return ids
}

// An exact-id EC2 drill sends its ids in DescribeInstances batches of at most
// 1,000 — "you should not specify more than 1,000 IDs in a single call"
// (https://docs.aws.amazon.com/ec2/latest/devguide/ec2-api-pagination.html)
// — and returns every instance asked for.
func TestEC2FetchByIDs_BatchesOfAtMostOneThousand(t *testing.T) {
	ids := t571InstanceIDs(2500)
	fake := &t571EC2DescribeByID{}

	rows, err := resource.GetFetchByIDs("ec2")(context.Background(), &awsclient.ServiceClients{EC2: fake}, ids)

	if err != nil {
		t.Fatalf("err = %v, want nil: every id exists", err)
	}
	sent := 0
	for i, n := range fake.batches {
		if n > 1000 {
			t.Errorf("DescribeInstances call %d carried %d ids, want at most 1000", i+1, n)
		}
		sent += n
	}
	if sent != len(ids) {
		t.Errorf("ids sent across %v = %d, want %d, each once", fake.batches, sent, len(ids))
	}
	got := make(map[string]bool, len(rows))
	for _, r := range rows {
		got[r.ID] = true
	}
	for _, id := range ids {
		if !got[id] {
			t.Errorf("instance %s not returned (%d of %d rows)", id, len(rows), len(ids))
			break
		}
	}
	if len(rows) != len(ids) {
		t.Errorf("rows = %d, want %d", len(rows), len(ids))
	}
}

// A drill within one batch is one call.
func TestEC2FetchByIDs_SmallDrillIsOneCall(t *testing.T) {
	fake := &t571EC2DescribeByID{}

	rows, err := resource.GetFetchByIDs("ec2")(context.Background(), &awsclient.ServiceClients{EC2: fake}, t571InstanceIDs(3))

	if err != nil || len(rows) != 3 {
		t.Fatalf("rows = %d, err = %v, want 3 rows and no error", len(rows), err)
	}
	if len(fake.batches) != 1 || fake.batches[0] != 3 {
		t.Errorf("DescribeInstances batches = %v, want [3]", fake.batches)
	}
}
