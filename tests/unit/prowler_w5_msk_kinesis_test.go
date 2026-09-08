package unit

// prowler_w5_msk_kinesis_test.go — batch w5 rows 5-8: the two MSK exposure
// signals and the two Kinesis stream signals.
//
// Rows 7 and 8 are wave 2, not wave 1 as the spec table first had them:
// ListStreams returns kinesistypes.StreamSummary, which carries neither
// EncryptionType nor RetentionPeriodHours. Both live on
// StreamDescriptionSummary, which only DescribeStreamSummary returns, so the
// tests below drive the enricher rather than the fetcher.

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	kafkasvc "github.com/aws/aws-sdk-go-v2/service/kafka"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"
	kinesissvc "github.com/aws/aws-sdk-go-v2/service/kinesis"
	kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ---------------------------------------------------------------------------
// Rows 5, 6 — msk.public-access, msk.unauthenticated
// ---------------------------------------------------------------------------

// w5MSKFake serves DescribeClusterV2 per cluster ARN.
type w5MSKFake struct {
	awsclient.MSKAPI
	clusters map[string]*kafkatypes.Cluster
	errByARN map[string]error
}

func newW5MSKFake() *w5MSKFake {
	return &w5MSKFake{
		clusters: map[string]*kafkatypes.Cluster{},
		errByARN: map[string]error{},
	}
}

func (f *w5MSKFake) DescribeClusterV2(_ context.Context, in *kafkasvc.DescribeClusterV2Input, _ ...func(*kafkasvc.Options)) (*kafkasvc.DescribeClusterV2Output, error) {
	arn := ""
	if in != nil && in.ClusterArn != nil {
		arn = *in.ClusterArn
	}
	if err, ok := f.errByARN[arn]; ok {
		return nil, err
	}
	return &kafkasvc.DescribeClusterV2Output{ClusterInfo: f.clusters[arn]}, nil
}

var _ awsclient.MSKAPI = (*w5MSKFake)(nil)

func w5ClusterARN(name string) string {
	return "arn:aws:kafka:us-east-1:123456789012:cluster/" + name + "/1234abcd-12ab-34cd-56ef-1234567890ab-1"
}

// w5MSKRes mirrors the msk fetcher: ID is the cluster name, the ARN the
// enricher needs to call DescribeClusterV2 lives in Fields["cluster_arn"].
func w5MSKRes(name string) resource.Resource {
	return resource.Resource{
		ID:     name,
		Name:   name,
		Fields: map[string]string{"cluster_name": name, "cluster_arn": w5ClusterARN(name), "state": "ACTIVE"},
	}
}

// w5ProvisionedCluster is a realistic DescribeClusterV2 body for a healthy
// provisioned cluster: current broker software, TLS to clients, public
// access off, unauthenticated access off. Each test flips exactly one field,
// so a finding that fires can only have come from the field under test.
func w5ProvisionedCluster(name string) *kafkatypes.Cluster {
	return &kafkatypes.Cluster{
		ClusterName: aws.String(name),
		ClusterArn:  aws.String(w5ClusterARN(name)),
		ClusterType: kafkatypes.ClusterTypeProvisioned,
		State:       kafkatypes.ClusterStateActive,
		Provisioned: &kafkatypes.Provisioned{
			NumberOfBrokerNodes:       aws.Int32(3),
			CurrentBrokerSoftwareInfo: &kafkatypes.BrokerSoftwareInfo{KafkaVersion: aws.String("3.6.0")},
			EncryptionInfo: &kafkatypes.EncryptionInfo{
				EncryptionInTransit: &kafkatypes.EncryptionInTransit{
					ClientBroker: kafkatypes.ClientBrokerTls,
					InCluster:    aws.Bool(true),
				},
			},
			BrokerNodeGroupInfo: &kafkatypes.BrokerNodeGroupInfo{
				InstanceType: aws.String("kafka.m5.large"),
				ClientSubnets: []string{
					"subnet-0a1b2c3d4e5f60001",
					"subnet-0a1b2c3d4e5f60002",
				},
				ConnectivityInfo: &kafkatypes.ConnectivityInfo{
					PublicAccess: &kafkatypes.PublicAccess{Type: aws.String("DISABLED")},
				},
			},
			ClientAuthentication: &kafkatypes.ClientAuthentication{
				Sasl:            &kafkatypes.Sasl{Iam: &kafkatypes.Iam{Enabled: aws.Bool(true)}},
				Unauthenticated: &kafkatypes.Unauthenticated{Enabled: aws.Bool(false)},
			},
		},
	}
}

