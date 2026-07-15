package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/wafv2"
)

// WAFv2ListWebACLsAPI defines the interface for the WAFv2 ListWebACLs operation.
type WAFv2ListWebACLsAPI interface {
	ListWebACLs(ctx context.Context, params *wafv2.ListWebACLsInput, optFns ...func(*wafv2.Options)) (*wafv2.ListWebACLsOutput, error)
}

// WAFGetLoggingConfigurationAPI defines the interface for the WAFv2 GetLoggingConfiguration operation.
// Used by Wave 2 enrichment to detect WebACLs with no logging configured.
type WAFGetLoggingConfigurationAPI interface {
	GetLoggingConfiguration(ctx context.Context, params *wafv2.GetLoggingConfigurationInput, optFns ...func(*wafv2.Options)) (*wafv2.GetLoggingConfigurationOutput, error)
}

// WAFv2ListResourcesForWebACLAPI defines the interface for the WAFv2
// ListResourcesForWebACL operation.
type WAFv2ListResourcesForWebACLAPI interface {
	ListResourcesForWebACL(ctx context.Context, params *wafv2.ListResourcesForWebACLInput, optFns ...func(*wafv2.Options)) (*wafv2.ListResourcesForWebACLOutput, error)
}

// WAFv2GetWebACLAPI defines the interface for the WAFv2 GetWebACL operation.
// Used by EnrichWAFLogging to count BLOCK rules per WebACL.
type WAFv2GetWebACLAPI interface {
	GetWebACL(ctx context.Context, params *wafv2.GetWebACLInput, optFns ...func(*wafv2.Options)) (*wafv2.GetWebACLOutput, error)
}

// WAFv2GetWebACLForResourceAPI is the forward "resource → WebACL" WAF lookup.
type WAFv2GetWebACLForResourceAPI interface {
	GetWebACLForResource(ctx context.Context, params *wafv2.GetWebACLForResourceInput, optFns ...func(*wafv2.Options)) (*wafv2.GetWebACLForResourceOutput, error)
}

// WAFv2API is the aggregate interface covering all WAFv2 operations used by a9s fetchers.
// *wafv2.Client structurally satisfies this interface.
type WAFv2API interface {
	WAFv2ListWebACLsAPI
	WAFv2ListResourcesForWebACLAPI
	WAFGetLoggingConfigurationAPI // Wave 2 enrichment
	// WAFv2GetWebACLAPI is intentionally excluded from the aggregate — EnrichWAFLogging
	// calls GetWebACL via type assertion so test fakes that only cover logging do not
	// need to implement it.
}
