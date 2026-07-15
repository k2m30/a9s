package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/transfer"
)

// TransferListServersAPI defines the interface for the Transfer ListServers operation.
type TransferListServersAPI interface {
	ListServers(ctx context.Context, params *transfer.ListServersInput, optFns ...func(*transfer.Options)) (*transfer.ListServersOutput, error)
}

// TransferDescribeServerAPI defines the interface for the Transfer DescribeServer operation.
type TransferDescribeServerAPI interface {
	DescribeServer(ctx context.Context, params *transfer.DescribeServerInput, optFns ...func(*transfer.Options)) (*transfer.DescribeServerOutput, error)
}

// TransferListAgreementsAPI defines the interface for the Transfer ListAgreements operation.
type TransferListAgreementsAPI interface {
	ListAgreements(ctx context.Context, params *transfer.ListAgreementsInput, optFns ...func(*transfer.Options)) (*transfer.ListAgreementsOutput, error)
}

// TransferDescribeAgreementAPI defines the interface for the Transfer DescribeAgreement operation.
type TransferDescribeAgreementAPI interface {
	DescribeAgreement(ctx context.Context, params *transfer.DescribeAgreementInput, optFns ...func(*transfer.Options)) (*transfer.DescribeAgreementOutput, error)
}

// TransferDescribeProfileAPI defines the interface for the Transfer DescribeProfile operation.
type TransferDescribeProfileAPI interface {
	DescribeProfile(ctx context.Context, params *transfer.DescribeProfileInput, optFns ...func(*transfer.Options)) (*transfer.DescribeProfileOutput, error)
}

// TransferDescribeCertificateAPI defines the interface for the Transfer DescribeCertificate operation.
type TransferDescribeCertificateAPI interface {
	DescribeCertificate(ctx context.Context, params *transfer.DescribeCertificateInput, optFns ...func(*transfer.Options)) (*transfer.DescribeCertificateOutput, error)
}

// TransferAPI is the aggregate interface covering all Transfer Family operations used by a9s fetchers.
// *transfer.Client structurally satisfies this interface.
type TransferAPI interface {
	TransferListServersAPI
	TransferDescribeServerAPI
	TransferListAgreementsAPI
	TransferDescribeAgreementAPI
	TransferDescribeProfileAPI
	TransferDescribeCertificateAPI
}
