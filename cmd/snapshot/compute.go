package main

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	autoscalingtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	smithy "github.com/aws/smithy-go"
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
	// GetFunctionErrorCode is set when the GetFunction read failed, so the
	// lifecycle fields are unknown rather than empty.
	GetFunctionErrorCode string `json:"get_function_error_code,omitempty"`
	Runtime              string `json:"runtime,omitempty"`
	HasDeadLetterConfig  bool   `json:"has_dead_letter_config"`
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
			switch {
			case err != nil:
				fn.GetFunctionErrorCode = apiErrorCode(err)
			case got.Configuration != nil:
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
	ClusterName                       string `json:"cluster_name"`
	ClusterArn                        string `json:"cluster_arn"`
	Status                            string `json:"status,omitempty"`
	PendingTasksCount                 int32  `json:"pending_tasks_count"`
	RunningTasksCount                 int32  `json:"running_tasks_count"`
	RegisteredContainerInstancesCount int32  `json:"registered_container_instances_count"`
	ActiveServicesCount               int32  `json:"active_services_count"`
}

// captureECS lists every cluster (ListClusters, all pages) then batches
// DescribeClusters(include=STATISTICS) to capture status + task/instance
// counts in one describe pass per docs/resources/ecs.md.
func captureECS(ctx context.Context, cfg aws.Config) (any, error) {
	client := ecs.NewFromConfig(cfg)

	var arns []string
	clusterPaginator := ecs.NewListClustersPaginator(client, &ecs.ListClustersInput{})
	for clusterPaginator.HasMorePages() {
		out, err := clusterPaginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		arns = append(arns, out.ClusterArns...)
	}

	var clusters []ecsCluster
	for i := 0; i < len(arns); i += 100 {
		end := min(i+100, len(arns))
		batch := arns[i:end]
		out, err := client.DescribeClusters(ctx, &ecs.DescribeClustersInput{
			Clusters: batch,
			Include:  []ecstypes.ClusterField{ecstypes.ClusterFieldStatistics},
		})
		if err != nil {
			return nil, err
		}
		for _, c := range out.Clusters {
			clusters = append(clusters, ecsCluster{
				ClusterName:                       aws.ToString(c.ClusterName),
				ClusterArn:                        aws.ToString(c.ClusterArn),
				Status:                            aws.ToString(c.Status),
				PendingTasksCount:                 c.PendingTasksCount,
				RunningTasksCount:                 c.RunningTasksCount,
				RegisteredContainerInstancesCount: c.RegisteredContainerInstancesCount,
				ActiveServicesCount:               c.ActiveServicesCount,
			})
		}
	}

	return ecsData{Clusters: clusters}, nil
}

// ---------------------------------------------------------------------------
// ecs-svc
// ---------------------------------------------------------------------------

type ecsSvcData struct {
	Services []ecsSvcService `json:"services"`
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
	ServiceName                string             `json:"service_name"`
	ServiceArn                 string             `json:"service_arn"`
	ClusterArn                 string             `json:"cluster_arn"`
	Status                     string             `json:"status,omitempty"`
	RunningCount               int32              `json:"running_count"`
	DesiredCount               int32              `json:"desired_count"`
	Deployments                []ecsSvcDeployment `json:"deployments,omitempty"`
	Events                     []ecsSvcEvent      `json:"events,omitempty"`
	DeploymentCircuitBreakerOn bool               `json:"deployment_circuit_breaker_on"`
}

