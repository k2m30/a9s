// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/s3control"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
)

// S3ControlFake implements aws.S3ControlAPI against the demo account's
// public access block.
type S3ControlFake struct{}

func NewS3Control() *S3ControlFake { return &S3ControlFake{} }

func (f *S3ControlFake) GetPublicAccessBlock(_ context.Context, _ *s3control.GetPublicAccessBlockInput, _ ...func(*s3control.Options)) (*s3control.GetPublicAccessBlockOutput, error) {
	block := fixtures.S3AccountBlock
	return &s3control.GetPublicAccessBlockOutput{PublicAccessBlockConfiguration: &block}, nil
}
