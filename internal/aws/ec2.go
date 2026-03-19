package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("ec2", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchEC2Instances(ctx, c.EC2)
	})
	resource.RegisterFieldKeys("ec2", []string{"instance_id", "name", "state", "type", "private_ip", "public_ip", "launch_time"})
}

// FetchEC2Instances calls the EC2 DescribeInstances API and converts the
// response into a slice of generic Resource structs.
func FetchEC2Instances(ctx context.Context, api EC2DescribeInstancesAPI) ([]resource.Resource, error) {
	output, err := api.DescribeInstances(ctx, &ec2.DescribeInstancesInput{})
	if err != nil {
		return nil, err
	}

	var resources []resource.Resource

	for _, reservation := range output.Reservations {
		for _, inst := range reservation.Instances {
			// Extract instance ID
			instanceID := ""
			if inst.InstanceId != nil {
				instanceID = *inst.InstanceId
			}

			// Extract Name tag
			name := ""
			for _, tag := range inst.Tags {
				if tag.Key != nil && *tag.Key == "Name" {
					if tag.Value != nil {
						name = *tag.Value
					}
					break
				}
			}

			// Extract state
			state := string(inst.State.Name)

			// Extract instance type
			instanceType := string(inst.InstanceType)

			// Extract private IP
			privateIP := ""
			if inst.PrivateIpAddress != nil {
				privateIP = *inst.PrivateIpAddress
			}

			// Extract public IP (may be nil)
			publicIP := ""
			if inst.PublicIpAddress != nil {
				publicIP = *inst.PublicIpAddress
			}

			// Format launch time
			launchTime := ""
			if inst.LaunchTime != nil {
				launchTime = inst.LaunchTime.Format("2006-01-02T15:04:05Z07:00")
			}

			// Build DetailData
			detail := buildEC2DetailData(inst, instanceID, name, state, instanceType, privateIP, publicIP, launchTime)

			// Build RawJSON
			rawJSON := ""
			if jsonBytes, err := json.MarshalIndent(inst, "", "  "); err == nil {
				rawJSON = string(jsonBytes)
			}

			r := resource.Resource{
				ID:     instanceID,
				Name:   name,
				Status: state,
				Fields: map[string]string{
					"instance_id": instanceID,
					"name":        name,
					"state":       state,
					"type":        instanceType,
					"private_ip":  privateIP,
					"public_ip":   publicIP,
					"launch_time": launchTime,
				},
				DetailData: detail,
				RawJSON:    rawJSON,
				RawStruct:  inst,
			}

			resources = append(resources, r)
		}
	}

	return resources, nil
}

func buildEC2DetailData(inst ec2types.Instance, instanceID, name, state, instanceType, privateIP, publicIP, launchTime string) map[string]string {
	detail := map[string]string{
		"Instance ID":   instanceID,
		"Name":          name,
		"State":         state,
		"Instance Type": instanceType,
		"Private IP":    privateIP,
		"Public IP":     publicIP,
		"Launch Time":   launchTime,
	}

	// AMI
	if inst.ImageId != nil {
		detail["AMI"] = *inst.ImageId
	} else {
		detail["AMI"] = ""
	}

	// VPC
	if inst.VpcId != nil {
		detail["VPC"] = *inst.VpcId
	} else {
		detail["VPC"] = ""
	}

	// Subnet
	if inst.SubnetId != nil {
		detail["Subnet"] = *inst.SubnetId
	} else {
		detail["Subnet"] = ""
	}

	// Security Groups
	var sgIDs []string
	for _, sg := range inst.SecurityGroups {
		if sg.GroupId != nil {
			sgIDs = append(sgIDs, *sg.GroupId)
		}
	}
	detail["Security Groups"] = strings.Join(sgIDs, ", ")

	// Architecture
	detail["Architecture"] = string(inst.Architecture)

	// Platform
	if inst.PlatformDetails != nil {
		detail["Platform"] = *inst.PlatformDetails
	} else {
		detail["Platform"] = ""
	}

	// Tags
	for _, tag := range inst.Tags {
		if tag.Key != nil && tag.Value != nil {
			detail["Tag: "+*tag.Key] = *tag.Value
		}
	}

	return detail
}
