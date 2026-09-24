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
		return NotRead("acm")
	}
	if server.Certificate == nil || *server.Certificate == "" {
		return foundNone("acm", "server.Certificate")
	}
	return relatedResultTrunc("acm", []string{*server.Certificate}, false)
}

// checkTransferLambda reads IdentityProviderDetails.Function directly
// (Pattern F) when IdentityProviderType == AWS_LAMBDA — the custom
// authorizer.
func checkTransferLambda(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	server, ok := assertStruct[transfertypes.DescribedServer](res.RawStruct)
	if !ok {
		return NotRead("lambda")
	}
	if server.IdentityProviderType != transfertypes.IdentityProviderTypeAwsLambda ||
		server.IdentityProviderDetails == nil || server.IdentityProviderDetails.Function == nil {
		return foundNone("lambda", "IdentityProviderDetails.Function")
	}
	return relatedRefs("lambda", []string{*server.IdentityProviderDetails.Function}, refContext(clients, cache, "lambda"))
}

// checkTransferLogs reads StructuredLogDestinations directly (Pattern F).
func checkTransferLogs(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	server, ok := assertStruct[transfertypes.DescribedServer](res.RawStruct)
	if !ok {
		return NotRead("logs")
	}
	return relatedRefs("logs", server.StructuredLogDestinations, refContext(clients, cache, "logs"))
}

// checkTransferRole returns the role in LoggingRole (Pattern F).
func checkTransferRole(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	server, ok := assertStruct[transfertypes.DescribedServer](res.RawStruct)
	if !ok {
		return NotRead("role")
	}
	if server.LoggingRole == nil || *server.LoggingRole == "" {
		return foundNone("role", "server.LoggingRole")
	}
	return relatedRefs("role", []string{*server.LoggingRole}, refContext(clients, cache, "role"))
}

// checkTransferSubnet reads EndpointDetails.SubnetIds directly (Pattern F);
// present only when EndpointType == VPC.
func checkTransferSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	server, ok := assertStruct[transfertypes.DescribedServer](res.RawStruct)
	if !ok {
		return NotRead("subnet")
	}
	if server.EndpointDetails == nil || len(server.EndpointDetails.SubnetIds) == 0 {
		return foundNone("subnet", "server.EndpointDetails.SubnetIds")
	}
	return relatedResultTrunc("subnet", server.EndpointDetails.SubnetIds, false)
}

// checkTransferEIP reads EndpointDetails.AddressAllocationIds directly
// (Pattern F); present only when EndpointType == VPC on an internet-facing
// server. AllocationIds already match the eip type's canonical Resource.ID
// (eip.go's FetchElasticIPs sets Resource.ID = AllocationId directly).
func checkTransferEIP(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	server, ok := assertStruct[transfertypes.DescribedServer](res.RawStruct)
	if !ok {
		return NotRead("eip")
	}
	if server.EndpointDetails == nil || len(server.EndpointDetails.AddressAllocationIds) == 0 {
		return foundNone("eip", "server.EndpointDetails.AddressAllocationIds")
	}
	return relatedResultTrunc("eip", server.EndpointDetails.AddressAllocationIds, false)
}

// checkTransferVPC reads EndpointDetails.VpcId directly (Pattern F);
// present only when EndpointType == VPC.
func checkTransferVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	server, ok := assertStruct[transfertypes.DescribedServer](res.RawStruct)
	if !ok {
		return NotRead("vpc")
	}
	if server.EndpointDetails == nil || server.EndpointDetails.VpcId == nil || *server.EndpointDetails.VpcId == "" {
		return foundNone("vpc", "server.EndpointDetails.VpcId")
	}
	return relatedResultTrunc("vpc", []string{*server.EndpointDetails.VpcId}, false)
}

// checkTransferVPCE reads EndpointDetails.VpcEndpointId directly (Pattern
// F); present only when EndpointType == VPC — the hop that carries the
// security groups.
func checkTransferVPCE(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	server, ok := assertStruct[transfertypes.DescribedServer](res.RawStruct)
	if !ok {
		return NotRead("vpce")
	}
	if server.EndpointDetails == nil || server.EndpointDetails.VpcEndpointId == nil || *server.EndpointDetails.VpcEndpointId == "" {
		return foundNone("vpce", "server.EndpointDetails.VpcEndpointId")
	}
	return relatedResultTrunc("vpce", []string{*server.EndpointDetails.VpcEndpointId}, false)
}
