package main

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"
	smithy "github.com/aws/smithy-go"
)

// secFormatTime mirrors s3.go's formatTime but is named distinctly to avoid a
// symbol collision — every file in this package is compiled together.
func secFormatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02 15:04")
}

// iamUserData is the raw, per-account view of IAM users the checklist generator reads.
// Records AWS facts only — the checklist generator applies docs/resources/iam-user.md to
// derive expected colors/glyphs/status.
type iamUserData struct {
	Users []iamUser `json:"users"`
}

type iamUser struct {
	UserName         string         `json:"user_name"`
	Arn              string         `json:"arn"`
	CreateDate       string         `json:"create_date,omitempty"`
	PasswordLastUsed string         `json:"password_last_used,omitempty"`
	AccessKeys       []iamAccessKey `json:"access_keys"`
	LoginProfile     iamOutcome     `json:"login_profile"`
	MFADeviceCount   int            `json:"mfa_device_count"`
}

// iamAccessKey captures ListAccessKeys + GetAccessKeyLastUsed per key.
type iamAccessKey struct {
	AccessKeyId  string `json:"access_key_id"`
	Status       string `json:"status"`
	CreateDate   string `json:"create_date,omitempty"`
	LastUsedDate string `json:"last_used_date,omitempty"`
}

// iamOutcome captures an outcome/error-code pair for a per-item describe call
// that either succeeds with no useful payload (login profile exists) or fails
// with a recorded error code (e.g. NoSuchEntity when no login profile exists).
type iamOutcome struct {
	Outcome   string `json:"outcome"`
	ErrorCode string `json:"error_code,omitempty"`
}

// captureIAMUser lists every IAM user (ListUsers, all pages) and captures the
// access-key ages and MFA/login-profile posture the doc's §3.2 needs.
func captureIAMUser(ctx context.Context, cfg aws.Config) (any, error) {
	client := iam.NewFromConfig(cfg)

	var users []iamUser
	paginator := iam.NewListUsersPaginator(client, &iam.ListUsersInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, u := range out.Users {
			users = append(users, iamUser{
				UserName:         aws.ToString(u.UserName),
				Arn:              aws.ToString(u.Arn),
				CreateDate:       secFormatTime(u.CreateDate),
				PasswordLastUsed: secFormatTime(u.PasswordLastUsed),
			})
		}
	}

	for i := range users {
		users[i].AccessKeys = captureIAMUserAccessKeys(ctx, client, users[i].UserName)
		users[i].LoginProfile = captureIAMLoginProfile(ctx, client, users[i].UserName)
		if users[i].LoginProfile.Outcome == "present" {
			users[i].MFADeviceCount = captureIAMMFADeviceCount(ctx, client, users[i].UserName)
		}
	}

	return iamUserData{Users: users}, nil
}

func captureIAMUserAccessKeys(ctx context.Context, client *iam.Client, userName string) []iamAccessKey {
	out, err := client.ListAccessKeys(ctx, &iam.ListAccessKeysInput{UserName: aws.String(userName)})
	if err != nil {
		return nil
	}
	keys := make([]iamAccessKey, 0, len(out.AccessKeyMetadata))
	for _, meta := range out.AccessKeyMetadata {
		key := iamAccessKey{
			AccessKeyId: aws.ToString(meta.AccessKeyId),
			Status:      string(meta.Status),
			CreateDate:  secFormatTime(meta.CreateDate),
		}
		lastUsedOut, err := client.GetAccessKeyLastUsed(ctx, &iam.GetAccessKeyLastUsedInput{AccessKeyId: meta.AccessKeyId})
		if err == nil && lastUsedOut.AccessKeyLastUsed != nil {
			key.LastUsedDate = secFormatTime(lastUsedOut.AccessKeyLastUsed.LastUsedDate)
		}
		keys = append(keys, key)
	}
	return keys
}

func captureIAMLoginProfile(ctx context.Context, client *iam.Client, userName string) iamOutcome {
	_, err := client.GetLoginProfile(ctx, &iam.GetLoginProfileInput{UserName: aws.String(userName)})
	if err != nil {
		if apiErr, ok := errors.AsType[smithy.APIError](err); ok {
			return iamOutcome{Outcome: "error", ErrorCode: apiErr.ErrorCode()}
		}
		return iamOutcome{Outcome: "error", ErrorCode: err.Error()}
	}
	return iamOutcome{Outcome: "present"}
}

