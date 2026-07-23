// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
)

// CFNDescribeStacksAPI defines the interface for the CloudFormation DescribeStacks operation.
type CFNDescribeStacksAPI interface {
	DescribeStacks(ctx context.Context, params *cloudformation.DescribeStacksInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error)
}

// CFNDescribeStackEventsAPI defines the interface for the CloudFormation DescribeStackEvents operation.
type CFNDescribeStackEventsAPI interface {
	DescribeStackEvents(ctx context.Context, params *cloudformation.DescribeStackEventsInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DescribeStackEventsOutput, error)
}

// CFNListStackResourcesAPI defines the interface for the CloudFormation ListStackResources operation.
type CFNListStackResourcesAPI interface {
	ListStackResources(ctx context.Context, params *cloudformation.ListStackResourcesInput, optFns ...func(*cloudformation.Options)) (*cloudformation.ListStackResourcesOutput, error)
}

// CFNGetTemplateAPI defines the interface for the CloudFormation GetTemplate
// operation. Used by the on-demand cfn detail enricher (enrichCfn) — not
// part of the CFNAPI aggregate since no fetcher or related checker needs it.
type CFNGetTemplateAPI interface {
	GetTemplate(ctx context.Context, params *cloudformation.GetTemplateInput, optFns ...func(*cloudformation.Options)) (*cloudformation.GetTemplateOutput, error)
}

// CFNAPI is the aggregate interface covering all CloudFormation operations used by a9s fetchers.
// *cloudformation.Client structurally satisfies this interface.
type CFNAPI interface {
	CFNDescribeStacksAPI
	CFNDescribeStackEventsAPI
	CFNListStackResourcesAPI
}
