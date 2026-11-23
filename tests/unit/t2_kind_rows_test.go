package unit_test

import (
	"context"
	"slices"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/tests/unit"
)

const (
	t2GlueLoggingOff       domain.FindingCode = "glue.continuous-logging-off"
	t2GlueNoSecConfig      domain.FindingCode = "glue.no-security-configuration"
	t2GlueSecConfigWeak    domain.FindingCode = "glue.security-configuration-encryption-off"
	t2SSMStale             domain.FindingCode = "ssm.value.stale"
	t2SSMPlaintext         domain.FindingCode = "ssm.value.plaintext-sensitive"
	t2LogsNoMetricFilters  domain.FindingCode = "logs.missing-metric-filters"
	t2DBIDeletionProtOff   domain.FindingCode = "dbi.warn.deletion_protection_off"
	t2DBISingleAZ          domain.FindingCode = "dbi.single-az"
	t2DBINoBackups         domain.FindingCode = "dbi.warn.no_automated_backups"
	t2SecretDormant        domain.FindingCode = "secrets.state.dormant"
	t2SecretRotationOff    domain.FindingCode = "secrets.rotation.disabled"
	t2SecretStale          domain.FindingCode = "secrets.value.stale"
	t2SecretRotationLate   domain.FindingCode = "secrets.state.rotation_overdue"
	t2EBSUnencrypted       domain.FindingCode = "ebs.encryption.disabled"
	t2EBSOrphanUnattached  domain.FindingCode = "ebs.orphan-unattached"
	t2GlueRoleARN                             = "arn:aws:iam::123456789012:role/acme-glue-service-role"
	t2GlueBookmarksRowName                    = "Job bookmarks"
)

func t2Codes(fs []domain.Finding) []domain.FindingCode {
	out := make([]domain.FindingCode, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.Code)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func t2Days(n int) *time.Time {
	ts := time.Now().Add(-time.Duration(n) * 24 * time.Hour)
	return &ts
}

type t2GlueFake struct {
	jobs    []gluetypes.Job
	configs map[string]gluetypes.SecurityConfiguration

	mu        sync.Mutex
	cfgReads  []string
	runsReads []string
}

func (f *t2GlueFake) GetJobs(context.Context, *glue.GetJobsInput, ...func(*glue.Options)) (*glue.GetJobsOutput, error) {
	return &glue.GetJobsOutput{Jobs: f.jobs}, nil
}

func (f *t2GlueFake) GetJobRuns(_ context.Context, in *glue.GetJobRunsInput, _ ...func(*glue.Options)) (*glue.GetJobRunsOutput, error) {
	f.mu.Lock()
	f.runsReads = append(f.runsReads, aws.ToString(in.JobName))
	f.mu.Unlock()
	return &glue.GetJobRunsOutput{JobRuns: []gluetypes.JobRun{{
		JobName:     in.JobName,
		Id:          aws.String("jr_0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"),
		JobRunState: gluetypes.JobRunStateSucceeded,
		StartedOn:   t2Days(1),
		CompletedOn: t2Days(1),
	}}}, nil
}

func (f *t2GlueFake) GetSecurityConfiguration(_ context.Context, in *glue.GetSecurityConfigurationInput, _ ...func(*glue.Options)) (*glue.GetSecurityConfigurationOutput, error) {
	name := aws.ToString(in.Name)
	f.mu.Lock()
	f.cfgReads = append(f.cfgReads, name)
	f.mu.Unlock()
	cfg, ok := f.configs[name]
	if !ok {
		return &glue.GetSecurityConfigurationOutput{}, nil
	}
	return &glue.GetSecurityConfigurationOutput{SecurityConfiguration: &cfg}, nil
}

// t2GlueJob is a job as GetJobs returns it. A Python shell job reports the
// default GlueVersion; a Ray job names its runtime; Spark jobs carry workers.
func t2GlueJob(name, command, glueVersion, secConfig string) gluetypes.Job {
	job := gluetypes.Job{
		Name:           aws.String(name),
		Role:           aws.String(t2GlueRoleARN),
		CreatedOn:      t2Days(200),
		LastModifiedOn: t2Days(30),
		GlueVersion:    aws.String(glueVersion),
		MaxRetries:     0,
		Timeout:        aws.Int32(2880),
		Command: &gluetypes.JobCommand{
			Name:           aws.String(command),
			ScriptLocation: aws.String("s3://acme-glue-scripts-123456789012/" + name + ".py"),
			PythonVersion:  aws.String("3"),
		},
		DefaultArguments: map[string]string{
			"--job-language": "python",
			"--TempDir":      "s3://acme-glue-temp-123456789012/" + name + "/",
		},
	}
	switch command {
	case "pythonshell":
		job.MaxCapacity = aws.Float64(0.0625)
		job.Command.PythonVersion = aws.String("3.9")
	case "glueray":
		job.WorkerType = gluetypes.WorkerType("Z.2X")
		job.NumberOfWorkers = aws.Int32(5)
		job.Command.Runtime = aws.String("Ray2.4")
		job.Command.PythonVersion = aws.String("3.9")
	default:
		job.WorkerType = gluetypes.WorkerTypeG1x
		job.NumberOfWorkers = aws.Int32(10)
		job.DefaultArguments["--enable-metrics"] = "true"
		job.DefaultArguments["--job-bookmark-option"] = "job-bookmark-enable"
	}
	if secConfig != "" {
		job.SecurityConfiguration = aws.String(secConfig)
	}
	return job
}

func t2GlueRows(t *testing.T, fake *t2GlueFake) map[string]resource.Resource {
	t.Helper()
	res, err := awsclient.FetchGlueJobsPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("FetchGlueJobsPage: %v", err)
	}
	out := map[string]resource.Resource{}
	for _, r := range res.Resources {
		out[r.ID] = r
	}
	if len(out) != len(fake.jobs) {
		t.Fatalf("fetched %d glue rows, want %d", len(out), len(fake.jobs))
	}
	return out
}

