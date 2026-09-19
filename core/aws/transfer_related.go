// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// transfer_related.go contains Transfer Family server related-resource
// checker functions. Every checker here is zero-extra-API-call: the
// fetcher's DescribeServer pass already carries every field these checkers
// read, so each one is a Pattern F (RawStruct read) — never a new AWS call,
// mirroring mwaa_related.go.
package aws

import (
	"context"

	transfertypes "github.com/aws/aws-sdk-go-v2/service/transfer/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkTransferACM reads DescribedServer.Certificate directly (Pattern F) —
// the FTPS server identity certificate; AS2 certificates are
// transfer-managed, not ACM, so this pivot is empty on any non-FTPS server.
// Degraded rows (RawStruct=ListedServer, which carries no Certificate
// field) type-assert cleanly to "not found" via assertStruct.
func checkTransferACM(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	server, ok := assertStruct[transfertypes.DescribedServer](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("acm")
	}
	if server.Certificate == nil || *server.Certificate == "" {
		return resource.KnownRelated("acm", nil, false)
	}
	return relatedResult("acm", []string{*server.Certificate})
}

// checkTransferLambda reads IdentityProviderDetails.Function directly
// (Pattern F) when IdentityProviderType == AWS_LAMBDA — the custom
// authorizer.
func checkTransferLambda(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	server, ok := assertStruct[transfertypes.DescribedServer](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("lambda")
	}
	if server.IdentityProviderType != transfertypes.IdentityProviderTypeAwsLambda ||
		server.IdentityProviderDetails == nil || server.IdentityProviderDetails.Function == nil {
		return resource.KnownRelated("lambda", nil, false)
	}
	return relatedRefs("lambda", []string{*server.IdentityProviderDetails.Function}, refContext(clients, cache, "lambda"))
}

// checkTransferLogs reads StructuredLogDestinations directly (Pattern F).
func checkTransferLogs(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	server, ok := assertStruct[transfertypes.DescribedServer](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("logs")
	}
	return relatedRefs("logs", server.StructuredLogDestinations, refContext(clients, cache, "logs"))
}

// checkTransferRole returns the role in LoggingRole (Pattern F).
func checkTransferRole(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	server, ok := assertStruct[transfertypes.DescribedServer](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("role")
	}
	if server.LoggingRole == nil || *server.LoggingRole == "" {
		return resource.KnownRelated("role", nil, false)
	}
	return relatedRefs("role", []string{*server.LoggingRole}, refContext(clients, cache, "role"))
}

// checkTransferSubnet reads EndpointDetails.SubnetIds directly (Pattern F);
// present only when EndpointType == VPC.
func checkTransferSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	server, ok := assertStruct[transfertypes.DescribedServer](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("subnet")
	}
	if server.EndpointDetails == nil || len(server.EndpointDetails.SubnetIds) == 0 {
		return resource.KnownRelated("subnet", nil, false)
	}
	return relatedResult("subnet", server.EndpointDetails.SubnetIds)
}

// checkTransferEIP reads EndpointDetails.AddressAllocationIds directly
// (Pattern F); present only when EndpointType == VPC on an internet-facing
// server. AllocationIds already match the eip type's canonical Resource.ID
// (eip.go's FetchElasticIPs sets Resource.ID = AllocationId directly).
func checkTransferEIP(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	server, ok := assertStruct[transfertypes.DescribedServer](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("eip")
	}
	if server.EndpointDetails == nil || len(server.EndpointDetails.AddressAllocationIds) == 0 {
		return resource.KnownRelated("eip", nil, false)
	}
	return relatedResult("eip", server.EndpointDetails.AddressAllocationIds)
}

// checkTransferVPC reads EndpointDetails.VpcId directly (Pattern F);
// present only when EndpointType == VPC.
func checkTransferVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	server, ok := assertStruct[transfertypes.DescribedServer](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("vpc")
	}
	if server.EndpointDetails == nil || server.EndpointDetails.VpcId == nil || *server.EndpointDetails.VpcId == "" {
		return resource.KnownRelated("vpc", nil, false)
	}
	return relatedResult("vpc", []string{*server.EndpointDetails.VpcId})
}

// checkTransferVPCE reads EndpointDetails.VpcEndpointId directly (Pattern
// F); present only when EndpointType == VPC — the hop that carries the
// security groups.
func checkTransferVPCE(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	server, ok := assertStruct[transfertypes.DescribedServer](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("vpce")
	}
	if server.EndpointDetails == nil || server.EndpointDetails.VpcEndpointId == nil || *server.EndpointDetails.VpcEndpointId == "" {
		return resource.KnownRelated("vpce", nil, false)
	}
	return relatedResult("vpce", []string{*server.EndpointDetails.VpcEndpointId})
}