func w5EnrichMSK(t *testing.T, f *w5MSKFake, rows ...resource.Resource) awsclient.IssueEnricherResult {
	t.Helper()
	res, err := awsclient.EnrichMSKCluster(
		context.Background(),
		&awsclient.ServiceClients{MSK: f},
		rows,
		nil,
	)
	w2AssertEnricherInvariants(t, res, err)
	return res
}

// Brokers published with service-provided elastic IPs are reachable from the
// public internet, not only from inside the VPC.
func TestW5_MSKPublicAccess_Positive(t *testing.T) {
	name := "acme-events-public"
	f := newW5MSKFake()
	c := w5ProvisionedCluster(name)
	c.Provisioned.BrokerNodeGroupInfo.ConnectivityInfo.PublicAccess.Type = aws.String("SERVICE_PROVIDED_EIPS")
	f.clusters[w5ClusterARN(name)] = c

	res := w5EnrichMSK(t, f, w5MSKRes(name))
	w2AssertFinding(t, res.Findings[name], "msk.public-access",
		"brokers reachable from the internet", domain.SevBroken, "wave2")
}

func TestW5_MSKPublicAccess_DisabledIsHealthy(t *testing.T) {
	name := "acme-events"
	f := newW5MSKFake()
	f.clusters[w5ClusterARN(name)] = w5ProvisionedCluster(name)

	res := w5EnrichMSK(t, f, w5MSKRes(name))
	w2AssertNoCode(t, res.Findings[name], "msk.public-access")
}

// A cluster that reports no connectivity block at all says nothing about
// public access. Unknown is not misconfigured, and each nil step on the way
// down is its own chance to panic or to guess.
func TestW5_MSKPublicAccess_NilConnectivityIsNotAFinding(t *testing.T) {
	tests := []struct {
		name string
		mut  func(*kafkatypes.Cluster)
	}{
		{"nil broker node group", func(c *kafkatypes.Cluster) { c.Provisioned.BrokerNodeGroupInfo = nil }},
		{"nil connectivity info", func(c *kafkatypes.Cluster) { c.Provisioned.BrokerNodeGroupInfo.ConnectivityInfo = nil }},
		{"nil public access", func(c *kafkatypes.Cluster) {
			c.Provisioned.BrokerNodeGroupInfo.ConnectivityInfo.PublicAccess = nil
		}},
		{"nil public access type", func(c *kafkatypes.Cluster) {
			c.Provisioned.BrokerNodeGroupInfo.ConnectivityInfo.PublicAccess.Type = nil
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cluster := "acme-events-partial"
			f := newW5MSKFake()
			c := w5ProvisionedCluster(cluster)
			tc.mut(c)
			f.clusters[w5ClusterARN(cluster)] = c

			res := w5EnrichMSK(t, f, w5MSKRes(cluster))
			w2AssertNoCode(t, res.Findings[cluster], "msk.public-access")
		})
	}
}

// A cluster accepting clients that present no credentials is open to anyone
// who can route to a broker.
func TestW5_MSKUnauthenticated_Positive(t *testing.T) {
	name := "acme-events-open"
	f := newW5MSKFake()
	c := w5ProvisionedCluster(name)
	c.Provisioned.ClientAuthentication.Unauthenticated.Enabled = aws.Bool(true)
	f.clusters[w5ClusterARN(name)] = c

	res := w5EnrichMSK(t, f, w5MSKRes(name))
	w2AssertFinding(t, res.Findings[name], "msk.unauthenticated",
		"unauthenticated access allowed", domain.SevBroken, "wave2")
}