// Continuous logging is a Spark driver/executor feature: AWS Glue offers it on
// Spark ETL (glueetl) and Spark streaming (gluestreaming) jobs. A Python shell
// job writes to /aws-glue/python-jobs/output and a Ray job to its own log
// groups, so neither can be told to add the Spark flag.
func TestT2Glue_ContinuousLoggingAppliesToSparkJobsOnly(t *testing.T) {
	fake := &t2GlueFake{jobs: []gluetypes.Job{
		t2GlueJob("acme-orders-etl", "glueetl", "4.0", "acme-sec-strict"),
		t2GlueJob("acme-clickstream-ingest", "gluestreaming", "4.0", "acme-sec-strict"),
		t2GlueJob("acme-nightly-report", "pythonshell", "3.0", "acme-sec-strict"),
		t2GlueJob("acme-feature-builder", "glueray", "4.0", ""),
	}}
	rows := t2GlueRows(t, fake)
	want := map[string]bool{
		"acme-orders-etl":         true,
		"acme-clickstream-ingest": true,
		"acme-nightly-report":     false,
		"acme-feature-builder":    false,
	}
	for name, has := range want {
		got := t1aHas(rows[name].Findings, t2GlueLoggingOff)
		if got != has {
			t.Errorf("%s (%s): continuous-logging-off present = %v, want %v; findings %v",
				name, rows[name].Fields["command"], got, has, t2Codes(rows[name].Findings))
		}
	}
}

// AWS Glue: "AWS Glue security configurations are not currently supported in
// Ray jobs." A Ray job naming none is not missing one; Spark and Python shell
// jobs that name none are.
func TestT2Glue_NoSecurityConfigurationDoesNotApplyToRayJobs(t *testing.T) {
	fake := &t2GlueFake{jobs: []gluetypes.Job{
		t2GlueJob("acme-orders-etl", "glueetl", "5.0", ""),
		t2GlueJob("acme-clickstream-ingest", "gluestreaming", "5.0", ""),
		t2GlueJob("acme-nightly-report", "pythonshell", "3.0", ""),
		t2GlueJob("acme-feature-builder", "glueray", "4.0", ""),
	}}
	rows := t2GlueRows(t, fake)
	want := map[string]bool{
		"acme-orders-etl":         true,
		"acme-clickstream-ingest": true,
		"acme-nightly-report":     true,
		"acme-feature-builder":    false,
	}
	for name, has := range want {
		if got := t1aHas(rows[name].Findings, t2GlueNoSecConfig); got != has {
			t.Errorf("%s (%s): no-security-configuration present = %v, want %v; findings %v",
				name, rows[name].Fields["command"], got, has, t2Codes(rows[name].Findings))
		}
	}
}

