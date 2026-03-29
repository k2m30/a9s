package demo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// registerAllHandlers registers all demo handlers onto t.
func registerAllHandlers(t *Transport) {
	registerLambdaHandlers(t)
	registerSecretsManagerHandlers(t)
	registerSTSHandlers(t)
	registerEC2Handlers(t)
	registerS3Handlers(t)
	registerIAMHandlers(t)
	registerECSHandlers(t)
	registerDynamoDBHandlers(t)
	// Phase 2:
	registerComputeHandlers(t)
	registerContainerHandlers(t)
	registerDatabaseHandlers(t)
	registerNetworkHandlers(t)
	registerSecurityHandlers(t)
	registerSecretsExtHandlers(t)
	registerMessagingHandlers(t)
	registerCICDHandlers(t)
	registerMonitoringHandlers(t)
	registerDNSCDNHandlers(t)
	registerDataHandlers(t)
	registerStorageHandlers(t)
	registerChildHandlers(t)
}

// ---------------------------------------------------------------------------
// Lambda (restjson1)
// ---------------------------------------------------------------------------

func registerLambdaHandlers(t *Transport) {
	t.Handle("lambda", "ListFunctions", func(req *http.Request) (*http.Response, error) {
		resources := demoData["lambda"]()
		fns := ExtractSDK[lambdatypes.FunctionConfiguration](resources)

		// Read pagination marker from query param
		marker := req.URL.Query().Get("Marker")
		page, nextToken := Paginate(fns, 20, marker)

		out := &lambda.ListFunctionsOutput{
			Functions:  page,
			NextMarker: nextToken,
		}
		return JSONResponse(out)
	})
}

// ---------------------------------------------------------------------------
// SecretsManager (awsjson11)
// ---------------------------------------------------------------------------

func registerSecretsManagerHandlers(t *Transport) {
	t.Handle("secretsmanager", "ListSecrets", func(req *http.Request) (*http.Response, error) {
		resources := demoData["secrets"]()
		secrets := ExtractSDK[smtypes.SecretListEntry](resources)

		out := &secretsmanager.ListSecretsOutput{
			SecretList: secrets,
		}
		return JSONResponse(out)
	})

	// GetSecretValue — return a demo secret value for reveal (x key).
	t.Handle("secretsmanager", "GetSecretValue", func(req *http.Request) (*http.Response, error) {
		var body map[string]interface{}
		if b, err := io.ReadAll(req.Body); err == nil {
			_ = json.Unmarshal(b, &body)
		}
		secretID, _ := body["SecretId"].(string)
		if secretID == "" {
			secretID = "demo/secret"
		}

		demoValue := `{"username":"demo-user","password":"demo-p@ssw0rd!","host":"demo-db.example.com","port":"5432"}`
		out := &secretsmanager.GetSecretValueOutput{
			Name:         &secretID,
			SecretString: &demoValue,
		}
		return JSONResponse(out)
	})
}

// ---------------------------------------------------------------------------
// STS (awsquery — returns XML)
// ---------------------------------------------------------------------------

func registerSTSHandlers(t *Transport) {
	t.Handle("sts", "GetCallerIdentity", func(_ *http.Request) (*http.Response, error) {
		body := `<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <GetCallerIdentityResult>
    <Account>123456789012</Account>
    <Arn>arn:aws:sts::123456789012:assumed-role/demo-admin/session</Arn>
    <UserId>AROADEMO123456789:session</UserId>
  </GetCallerIdentityResult>
  <ResponseMetadata><RequestId>demo-request-id</RequestId></ResponseMetadata>
</GetCallerIdentityResponse>`
		return XMLResponse(body), nil
	})
}

// ---------------------------------------------------------------------------
// EC2 (ec2query — returns XML)
// ---------------------------------------------------------------------------

