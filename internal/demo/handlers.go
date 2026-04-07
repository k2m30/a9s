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
	"github.com/k2m30/a9s/v3/internal/resource"
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
		page, nextToken, err := Paginate(fns, 20, marker)
		if err != nil {
			return &http.Response{
				StatusCode: 400,
				Status:     "400 Bad Request",
				Body:       io.NopCloser(strings.NewReader(err.Error())),
			}, nil
		}

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

	t.Handle("ec2", "DescribeInstanceStatus", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["ec2"]()
		xml := buildDescribeInstanceStatusXML(resources)
		return XMLResponse(xml), nil
	})

	t.Handle("ec2", "DescribeVolumes", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["ebs"]()
		volumes := ExtractSDK[ec2types.Volume](resources)

		var items strings.Builder
		for _, v := range volumes {
			items.WriteString("<item>")
			items.WriteString(ec2ItemXML(v))
			items.WriteString("</item>")
		}

		return XMLResponse(ec2QueryXML("DescribeVolumes", "volumeSet", items.String())), nil
	})

	t.Handle("ec2", "DescribeSnapshots", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["ebs-snap"]()
		snapshots := ExtractSDK[ec2types.Snapshot](resources)

		var items strings.Builder
		for _, s := range snapshots {
			items.WriteString("<item>")
			items.WriteString(ec2ItemXML(s))
			items.WriteString("</item>")
		}

		return XMLResponse(ec2QueryXML("DescribeSnapshots", "snapshotSet", items.String())), nil
	})

	t.Handle("ec2", "DescribeImages", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["ami"]()
		images := ExtractSDK[ec2types.Image](resources)

		var items strings.Builder
		for _, img := range images {
			items.WriteString("<item>")
			items.WriteString(ec2ItemXML(img))
			items.WriteString("</item>")
		}

		return XMLResponse(ec2QueryXML("DescribeImages", "imagesSet", items.String())), nil
	})
}

func buildDescribeInstancesXML(instances []ec2types.Instance) string {
	var sb strings.Builder
	sb.WriteString(`<DescribeInstancesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">`)
	sb.WriteString(`<requestId>demo-request-id</requestId><reservationSet>`)
	for i, inst := range instances {
		fmt.Fprintf(&sb, `<item><reservationId>r-demo%04d</reservationId><instancesSet><item>`, i+1)
		sb.WriteString(ec2ItemXML(inst))
		sb.WriteString(`</item></instancesSet></item>`)
	}
	sb.WriteString(`</reservationSet></DescribeInstancesResponse>`)
	return sb.String()
}

func buildDescribeInstanceStatusXML(resources []resource.Resource) string {
	var sb strings.Builder
	sb.WriteString(`<DescribeInstanceStatusResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">`)
	sb.WriteString(`<requestId>demo-request-id</requestId><instanceStatusSet>`)
	for _, r := range resources {
		sysStatus := r.Fields["system_status"]
		instStatus := r.Fields["instance_status"]
		if sysStatus == "" && instStatus == "" {
			continue
		}
		if sysStatus == "" {
			sysStatus = "not-applicable"
		}
		if instStatus == "" {
			instStatus = "not-applicable"
		}
		fmt.Fprintf(&sb, `<item><instanceId>%s</instanceId>`, xmlEscape(r.ID))
		fmt.Fprintf(&sb, `<systemStatus><status>%s</status></systemStatus>`, xmlEscape(sysStatus))
		fmt.Fprintf(&sb, `<instanceStatus><status>%s</status></instanceStatus>`, xmlEscape(instStatus))
		sb.WriteString(`</item>`)
	}
	sb.WriteString(`</instanceStatusSet></DescribeInstanceStatusResponse>`)
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

