package fakes

import (
	"context"
	"slices"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/transfer"
	transfertypes "github.com/aws/aws-sdk-go-v2/service/transfer/types"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
)

// TransferFake implements aws.TransferAPI against fixture data loaded at construction time.
type TransferFake struct {
	fix *fixtures.TransferFixtures
}

// NewTransfer constructs a TransferFake backed by fixture data from the fixtures package.
func NewTransfer() *TransferFake {
	return &TransferFake{fix: fixtures.NewTransferFixtures()}
}

// ListServers returns the fixture's listed servers, sorted for deterministic
// demo/test output. Includes servers whose DescribeServer call is denied —
// list and describe are separate IAM permissions in the real API.
func (f *TransferFake) ListServers(_ context.Context, _ *transfer.ListServersInput, _ ...func(*transfer.Options)) (*transfer.ListServersOutput, error) {
	servers := slices.Clone(f.fix.ListedServers)
	sort.Slice(servers, func(i, j int) bool {
		return aws.ToString(servers[i].ServerId) < aws.ToString(servers[j].ServerId)
	})
	return &transfer.ListServersOutput{Servers: servers}, nil
}

// DescribeServer returns the fixture's described server for the given id.
// Ids in DeniedServerIDs return AccessDeniedException — the live-witnessed
// IAM shape where a role may list servers but not read one's full details.
// Unknown ids return ResourceNotFoundException.
func (f *TransferFake) DescribeServer(_ context.Context, input *transfer.DescribeServerInput, _ ...func(*transfer.Options)) (*transfer.DescribeServerOutput, error) {
	id := aws.ToString(input.ServerId)
	if slices.Contains(f.fix.DeniedServerIDs, id) {
		return nil, &transfertypes.AccessDeniedException{
			Message: aws.String("User is not authorized to perform: transfer:DescribeServer on resource: " + id),
		}
	}
	server, ok := f.fix.Servers[id]
	if !ok {
		return nil, &transfertypes.ResourceNotFoundException{
			Message: aws.String("Server " + id + " not found"),
		}
	}
	return &transfer.DescribeServerOutput{Server: &server}, nil
}

// ListAgreements returns the fixture's listed agreements for the given server, sorted.
func (f *TransferFake) ListAgreements(_ context.Context, input *transfer.ListAgreementsInput, _ ...func(*transfer.Options)) (*transfer.ListAgreementsOutput, error) {
	serverID := aws.ToString(input.ServerId)
	agreements := slices.Clone(f.fix.ListedAgreementsByServer[serverID])
	sort.Slice(agreements, func(i, j int) bool {
		return aws.ToString(agreements[i].AgreementId) < aws.ToString(agreements[j].AgreementId)
	})
	return &transfer.ListAgreementsOutput{Agreements: agreements}, nil
}

// DescribeAgreement returns the fixture's described agreement for the given id.
// Unknown ids return ResourceNotFoundException.
func (f *TransferFake) DescribeAgreement(_ context.Context, input *transfer.DescribeAgreementInput, _ ...func(*transfer.Options)) (*transfer.DescribeAgreementOutput, error) {
	id := aws.ToString(input.AgreementId)
	agreement, ok := f.fix.Agreements[id]
	if !ok {
		return nil, &transfertypes.ResourceNotFoundException{
			Message: aws.String("Agreement " + id + " not found"),
		}
	}
	return &transfer.DescribeAgreementOutput{Agreement: &agreement}, nil
}

// DescribeProfile returns the fixture's described profile for the given id.
// Unknown ids return ResourceNotFoundException.
func (f *TransferFake) DescribeProfile(_ context.Context, input *transfer.DescribeProfileInput, _ ...func(*transfer.Options)) (*transfer.DescribeProfileOutput, error) {
	id := aws.ToString(input.ProfileId)
	profile, ok := f.fix.Profiles[id]
	if !ok {
		return nil, &transfertypes.ResourceNotFoundException{
			Message: aws.String("Profile " + id + " not found"),
		}
	}
	return &transfer.DescribeProfileOutput{Profile: &profile}, nil
}

// DescribeCertificate returns the fixture's described certificate for the given id.
// Unknown ids return ResourceNotFoundException.
func (f *TransferFake) DescribeCertificate(_ context.Context, input *transfer.DescribeCertificateInput, _ ...func(*transfer.Options)) (*transfer.DescribeCertificateOutput, error) {
	id := aws.ToString(input.CertificateId)
	cert, ok := f.fix.Certificates[id]
	if !ok {
		return nil, &transfertypes.ResourceNotFoundException{
			Message: aws.String("Certificate " + id + " not found"),
		}
	}
	return &transfer.DescribeCertificateOutput{Certificate: &cert}, nil
}
