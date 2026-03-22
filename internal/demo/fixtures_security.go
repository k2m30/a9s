package demo

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	demoData["role"] = iamRoleFixtures
	demoData["policy"] = iamPolicyFixtures
	demoData["iam-user"] = iamUserFixtures
	demoData["iam-group"] = iamGroupFixtures
	demoData["waf"] = wafFixtures
}

// iamRoleFixtures returns demo IAM Role fixtures.
func iamRoleFixtures() []resource.Resource {
	return []resource.Resource{
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
				RoleName:   aws.String("acme-eks-node-role"),
				RoleId:     aws.String("AROAEXAMPLE111111111"),
				Arn:        aws.String("arn:aws:iam::123456789012:role/acme-eks-node-role"),
				Path:       aws.String("/"),
				CreateDate: aws.Time(mustParseTime("2025-06-15T10:30:00+00:00")),
				Description: aws.String("Role for EKS managed node groups"),
				AssumeRolePolicyDocument: aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"ec2.amazonaws.com"},"Action":"sts:AssumeRole"}]}`),
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
				RoleName:   aws.String("acme-lambda-execution"),
				RoleId:     aws.String("AROAEXAMPLE222222222"),
				Arn:        aws.String("arn:aws:iam::123456789012:role/service-role/acme-lambda-execution"),
				Path:       aws.String("/service-role/"),
				CreateDate: aws.Time(mustParseTime("2025-03-10T08:15:00+00:00")),
				Description: aws.String("Execution role for Lambda functions"),
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
				RoleName:   aws.String("acme-ci-deploy-role"),
				RoleId:     aws.String("AROAEXAMPLE333333333"),
				Arn:        aws.String("arn:aws:iam::123456789012:role/acme-ci-deploy-role"),
				Path:       aws.String("/"),
				CreateDate: aws.Time(mustParseTime("2025-01-20T14:00:00+00:00")),
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
				RoleName:   aws.String("acme-rds-monitoring"),
				RoleId:     aws.String("AROAEXAMPLE444444444"),
				Arn:        aws.String("arn:aws:iam::123456789012:role/acme-rds-monitoring"),
				Path:       aws.String("/"),
				CreateDate: aws.Time(mustParseTime("2025-04-05T16:45:00+00:00")),
				Description: aws.String("Enhanced monitoring role for RDS instances"),
			},
		},
	}
}

// iamPolicyFixtures returns demo IAM Policy fixtures.
func iamPolicyFixtures() []resource.Resource {
	return []resource.Resource{
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
				PolicyName:      aws.String("acme-s3-read-only"),
				PolicyId:        aws.String("ANPAEXAMPLE111111111"),
				Arn:             aws.String("arn:aws:iam::123456789012:policy/acme-s3-read-only"),
				AttachmentCount: aws.Int32(5),
				Path:            aws.String("/"),
				CreateDate:      aws.Time(mustParseTime("2025-02-10T09:00:00+00:00")),
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