func registerEC2Handlers(t *Transport) {
	t.Handle("ec2", "DescribeInstances", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["ec2"]()
		instances := ExtractSDK[ec2types.Instance](resources)

		xml := buildDescribeInstancesXML(instances)
		return XMLResponse(xml), nil
	})

	t.Handle("ec2", "DescribeVolumes", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["ebs"]()
		volumes := ExtractSDK[ec2types.Volume](resources)

		var items strings.Builder
		for _, v := range volumes {
			volumeID := ""
			if v.VolumeId != nil {
				volumeID = *v.VolumeId
			}
			state := string(v.State)
			volumeType := string(v.VolumeType)
			az := ""
			if v.AvailabilityZone != nil {
				az = *v.AvailabilityZone
			}
			size := int32(0)
			if v.Size != nil {
				size = *v.Size
			}
			iops := int32(0)
			if v.Iops != nil {
				iops = *v.Iops
			}
			throughput := int32(0)
			if v.Throughput != nil {
				throughput = *v.Throughput
			}
			encrypted := "false"
			if v.Encrypted != nil && *v.Encrypted {
				encrypted = "true"
			}
			createTime := ""
			if v.CreateTime != nil {
				createTime = v.CreateTime.UTC().Format("2006-01-02T15:04:05.000Z")
			}

			fmt.Fprintf(&items, `<item>`)
			fmt.Fprintf(&items, `<volumeId>%s</volumeId>`, xmlEscape(volumeID))
			fmt.Fprintf(&items, `<status>%s</status>`, xmlEscape(state))
			fmt.Fprintf(&items, `<volumeType>%s</volumeType>`, xmlEscape(volumeType))
			fmt.Fprintf(&items, `<availabilityZone>%s</availabilityZone>`, xmlEscape(az))
			fmt.Fprintf(&items, `<size>%d</size>`, size)
			fmt.Fprintf(&items, `<iops>%d</iops>`, iops)
			if throughput > 0 {
				fmt.Fprintf(&items, `<throughput>%d</throughput>`, throughput)
			}
			fmt.Fprintf(&items, `<encrypted>%s</encrypted>`, encrypted)
			if createTime != "" {
				fmt.Fprintf(&items, `<createTime>%s</createTime>`, createTime)
			}
			// attachmentSet
			items.WriteString(`<attachmentSet>`)
			for _, att := range v.Attachments {
				instanceID := ""
				if att.InstanceId != nil {
					instanceID = *att.InstanceId
				}
				device := ""
				if att.Device != nil {
					device = *att.Device
				}
				attState := string(att.State)
				fmt.Fprintf(&items, `<item><instanceId>%s</instanceId><device>%s</device><status>%s</status></item>`,
					xmlEscape(instanceID), xmlEscape(device), xmlEscape(attState))
			}
			items.WriteString(`</attachmentSet>`)
			items.WriteString(buildTagSetXML(v.Tags))
			fmt.Fprintf(&items, `</item>`)
		}

		return XMLResponse(ec2QueryXML("DescribeVolumes", "volumeSet", items.String())), nil
	})

	t.Handle("ec2", "DescribeSnapshots", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["ebs-snap"]()
		snapshots := ExtractSDK[ec2types.Snapshot](resources)

		var items strings.Builder
		for _, s := range snapshots {
			snapshotID := ""
			if s.SnapshotId != nil {
				snapshotID = *s.SnapshotId
			}
			state := string(s.State)
			volumeID := ""
			if s.VolumeId != nil {
				volumeID = *s.VolumeId
			}
			volumeSize := int32(0)
			if s.VolumeSize != nil {
				volumeSize = *s.VolumeSize
			}
			desc := ""
			if s.Description != nil {
				desc = *s.Description
			}
			progress := ""
			if s.Progress != nil {
				progress = *s.Progress
			}
			ownerID := ""
			if s.OwnerId != nil {
				ownerID = *s.OwnerId
			}
			encrypted := "false"
			if s.Encrypted != nil && *s.Encrypted {
				encrypted = "true"
			}
			startTime := ""
			if s.StartTime != nil {
				startTime = s.StartTime.UTC().Format("2006-01-02T15:04:05.000Z")
			}

			fmt.Fprintf(&items, `<item>`)
			fmt.Fprintf(&items, `<snapshotId>%s</snapshotId>`, xmlEscape(snapshotID))
			fmt.Fprintf(&items, `<status>%s</status>`, xmlEscape(state))
			fmt.Fprintf(&items, `<volumeId>%s</volumeId>`, xmlEscape(volumeID))
			fmt.Fprintf(&items, `<volumeSize>%d</volumeSize>`, volumeSize)
			fmt.Fprintf(&items, `<description>%s</description>`, xmlEscape(desc))
			fmt.Fprintf(&items, `<progress>%s</progress>`, xmlEscape(progress))
			fmt.Fprintf(&items, `<ownerId>%s</ownerId>`, xmlEscape(ownerID))
			fmt.Fprintf(&items, `<encrypted>%s</encrypted>`, encrypted)
			if startTime != "" {
				fmt.Fprintf(&items, `<startTime>%s</startTime>`, startTime)
			}
			items.WriteString(buildTagSetXML(s.Tags))
			fmt.Fprintf(&items, `</item>`)
		}

		return XMLResponse(ec2QueryXML("DescribeSnapshots", "snapshotSet", items.String())), nil
	})

	t.Handle("ec2", "DescribeImages", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["ami"]()
		images := ExtractSDK[ec2types.Image](resources)

		var items strings.Builder
		for _, img := range images {
			imageID := ""
			if img.ImageId != nil {
				imageID = *img.ImageId
			}
			name := ""
			if img.Name != nil {
				name = *img.Name
			}
			state := string(img.State)
			arch := string(img.Architecture)
			platformDetails := ""
			if img.PlatformDetails != nil {
				platformDetails = *img.PlatformDetails
			}
			rootDeviceType := string(img.RootDeviceType)
			creationDate := ""
			if img.CreationDate != nil {
				creationDate = *img.CreationDate
			}
			isPublic := "false"
			if img.Public != nil && *img.Public {
				isPublic = "true"
			}
			desc := ""
			if img.Description != nil {
				desc = *img.Description
			}
			enaSupport := "false"
			if img.EnaSupport != nil && *img.EnaSupport {
				enaSupport = "true"
			}
			ownerID := ""
			if img.OwnerId != nil {
				ownerID = *img.OwnerId
			}

			fmt.Fprintf(&items, `<item>`)
			fmt.Fprintf(&items, `<imageId>%s</imageId>`, xmlEscape(imageID))
			fmt.Fprintf(&items, `<name>%s</name>`, xmlEscape(name))
			fmt.Fprintf(&items, `<imageState>%s</imageState>`, xmlEscape(state))
			fmt.Fprintf(&items, `<architecture>%s</architecture>`, xmlEscape(arch))
			fmt.Fprintf(&items, `<platformDetails>%s</platformDetails>`, xmlEscape(platformDetails))
			fmt.Fprintf(&items, `<rootDeviceType>%s</rootDeviceType>`, xmlEscape(rootDeviceType))
			if creationDate != "" {
				fmt.Fprintf(&items, `<creationDate>%s</creationDate>`, xmlEscape(creationDate))
			}
			fmt.Fprintf(&items, `<isPublic>%s</isPublic>`, isPublic)
			fmt.Fprintf(&items, `<description>%s</description>`, xmlEscape(desc))
			fmt.Fprintf(&items, `<enaSupport>%s</enaSupport>`, enaSupport)
			fmt.Fprintf(&items, `<imageOwnerId>%s</imageOwnerId>`, xmlEscape(ownerID))
			items.WriteString(buildTagSetXML(img.Tags))
			fmt.Fprintf(&items, `</item>`)
		}

		return XMLResponse(ec2QueryXML("DescribeImages", "imagesSet", items.String())), nil
	})
}