func captureIAMMFADeviceCount(ctx context.Context, client *iam.Client, userName string) int {
	out, err := client.ListMFADevices(ctx, &iam.ListMFADevicesInput{UserName: aws.String(userName)})
	if err != nil {
		return 0
	}
	return len(out.MFADevices)
}

// iamGroupData is the raw view of IAM groups the checklist generator reads.
type iamGroupData struct {
	Groups []iamGroup `json:"groups"`
}

type iamGroup struct {
	GroupName  string `json:"group_name"`
	Arn        string `json:"arn"`
	CreateDate string `json:"create_date,omitempty"`
	UserCount  int    `json:"user_count"`
}

// captureIAMGroup lists every IAM group (ListGroups, all pages) and captures
// GetGroup's Users[] count per the doc's §3.2 (empty-group-orphan signal).
func captureIAMGroup(ctx context.Context, cfg aws.Config) (any, error) {
	client := iam.NewFromConfig(cfg)

	var groups []iamGroup
	paginator := iam.NewListGroupsPaginator(client, &iam.ListGroupsInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, g := range out.Groups {
			groups = append(groups, iamGroup{
				GroupName:  aws.ToString(g.GroupName),
				Arn:        aws.ToString(g.Arn),
				CreateDate: secFormatTime(g.CreateDate),
			})
		}
	}

	for i := range groups {
		out, err := client.GetGroup(ctx, &iam.GetGroupInput{GroupName: aws.String(groups[i].GroupName)})
		if err != nil {
			continue
		}
		groups[i].UserCount = len(out.Users)
	}

	return iamGroupData{Groups: groups}, nil
}

// roleData is the raw view of IAM roles the checklist generator reads.
type roleData struct {
	Roles []roleEntry `json:"roles"`
}

type roleEntry struct {
	RoleName                 string `json:"role_name"`
	Arn                      string `json:"arn"`
	CreateDate               string `json:"create_date,omitempty"`
	AssumeRolePolicyDocument string `json:"assume_role_policy_document,omitempty"`
	RoleLastUsedDate         string `json:"role_last_used_date,omitempty"`
}

// captureRole lists every IAM role (ListRoles, all pages) and captures
// RoleLastUsed via GetRole per the doc's §3.2.
func captureRole(ctx context.Context, cfg aws.Config) (any, error) {
	client := iam.NewFromConfig(cfg)

	var roles []roleEntry
	paginator := iam.NewListRolesPaginator(client, &iam.ListRolesInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, r := range out.Roles {
			roles = append(roles, roleEntry{
				RoleName:                 aws.ToString(r.RoleName),
				Arn:                      aws.ToString(r.Arn),
				CreateDate:               secFormatTime(r.CreateDate),
				AssumeRolePolicyDocument: aws.ToString(r.AssumeRolePolicyDocument),
			})
		}
	}

	for i := range roles {
		out, err := client.GetRole(ctx, &iam.GetRoleInput{RoleName: aws.String(roles[i].RoleName)})
		if err != nil || out.Role == nil || out.Role.RoleLastUsed == nil {
			continue
		}
		roles[i].RoleLastUsedDate = secFormatTime(out.Role.RoleLastUsed.LastUsedDate)
	}

	return roleData{Roles: roles}, nil
}

// policyData is the raw view of customer-managed IAM policies the checklist generator reads.
type policyData struct {
	Policies []policyEntry `json:"policies"`
}

type policyEntry struct {
	PolicyName       string `json:"policy_name"`
	Arn              string `json:"arn"`
	AttachmentCount  int32  `json:"attachment_count"`
	DefaultVersionId string `json:"default_version_id,omitempty"`
	CreateDate       string `json:"create_date,omitempty"`
	UpdateDate       string `json:"update_date,omitempty"`
	Document         string `json:"document,omitempty"`
}

// capturePolicy lists every customer-managed policy (ListPolicies with
// Scope=Local, all pages) and captures the default version's Document per the
// doc's §3.2 (wildcard-admin detection).
func capturePolicy(ctx context.Context, cfg aws.Config) (any, error) {
	client := iam.NewFromConfig(cfg)

	var policies []policyEntry
	paginator := iam.NewListPoliciesPaginator(client, &iam.ListPoliciesInput{
		Scope: iamtypes.PolicyScopeTypeLocal,
	})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, p := range out.Policies {
			policies = append(policies, policyEntry{
				PolicyName:       aws.ToString(p.PolicyName),
				Arn:              aws.ToString(p.Arn),
				AttachmentCount:  aws.ToInt32(p.AttachmentCount),
				DefaultVersionId: aws.ToString(p.DefaultVersionId),
				CreateDate:       secFormatTime(p.CreateDate),
				UpdateDate:       secFormatTime(p.UpdateDate),
			})
		}
	}

	for i := range policies {
		if policies[i].DefaultVersionId == "" {
			continue
		}
		out, err := client.GetPolicyVersion(ctx, &iam.GetPolicyVersionInput{
			PolicyArn: aws.String(policies[i].Arn),
			VersionId: aws.String(policies[i].DefaultVersionId),
		})
		if err != nil || out.PolicyVersion == nil {
			continue
		}
		policies[i].Document = aws.ToString(out.PolicyVersion.Document)
	}

	return policyData{Policies: policies}, nil
}

