package unit

import (
	"context"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	ostypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// A stack in DELETE_FAILED still holds the resources that blocked the delete
// and waits for the operator with no end date, so its events are read and a
// credential in its outputs is still reported. Termination protection is
// advice against the delete the operator already asked for.
func TestT1aDev_CFNDeleteFailedIsAFailedLiveStack(t *testing.T) {
	for raw, want := range map[string]catalog.LifecycleClass{
		"DELETE_FAILED":      catalog.LifecycleFailedLive,
		"DELETE_IN_PROGRESS": catalog.LifecycleTearingDown,
		"DELETE_COMPLETE":    catalog.LifecycleEnded,
	} {
		if got := awsclient.LifecycleClassOf("cfn", raw); got != want {
			t.Errorf("cfn %s class = %v, want %v", raw, got, want)
		}
	}

	const name = "acme-logs-retired"
	rs := w6bFetchCFN(t, t578CFNStackWithSecretOutput(name, "DELETE_FAILED"))
	r := pw1ResourceByID(t, rs, name)
	pw1RequireFinding(t, r.Findings, awsclient.CodeCFNStackDeleteFailed, "delete failed", domain.SevBroken, "wave1")
	pw1RequireFinding(t, r.Findings, w6bCFNCodeOutputSecret, "credential in stack outputs", domain.SevBroken, "wave1")
	pw1RequireNoFinding(t, r.Findings, w6bCFNCodeTerminationProtectionOff)

	fake := &t1aDevCFNFake{t578CFNEventsFake{events: map[string][]cfntypes.StackEvent{name: {
		t578Event(name, 12, "", "", cfntypes.ResourceStatusDeleteFailed, "The following resource(s) failed to delete: [LogsBucket]."),
		t578Event(name, 11, "LogsBucket", "AWS::S3::Bucket", cfntypes.ResourceStatusDeleteFailed, "The bucket you tried to delete is not empty"),
		t578Event(name, 10, "", "", cfntypes.ResourceStatusDeleteInProgress, "User Initiated"),
	}}}}
	res := Wave2Result(t, "cfn", &awsclient.ServiceClients{CloudFormation: fake}, rs)
	if got, want := t578FailedResourceValues(res, name), "AWS::S3::Bucket/LogsBucket"; !slices.Contains(got, want) {
		t.Errorf("DELETE_FAILED stack Failed resource rows = %v, want one naming %s", got, want)
	}
}

type t1aDevCFNFake struct{ t578CFNEventsFake }

func (*t1aDevCFNFake) DescribeStacks(_ context.Context, _ *cloudformation.DescribeStacksInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
	return &cloudformation.DescribeStacksOutput{}, nil
}

// A stack made by CreateChangeSet waits in REVIEW_IN_PROGRESS with no
// resources until someone runs the change set, so there is nothing for
// termination protection to protect.
func TestT1aDev_CFNReviewInProgressRaisesNoTerminationProtection(t *testing.T) {
	const name = "acme-orders-changeset"
	r := pw1ResourceByID(t, w6bFetchCFN(t, w6bCFNStack(name, "REVIEW_IN_PROGRESS", aws.Bool(false))), name)
	pw1RequireFinding(t, r.Findings, awsclient.CodeCFNStackInProgress, "review in progress", domain.SevWarn, "wave1")
	pw1RequireNoFinding(t, r.Findings, w6bCFNCodeTerminationProtectionOff)
}

// The VPC peering lifecycle: a failed, expired or rejected request cannot be
// accepted, repaired or acted on, and stays visible for a fixed time before
// AWS drops it, as a deleted one does.
func TestT1aDev_VpcPeerTerminalStatesShareTheEndedClass(t *testing.T) {
	for _, raw := range []string{"failed", "expired", "rejected", "deleted"} {
		if got := awsclient.LifecycleClassOf("vpc-peer", raw); got != catalog.LifecycleEnded {
			t.Errorf("vpc-peer %s class = %v, want Ended", raw, got)
		}
	}
}

// An Unavailable or Failed ClusterAvailabilityStatus restates any status that
// is not a normal one (a paused or creating cluster is not available for
// queries), so it is a finding of its own only beside a normal status. The
// Maintenance and Modifying values say something the status does not.
func TestT1aDev_RedshiftAvailabilitySpeaksOnlyBesideANormalStatus(t *testing.T) {
	for _, tc := range []struct {
		status, availability string
		code                 domain.FindingCode
		want                 bool
	}{
		{"available", "Unavailable", awsclient.CodeRedshiftUnavailable, true},
		{"paused", "Unavailable", awsclient.CodeRedshiftUnavailable, false},
		{"creating", "Unavailable", awsclient.CodeRedshiftUnavailable, false},
		{"available", "Failed", awsclient.CodeRedshiftFailed, true},
		{"hardware-failure", "Failed", awsclient.CodeRedshiftFailed, false},
		{"available", "Maintenance", awsclient.CodeRedshiftMaintenance, true},
		{"available", "Modifying", awsclient.CodeRedshiftAvailabilityModifying, true},
	} {
		t.Run(tc.status+"/"+tc.availability, func(t *testing.T) {
			const id = "acme-dwh-axis"
			c := t578RedshiftCluster(id, tc.status)
			c.ClusterAvailabilityStatus = aws.String(tc.availability)
			r := pw1ResourceByID(t, t578FetchRedshift(t, &t578RedshiftFake{clusters: []redshifttypes.Cluster{c}}), id)
			if got := hasCode(r.Findings, tc.code); got != tc.want {
				t.Errorf("%s with availability %s: has %s = %v, want %v (findings %v)", tc.status, tc.availability, tc.code, got, tc.want, r.Findings)
			}
			if tc.status == "paused" {
				top, _ := domain.TopFinding(r.Findings)
				if top.Severity != domain.SevDim {
					t.Errorf("paused cluster top finding %s severity = %v, want Dim", top.Code, top.Severity)
				}
			}
		})
	}
}

// Grey health is "Your environment is being updated", a restatement of any
// status but Ready. Red and Yellow are the outcome of the operation that is
// running, so they stand beside a Ready status and an in-flight one.
func TestT1aDev_EBHealthSpeaksOnlyBesideAReadyEnvironment(t *testing.T) {
	for _, tc := range []struct {
		status ebtypes.EnvironmentStatus
		health ebtypes.EnvironmentHealth
		code   domain.FindingCode
		want   bool
	}{
		{ebtypes.EnvironmentStatusReady, ebtypes.EnvironmentHealthGrey, awsclient.CodeEBHealthGrey, true},
		{ebtypes.EnvironmentStatusUpdating, ebtypes.EnvironmentHealthGrey, awsclient.CodeEBHealthGrey, false},
		{ebtypes.EnvironmentStatusReady, ebtypes.EnvironmentHealthRed, awsclient.CodeEBHealthRed, true},
		{ebtypes.EnvironmentStatusLaunching, ebtypes.EnvironmentHealthRed, awsclient.CodeEBHealthRed, true},
		{ebtypes.EnvironmentStatusReady, ebtypes.EnvironmentHealthYellow, awsclient.CodeEBHealthYellow, true},
		{ebtypes.EnvironmentStatusUpdating, ebtypes.EnvironmentHealthYellow, awsclient.CodeEBHealthYellow, true},
	} {
		t.Run(string(tc.status)+"/"+string(tc.health), func(t *testing.T) {
			const id = "e-acmeaxis0001"
			fake := &t578EBFake{envs: []ebtypes.EnvironmentDescription{t578EBEnv(id, "acme-checkout-prod", tc.status, tc.health)}}
			out, err := awsclient.FetchEBEnvironmentsPage(context.Background(), fake, "")
			if err != nil {
				t.Fatalf("FetchEBEnvironmentsPage: %v", err)
			}
			r := pw1ResourceByID(t, out.Resources, id)
			if got := hasCode(r.Findings, tc.code); got != tc.want {
				t.Errorf("%s with health %s: has %s = %v, want %v (findings %v)", tc.status, tc.health, tc.code, got, tc.want, r.Findings)
			}
		})
	}
}

// Processing is the state finding while the domain is modifying, upgrading or
// updating, and the same finding beside an Active status: one finding in
// either case.
func TestT1aDev_OpenSearchProcessingIsOneFindingInEveryState(t *testing.T) {
	for _, st := range []ostypes.DomainProcessingStatusType{
		ostypes.DomainProcessingStatusTypeActive,
		ostypes.DomainProcessingStatusTypeModifying,
		ostypes.DomainProcessingStatusTypeUpgrading,
	} {
		t.Run(string(st), func(t *testing.T) {
			d := w2OSDomain("acme-search-axis")
			d.DomainProcessingStatus = st
			d.Processing = aws.Bool(true)
			byID, _ := w2OSRun(t, d)
			r, ok := byID["acme-search-axis"]
			if !ok {
				t.Fatalf("no row for the domain: %v", byID)
			}
			n := 0
			for _, f := range r.Findings {
				if f.Code == awsclient.CodeOpenSearchProcessing {
					n++
				}
			}
			if n != 1 {
				t.Errorf("%s with Processing=true: %d processing findings, want 1 (findings %v)", st, n, r.Findings)
			}
		})
	}
}

// A failing stop code is AWS's account of why a task stopped. AWS sets it
// once a stop starts, so the finding is declared for every lifecycle class
// and reaches the row through the composer in each of them.
func TestT1aDev_ECSStopCodeIsDeclaredForEveryTaskState(t *testing.T) {
	td := catalog.FindAny("ecs-task")
	if td == nil || td.Fetcher == nil {
		t.Fatal("ecs-task has no catalog Fetcher")
	}
	for _, raw := range td.StateEnum {
		class := awsclient.LifecycleClassOf("ecs-task", raw)
		if !catalog.Applies("ecs-task", awsclient.CodeECSTaskStopCodeFailed, class, raw, "") {
			t.Errorf("stop-code finding does not apply in %s (%v)", raw, class)
		}
		t.Run(raw, func(t *testing.T) {
			const id = "0aaaa1111bbbb2222cccc3333dddd4471"
			task := pw1Task(id, pw1TaskDefARN)
			task.LastStatus = aws.String(raw)
			task.StopCode = ecstypes.TaskStopCodeEssentialContainerExited
			fake := &pw1ECSTaskFake{tasks: map[string]ecstypes.Task{id: task}, defs: map[string]ecstypes.TaskDefinition{pw1TaskDefARN: pw1TaskDef(pw1TaskDefARN, pw1HealthyContainer("app"))}}
			page, err := td.Fetcher(context.Background(), &awsclient.ServiceClients{ECS: fake}, "")
			if err != nil {
				t.Fatalf("ecs-task fetcher: %v", err)
			}
			r := pw1ResourceByID(t, page.Resources, id)
			if !hasCode(r.Findings, awsclient.CodeECSTaskStopCodeFailed) {
				t.Errorf("%s task with EssentialContainerExited: findings %v lack %s", raw, r.Findings, awsclient.CodeECSTaskStopCodeFailed)
			}
		})
	}
}

// A narrowing of a posture check to some lifecycle classes is declared on the
// check's own FindingDef, so the catalog says what the enricher does.
func TestT1aDev_PostureNarrowingsAreDeclaredOnTheirFindingDefs(t *testing.T) {
	for _, tc := range []struct {
		short string
		code  domain.FindingCode
		raw   string
		want  bool
	}{
		{"ec2", "ec2.internet-exposed", "running", true},
		{"ec2", "ec2.internet-exposed", "pending", false},
		{"ec2", "ec2.internet-exposed", "stopped", false},
		{"ec2", "ec2.internet-exposed-all", "running", true},
		{"ec2", "ec2.internet-exposed-all", "stopped", false},
		{"vpc-peer", "vpc-peer.warn.no_local_route", "active", true},
		{"vpc-peer", "vpc-peer.warn.no_local_route", "pending-acceptance", false},
		{"vpc-peer", "vpc-peer.warn.route_blackholed", "active", true},
		{"vpc-peer", "vpc-peer.warn.route_blackholed", "provisioning", false},
		{"eb-rule", "eb-rule.no-targets", "ENABLED", true},
		{"eb-rule", "eb-rule.no-targets", "DISABLED", false},
		{"asg", "asg.scaling-activity-failed", "Delete in progress", true},
		{"asg", "asg.launch-config.public-ip", "Delete in progress", false},
	} {
		class := awsclient.LifecycleClassOf(tc.short, tc.raw)
		if got := catalog.Applies(tc.short, tc.code, class, tc.raw, ""); got != tc.want {
			t.Errorf("Applies(%s, %s %q) = %v, want %v", tc.code, tc.short, tc.raw, got, tc.want)
		}
	}
}

type t1aDevASGFake struct {
	*pw1ASGEnrichFake
	failed map[string]string
	asked  []string
}

func (f *t1aDevASGFake) DescribeScalingActivities(_ context.Context, in *autoscaling.DescribeScalingActivitiesInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeScalingActivitiesOutput, error) {
	name := aws.ToString(in.AutoScalingGroupName)
	f.asked = append(f.asked, name)
	msg, ok := f.failed[name]
	if !ok {
		return &autoscaling.DescribeScalingActivitiesOutput{}, nil
	}
	return &autoscaling.DescribeScalingActivitiesOutput{Activities: []asgtypes.Activity{{
		ActivityId:           aws.String("a1b2c3d4-0000-1111-2222-333344445555"),
		AutoScalingGroupName: aws.String(name),
		StatusCode:           asgtypes.ScalingActivityStatusCodeFailed,
		StatusMessage:        aws.String(msg),
	}}}, nil
}

// A group stuck in "Delete in progress" is waiting on instances that cannot
// terminate; the failed scaling activity is AWS's statement of why, so it is
// read. The launch configuration posture of a group being deleted is not.
func TestT1aDev_ASGDeletingGroupKeepsItsScalingActivity(t *testing.T) {
	const name, lcName = "acme-stuck-asg", "acme-stuck-lc"
	g := pw1ASGWithLaunchConfig(name, lcName)
	g.Status = aws.String("Delete in progress")
	lc := pw1LaunchConfig(lcName)
	lc.AssociatePublicIpAddress = aws.Bool(true)
	const why = "Instance i-0aaaa1111bbbb2222 is protected from scale in."
	fake := &t1aDevASGFake{
		pw1ASGEnrichFake: &pw1ASGEnrichFake{configs: map[string]asgtypes.LaunchConfiguration{lcName: lc}},
		failed:           map[string]string{name: why},
	}
	res := Wave2Result(t, "asg", &awsclient.ServiceClients{AutoScaling: fake, EC2: t581NoIMDSDefaultEC2{}}, pw1FetchASGs(t, g))
	pw1RequireRow(t, pw1Rows(res, name, "asg.scaling-activity-failed"), "Message", why)
	if _, ok := pw1FindFinding(res.Findings[name], pw1ASGCodeLCPublicIP); ok {
		t.Errorf("%s emitted for a group AWS is deleting", pw1ASGCodeLCPublicIP)
	}
	if fake.lcCalls != 0 {
		t.Errorf("DescribeLaunchConfigurations called %d times for a group being deleted, want 0", fake.lcCalls)
	}
}

// A degraded row records the code of its finding and leaves the lifecycle
// field empty: the Status cell is drawn from the finding, and the field holds
// only what AWS reported.
func TestT1aDev_DegradedRowLeavesTheLifecycleFieldEmpty(t *testing.T) {
	for _, short := range []string{"ng", "eks", "opensearch", "ecs-svc", "ecs", "ecs-task", "ddb", "sqs", "cb"} {
		r := awsclient.DegradedDetails(short, "acme-row-1", &smithy.GenericAPIError{Code: "AccessDeniedException"})
		td := resource.FindResourceType(short)
		if td == nil {
			t.Fatalf("%s is not registered", short)
		}
		if got := r.Fields[td.StatusKey()]; got != "" {
			t.Errorf("%s degraded row Fields[%q] = %q, want empty", short, td.StatusKey(), got)
		}
		if got := r.Fields["status"]; got != "" {
			t.Errorf("%s degraded row Fields[status] = %q, want empty", short, got)
		}
		if len(r.Findings) != 1 || r.Findings[0].Phrase != "details denied" {
			t.Errorf("%s degraded row findings = %v, want the details-denied finding", short, r.Findings)
		}
		if got := r.Fields[awsclient.DegradedFindingField]; got != string(awsclient.DetailsDeniedCode(short)) {
			t.Errorf("%s degraded row Fields[%s] = %q, want %s", short, awsclient.DegradedFindingField, got, awsclient.DetailsDeniedCode(short))
		}
	}
}