func t2GlueSecConfig(name string, s3, cw, bookmarks bool) gluetypes.SecurityConfiguration {
	const key = "arn:aws:kms:us-east-1:123456789012:key/1234abcd-12ab-34cd-56ef-1234567890ab"
	ec := &gluetypes.EncryptionConfiguration{
		S3Encryption:           []gluetypes.S3Encryption{{S3EncryptionMode: gluetypes.S3EncryptionModeDisabled}},
		CloudWatchEncryption:   &gluetypes.CloudWatchEncryption{CloudWatchEncryptionMode: gluetypes.CloudWatchEncryptionModeDisabled},
		JobBookmarksEncryption: &gluetypes.JobBookmarksEncryption{JobBookmarksEncryptionMode: gluetypes.JobBookmarksEncryptionModeDisabled},
	}
	if s3 {
		ec.S3Encryption = []gluetypes.S3Encryption{{S3EncryptionMode: gluetypes.S3EncryptionModeSsekms, KmsKeyArn: aws.String(key)}}
	}
	if cw {
		ec.CloudWatchEncryption = &gluetypes.CloudWatchEncryption{CloudWatchEncryptionMode: gluetypes.CloudWatchEncryptionModeSsekms, KmsKeyArn: aws.String(key)}
	}
	if bookmarks {
		ec.JobBookmarksEncryption = &gluetypes.JobBookmarksEncryption{JobBookmarksEncryptionMode: gluetypes.JobBookmarksEncryptionModeCsekms, KmsKeyArn: aws.String(key)}
	}
	return gluetypes.SecurityConfiguration{Name: aws.String(name), CreatedTimeStamp: t2Days(400), EncryptionConfiguration: ec}
}

func t2RowLabels(res awsclient.IssueEnricherResult, id string, code domain.FindingCode) []string {
	var out []string
	for _, row := range res.AttentionDetails[id][code].Rows {
		out = append(out, row.Label)
	}
	sort.Strings(out)
	return out
}

// Job bookmarks exist on Spark ETL jobs only. AWS Glue: Python shell — "You
// can't use job bookmarks with Python shell jobs"; streaming — "AWS Glue
// streaming jobs use checkpoints rather than job bookmarks" and "Job bookmarks
// aren't supported"; Ray — security configurations are not supported at all.
// So "Job bookmarks: not encrypted" is a defect of a glueetl job alone, a
// configuration whose only gap is bookmark encryption is no finding on any
// other kind, and a Ray job's configuration is never read or marked
// uninspected.
func TestT2Glue_SecurityConfigurationRowsFollowTheJobKind(t *testing.T) {
	fake := &t2GlueFake{
		jobs: []gluetypes.Job{
			t2GlueJob("acme-orders-etl", "glueetl", "5.0", "acme-sec-weak"),
			t2GlueJob("acme-clickstream-ingest", "gluestreaming", "5.0", "acme-sec-weak"),
			t2GlueJob("acme-nightly-report", "pythonshell", "3.0", "acme-sec-weak"),
			t2GlueJob("acme-sessionizer", "gluestreaming", "5.0", "acme-sec-bookmarks-off"),
			t2GlueJob("acme-feature-builder", "glueray", "4.0", "acme-sec-ray"),
		},
		configs: map[string]gluetypes.SecurityConfiguration{
			"acme-sec-weak":          t2GlueSecConfig("acme-sec-weak", false, false, false),
			"acme-sec-bookmarks-off": t2GlueSecConfig("acme-sec-bookmarks-off", true, true, false),
			"acme-sec-ray":           t2GlueSecConfig("acme-sec-ray", false, false, false),
		},
	}
	rows := t2GlueRows(t, fake)
	list := make([]resource.Resource, 0, len(rows))
	for _, r := range rows {
		list = append(list, r)
	}
	res := unit.Wave2Result(t, "glue", &awsclient.ServiceClients{Glue: fake}, list)

	allThree := []string{"CloudWatch logs", t2GlueBookmarksRowName, "S3 output"}
	noBookmarks := []string{"CloudWatch logs", "S3 output"}
	cases := []struct {
		job  string
		want []string
	}{
		{"acme-orders-etl", allThree},
		{"acme-clickstream-ingest", noBookmarks},
		{"acme-nightly-report", noBookmarks},
		{"acme-sessionizer", nil},
		{"acme-feature-builder", nil},
	}
	for _, tc := range cases {
		has := t1aHas(res.Findings[tc.job], t2GlueSecConfigWeak)
		if has != (tc.want != nil) {
			t.Errorf("%s (%s): security-configuration-encryption-off present = %v, want %v",
				tc.job, rows[tc.job].Fields["command"], has, tc.want != nil)
			continue
		}
		if got := t2RowLabels(res, tc.job, t2GlueSecConfigWeak); tc.want != nil && !slices.Equal(got, tc.want) {
			t.Errorf("%s (%s): supporting rows %v, want %v", tc.job, rows[tc.job].Fields["command"], got, tc.want)
		}
	}
	if slices.Contains(fake.cfgReads, "acme-sec-ray") {
		t.Errorf("GetSecurityConfiguration was called for the Ray job's configuration; reads %v", fake.cfgReads)
	}
	if reason, marked := res.TruncatedIDs["acme-feature-builder"]; marked {
		t.Errorf("Ray job marked not inspected (%q); nothing about it was left unread", reason)
	}
}

