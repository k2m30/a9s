package demo

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	demoData["role"] = iamRoleFixtures
	demoData["policy"] = iamPolicyFixtures
	demoData["iam-user"] = iamUserFixtures
	demoData["iam-group"] = iamGroupFixtures
	demoData["waf"] = wafFixtures

	RegisterChildDemo("role_policies", func(parentCtx map[string]string) []resource.Resource {
		return rolePolicyFixtures()
	})
	RegisterChildDemo("iam_group_members", func(parentCtx map[string]string) []resource.Resource {
		return iamGroupMemberFixtures()
	})
}

// iamRoleFixtures returns demo IAM Role fixtures.
func iamRoleFixtures() []resource.Resource {
	roles := []resource.Resource{
		{
			ID:     "acme-eks-node-role",
			Name:   "acme-eks-node-role",
			Status: "",
			Fields: map[string]string{
				"role_name":   "acme-eks-node-role",
				"role_id":     "AROAEXAMPLE111111111",
				"path":        "/",
				"create_date": "2025-06-15T10:30:00+00:00",
				"description": "Role for EKS managed node groups",
			},
			RawStruct: iamtypes.Role{
				RoleName:                 aws.String("acme-eks-node-role"),
				RoleId:                   aws.String("AROAEXAMPLE111111111"),
				Arn:                      aws.String("arn:aws:iam::123456789012:role/acme-eks-node-role"),
				Path:                     aws.String("/"),
				CreateDate:               aws.Time(mustParseTime("2025-06-15T10:30:00+00:00")),
				Description:              aws.String("Role for EKS managed node groups"),
				AssumeRolePolicyDocument: aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"ec2.amazonaws.com"},"Action":"sts:AssumeRole"}]}`),
				MaxSessionDuration:       aws.Int32(3600),
				PermissionsBoundary: &iamtypes.AttachedPermissionsBoundary{
					PermissionsBoundaryArn:  aws.String("arn:aws:iam::aws:policy/PowerUserAccess"),
					PermissionsBoundaryType: iamtypes.PermissionsBoundaryAttachmentTypePolicy,
				},
				RoleLastUsed: &iamtypes.RoleLastUsed{
					LastUsedDate: aws.Time(time.Date(2026, 3, 21, 9, 0, 0, 0, time.UTC)),
					Region:       aws.String("us-east-1"),
				},
				Tags: []iamtypes.Tag{
					{Key: aws.String("Environment"), Value: aws.String("production")},
					{Key: aws.String("Team"), Value: aws.String("platform")},
				},
			},
		},
		{
			ID:     "acme-lambda-execution",
			Name:   "acme-lambda-execution",
			Status: "",
			Fields: map[string]string{
				"role_name":   "acme-lambda-execution",
				"role_id":     "AROAEXAMPLE222222222",
				"path":        "/service-role/",
				"create_date": "2025-03-10T08:15:00+00:00",
				"description": "Execution role for Lambda functions",
			},
			RawStruct: iamtypes.Role{
				RoleName:    aws.String("acme-lambda-execution"),
				RoleId:      aws.String("AROAEXAMPLE222222222"),
				Arn:         aws.String(prodLambdaRoleARN),
				Path:        aws.String("/service-role/"),
				CreateDate:  aws.Time(mustParseTime("2025-03-10T08:15:00+00:00")),
				Description: aws.String("Execution role for Lambda functions"),
			},
		},
		{
			// Lambda navigable-field alias: lambdatypes.FunctionConfiguration.Role
			// stores the full ARN. The infrastructure integrity test (Part 3) checks
			// that the extracted field value matches a role fixture ID. This entry
			// provides that ARN-keyed ID while the name-keyed entry above satisfies
			// arnLeaf-based cross-reference checks.
			// RoleName is set to the ARN so the IAM demo handler generates a unique
			// resource entry distinct from the name-keyed fixture above.
			ID:     prodLambdaRoleARN,
			Name:   prodLambdaRoleARN,
			Status: "",
			Fields: map[string]string{
				"role_name":   prodLambdaRoleARN,
				"role_id":     "AROAEXAMPLE222222223",
				"path":        "/service-role/",
				"create_date": "2025-03-10T08:15:00+00:00",
				"description": "Lambda execution role ARN alias (navigable-field cross-reference)",
			},
			RawStruct: iamtypes.Role{
				RoleName:    aws.String(prodLambdaRoleARN),
				RoleId:      aws.String("AROAEXAMPLE222222223"),
				Arn:         aws.String(prodLambdaRoleARN),
				Path:        aws.String("/service-role/"),
				CreateDate:  aws.Time(mustParseTime("2025-03-10T08:15:00+00:00")),
				Description: aws.String("Lambda execution role ARN alias (navigable-field cross-reference)"),
			},
		},
		{
			ID:     "acme-ci-deploy-role",
			Name:   "acme-ci-deploy-role",
			Status: "",
			Fields: map[string]string{
				"role_name":   "acme-ci-deploy-role",
				"role_id":     "AROAEXAMPLE333333333",
				"path":        "/",
				"create_date": "2025-01-20T14:00:00+00:00",
				"description": "CI/CD deployment role for CodePipeline",
			},
			RawStruct: iamtypes.Role{
				RoleName:    aws.String("acme-ci-deploy-role"),
				RoleId:      aws.String("AROAEXAMPLE333333333"),
				Arn:         aws.String("arn:aws:iam::123456789012:role/acme-ci-deploy-role"),
				Path:        aws.String("/"),
				CreateDate:  aws.Time(mustParseTime("2025-01-20T14:00:00+00:00")),
				Description: aws.String("CI/CD deployment role for CodePipeline"),
			},
		},
		{
			ID:     "acme-rds-monitoring",
			Name:   "acme-rds-monitoring",
			Status: "",
			Fields: map[string]string{
				"role_name":   "acme-rds-monitoring",
				"role_id":     "AROAEXAMPLE444444444",
				"path":        "/",
				"create_date": "2025-04-05T16:45:00+00:00",
				"description": "Enhanced monitoring role for RDS instances",
			},
			RawStruct: iamtypes.Role{
				RoleName:    aws.String("acme-rds-monitoring"),
				RoleId:      aws.String("AROAEXAMPLE444444444"),
				Arn:         aws.String("arn:aws:iam::123456789012:role/acme-rds-monitoring"),
				Path:        aws.String("/"),
				CreateDate:  aws.Time(mustParseTime("2025-04-05T16:45:00+00:00")),
				Description: aws.String("Enhanced monitoring role for RDS instances"),
			},
		},
		{
			ID:     "deploy-bot",
			Name:   "deploy-bot",
			Status: "",
			Fields: map[string]string{
				"role_name":   "deploy-bot",
				"role_id":     "AROAEXAMPLE555555555",
				"path":        "/",
				"create_date": "2025-05-01T10:00:00+00:00",
				"description": "Automation role used by deployment bot sessions",
			},
			RawStruct: iamtypes.Role{
				RoleName:    aws.String("deploy-bot"),
				RoleId:      aws.String("AROAEXAMPLE555555555"),
				Arn:         aws.String("arn:aws:iam::123456789012:role/deploy-bot"),
				Path:        aws.String("/"),
				CreateDate:  aws.Time(mustParseTime("2025-05-01T10:00:00+00:00")),
				Description: aws.String("Automation role used by deployment bot sessions"),
			},
		},
		{
			ID:     "ci-runner",
			Name:   "ci-runner",
			Status: "",
			Fields: map[string]string{
				"role_name":   "ci-runner",
				"role_id":     "AROAEXAMPLE666666666",
				"path":        "/",
				"create_date": "2025-05-10T11:00:00+00:00",
				"description": "Automation role used by CI runner sessions",
			},
			RawStruct: iamtypes.Role{
				RoleName:    aws.String("ci-runner"),
				RoleId:      aws.String("AROAEXAMPLE666666666"),
				Arn:         aws.String("arn:aws:iam::123456789012:role/ci-runner"),
				Path:        aws.String("/"),
				CreateDate:  aws.Time(mustParseTime("2025-05-10T11:00:00+00:00")),
				Description: aws.String("Automation role used by CI runner sessions"),
			},
		},
		{
			ID:     "monitoring-agent",
			Name:   "monitoring-agent",
			Status: "",
			Fields: map[string]string{
				"role_name":   "monitoring-agent",
				"role_id":     "AROAEXAMPLE777777777",
				"path":        "/",
				"create_date": "2025-05-20T12:00:00+00:00",
				"description": "Read-only role used by monitoring collectors",
			},
			RawStruct: iamtypes.Role{
				RoleName:    aws.String("monitoring-agent"),
				RoleId:      aws.String("AROAEXAMPLE777777777"),
				Arn:         aws.String("arn:aws:iam::123456789012:role/monitoring-agent"),
				Path:        aws.String("/"),
				CreateDate:  aws.Time(mustParseTime("2025-05-20T12:00:00+00:00")),
				Description: aws.String("Read-only role used by monitoring collectors"),
			},
		},
		{
			ID:     "acme-glue-role",
			Name:   "acme-glue-role",
			Status: "",
			Fields: map[string]string{
				"role_name":   "acme-glue-role",
				"role_id":     "AROAEXAMPLE888888888",
				"path":        "/",
				"create_date": "2025-07-01T09:00:00+00:00",
				"description": "Service role for Glue ETL jobs",
			},
			RawStruct: iamtypes.Role{
				RoleName:    aws.String("acme-glue-role"),
				RoleId:      aws.String("AROAEXAMPLE888888888"),
				Arn:         aws.String("arn:aws:iam::123456789012:role/acme-glue-role"),
				Path:        aws.String("/"),
				CreateDate:  aws.Time(mustParseTime("2025-07-01T09:00:00+00:00")),
				Description: aws.String("Service role for Glue ETL jobs"),
			},
		},
		// ARN-keyed alias fixtures for EKS node group NodeRole navigable field.
		// The ng fixtures store NodeRole as a full ARN; the integrity test checks that
		// the extracted ARN matches a role fixture ID. These entries provide that mapping.
		{
			ID:   "arn:aws:iam::123456789012:role/eks-node-role",
			Name: "arn:aws:iam::123456789012:role/eks-node-role",
			Fields: map[string]string{
				"role_name":   "arn:aws:iam::123456789012:role/eks-node-role",
				"role_id":     "AROAEXAMPLENGNODE001",
				"path":        "/",
				"create_date": "2025-02-20T12:00:00+00:00",
				"description": "EKS node role ARN alias (navigable-field cross-reference)",
			},
			RawStruct: iamtypes.Role{
				RoleName:   aws.String("arn:aws:iam::123456789012:role/eks-node-role"),
				RoleId:     aws.String("AROAEXAMPLENGNODE001"),
				Arn:        aws.String("arn:aws:iam::123456789012:role/eks-node-role"),
				Path:       aws.String("/"),
				CreateDate: aws.Time(mustParseTime("2025-02-20T12:00:00+00:00")),
				Description: aws.String("EKS node role ARN alias (navigable-field cross-reference)"),
			},
		},
		{
			ID:   "arn:aws:iam::123456789012:role/eks-gpu-node-role",
			Name: "arn:aws:iam::123456789012:role/eks-gpu-node-role",
			Fields: map[string]string{
				"role_name":   "arn:aws:iam::123456789012:role/eks-gpu-node-role",
				"role_id":     "AROAEXAMPLENGNODE002",
				"path":        "/",
				"create_date": "2025-04-05T09:30:00+00:00",
				"description": "EKS GPU node role ARN alias (navigable-field cross-reference)",
			},
			RawStruct: iamtypes.Role{
				RoleName:   aws.String("arn:aws:iam::123456789012:role/eks-gpu-node-role"),
				RoleId:     aws.String("AROAEXAMPLENGNODE002"),
				Arn:        aws.String("arn:aws:iam::123456789012:role/eks-gpu-node-role"),
				Path:       aws.String("/"),
				CreateDate: aws.Time(mustParseTime("2025-04-05T09:30:00+00:00")),
				Description: aws.String("EKS GPU node role ARN alias (navigable-field cross-reference)"),
			},
		},
	}

	// Extra roles required by ct-event-detail wireframe cases A, B, F, G, I (T029).
	for _, rd := range []struct{ id, desc string }{
		{"KarpenterNodeRole", "Karpenter node provisioner role (ct-events case A cross-ref)"},
		{"AWSReservedSSO_AdminAccess_3c4d5e6f7a8b9c0d", "SSO AdminAccess reserved role (ct-events case B cross-ref)"},
		{"eks-checkout-svc-sa", "EKS IRSA service account role for checkout service (ct-events case F cross-ref)"},
		{"CiBuildRole", "CI/CD build role for cross-account artifact publishing (ct-events case G cross-ref)"},
		{"DataPipelineRole", "Data pipeline ETL role for VPCE access (ct-events case I cross-ref)"},
	} {
		roles = append(roles, resource.Resource{
			ID:     rd.id,
			Name:   rd.id,
			Status: "",
			Fields: map[string]string{
				"role_name":   rd.id,
				"role_id":     fmt.Sprintf("AROACT029%s", rd.id[:8]),
				"path":        "/",
				"create_date": "2026-01-01T00:00:00+00:00",
				"description": rd.desc,
			},
			RawStruct: iamtypes.Role{
				RoleName:    aws.String(rd.id),
				RoleId:      aws.String(fmt.Sprintf("AROACT029%s", rd.id[:8])),
				Arn:         aws.String(fmt.Sprintf("arn:aws:iam::111111111111:role/%s", rd.id)),
				Path:        aws.String("/"),
				CreateDate:  aws.Time(mustParseTime("2026-01-01T00:00:00+00:00")),
				Description: aws.String(rd.desc),
			},
		})
	}

	// Generate 18 more roles to reach 25 total
	paths := []string{"/", "/service-role/", "/", "/aws-service-role/"}
	for i := 0; i < 18; i++ {
		name := roleNamePool[i%len(roleNamePool)]
		desc := roleDescPool[i%len(roleDescPool)]
		roleID := fmt.Sprintf("AROAEXAMPLE%09d", 500+i)
		path := paths[i%len(paths)]
		createDate := fmt.Sprintf("2025-%02d-%02dT%02d:00:00+00:00", 1+(i%12), 1+i, 8+(i%12))
		roles = append(roles, resource.Resource{
			ID:     name,
			Name:   name,
			Status: "",
			Fields: map[string]string{
				"role_name":   name,
				"role_id":     roleID,
				"path":        path,
				"create_date": createDate,
				"description": desc,
			},
			RawStruct: iamtypes.Role{
				RoleName:    aws.String(name),
				RoleId:      aws.String(roleID),
				Arn:         aws.String(fmt.Sprintf("arn:aws:iam::123456789012:role%s%s", path, name)),
				Path:        aws.String(path),
				CreateDate:  aws.Time(mustParseTime(createDate)),
				Description: aws.String(desc),
			},
		})
	}

	return roles
}

// iamPolicyFixtures returns demo IAM Policy fixtures.
func iamPolicyFixtures() []resource.Resource {
	policies := []resource.Resource{
		{
			ID:     "acme-s3-read-only",
			Name:   "acme-s3-read-only",
			Status: "",
			Fields: map[string]string{
				"policy_name":      "acme-s3-read-only",
				"policy_id":        "ANPAEXAMPLE111111111",
				"attachment_count": "5",
				"path":             "/",
				"create_date":      "2025-02-10T09:00:00+00:00",
			},
			RawStruct: iamtypes.Policy{
				PolicyName:                     aws.String("acme-s3-read-only"),
				PolicyId:                       aws.String("ANPAEXAMPLE111111111"),
				Arn:                            aws.String("arn:aws:iam::123456789012:policy/acme-s3-read-only"),
				AttachmentCount:                aws.Int32(5),
				Path:                           aws.String("/"),
				CreateDate:                     aws.Time(mustParseTime("2025-02-10T09:00:00+00:00")),
				DefaultVersionId:               aws.String("v3"),
				Description:                    aws.String("Allows EC2 and S3 read access"),
				PermissionsBoundaryUsageCount:  aws.Int32(0),
				Tags:                           []iamtypes.Tag{{Key: aws.String("Environment"), Value: aws.String("production")}},
				UpdateDate:                     aws.Time(time.Date(2026, 2, 10, 14, 30, 0, 0, time.UTC)),
			},
		},
		{
			ID:     "acme-deploy-policy",
			Name:   "acme-deploy-policy",
			Status: "",
			Fields: map[string]string{
				"policy_name":      "acme-deploy-policy",
				"policy_id":        "ANPAEXAMPLE222222222",
				"attachment_count": "3",
				"path":             "/",
				"create_date":      "2025-01-15T11:30:00+00:00",
			},
			RawStruct: iamtypes.Policy{
				PolicyName:      aws.String("acme-deploy-policy"),
				PolicyId:        aws.String("ANPAEXAMPLE222222222"),
				Arn:             aws.String("arn:aws:iam::123456789012:policy/acme-deploy-policy"),
				AttachmentCount: aws.Int32(3),
				Path:            aws.String("/"),
				CreateDate:      aws.Time(mustParseTime("2025-01-15T11:30:00+00:00")),
			},
		},
		{
			ID:     "acme-secrets-access",
			Name:   "acme-secrets-access",
			Status: "",
			Fields: map[string]string{
				"policy_name":      "acme-secrets-access",
				"policy_id":        "ANPAEXAMPLE333333333",
				"attachment_count": "2",
				"path":             "/",
				"create_date":      "2025-05-20T13:15:00+00:00",
			},
			RawStruct: iamtypes.Policy{
				PolicyName:      aws.String("acme-secrets-access"),
				PolicyId:        aws.String("ANPAEXAMPLE333333333"),
				Arn:             aws.String("arn:aws:iam::123456789012:policy/acme-secrets-access"),
				AttachmentCount: aws.Int32(2),
				Path:            aws.String("/"),
				CreateDate:      aws.Time(mustParseTime("2025-05-20T13:15:00+00:00")),
			},
		},
		{
			ID:     "acme-cloudwatch-logs",
			Name:   "acme-cloudwatch-logs",
			Status: "",
			Fields: map[string]string{
				"policy_name":      "acme-cloudwatch-logs",
				"policy_id":        "ANPAEXAMPLE444444444",
				"attachment_count": "8",
				"path":             "/",
				"create_date":      "2024-11-01T07:45:00+00:00",
			},
			RawStruct: iamtypes.Policy{
				PolicyName:      aws.String("acme-cloudwatch-logs"),
				PolicyId:        aws.String("ANPAEXAMPLE444444444"),
				Arn:             aws.String("arn:aws:iam::123456789012:policy/acme-cloudwatch-logs"),
				AttachmentCount: aws.Int32(8),
				Path:            aws.String("/"),
				CreateDate:      aws.Time(mustParseTime("2024-11-01T07:45:00+00:00")),
			},
		},
	}

	// AWS-managed AdministratorAccess policy — required by ct-events Case K
	// (AttachUserPolicy ResourceName cross-reference).
	policies = append(policies, resource.Resource{
		ID:     "arn:aws:iam::aws:policy/AdministratorAccess",
		Name:   "AdministratorAccess",
		Status: "",
		Fields: map[string]string{
			"policy_name":      "AdministratorAccess",
			"policy_id":        "ANPAEXAMPLE000000001",
			"attachment_count": "12",
			"path":             "/",
			"create_date":      "2015-02-06T18:40:16+00:00",
		},
		RawStruct: iamtypes.Policy{
			PolicyName:      aws.String("AdministratorAccess"),
			PolicyId:        aws.String("ANPAEXAMPLE000000001"),
			Arn:             aws.String("arn:aws:iam::aws:policy/AdministratorAccess"),
			AttachmentCount: aws.Int32(12),
			Path:            aws.String("/"),
			CreateDate:      aws.Time(mustParseTime("2015-02-06T18:40:16+00:00")),
		},
	})

	// Generate 18 more policies to reach 23 total
	for i := 0; i < 18; i++ {
		name := policyNamePool[i]
		policyID := fmt.Sprintf("ANPAEXAMPLE%09d", 500+i)
		attachCount := int32(1 + (i % 6))
		createDate := fmt.Sprintf("2025-%02d-%02dT%02d:00:00+00:00", 1+(i%12), 1+i, 9+(i%10))
		policies = append(policies, resource.Resource{
			ID:     name,
			Name:   name,
			Status: "",
			Fields: map[string]string{
				"policy_name":      name,
				"policy_id":        policyID,
				"attachment_count": fmt.Sprintf("%d", attachCount),
				"path":             "/",
				"create_date":      createDate,
			},
			RawStruct: iamtypes.Policy{
				PolicyName:      aws.String(name),
				PolicyId:        aws.String(policyID),
				Arn:             aws.String("arn:aws:iam::123456789012:policy/" + name),
				AttachmentCount: aws.Int32(attachCount),
				Path:            aws.String("/"),
				CreateDate:      aws.Time(mustParseTime(createDate)),
			},
		})
	}

	return policies
}

// iamUserFixtures returns demo IAM User fixtures.
func iamUserFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "alice.johnson",
			Name:   "alice.johnson",
			Status: "",
			Fields: map[string]string{
				"user_name":          "alice.johnson",
				"user_id":            "AIDAEXAMPLE111111111",
				"path":               "/",
				"create_date":        "2024-06-15T09:00:00+00:00",
				"password_last_used": "2026-03-20T14:22:00+00:00",
			},
			RawStruct: iamtypes.User{
				UserName:         aws.String("alice.johnson"),
				UserId:           aws.String("AIDAEXAMPLE111111111"),
				Arn:              aws.String("arn:aws:iam::123456789012:user/alice.johnson"),
				Path:             aws.String("/"),
				CreateDate:       aws.Time(mustParseTime("2024-06-15T09:00:00+00:00")),
				PasswordLastUsed: aws.Time(mustParseTime("2026-03-20T14:22:00+00:00")),
				PermissionsBoundary: &iamtypes.AttachedPermissionsBoundary{
					PermissionsBoundaryArn:  aws.String("arn:aws:iam::aws:policy/PowerUserAccess"),
					PermissionsBoundaryType: iamtypes.PermissionsBoundaryAttachmentTypePolicy,
				},
				Tags: []iamtypes.Tag{
					{Key: aws.String("Department"), Value: aws.String("Engineering")},
					{Key: aws.String("CostCenter"), Value: aws.String("CC-1234")},
				},
			},
		},
		{
			ID:     "bob.smith",
			Name:   "bob.smith",
			Status: "",
			Fields: map[string]string{
				"user_name":          "bob.smith",
				"user_id":            "AIDAEXAMPLE222222222",
				"path":               "/",
				"create_date":        "2024-09-01T10:30:00+00:00",
				"password_last_used": "2026-03-19T08:55:00+00:00",
			},
			RawStruct: iamtypes.User{
				UserName:         aws.String("bob.smith"),
				UserId:           aws.String("AIDAEXAMPLE222222222"),
				Arn:              aws.String("arn:aws:iam::123456789012:user/bob.smith"),
				Path:             aws.String("/"),
				CreateDate:       aws.Time(mustParseTime("2024-09-01T10:30:00+00:00")),
				PasswordLastUsed: aws.Time(mustParseTime("2026-03-19T08:55:00+00:00")),
			},
		},
		{
			ID:     "ci-service-account",
			Name:   "ci-service-account",
			Status: "",
			Fields: map[string]string{
				"user_name":          "ci-service-account",
				"user_id":            "AIDAEXAMPLE333333333",
				"path":               "/service-accounts/",
				"create_date":        "2025-01-10T12:00:00+00:00",
				"password_last_used": "Never",
			},
			RawStruct: iamtypes.User{
				UserName:   aws.String("ci-service-account"),
				UserId:     aws.String("AIDAEXAMPLE333333333"),
				Arn:        aws.String("arn:aws:iam::123456789012:user/service-accounts/ci-service-account"),
				Path:       aws.String("/service-accounts/"),
				CreateDate: aws.Time(mustParseTime("2025-01-10T12:00:00+00:00")),
				// PasswordLastUsed is nil for programmatic-only accounts
			},
		},
		// CT-event cross-reference user required by ctdetail nav tests (T029).
		// Case C references "arn:aws:iam::333333333333:user/bob" — the bare user name
		// "bob" must resolve in demo.GetResources("iam-user") for assertTargetResolves.
		{
			ID:     "bob",
			Name:   "bob",
			Status: "",
			Fields: map[string]string{
				"user_name":          "bob",
				"user_id":            "AIDAEXAMPLE444444444",
				"path":               "/",
				"create_date":        "2025-11-01T09:00:00+00:00",
				"password_last_used": "2026-04-07T16:00:00+00:00",
			},
			RawStruct: iamtypes.User{
				UserName:         aws.String("bob"),
				UserId:           aws.String("AIDAEXAMPLE444444444"),
				Arn:              aws.String("arn:aws:iam::333333333333:user/bob"),
				Path:             aws.String("/"),
				CreateDate:       aws.Time(mustParseTime("2025-11-01T09:00:00+00:00")),
				PasswordLastUsed: aws.Time(mustParseTime("2026-04-07T16:00:00+00:00")),
			},
		},
		// CT-event cross-reference user required by Case J (CreateUser demo fixture).
		// Case J resourceName "charlie" must resolve in demo.GetResources("iam-user").
		{
			ID:     "charlie",
			Name:   "charlie",
			Status: "",
			Fields: map[string]string{
				"user_name":          "charlie",
				"user_id":            "AIDAEXAMPLE555555555",
				"path":               "/",
				"create_date":        "2026-04-07T15:10:05+00:00",
				"password_last_used": "Never",
			},
			RawStruct: iamtypes.User{
				UserName:   aws.String("charlie"),
				UserId:     aws.String("AIDAEXAMPLE555555555"),
				Arn:        aws.String("arn:aws:iam::123456789012:user/charlie"),
				Path:       aws.String("/"),
				CreateDate: aws.Time(mustParseTime("2026-04-07T15:10:05+00:00")),
			},
		},
	}
}

// iamGroupFixtures returns demo IAM Group fixtures.
func iamGroupFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "admins",
			Name:   "admins",
			Status: "",
			Fields: map[string]string{
				"group_name":  "admins",
				"group_id":    "AGPAEXAMPLE111111111",
				"path":        "/",
				"create_date": "2024-03-01T08:00:00+00:00",
				"arn":         "arn:aws:iam::123456789012:group/admins",
			},
			RawStruct: iamtypes.Group{
				GroupName:  aws.String("admins"),
				GroupId:    aws.String("AGPAEXAMPLE111111111"),
				Arn:        aws.String("arn:aws:iam::123456789012:group/admins"),
				Path:       aws.String("/"),
				CreateDate: aws.Time(mustParseTime("2024-03-01T08:00:00+00:00")),
			},
		},
		{
			ID:     "developers",
			Name:   "developers",
			Status: "",
			Fields: map[string]string{
				"group_name":  "developers",
				"group_id":    "AGPAEXAMPLE222222222",
				"path":        "/",
				"create_date": "2024-03-01T08:05:00+00:00",
				"arn":         "arn:aws:iam::123456789012:group/developers",
			},
			RawStruct: iamtypes.Group{
				GroupName:  aws.String("developers"),
				GroupId:    aws.String("AGPAEXAMPLE222222222"),
				Arn:        aws.String("arn:aws:iam::123456789012:group/developers"),
				Path:       aws.String("/"),
				CreateDate: aws.Time(mustParseTime("2024-03-01T08:05:00+00:00")),
			},
		},
		{
			ID:     "readonly",
			Name:   "readonly",
			Status: "",
			Fields: map[string]string{
				"group_name":  "readonly",
				"group_id":    "AGPAEXAMPLE333333333",
				"path":        "/",
				"create_date": "2024-03-01T08:10:00+00:00",
				"arn":         "arn:aws:iam::123456789012:group/readonly",
			},
			RawStruct: iamtypes.Group{
				GroupName:  aws.String("readonly"),
				GroupId:    aws.String("AGPAEXAMPLE333333333"),
				Arn:        aws.String("arn:aws:iam::123456789012:group/readonly"),
				Path:       aws.String("/"),
				CreateDate: aws.Time(mustParseTime("2024-03-01T08:10:00+00:00")),
			},
		},
	}
}

// rolePolicyFixtures returns demo role policy fixtures (5 managed + 2 inline).
func rolePolicyFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "arn:aws:iam::aws:policy/ReadOnlyAccess",
			Name:   "ReadOnlyAccess",
			Status: "",
			Fields: map[string]string{
				"policy_name": "ReadOnlyAccess",
				"policy_arn":  "arn:aws:iam::aws:policy/ReadOnlyAccess",
				"policy_type": "Managed",
			},
			RawStruct: awsclient.RolePolicyRow{
				PolicyName: "ReadOnlyAccess",
				PolicyArn:  "arn:aws:iam::aws:policy/ReadOnlyAccess",
				PolicyType: "Managed",
			},
		},
		{
			ID:     "arn:aws:iam::aws:policy/CloudWatchFullAccess",
			Name:   "CloudWatchFullAccess",
			Status: "",
			Fields: map[string]string{
				"policy_name": "CloudWatchFullAccess",
				"policy_arn":  "arn:aws:iam::aws:policy/CloudWatchFullAccess",
				"policy_type": "Managed",
			},
			RawStruct: awsclient.RolePolicyRow{
				PolicyName: "CloudWatchFullAccess",
				PolicyArn:  "arn:aws:iam::aws:policy/CloudWatchFullAccess",
				PolicyType: "Managed",
			},
		},
		{
			ID:     "arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess",
			Name:   "AmazonS3ReadOnlyAccess",
			Status: "",
			Fields: map[string]string{
				"policy_name": "AmazonS3ReadOnlyAccess",
				"policy_arn":  "arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess",
				"policy_type": "Managed",
			},
			RawStruct: awsclient.RolePolicyRow{
				PolicyName: "AmazonS3ReadOnlyAccess",
				PolicyArn:  "arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess",
				PolicyType: "Managed",
			},
		},
		{
			ID:     "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
			Name:   "AmazonEKSWorkerNodePolicy",
			Status: "",
			Fields: map[string]string{
				"policy_name": "AmazonEKSWorkerNodePolicy",
				"policy_arn":  "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
				"policy_type": "Managed",
			},
			RawStruct: awsclient.RolePolicyRow{
				PolicyName: "AmazonEKSWorkerNodePolicy",
				PolicyArn:  "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
				PolicyType: "Managed",
			},
		},
		{
			ID:     "arn:aws:iam::aws:policy/AdministratorAccess",
			Name:   "AdministratorAccess",
			Status: "failed",
			Fields: map[string]string{
				"policy_name": "AdministratorAccess",
				"policy_arn":  "arn:aws:iam::aws:policy/AdministratorAccess",
				"policy_type": "Managed",
			},
			RawStruct: awsclient.RolePolicyRow{
				PolicyName: "AdministratorAccess",
				PolicyArn:  "arn:aws:iam::aws:policy/AdministratorAccess",
				PolicyType: "Managed",
			},
		},
		{
			ID:     "trust-policy",
			Name:   "trust-policy",
			Status: "terminated",
			Fields: map[string]string{
				"policy_name": "trust-policy",
				"policy_arn":  "",
				"policy_type": "Inline",
			},
			RawStruct: awsclient.RolePolicyRow{
				PolicyName: "trust-policy",
				PolicyArn:  "",
				PolicyType: "Inline",
			},
		},
		{
			ID:     "logging-policy",
			Name:   "logging-policy",
			Status: "terminated",
			Fields: map[string]string{
				"policy_name": "logging-policy",
				"policy_arn":  "",
				"policy_type": "Inline",
			},
			RawStruct: awsclient.RolePolicyRow{
				PolicyName: "logging-policy",
				PolicyArn:  "",
				PolicyType: "Inline",
			},
		},
	}
}

// iamGroupMemberFixtures returns demo IAM group member fixtures.
func iamGroupMemberFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "alice.johnson",
			Name:   "alice.johnson",
			Status: "",
			Fields: map[string]string{
				"user_name":          "alice.johnson",
				"user_id":            "AIDAEXAMPLE111111111",
				"arn":                "arn:aws:iam::123456789012:user/alice.johnson",
				"path":               "/",
				"create_date":        "2024-06-15 09:00",
				"password_last_used": "N/A (not in API)",
			},
			RawStruct: iamtypes.User{
				UserName: aws.String("alice.johnson"),
				UserId:   aws.String("AIDAEXAMPLE111111111"),
				Arn:      aws.String("arn:aws:iam::123456789012:user/alice.johnson"),
				Path:     aws.String("/"),
			},
		},
		{
			ID:     "bob.smith",
			Name:   "bob.smith",
			Status: "",
			Fields: map[string]string{
				"user_name":          "bob.smith",
				"user_id":            "AIDAEXAMPLE222222222",
				"arn":                "arn:aws:iam::123456789012:user/bob.smith",
				"path":               "/",
				"create_date":        "2024-09-01 10:30",
				"password_last_used": "N/A (not in API)",
			},
			RawStruct: iamtypes.User{
				UserName: aws.String("bob.smith"),
				UserId:   aws.String("AIDAEXAMPLE222222222"),
				Arn:      aws.String("arn:aws:iam::123456789012:user/bob.smith"),
				Path:     aws.String("/"),
			},
		},
		{
			ID:     "ci-service-account",
			Name:   "ci-service-account",
			Status: "",
			Fields: map[string]string{
				"user_name":          "ci-service-account",
				"user_id":            "AIDAEXAMPLE333333333",
				"arn":                "arn:aws:iam::123456789012:user/service-accounts/ci-service-account",
				"path":               "/service-accounts/",
				"create_date":        "2025-01-10 12:00",
				"password_last_used": "N/A (not in API)",
			},
			RawStruct: iamtypes.User{
				UserName: aws.String("ci-service-account"),
				UserId:   aws.String("AIDAEXAMPLE333333333"),
				Arn:      aws.String("arn:aws:iam::123456789012:user/service-accounts/ci-service-account"),
				Path:     aws.String("/service-accounts/"),
			},
		},
		{
			ID:     "deploy-bot",
			Name:   "deploy-bot",
			Status: "",
			Fields: map[string]string{
				"user_name":          "deploy-bot",
				"user_id":            "AIDAEXAMPLE444444444",
				"arn":                "arn:aws:iam::123456789012:user/deploy-bot",
				"path":               "/",
				"create_date":        "2025-02-20 16:45",
				"password_last_used": "N/A (not in API)",
			},
			RawStruct: iamtypes.User{
				UserName: aws.String("deploy-bot"),
				UserId:   aws.String("AIDAEXAMPLE444444444"),
				Arn:      aws.String("arn:aws:iam::123456789012:user/deploy-bot"),
				Path:     aws.String("/"),
			},
		},
	}
}

// wafFixtures returns demo WAF Web ACL fixtures.
func wafFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "a1b2c3d4-5678-90ab-cdef-111111111111",
			Name:   "acme-prod-api-waf",
			Status: "",
			Fields: map[string]string{
				"name":        "acme-prod-api-waf",
				"id":          "a1b2c3d4-5678-90ab-cdef-111111111111",
				"arn":         "arn:aws:wafv2:us-east-1:123456789012:regional/webacl/acme-prod-api-waf/a1b2c3d4-5678-90ab-cdef-111111111111",
				"description": "WAF for production API Gateway",
				"lock_token":  "aaaa1111-bb22-cc33-dd44-eeee5555ffff",
			},
			RawStruct: wafv2types.WebACLSummary{
				Name:        aws.String("acme-prod-api-waf"),
				Id:          aws.String("a1b2c3d4-5678-90ab-cdef-111111111111"),
				ARN:         aws.String("arn:aws:wafv2:us-east-1:123456789012:regional/webacl/acme-prod-api-waf/a1b2c3d4-5678-90ab-cdef-111111111111"),
				Description: aws.String("WAF for production API Gateway"),
				LockToken:   aws.String("aaaa1111-bb22-cc33-dd44-eeee5555ffff"),
			},
		},
		{
			ID:     "a1b2c3d4-5678-90ab-cdef-222222222222",
			Name:   "acme-cloudfront-waf",
			Status: "",
			Fields: map[string]string{
				"name":        "acme-cloudfront-waf",
				"id":          "a1b2c3d4-5678-90ab-cdef-222222222222",
				"arn":         "arn:aws:wafv2:us-east-1:123456789012:regional/webacl/acme-cloudfront-waf/a1b2c3d4-5678-90ab-cdef-222222222222",
				"description": "WAF for CloudFront distributions",
				"lock_token":  "bbbb2222-cc33-dd44-ee55-ffff6666aaaa",
			},
			RawStruct: wafv2types.WebACLSummary{
				Name:        aws.String("acme-cloudfront-waf"),
				Id:          aws.String("a1b2c3d4-5678-90ab-cdef-222222222222"),
				ARN:         aws.String("arn:aws:wafv2:us-east-1:123456789012:regional/webacl/acme-cloudfront-waf/a1b2c3d4-5678-90ab-cdef-222222222222"),
				Description: aws.String("WAF for CloudFront distributions"),
				LockToken:   aws.String("bbbb2222-cc33-dd44-ee55-ffff6666aaaa"),
			},
		},
		{
			ID:     "a1b2c3d4-5678-90ab-cdef-333333333333",
			Name:   "acme-staging-waf",
			Status: "",
			Fields: map[string]string{
				"name":        "acme-staging-waf",
				"id":          "a1b2c3d4-5678-90ab-cdef-333333333333",
				"arn":         "arn:aws:wafv2:us-east-1:123456789012:regional/webacl/acme-staging-waf/a1b2c3d4-5678-90ab-cdef-333333333333",
				"description": "WAF for staging environment ALB",
				"lock_token":  "cccc3333-dd44-ee55-ff66-aaaa7777bbbb",
			},
			RawStruct: wafv2types.WebACLSummary{
				Name:        aws.String("acme-staging-waf"),
				Id:          aws.String("a1b2c3d4-5678-90ab-cdef-333333333333"),
				ARN:         aws.String("arn:aws:wafv2:us-east-1:123456789012:regional/webacl/acme-staging-waf/a1b2c3d4-5678-90ab-cdef-333333333333"),
				Description: aws.String("WAF for staging environment ALB"),
				LockToken:   aws.String("cccc3333-dd44-ee55-ff66-aaaa7777bbbb"),
			},
		},
	}
}
