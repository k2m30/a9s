package unit_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

type t571DevImagesByID struct {
	awsclient.EC2API
	batches []int
}

func (f *t571DevImagesByID) DescribeImages(_ context.Context, in *ec2.DescribeImagesInput, _ ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	f.batches = append(f.batches, len(in.ImageIds))
	out := &ec2.DescribeImagesOutput{}
	for _, id := range in.ImageIds {
		out.Images = append(out.Images, ec2types.Image{ImageId: aws.String(id), Name: aws.String("img-" + id)})
	}
	return out, nil
}

// An exact-id AMI drill sends its ids in DescribeImages batches of at most
// 1,000, the same bound as every EC2 describe by ID.
func TestT571Dev_AMIFetchByIDs_BatchesOfAtMostOneThousand(t *testing.T) {
	ids := make([]string, 1500)
	for i := range ids {
		ids[i] = fmt.Sprintf("ami-0%016x", i+1)
	}
	fake := &t571DevImagesByID{}
	rows, err := awsclient.FetchAMIsByIDs(context.Background(), fake, ids)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(fake.batches) != 2 || fake.batches[0] != 1000 || fake.batches[1] != 500 {
		t.Errorf("batches = %v, want [1000 500]", fake.batches)
	}
	if len(rows) != len(ids) {
		t.Errorf("rows = %d, want %d", len(rows), len(ids))
	}
}