type t2SSMFake struct{ params []ssmtypes.ParameterMetadata }

func (f *t2SSMFake) DescribeParameters(context.Context, *ssm.DescribeParametersInput, ...func(*ssm.Options)) (*ssm.DescribeParametersOutput, error) {
	return &ssm.DescribeParametersOutput{Parameters: f.params}, nil
}

func t2Param(name string, typ ssmtypes.ParameterType, ageDays int) ssmtypes.ParameterMetadata {
	return ssmtypes.ParameterMetadata{
		Name:             aws.String(name),
		ARN:              aws.String("arn:aws:ssm:us-east-1:123456789012:parameter" + name),
		Type:             typ,
		Tier:             ssmtypes.ParameterTierStandard,
		DataType:         aws.String("text"),
		Version:          3,
		LastModifiedDate: t2Days(ageDays),
		LastModifiedUser: aws.String("arn:aws:iam::123456789012:role/acme-deployer"),
	}
}

// A year-old value is a rotation problem only where the value is a secret: a
// SecureString. A String or StringList parameter is plaintext configuration,
// and its age says nothing an operator should act on. A plaintext parameter
// whose name says credential still carries its own finding.
func TestT2SSM_StaleValueAppliesToSecureStringOnly(t *testing.T) {
	fake := &t2SSMFake{params: []ssmtypes.ParameterMetadata{
		t2Param("/acme/app/feature-flags", ssmtypes.ParameterTypeString, 400),
		t2Param("/acme/app/allowed-origins", ssmtypes.ParameterTypeStringList, 400),
		t2Param("/acme/app/api-token", ssmtypes.ParameterTypeSecureString, 400),
		t2Param("/acme/app/session-key", ssmtypes.ParameterTypeSecureString, 10),
		t2Param("/acme/db/password", ssmtypes.ParameterTypeString, 400),
	}}
	res, err := awsclient.FetchSSMParametersPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("FetchSSMParametersPage: %v", err)
	}
	want := map[string][]domain.FindingCode{
		"/acme/app/feature-flags":   {},
		"/acme/app/allowed-origins": {},
		"/acme/app/api-token":       {t2SSMStale},
		"/acme/app/session-key":     {},
		"/acme/db/password":         {t2SSMPlaintext},
	}
	if len(res.Resources) != len(want) {
		t.Fatalf("fetched %d ssm rows, want %d", len(res.Resources), len(want))
	}
	for _, r := range res.Resources {
		if got := t2Codes(r.Findings); !slices.Equal(got, want[r.ID]) {
			t.Errorf("%s (%s): findings %v, want %v", r.ID, r.Fields["type"], got, want[r.ID])
		}
	}
}

type t2RDSFake struct{ instances []rdstypes.DBInstance }

func (f *t2RDSFake) DescribeDBInstances(context.Context, *rds.DescribeDBInstancesInput, ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
	return &rds.DescribeDBInstancesOutput{DBInstances: f.instances}, nil
}