// kmsData is the raw view of KMS keys the checklist generator reads.
type kmsData struct {
	Keys []kmsKey `json:"keys"`
}

type kmsKey struct {
	KeyId               string     `json:"key_id"`
	KeyArn              string     `json:"key_arn"`
	KeyState            string     `json:"key_state,omitempty"`
	KeyManager          string     `json:"key_manager,omitempty"`
	Describe            iamOutcome `json:"describe"`
	KeyRotationEnabled  bool       `json:"key_rotation_enabled"`
	RotationStatusKnown bool       `json:"rotation_status_known"`
}

// captureKMS lists every KMS key (ListKeys, all pages) and captures
// DescribeKey (KeyState, KeyManager) + GetKeyRotationStatus per the doc's §3.2
// — all KMS attention signals are Wave 2.
func captureKMS(ctx context.Context, cfg aws.Config) (any, error) {
	client := kms.NewFromConfig(cfg)

	var keys []kmsKey
	paginator := kms.NewListKeysPaginator(client, &kms.ListKeysInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, k := range out.Keys {
			keys = append(keys, kmsKey{
				KeyId:  aws.ToString(k.KeyId),
				KeyArn: aws.ToString(k.KeyArn),
			})
		}
	}

	for i := range keys {
		describeOut, err := client.DescribeKey(ctx, &kms.DescribeKeyInput{KeyId: aws.String(keys[i].KeyId)})
		if err != nil {
			if apiErr, ok := errors.AsType[smithy.APIError](err); ok {
				keys[i].Describe = iamOutcome{Outcome: "error", ErrorCode: apiErr.ErrorCode()}
			} else {
				keys[i].Describe = iamOutcome{Outcome: "error", ErrorCode: err.Error()}
			}
			continue
		}
		keys[i].Describe = iamOutcome{Outcome: "ok"}
		if describeOut.KeyMetadata != nil {
			keys[i].KeyState = string(describeOut.KeyMetadata.KeyState)
			keys[i].KeyManager = string(describeOut.KeyMetadata.KeyManager)
		}

		rotationOut, err := client.GetKeyRotationStatus(ctx, &kms.GetKeyRotationStatusInput{KeyId: aws.String(keys[i].KeyId)})
		if err == nil {
			keys[i].KeyRotationEnabled = rotationOut.KeyRotationEnabled
			keys[i].RotationStatusKnown = true
		}
	}

	return kmsData{Keys: keys}, nil
}

// secretsData is the raw view of Secrets Manager secrets the checklist generator reads.
type secretsData struct {
	Secrets []secretEntry `json:"secrets"`
}

type secretEntry struct {
	Name                   string     `json:"name"`
	Arn                    string     `json:"arn"`
	KmsKeyId               string     `json:"kms_key_id,omitempty"`
	RotationEnabled        bool       `json:"rotation_enabled"`
	RotationLambdaARN      string     `json:"rotation_lambda_arn,omitempty"`
	LastRotatedDate        string     `json:"last_rotated_date,omitempty"`
	NextRotationDate       string     `json:"next_rotation_date,omitempty"`
	LastAccessedDate       string     `json:"last_accessed_date,omitempty"`
	DeletedDate            string     `json:"deleted_date,omitempty"`
	AutomaticallyAfterDays int64      `json:"automatically_after_days,omitempty"`
	Describe               iamOutcome `json:"describe"`
	StuckOnAwsPending      bool       `json:"stuck_on_aws_pending"`
}

