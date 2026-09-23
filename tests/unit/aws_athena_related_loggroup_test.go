// aws_athena_related_loggroup_test.go — the Athena workgroup's CloudWatch Logs
// pivot reads the log group the workgroup is configured to write to.
//
// Athena writes workgroup logs only where
// Configuration.MonitoringConfiguration.CloudWatchLoggingConfiguration names
// a LogGroup with Enabled=true. PublishCloudWatchMetricsEnabled is a metrics
// switch and names no log group; no log group name is derived from the
// workgroup name.
package unit_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// fakeAthenaWorkGroups answers ListWorkGroups with summaries (which carry no
// Configuration) and GetWorkGroup per workgroup name, as Athena does.
type fakeAthenaWorkGroups struct {
	byName map[string]athenatypes.WorkGroup
	getErr error
}

func (f *fakeAthenaWorkGroups) ListWorkGroups(_ context.Context, _ *athena.ListWorkGroupsInput, _ ...func(*athena.Options)) (*athena.ListWorkGroupsOutput, error) {
	out := &athena.ListWorkGroupsOutput{}
	for name, wg := range f.byName {
		out.WorkGroups = append(out.WorkGroups, athenatypes.WorkGroupSummary{Name: aws.String(name), State: wg.State, CreationTime: wg.CreationTime})
	}
	return out, nil
}

func (f *fakeAthenaWorkGroups) GetWorkGroup(_ context.Context, in *athena.GetWorkGroupInput, _ ...func(*athena.Options)) (*athena.GetWorkGroupOutput, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	wg, ok := f.byName[aws.ToString(in.WorkGroup)]
	if !ok {
		return nil, &smithy.GenericAPIError{Code: "InvalidRequestException", Message: "WorkGroup is not found."}
	}
	return &athena.GetWorkGroupOutput{WorkGroup: &wg}, nil
}

var _ awsclient.AthenaAPI = (*fakeAthenaWorkGroups)(nil)

func athenaWorkGroup(name string, cfg *athenatypes.WorkGroupConfiguration) athenatypes.WorkGroup {
	return athenatypes.WorkGroup{
		Name:          aws.String(name),
		State:         athenatypes.WorkGroupStateEnabled,
		CreationTime:  aws.Time(time.Date(2025, 11, 3, 9, 30, 0, 0, time.UTC)),
		Configuration: cfg,
	}
}

func athenaLogWorld() *fakeAthenaWorkGroups {
	results := &athenatypes.ResultConfiguration{OutputLocation: aws.String("s3://acme-athena-results/")}
	return &fakeAthenaWorkGroups{byName: map[string]athenatypes.WorkGroup{
		"acme-spark": athenaWorkGroup("acme-spark", &athenatypes.WorkGroupConfiguration{
			ResultConfiguration:             results,
			PublishCloudWatchMetricsEnabled: aws.Bool(true),
			ExecutionRole:                   aws.String("arn:aws:iam::123456789012:role/acme-athena-spark"),
			MonitoringConfiguration: &athenatypes.MonitoringConfiguration{
				CloudWatchLoggingConfiguration: &athenatypes.CloudWatchLoggingConfiguration{
					Enabled:             aws.Bool(true),
					LogGroup:            aws.String("/acme/athena/spark-driver"),
					LogStreamNamePrefix: aws.String("acme-spark"),
					LogTypes:            map[string][]string{"SPARK_DRIVER": {"STDOUT", "STDERR"}},
				},
			},
		}),
		"acme-adhoc": athenaWorkGroup("acme-adhoc", &athenatypes.WorkGroupConfiguration{
			ResultConfiguration:             results,
			PublishCloudWatchMetricsEnabled: aws.Bool(true),
			MonitoringConfiguration: &athenatypes.MonitoringConfiguration{
				CloudWatchLoggingConfiguration: &athenatypes.CloudWatchLoggingConfiguration{
					Enabled:  aws.Bool(false),
					LogGroup: aws.String("/acme/athena/adhoc"),
				},
			},
		}),
		"acme-reports": athenaWorkGroup("acme-reports", &athenatypes.WorkGroupConfiguration{
			ResultConfiguration:             results,
			PublishCloudWatchMetricsEnabled: aws.Bool(true),
			EnforceWorkGroupConfiguration:   aws.Bool(true),
		}),
	}}
}

func checkAthenaLogsFor(t *testing.T, api awsclient.AthenaAPI, wg string) resource.RelatedCheckResult {
	t.Helper()
	src := resource.Resource{
		ID:        wg,
		Name:      wg,
		Fields:    map[string]string{},
		RawStruct: athenatypes.WorkGroupSummary{Name: aws.String(wg), State: athenatypes.WorkGroupStateEnabled},
	}
	return athenaCheckerByTarget(t, "logs")(context.Background(), &awsclient.ServiceClients{Athena: api}, src, resource.ResourceCache{})
}

func TestRelated_Athena_Logs_ReadsConfiguredLogGroup(t *testing.T) {
	api := athenaLogWorld()

	t.Run("logging enabled links exactly the configured group", func(t *testing.T) {
		r := checkAthenaLogsFor(t, api, "acme-spark")
		if r.Err() != nil || r.EffectiveState() != domain.RelatedResolved || r.Truncated() {
			t.Fatalf("state %v truncated %v err %v, want a complete resolved count", r.EffectiveState(), r.Truncated(), r.Err())
		}
		if !slices.Equal(r.ResourceIDs(), []string{"/acme/athena/spark-driver"}) || r.Count() != 1 {
			t.Errorf("ResourceIDs = %v Count = %d, want [/acme/athena/spark-driver] 1", r.ResourceIDs(), r.Count())
		}
	})

	for _, wg := range []string{"acme-adhoc", "acme-reports"} {
		t.Run(wg+" writes no logs", func(t *testing.T) {
			r := checkAthenaLogsFor(t, api, wg)
			if !isProvenZero(r) {
				t.Errorf("state %v count %d truncated %v err %v ids %v, want a proven zero",
					r.EffectiveState(), r.Count(), r.Truncated(), r.Err(), r.ResourceIDs())
			}
		})
	}
}

// A workgroup whose configuration could not be read has an unknown log group,
// not a proven zero and not a guessed name.
func TestRelated_Athena_Logs_ReadFailureIsUnknown(t *testing.T) {
	api := athenaLogWorld()
	api.getErr = &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "not authorized to perform: athena:GetWorkGroup"}

	r := checkAthenaLogsFor(t, api, "acme-spark")
	if r.EffectiveState() == domain.RelatedResolved {
		t.Errorf("state resolved (count %d, ids %v), want unknown after a failed GetWorkGroup", r.Count(), r.ResourceIDs())
	}
}
