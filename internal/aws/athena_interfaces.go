package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/athena"
)

// AthenaListWorkGroupsAPI defines the interface for the Athena ListWorkGroups operation.
type AthenaListWorkGroupsAPI interface {
	ListWorkGroups(ctx context.Context, params *athena.ListWorkGroupsInput, optFns ...func(*athena.Options)) (*athena.ListWorkGroupsOutput, error)
}

// AthenaGetWorkGroupAPI defines the interface for the Athena GetWorkGroup operation.
// Used by EnrichAthenaWorkGroup (Wave 2 enrichment).
type AthenaGetWorkGroupAPI interface {
	GetWorkGroup(ctx context.Context, params *athena.GetWorkGroupInput, optFns ...func(*athena.Options)) (*athena.GetWorkGroupOutput, error)
}

// AthenaAPI is the aggregate interface covering all Athena operations used by a9s fetchers.
// *athena.Client structurally satisfies this interface.
type AthenaAPI interface {
	AthenaListWorkGroupsAPI
	AthenaGetWorkGroupAPI // Wave 2 enrichment
}