// captureSecrets lists every secret (ListSecrets, all pages) and captures
// DescribeSecret's VersionIdsToStages per the doc's §3.2 (stuck-AWSPENDING
// signal).
func captureSecrets(ctx context.Context, cfg aws.Config) (any, error) {
	client := secretsmanager.NewFromConfig(cfg)

	var secrets []secretEntry
	paginator := secretsmanager.NewListSecretsPaginator(client, &secretsmanager.ListSecretsInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, s := range out.SecretList {
			entry := secretEntry{
				Name:              aws.ToString(s.Name),
				Arn:               aws.ToString(s.ARN),
				KmsKeyId:          aws.ToString(s.KmsKeyId),
				RotationEnabled:   aws.ToBool(s.RotationEnabled),
				RotationLambdaARN: aws.ToString(s.RotationLambdaARN),
				LastRotatedDate:   secFormatTime(s.LastRotatedDate),
				NextRotationDate:  secFormatTime(s.NextRotationDate),
				LastAccessedDate:  secFormatTime(s.LastAccessedDate),
				DeletedDate:       secFormatTime(s.DeletedDate),
			}
			if s.RotationRules != nil {
				entry.AutomaticallyAfterDays = aws.ToInt64(s.RotationRules.AutomaticallyAfterDays)
			}
			secrets = append(secrets, entry)
		}
	}

	for i := range secrets {
		out, err := client.DescribeSecret(ctx, &secretsmanager.DescribeSecretInput{SecretId: aws.String(secrets[i].Arn)})
		if err != nil {
			if apiErr, ok := errors.AsType[smithy.APIError](err); ok {
				secrets[i].Describe = iamOutcome{Outcome: "error", ErrorCode: apiErr.ErrorCode()}
			} else {
				secrets[i].Describe = iamOutcome{Outcome: "error", ErrorCode: err.Error()}
			}
			continue
		}
		secrets[i].Describe = iamOutcome{Outcome: "ok"}
		for _, stages := range out.VersionIdsToStages {
			for _, stage := range stages {
				if stage == "AWSPENDING" {
					secrets[i].StuckOnAwsPending = true
				}
			}
		}
	}

	return secretsData{Secrets: secrets}, nil
}

// acmData is the raw view of ACM certificates the checklist generator reads.
type acmData struct {
	Certificates []acmCertificate `json:"certificates"`
}

type acmCertificate struct {
	CertificateArn          string     `json:"certificate_arn"`
	DomainName              string     `json:"domain_name,omitempty"`
	Status                  string     `json:"status,omitempty"`
	NotBefore               string     `json:"not_before,omitempty"`
	NotAfter                string     `json:"not_after,omitempty"`
	InUse                   bool       `json:"in_use"`
	Describe                iamOutcome `json:"describe"`
	RenewalStatus           string     `json:"renewal_status,omitempty"`
	FailedValidationDomains []string   `json:"failed_validation_domains,omitempty"`
}

// captureACM lists every certificate (ListCertificates, all pages) and
// captures DescribeCertificate's RenewalSummary + DomainValidationOptions per
// the doc's §3.2.
func captureACM(ctx context.Context, cfg aws.Config) (any, error) {
	client := acm.NewFromConfig(cfg)

	var certs []acmCertificate
	paginator := acm.NewListCertificatesPaginator(client, &acm.ListCertificatesInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, c := range out.CertificateSummaryList {
			certs = append(certs, acmCertificate{
				CertificateArn: aws.ToString(c.CertificateArn),
				DomainName:     aws.ToString(c.DomainName),
				Status:         string(c.Status),
				NotBefore:      secFormatTime(c.NotBefore),
				NotAfter:       secFormatTime(c.NotAfter),
				InUse:          aws.ToBool(c.InUse),
			})
		}
	}

	for i := range certs {
		out, err := client.DescribeCertificate(ctx, &acm.DescribeCertificateInput{CertificateArn: aws.String(certs[i].CertificateArn)})
		if err != nil {
			if apiErr, ok := errors.AsType[smithy.APIError](err); ok {
				certs[i].Describe = iamOutcome{Outcome: "error", ErrorCode: apiErr.ErrorCode()}
			} else {
				certs[i].Describe = iamOutcome{Outcome: "error", ErrorCode: err.Error()}
			}
			continue
		}
		certs[i].Describe = iamOutcome{Outcome: "ok"}
		if out.Certificate == nil {
			continue
		}
		if out.Certificate.RenewalSummary != nil {
			certs[i].RenewalStatus = string(out.Certificate.RenewalSummary.RenewalStatus)
		}
		for _, dv := range out.Certificate.DomainValidationOptions {
			if dv.ValidationStatus == "FAILED" {
				certs[i].FailedValidationDomains = append(certs[i].FailedValidationDomains, aws.ToString(dv.DomainName))
			}
		}
	}

	return acmData{Certificates: certs}, nil
}