func t2DBInstance(id, engine, version, cluster string) rdstypes.DBInstance {
	db := rdstypes.DBInstance{
		DBInstanceIdentifier:             aws.String(id),
		DBInstanceArn:                    aws.String("arn:aws:rds:us-east-1:123456789012:db:" + id),
		DbiResourceId:                    aws.String("db-ABCDEFGHIJKLMNOPQRSTUV0123"),
		DBInstanceClass:                  aws.String("db.r6g.large"),
		DBInstanceStatus:                 aws.String("available"),
		Engine:                           aws.String(engine),
		EngineVersion:                    aws.String(version),
		AvailabilityZone:                 aws.String("us-east-1a"),
		Endpoint:                         &rdstypes.Endpoint{Address: aws.String(id + ".c1a2b3c4d5e6.us-east-1.rds.amazonaws.com"), Port: aws.Int32(5432)},
		MasterUsername:                   aws.String("acme_admin"),
		MultiAZ:                          aws.Bool(false),
		DeletionProtection:               aws.Bool(false),
		BackupRetentionPeriod:            aws.Int32(0),
		PubliclyAccessible:               aws.Bool(false),
		StorageEncrypted:                 aws.Bool(true),
		AutoMinorVersionUpgrade:          aws.Bool(true),
		IAMDatabaseAuthenticationEnabled: aws.Bool(true),
		InstanceCreateTime:               t2Days(300),
	}
	if cluster != "" {
		db.DBClusterIdentifier = aws.String(cluster)
	}
	return db
}

// A DB instance in a cluster takes deletion protection, its AZ layout and
// backup retention from the cluster. AWS, CreateDBInstance: BackupRetentionPeriod
// "doesn't apply to Amazon Aurora DB instances. The retention period for
// automated backups is managed by the DB cluster"; DeletionProtection "doesn't
// apply to Amazon Aurora DB instances. You can enable or disable deletion
// protection for the DB cluster"; MultiAZ "doesn't apply to Amazon Aurora
// because the DB instance Availability Zones (AZs) are managed by the DB
// cluster". CreateDBCluster: BackupRetentionPeriod and DeletionProtection are
// "Valid for Cluster Type: Aurora DB clusters and Multi-AZ DB clusters", and a
// Multi-AZ DB cluster gets "a writer and two reader DB instances ... each in a
// different Availability Zone". So a member — Aurora, DocumentDB, Neptune or a
// Multi-AZ DB cluster's mysql/postgres instance — raises none of the three; a
// standalone instance with the same values raises all three.
func TestT2DBI_ClusterMembersDoNotCarryClusterManagedFindings(t *testing.T) {
	fake := &t2RDSFake{instances: []rdstypes.DBInstance{
		t2DBInstance("acme-aurora-writer", "aurora-postgresql", "15.4", "acme-aurora"),
		t2DBInstance("acme-docdb-1", "docdb", "5.0.0", "acme-docdb"),
		t2DBInstance("acme-graph-1", "neptune", "1.3.1.0", "acme-graph"),
		t2DBInstance("acme-orders-maz-instance-1", "mysql", "8.0.35", "acme-orders-maz"),
		t2DBInstance("acme-ledger-maz-instance-2", "postgres", "16.1", "acme-ledger-maz"),
		t2DBInstance("acme-legacy-pg", "postgres", "16.1", ""),
	}}
	res, err := awsclient.FetchRDSInstancesPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("FetchRDSInstancesPage: %v", err)
	}
	clusterManaged := []domain.FindingCode{t2DBIDeletionProtOff, t2DBISingleAZ, t2DBINoBackups}
	if len(res.Resources) != len(fake.instances) {
		t.Fatalf("fetched %d dbi rows, want %d", len(res.Resources), len(fake.instances))
	}
	for _, r := range res.Resources {
		standalone := r.ID == "acme-legacy-pg"
		for _, code := range clusterManaged {
			if got := t1aHas(r.Findings, code); got != standalone {
				t.Errorf("%s (engine %s, cluster member %v): %s present = %v, want %v",
					r.ID, r.Fields["engine"], !standalone, code, got, standalone)
			}
		}
	}
}