// captureECSSvc lists every cluster's services via ListServices+DescribeServices
// (batched 10 per call, per docs/resources/ecs-svc.md — the same wire call
// carries both list and Wave 1/Wave 2 fields).
func captureECSSvc(ctx context.Context, cfg aws.Config) (any, error) {
	client := ecs.NewFromConfig(cfg)

	clusterArns, err := listAllECSClusterArns(ctx, client)
	if err != nil {
		return nil, err
	}

	var services []ecsSvcService
	for _, clusterArn := range clusterArns {
		svcArns, err := listAllServiceArns(ctx, client, clusterArn)
		if err != nil {
			return nil, err
		}
		for i := 0; i < len(svcArns); i += 10 {
			end := min(i+10, len(svcArns))
			batch := svcArns[i:end]
			out, err := client.DescribeServices(ctx, &ecs.DescribeServicesInput{
				Cluster:  aws.String(clusterArn),
				Services: batch,
			})
			if err != nil {
				return nil, err
			}
			for _, s := range out.Services {
				svc := ecsSvcService{
					ServiceName:  aws.ToString(s.ServiceName),
					ServiceArn:   aws.ToString(s.ServiceArn),
					ClusterArn:   aws.ToString(s.ClusterArn),
					Status:       aws.ToString(s.Status),
					RunningCount: s.RunningCount,
					DesiredCount: s.DesiredCount,
				}
				for _, d := range s.Deployments {
					svc.Deployments = append(svc.Deployments, ecsSvcDeployment{
						RolloutState: string(d.RolloutState),
						Status:       aws.ToString(d.Status),
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
				services = append(services, svc)
			}
		}
	}

	return ecsSvcData{Services: services}, nil
}

// ---------------------------------------------------------------------------
// ecs-task
// ---------------------------------------------------------------------------

type ecsTaskData struct {
	Tasks []ecsTaskTask `json:"tasks"`
}

type ecsTaskContainer struct {
	Name      string `json:"name,omitempty"`
	ExitCode  *int32 `json:"exit_code,omitempty"`
	Essential bool   `json:"essential"`
	Reason    string `json:"reason,omitempty"`
}

type ecsTaskTask struct {
	TaskArn           string             `json:"task_arn"`
	ClusterArn        string             `json:"cluster_arn"`
	Group             string             `json:"group,omitempty"`
	LastStatus        string             `json:"last_status,omitempty"`
	StopCode          string             `json:"stop_code,omitempty"`
	StoppedReason     string             `json:"stopped_reason,omitempty"`
	HealthStatus      string             `json:"health_status,omitempty"`
	TaskDefinitionArn string             `json:"task_definition_arn,omitempty"`
	Containers        []ecsTaskContainer `json:"containers,omitempty"`
}

// captureECSTask lists every cluster's tasks via ListTasks+DescribeTasks (up
// to 100 IDs per describe call per docs/resources/ecs-task.md). Container
// Essential is resolved per distinct TaskDefinitionArn via DescribeTaskDefinition,
// cached to collapse repeated revisions to one call, satisfying Wave 2's
// "essential=true + ExitCode!=0" signal.
func captureECSTask(ctx context.Context, cfg aws.Config) (any, error) {
	client := ecs.NewFromConfig(cfg)

	clusterArns, err := listAllECSClusterArns(ctx, client)
	if err != nil {
		return nil, err
	}

	essentialCache := map[string]map[string]bool{} // taskDefArn -> containerName -> essential

	var tasks []ecsTaskTask
	for _, clusterArn := range clusterArns {
		taskArns, err := listAllTaskArns(ctx, client, clusterArn)
		if err != nil {
			return nil, err
		}
		for i := 0; i < len(taskArns); i += 100 {
			end := min(i+100, len(taskArns))
			batch := taskArns[i:end]
			out, err := client.DescribeTasks(ctx, &ecs.DescribeTasksInput{
				Cluster: aws.String(clusterArn),
				Tasks:   batch,
			})
			if err != nil {
				return nil, err
			}
			for _, t := range out.Tasks {
				taskDefArn := aws.ToString(t.TaskDefinitionArn)
				essential, ok := essentialCache[taskDefArn]
				if !ok && taskDefArn != "" {
					essential = fetchEssentialMap(ctx, client, taskDefArn)
					essentialCache[taskDefArn] = essential
				}

				task := ecsTaskTask{
					TaskArn:           aws.ToString(t.TaskArn),
					ClusterArn:        aws.ToString(t.ClusterArn),
					Group:             aws.ToString(t.Group),
					LastStatus:        aws.ToString(t.LastStatus),
					StopCode:          string(t.StopCode),
					StoppedReason:     aws.ToString(t.StoppedReason),
					HealthStatus:      string(t.HealthStatus),
					TaskDefinitionArn: taskDefArn,
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
				tasks = append(tasks, task)
			}
		}
	}

	return ecsTaskData{Tasks: tasks}, nil
}

func fetchEssentialMap(ctx context.Context, client *ecs.Client, taskDefArn string) map[string]bool {
	out, err := client.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskDefArn),
	})
	if err != nil || out.TaskDefinition == nil {
		return map[string]bool{}
	}
	m := make(map[string]bool, len(out.TaskDefinition.ContainerDefinitions))
	for _, cd := range out.TaskDefinition.ContainerDefinitions {
		m[aws.ToString(cd.Name)] = aws.ToBool(cd.Essential)
	}
	return m
}

func listAllECSClusterArns(ctx context.Context, client *ecs.Client) ([]string, error) {
	var arns []string
	paginator := ecs.NewListClustersPaginator(client, &ecs.ListClustersInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		arns = append(arns, out.ClusterArns...)
	}
	return arns, nil
}

func listAllServiceArns(ctx context.Context, client *ecs.Client, clusterArn string) ([]string, error) {
	var arns []string
	paginator := ecs.NewListServicesPaginator(client, &ecs.ListServicesInput{
		Cluster: aws.String(clusterArn),
	})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		arns = append(arns, out.ServiceArns...)
	}
	return arns, nil
}