func TestW5_MSKUnauthenticated_DisabledIsHealthy(t *testing.T) {
	name := "acme-events"
	f := newW5MSKFake()
	f.clusters[w5ClusterARN(name)] = w5ProvisionedCluster(name)

	res := w5EnrichMSK(t, f, w5MSKRes(name))
	w2AssertNoCode(t, res.Findings[name], "msk.unauthenticated")
}

// An absent Enabled pointer is unknown. AWS omits it on clusters that
// predate the setting, and a nil read as "true" would flag every one of them.
func TestW5_MSKUnauthenticated_NilIsNotAFinding(t *testing.T) {
	tests := []struct {
		name string
		mut  func(*kafkatypes.Cluster)
	}{
		{"nil client authentication", func(c *kafkatypes.Cluster) { c.Provisioned.ClientAuthentication = nil }},
		{"nil unauthenticated block", func(c *kafkatypes.Cluster) { c.Provisioned.ClientAuthentication.Unauthenticated = nil }},
		{"nil enabled flag", func(c *kafkatypes.Cluster) {
			c.Provisioned.ClientAuthentication.Unauthenticated.Enabled = nil
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cluster := "acme-events-partial"
			f := newW5MSKFake()
			c := w5ProvisionedCluster(cluster)
			tc.mut(c)
			f.clusters[w5ClusterARN(cluster)] = c

			res := w5EnrichMSK(t, f, w5MSKRes(cluster))
			w2AssertNoCode(t, res.Findings[cluster], "msk.unauthenticated")
		})
	}
}

// Rule 4: a cluster that is both published and unauthenticated reports both,
// each with its own code, rather than the first one the enricher happened to
// evaluate.
func TestW5_MSKPublicAndUnauthenticated_AreTwoFindings(t *testing.T) {
	name := "acme-events-wide-open"
	f := newW5MSKFake()
	c := w5ProvisionedCluster(name)
	c.Provisioned.BrokerNodeGroupInfo.ConnectivityInfo.PublicAccess.Type = aws.String("SERVICE_PROVIDED_EIPS")
	c.Provisioned.ClientAuthentication.Unauthenticated.Enabled = aws.Bool(true)
	f.clusters[w5ClusterARN(name)] = c

	res := w5EnrichMSK(t, f, w5MSKRes(name))
	w2AssertFinding(t, res.Findings[name], "msk.public-access",
		"brokers reachable from the internet", domain.SevBroken, "wave2")
	w2AssertFinding(t, res.Findings[name], "msk.unauthenticated",
		"unauthenticated access allowed", domain.SevBroken, "wave2")
}

// Serverless clusters carry no Provisioned block and neither setting exists
// on them. They must be skipped, not reported.
func TestW5_MSKServerlessClusterEmitsNeitherFinding(t *testing.T) {
	name := "acme-events-serverless"
	f := newW5MSKFake()
	f.clusters[w5ClusterARN(name)] = &kafkatypes.Cluster{
		ClusterName: aws.String(name),
		ClusterArn:  aws.String(w5ClusterARN(name)),
		ClusterType: kafkatypes.ClusterTypeServerless,
		State:       kafkatypes.ClusterStateActive,
		Serverless:  &kafkatypes.Serverless{},
	}

	res := w5EnrichMSK(t, f, w5MSKRes(name))
	w2AssertNoCode(t, res.Findings[name], "msk.public-access")
	w2AssertNoCode(t, res.Findings[name], "msk.unauthenticated")
}

