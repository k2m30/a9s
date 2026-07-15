// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/mwaa"
)

// MWAAListEnvironmentsAPI defines the interface for the MWAA ListEnvironments operation.
type MWAAListEnvironmentsAPI interface {
	ListEnvironments(ctx context.Context, params *mwaa.ListEnvironmentsInput, optFns ...func(*mwaa.Options)) (*mwaa.ListEnvironmentsOutput, error)
}

// MWAAGetEnvironmentAPI defines the interface for the MWAA GetEnvironment operation.
type MWAAGetEnvironmentAPI interface {
	GetEnvironment(ctx context.Context, params *mwaa.GetEnvironmentInput, optFns ...func(*mwaa.Options)) (*mwaa.GetEnvironmentOutput, error)
}

// MWAAAPI is the aggregate interface covering all MWAA operations used by a9s fetchers.
// *mwaa.Client structurally satisfies this interface.
type MWAAAPI interface {
	MWAAListEnvironmentsAPI
	MWAAGetEnvironmentAPI
}
