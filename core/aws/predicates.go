// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// predicates.go holds the membership predicates a relationship is read
// through. Both directions of a pivot call the same one, so the two ends of
// a relationship cannot disagree about who belongs to it.
package aws

import (
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// imageRefersToRepo reports whether a container image URI names the ECR
// repository whose RepositoryUri is repoURI. An image URI is
// "<registry>/<repository>[:<tag>|@<digest>]", where the registry is
// "<account>.dkr.ecr.<region>.amazonaws.com" and the repository path may
// itself contain "/". Registry and repository must both be equal: "api" is
// neither "api-worker" nor "team/api", and the same repository name in
// another account or region is another repository.
func imageRefersToRepo(imageURI, repoURI string) bool {
	if repoURI == "" {
		return false
	}
	registry, repo, ok := strings.Cut(imageURI, "/")
	if !ok {
		return false
	}
	repo, _, _ = strings.Cut(repo, "@")
	if tag := strings.LastIndex(repo, ":"); tag >= 0 {
		repo = repo[:tag]
	}
	return registry+"/"+repo == repoURI
}

// routeGatewayTarget returns the id of the target of the given family
// ("igw", "nat", "tgw", "pcx") that the route sends traffic to, and "" when
// it names none. A blackhole route carries no traffic: AWS leaves the target
// id on the route after the gateway or connection is detached or deleted, so
// it names no live target. GatewayId also carries "local" and virtual
// private gateways, which are no internet gateway.
func routeGatewayTarget(route ec2types.Route, family string) string {
	if route.State == ec2types.RouteStateBlackhole {
		return ""
	}
	switch family {
	case "nat":
		return aws.ToString(route.NatGatewayId)
	case "tgw":
		return aws.ToString(route.TransitGatewayId)
	case "pcx":
		return aws.ToString(route.VpcPeeringConnectionId)
	case "igw":
		if id := aws.ToString(route.GatewayId); strings.HasPrefix(id, "igw-") {
			return id
		}
	}
	return ""
}

// routeTargetsGateway reports whether route sends traffic to targetID. The
// id's own prefix names the family, and so the field it sits in.
func routeTargetsGateway(route ec2types.Route, targetID string) bool {
	family, _, _ := strings.Cut(targetID, "-")
	return targetID != "" && routeGatewayTarget(route, family) == targetID
}

// endpointIsQueue reports whether an SNS subscription's Endpoint delivers to
// the SQS queue with queueARN / queueName. An sqs-protocol subscription's
// Endpoint is the queue's ARN, whole: one queue's ARN is the prefix of
// another's ("…:orders" of "…:orders-dlq"), so anything looser reads a
// dead-letter queue's subscription as the queue's own. The name fallback is
// anchored on the ":" that opens an ARN's last field.
func endpointIsQueue(endpoint, queueARN, queueName string) bool {
	if queueARN != "" && endpoint == queueARN {
		return true
	}
	return queueName != "" && strings.HasSuffix(endpoint, ":"+queueName)
}

// namesLaunchTemplate reports whether a launch-template specification, which
// carries an id or a name, names the template with ltID / ltName. The Auto
// Scaling and EKS APIs require either of the two on a specification, so a
// group that launches from a template may carry only its name.
func namesLaunchTemplate(specID, specName, ltID, ltName string) bool {
	return (specID != "" && specID == ltID) || (specName != "" && specName == ltName)
}

// dnsZoneHosts reports whether the hosted zone called zoneName is where the
// record called name is written: the zone's name is the record's own name,
// or its parent at a label boundary. "app.notexample.com" ends in the
// characters of "example.com" while being a name in another domain.
func dnsZoneHosts(zoneName, name string) bool {
	zoneName, name = canonicalDNS(zoneName), canonicalDNS(name)
	return zoneName != "" && (name == zoneName || strings.HasSuffix(name, "."+zoneName))
}

// isLambdaENI reports whether the ENI is a Lambda hyperplane ENI, the one
// EC2 creates for a VPC-attached function. EC2 types it "lambda"; its
// RequesterId is an account or service alias that varies, and its
// Description mentions Lambda only by convention.
func isLambdaENI(eni ec2types.NetworkInterface) bool {
	return eni.InterfaceType == ec2types.NetworkInterfaceTypeLambda
}

// efsFileSystemID matches the file system id EFS writes into a mount-target
// ENI's description, in either the documented form "Mount target fsmt-… for
// file system fs-…" or the console's "EFS mount target for fs-… (fsmt-…)".
// The id is taken as a whole token, since the mount target's own id starts
// "fsmt-" and one file system's id can be the prefix of another's; what the
// token is spelled with is not part of the rule.
var efsFileSystemID = regexp.MustCompile(`\bfs-[0-9a-z]+\b`) //nolint:gochecknoglobals // compiled once: intentional package-level var

// efsIDFromENIDescription returns the file system id a mount-target ENI's
// description names.
func efsIDFromENIDescription(desc string) (string, bool) {
	id := efsFileSystemID.FindString(desc)
	return id, id != ""
}

// eniMountsFileSystem reports whether the ENI is a mount target of the file
// system with fsID.
func eniMountsFileSystem(eni ec2types.NetworkInterface, fsID string) bool {
	id, ok := efsIDFromENIDescription(aws.ToString(eni.Description))
	return ok && id == fsID
}

// dnsAliasNames reports whether name, as written in a Route 53 alias target
// or a CloudFront origin, names host. DNS is case-insensitive, Route 53
// writes an alias target fully qualified with a trailing dot, and an alias
// to a load balancer's dual-stack name carries a "dualstack." prefix.
func dnsAliasNames(name, host string) bool {
	host = canonicalDNS(host)
	return host != "" && strings.TrimPrefix(canonicalDNS(name), "dualstack.") == host
}

// maybeELBDNS reports whether name could be a load balancer's own DNS name.
// Every one of them carries an "elb" label: before the region for a network
// or gateway load balancer, after it for an application or classic one.
func maybeELBDNS(name string) bool {
	return strings.Contains(canonicalDNS(name), ".elb.")
}