// One cluster's DescribeClusterV2 failing marks that cluster unknown and
// leaves the rest of the batch evaluated.
func TestW5_MSK_APIErrorOnOneClusterTruncatesOnlyThatCluster(t *testing.T) {
	broken, open := "acme-broken", "acme-events-open"
	f := newW5MSKFake()
	f.errByARN[w5ClusterARN(broken)] = errors.New("ForbiddenException: not authorized to perform kafka:DescribeClusterV2")
	c := w5ProvisionedCluster(open)
	c.Provisioned.ClientAuthentication.Unauthenticated.Enabled = aws.Bool(true)
	f.clusters[w5ClusterARN(open)] = c

	res, _ := awsclient.EnrichMSKCluster(
		context.Background(),
		&awsclient.ServiceClients{MSK: f},
		[]resource.Resource{w5MSKRes(broken), w5MSKRes(open)},
		nil,
	)
	w2AssertEnricherShape(t, res)

	if !res.TruncatedIDs[broken] {
		t.Errorf("TruncatedIDs[%q] = false; a cluster that could not be described is unknown, not healthy", broken)
	}
	if len(res.Findings[broken]) != 0 {
		t.Errorf("failed cluster %q carries findings %v", broken, w2Codes(res.Findings[broken]))
	}
	w2AssertFinding(t, res.Findings[open], "msk.unauthenticated",
		"unauthenticated access allowed", domain.SevBroken, "wave2")
}

func TestW5_MSK_NilClientReturnsEmptyResult(t *testing.T) {
	res, err := awsclient.EnrichMSKCluster(
		context.Background(),
		&awsclient.ServiceClients{},
		[]resource.Resource{w5MSKRes("acme-events")},
		nil,
	)
	w2AssertEnricherInvariants(t, res, err)
	if len(res.Findings) != 0 {
		t.Errorf("Findings = %v on a session with no MSK client", res.Findings)
	}
}

func TestW5_MSKCatalogDefs(t *testing.T) {
	w2AssertFindingDef(t, "msk", "msk.public-access",
		"brokers reachable from the internet", domain.SevBroken, "wave2")
	w2AssertFindingDef(t, "msk", "msk.unauthenticated",
		"unauthenticated access allowed", domain.SevBroken, "wave2")
}

// ---------------------------------------------------------------------------
// Rows 7, 8 — kinesis.unencrypted, kinesis.min-retention
// ---------------------------------------------------------------------------

// w5KinesisFake serves DescribeStreamSummary per stream name.
type w5KinesisFake struct {
	awsclient.KinesisAPI
	summaries map[string]*kinesistypes.StreamDescriptionSummary
	errByName map[string]error
}

func newW5KinesisFake() *w5KinesisFake {
	return &w5KinesisFake{
		summaries: map[string]*kinesistypes.StreamDescriptionSummary{},
		errByName: map[string]error{},
	}
}

func (f *w5KinesisFake) DescribeStreamSummary(_ context.Context, in *kinesissvc.DescribeStreamSummaryInput, _ ...func(*kinesissvc.Options)) (*kinesissvc.DescribeStreamSummaryOutput, error) {
	name := ""
	switch {
	case in != nil && in.StreamName != nil:
		name = *in.StreamName
	case in != nil && in.StreamARN != nil:
		name = w5StreamNameFromARN(*in.StreamARN)
	}
	if err, ok := f.errByName[name]; ok {
		return nil, err
	}
	return &kinesissvc.DescribeStreamSummaryOutput{StreamDescriptionSummary: f.summaries[name]}, nil
}

var _ awsclient.KinesisAPI = (*w5KinesisFake)(nil)

func w5StreamARN(name string) string {
	return "arn:aws:kinesis:us-east-1:123456789012:stream/" + name
}

// w5StreamNameFromARN lets the fake answer whether the enricher addresses a
// stream by name or by ARN — both are valid DescribeStreamSummary inputs and
// the row's posture must not depend on which one it picked.
func w5StreamNameFromARN(arn string) string {
	for i := len(arn) - 1; i >= 0; i-- {
		if arn[i] == '/' {
			return arn[i+1:]
		}
	}
	return arn
}

// w5KinesisRes mirrors the kinesis fetcher: ID and Fields["stream_name"] are
// the bare name, Fields["stream_arn"] the ARN.
func w5KinesisRes(name string) resource.Resource {
	return resource.Resource{
		ID:   name,
		Name: name,
		Fields: map[string]string{
			"stream_name":   name,
			"stream_arn":    w5StreamARN(name),
			"stream_status": "ACTIVE",
			"stream_mode":   "ON_DEMAND",
		},
	}
}

