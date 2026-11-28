package unit_test

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kafka"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func t9cProduced(td catalog.ResourceTypeDef, key string) bool {
	if key == "" {
		return false
	}
	if slices.Contains(td.FieldKeys, key) || slices.Contains(td.IssueEnricherFieldKeys, key) {
		return true
	}
	for _, alias := range td.FieldAliases {
		if alias == key {
			return true
		}
	}
	return false
}

func t9cColumn(t *testing.T, typ, key string) domain.Column {
	t.Helper()
	td := catalog.FindAny(typ)
	if td == nil {
		t.Fatalf("no catalog type %s", typ)
	}
	for _, c := range td.Columns {
		if c.Key == key {
			return c
		}
	}
	t.Fatalf("%s has no column with key %q", typ, key)
	return domain.Column{}
}

func t9cNoSecondSource(t *testing.T, typ, key string) {
	t.Helper()
	if c := t9cColumn(t, typ, key); c.Path != "" {
		t.Errorf("%s column %q reads Fields[%q], which the fetcher writes, and also names path %s", typ, c.Title, key, c.Path)
	}
}

func t9cFetch(t *testing.T, typ string, clients *awsclient.ServiceClients) []resource.Resource {
	t.Helper()
	fetch := resource.GetPaginatedFetcher(typ)
	if fetch == nil {
		t.Fatalf("no fetcher for %s", typ)
	}
	got, err := fetch(context.Background(), clients, "")
	if err != nil {
		t.Fatalf("%s fetch: %v", typ, err)
	}
	return got.Resources
}

type t9cMSK struct {
	awsclient.MSKAPI
	clusters []kafkatypes.Cluster
}

func (f *t9cMSK) ListClustersV2(_ context.Context, _ *kafka.ListClustersV2Input, _ ...func(*kafka.Options)) (*kafka.ListClustersV2Output, error) {
	return &kafka.ListClustersV2Output{ClusterInfoList: f.clusters}, nil
}

// The msk Version column is the Kafka version the brokers run. CurrentVersion
// is the revision token an update call carries, not a version, and a
// serverless cluster reports no broker software, so its cell is blank.
func TestT9c_Row7_MSKVersionIsTheBrokerKafkaVersion(t *testing.T) {
	clients := demo.NewServiceClients()
	clients.MSK = &t9cMSK{MSKAPI: clients.MSK, clusters: []kafkatypes.Cluster{
		{
			ClusterName:    aws.String("orders-provisioned"),
			ClusterArn:     aws.String("arn:aws:kafka:us-east-1:123456789012:cluster/orders-provisioned/3f1c2b4a-5d6e-4f70-8a9b-0c1d2e3f4a5b-2"),
			ClusterType:    kafkatypes.ClusterTypeProvisioned,
			State:          kafkatypes.ClusterStateActive,
			CurrentVersion: aws.String("K3AEGXETSR30VB"),
			Provisioned: &kafkatypes.Provisioned{
				NumberOfBrokerNodes:       aws.Int32(3),
				CurrentBrokerSoftwareInfo: &kafkatypes.BrokerSoftwareInfo{KafkaVersion: aws.String("3.6.0")},
			},
		},
		{
			ClusterName:    aws.String("events-serverless"),
			ClusterArn:     aws.String("arn:aws:kafka:us-east-1:123456789012:cluster/events-serverless/9a8b7c6d-5e4f-4a3b-2c1d-0e9f8a7b6c5d-s1"),
			ClusterType:    kafkatypes.ClusterTypeServerless,
			State:          kafkatypes.ClusterStateActive,
			CurrentVersion: aws.String("K2EUQ1WTGCTBG2"),
			Serverless: &kafkatypes.Serverless{VpcConfigs: []kafkatypes.VpcConfig{{
				SubnetIds: []string{"subnet-0a1b2c3d4e5f60718"},
			}}},
		},
	}}
	cells := t9cListCells(t, "msk", t9cFetch(t, "msk", clients))
	for id, want := range map[string]string{"orders-provisioned": "3.6.0", "events-serverless": ""} {
		if got := cells[id]["Version"]; got != want {
			t.Errorf("msk %s Version cell %q, want %q", id, got, want)
		}
	}
	t9cNoSecondSource(t, "msk", "version")
}

type t9cSecrets struct {
	awsclient.SecretsManagerAPI
	secrets []smtypes.SecretListEntry
}

func (f *t9cSecrets) ListSecrets(_ context.Context, _ *secretsmanager.ListSecretsInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error) {
	return &secretsmanager.ListSecretsOutput{SecretList: f.secrets}, nil
}