func listAllTaskArns(ctx context.Context, client *ecs.Client, clusterArn string) ([]string, error) {
	var arns []string
	paginator := ecs.NewListTasksPaginator(client, &ecs.ListTasksInput{
		Cluster: aws.String(clusterArn),
	})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		arns = append(arns, out.TaskArns...)
	}
	return arns, nil
}

// ---------------------------------------------------------------------------
// ecr
// ---------------------------------------------------------------------------

type ecrData struct {
	Repositories []ecrRepository `json:"repositories"`
}

type ecrRepository struct {
	RepositoryName string `json:"repository_name"`
	RepositoryArn  string `json:"repository_arn"`
	ScanOnPush     bool   `json:"scan_on_push"`
	// ImagesErrorCode is set when DescribeImages itself failed.
	ImagesErrorCode string     `json:"images_error_code,omitempty"`
	Images          []ecrImage `json:"images"`
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
		if err != nil {
			repo.ImagesErrorCode = apiErrorCode(err)
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
		var notScanned *ecrtypes.ScanNotFoundException
		switch {
		case errors.As(err, &notScanned):
			return nil, "", ""
		case err != nil:
			return nil, "", apiErrorCode(err)
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
}

type eksClusterIssue struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type eksCluster struct {
	Name   string            `json:"name"`
	Arn    string            `json:"arn"`
	Status string            `json:"status,omitempty"`
	Issues []eksClusterIssue `json:"issues,omitempty"`
}

// captureEKS lists every cluster name (ListClusters, all pages) then fans out
// DescribeCluster per cluster to capture Status + Health.Issues[] per
// docs/resources/eks.md (ListClusters returns names only).
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

	var clusters []eksCluster
	for _, name := range names {
		out, err := client.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: aws.String(name)})
		if err != nil {
			return nil, err
		}
		if out.Cluster == nil {
			continue
		}
		c := out.Cluster
		cluster := eksCluster{
			Name:   aws.ToString(c.Name),
			Arn:    aws.ToString(c.Arn),
			Status: string(c.Status),
		}
		if c.Health != nil {
			for _, issue := range c.Health.Issues {
				cluster.Issues = append(cluster.Issues, eksClusterIssue{
					Code:    string(issue.Code),
					Message: aws.ToString(issue.Message),
				})
			}
		}
		clusters = append(clusters, cluster)
	}

	return eksData{Clusters: clusters}, nil
}

// ---------------------------------------------------------------------------
// ng (EKS node groups)
// ---------------------------------------------------------------------------

type ngData struct {
	Nodegroups []ngNodegroup `json:"nodegroups"`
}

type ngIssue struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type ngNodegroup struct {
	NodegroupName string    `json:"nodegroup_name"`
	ClusterName   string    `json:"cluster_name"`
	Status        string    `json:"status,omitempty"`
	Issues        []ngIssue `json:"issues,omitempty"`
}