// w5StreamSummary is the healthy default: KMS encrypted, a week of
// retention. Each test flips one field.
func w5StreamSummary(name string) *kinesistypes.StreamDescriptionSummary {
	return &kinesistypes.StreamDescriptionSummary{
		StreamName:              aws.String(name),
		StreamARN:               aws.String(w5StreamARN(name)),
		StreamStatus:            kinesistypes.StreamStatusActive,
		EncryptionType:          kinesistypes.EncryptionTypeKms,
		KeyId:                   aws.String("alias/aws/kinesis"),
		RetentionPeriodHours:    aws.Int32(168),
		OpenShardCount:          aws.Int32(2),
		StreamModeDetails:       &kinesistypes.StreamModeDetails{StreamMode: kinesistypes.StreamModeOnDemand},
		EnhancedMonitoring:      []kinesistypes.EnhancedMetrics{},
		ConsumerCount:           aws.Int32(1),
		StreamCreationTimestamp: nil,
	}
}

func w5EnrichKinesis(t *testing.T, f *w5KinesisFake, rows ...resource.Resource) awsclient.IssueEnricherResult {
	t.Helper()
	res, err := awsclient.EnrichKinesisStreamSummary(
		context.Background(),
		&awsclient.ServiceClients{Kinesis: f},
		rows,
		nil,
	)
	w2AssertEnricherInvariants(t, res, err)
	return res
}

// A stream with EncryptionType NONE stores every record in clear text. AWS
// also reports an unencrypted stream with the field absent, which is the
// same fact, so both must fire.
func TestW5_KinesisUnencrypted_Positive(t *testing.T) {
	tests := []struct {
		name           string
		encryptionType kinesistypes.EncryptionType
	}{
		{"explicit none", kinesistypes.EncryptionTypeNone},
		{"absent encryption type", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stream := "acme-clickstream-plain"
			f := newW5KinesisFake()
			s := w5StreamSummary(stream)
			s.EncryptionType = tc.encryptionType
			s.KeyId = nil
			f.summaries[stream] = s

			res := w5EnrichKinesis(t, f, w5KinesisRes(stream))
			w2AssertFinding(t, res.Findings[stream], "kinesis.unencrypted",
				"not encrypted at rest", domain.SevWarn, "wave2")

			rows := w2Rows(t, res, stream, "kinesis.unencrypted")
			var got []string
			for _, r := range rows {
				got = append(got, r.Label+": "+r.Value)
				if r.Value == "none" {
					return
				}
			}
			t.Errorf("no supporting row valued %q; got %v", "none", got)
		})
	}
}

func TestW5_KinesisUnencrypted_KMSEncryptedIsHealthy(t *testing.T) {
	stream := "acme-clickstream"
	f := newW5KinesisFake()
	f.summaries[stream] = w5StreamSummary(stream)

	res := w5EnrichKinesis(t, f, w5KinesisRes(stream))
	w2AssertNoCode(t, res.Findings[stream], "kinesis.unencrypted")
}

// The default retention is 24 hours: a stream left there loses every record
// a day after it arrives, so a consumer outage over a weekend is data loss.
// Anything longer than the default is a deliberate choice and stays silent.
func TestW5_KinesisMinRetention_Boundary(t *testing.T) {
	tests := []struct {
		name  string
		hours *int32
		want  bool
	}{
		{"the 24-hour default", aws.Int32(24), true},
		{"below the default", aws.Int32(12), true},
		{"one hour above the default", aws.Int32(25), false},
		{"a week", aws.Int32(168), false},
		{"absent", nil, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stream := "acme-clickstream-retention"
			f := newW5KinesisFake()
			s := w5StreamSummary(stream)
			s.RetentionPeriodHours = tc.hours
			f.summaries[stream] = s

			res := w5EnrichKinesis(t, f, w5KinesisRes(stream))
			if !tc.want {
				w2AssertNoCode(t, res.Findings[stream], "kinesis.min-retention")
				return
			}
			w2AssertFinding(t, res.Findings[stream], "kinesis.min-retention",
				"24h retention", domain.SevWarn, "wave2")
		})
	}
}