func buildDescribeInstancesXML(instances []ec2types.Instance) string {
	var sb strings.Builder
	sb.WriteString(`<DescribeInstancesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">`)
	sb.WriteString(`<requestId>demo-request-id</requestId>`)
	sb.WriteString(`<reservationSet>`)

	for i, inst := range instances {
		instanceID := ""
		if inst.InstanceId != nil {
			instanceID = *inst.InstanceId
		}
		instanceType := string(inst.InstanceType)

		stateCode := int32(0)
		stateName := "pending"
		if inst.State != nil {
			if inst.State.Code != nil {
				stateCode = *inst.State.Code
			}
			stateName = string(inst.State.Name)
		}

		privateIP := ""
		if inst.PrivateIpAddress != nil {
			privateIP = *inst.PrivateIpAddress
		}

		launchTime := ""
		if inst.LaunchTime != nil {
			launchTime = inst.LaunchTime.UTC().Format("2006-01-02T15:04:05.000Z")
		}

		fmt.Fprintf(&sb, `<item>`)
		fmt.Fprintf(&sb, `<reservationId>r-demo%04d</reservationId>`, i+1)
		fmt.Fprintf(&sb, `<instancesSet><item>`)
		fmt.Fprintf(&sb, `<instanceId>%s</instanceId>`, xmlEscape(instanceID))
		fmt.Fprintf(&sb, `<instanceType>%s</instanceType>`, xmlEscape(instanceType))
		fmt.Fprintf(&sb, `<instanceState><code>%d</code><name>%s</name></instanceState>`,
			stateCode, xmlEscape(stateName))
		if privateIP != "" {
			fmt.Fprintf(&sb, `<privateIpAddress>%s</privateIpAddress>`, xmlEscape(privateIP))
		}
		if launchTime != "" {
			fmt.Fprintf(&sb, `<launchTime>%s</launchTime>`, launchTime)
		}
		if len(inst.Tags) > 0 {
			sb.WriteString(`<tagSet>`)
			for _, tag := range inst.Tags {
				key := ""
				if tag.Key != nil {
					key = *tag.Key
				}
				value := ""
				if tag.Value != nil {
					value = *tag.Value
				}
				fmt.Fprintf(&sb, `<item><key>%s</key><value>%s</value></item>`, xmlEscape(key), xmlEscape(value))
			}
			sb.WriteString(`</tagSet>`)
		}
		fmt.Fprintf(&sb, `</item></instancesSet>`)
		fmt.Fprintf(&sb, `</item>`)
	}

	sb.WriteString(`</reservationSet>`)
	sb.WriteString(`</DescribeInstancesResponse>`)
	return sb.String()
}