type t2LogsFake struct {
	groups []cwlogstypes.LogGroup

	mu            sync.Mutex
	filterReads   []string
	getEventsRead []string
}

func (f *t2LogsFake) DescribeLogGroups(context.Context, *cloudwatchlogs.DescribeLogGroupsInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
	return &cloudwatchlogs.DescribeLogGroupsOutput{LogGroups: f.groups}, nil
}

func (f *t2LogsFake) DescribeLogStreams(_ context.Context, in *cloudwatchlogs.DescribeLogStreamsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
	group := aws.ToString(in.LogGroupName)
	now := time.Now()
	return &cloudwatchlogs.DescribeLogStreamsOutput{LogStreams: []cwlogstypes.LogStream{{
		LogStreamName:       aws.String("123456789012_CloudTrail_us-east-1"),
		Arn:                 aws.String("arn:aws:logs:us-east-1:123456789012:log-group:" + group + ":log-stream:123456789012_CloudTrail_us-east-1"),
		CreationTime:        aws.Int64(now.Add(-90 * 24 * time.Hour).UnixMilli()),
		FirstEventTimestamp: aws.Int64(now.Add(-90 * 24 * time.Hour).UnixMilli()),
		LastEventTimestamp:  aws.Int64(now.Add(-10 * time.Minute).UnixMilli()),
	}}}, nil
}

func (f *t2LogsFake) GetLogEvents(_ context.Context, in *cloudwatchlogs.GetLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error) {
	f.mu.Lock()
	f.getEventsRead = append(f.getEventsRead, aws.ToString(in.LogGroupName))
	f.mu.Unlock()
	return &cloudwatchlogs.GetLogEventsOutput{}, nil
}

func (f *t2LogsFake) FilterLogEvents(context.Context, *cloudwatchlogs.FilterLogEventsInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error) {
	return &cloudwatchlogs.FilterLogEventsOutput{}, nil
}

func (f *t2LogsFake) DescribeMetricFilters(_ context.Context, in *cloudwatchlogs.DescribeMetricFiltersInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeMetricFiltersOutput, error) {
	f.mu.Lock()
	f.filterReads = append(f.filterReads, aws.ToString(in.LogGroupName))
	f.mu.Unlock()
	return &cloudwatchlogs.DescribeMetricFiltersOutput{}, nil
}

// t2LogClasses names one CloudTrail log group per class value. "" is a group
// created before log classes existed, which DescribeLogGroups reports with no
// class; ESSENTIALS is what read-path APIs return for Infrequent Access from
// December 2026; FUTURE_CLASS stands for any class AWS adds later.
var t2LogClasses = []struct {
	class    cwlogstypes.LogGroupClass
	standard bool
}{
	{cwlogstypes.LogGroupClassStandard, true},
	{"", true},
	{cwlogstypes.LogGroupClassInfrequentAccess, false},
	{cwlogstypes.LogGroupClass("ESSENTIALS"), false},
	{cwlogstypes.LogGroupClassDelivery, false},
	{cwlogstypes.LogGroupClass("FUTURE_CLASS"), false},
}

func t2LogGroupName(class cwlogstypes.LogGroupClass) string {
	if class == "" {
		return "/aws/cloudtrail/acme-org-trail-legacy"
	}
	return "/aws/cloudtrail/acme-org-trail-" + string(class)
}

func t2LogRows(t *testing.T) (*t2LogsFake, []resource.Resource) {
	t.Helper()
	fake := &t2LogsFake{}
	created := time.Now().Add(-400 * 24 * time.Hour).UnixMilli()
	for _, c := range t2LogClasses {
		name := t2LogGroupName(c.class)
		fake.groups = append(fake.groups, cwlogstypes.LogGroup{
			LogGroupName:    aws.String(name),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:" + name + ":*"),
			LogGroupArn:     aws.String("arn:aws:logs:us-east-1:123456789012:log-group:" + name),
			CreationTime:    aws.Int64(created),
			RetentionInDays: aws.Int32(365),
			StoredBytes:     aws.Int64(52_428_800),
			KmsKeyId:        aws.String("arn:aws:kms:us-east-1:123456789012:key/1234abcd-12ab-34cd-56ef-1234567890ab"),
			LogGroupClass:   c.class,
		})
	}
	res, err := awsclient.FetchCloudWatchLogGroupsPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("FetchCloudWatchLogGroupsPage: %v", err)
	}
	if len(res.Resources) != len(t2LogClasses) {
		t.Fatalf("fetched %d log groups, want %d", len(res.Resources), len(t2LogClasses))
	}
	return fake, res.Resources
}