// The supporting row carries the stream's actual retention, which is the one
// number the phrase's fixed "24h" cannot express for a stream set lower.
func TestW5_KinesisMinRetention_RowCarriesTheActualHours(t *testing.T) {
	stream := "acme-clickstream-12h"
	f := newW5KinesisFake()
	s := w5StreamSummary(stream)
	s.RetentionPeriodHours = aws.Int32(12)
	f.summaries[stream] = s

	res := w5EnrichKinesis(t, f, w5KinesisRes(stream))
	rows := w2Rows(t, res, stream, "kinesis.min-retention")
	var got []string
	for _, r := range rows {
		got = append(got, r.Label+": "+r.Value)
		if r.Value == "12" || r.Value == "12h" {
			return
		}
	}
	t.Errorf("no supporting row carrying the stream's own 12-hour retention; got %v", got)
}

// Rule 4: an unencrypted stream still on the default retention reports both.
func TestW5_KinesisUnencryptedAndMinRetention_AreTwoFindings(t *testing.T) {
	stream := "acme-clickstream-bare"
	f := newW5KinesisFake()
	s := w5StreamSummary(stream)
	s.EncryptionType = kinesistypes.EncryptionTypeNone
	s.KeyId = nil
	s.RetentionPeriodHours = aws.Int32(24)
	f.summaries[stream] = s

	res := w5EnrichKinesis(t, f, w5KinesisRes(stream))
	w2AssertFinding(t, res.Findings[stream], "kinesis.unencrypted",
		"not encrypted at rest", domain.SevWarn, "wave2")
	w2AssertFinding(t, res.Findings[stream], "kinesis.min-retention",
		"24h retention", domain.SevWarn, "wave2")
}

// A stream being torn down is not a posture problem; rule 4 keeps deleting
// resources out of the findings entirely.
func TestW5_KinesisDeletingStreamEmitsNoPostureFinding(t *testing.T) {
	stream := "acme-clickstream-deleting"
	f := newW5KinesisFake()
	s := w5StreamSummary(stream)
	s.StreamStatus = kinesistypes.StreamStatusDeleting
	s.EncryptionType = kinesistypes.EncryptionTypeNone
	s.RetentionPeriodHours = aws.Int32(24)
	f.summaries[stream] = s

	r := w5KinesisRes(stream)
	r.Fields["stream_status"] = "DELETING"

	res := w5EnrichKinesis(t, f, r)
	w2AssertNoCode(t, res.Findings[stream], "kinesis.unencrypted")
	w2AssertNoCode(t, res.Findings[stream], "kinesis.min-retention")
}

// A stream whose own DescribeStreamSummary fails is unknown for both rows.
func TestW5_Kinesis_APIErrorOnOneStreamTruncatesOnlyThatStream(t *testing.T) {
	broken, plain := "acme-broken", "acme-clickstream-plain"
	f := newW5KinesisFake()
	f.errByName[broken] = errors.New("AccessDeniedException: not authorized to perform kinesis:DescribeStreamSummary")
	s := w5StreamSummary(plain)
	s.EncryptionType = kinesistypes.EncryptionTypeNone
	f.summaries[plain] = s

	res, _ := awsclient.EnrichKinesisStreamSummary(
		context.Background(),
		&awsclient.ServiceClients{Kinesis: f},
		[]resource.Resource{w5KinesisRes(broken), w5KinesisRes(plain)},
		nil,
	)
	w2AssertEnricherShape(t, res)

	if !res.TruncatedIDs[broken] {
		t.Errorf("TruncatedIDs[%q] = false; a stream that could not be described is unknown, not healthy", broken)
	}
	w2AssertNoCode(t, res.Findings[broken], "kinesis.unencrypted")
	w2AssertNoCode(t, res.Findings[broken], "kinesis.min-retention")

	if res.TruncatedIDs[plain] {
		t.Errorf("TruncatedIDs[%q] = true; one stream's failure must not mark the others unknown", plain)
	}
	w2AssertFinding(t, res.Findings[plain], "kinesis.unencrypted",
		"not encrypted at rest", domain.SevWarn, "wave2")
}

