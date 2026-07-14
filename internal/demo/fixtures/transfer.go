// Package fixtures provides AWS Transfer Family fixture data for the Transfer fake.
package fixtures

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	transfertypes "github.com/aws/aws-sdk-go-v2/service/transfer/types"
)

// TransferFixtures holds typed fixture data for AWS Transfer Family.
type TransferFixtures struct {
	// ListedServers backs transfer:ListServers — includes every demo server,
	// including WarnTransferDetailsDeniedID. List and describe access are
	// separate IAM permissions in the real API, so a server can be listed
	// while its describe call is denied.
	ListedServers []transfertypes.ListedServer
	// Servers backs transfer:DescribeServer, keyed by ServerId. Deliberately
	// excludes WarnTransferDetailsDeniedID — DeniedServerIDs names it instead.
	Servers map[string]transfertypes.DescribedServer
	// DeniedServerIDs are present in ListedServers but DescribeServer denies
	// them — the fetcher keeps these as rich degraded rows built from the
	// list fields (transfer.warn.details_denied finding), unlike MWAA's
	// name-only degradation (MWAA's ListEnvironments carries no rich fields).
	DeniedServerIDs []string

	// ListedAgreementsByServer backs transfer:ListAgreements(ServerId), keyed
	// by ServerId — Agreements are the only server-scoped child view
	// (docs/resources/transfer.md §2.1).
	ListedAgreementsByServer map[string][]transfertypes.ListedAgreement
	// Agreements backs transfer:DescribeAgreement, keyed by AgreementId
	// (agreement ids are unique across the whole demo account).
	Agreements map[string]transfertypes.DescribedAgreement

	// Profiles backs transfer:DescribeProfile, keyed by ProfileId. Profiles
	// are account-scoped (ListedProfile/DescribedProfile carry no ServerId)
	// — deliberately not a server child; resolved inline from an agreement's
	// LocalProfileId/PartnerProfileId (docs/resources/transfer.md §2.1).
	Profiles map[string]transfertypes.DescribedProfile
	// Certificates backs transfer:DescribeCertificate, keyed by
	// CertificateId — resolved inline from a profile's CertificateIds.
	Certificates map[string]transfertypes.DescribedCertificate
}

// Exported server-id constants — referenced by sibling fixture files (ec2.go's
// VpcEndpoint entry, cwlogs.go's log groups) and by QA tests.
const (
	ProdAS2GatewayID            = "prod-as2-gateway"
	SftpUsersProdID             = "sftp-users-prod"
	SftpLambdaAuthID            = "sftp-lambda-auth"
	WarnTransferOfflineID       = "warn-transfer-offline"
	WarnTransferStartingID      = "warn-transfer-starting"
	WarnTransferStoppingID      = "warn-transfer-stopping"
	BrokenTransferStartFailedID = "broken-transfer-start-failed"
	WarnTransferStopFailedID    = "warn-transfer-stop-failed"
	WarnTransferLegacyPolicyID  = "warn-transfer-legacy-policy"
	WarnTransferNoLoggingID     = "warn-transfer-no-logging"
	WarnTransferMultiID         = "warn-transfer-multi"
	WarnTransferDetailsDeniedID = "warn-transfer-details-denied"
)

// ProdAS2GatewayVpcEndpointID is the auto-created VPC endpoint backing the
// graph root's VPC-hosted server; referenced from ec2.go's VpcEndpoint
// fixture list so the vpce related-panel pivot drills through to a real row.
const ProdAS2GatewayVpcEndpointID = "vpce-0transfer1111a"

// Agreement, profile, and certificate constants — agr-old-partner is
// INACTIVE (§4 "inactive: partner traffic rejected" child-row finding); the
// two profiles and three certificates back the agreement-detail inline
// resolution (docs/resources/transfer.md §3.2).
const (
	AgreementProdPartnerID = "agr-prod-partner"
	AgreementOldPartnerID  = "agr-old-partner"

	LocalProfileID   = "lp-1"
	PartnerProfileID = "pp-1"

	CertFreshID    = "cert-fresh"
	CertExpiringID = "cert-expiring"
	CertExpiredID  = "cert-expired"
)