// AWS, "Log classes": metric filters are a Standard-class feature ("Metric
// filters | Yes | No"), and the Delivery class "doesn't offer rich CloudWatch
// Logs capabilities". The check applies when the class is STANDARD or absent;
// on any other class there is no metric filter to be missing, so the group is
// neither read nor marked uninspected.
func TestT2Logs_MetricFilterCheckAppliesToStandardOrUnclassedGroupsOnly(t *testing.T) {
	fake, rows := t2LogRows(t)
	res := unit.Wave2Result(t, "logs", &awsclient.ServiceClients{CloudWatchLogs: fake}, rows)
	for _, c := range t2LogClasses {
		name := t2LogGroupName(c.class)
		if got := t1aHas(res.Findings[name], t2LogsNoMetricFilters); got != c.standard {
			t.Errorf("class %q: missing-metric-filters present = %v, want %v", c.class, got, c.standard)
		}
		read := slices.Contains(fake.filterReads, name)
		if read != c.standard {
			t.Errorf("class %q: DescribeMetricFilters called = %v, want %v", c.class, read, c.standard)
		}
		if reason, marked := res.TruncatedIDs[name]; marked && !c.standard {
			t.Errorf("class %q: group marked not inspected (%q) for a check its class cannot have", c.class, reason)
		}
	}
}

// t2ReachesLogEvents drives the real drill path from a log group row through
// its stream listing to the event view, and reports whether the event view's
// fetch was dispatched.
func t2ReachesLogEvents(t *testing.T, clients *awsclient.ServiceClients, group resource.Resource) bool {
	t.Helper()
	logsTD := resource.FindResourceType("logs")
	streamsTD := resource.GetChildType("log_streams")
	if logsTD == nil || streamsTD == nil || streamsTD.ChildFetcher == nil {
		t.Fatal("logs or log_streams is not registered")
	}
	idx := slices.IndexFunc(logsTD.Children, func(c resource.ChildViewDef) bool { return c.ChildType == "log_streams" })
	eIdx := slices.IndexFunc(streamsTD.Children, func(c resource.ChildViewDef) bool { return c.ChildType == "log_events" })
	if idx < 0 || eIdx < 0 {
		t.Fatal("the logs -> log_streams -> log_events drill is not declared")
	}
	toStreams, toEvents := logsTD.Children[idx], streamsTD.Children[eIdx]

	c := newVisibilityListController(t, "logs")
	c.ApplyResourcesLoaded("logs", []resource.Resource{group}, nil, false)
	if _, tasks := c.Apply(app.Action{Kind: app.ActionChildView, Arg: toStreams.Key}); len(tasks) == 0 {
		return false
	}
	dr := domain.Resource(group)
	streams, err := streamsTD.ChildFetcher(context.Background(), clients, resource.ResolveChildContext(toStreams, &dr, nil), "")
	if err != nil || len(streams.Resources) == 0 {
		t.Fatalf("%s: stream listing returned %d rows, err %v", group.ID, len(streams.Resources), err)
	}
	c.ApplyResourcesLoaded("log_streams", streams.Resources, nil, false)
	_, tasks := c.Apply(app.Action{Kind: app.ActionChildView, Arg: toEvents.Key})
	return len(tasks) > 0
}

// AWS, "Log classes": for the Essentials class the GetLogEvents and
// FilterLogEvents API operations are "Not supported. Use CloudWatch Logs
// Insights to view log events". The event view reads GetLogEvents, so it is
// offered from a STANDARD or unclassed group and from no other.
func TestT2Logs_EventViewIsOfferedOnStandardOrUnclassedGroupsOnly(t *testing.T) {
	fake, rows := t2LogRows(t)
	clients := &awsclient.ServiceClients{CloudWatchLogs: fake}
	for i, c := range t2LogClasses {
		if got := t2ReachesLogEvents(t, clients, rows[i]); got != c.standard {
			t.Errorf("class %q (%s): log event view offered = %v, want %v", c.class, rows[i].ID, got, c.standard)
		}
	}
}