// A stream AWS reports as gone between the list and the describe is not a
// failure — the row is unknown, and the batch is not marked failed for it.
func TestW5_Kinesis_NotFoundMarksTruncatedWithoutFailing(t *testing.T) {
	missing, plain := "acme-vanished", "acme-clickstream-plain"
	f := newW5KinesisFake()
	// The typed exception AWS actually returns, not an errors.New whose text
	// happens to spell the code. A fake that injects the string leaves the
	// typed errors.As arm untested and lets a text-match fallback look load
	// bearing when it is only propping up the fake.
	f.errByName[missing] = &kinesistypes.ResourceNotFoundException{
		Message: aws.String("Stream acme-vanished under account 123456789012 not found"),
	}
	s := w5StreamSummary(plain)
	s.EncryptionType = kinesistypes.EncryptionTypeNone
	f.summaries[plain] = s

	res, err := awsclient.EnrichKinesisStreamSummary(
		context.Background(),
		&awsclient.ServiceClients{Kinesis: f},
		[]resource.Resource{w5KinesisRes(missing), w5KinesisRes(plain)},
		nil,
	)
	w2AssertEnricherShape(t, res)

	if !res.TruncatedIDs[missing] {
		t.Errorf("TruncatedIDs[%q] = false; a stream that no longer exists is unknown", missing)
	}
	if err != nil {
		t.Errorf("enricher returned %v; a stream deleted between the list and the describe is an expected race, not a batch failure", err)
	}
	w2AssertFinding(t, res.Findings[plain], "kinesis.unencrypted",
		"not encrypted at rest", domain.SevWarn, "wave2")
}

func TestW5_Kinesis_CapPlusOne(t *testing.T) {
	f := newW5KinesisFake()
	var rows []resource.Resource
	for i := range awsclient.EnrichmentCap + 1 {
		name := fmt.Sprintf("acme-stream-%03d", i)
		s := w5StreamSummary(name)
		s.EncryptionType = kinesistypes.EncryptionTypeNone
		f.summaries[name] = s
		rows = append(rows, w5KinesisRes(name))
	}

	res := w5EnrichKinesis(t, f, rows...)
	if len(res.Findings) > awsclient.EnrichmentCap {
		t.Errorf("%d streams carry findings with a cap of %d; the enricher inspected past its own bound",
			len(res.Findings), awsclient.EnrichmentCap)
	}
	if len(res.Findings) == 0 {
		t.Error("no stream carries a finding; the cap must bound the work, not eliminate it")
	}
}

func TestW5_Kinesis_NilClientReturnsEmptyResult(t *testing.T) {
	res, err := awsclient.EnrichKinesisStreamSummary(
		context.Background(),
		&awsclient.ServiceClients{},
		[]resource.Resource{w5KinesisRes("acme-clickstream")},
		nil,
	)
	w2AssertEnricherInvariants(t, res, err)
	if len(res.Findings) != 0 {
		t.Errorf("Findings = %v on a session with no Kinesis client", res.Findings)
	}
}

// The type gained a Wave2 registration it never had. Without it the enricher
// exists but nothing in the app ever calls it.
func TestW5_KinesisWave2EnricherIsRegistered(t *testing.T) {
	if e := w2Enricher(t, "kinesis"); e == nil {
		t.Fatal("kinesis has no Wave2 enricher registered on its catalog literal")
	}
}

func TestW5_KinesisCatalogDefs(t *testing.T) {
	w2AssertFindingDef(t, "kinesis", "kinesis.unencrypted",
		"not encrypted at rest", domain.SevWarn, "wave2")
	w2AssertFindingDef(t, "kinesis", "kinesis.min-retention",
		"24h retention", domain.SevWarn, "wave2")
}

