// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"cmp"
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// asgLaunchSources is the one reader of where an Auto Scaling group launches
// its instances from: its launch template, its launch configuration, and a
// mixed-instances policy's launch template with each template its Overrides[]
// name for an instance type, and each override's ImageId, "the AMI to use for
// instances launched with this override"
// (https://docs.aws.amazon.com/autoscaling/ec2/APIReference/API_LaunchTemplateOverrides.html).
// A group at desired capacity 0 launches from them all the same.
func asgLaunchSources(asg asgtypes.AutoScalingGroup) (templates []asgtypes.LaunchTemplateSpecification, configuration string, images []string) {
	add := func(spec *asgtypes.LaunchTemplateSpecification) {
		if spec != nil && (aws.ToString(spec.LaunchTemplateId) != "" || aws.ToString(spec.LaunchTemplateName) != "") {
			templates = append(templates, *spec)
		}
	}
	add(asg.LaunchTemplate)
	if mip := asg.MixedInstancesPolicy; mip != nil && mip.LaunchTemplate != nil {
		add(mip.LaunchTemplate.LaunchTemplateSpecification)
		for _, o := range mip.LaunchTemplate.Overrides {
			add(o.LaunchTemplateSpecification)
			images = append(images, aws.ToString(o.ImageId))
		}
	}
	return templates, aws.ToString(asg.LaunchConfigurationName), nonEmpty(images...)
}

// asgLaunch is what a group's instances launch with, over every launch
// source: the images, the security groups and the instance profiles (a name
// or an ARN).
type asgLaunch struct {
	images, securityGroups, profiles []string
}

// readASGLaunch reads the settings of every launch source of asg: the
// launch configuration with DescribeLaunchConfigurations, each template's
// version with DescribeLaunchTemplateVersions. A source that could not be
// read leaves the read partial, and none read leaves it failed. A group names
// at least one source (CreateAutoScalingGroup), so a row naming none was not
// read whole and is unread. The overrides' images need no read.
func readASGLaunch(ctx context.Context, clients any, asg asgtypes.AutoScalingGroup) (asgLaunch, relatedRead) {
	templates, configuration, images := asgLaunchSources(asg)
	sources := len(templates)
	if configuration != "" {
		sources++
	}
	c, ok := clients.(*ServiceClients)
	if sources == 0 || !ok || c == nil {
		return asgLaunch{images: images}, relatedRead{unread: true}
	}
	out := asgLaunch{images: images}
	var failures []Failure
	if configuration != "" {
		lcs, err := launchConfigurations(ctx, c.AutoScaling, configuration)
		if err != nil {
			failures = append(failures, FailedCall(configuration, err))
		}
		for _, lc := range lcs {
			out.images = append(out.images, aws.ToString(lc.ImageId))
			out.securityGroups = append(out.securityGroups, lc.SecurityGroups...)
			out.profiles = append(out.profiles, aws.ToString(lc.IamInstanceProfile))
		}
	}
	for _, spec := range templates {
		data, err := launchTemplateData(ctx, c.EC2, spec)
		if err != nil {
			failures = append(failures, FailedCall(cmp.Or(aws.ToString(spec.LaunchTemplateId), aws.ToString(spec.LaunchTemplateName)), err))
			continue
		}
		if data == nil {
			continue
		}
		out.images = append(out.images, aws.ToString(data.ImageId))
		out.securityGroups = append(out.securityGroups, data.SecurityGroupIds...)
		for _, ni := range data.NetworkInterfaces {
			out.securityGroups = append(out.securityGroups, ni.Groups...)
		}
		if p := data.IamInstanceProfile; p != nil {
			out.profiles = append(out.profiles, cmp.Or(aws.ToString(p.Arn), aws.ToString(p.Name)))
		}
	}
	failure := AggregateFailures("asg-related: launch source", failures, sources)
	if len(failures) == sources {
		return out, relatedRead{unread: true, failed: true, failure: failure}
	}
	return out, relatedRead{partial: len(failures) > 0, failure: failure}
}

// launchTemplateData reads the version of the template spec names, by id or
// by name; a spec that names no version takes "$Default"
// (https://docs.aws.amazon.com/autoscaling/ec2/APIReference/API_LaunchTemplateSpecification.html).
func launchTemplateData(ctx context.Context, api EC2DescribeLaunchTemplateVersionsAPI, spec asgtypes.LaunchTemplateSpecification) (*ec2types.ResponseLaunchTemplateData, error) {
	in := &ec2.DescribeLaunchTemplateVersionsInput{Versions: []string{cmp.Or(aws.ToString(spec.Version), "$Default")}}
	if id := aws.ToString(spec.LaunchTemplateId); id != "" {
		in.LaunchTemplateId = aws.String(id)
	} else {
		in.LaunchTemplateName = spec.LaunchTemplateName
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
		return api.DescribeLaunchTemplateVersions(ctx, in)
	})
	if err != nil {
		return nil, err
	}
	// no finding: a version DescribeLaunchTemplateVersions cannot find is an
	// error, so an empty answer names no version to read.
	if len(out.LaunchTemplateVersions) == 0 {
		return nil, nil
	}
	return out.LaunchTemplateVersions[0].LaunchTemplateData, nil
}

// asgMembers is the one reader of which instances an Auto Scaling group
// holds: Instances[] minus those whose LifecycleState is a Terminating or
// Terminated one, warm pool's included, or Detached, which the group lists
// while they leave it
// (https://docs.aws.amazon.com/autoscaling/ec2/APIReference/API_Instance.html).
func asgMembers(asg asgtypes.AutoScalingGroup) []string {
	var ids []string
	for _, inst := range asg.Instances {
		state := string(inst.LifecycleState)
		if strings.Contains(state, "Terminat") || state == string(asgtypes.LifecycleStateDetached) {
			continue
		}
		if id := aws.ToString(inst.InstanceId); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}