const (
	transferRegion    = "us-east-1"
	transferAccountID = "123456789012"

	transferModernPolicy = "TransferSecurityPolicy-2024-01"
	transferLegacyPolicy = "TransferSecurityPolicy-2018-11"

	// transferLogGroupPrimaryName / transferLogGroupPartnerAuditName are the
	// graph root's two structured-log destinations — required so the logs
	// related-panel pivot counts ≥2 (transfer-impl-plan.md §2: subnet+logs
	// are the only list-valued pivot fields on this server; the rest are 1:1
	// by API shape, a documented structural ceiling).
	transferLogGroupPrimaryName      = "/aws/transfer/prod-as2-gateway"
	transferLogGroupPartnerAuditName = "/aws/transfer/prod-as2-gateway/partner-audit"
)

func transferServerArn(id string) string {
	return "arn:aws:transfer:" + transferRegion + ":" + transferAccountID + ":server/" + id
}

// transferLogGroupArn builds the CloudWatch log-group ARN AWS reports on
// StructuredLogDestinations (trailing ":*" wildcard, matching the documented
// example shape — mirrors mwaaLogGroupArn in mwaa.go).
func transferLogGroupArn(name string) string {
	return "arn:aws:logs:" + transferRegion + ":" + transferAccountID + ":log-group:" + name + ":*"
}

func transferAgreementArn(serverID, agreementID string) string {
	return "arn:aws:transfer:" + transferRegion + ":" + transferAccountID + ":agreement/" + serverID + "/" + agreementID
}

func transferProfileArn(profileID string) string {
	return "arn:aws:transfer:" + transferRegion + ":" + transferAccountID + ":profile/" + profileID
}

func transferCertificateArn(certID string) string {
	return "arn:aws:transfer:" + transferRegion + ":" + transferAccountID + ":certificate/" + certID
}

// transferBaseServer returns the common healthy-shape DescribedServer;
// callers override the fields that vary per fixture. Defaults model an SFTP,
// service-managed, publicly-reachable server on a current security policy
// with logging configured — the "no signal fires" baseline (§3 wave3_anti).
func transferBaseServer(id string, state transfertypes.State, hostKeyFingerprint string) transfertypes.DescribedServer {
	return transfertypes.DescribedServer{
		Arn:                  aws.String(transferServerArn(id)),
		ServerId:             aws.String(id),
		State:                state,
		Domain:               transfertypes.DomainS3,
		EndpointType:         transfertypes.EndpointTypePublic,
		IdentityProviderType: transfertypes.IdentityProviderTypeServiceManaged,
		LoggingRole:          aws.String(fixtIAMProdLambdaRoleARN),
		UserCount:            aws.Int32(0),
		SecurityPolicyName:   aws.String(transferModernPolicy),
		Protocols:            []transfertypes.Protocol{transfertypes.ProtocolSftp},
		IpAddressType:        transfertypes.IpAddressTypeIpv4,
		HostKeyFingerprint:   aws.String(hostKeyFingerprint),
	}
}

// transferListedFromDescribed projects the subset of fields ListServers
// actually returns. Real ListedServer/DescribedServer report identical
// values for every field they share, so building the listed row from the
// described one (rather than maintaining two parallel literals) keeps them
// from ever drifting apart.
func transferListedFromDescribed(d transfertypes.DescribedServer) transfertypes.ListedServer {
	return transfertypes.ListedServer{
		Arn:                  d.Arn,
		ServerId:             d.ServerId,
		State:                d.State,
		Domain:               d.Domain,
		EndpointType:         d.EndpointType,
		IdentityProviderType: d.IdentityProviderType,
		LoggingRole:          d.LoggingRole,
		UserCount:            d.UserCount,
	}
}

// withNoLogging clears both logging signals — the transfer.warn.no_logging
// finding requires LoggingRole nil AND StructuredLogDestinations empty.
func withNoLogging(s transfertypes.DescribedServer) transfertypes.DescribedServer {
	s.LoggingRole = nil
	s.StructuredLogDestinations = nil
	return s
}

func withSecurityPolicy(s transfertypes.DescribedServer, policy string) transfertypes.DescribedServer {
	s.SecurityPolicyName = aws.String(policy)
	return s
}