// Rule 4: a cluster being torn down, or already failed, carries no
// reachability finding. No demo cluster in either state supplies the
// connectivity or client-authentication blocks these rows read, so the
// cluster below is hand-built to trip BOTH at once — otherwise the guard
// would look proven by a fixture that never reached the code it guards.
func TestW5_MSKLifecycleEndedEmitsNoReachabilityFinding(t *testing.T) {
	for _, state := range []string{"DELETING", "FAILED"} {
		t.Run(state, func(t *testing.T) {
			name := "acme-events-going-away"
			f := newW5MSKFake()
			c := w5ProvisionedCluster(name)
			c.Provisioned.BrokerNodeGroupInfo.ConnectivityInfo.PublicAccess.Type = aws.String("SERVICE_PROVIDED_EIPS")
			c.Provisioned.ClientAuthentication.Unauthenticated.Enabled = aws.Bool(true)
			f.clusters[w5ClusterARN(name)] = c

			r := w5MSKRes(name)
			r.Fields["state"] = state

			res := w5EnrichMSK(t, f, r)
			w2AssertNoCode(t, res.Findings[name], "msk.public-access")
			w2AssertNoCode(t, res.Findings[name], "msk.unauthenticated")
		})
	}
}

// The guard sits BELOW the broker-version and encryption-in-transit checks
// on purpose: those describe the software the cluster is running, which
// stays true while it drains, and they predate this batch. A guard hoisted
// to the top of the loop would silence them too, and nothing else would say
// so — the demo fixtures do not put a draining cluster on old brokers.
func TestW5_MSKLifecycleEndedStillReportsSoftwareFindings(t *testing.T) {
	name := "acme-events-draining"
	f := newW5MSKFake()
	c := w5ProvisionedCluster(name)
	c.Provisioned.CurrentBrokerSoftwareInfo.KafkaVersion = aws.String("2.6.1")
	c.Provisioned.EncryptionInfo.EncryptionInTransit.ClientBroker = kafkatypes.ClientBrokerTlsPlaintext
	f.clusters[w5ClusterARN(name)] = c

	r := w5MSKRes(name)
	r.Fields["state"] = "DELETING"

	res := w5EnrichMSK(t, f, r)

	// Asserted by code and phrase rather than through w2AssertFinding: these
	// two codes predate this batch and carry no Detail sentence, and the
	// shared helper enforces one because that is this batch's contract for
	// its own fifteen rows. What this test is about is that the guard did
	// not reach up and silence them.
	for _, want := range []struct{ code, phrase string }{
		{"msk.broker-outdated", "broker software outdated"},
		{"msk.encryption-not-tls", "encryption in transit not enforced"},
	} {
		got, ok := w2Find(res.Findings[name], want.code)
		if !ok {
			t.Errorf("no finding %q on a draining cluster; the guard sits below the software checks and must not silence them. got %v",
				want.code, w2Codes(res.Findings[name]))
			continue
		}
		if got.Phrase != want.phrase {
			t.Errorf("%s: Phrase = %q, want %q", want.code, got.Phrase, want.phrase)
		}
		if got.Severity != domain.SevWarn {
			t.Errorf("%s: Severity = %v, want %v", want.code, got.Severity, domain.SevWarn)
		}
	}
}

// The twin that keeps the guard narrow. A cluster mid-maintenance or
// rebooting is still serving traffic, so public brokers and unauthenticated
// access are exactly what an operator wants reported. Widening the guard to
// "not ACTIVE" would pass the test above and fail here.
func TestW5_MSKDegradedClusterStillReportsReachability(t *testing.T) {
	for _, state := range []string{"ACTIVE", "MAINTENANCE", "REBOOTING_BROKER", "HEALING", "UPDATING"} {
		t.Run(state, func(t *testing.T) {
			name := "acme-events-busy"
			f := newW5MSKFake()
			c := w5ProvisionedCluster(name)
			c.Provisioned.BrokerNodeGroupInfo.ConnectivityInfo.PublicAccess.Type = aws.String("SERVICE_PROVIDED_EIPS")
			c.Provisioned.ClientAuthentication.Unauthenticated.Enabled = aws.Bool(true)
			f.clusters[w5ClusterARN(name)] = c

			r := w5MSKRes(name)
			r.Fields["state"] = state

			res := w5EnrichMSK(t, f, r)
			w2AssertFinding(t, res.Findings[name], "msk.public-access",
				"brokers reachable from the internet", domain.SevBroken, "wave2")
			w2AssertFinding(t, res.Findings[name], "msk.unauthenticated",
				"unauthenticated access allowed", domain.SevBroken, "wave2")
		})
	}
}
