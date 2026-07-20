// fakes_transfer_test.go is the single source for AWS Transfer Family
// SDK-interface fakes shared across tests/unit.
//
// Convention (docs/go-codebase-checklist.md, DRY section): one configurable
// fake per SDK interface, in a service-named fakes_<service>_test.go file --
// never re-implement the same interface under a new name in another file,
// and never add another wave/batch-named fake file (fakes_us1_batchN_test.go,
// fakes_related_checkers_branch_coverage_test.go, fakes_related_checkers_misc_test.go are historical
// accretion naming, not a pattern to extend).
package unit

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/transfer"
	transfertypes "github.com/aws/aws-sdk-go-v2/service/transfer/types"
)

// fakeTransferServers implements awsclient.TransferAPI's ListServers and
// DescribeServer operations. transferUnimplementedAPI (aws_transfer_test.go)
// supplies no-op stubs for the rest of the interface.
//
// ListServers: configure ListErr to fail the call, else Listed is returned
// verbatim.
//
// DescribeServer: configured in priority order —
//  1. DescribeErr, if set, is returned unconditionally (regardless of the
//     requested server ID) — for regression tests that must prove
//     DescribeServer is never called after a ListServers failure.
//  2. ErrByName, keyed by server ID, if the ID has an entry.
//  3. Servers, keyed by server ID, if the ID has an entry.
//  4. Described, if set, is returned unconditionally regardless of ID.
//  5. Otherwise a "not found" error, mirroring the real API.
type fakeTransferServers struct {
	transferUnimplementedAPI

	Listed  []transfertypes.ListedServer
	ListErr error

	Servers     map[string]transfertypes.DescribedServer
	ErrByName   map[string]error
	Described   *transfertypes.DescribedServer
	DescribeErr error
}

func (f *fakeTransferServers) ListServers(
	_ context.Context, _ *transfer.ListServersInput, _ ...func(*transfer.Options),
) (*transfer.ListServersOutput, error) {
	if f.ListErr != nil {
		return nil, f.ListErr
	}
	return &transfer.ListServersOutput{Servers: f.Listed}, nil
}

func (f *fakeTransferServers) DescribeServer(
	_ context.Context, input *transfer.DescribeServerInput, _ ...func(*transfer.Options),
) (*transfer.DescribeServerOutput, error) {
	if f.DescribeErr != nil {
		return nil, f.DescribeErr
	}
	id := ""
	if input != nil {
		id = aws.ToString(input.ServerId)
	}
	if err, ok := f.ErrByName[id]; ok {
		return nil, err
	}
	if s, ok := f.Servers[id]; ok {
		return &transfer.DescribeServerOutput{Server: &s}, nil
	}
	if f.Described != nil {
		d := *f.Described
		return &transfer.DescribeServerOutput{Server: &d}, nil
	}
	return nil, fmt.Errorf("server %q not found", id)
}