// sharedTransferFixtures constructs the canonical demo data once per process.
var sharedTransferFixtures = sync.OnceValue(func() *TransferFixtures {
	now := time.Now().UTC()

	servers := make(map[string]transfertypes.DescribedServer, 11)
	var listedServers []transfertypes.ListedServer
	addServer := func(s transfertypes.DescribedServer) {
		servers[aws.ToString(s.ServerId)] = s
		listedServers = append(listedServers, transferListedFromDescribed(s))
	}

	// GRAPH ROOT — AS2+FTPS gateway. Countable pivots: role 1, vpc 1,
	// subnet 3, vpce 1, logs 2, acm 1 = 6, with ≥2 on subnet+logs (2/6 —
	// VpcId/VpcEndpointId/Certificate/LoggingRole are 1:1 by API shape;
	// documented structural ceiling, transfer-impl-plan.md §2).
	gateway := transferBaseServer(ProdAS2GatewayID, transfertypes.StateOnline, "SHA256:6b1f9c9e2a4d4e8f91a37d5c8e2f4b6a70c1d2e3f4a5")
	gateway.EndpointType = transfertypes.EndpointTypeVpc
	gateway.Protocols = []transfertypes.Protocol{transfertypes.ProtocolAs2, transfertypes.ProtocolFtps}
	gateway.EndpointDetails = &transfertypes.EndpointDetails{
		VpcId:         aws.String(fixtProdVPCID),
		VpcEndpointId: aws.String(ProdAS2GatewayVpcEndpointID),
		SubnetIds:     []string{fixtProdPublicSubnetA, fixtProdPublicSubnetB, fixtProdPrivateSubnetA},
	}
	gateway.Certificate = aws.String(ProdACMCertARN1)
	gateway.StructuredLogDestinations = []string{
		transferLogGroupArn(transferLogGroupPrimaryName),
		transferLogGroupArn(transferLogGroupPartnerAuditName),
	}
	gateway.As2ServiceManagedEgressIpAddresses = []string{"44.192.42.10", "44.192.42.11", "44.192.42.12"}
	addServer(gateway)

	// PUBLIC endpoint (EndpointDetails nil, the transferBaseServer default)
	// — proves conditional pivots absent cleanly.
	sftpUsers := transferBaseServer(SftpUsersProdID, transfertypes.StateOnline, "SHA256:8b2d4f6a1c3e4a7b9d5f2e4c6a8b0d1f2a3b4c5d6e7f")
	sftpUsers.UserCount = aws.Int32(12)
	addServer(sftpUsers)

	// AWS_LAMBDA custom authorizer — reuses the lambda.go api-gateway-authorizer
	// fixture (already graph-connected by literal ARN across sfn.go/apigw.go/
	// cloudfront.go/cloudwatch.go, following the same convention here).
	lambdaAuth := transferBaseServer(SftpLambdaAuthID, transfertypes.StateOnline, "SHA256:1a2b3c4d5e6f4a7b8c9d0e1f2a3b4c5d6e7f8091a2b3")
	lambdaAuth.IdentityProviderType = transfertypes.IdentityProviderTypeAwsLambda
	lambdaAuth.IdentityProviderDetails = &transfertypes.IdentityProviderDetails{
		Function: aws.String("arn:aws:lambda:" + transferRegion + ":" + transferAccountID + ":function:api-gateway-authorizer"),
	}
	addServer(lambdaAuth)

	addServer(transferBaseServer(WarnTransferOfflineID, transfertypes.StateOffline, "SHA256:2b3c4d5e6f7a4b8c9d0e1f2a3b4c5d6e7f8091a2b3c4"))
	addServer(transferBaseServer(WarnTransferStartingID, transfertypes.StateStarting, "SHA256:3c4d5e6f7a8b4c9d0e1f2a3b4c5d6e7f8091a2b3c4d5"))
	addServer(transferBaseServer(WarnTransferStoppingID, transfertypes.StateStopping, "SHA256:4d5e6f7a8b9c4d0e1f2a3b4c5d6e7f8091a2b3c4d5e6"))
	addServer(transferBaseServer(BrokenTransferStartFailedID, transfertypes.StateStartFailed, "SHA256:5e6f7a8b9c0d4e1f2a3b4c5d6e7f8091a2b3c4d5e6f7"))
	addServer(transferBaseServer(WarnTransferStopFailedID, transfertypes.StateStopFailed, "SHA256:6f7a8b9c0d1e4f2a3b4c5d6e7f8091a2b3c4d5e6f7a8"))

	addServer(withSecurityPolicy(
		transferBaseServer(WarnTransferLegacyPolicyID, transfertypes.StateOnline, "SHA256:7a8b9c0d1e2f4a3b4c5d6e7f8091a2b3c4d5e6f7a8b9"),
		transferLegacyPolicy,
	))

	addServer(withNoLogging(
		transferBaseServer(WarnTransferNoLoggingID, transfertypes.StateOnline, "SHA256:8b9c0d1e2f3a4b4c5d6e7f8091a2b3c4d5e6f7a8b9c0"),
	))

	// OFFLINE + legacy policy + no logging — three findings stack on one row
	// (§4 "offline: not accepting transfers (+2)").
	multi := transferBaseServer(WarnTransferMultiID, transfertypes.StateOffline, "SHA256:9c0d1e2f3a4b4c5d6e7f8091a2b3c4d5e6f7a8b9c0d1")
	multi = withSecurityPolicy(multi, transferLegacyPolicy)
	multi = withNoLogging(multi)
	addServer(multi)

	// Listed but DescribeServer-denied — rich degraded row: kept via list
	// fields (State ONLINE, so only the details-denied finding fires),
	// deliberately never added to the Servers map.
	denied := transferBaseServer(WarnTransferDetailsDeniedID, transfertypes.StateOnline, "SHA256:0d1e2f3a4b4c4d5e6f708091a2b3c4d5e6f7a8b9c0d1e")
	listedServers = append(listedServers, transferListedFromDescribed(denied))

	// Agreements — server-scoped, both hang off the graph root.
	agreements := map[string]transfertypes.DescribedAgreement{
		AgreementProdPartnerID: {
			Arn:                   aws.String(transferAgreementArn(ProdAS2GatewayID, AgreementProdPartnerID)),
			AgreementId:           aws.String(AgreementProdPartnerID),
			ServerId:              aws.String(ProdAS2GatewayID),
			LocalProfileId:        aws.String(LocalProfileID),
			PartnerProfileId:      aws.String(PartnerProfileID),
			BaseDirectory:         aws.String("/" + HealthyBucketName + "/inbound"),
			AccessRole:            aws.String(fixtIAMProdLambdaRoleARN),
			Status:                transfertypes.AgreementStatusTypeActive,
			Description:           aws.String("ACME Corp AS2 partner exchange"),
			EnforceMessageSigning: transfertypes.EnforceMessageSigningTypeDisabled,
			PreserveFilename:      transfertypes.PreserveFilenameTypeDisabled,
		},
		AgreementOldPartnerID: {
			Arn:                   aws.String(transferAgreementArn(ProdAS2GatewayID, AgreementOldPartnerID)),
			AgreementId:           aws.String(AgreementOldPartnerID),
			ServerId:              aws.String(ProdAS2GatewayID),
			LocalProfileId:        aws.String(LocalProfileID),
			PartnerProfileId:      aws.String(PartnerProfileID),
			BaseDirectory:         aws.String("/" + HealthyBucketName + "/archive/old-partner"),
			AccessRole:            aws.String(fixtIAMProdLambdaRoleARN),
			Status:                transfertypes.AgreementStatusTypeInactive,
			Description:           aws.String("Decommissioned partner — traffic rejected"),
			EnforceMessageSigning: transfertypes.EnforceMessageSigningTypeDisabled,
			PreserveFilename:      transfertypes.PreserveFilenameTypeDisabled,
		},
	}

	listedAgreements := map[string][]transfertypes.ListedAgreement{
		ProdAS2GatewayID: {
			{
				AgreementId:      aws.String(AgreementProdPartnerID),
				Arn:              aws.String(transferAgreementArn(ProdAS2GatewayID, AgreementProdPartnerID)),
				Description:      aws.String("ACME Corp AS2 partner exchange"),
				LocalProfileId:   aws.String(LocalProfileID),
				PartnerProfileId: aws.String(PartnerProfileID),
				ServerId:         aws.String(ProdAS2GatewayID),
				Status:           transfertypes.AgreementStatusTypeActive,
			},
			{
				AgreementId:      aws.String(AgreementOldPartnerID),
				Arn:              aws.String(transferAgreementArn(ProdAS2GatewayID, AgreementOldPartnerID)),
				Description:      aws.String("Decommissioned partner — traffic rejected"),
				LocalProfileId:   aws.String(LocalProfileID),
				PartnerProfileId: aws.String(PartnerProfileID),
				ServerId:         aws.String(ProdAS2GatewayID),
				Status:           transfertypes.AgreementStatusTypeInactive,
			},
		},
	}

	// Profiles — account-scoped; certificates are split across both so the
	// agreement detail's inline profile/cert resolution witnesses all three.
	profiles := map[string]transfertypes.DescribedProfile{
		LocalProfileID: {
			Arn:            aws.String(transferProfileArn(LocalProfileID)),
			ProfileId:      aws.String(LocalProfileID),
			As2Id:          aws.String("ACME-LOCAL"),
			ProfileType:    transfertypes.ProfileTypeLocal,
			CertificateIds: []string{CertFreshID, CertExpiringID},
		},
		PartnerProfileID: {
			Arn:            aws.String(transferProfileArn(PartnerProfileID)),
			ProfileId:      aws.String(PartnerProfileID),
			As2Id:          aws.String("PARTNER-CO"),
			ProfileType:    transfertypes.ProfileTypePartner,
			CertificateIds: []string{CertExpiredID},
		},
	}

	// Certificates — InactiveDate anchored relative to time.Now() so the
	// expiry findings (Broken "expired" / Warning "expires in <N>d") stay
	// witnessable deterministically regardless of when the demo/tests run
	// (mirrors dbi-snap.go/redshift.go's now-relative date pattern).
	certificates := map[string]transfertypes.DescribedCertificate{
		CertFreshID: {
			Arn:           aws.String(transferCertificateArn(CertFreshID)),
			CertificateId: aws.String(CertFreshID),
			Description:   aws.String("ACME Corp AS2 signing certificate"),
			Status:        transfertypes.CertificateStatusTypeActive,
			Type:          transfertypes.CertificateTypeCertificate,
			Usage:         transfertypes.CertificateUsageTypeSigning,
			NotBeforeDate: aws.Time(now.Add(-365 * 24 * time.Hour)),
			InactiveDate:  aws.Time(now.Add(2 * 365 * 24 * time.Hour)),
		},
		CertExpiringID: {
			Arn:           aws.String(transferCertificateArn(CertExpiringID)),
			CertificateId: aws.String(CertExpiringID),
			Description:   aws.String("ACME Corp AS2 encryption certificate"),
			Status:        transfertypes.CertificateStatusTypeActive,
			Type:          transfertypes.CertificateTypeCertificate,
			Usage:         transfertypes.CertificateUsageTypeEncryption,
			NotBeforeDate: aws.Time(now.Add(-300 * 24 * time.Hour)),
			InactiveDate:  aws.Time(now.Add(20 * 24 * time.Hour)),
		},
		CertExpiredID: {
			Arn:           aws.String(transferCertificateArn(CertExpiredID)),
			CertificateId: aws.String(CertExpiredID),
			Description:   aws.String("Partner Co AS2 TLS certificate"),
			Status:        transfertypes.CertificateStatusTypeInactive,
			Type:          transfertypes.CertificateTypeCertificate,
			Usage:         transfertypes.CertificateUsageTypeTls,
			NotBeforeDate: aws.Time(now.Add(-400 * 24 * time.Hour)),
			InactiveDate:  aws.Time(now.Add(-10 * 24 * time.Hour)),
		},
	}

	return &TransferFixtures{
		ListedServers:            listedServers,
		Servers:                  servers,
		DeniedServerIDs:          []string{WarnTransferDetailsDeniedID},
		ListedAgreementsByServer: listedAgreements,
		Agreements:               agreements,
		Profiles:                 profiles,
		Certificates:             certificates,
	}
})

// NewTransferFixtures constructs TransferFixtures from the canonical demo data.
func NewTransferFixtures() *TransferFixtures {
	return sharedTransferFixtures()
}