// captureNG lists every EKS cluster's node groups via ListNodegroups then
// fans out DescribeNodegroup per group to capture Status + Health.Issues[]
// per docs/resources/ng.md.
func captureNG(ctx context.Context, cfg aws.Config) (any, error) {
	eksClient := eks.NewFromConfig(cfg)

	var clusterNames []string
	clusterPaginator := eks.NewListClustersPaginator(eksClient, &eks.ListClustersInput{})
	for clusterPaginator.HasMorePages() {
		out, err := clusterPaginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		clusterNames = append(clusterNames, out.Clusters...)
	}

	var nodegroups []ngNodegroup
	for _, clusterName := range clusterNames {
		var ngNames []string
		ngPaginator := eks.NewListNodegroupsPaginator(eksClient, &eks.ListNodegroupsInput{
			ClusterName: aws.String(clusterName),
		})
		for ngPaginator.HasMorePages() {
			out, err := ngPaginator.NextPage(ctx)
			if err != nil {
				return nil, err
			}
			ngNames = append(ngNames, out.Nodegroups...)
		}

		for _, ngName := range ngNames {
			out, err := eksClient.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
				ClusterName:   aws.String(clusterName),
				NodegroupName: aws.String(ngName),
			})
			if err != nil {
				return nil, err
			}
			if out.Nodegroup == nil {
				continue
			}
			n := out.Nodegroup
			nodegroup := ngNodegroup{
				NodegroupName: aws.ToString(n.NodegroupName),
				ClusterName:   aws.ToString(n.ClusterName),
				Status:        string(n.Status),
			}
			if n.Health != nil {
				for _, issue := range n.Health.Issues {
					nodegroup.Issues = append(nodegroup.Issues, ngIssue{
						Code:    string(issue.Code),
						Message: aws.ToString(issue.Message),
					})
				}
			}
			nodegroups = append(nodegroups, nodegroup)
		}
	}

	return ngData{Nodegroups: nodegroups}, nil
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
	out, err := client.DescribeScalingActivities(ctx, &autoscaling.DescribeScalingActivitiesInput{
		AutoScalingGroupName: aws.String(name),
		MaxRecords:           aws.Int32(1),
	})
	if err != nil {
		if apiErr, ok := errors.AsType[smithy.APIError](err); ok {
			return asgScalingActivity{Outcome: "error", ErrorCode: apiErr.ErrorCode()}
		}
		return asgScalingActivity{Outcome: "error", ErrorCode: err.Error()}
	}
	if len(out.Activities) == 0 {
		return asgScalingActivity{Outcome: "none"}
	}
	a := out.Activities[0]
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

	// elasticbeanstalk's generated paginators cover
	// DescribeEvents/ManagedActionHistory/Platform* only, so the
	// DescribeEnvironments NextToken loop is hand-rolled.
	var envs []ebtypes.EnvironmentDescription
	var nextToken *string
	for {
		out, err := client.DescribeEnvironments(ctx, &elasticbeanstalk.DescribeEnvironmentsInput{NextToken: nextToken})
		if err != nil {
			return nil, err
		}
		envs = append(envs, out.Environments...)
		if out.NextToken == nil || *out.NextToken == "" {
			break
		}
		nextToken = out.NextToken
	}

	var result []ebEnvironment
	for _, e := range envs {
		env := ebEnvironment{
			EnvironmentName: aws.ToString(e.EnvironmentName),
			EnvironmentId:   aws.ToString(e.EnvironmentId),
			Health:          string(e.Health),
			Status:          string(e.Status),
		}
		healthOut, err := client.DescribeEnvironmentHealth(ctx, &elasticbeanstalk.DescribeEnvironmentHealthInput{
			EnvironmentName: e.EnvironmentName,
			AttributeNames:  []ebtypes.EnvironmentHealthAttribute{ebtypes.EnvironmentHealthAttributeCauses},
		})
		if err != nil {
			env.CausesOutcome = "error"
			if apiErr, ok := errors.AsType[smithy.APIError](err); ok {
				env.CausesErrorCode = apiErr.ErrorCode()
			} else {
				env.CausesErrorCode = err.Error()
			}
		} else {
			env.CausesOutcome = "ok"
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
		if err != nil {
			item.TargetHealthOutcome = "error"
			if apiErr, ok := errors.AsType[smithy.APIError](err); ok {
				item.TargetHealthError = apiErr.ErrorCode()
			} else {
				item.TargetHealthError = err.Error()
			}
		} else {
			item.TargetHealthOutcome = "ok"
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