// xmlEscape escapes special XML characters.
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

// ---------------------------------------------------------------------------
// S3 (restxml — returns XML)
// ---------------------------------------------------------------------------

func registerS3Handlers(t *Transport) {
	t.Handle("s3", "ListBuckets", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["s3"]()
		buckets := ExtractSDK[s3types.Bucket](resources)

		xml := buildListBucketsXML(buckets)
		return XMLResponse(xml), nil
	})
}

func buildListBucketsXML(buckets []s3types.Bucket) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString(`<ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">`)
	sb.WriteString(`<Owner><ID>demo-owner-id</ID><DisplayName>demo-account</DisplayName></Owner>`)
	sb.WriteString(`<Buckets>`)

	for _, b := range buckets {
		name := ""
		if b.Name != nil {
			name = *b.Name
		}
		creationDate := ""
		if b.CreationDate != nil {
			creationDate = b.CreationDate.UTC().Format("2006-01-02T15:04:05.000Z")
		}
		fmt.Fprintf(&sb, `<Bucket><Name>%s</Name><CreationDate>%s</CreationDate></Bucket>`,
			xmlEscape(name), creationDate)
	}

	sb.WriteString(`</Buckets>`)
	sb.WriteString(`</ListAllMyBucketsResult>`)
	return sb.String()
}

// ---------------------------------------------------------------------------
// IAM (awsquery — returns XML)
// ---------------------------------------------------------------------------

func registerIAMHandlers(t *Transport) {
	t.Handle("iam", "ListRoles", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["role"]()
		roles := ExtractSDK[iamtypes.Role](resources)

		xml := buildListRolesXML(roles)
		return XMLResponse(xml), nil
	})
}

func buildListRolesXML(roles []iamtypes.Role) string {
	var sb strings.Builder
	sb.WriteString(`<ListRolesResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">`)
	sb.WriteString(`<ListRolesResult>`)
	sb.WriteString(`<IsTruncated>false</IsTruncated>`)
	sb.WriteString(`<Roles>`)

	for _, r := range roles {
		roleName := aws.ToString(r.RoleName)
		roleID := aws.ToString(r.RoleId)
		arn := aws.ToString(r.Arn)
		path := aws.ToString(r.Path)
		createDate := ""
		if r.CreateDate != nil {
			createDate = r.CreateDate.UTC().Format(time.RFC3339)
		}
		fmt.Fprintf(&sb, `<member>`)
		fmt.Fprintf(&sb, `<RoleName>%s</RoleName>`, xmlEscape(roleName))
		fmt.Fprintf(&sb, `<RoleId>%s</RoleId>`, xmlEscape(roleID))
		fmt.Fprintf(&sb, `<Arn>%s</Arn>`, xmlEscape(arn))
		fmt.Fprintf(&sb, `<Path>%s</Path>`, xmlEscape(path))
		fmt.Fprintf(&sb, `<CreateDate>%s</CreateDate>`, xmlEscape(createDate))
		fmt.Fprintf(&sb, `</member>`)
	}

	sb.WriteString(`</Roles>`)
	sb.WriteString(`</ListRolesResult>`)
	sb.WriteString(`<ResponseMetadata><RequestId>demo-request-id</RequestId></ResponseMetadata>`)
	sb.WriteString(`</ListRolesResponse>`)
	return sb.String()
}

// ---------------------------------------------------------------------------
// ECS (awsjson11)
// ---------------------------------------------------------------------------

func registerECSHandlers(t *Transport) {
	t.Handle("ecs", "ListClusters", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["ecs"]()
		clusters := ExtractSDK[ecstypes.Cluster](resources)

		arns := make([]string, 0, len(clusters))
		for _, c := range clusters {
			if c.ClusterArn != nil {
				arns = append(arns, *c.ClusterArn)
			}
		}

		resp := map[string]interface{}{"clusterArns": arns}
		return JSONResponse(resp)
	})
}

// ---------------------------------------------------------------------------
// DynamoDB (awsjson10)
// ---------------------------------------------------------------------------

func registerDynamoDBHandlers(t *Transport) {
	t.Handle("dynamodb", "ListTables", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["ddb"]()

		tableNames := make([]string, 0, len(resources))
		for _, r := range resources {
			tableNames = append(tableNames, r.ID)
		}

		resp := map[string]interface{}{"TableNames": tableNames}
		return JSONResponse(resp)
	})
}

