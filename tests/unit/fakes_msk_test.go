// fakes_msk_test.go is the single source for MSK SDK-interface fakes shared
// across tests/unit.
//
// Convention (docs/go-codebase-checklist.md, DRY section): one configurable
// fake per SDK interface, in a service-named fakes_<service>_test.go file --
// never re-implement the same interface under a new name in another file,
// and never add another wave/batch-named fake file (fakes_us1_batchN_test.go,
// fakes_wave5_test.go, fakes_coverage_restore_test.go are historical
// accretion naming, not a pattern to extend).
package unit

import (
	"context"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/kafka"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

// fakeMSKDescribeClusterV2 implements awsclient.MSKDescribeClusterV2API.
//
// Configure any combination of:
//   - Results: maps ClusterArn -> cluster info returned on a hit
//   - ErrByArn: maps ClusterArn -> error, takes priority over Results
//   - RejectNonARN: reject any ClusterArn not prefixed "arn:aws:" with a
//     ValidationError, mirroring the real API — used by regression tests
//     that must prove a caller passes the ARN, not a bare resource name
//
// CalledWith (thread-safe) always records the ClusterArn from the most
// recent call — EnrichMSKCluster fans calls out across goroutines.
type fakeMSKDescribeClusterV2 struct {
	awsclient.MSKAPI

	Results      map[string]*kafkatypes.Cluster
	ErrByArn     map[string]error
	RejectNonARN bool

	mu         sync.Mutex
	calledWith string
}

func (f *fakeMSKDescribeClusterV2) DescribeClusterV2(
	_ context.Context,
	in *kafka.DescribeClusterV2Input,
	_ ...func(*kafka.Options),
) (*kafka.DescribeClusterV2Output, error) {
	arn := ""
	if in != nil && in.ClusterArn != nil {
		arn = *in.ClusterArn
	}
	f.mu.Lock()
	f.calledWith = arn
	f.mu.Unlock()

	if f.RejectNonARN && !strings.HasPrefix(arn, "arn:aws:") {
		return nil, &smithy.GenericAPIError{
			Code:    "ValidationError",
			Message: "'" + arn + "' is not a valid ARN",
		}
	}
	if f.ErrByArn != nil {
		if err, ok := f.ErrByArn[arn]; ok {
			return nil, err
		}
	}
	cluster, ok := f.Results[arn]
	if !ok {
		return &kafka.DescribeClusterV2Output{}, nil
	}
	return &kafka.DescribeClusterV2Output{ClusterInfo: cluster}, nil
}

// CalledWith returns the ClusterArn passed to the most recent
// DescribeClusterV2 call. Safe for concurrent use.
func (f *fakeMSKDescribeClusterV2) CalledWith() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calledWith
}
