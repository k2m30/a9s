package main

import (
	"cmp"
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	autoscalingtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

// ---------------------------------------------------------------------------
// lambda
// ---------------------------------------------------------------------------

type lambdaData struct {
	Functions []lambdaFunction `json:"functions"`
}

type lambdaFunction struct {
	FunctionName               string `json:"function_name"`
	FunctionArn                string `json:"function_arn"`
	State                      string `json:"state,omitempty"`
	StateReason                string `json:"state_reason,omitempty"`
	StateReasonCode            string `json:"state_reason_code,omitempty"`
	LastUpdateStatus           string `json:"last_update_status,omitempty"`
	LastUpdateStatusReason     string `json:"last_update_status_reason,omitempty"`
	LastUpdateStatusReasonCode string `json:"last_update_status_reason_code,omitempty"`
	// GetFunction is the read the lifecycle fields come from: they are
	// unknown, not empty, unless it is "ok".
	GetFunction         readOutcome `json:"get_function"`
	Runtime             string      `json:"runtime,omitempty"`
	HasDeadLetterConfig bool        `json:"has_dead_letter_config"`
}

// captureLambda lists every function (ListFunctions, all pages) and reads the
// lifecycle fields from one GetFunction per function: ListFunctions returns
// none of them.
func captureLambda(ctx context.Context, cfg aws.Config) (any, error) {
	client := lambda.NewFromConfig(cfg)

	var functions []lambdaFunction
	paginator := lambda.NewListFunctionsPaginator(client, &lambda.ListFunctionsInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, f := range out.Functions {
			fn := lambdaFunction{
				FunctionName:        aws.ToString(f.FunctionName),
				FunctionArn:         aws.ToString(f.FunctionArn),
				Runtime:             string(f.Runtime),
				HasDeadLetterConfig: f.DeadLetterConfig != nil,
			}
			got, err := client.GetFunction(ctx, &lambda.GetFunctionInput{FunctionName: f.FunctionName})
			fn.GetFunction = readOf(err)
			if got != nil && got.Configuration != nil {
				c := got.Configuration
				fn.State = string(c.State)
				fn.StateReason = aws.ToString(c.StateReason)
				fn.StateReasonCode = string(c.StateReasonCode)
				fn.LastUpdateStatus = string(c.LastUpdateStatus)
				fn.LastUpdateStatusReason = aws.ToString(c.LastUpdateStatusReason)
				fn.LastUpdateStatusReasonCode = string(c.LastUpdateStatusReasonCode)
			}
			functions = append(functions, fn)
		}
	}

	return lambdaData{Functions: functions}, nil
}

// ---------------------------------------------------------------------------
// ecs
// ---------------------------------------------------------------------------

type ecsData struct {
	Clusters []ecsCluster `json:"clusters"`
}

type ecsCluster struct {
	ClusterName string `json:"cluster_name"`
	ClusterArn  string `json:"cluster_arn"`
	Status      string `json:"status,omitempty"`
	readOutcome
	Statistics *ecsClusterStatistics `json:"statistics,omitempty"`
}

// ecsClusterStatistics is the DescribeClusters(include=STATISTICS) answer
// the cluster's task-count findings are judged on.
type ecsClusterStatistics struct {
	readOutcome
	PendingTasksCount                 int32 `json:"pending_tasks_count"`
	RunningTasksCount                 int32 `json:"running_tasks_count"`
	RegisteredContainerInstancesCount int32 `json:"registered_container_instances_count"`
	ActiveServicesCount               int32 `json:"active_services_count"`
}

// ecsDescribeClusters describes clusters 100 per call (the DescribeClusters
// limit); a cluster reported MISSING was deleted after the list.
func ecsDescribeClusters(ctx context.Context, client *ecs.Client, ids []string, include []ecstypes.ClusterField, idOf func(ecstypes.Cluster) string) []listedRow[ecstypes.Cluster] {
	return describeListed(ids, 100, func(batch []string) (map[string]ecstypes.Cluster, []string, error) {
		out, err := client.DescribeClusters(ctx, &ecs.DescribeClustersInput{Clusters: batch, Include: include})
		if err != nil {
			return nil, nil, err
		}
		found := map[string]ecstypes.Cluster{}
		for _, c := range out.Clusters {
			found[idOf(c)] = c
		}
		return found, ecsMissing(out.Failures), nil
	})
}

func ecsMissing(failures []ecstypes.Failure) []string {
	var gone []string
	for _, f := range failures {
		if aws.ToString(f.Reason) == "MISSING" {
			gone = append(gone, aws.ToString(f.Arn))
		}
	}
	return gone
}

// The task counts come from a second DescribeClusters, by name with
// STATISTICS: that is the answer the cluster findings judge.
func captureECS(ctx context.Context, cfg aws.Config) (any, error) {
	client := ecs.NewFromConfig(cfg)

	arns, err := listECSClusterArns(ctx, cfg)
	if err != nil {
		return nil, err
	}

	clusterArn := func(c ecstypes.Cluster) string { return aws.ToString(c.ClusterArn) }
	rows := ecsDescribeClusters(ctx, client, arns, []ecstypes.ClusterField{ecstypes.ClusterFieldConfigurations, ecstypes.ClusterFieldTags}, clusterArn)
	clusters := make([]ecsCluster, 0, len(rows))
	var names []string
	for _, row := range rows {
		c := ecsCluster{ClusterArn: row.id, readOutcome: row.read}
		if row.item != nil {
			c.ClusterName = aws.ToString(row.item.ClusterName)
			c.Status = aws.ToString(row.item.Status)
			if c.ClusterName != "" {
				names = append(names, c.ClusterName)
			}
		}
		clusters = append(clusters, c)
	}

	clusterName := func(c ecstypes.Cluster) string { return aws.ToString(c.ClusterName) }
	stats := map[string]listedRow[ecstypes.Cluster]{}
	for _, row := range ecsDescribeClusters(ctx, client, names, []ecstypes.ClusterField{ecstypes.ClusterFieldStatistics}, clusterName) {
		stats[row.id] = row
	}
	for i := range clusters {
		row, ok := stats[clusters[i].ClusterName]
		if !ok {
			continue
		}
		st := &ecsClusterStatistics{readOutcome: row.read}
		if c := row.item; c != nil {
			st.PendingTasksCount = c.PendingTasksCount
			st.RunningTasksCount = c.RunningTasksCount
			st.RegisteredContainerInstancesCount = c.RegisteredContainerInstancesCount
			st.ActiveServicesCount = c.ActiveServicesCount
		}
		clusters[i].Statistics = st
	}

	return ecsData{Clusters: clusters}, nil
}

// ---------------------------------------------------------------------------
// ecs-svc
// ---------------------------------------------------------------------------

type ecsSvcData struct {
	Services []ecsSvcService `json:"services"`
	partialRead
}

type ecsSvcDeployment struct {
	RolloutState string `json:"rollout_state,omitempty"`
	Status       string `json:"status,omitempty"`
}

type ecsSvcEvent struct {
	Message   string `json:"message,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

type ecsSvcService struct {
	ServiceName string `json:"service_name"`
	ServiceArn  string `json:"service_arn"`
	ClusterArn  string `json:"cluster_arn"`
	readOutcome
	Status                     string             `json:"status,omitempty"`
	RunningCount               int32              `json:"running_count"`
	DesiredCount               int32              `json:"desired_count"`
	Deployments                []ecsSvcDeployment `json:"deployments,omitempty"`
	Events                     []ecsSvcEvent      `json:"events,omitempty"`
	DeploymentCircuitBreakerOn bool               `json:"deployment_circuit_breaker_on"`
}

// DescribeServices takes at most 10 services per call.
func ecsSvcCluster(ctx context.Context, cfg aws.Config, clusterArn string, d *ecsSvcData) error {
	client := ecs.NewFromConfig(cfg)
	svcArns, listErr := pageAll(ctx, &ecs.ListServicesInput{Cluster: aws.String(clusterArn)},
		func(ctx context.Context, in *ecs.ListServicesInput) (*ecs.ListServicesOutput, error) {
			return client.ListServices(ctx, in)
		},
		func(out *ecs.ListServicesOutput) []string { return out.ServiceArns }, allPages)
	rows := describeListed(svcArns, 10, func(batch []string) (map[string]ecstypes.Service, []string, error) {
		out, err := client.DescribeServices(ctx, &ecs.DescribeServicesInput{
			Cluster:  aws.String(clusterArn),
			Services: batch,
			Include:  []ecstypes.ServiceField{ecstypes.ServiceFieldTags},
		})
		if err != nil {
			return nil, nil, err
		}
		found := map[string]ecstypes.Service{}
		for _, s := range out.Services {
			found[aws.ToString(s.ServiceArn)] = s
		}
		return found, ecsMissing(out.Failures), nil
	})
	for _, row := range rows {
		svc := ecsSvcService{ServiceArn: row.id, ClusterArn: clusterArn, readOutcome: row.read}
		if s := row.item; s != nil {
			svc.ServiceName = aws.ToString(s.ServiceName)
			svc.ClusterArn = aws.ToString(s.ClusterArn)
			svc.Status = aws.ToString(s.Status)
			svc.RunningCount = s.RunningCount
			svc.DesiredCount = s.DesiredCount
			for _, dep := range s.Deployments {
				svc.Deployments = append(svc.Deployments, ecsSvcDeployment{
					RolloutState: string(dep.RolloutState),
					Status:       aws.ToString(dep.Status),
				})
			}
			if s.DeploymentConfiguration != nil && s.DeploymentConfiguration.DeploymentCircuitBreaker != nil {
				svc.DeploymentCircuitBreakerOn = s.DeploymentConfiguration.DeploymentCircuitBreaker.Enable
			}
			for _, e := range s.Events {
				svc.Events = append(svc.Events, ecsSvcEvent{
					Message:   aws.ToString(e.Message),
					CreatedAt: snapFormatTime(e.CreatedAt),
				})
			}
		}
		d.Services = append(d.Services, svc)
	}
	return listErr
}

// ---------------------------------------------------------------------------
// ecs-task
// ---------------------------------------------------------------------------

type ecsTaskData struct {
	Tasks []ecsTaskTask `json:"tasks"`
	partialRead
	definitions map[string]*ecsTaskDefinition
}

type ecsTaskContainer struct {
	Name      string `json:"name,omitempty"`
	ExitCode  *int32 `json:"exit_code,omitempty"`
	Essential bool   `json:"essential"`
	Reason    string `json:"reason,omitempty"`
}

type ecsTaskTask struct {
	TaskArn    string `json:"task_arn"`
	ClusterArn string `json:"cluster_arn"`
	readOutcome
	TaskDefinition    *readOutcome       `json:"task_definition,omitempty"`
	Group             string             `json:"group,omitempty"`
	LastStatus        string             `json:"last_status,omitempty"`
	StopCode          string             `json:"stop_code,omitempty"`
	StoppedReason     string             `json:"stopped_reason,omitempty"`
	HealthStatus      string             `json:"health_status,omitempty"`
	TaskDefinitionArn string             `json:"task_definition_arn,omitempty"`
	Containers        []ecsTaskContainer `json:"containers,omitempty"`
}

// DescribeTasks takes at most 100 tasks per call. Wave 2 judges a stopped
// essential container, and Essential is stated on the task definition only.
func ecsTaskCluster(ctx context.Context, cfg aws.Config, clusterArn string, d *ecsTaskData) error {
	client := ecs.NewFromConfig(cfg)
	if d.definitions == nil {
		d.definitions = map[string]*ecsTaskDefinition{}
	}
	taskArns, listErr := pageAll(ctx, &ecs.ListTasksInput{Cluster: aws.String(clusterArn)},
		func(ctx context.Context, in *ecs.ListTasksInput) (*ecs.ListTasksOutput, error) {
			return client.ListTasks(ctx, in)
		},
		func(out *ecs.ListTasksOutput) []string { return out.TaskArns }, allPages)
	rows := describeListed(taskArns, 100, func(batch []string) (map[string]ecstypes.Task, []string, error) {
		out, err := client.DescribeTasks(ctx, &ecs.DescribeTasksInput{
			Cluster: aws.String(clusterArn),
			Tasks:   batch,
			Include: []ecstypes.TaskField{ecstypes.TaskFieldTags},
		})
		if err != nil {
			return nil, nil, err
		}
		found := map[string]ecstypes.Task{}
		for _, t := range out.Tasks {
			found[aws.ToString(t.TaskArn)] = t
		}
		return found, ecsMissing(out.Failures), nil
	})
	for _, row := range rows {
		task := ecsTaskTask{TaskArn: row.id, ClusterArn: clusterArn, readOutcome: row.read}
		t := row.item
		if t == nil {
			d.Tasks = append(d.Tasks, task)
			continue
		}
		taskDefArn := aws.ToString(t.TaskDefinitionArn)
		def, ok := d.definitions[taskDefArn]
		if !ok && taskDefArn != "" {
			def = fetchEssentialMap(ctx, client, taskDefArn)
			d.definitions[taskDefArn] = def
		}
		task.ClusterArn = aws.ToString(t.ClusterArn)
		task.Group = aws.ToString(t.Group)
		task.LastStatus = aws.ToString(t.LastStatus)
		task.StopCode = string(t.StopCode)
		task.StoppedReason = aws.ToString(t.StoppedReason)
		task.HealthStatus = string(t.HealthStatus)
		task.TaskDefinitionArn = taskDefArn
		var essential map[string]bool
		if def != nil {
			task.TaskDefinition = &def.readOutcome
			essential = def.essential
		}
		for _, c := range t.Containers {
			name := aws.ToString(c.Name)
			task.Containers = append(task.Containers, ecsTaskContainer{
				Name:      name,
				ExitCode:  c.ExitCode,
				Essential: essential[name],
				Reason:    aws.ToString(c.Reason),
			})
		}
		d.Tasks = append(d.Tasks, task)
	}
	return listErr
}

type ecsTaskDefinition struct {
	readOutcome
	essential map[string]bool
}

func fetchEssentialMap(ctx context.Context, client *ecs.Client, taskDefArn string) *ecsTaskDefinition {
	out, err := client.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskDefArn),
	})
	def := &ecsTaskDefinition{readOutcome: readOf(err), essential: map[string]bool{}}
	if out == nil || out.TaskDefinition == nil {
		return def
	}
	for _, cd := range out.TaskDefinition.ContainerDefinitions {
		def.essential[aws.ToString(cd.Name)] = aws.ToBool(cd.Essential)
	}
	return def
}

func listECSClusterArns(ctx context.Context, cfg aws.Config) ([]string, error) {
	client := ecs.NewFromConfig(cfg)
	return pageAll(ctx, &ecs.ListClustersInput{},
		func(ctx context.Context, in *ecs.ListClustersInput) (*ecs.ListClustersOutput, error) {
			return client.ListClusters(ctx, in)
		},
		func(out *ecs.ListClustersOutput) []string { return out.ClusterArns }, allPages)
}

// ---------------------------------------------------------------------------
// ecr
// ---------------------------------------------------------------------------

type ecrData struct {
	Repositories []ecrRepository `json:"repositories"`
}

type ecrRepository struct {
	RepositoryName string      `json:"repository_name"`
	RepositoryArn  string      `json:"repository_arn"`
	ScanOnPush     bool        `json:"scan_on_push"`
	ImagesRead     readOutcome `json:"images_read"`
	Images         []ecrImage  `json:"images"`
	// The totals are absent when the newest image's scan results could not be
	// read, or its scan holds no findings to count (never scanned, or a status
	// other than COMPLETE or ACTIVE): neither is a proven zero.
	CriticalTotal *int32 `json:"critical_total,omitempty"`
	HighTotal     *int32 `json:"high_total,omitempty"`
}

// ecrImage is one image the app inspects and its scan counts. ScanErrorCode
// is set when its scan results could not be read, so its counts are unknown
// rather than zero. ScanStatus is the scan's API_ImageScanStatus status, ""
// when the image was never scanned or the answer named none.
type ecrImage struct {
	ImageDigest           string           `json:"image_digest"`
	ImagePushedAt         string           `json:"image_pushed_at,omitempty"`
	ScanStatus            string           `json:"scan_status,omitempty"`
	FindingSeverityCounts map[string]int32 `json:"finding_severity_counts,omitempty"`
	ScanErrorCode         string           `json:"scan_error_code,omitempty"`
}

// captureECR lists every repository (DescribeRepositories, all pages) and
// captures the Wave 1 scanOnPush flag plus the scan evidence the Wave 2
// repository pass reads, per docs/resources/ecr.md.
func captureECR(ctx context.Context, cfg aws.Config) (any, error) {
	client := ecr.NewFromConfig(cfg)

	var repos []ecrtypes.Repository
	paginator := ecr.NewDescribeRepositoriesPaginator(client, &ecr.DescribeRepositoriesInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		repos = append(repos, out.Repositories...)
	}

	var repositories []ecrRepository
	for _, r := range repos {
		repo := ecrRepository{
			RepositoryName: aws.ToString(r.RepositoryName),
			RepositoryArn:  aws.ToString(r.RepositoryArn),
			ScanOnPush:     r.ImageScanningConfiguration != nil && r.ImageScanningConfiguration.ScanOnPush,
		}
		captureECRImages(ctx, client, &repo)
		repositories = append(repositories, repo)
	}

	return ecrData{Repositories: repositories}, nil
}

// captureECRImages records the image the app inspects: the newest by
// imagePushedAt over every DescribeImages page, and its severity counts
// tallied from findings and enhancedFindings over every
// DescribeImageScanFindings page.
func captureECRImages(ctx context.Context, client *ecr.Client, repo *ecrRepository) {
	var latest *ecrtypes.ImageDetail
	images := ecr.NewDescribeImagesPaginator(client, &ecr.DescribeImagesInput{RepositoryName: aws.String(repo.RepositoryName)})
	for images.HasMorePages() {
		out, err := images.NextPage(ctx)
		repo.ImagesRead = readOf(err)
		if err != nil {
			return
		}
		for i := range out.ImageDetails {
			at := out.ImageDetails[i].ImagePushedAt
			if latest == nil || at != nil && (latest.ImagePushedAt == nil || at.After(*latest.ImagePushedAt)) {
				latest = &out.ImageDetails[i]
			}
		}
	}
	var critical, high int32
	if latest == nil {
		repo.CriticalTotal, repo.HighTotal = &critical, &high
		return
	}
	img := ecrImage{
		ImageDigest:   aws.ToString(latest.ImageDigest),
		ImagePushedAt: snapFormatTime(latest.ImagePushedAt),
	}
	scanned := latest.ImageScanFindingsSummary != nil
	if scanned {
		img.FindingSeverityCounts = latest.ImageScanFindingsSummary.FindingSeverityCounts
		if latest.ImageScanStatus != nil {
			img.ScanStatus = string(latest.ImageScanStatus.Status)
		}
	} else {
		// Basic Scanning leaves DescribeImages without a scan summary;
		// the counts are only in DescribeImageScanFindings.
		img.FindingSeverityCounts, img.ScanStatus, img.ScanErrorCode = captureECRScanCounts(ctx, client, repo.RepositoryName, latest.ImageDigest)
		scanned = img.FindingSeverityCounts != nil
	}
	repo.Images = []ecrImage{img}
	st := ecrtypes.ScanStatus(img.ScanStatus)
	if img.ScanErrorCode != "" || !scanned || st != "" && st != ecrtypes.ScanStatusComplete && st != ecrtypes.ScanStatusActive {
		return
	}
	critical = img.FindingSeverityCounts[string(ecrtypes.FindingSeverityCritical)]
	high = img.FindingSeverityCounts[string(ecrtypes.FindingSeverityHigh)]
	repo.CriticalTotal, repo.HighTotal = &critical, &high
}

// captureECRScanCounts tallies one image's scan findings by severity over
// every page and reads the scan's status; an image never scanned, or a scan
// that returned no findings, has no counts, and a failed read names its error
// code.
func captureECRScanCounts(ctx context.Context, client *ecr.Client, repoName string, digest *string) (counts map[string]int32, status, errCode string) {
	pages := ecr.NewDescribeImageScanFindingsPaginator(client, &ecr.DescribeImageScanFindingsInput{
		RepositoryName: aws.String(repoName),
		ImageId:        &ecrtypes.ImageIdentifier{ImageDigest: digest},
	})
	for pages.HasMorePages() {
		out, err := pages.NextPage(ctx)
		if err != nil {
			_, code := outcomeOf(err)
			return nil, "", code
		}
		if out.ImageScanStatus != nil {
			status = string(out.ImageScanStatus.Status)
		}
		f := out.ImageScanFindings
		if f == nil {
			continue
		}
		if counts == nil {
			counts = map[string]int32{}
		}
		for _, finding := range f.Findings {
			if finding.Severity != "" {
				counts[string(finding.Severity)]++
			}
		}
		for _, finding := range f.EnhancedFindings {
			if sev := aws.ToString(finding.Severity); sev != "" {
				counts[sev]++
			}
		}
	}
	return counts, status, ""
}

// ---------------------------------------------------------------------------
// eks
// ---------------------------------------------------------------------------

type eksData struct {
	Clusters []eksCluster `json:"clusters"`
	// Versions is what DescribeClusterVersions says of every Kubernetes
	// version, keyed by version: the cluster's support verdict is read from it.
	Versions eksVersions `json:"versions"`
}

type eksVersions struct {
	readOutcome
	Status               map[string]string `json:"status,omitempty"`
	EndOfStandardSupport map[string]string `json:"end_of_standard_support,omitempty"`
}

type eksClusterIssue struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type eksCluster struct {
	Name string `json:"name"`
	readOutcome
	Arn     string            `json:"arn,omitempty"`
	Status  string            `json:"status,omitempty"`
	Version string            `json:"version,omitempty"`
	Issues  []eksClusterIssue `json:"issues,omitempty"`
}

// ListClusters returns names only, so each cluster is described.
func captureEKS(ctx context.Context, cfg aws.Config) (any, error) {
	client := eks.NewFromConfig(cfg)

	var names []string
	paginator := eks.NewListClustersPaginator(client, &eks.ListClustersInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		names = append(names, out.Clusters...)
	}

	rows := describeListed(names, 1, func(batch []string) (map[string]ekstypes.Cluster, []string, error) {
		out, err := client.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: aws.String(batch[0])})
		if err != nil {
			return nil, nil, err
		}
		return describedAs(batch[0], out.Cluster), nil, nil
	})
	clusters := make([]eksCluster, 0, len(rows))
	for _, row := range rows {
		cluster := eksCluster{Name: row.id, readOutcome: row.read}
		if c := row.item; c != nil {
			cluster.Arn = aws.ToString(c.Arn)
			cluster.Status = string(c.Status)
			cluster.Version = aws.ToString(c.Version)
			if c.Health != nil {
				for _, issue := range c.Health.Issues {
					cluster.Issues = append(cluster.Issues, eksClusterIssue{
						Code:    string(issue.Code),
						Message: aws.ToString(issue.Message),
					})
				}
			}
		}
		clusters = append(clusters, cluster)
	}

	return eksData{Clusters: clusters, Versions: captureEKSVersions(ctx, client)}, nil
}

func captureEKSVersions(ctx context.Context, client *eks.Client) eksVersions {
	v := eksVersions{Status: map[string]string{}, EndOfStandardSupport: map[string]string{}}
	paginator := eks.NewDescribeClusterVersionsPaginator(client, &eks.DescribeClusterVersionsInput{IncludeAll: aws.Bool(true)})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			v.readOutcome = readOf(err)
			return v
		}
		for _, cv := range out.ClusterVersions {
			version := aws.ToString(cv.ClusterVersion)
			v.Status[version] = string(cv.VersionStatus)
			if cv.EndOfStandardSupportDate != nil {
				v.EndOfStandardSupport[version] = snapFormatTime(cv.EndOfStandardSupportDate)
			}
		}
	}
	v.readOutcome = readOf(nil)
	return v
}

// ---------------------------------------------------------------------------
// ng (EKS node groups)
// ---------------------------------------------------------------------------

type ngData struct {
	Nodegroups []ngNodegroup `json:"nodegroups"`
	partialRead
}

type ngIssue struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type ngNodegroup struct {
	NodegroupName string `json:"nodegroup_name"`
	ClusterName   string `json:"cluster_name"`
	readOutcome
	Status         string            `json:"status,omitempty"`
	Issues         []ngIssue         `json:"issues,omitempty"`
	LaunchTemplate *ngLaunchTemplate `json:"launch_template,omitempty"`
}

type ngLaunchTemplate struct {
	readOutcome
	Id      string `json:"id"`
	Version string `json:"version"`
	ImageId string `json:"image_id,omitempty"`
}

func listEKSClusterNames(ctx context.Context, cfg aws.Config) ([]string, error) {
	client := eks.NewFromConfig(cfg)
	return pageAll(ctx, &eks.ListClustersInput{},
		func(ctx context.Context, in *eks.ListClustersInput) (*eks.ListClustersOutput, error) {
			return client.ListClusters(ctx, in)
		},
		func(out *eks.ListClustersOutput) []string { return out.Clusters }, allPages)
}

func ngCluster(ctx context.Context, cfg aws.Config, clusterName string, d *ngData) error {
	eksClient := eks.NewFromConfig(cfg)
	ec2Client := ec2.NewFromConfig(cfg)
	ngNames, listErr := pageAll(ctx, &eks.ListNodegroupsInput{ClusterName: aws.String(clusterName)},
		func(ctx context.Context, in *eks.ListNodegroupsInput) (*eks.ListNodegroupsOutput, error) {
			return eksClient.ListNodegroups(ctx, in)
		},
		func(out *eks.ListNodegroupsOutput) []string { return out.Nodegroups }, allPages)
	rows := describeListed(ngNames, 1, func(batch []string) (map[string]ekstypes.Nodegroup, []string, error) {
		out, err := eksClient.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
			ClusterName:   aws.String(clusterName),
			NodegroupName: aws.String(batch[0]),
		})
		if err != nil {
			return nil, nil, err
		}
		if out.Nodegroup == nil {
			return nil, nil, nil
		}
		return describedAs(aws.ToString(out.Nodegroup.NodegroupName), out.Nodegroup), nil, nil
	})
	for _, row := range rows {
		nodegroup := ngNodegroup{NodegroupName: row.id, ClusterName: clusterName, readOutcome: row.read}
		if n := row.item; n != nil {
			nodegroup.ClusterName = aws.ToString(n.ClusterName)
			nodegroup.Status = string(n.Status)
			if n.Health != nil {
				for _, issue := range n.Health.Issues {
					nodegroup.Issues = append(nodegroup.Issues, ngIssue{
						Code:    string(issue.Code),
						Message: aws.ToString(issue.Message),
					})
				}
			}
			if lt := n.LaunchTemplate; lt != nil && aws.ToString(lt.Id) != "" {
				nodegroup.LaunchTemplate = captureNGLaunchTemplate(ctx, ec2Client, aws.ToString(lt.Id), cmp.Or(aws.ToString(lt.Version), "$Default"))
			}
		}
		d.Nodegroups = append(d.Nodegroups, nodegroup)
	}
	return listErr
}

func captureNGLaunchTemplate(ctx context.Context, client *ec2.Client, id, version string) *ngLaunchTemplate {
	got, read := readOne(func() (*ec2types.LaunchTemplateVersion, error) {
		versions, err := pageAll(ctx, &ec2.DescribeLaunchTemplateVersionsInput{LaunchTemplateId: aws.String(id), Versions: []string{version}},
			func(ctx context.Context, in *ec2.DescribeLaunchTemplateVersionsInput) (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
				return client.DescribeLaunchTemplateVersions(ctx, in)
			},
			func(out *ec2.DescribeLaunchTemplateVersionsOutput) []ec2types.LaunchTemplateVersion {
				return out.LaunchTemplateVersions
			}, allPages)
		if err != nil || len(versions) == 0 {
			return nil, err
		}
		return &versions[0], nil
	})
	lt := &ngLaunchTemplate{Id: id, Version: version, readOutcome: read}
	if got != nil && got.LaunchTemplateData != nil {
		lt.ImageId = aws.ToString(got.LaunchTemplateData.ImageId)
	}
	return lt
}

// ---------------------------------------------------------------------------
// asg
// ---------------------------------------------------------------------------

type asgData struct {
	Groups []asgGroup `json:"groups"`
}

type asgInstance struct {
	InstanceId     string `json:"instance_id"`
	HealthStatus   string `json:"health_status,omitempty"`
	LifecycleState string `json:"lifecycle_state,omitempty"`
}

type asgGroup struct {
	AutoScalingGroupName string             `json:"auto_scaling_group_name"`
	Status               string             `json:"status,omitempty"`
	MinSize              int32              `json:"min_size"`
	MaxSize              int32              `json:"max_size"`
	Instances            []asgInstance      `json:"instances,omitempty"`
	SuspendedProcesses   []string           `json:"suspended_processes,omitempty"`
	LatestActivity       asgScalingActivity `json:"latest_activity"`
}

// asgScalingActivity captures the DescribeScalingActivities(MaxRecords=1)
// outcome for the most recent activity.
type asgScalingActivity struct {
	Outcome       string `json:"outcome"`
	ErrorCode     string `json:"error_code,omitempty"`
	StatusCode    string `json:"status_code,omitempty"`
	StatusMessage string `json:"status_message,omitempty"`
}

// captureASG lists every group (DescribeAutoScalingGroups, all pages) and
// captures Wave 1 fields plus the Wave 2 latest-scaling-activity per group
// per docs/resources/asg.md.
func captureASG(ctx context.Context, cfg aws.Config) (any, error) {
	client := autoscaling.NewFromConfig(cfg)

	var groups []autoscalingtypes.AutoScalingGroup
	paginator := autoscaling.NewDescribeAutoScalingGroupsPaginator(client, &autoscaling.DescribeAutoScalingGroupsInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		groups = append(groups, out.AutoScalingGroups...)
	}

	var result []asgGroup
	for _, g := range groups {
		group := asgGroup{
			AutoScalingGroupName: aws.ToString(g.AutoScalingGroupName),
			Status:               aws.ToString(g.Status),
			MinSize:              aws.ToInt32(g.MinSize),
			MaxSize:              aws.ToInt32(g.MaxSize),
		}
		for _, inst := range g.Instances {
			group.Instances = append(group.Instances, asgInstance{
				InstanceId:     aws.ToString(inst.InstanceId),
				HealthStatus:   aws.ToString(inst.HealthStatus),
				LifecycleState: string(inst.LifecycleState),
			})
		}
		for _, sp := range g.SuspendedProcesses {
			group.SuspendedProcesses = append(group.SuspendedProcesses, aws.ToString(sp.ProcessName))
		}
		group.LatestActivity = captureLatestScalingActivity(ctx, client, aws.ToString(g.AutoScalingGroupName))
		result = append(result, group)
	}

	return asgData{Groups: result}, nil
}

func captureLatestScalingActivity(ctx context.Context, client *autoscaling.Client, name string) asgScalingActivity {
	activities, err := pageAll(ctx, &autoscaling.DescribeScalingActivitiesInput{
		AutoScalingGroupName: aws.String(name),
		MaxRecords:           aws.Int32(1),
	},
		func(ctx context.Context, in *autoscaling.DescribeScalingActivitiesInput) (*autoscaling.DescribeScalingActivitiesOutput, error) {
			return client.DescribeScalingActivities(ctx, in)
		},
		func(out *autoscaling.DescribeScalingActivitiesOutput) []autoscalingtypes.Activity {
			return out.Activities
		}, onePage)
	if err != nil {
		o, code := outcomeOf(err)
		return asgScalingActivity{Outcome: o, ErrorCode: code}
	}
	if len(activities) == 0 {
		return asgScalingActivity{Outcome: "none"}
	}
	a := activities[0]
	return asgScalingActivity{
		Outcome:       "found",
		StatusCode:    string(a.StatusCode),
		StatusMessage: aws.ToString(a.StatusMessage),
	}
}

// ---------------------------------------------------------------------------
// eb (Elastic Beanstalk)
// ---------------------------------------------------------------------------

type ebData struct {
	Environments []ebEnvironment `json:"environments"`
}

type ebEnvironment struct {
	EnvironmentName string   `json:"environment_name"`
	EnvironmentId   string   `json:"environment_id"`
	Health          string   `json:"health,omitempty"`
	Status          string   `json:"status,omitempty"`
	Causes          []string `json:"causes,omitempty"`
	CausesOutcome   string   `json:"causes_outcome"`
	CausesErrorCode string   `json:"causes_error_code,omitempty"`
}

// captureEB lists every environment (DescribeEnvironments, all pages) and
// captures Wave 1 Health/Status plus the Wave 2 DescribeEnvironmentHealth
// Causes[] per docs/resources/eb.md.
func captureEB(ctx context.Context, cfg aws.Config) (any, error) {
	client := elasticbeanstalk.NewFromConfig(cfg)

	envs, err := pageAll(ctx, &elasticbeanstalk.DescribeEnvironmentsInput{},
		func(ctx context.Context, in *elasticbeanstalk.DescribeEnvironmentsInput) (*elasticbeanstalk.DescribeEnvironmentsOutput, error) {
			return client.DescribeEnvironments(ctx, in)
		},
		func(out *elasticbeanstalk.DescribeEnvironmentsOutput) []ebtypes.EnvironmentDescription {
			return out.Environments
		}, allPages)
	if err != nil {
		return nil, err
	}

	var result []ebEnvironment
	for _, e := range envs {
		env := ebEnvironment{
			EnvironmentName: aws.ToString(e.EnvironmentName),
			EnvironmentId:   aws.ToString(e.EnvironmentId),
			Health:          string(e.Health),
			Status:          string(e.Status),
		}
		// A terminated environment keeps its name, which a live one may take
		// again; the id names this environment alone.
		in := &elasticbeanstalk.DescribeEnvironmentHealthInput{
			AttributeNames: []ebtypes.EnvironmentHealthAttribute{ebtypes.EnvironmentHealthAttributeCauses},
		}
		if aws.ToString(e.EnvironmentId) != "" {
			in.EnvironmentId = e.EnvironmentId
		} else {
			in.EnvironmentName = e.EnvironmentName
		}
		healthOut, err := client.DescribeEnvironmentHealth(ctx, in)
		env.CausesOutcome, env.CausesErrorCode = outcomeOf(err)
		if healthOut != nil {
			env.Causes = healthOut.Causes
		}
		result = append(result, env)
	}

	return ebData{Environments: result}, nil
}

// ---------------------------------------------------------------------------
// elb (Application/Network Load Balancers, ELBv2)
// ---------------------------------------------------------------------------

type elbData struct {
	LoadBalancers []elbLoadBalancer `json:"load_balancers"`
}

type elbLoadBalancer struct {
	LoadBalancerName string `json:"load_balancer_name"`
	LoadBalancerArn  string `json:"load_balancer_arn"`
	StateCode        string `json:"state_code,omitempty"`
	StateReason      string `json:"state_reason,omitempty"`
}

// captureELB lists every ELBv2 load balancer (DescribeLoadBalancers, all
// pages) and captures the Wave 1 State.Code/State.Reason fields per
// docs/resources/elb.md.
func captureELB(ctx context.Context, cfg aws.Config) (any, error) {
	client := elasticloadbalancingv2.NewFromConfig(cfg)

	var lbs []elbv2types.LoadBalancer
	paginator := elasticloadbalancingv2.NewDescribeLoadBalancersPaginator(client, &elasticloadbalancingv2.DescribeLoadBalancersInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		lbs = append(lbs, out.LoadBalancers...)
	}

	var result []elbLoadBalancer
	for _, lb := range lbs {
		item := elbLoadBalancer{
			LoadBalancerName: aws.ToString(lb.LoadBalancerName),
			LoadBalancerArn:  aws.ToString(lb.LoadBalancerArn),
		}
		if lb.State != nil {
			item.StateCode = string(lb.State.Code)
			item.StateReason = aws.ToString(lb.State.Reason)
		}
		result = append(result, item)
	}

	return elbData{LoadBalancers: result}, nil
}

// ---------------------------------------------------------------------------
// tg (Target Groups)
// ---------------------------------------------------------------------------

type tgData struct {
	TargetGroups []tgTargetGroup `json:"target_groups"`
}

type tgTargetHealth struct {
	TargetId string `json:"target_id"`
	State    string `json:"state,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

type tgTargetGroup struct {
	TargetGroupName     string           `json:"target_group_name"`
	TargetGroupArn      string           `json:"target_group_arn"`
	TargetType          string           `json:"target_type,omitempty"`
	LoadBalancerArns    []string         `json:"load_balancer_arns,omitempty"`
	TargetHealthOutcome string           `json:"target_health_outcome"`
	TargetHealthError   string           `json:"target_health_error,omitempty"`
	Targets             []tgTargetHealth `json:"targets,omitempty"`
}

// captureTG lists every target group (DescribeTargetGroups, all pages) and
// captures the Wave 1 orphan signal (LoadBalancerArns) plus the Wave 2
// DescribeTargetHealth raw per-target state per docs/resources/tg.md.
func captureTG(ctx context.Context, cfg aws.Config) (any, error) {
	client := elasticloadbalancingv2.NewFromConfig(cfg)

	var groups []elbv2types.TargetGroup
	paginator := elasticloadbalancingv2.NewDescribeTargetGroupsPaginator(client, &elasticloadbalancingv2.DescribeTargetGroupsInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		groups = append(groups, out.TargetGroups...)
	}

	var result []tgTargetGroup
	for _, g := range groups {
		item := tgTargetGroup{
			TargetGroupName:  aws.ToString(g.TargetGroupName),
			TargetGroupArn:   aws.ToString(g.TargetGroupArn),
			TargetType:       string(g.TargetType),
			LoadBalancerArns: g.LoadBalancerArns,
		}

		healthOut, err := client.DescribeTargetHealth(ctx, &elasticloadbalancingv2.DescribeTargetHealthInput{
			TargetGroupArn: g.TargetGroupArn,
		})
		item.TargetHealthOutcome, item.TargetHealthError = outcomeOf(err)
		if healthOut != nil {
			for _, th := range healthOut.TargetHealthDescriptions {
				entry := tgTargetHealth{}
				if th.Target != nil {
					entry.TargetId = aws.ToString(th.Target.Id)
				}
				if th.TargetHealth != nil {
					entry.State = string(th.TargetHealth.State)
					entry.Reason = string(th.TargetHealth.Reason)
				}
				item.Targets = append(item.Targets, entry)
			}
		}

		result = append(result, item)
	}

	return tgData{TargetGroups: result}, nil
}