type t2SecretsFake struct{ secrets []smtypes.SecretListEntry }

func (f *t2SecretsFake) ListSecrets(context.Context, *secretsmanager.ListSecretsInput, ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error) {
	return &secretsmanager.ListSecretsOutput{SecretList: f.secrets}, nil
}

func t2Secret(name string, rotation bool, nextRotation *time.Time, accessedDays, changedDays int) smtypes.SecretListEntry {
	s := smtypes.SecretListEntry{
		Name:             aws.String(name),
		ARN:              aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:" + name + "-AbCdEf"),
		Description:      aws.String("Credentials for " + name),
		CreatedDate:      t2Days(900),
		LastAccessedDate: t2Days(accessedDays),
		LastChangedDate:  t2Days(changedDays),
		RotationEnabled:  aws.Bool(rotation),
		NextRotationDate: nextRotation,
	}
	if rotation {
		s.RotationLambdaARN = aws.String("arn:aws:lambda:us-east-1:123456789012:function:acme-secret-rotator")
		s.RotationRules = &smtypes.RotationRulesType{AutomaticallyAfterDays: aws.Int64(30)}
	}
	return s
}

// Each posture check on a secret is its own fact, and fixing one leaves the
// others: a secret nobody reads, with rotation off and a year-old value, shows
// all three; an overdue rotation on a secret nobody reads shows both.
func TestT2Secrets_EveryPostureDefectIsItsOwnFinding(t *testing.T) {
	fake := &t2SecretsFake{secrets: []smtypes.SecretListEntry{
		t2Secret("acme/prod/legacy-db", false, nil, 200, 400),
		t2Secret("acme/prod/partner-api", true, t2Days(5), 200, 100),
		t2Secret("acme/prod/orders-db", true, aws.Time(time.Now().Add(10*24*time.Hour)), 1, 20),
	}}
	res, err := awsclient.FetchSecretsPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("FetchSecretsPage: %v", err)
	}
	want := map[string][]domain.FindingCode{
		"acme/prod/legacy-db":   {t2SecretRotationOff, t2SecretDormant, t2SecretStale},
		"acme/prod/partner-api": {t2SecretDormant, t2SecretRotationLate},
		"acme/prod/orders-db":   {},
	}
	for _, r := range res.Resources {
		w := slices.Clone(want[r.ID])
		slices.Sort(w)
		if got := t2Codes(r.Findings); !slices.Equal(got, w) {
			t.Errorf("%s: findings %v, want %v", r.ID, got, w)
		}
	}
}

type t2VolumesFake struct{ volumes []ec2types.Volume }

func (f *t2VolumesFake) DescribeVolumes(context.Context, *ec2.DescribeVolumesInput, ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	return &ec2.DescribeVolumesOutput{Volumes: f.volumes}, nil
}

// An unattached volume that is also unencrypted has two defects with two
// fixes; the row shows both.
func TestT2EBS_UnencryptedOrphanShowsBothFindings(t *testing.T) {
	fake := &t2VolumesFake{volumes: []ec2types.Volume{{
		VolumeId:         aws.String("vol-0a1b2c3d4e5f60718"),
		State:            ec2types.VolumeStateAvailable,
		Size:             aws.Int32(100),
		VolumeType:       ec2types.VolumeTypeGp3,
		Iops:             aws.Int32(3000),
		Encrypted:        aws.Bool(false),
		AvailabilityZone: aws.String("us-east-1a"),
		CreateTime:       t2Days(40),
		Tags:             []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("acme-old-build-cache")}},
	}}}
	res, err := awsclient.FetchEBSVolumesPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("FetchEBSVolumesPage: %v", err)
	}
	if len(res.Resources) != 1 {
		t.Fatalf("fetched %d volumes, want 1", len(res.Resources))
	}
	want := []domain.FindingCode{t2EBSOrphanUnattached, t2EBSUnencrypted}
	slices.Sort(want)
	if got := t2Codes(res.Resources[0].Findings); !slices.Equal(got, want) {
		t.Errorf("unattached unencrypted volume: findings %v, want %v", got, want)
	}
}
