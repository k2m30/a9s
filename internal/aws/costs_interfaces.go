package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
)

// CostsGetCostAndUsageAPI defines the interface for the Cost Explorer GetCostAndUsage operation.
type CostsGetCostAndUsageAPI interface {
	GetCostAndUsage(ctx context.Context, params *costexplorer.GetCostAndUsageInput, optFns ...func(*costexplorer.Options)) (*costexplorer.GetCostAndUsageOutput, error)
}

// CostsGetCostAndUsageWithResourcesAPI defines the interface for the Cost Explorer GetCostAndUsageWithResources operation.
type CostsGetCostAndUsageWithResourcesAPI interface {
	GetCostAndUsageWithResources(ctx context.Context, params *costexplorer.GetCostAndUsageWithResourcesInput, optFns ...func(*costexplorer.Options)) (*costexplorer.GetCostAndUsageWithResourcesOutput, error)
}

// CostsGetAnomaliesAPI defines the interface for the Cost Explorer GetAnomalies operation.
type CostsGetAnomaliesAPI interface {
	GetAnomalies(ctx context.Context, params *costexplorer.GetAnomaliesInput, optFns ...func(*costexplorer.Options)) (*costexplorer.GetAnomaliesOutput, error)
}

// CostsGetDimensionValuesAPI defines the interface for the Cost Explorer GetDimensionValues operation.
type CostsGetDimensionValuesAPI interface {
	GetDimensionValues(ctx context.Context, params *costexplorer.GetDimensionValuesInput, optFns ...func(*costexplorer.Options)) (*costexplorer.GetDimensionValuesOutput, error)
}

// CostsAPI is the aggregate interface covering all Cost Explorer operations
// used by a9s fetchers. *costexplorer.Client structurally satisfies this
// interface.
type CostsAPI interface {
	CostsGetCostAndUsageAPI
	CostsGetCostAndUsageWithResourcesAPI
	CostsGetAnomaliesAPI
	CostsGetDimensionValuesAPI
}