// The secrets Status cell is the lifecycle the fetcher derives from
// DeletedDate (ListSecrets has no status field): blank for an active secret
// with nothing wrong, the deletion phrase for one in its recovery window.
// The secret's name is not its status.
func TestT9c_Row7_SecretsStatusIsTheLifecycle(t *testing.T) {
	now := time.Now()
	clients := demo.NewServiceClients()
	clients.SecretsManager = &t9cSecrets{SecretsManagerAPI: clients.SecretsManager, secrets: []smtypes.SecretListEntry{
		{
			Name:             aws.String("prod/orders/db-password"),
			ARN:              aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/orders/db-password-AbCdEf"),
			RotationEnabled:  aws.Bool(true),
			NextRotationDate: aws.Time(now.Add(20 * 24 * time.Hour)),
			LastChangedDate:  aws.Time(now.Add(-10 * 24 * time.Hour)),
			LastAccessedDate: aws.Time(now.Add(-24 * time.Hour)),
		},
		{
			Name:             aws.String("staging/legacy-api-key"),
			ARN:              aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:staging/legacy-api-key-GhIjKl"),
			RotationEnabled:  aws.Bool(true),
			NextRotationDate: aws.Time(now.Add(20 * 24 * time.Hour)),
			LastChangedDate:  aws.Time(now.Add(-10 * 24 * time.Hour)),
			LastAccessedDate: aws.Time(now.Add(-24 * time.Hour)),
			DeletedDate:      aws.Time(now.Add(-2 * 24 * time.Hour)),
		},
	}}
	cells := t9cListCells(t, "secrets", t9cFetch(t, "secrets", clients))
	for id, want := range map[string]string{"prod/orders/db-password": "", "staging/legacy-api-key": "scheduled for deletion"} {
		if got := cells[id]["Status"]; got != want {
			t.Errorf("secrets %s Status cell %q, want %q", id, got, want)
		}
	}
	t9cNoSecondSource(t, "secrets", "status")
}

// A task's ID is the last segment of its ARN; the ARN is a different fact
// and no source of the Task ID column.
func TestT9c_Row7_ECSTaskIDAndClusterAreTheProducedKeys(t *testing.T) {
	clients, _ := t9cECSClients()
	rows := t9cFetch(t, "ecs-task", clients)
	cells := t9cListCells(t, "ecs-task", rows)
	running, ok := t9cRowByID(rows, t9cRunningTaskID)
	if !ok {
		t.Fatalf("no ecs-task row %s; rows %d", t9cRunningTaskID, len(rows))
	}
	if got := cells[running.ID]["Task ID"]; got != t9cRunningTaskID {
		t.Errorf("ecs-task Task ID cell %q, want %q", got, t9cRunningTaskID)
	}
	if got, want := cells[running.ID]["Cluster"], running.Fields["cluster"]; got != want || want == "" {
		t.Errorf("ecs-task Cluster cell %q, the fetcher writes %q", got, want)
	}
	t9cNoSecondSource(t, "ecs-task", "task_id")
	t9cNoSecondSource(t, "ecs-task", "cluster")
}

func t9cNonScalarOnPath(rt reflect.Type, path string) (reflect.Kind, bool) {
	segs := strings.Split(path, ".")
	for i := range segs {
		leaf, ok := t9cResolve(rt, strings.Join(segs[:i+1], "."))
		if !ok {
			return 0, false
		}
		if k := t9cDeref(leaf).Kind(); k == reflect.Slice || k == reflect.Map {
			return k, true
		}
	}
	return 0, false
}

// A column reads one source: the Fields key a fetcher or the Wave 2 enricher
// writes, or, for a key nothing writes, a path on the row struct. A path
// that ends on a slice or a map renders no scalar, so it is no column source.
func TestT9c_Row8_EveryColumnHasOneSource(t *testing.T) {
	stored := t9cListRowTypes(t)
	var problems []string
	for _, td := range t9cAllDefs() {
		types := append([]reflect.Type{}, t9cRowTypes(td)...)
		for rt := range stored[td.ShortName] {
			types = append(types, rt)
		}
		for _, c := range td.Columns {
			if c.Path == "" {
				continue
			}
			if t9cProduced(td, c.Key) {
				problems = append(problems, fmt.Sprintf("%s column %q: key %q is produced and path %s is a second source", td.ShortName, c.Title, c.Key, c.Path))
			}
			for _, rt := range types {
				if k, ok := t9cNonScalarOnPath(rt, c.Path); ok {
					problems = append(problems, fmt.Sprintf("%s column %q: path %s names a %s on %v", td.ShortName, c.Title, c.Path, k, rt))
					break
				}
			}
		}
	}
	t9bReport(t, "columns with two sources or a non-scalar path", problems)
}