// wafData is the raw view of WAFv2 Web ACLs the checklist generator reads, across both the
// REGIONAL and CLOUDFRONT scopes per the doc's §1.
type wafData struct {
	WebACLs []wafWebACL `json:"web_acls"`
}

type wafWebACL struct {
	Name                 string     `json:"name"`
	Id                   string     `json:"id"`
	ARN                  string     `json:"arn"`
	Scope                string     `json:"scope"`
	Describe             iamOutcome `json:"describe"`
	RuleCount            int        `json:"rule_count"`
	DefaultActionIsAllow bool       `json:"default_action_is_allow"`
}

// captureWAF lists Web ACLs in both the REGIONAL scope and, when the current
// region is us-east-1 (the only region CLOUDFRONT-scope ACLs can be listed
// from), the CLOUDFRONT scope, then captures GetWebACL's Rules/DefaultAction
// per the doc's §3.2.
func captureWAF(ctx context.Context, cfg aws.Config) (any, error) {
	client := wafv2.NewFromConfig(cfg)

	scopes := []wafv2types.Scope{wafv2types.ScopeRegional}
	if cfg.Region == "us-east-1" {
		scopes = append(scopes, wafv2types.ScopeCloudfront)
	}

	var acls []wafWebACL
	for _, scope := range scopes {
		marker := ""
		for {
			in := &wafv2.ListWebACLsInput{Scope: scope}
			if marker != "" {
				in.NextMarker = aws.String(marker)
			}
			out, err := client.ListWebACLs(ctx, in)
			if err != nil {
				return nil, err
			}
			for _, w := range out.WebACLs {
				acls = append(acls, wafWebACL{
					Name:  aws.ToString(w.Name),
					Id:    aws.ToString(w.Id),
					ARN:   aws.ToString(w.ARN),
					Scope: string(scope),
				})
			}
			if out.NextMarker == nil || aws.ToString(out.NextMarker) == "" {
				break
			}
			marker = aws.ToString(out.NextMarker)
		}
	}

	for i := range acls {
		scope := wafv2types.ScopeRegional
		if acls[i].Scope == string(wafv2types.ScopeCloudfront) {
			scope = wafv2types.ScopeCloudfront
		}
		out, err := client.GetWebACL(ctx, &wafv2.GetWebACLInput{
			Name:  aws.String(acls[i].Name),
			Id:    aws.String(acls[i].Id),
			Scope: scope,
		})
		if err != nil {
			if apiErr, ok := errors.AsType[smithy.APIError](err); ok {
				acls[i].Describe = iamOutcome{Outcome: "error", ErrorCode: apiErr.ErrorCode()}
			} else {
				acls[i].Describe = iamOutcome{Outcome: "error", ErrorCode: err.Error()}
			}
			continue
		}
		acls[i].Describe = iamOutcome{Outcome: "ok"}
		if out.WebACL == nil {
			continue
		}
		acls[i].RuleCount = len(out.WebACL.Rules)
		if out.WebACL.DefaultAction != nil {
			acls[i].DefaultActionIsAllow = out.WebACL.DefaultAction.Allow != nil
		}
	}

	return wafData{WebACLs: acls}, nil
}

// ssmData is the raw view of SSM parameters the checklist generator reads.
type ssmData struct {
	Parameters []ssmParameter `json:"parameters"`
}

type ssmParameter struct {
	Name             string `json:"name"`
	ARN              string `json:"arn,omitempty"`
	Type             string `json:"type,omitempty"`
	Tier             string `json:"tier,omitempty"`
	KeyId            string `json:"key_id,omitempty"`
	LastModifiedDate string `json:"last_modified_date,omitempty"`
}

// captureSSM lists every parameter (DescribeParameters, all pages). No Wave 2
// describe call is defined by the doc — the §3.1 signals are all derivable
// from ParameterMetadata on the list response.
func captureSSM(ctx context.Context, cfg aws.Config) (any, error) {
	client := ssm.NewFromConfig(cfg)

	var params []ssmParameter
	paginator := ssm.NewDescribeParametersPaginator(client, &ssm.DescribeParametersInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, p := range out.Parameters {
			params = append(params, ssmParameter{
				Name:             aws.ToString(p.Name),
				ARN:              aws.ToString(p.ARN),
				Type:             string(p.Type),
				Tier:             string(p.Tier),
				KeyId:            aws.ToString(p.KeyId),
				LastModifiedDate: secFormatTime(p.LastModifiedDate),
			})
		}
	}

	return ssmData{Parameters: params}, nil
}
