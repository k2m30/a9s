package demo

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
)

// registerComputeHandlers registers ASG and Elastic Beanstalk handlers.
func registerComputeHandlers(t *Transport) {
	registerASGHandlers(t)
	registerEBHandlers(t)
}

// ---------------------------------------------------------------------------
// Auto Scaling (awsquery — XML)
// ---------------------------------------------------------------------------

func registerASGHandlers(t *Transport) {
	t.Handle("autoscaling", "DescribeAutoScalingGroups", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["asg"]()
		asgs := ExtractSDK[asgtypes.AutoScalingGroup](resources)

		var sb strings.Builder
		sb.WriteString(`<AutoScalingGroups>`)
		for _, asg := range asgs {
			name := aws.ToString(asg.AutoScalingGroupName)
			arn := aws.ToString(asg.AutoScalingGroupARN)
			minSize := int32(0)
			if asg.MinSize != nil {
				minSize = *asg.MinSize
			}
			maxSize := int32(0)
			if asg.MaxSize != nil {
				maxSize = *asg.MaxSize
			}
			desired := int32(0)
			if asg.DesiredCapacity != nil {
				desired = *asg.DesiredCapacity
			}
			status := aws.ToString(asg.Status)
			createdTime := ""
			if asg.CreatedTime != nil {
				createdTime = asg.CreatedTime.UTC().Format("2006-01-02T15:04:05Z")
			}

			fmt.Fprintf(&sb, `<member>`)
			fmt.Fprintf(&sb, `<AutoScalingGroupName>%s</AutoScalingGroupName>`, xmlEscape(name))
			fmt.Fprintf(&sb, `<AutoScalingGroupARN>%s</AutoScalingGroupARN>`, xmlEscape(arn))
			fmt.Fprintf(&sb, `<MinSize>%d</MinSize>`, minSize)
			fmt.Fprintf(&sb, `<MaxSize>%d</MaxSize>`, maxSize)
			fmt.Fprintf(&sb, `<DesiredCapacity>%d</DesiredCapacity>`, desired)
			fmt.Fprintf(&sb, `<DefaultCooldown>300</DefaultCooldown>`)
			fmt.Fprintf(&sb, `<HealthCheckType>EC2</HealthCheckType>`)
			fmt.Fprintf(&sb, `<HealthCheckGracePeriod>300</HealthCheckGracePeriod>`)
			if status != "" {
				fmt.Fprintf(&sb, `<Status>%s</Status>`, xmlEscape(status))
			}
			if createdTime != "" {
				fmt.Fprintf(&sb, `<CreatedTime>%s</CreatedTime>`, createdTime)
			}
			// Availability zones
			sb.WriteString(`<AvailabilityZones>`)
			for _, az := range asg.AvailabilityZones {
				fmt.Fprintf(&sb, `<member>%s</member>`, xmlEscape(az))
			}
			sb.WriteString(`</AvailabilityZones>`)
			// Instances
			sb.WriteString(`<Instances>`)
			for _, inst := range asg.Instances {
				fmt.Fprintf(&sb, `<member><InstanceId>%s</InstanceId><AvailabilityZone>%s</AvailabilityZone><HealthStatus>%s</HealthStatus><LifecycleState>%s</LifecycleState></member>`,
					xmlEscape(aws.ToString(inst.InstanceId)),
					xmlEscape(aws.ToString(inst.AvailabilityZone)),
					xmlEscape(aws.ToString(inst.HealthStatus)),
					xmlEscape(string(inst.LifecycleState)),
				)
			}
			sb.WriteString(`</Instances>`)
			fmt.Fprintf(&sb, `</member>`)
		}
		sb.WriteString(`</AutoScalingGroups>`)

		body := awsQueryXML("DescribeAutoScalingGroups", "http://autoscaling.amazonaws.com/doc/2011-01-01/", sb.String())
		return XMLResponse(body), nil
	})
}

// ---------------------------------------------------------------------------
// Elastic Beanstalk (awsquery — XML)
// ---------------------------------------------------------------------------

func registerEBHandlers(t *Transport) {
	t.Handle("elasticbeanstalk", "DescribeEnvironments", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["eb"]()
		envs := ExtractSDK[ebtypes.EnvironmentDescription](resources)

		var sb strings.Builder
		sb.WriteString(`<Environments>`)
		for _, env := range envs {
			envName := aws.ToString(env.EnvironmentName)
			envID := aws.ToString(env.EnvironmentId)
			appName := aws.ToString(env.ApplicationName)
			status := string(env.Status)
			health := string(env.Health)
			versionLabel := aws.ToString(env.VersionLabel)
			envArn := aws.ToString(env.EnvironmentArn)

			dateCreated := ""
			if env.DateCreated != nil {
				dateCreated = env.DateCreated.UTC().Format("2006-01-02T15:04:05Z")
			}
			dateUpdated := ""
			if env.DateUpdated != nil {
				dateUpdated = env.DateUpdated.UTC().Format("2006-01-02T15:04:05Z")
			}

			fmt.Fprintf(&sb, `<member>`)
			fmt.Fprintf(&sb, `<EnvironmentName>%s</EnvironmentName>`, xmlEscape(envName))
			fmt.Fprintf(&sb, `<EnvironmentId>%s</EnvironmentId>`, xmlEscape(envID))
			fmt.Fprintf(&sb, `<ApplicationName>%s</ApplicationName>`, xmlEscape(appName))
			fmt.Fprintf(&sb, `<Status>%s</Status>`, xmlEscape(status))
			fmt.Fprintf(&sb, `<Health>%s</Health>`, xmlEscape(health))
			if versionLabel != "" {
				fmt.Fprintf(&sb, `<VersionLabel>%s</VersionLabel>`, xmlEscape(versionLabel))
			}
			if envArn != "" {
				fmt.Fprintf(&sb, `<EnvironmentArn>%s</EnvironmentArn>`, xmlEscape(envArn))
			}
			if dateCreated != "" {
				fmt.Fprintf(&sb, `<DateCreated>%s</DateCreated>`, dateCreated)
			}
			if dateUpdated != "" {
				fmt.Fprintf(&sb, `<DateUpdated>%s</DateUpdated>`, dateUpdated)
			}
			fmt.Fprintf(&sb, `</member>`)
		}
		sb.WriteString(`</Environments>`)

		body := awsQueryXML("DescribeEnvironments", "https://elasticbeanstalk.amazonaws.com/docs/2010-12-01/", sb.String())
		return XMLResponse(body), nil
	})
}
