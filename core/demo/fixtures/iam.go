// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package fixtures provides IAM fixture data for the IAM fake.
package fixtures

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
)

// IAMFixtures holds all IAM domain objects served by the fake.
type IAMFixtures struct {
	Roles    []iamtypes.Role
	Policies []iamtypes.Policy
	Users    []iamtypes.User
	Groups   []iamtypes.Group
	// AttachedRolePolicies keyed by role name
	AttachedRolePolicies map[string][]iamtypes.AttachedPolicy
	// InlineRolePolicies keyed by role name
	InlineRolePolicies map[string][]string
	// AttachedUserPolicies keyed by user name
	AttachedUserPolicies map[string][]iamtypes.AttachedPolicy
	// AttachedGroupPolicies keyed by group name
	AttachedGroupPolicies map[string][]iamtypes.AttachedPolicy
	// InlineGroupPolicies keyed by group name
	InlineGroupPolicies map[string][]string
	// GroupUsers keyed by group name
	GroupUsers map[string][]iamtypes.User
	// GroupsForUser keyed by user name
	GroupsForUser map[string][]iamtypes.Group
	// EntitiesForPolicy keyed by policy ARN
	EntitiesForPolicy map[string]*PolicyEntities
	// AccountAliases
	AccountAliases []string
	// PolicyDocuments keyed by policy ARN — URL-encoded JSON document strings
	PolicyDocuments map[string]string
	// InlinePolicyDocuments keyed by "roleName/policyName" — URL-encoded JSON document strings
	InlinePolicyDocuments map[string]string
	// InstanceProfiles keyed by instance-profile name — backs iam:GetInstanceProfile
	// for the asg→role and eb→role related-panel pivots (checkASGRole /
	// checkEbRole via asgInstanceProfileToRoles).
	InstanceProfiles map[string]iamtypes.InstanceProfile
	// ConsoleUsers is the set of user names with a console password (GetLoginProfile
	// succeeds). Backs EnrichIAMUserMFA's Wave-2 no-MFA issue check.
	ConsoleUsers map[string]bool
	// MFADevicesByUser keyed by user name — backs EnrichIAMUserMFA.
	MFADevicesByUser map[string][]iamtypes.MFADevice
	// AccessKeysByUser keyed by user name — backs EnrichIAMUserMFA's stale
	// access-key check.
	AccessKeysByUser map[string][]iamtypes.AccessKeyMetadata
	// AccessKeyLastUsed keyed by access key ID — backs
	// iam:GetAccessKeyLastUsed. A key absent from this map has never been
	// used, which the fake reports as a nil LastUsedDate.
	AccessKeyLastUsed map[string]time.Time
}

// PolicyEntities holds the entities (roles, users, groups) attached to a policy.
type PolicyEntities struct {
	Roles  []iamtypes.PolicyRole
	Users  []iamtypes.PolicyUser
	Groups []iamtypes.PolicyGroup
}

const (
	fixtIAMProdLambdaRoleARN = "arn:aws:iam::123456789012:role/service-role/acme-lambda-execution"

	// Witness rows for the IAM posture findings. Each names the ONE demo
	// resource that carries the finding; every other row of the same type is
	// explicitly set to the healthy value for that condition.

	// RoleConfusedDeputy trusts lambda.amazonaws.com with no aws:SourceAccount
	// or aws:SourceArn condition — role.trust.confused-deputy.
	RoleConfusedDeputy = "acme-invoker-callback-role"
	// RoleScopedWildcardTrust is the healthy counterpart of
	// role.trust.wildcard-principal: it trusts any AWS principal, but only one
	// holding the agreed external ID, which is the documented cross-account
	// pattern. It must render with no finding.
	RoleScopedWildcardTrust = "acme-partner-integration-role"
	// RoleAdminAttached carries the AWS-managed AdministratorAccess policy —
	// role.admin-attached.
	RoleAdminAttached = "acme-break-glass-role"
	// RoleInlinePrivEsc has an inline policy granting iam:PassRole plus
	// lambda:CreateFunction plus lambda:InvokeFunction —
	// role.inline-privilege-escalation.
	RoleInlinePrivEsc = "acme-pipeline-builder-role"
	// PolicyPrivEsc is a customer-managed policy granting the same
	// escalation combination — policy.privilege-escalation.
	PolicyPrivEsc = "acme-privesc-policy"
	// IAMUserAdminAttached carries AdministratorAccess — iam-user.admin-attached.
	IAMUserAdminAttached = "break-glass-admin"
	// IAMUserConsoleNeverUsed has a console password created long ago and
	// never used — iam-user.console-never-used.
	IAMUserConsoleNeverUsed = "dormant-console-user"
	// IAMUserKeyUnused holds the one access key old enough to report, and
	// which has never signed a request — iam-user.old-key and
	// iam-user.access-key-unused.
	IAMUserKeyUnused = "stale-key-user"
	// IAMUserTwoKeys has both access-key slots active — iam-user.two-active-keys.
	IAMUserTwoKeys = "dual-key-user"
	// IAMGroupAdminAttached is the pre-existing admins group, whose
	// AdministratorAccess attachment is the natural witness for
	// iam-group.admin-attached.
	IAMGroupAdminAttached = "admins"
	// IAMGroupPowerUser carries PowerUserAccess, the other member of
	// adminManagedPolicyARNs, so the finding's Policy row is witnessed naming
	// something other than AdministratorAccess.
	IAMGroupPowerUser = "platform-engineers"
)

func IsCustomerManagedPolicyARN(policyARN string) bool {
	return policyARN != "" && !strings.Contains(policyARN, ":aws:policy/")
}

// NewIAMFixtures builds and returns a fully-populated IAMFixtures struct.
var sharedIAMFixtures = sync.OnceValue(func() *IAMFixtures {
	f := &IAMFixtures{
		AttachedRolePolicies:  make(map[string][]iamtypes.AttachedPolicy),
		InlineRolePolicies:    make(map[string][]string),
		AttachedUserPolicies:  make(map[string][]iamtypes.AttachedPolicy),
		AttachedGroupPolicies: make(map[string][]iamtypes.AttachedPolicy),
		InlineGroupPolicies:   make(map[string][]string),
		GroupUsers:            make(map[string][]iamtypes.User),
		GroupsForUser:         make(map[string][]iamtypes.Group),
		EntitiesForPolicy:     make(map[string]*PolicyEntities),
		PolicyDocuments:       make(map[string]string),
		InlinePolicyDocuments: make(map[string]string),
	}
	f.Roles = buildIAMRoles()
	f.Policies = buildIAMPolicies()
	f.Users = buildIAMUsers()
	f.Groups = buildIAMGroups()
	buildIAMRelations(f)
	f.InstanceProfiles = buildIAMInstanceProfiles(f.Roles)
	f.AccountAliases = []string{"acme-corp"}
	f.InlineGroupPolicies["developers"] = []string{"AllowAssumeRole", "AllowChangeOwnPassword"}
	f.InlineGroupPolicies["readonly"] = []string{"DenyS3Delete"}
	// ConsoleUsers / MFADevicesByUser / AccessKeysByUser — required for
	// EnrichIAMUserMFA's Wave-2 issue checks. alice.johnson has a console
	// password (PasswordLastUsed is set) but no MFA device registered → "!"
	// finding (CIS IAM.5). bob.smith has an active access key older than 90
	// days → "~" finding (rotation).
	//
	// Key ages and last-use dates are relative to fixture construction so the
	// 90-day thresholds keep meaning the same thing as the calendar moves.
	f.ConsoleUsers = map[string]bool{
		"alice.johnson":         true,
		IAMUserConsoleNeverUsed: true,
	}
	// The console-never-used witness has MFA registered, so it carries that
	// finding alone and not the no-MFA one.
	f.MFADevicesByUser = map[string][]iamtypes.MFADevice{
		IAMUserConsoleNeverUsed: {{
			UserName:     aws.String(IAMUserConsoleNeverUsed),
			SerialNumber: aws.String("arn:aws:iam::123456789012:mfa/" + IAMUserConsoleNeverUsed),
			EnableDate:   aws.Time(time.Now().AddDate(0, 0, -400)),
		}},
	}
	const (
		freshKeyID = "AKIAEXAMPLE2222222222"
		staleKeyID = "AKIAEXAMPLE4444444444"
		dualKeyID1 = "AKIAEXAMPLE5555555555"
		dualKeyID2 = "AKIAEXAMPLE6666666666"
	)
	f.AccessKeysByUser = map[string][]iamtypes.AccessKeyMetadata{
		"bob.smith": {
			{
				AccessKeyId: aws.String(freshKeyID),
				Status:      iamtypes.StatusTypeActive,
				CreateDate:  aws.Time(time.Now().AddDate(0, 0, -20)),
			},
		},
		IAMUserKeyUnused: {
			{
				AccessKeyId: aws.String(staleKeyID),
				Status:      iamtypes.StatusTypeActive,
				CreateDate:  aws.Time(time.Now().AddDate(0, 0, -400)),
			},
		},
		IAMUserTwoKeys: {
			{
				AccessKeyId: aws.String(dualKeyID1),
				Status:      iamtypes.StatusTypeActive,
				CreateDate:  aws.Time(time.Now().AddDate(0, 0, -30)),
			},
			{
				AccessKeyId: aws.String(dualKeyID2),
				Status:      iamtypes.StatusTypeActive,
				CreateDate:  aws.Time(time.Now().AddDate(0, 0, -10)),
			},
		},
	}
	// staleKeyID is deliberately absent: never used is what makes it the
	// unused-key witness. Every other active key was used in the last week.
	f.AccessKeyLastUsed = map[string]time.Time{
		freshKeyID: time.Now().AddDate(0, 0, -2),
		dualKeyID1: time.Now().AddDate(0, 0, -3),
		dualKeyID2: time.Now().AddDate(0, 0, -1),
	}
	return f
})

func NewIAMFixtures() *IAMFixtures {
	return sharedIAMFixtures()
}

func buildIAMRoles() []iamtypes.Role {
	roles := []iamtypes.Role{
		{
			RoleName:                 aws.String("acme-eks-node-role"),
			RoleId:                   aws.String("AROAEXAMPLE111111111"),
			Arn:                      aws.String("arn:aws:iam::123456789012:role/acme-eks-node-role"),
			Path:                     aws.String("/"),
			CreateDate:               aws.Time(time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)),
			Description:              aws.String("Role for EKS managed node groups"),
			AssumeRolePolicyDocument: aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"ec2.amazonaws.com"},"Action":"sts:AssumeRole"}]}`),
			MaxSessionDuration:       aws.Int32(3600),
		},
		{
			RoleName:    aws.String("acme-lambda-execution"),
			RoleId:      aws.String("AROAEXAMPLE222222222"),
			Arn:         aws.String(fixtIAMProdLambdaRoleARN),
			Path:        aws.String("/service-role/"),
			CreateDate:  aws.Time(time.Date(2025, 3, 10, 8, 15, 0, 0, time.UTC)),
			Description: aws.String("Execution role for Lambda functions"),
		},
		{
			RoleName:    aws.String(fixtIAMProdLambdaRoleARN),
			RoleId:      aws.String("AROAEXAMPLE222222223"),
			Arn:         aws.String(fixtIAMProdLambdaRoleARN),
			Path:        aws.String("/service-role/"),
			CreateDate:  aws.Time(time.Date(2025, 3, 10, 8, 15, 0, 0, time.UTC)),
			Description: aws.String("Lambda execution role ARN alias (navigable-field cross-reference)"),
		},
		{
			RoleName:    aws.String("acme-ci-deploy-role"),
			RoleId:      aws.String("AROAEXAMPLE333333333"),
			Arn:         aws.String("arn:aws:iam::123456789012:role/acme-ci-deploy-role"),
			Path:        aws.String("/"),
			CreateDate:  aws.Time(time.Date(2025, 1, 20, 14, 0, 0, 0, time.UTC)),
			Description: aws.String("CI/CD deployment role for CodePipeline"),
			// AssumeRolePolicyDocument — required for the role:iam-group and
			// role:iam-user related-panel pivots (checkRoleIamGroup /
			// checkRoleIamUser, offline parse of Principal.AWS ARNs). Models
			// a realistic "break-glass manual deploy" trust policy alongside
			// the CodePipeline service principal.
			AssumeRolePolicyDocument: aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"codepipeline.amazonaws.com"},"Action":"sts:AssumeRole","Condition":{"StringEquals":{"aws:SourceAccount":"123456789012"}}},{"Effect":"Allow","Principal":{"AWS":["arn:aws:iam::123456789012:group/admins","arn:aws:iam::123456789012:user/alice.johnson"]},"Action":"sts:AssumeRole","Condition":{"Bool":{"aws:MultiFactorAuthPresent":"true"}}}]}`),
		},
		{
			RoleName:    aws.String("acme-rds-monitoring"),
			RoleId:      aws.String("AROAEXAMPLE444444444"),
			Arn:         aws.String("arn:aws:iam::123456789012:role/acme-rds-monitoring"),
			Path:        aws.String("/"),
			CreateDate:  aws.Time(time.Date(2025, 4, 5, 16, 45, 0, 0, time.UTC)),
			Description: aws.String("Enhanced monitoring role for RDS instances"),
		},
		{
			RoleName:    aws.String("deploy-bot"),
			RoleId:      aws.String("AROAEXAMPLE555555555"),
			Arn:         aws.String("arn:aws:iam::123456789012:role/deploy-bot"),
			Path:        aws.String("/"),
			CreateDate:  aws.Time(time.Date(2025, 5, 1, 10, 0, 0, 0, time.UTC)),
			Description: aws.String("Automation role used by deployment bot sessions"),
		},
		{
			RoleName:    aws.String("ci-runner"),
			RoleId:      aws.String("AROAEXAMPLE666666666"),
			Arn:         aws.String("arn:aws:iam::123456789012:role/ci-runner"),
			Path:        aws.String("/"),
			CreateDate:  aws.Time(time.Date(2025, 5, 10, 11, 0, 0, 0, time.UTC)),
			Description: aws.String("Automation role used by CI runner sessions"),
		},
		{
			RoleName:    aws.String("acme-glue-role"),
			RoleId:      aws.String("AROAEXAMPLE888888888"),
			Arn:         aws.String("arn:aws:iam::123456789012:role/acme-glue-role"),
			Path:        aws.String("/"),
			CreateDate:  aws.Time(time.Date(2025, 7, 1, 9, 0, 0, 0, time.UTC)),
			Description: aws.String("Service role for Glue ETL jobs"),
		},
		// acme-ec2-instance-role — attached to the acme-ec2-instance-profile
		// instance profile referenced by acme-web-prod-lc's IamInstanceProfile.
		// Required for the asg:role related-panel pivot (checkASGRole via
		// asgResolveInstanceProfile → asgInstanceProfileToRoles).
		{
			RoleName:    aws.String("acme-ec2-instance-role"),
			RoleId:      aws.String("AROAEXAMPLE999999999"),
			Arn:         aws.String("arn:aws:iam::123456789012:role/acme-ec2-instance-role"),
			Path:        aws.String("/"),
			CreateDate:  aws.Time(time.Date(2025, 1, 5, 9, 0, 0, 0, time.UTC)),
			Description: aws.String("EC2 instance role for web-tier ASG instances"),
		},
		// ARN-keyed alias fixtures for EKS node group NodeRole navigable field
		{
			RoleName:    aws.String("arn:aws:iam::123456789012:role/eks-node-role"),
			RoleId:      aws.String("AROAEXAMPLENGNODE001"),
			Arn:         aws.String("arn:aws:iam::123456789012:role/eks-node-role"),
			Path:        aws.String("/"),
			CreateDate:  aws.Time(time.Date(2025, 2, 20, 12, 0, 0, 0, time.UTC)),
			Description: aws.String("EKS node role ARN alias (navigable-field cross-reference)"),
		},
		{
			RoleName:    aws.String("arn:aws:iam::123456789012:role/eks-gpu-node-role"),
			RoleId:      aws.String("AROAEXAMPLENGNODE002"),
			Arn:         aws.String("arn:aws:iam::123456789012:role/eks-gpu-node-role"),
			Path:        aws.String("/"),
			CreateDate:  aws.Time(time.Date(2025, 4, 5, 9, 30, 0, 0, time.UTC)),
			Description: aws.String("EKS GPU node role ARN alias (navigable-field cross-reference)"),
		},
	}

	// Witness roles for the trust-policy and attachment posture findings.
	roles = append(roles,
		iamtypes.Role{
			RoleName:                 aws.String(RoleConfusedDeputy),
			RoleId:                   aws.String("AROAEXAMPLECONFDEP01"),
			Arn:                      aws.String("arn:aws:iam::123456789012:role/" + RoleConfusedDeputy),
			Path:                     aws.String("/"),
			CreateDate:               aws.Time(time.Date(2025, 6, 2, 11, 0, 0, 0, time.UTC)),
			Description:              aws.String("Role Lambda assumes to call back into this account — no aws:SourceAccount or aws:SourceArn scoping"),
			AssumeRolePolicyDocument: aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"lambda.amazonaws.com"},"Action":"sts:AssumeRole"}]}`),
		},
		iamtypes.Role{
			RoleName:                 aws.String(RoleScopedWildcardTrust),
			RoleId:                   aws.String("AROAEXAMPLESCOPEDWC1"),
			Arn:                      aws.String("arn:aws:iam::123456789012:role/" + RoleScopedWildcardTrust),
			Path:                     aws.String("/"),
			CreateDate:               aws.Time(time.Date(2025, 4, 8, 10, 0, 0, 0, time.UTC)),
			Description:              aws.String("Cross-account role a partner assumes with the agreed external ID"),
			AssumeRolePolicyDocument: aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"*"},"Action":"sts:AssumeRole","Condition":{"StringEquals":{"sts:ExternalId":"acme-partner-9f3c2"}}}]}`),
			// Recently assumed, so the scoped-trust witness renders with a
			// blank status rather than the dormant-role warning every other
			// demo role carries.
			RoleLastUsed: &iamtypes.RoleLastUsed{LastUsedDate: aws.Time(time.Now().AddDate(0, 0, -3))},
		},
		iamtypes.Role{
			RoleName:                 aws.String(RoleAdminAttached),
			RoleId:                   aws.String("AROAEXAMPLEADMINATT1"),
			Arn:                      aws.String("arn:aws:iam::123456789012:role/" + RoleAdminAttached),
			Path:                     aws.String("/"),
			CreateDate:               aws.Time(time.Date(2025, 2, 14, 9, 0, 0, 0, time.UTC)),
			Description:              aws.String("Break-glass operator role carrying the AWS-managed AdministratorAccess policy"),
			AssumeRolePolicyDocument: aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:root"},"Action":"sts:AssumeRole","Condition":{"Bool":{"aws:MultiFactorAuthPresent":"true"}}}]}`),
		},
		iamtypes.Role{
			RoleName:                 aws.String(RoleInlinePrivEsc),
			RoleId:                   aws.String("AROAEXAMPLEINLPRIVE1"),
			Arn:                      aws.String("arn:aws:iam::123456789012:role/" + RoleInlinePrivEsc),
			Path:                     aws.String("/"),
			CreateDate:               aws.Time(time.Date(2025, 5, 20, 13, 0, 0, 0, time.UTC)),
			Description:              aws.String("Pipeline build role whose inline policy can pass a role into a Lambda it creates and invokes"),
			AssumeRolePolicyDocument: aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"codebuild.amazonaws.com"},"Action":"sts:AssumeRole","Condition":{"StringEquals":{"aws:SourceAccount":"123456789012"}}}]}`),
		},
	)

	// Backup prod role — required for backup→role related-panel pivot (checkBackupRole).
	// ListBackupSelections for plan-broken-2failed returns this ARN in IamRoleArn.
	// checkBackupRole extracts the role name as the last segment after "/".
	roles = append(roles, iamtypes.Role{
		RoleName:                 aws.String("AcmeBackupRoleProd"),
		RoleId:                   aws.String("AROAEXAMPLEBKUPPROD01"),
		Arn:                      aws.String(AcmeBackupRoleARN),
		Path:                     aws.String("/"),
		CreateDate:               aws.Time(time.Date(2025, 4, 10, 9, 0, 0, 0, time.UTC)),
		Description:              aws.String("IAM role assumed by AWS Backup for prod database and critical plan backups"),
		AssumeRolePolicyDocument: aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"backup.amazonaws.com"},"Action":"sts:AssumeRole","Condition":{"StringEquals":{"aws:SourceAccount":"123456789012"}}}]}`),
	})

	// CloudTrail-to-CloudWatch-Logs delivery role — required for trail:role
	// related-panel pivot (checkTrailRole). acme-management-trail's
	// CloudWatchLogsRoleArn points here; checkTrailRole extracts the bare
	// role name as the last segment after "/".
	roles = append(roles, iamtypes.Role{
		RoleName:                 aws.String("acme-cloudtrail-cwlogs-role"),
		RoleId:                   aws.String("AROAEXAMPLECTLOGS001"),
		Arn:                      aws.String("arn:aws:iam::123456789012:role/acme-cloudtrail-cwlogs-role"),
		Path:                     aws.String("/"),
		CreateDate:               aws.Time(time.Date(2025, 2, 1, 9, 0, 0, 0, time.UTC)),
		Description:              aws.String("IAM role assumed by CloudTrail to deliver events to CloudWatch Logs"),
		AssumeRolePolicyDocument: aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"cloudtrail.amazonaws.com"},"Action":"sts:AssumeRole","Condition":{"StringEquals":{"aws:SourceAccount":"123456789012"}}}]}`),
	})

	// RDS monitoring roles — required for dbi→role related-panel pivot.
	// Matched by checkDbiRole via AssociatedRoles[].RoleArn and MonitoringRoleArn on prod-dbi-1.
	roles = append(roles, iamtypes.Role{
		RoleName:                 aws.String("rds-monitoring-role"),
		RoleId:                   aws.String("AROAEXAMPLERDSMON001"),
		Arn:                      aws.String("arn:aws:iam::123456789012:role/rds-monitoring-role"),
		Path:                     aws.String("/"),
		CreateDate:               aws.Time(time.Date(2025, 2, 10, 9, 0, 0, 0, time.UTC)),
		Description:              aws.String("IAM role for RDS Enhanced Monitoring (associated via AssociatedRoles)"),
		AssumeRolePolicyDocument: aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"monitoring.rds.amazonaws.com"},"Action":"sts:AssumeRole"}]}`),
	})
	roles = append(roles, iamtypes.Role{
		RoleName:                 aws.String("rds-enhanced-monitoring"),
		RoleId:                   aws.String("AROAEXAMPLERDSMON002"),
		Arn:                      aws.String("arn:aws:iam::123456789012:role/rds-enhanced-monitoring"),
		Path:                     aws.String("/"),
		CreateDate:               aws.Time(time.Date(2025, 2, 10, 9, 5, 0, 0, time.UTC)),
		Description:              aws.String("IAM role for RDS Enhanced Monitoring (referenced via MonitoringRoleArn)"),
		AssumeRolePolicyDocument: aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"monitoring.rds.amazonaws.com"},"Action":"sts:AssumeRole"}]}`),
	})

	// Issue: AssumeRolePolicyDocument with Principal="*" without condition → Broken (wildcard trust)
	roles = append(roles, iamtypes.Role{
		RoleName:                 aws.String("wildcard-trust-role"),
		RoleId:                   aws.String("AROAEXAMPLE777777777"),
		Arn:                      aws.String("arn:aws:iam::123456789012:role/wildcard-trust-role"),
		Path:                     aws.String("/"),
		CreateDate:               aws.Time(time.Date(2025, 8, 10, 12, 0, 0, 0, time.UTC)),
		Description:              aws.String("Legacy role with overly broad trust — any AWS principal can assume it"),
		AssumeRolePolicyDocument: aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"*"},"Action":"sts:AssumeRole"}]}`),
		MaxSessionDuration:       aws.Int32(3600),
	})

	// Issue: AssumeRolePolicyDocument with bare-string Principal="*" → Broken
	// (colorRole matches the literal `"Principal":"*"` / `"Principal": "*"`
	// substring — distinct from the nested-object `{"AWS":"*"}` form used by
	// wildcard-trust-role above, which does not match that substring).
	roles = append(roles, iamtypes.Role{
		RoleName:                 aws.String("anyone-can-assume-role"),
		RoleId:                   aws.String("AROAEXAMPLE787878787"),
		Arn:                      aws.String("arn:aws:iam::123456789012:role/anyone-can-assume-role"),
		Path:                     aws.String("/"),
		CreateDate:               aws.Time(time.Date(2025, 9, 1, 10, 0, 0, 0, time.UTC)),
		Description:              aws.String("Misconfigured role — bare-string wildcard principal allows any AWS account to assume it"),
		AssumeRolePolicyDocument: aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*","Action":"sts:AssumeRole"}]}`),
	})

	// Redshift IAM roles — required for redshift→role related-panel pivot.
	// checkRedshiftRole matches IamRoles[].IamRoleArn on the cluster; the checker
	// extracts the role name as the last segment after "/" from each ARN.
	roles = append(roles, iamtypes.Role{
		RoleName:                 aws.String("redshift-copy-role"),
		RoleId:                   aws.String("AROAEXAMPLERSSHIFT001"),
		Arn:                      aws.String(RedshiftCopyRoleARN),
		Path:                     aws.String("/"),
		CreateDate:               aws.Time(time.Date(2025, 3, 1, 9, 0, 0, 0, time.UTC)),
		Description:              aws.String("IAM role allowing acme-warehouse Redshift to COPY data from S3"),
		AssumeRolePolicyDocument: aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"redshift.amazonaws.com"},"Action":"sts:AssumeRole"}]}`),
	})
	roles = append(roles, iamtypes.Role{
		RoleName:                 aws.String("redshift-unload-role"),
		RoleId:                   aws.String("AROAEXAMPLERSSHIFT002"),
		Arn:                      aws.String(RedshiftUnloadRoleARN),
		Path:                     aws.String("/"),
		CreateDate:               aws.Time(time.Date(2025, 3, 1, 9, 5, 0, 0, time.UTC)),
		Description:              aws.String("IAM role allowing acme-warehouse Redshift to UNLOAD data to S3"),
		AssumeRolePolicyDocument: aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"redshift.amazonaws.com"},"Action":"sts:AssumeRole"}]}`),
	})
	roles = append(roles, iamtypes.Role{
		RoleName:                 aws.String("redshift-reporting-copy-role"),
		RoleId:                   aws.String("AROAEXAMPLERSSHIFT003"),
		Arn:                      aws.String(RedshiftReportingCopyRoleARN),
		Path:                     aws.String("/"),
		CreateDate:               aws.Time(time.Date(2025, 7, 22, 9, 0, 0, 0, time.UTC)),
		Description:              aws.String("IAM role allowing acme-reporting Redshift to COPY data from S3"),
		AssumeRolePolicyDocument: aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"redshift.amazonaws.com"},"Action":"sts:AssumeRole"}]}`),
	})

	// EC2 instance-profile role — required for the ec2:role related-panel
	// pivot. checkEC2Role extracts the last "/" segment of
	// Instance.IamInstanceProfile.Arn (fixtProdInstanceProfileARN, ec2.go)
	// and uses it directly as the role name.
	roles = append(roles, iamtypes.Role{
		RoleName:                 aws.String("acme-ec2-instance-profile"),
		RoleId:                   aws.String("AROAEXAMPLEEC2PROFILE1"),
		Arn:                      aws.String("arn:aws:iam::123456789012:role/acme-ec2-instance-profile"),
		Path:                     aws.String("/"),
		CreateDate:               aws.Time(time.Date(2025, 1, 5, 9, 0, 0, 0, time.UTC)),
		Description:              aws.String("Instance-profile role attached to acme production EC2 instances"),
		AssumeRolePolicyDocument: aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"ec2.amazonaws.com"},"Action":"sts:AssumeRole"}]}`),
	})

	// AWS Backup default service role — required for backup:role related-panel
	// pivot on plans that use the AWS-managed default role (as opposed to
	// AcmeBackupRoleProd's custom role above).
	roles = append(roles, iamtypes.Role{
		RoleName:                 aws.String("AWSBackupDefaultServiceRole"),
		RoleId:                   aws.String("AROAEXAMPLEBKUPDFLT01"),
		Arn:                      aws.String("arn:aws:iam::123456789012:role/service-role/AWSBackupDefaultServiceRole"),
		Path:                     aws.String("/service-role/"),
		CreateDate:               aws.Time(time.Date(2024, 9, 1, 8, 0, 0, 0, time.UTC)),
		Description:              aws.String("Default AWS-managed service role used by AWS Backup for plans without a custom IAM role"),
		AssumeRolePolicyDocument: aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"backup.amazonaws.com"},"Action":"sts:AssumeRole","Condition":{"StringEquals":{"aws:SourceAccount":"123456789012"}}}]}`),
	})

	// EventBridge rule target-invocation role — required for eb-rule:role
	// related-panel pivot (checkEBRuleRole matches Target.RoleArn on rule targets).
	roles = append(roles, iamtypes.Role{
		RoleName:                 aws.String("prod-ci-deploy-role"),
		RoleId:                   aws.String("AROAEXAMPLEEBRULE001"),
		Arn:                      aws.String("arn:aws:iam::123456789012:role/prod-ci-deploy-role"),
		Path:                     aws.String("/"),
		CreateDate:               aws.Time(time.Date(2025, 3, 12, 9, 0, 0, 0, time.UTC)),
		Description:              aws.String("Role assumed by EventBridge to invoke CI/CD deployment targets"),
		AssumeRolePolicyDocument: aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"events.amazonaws.com"},"Action":"sts:AssumeRole","Condition":{"StringEquals":{"aws:SourceAccount":"123456789012"}}}]}`),
	})

	// ECS service-linked role — required for ecs-svc:role related-panel pivot
	// (checkECSSvcRole matches Service.RoleArn on the api-gateway service).
	roles = append(roles, iamtypes.Role{
		RoleName:                 aws.String("acme-ecs-service-role"),
		RoleId:                   aws.String("AROAEXAMPLEECSSVC001"),
		Arn:                      aws.String("arn:aws:iam::123456789012:role/aws-service-role/ecs.amazonaws.com/acme-ecs-service-role"),
		Path:                     aws.String("/aws-service-role/ecs.amazonaws.com/"),
		CreateDate:               aws.Time(time.Date(2025, 1, 25, 10, 0, 0, 0, time.UTC)),
		Description:              aws.String("Service-linked role allowing ECS to manage load balancers and network interfaces on behalf of the api-gateway service"),
		AssumeRolePolicyDocument: aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"ecs.amazonaws.com"},"Action":"sts:AssumeRole"}]}`),
	})

	// EKS cluster role — required for eks:role related-panel pivot
	// (checkEKSRole matches Cluster.RoleArn across all acme-* EKS clusters).
	roles = append(roles, iamtypes.Role{
		RoleName:                 aws.String("acme-eks-cluster-role"),
		RoleId:                   aws.String("AROAEXAMPLEEKSCLST01"),
		Arn:                      aws.String("arn:aws:iam::123456789012:role/acme-eks-cluster-role"),
		Path:                     aws.String("/"),
		CreateDate:               aws.Time(time.Date(2025, 1, 5, 8, 0, 0, 0, time.UTC)),
		Description:              aws.String("EKS control-plane role assumed by the acme-* clusters to manage AWS resources on their behalf"),
		AssumeRolePolicyDocument: aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"eks.amazonaws.com"},"Action":"sts:AssumeRole","Condition":{"StringEquals":{"aws:SourceAccount":"123456789012"}}}]}`),
	})

	// S3 healthy-bucket access role (checkS3Role pivot).
	// checkS3Role matches roles whose policy_resources field contains the bucket ARN.
	// policy_resources is emitted by the IAM roles fetcher; this role is pre-set
	// so the demo related graph renders.
	roles = append(roles, iamtypes.Role{
		RoleName:                 aws.String("a9s-demo-s3-access-role"),
		RoleId:                   aws.String("AROAEXAMPLES3ACCESS01"),
		Arn:                      aws.String("arn:aws:iam::123456789012:role/a9s-demo-s3-access-role"),
		Path:                     aws.String("/"),
		CreateDate:               aws.Time(time.Date(2025, 1, 10, 10, 0, 0, 0, time.UTC)),
		Description:              aws.String("Role granting read access to a9s-demo-healthy S3 bucket (" + HealthyBucketARN + ")"),
		AssumeRolePolicyDocument: aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"lambda.amazonaws.com"},"Action":"sts:AssumeRole","Condition":{"StringEquals":{"aws:SourceAccount":"123456789012"}}}]}`),
	})

	// CT-event cross-reference roles for ctdetail nav tests
	for _, rd := range []struct{ id, desc string }{
		{"KarpenterNodeRole", "Karpenter node provisioner role (ct-events case A cross-ref)"},
		{"AWSReservedSSO_AdminAccess_3c4d5e6f7a8b9c0d", "SSO AdminAccess reserved role (ct-events case B cross-ref)"},
		{"eks-checkout-svc-sa", "EKS IRSA service account role for checkout service (ct-events case F cross-ref)"},
		{"CiBuildRole", "CI/CD build role for cross-account artifact publishing (ct-events case G cross-ref)"},
		{"DataPipelineRole", "Data pipeline ETL role for VPCE access (ct-events case I cross-ref)"},
	} {
		id := rd.id
		prefix := id
		if len(prefix) > 8 {
			prefix = prefix[:8]
		}
		roles = append(roles, iamtypes.Role{
			RoleName:    aws.String(id),
			RoleId:      aws.String(fmt.Sprintf("AROACT029%s", prefix)),
			Arn:         aws.String(fmt.Sprintf("arn:aws:iam::111111111111:role/%s", id)),
			Path:        aws.String("/"),
			CreateDate:  aws.Time(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
			Description: aws.String(rd.desc),
		})
	}

	// Generate 18 more roles to total 25+
	roleNames := []string{
		"acme-s3-replication", "acme-backup-service", "acme-ssm-managed", "acme-vpc-flow-logs",
		"acme-config-rule", "acme-xray-daemon", "acme-firehose-delivery", "acme-emr-service",
		"acme-sagemaker-exec", "acme-dms-service", "acme-ecs-task-exec", "acme-ecs-task",
		"acme-step-functions", "acme-eventbridge-invoke", "acme-kms-admin", "acme-waf-logging",
		"acme-shield-response", "acme-guardduty-service",
	}
	paths := []string{"/", "/service-role/", "/", "/aws-service-role/"}
	for i, name := range roleNames {
		roleID := fmt.Sprintf("AROAEXAMPLE%09d", 500+i)
		path := paths[i%len(paths)]
		createDate := time.Date(2025, time.Month(1+i%12), 1+i%28, 8+i%12, 0, 0, 0, time.UTC)
		roles = append(roles, iamtypes.Role{
			RoleName:    aws.String(name),
			RoleId:      aws.String(roleID),
			Arn:         aws.String(fmt.Sprintf("arn:aws:iam::123456789012:role%s%s", path, name)),
			Path:        aws.String(path),
			CreateDate:  aws.Time(createDate),
			Description: aws.String(fmt.Sprintf("Service role for %s", name)),
		})
	}
	// AWSServiceRoleForVPCTransitGateway — required for the tgw→role
	// related-panel pivot (checkTGWRole), which probes for this exact
	// service-linked-role name via iam:GetRole. checkTGWRole's navigation ID
	// is the role's ARN (not the bare name), and the demo drill's role
	// FetchByIDs passes that ID straight through as GetRoleInput.RoleName —
	// so a second alias entry keyed by the ARN itself is required, mirroring
	// the fixtIAMProdLambdaRoleARN pattern above.
	const tgwSLRArn = "arn:aws:iam::123456789012:role/aws-service-role/transitgateway.amazonaws.com/AWSServiceRoleForVPCTransitGateway"
	roles = append(roles,
		iamtypes.Role{
			RoleName:    aws.String("AWSServiceRoleForVPCTransitGateway"),
			RoleId:      aws.String("AROAEXAMPLETGW0001"),
			Arn:         aws.String(tgwSLRArn),
			Path:        aws.String("/aws-service-role/transitgateway.amazonaws.com/"),
			CreateDate:  aws.Time(time.Date(2025, 3, 1, 9, 0, 0, 0, time.UTC)),
			Description: aws.String("Service-linked role for AWS Transit Gateway"),
		},
		iamtypes.Role{
			RoleName:    aws.String(tgwSLRArn),
			RoleId:      aws.String("AROAEXAMPLETGW0002"),
			Arn:         aws.String(tgwSLRArn),
			Path:        aws.String("/aws-service-role/transitgateway.amazonaws.com/"),
			CreateDate:  aws.Time(time.Date(2025, 3, 1, 9, 0, 0, 0, time.UTC)),
			Description: aws.String("Transit Gateway SLR ARN alias (navigable-field cross-reference)"),
		},
	)
	return roles
}

// buildIAMInstanceProfiles maps instance-profile name → InstanceProfile for
// iam:GetInstanceProfile, required by the asg:role related-panel pivot
// (checkASGRole → asgResolveInstanceProfile → asgInstanceProfileToRoles).
// The acme-ec2-instance-profile role doubles as the instance-profile role —
// AWS instance profiles commonly carry a single role of the same name.
func buildIAMInstanceProfiles(roles []iamtypes.Role) map[string]iamtypes.InstanceProfile {
	var ec2ProfileRole iamtypes.Role
	for _, r := range roles {
		if aws.ToString(r.RoleName) == "acme-ec2-instance-role" {
			ec2ProfileRole = r
			break
		}
	}
	return map[string]iamtypes.InstanceProfile{
		"acme-ec2-instance-profile": {
			InstanceProfileName: aws.String("acme-ec2-instance-profile"),
			InstanceProfileId:   aws.String("AIPAEXAMPLEEC2PROFILE1"),
			Arn:                 aws.String("arn:aws:iam::123456789012:instance-profile/acme-ec2-instance-profile"),
			Path:                aws.String("/"),
			CreateDate:          aws.Time(time.Date(2025, 1, 5, 9, 0, 0, 0, time.UTC)),
			Roles:               []iamtypes.Role{ec2ProfileRole},
		},
	}
}

func buildIAMPolicies() []iamtypes.Policy {
	policies := []iamtypes.Policy{
		// Witness: a customer-managed policy whose action set adds up to
		// administrator even though no single action looks privileged.
		{
			PolicyName:       aws.String(PolicyPrivEsc),
			PolicyId:         aws.String("ANPAEXAMPLEPRIVESC01"),
			Arn:              aws.String("arn:aws:iam::123456789012:policy/" + PolicyPrivEsc),
			AttachmentCount:  aws.Int32(1),
			Path:             aws.String("/"),
			CreateDate:       aws.Time(time.Date(2025, 5, 20, 13, 0, 0, 0, time.UTC)),
			DefaultVersionId: aws.String("v1"),
			Description:      aws.String("Lets a build pipeline pass a role into a Lambda it creates and invokes"),
			IsAttachable:     true,
		},
		{
			PolicyName:       aws.String("acme-s3-read-only"),
			PolicyId:         aws.String("ANPAEXAMPLE111111111"),
			Arn:              aws.String("arn:aws:iam::123456789012:policy/acme-s3-read-only"),
			AttachmentCount:  aws.Int32(5),
			Path:             aws.String("/"),
			CreateDate:       aws.Time(time.Date(2025, 2, 10, 9, 0, 0, 0, time.UTC)),
			DefaultVersionId: aws.String("v3"),
			Description:      aws.String("Allows EC2 and S3 read access"),
		},
		{
			PolicyName:       aws.String("acme-deploy-policy"),
			PolicyId:         aws.String("ANPAEXAMPLE222222222"),
			Arn:              aws.String("arn:aws:iam::123456789012:policy/acme-deploy-policy"),
			AttachmentCount:  aws.Int32(3),
			Path:             aws.String("/"),
			CreateDate:       aws.Time(time.Date(2025, 1, 15, 11, 30, 0, 0, time.UTC)),
			DefaultVersionId: aws.String("v1"),
		},
		{
			PolicyName:       aws.String("acme-secrets-access"),
			PolicyId:         aws.String("ANPAEXAMPLE333333333"),
			Arn:              aws.String("arn:aws:iam::123456789012:policy/acme-secrets-access"),
			AttachmentCount:  aws.Int32(2),
			Path:             aws.String("/"),
			CreateDate:       aws.Time(time.Date(2025, 5, 20, 13, 15, 0, 0, time.UTC)),
			DefaultVersionId: aws.String("v1"),
		},
		{
			PolicyName:       aws.String("acme-cloudwatch-logs"),
			PolicyId:         aws.String("ANPAEXAMPLE444444444"),
			Arn:              aws.String("arn:aws:iam::123456789012:policy/acme-cloudwatch-logs"),
			AttachmentCount:  aws.Int32(8),
			Path:             aws.String("/"),
			CreateDate:       aws.Time(time.Date(2024, 11, 1, 7, 45, 0, 0, time.UTC)),
			DefaultVersionId: aws.String("v1"),
		},
		// AWS-managed AdministratorAccess policy (ct-events Case K cross-reference)
		{
			PolicyName:       aws.String("AdministratorAccess"),
			PolicyId:         aws.String("ANPAEXAMPLE000000001"),
			Arn:              aws.String("arn:aws:iam::aws:policy/AdministratorAccess"),
			AttachmentCount:  aws.Int32(12),
			Path:             aws.String("/"),
			CreateDate:       aws.Time(time.Date(2015, 2, 6, 18, 40, 16, 0, time.UTC)),
			DefaultVersionId: aws.String("v1"),
		},
		// AWS-managed PowerUserAccess, attached to IAMGroupPowerUser so
		// the admin-attached finding's Policy row is witnessed naming a policy
		// other than AdministratorAccess.
		{
			PolicyName:       aws.String("PowerUserAccess"),
			PolicyId:         aws.String("ANPAEXAMPLE000000002"),
			Arn:              aws.String("arn:aws:iam::aws:policy/PowerUserAccess"),
			AttachmentCount:  aws.Int32(1),
			Path:             aws.String("/"),
			CreateDate:       aws.Time(time.Date(2015, 2, 6, 18, 39, 47, 0, time.UTC)),
			DefaultVersionId: aws.String("v1"),
		},
		// AWS-managed policies referenced by role_policies fixtures
		{
			PolicyName:       aws.String("AmazonEKSWorkerNodePolicy"),
			PolicyId:         aws.String("ANPAEXAMPLE000000002"),
			Arn:              aws.String("arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy"),
			AttachmentCount:  aws.Int32(6),
			Path:             aws.String("/"),
			CreateDate:       aws.Time(time.Date(2018, 5, 27, 0, 0, 0, 0, time.UTC)),
			DefaultVersionId: aws.String("v1"),
		},
		{
			PolicyName:       aws.String("AmazonEC2ContainerRegistryReadOnly"),
			PolicyId:         aws.String("ANPAEXAMPLE000000003"),
			Arn:              aws.String("arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"),
			AttachmentCount:  aws.Int32(6),
			Path:             aws.String("/"),
			CreateDate:       aws.Time(time.Date(2015, 12, 21, 0, 0, 0, 0, time.UTC)),
			DefaultVersionId: aws.String("v1"),
		},
		// AWS-managed AWSBackupServiceRolePolicyForBackup — attached to
		// AcmeBackupRoleProd (role→policy related-panel pivot).
		{
			PolicyName:       aws.String("AWSBackupServiceRolePolicyForBackup"),
			PolicyId:         aws.String("ANPAEXAMPLE000000004"),
			Arn:              aws.String("arn:aws:iam::aws:policy/service-role/AWSBackupServiceRolePolicyForBackup"),
			AttachmentCount:  aws.Int32(1),
			Path:             aws.String("/service-role/"),
			CreateDate:       aws.Time(time.Date(2018, 11, 26, 0, 0, 0, 0, time.UTC)),
			DefaultVersionId: aws.String("v1"),
		},
		// AWS-managed AmazonRedshiftAllCommandsFullAccess — attached to
		// redshift-reporting-copy-role (role→policy related-panel pivot).
		{
			PolicyName:       aws.String("AmazonRedshiftAllCommandsFullAccess"),
			PolicyId:         aws.String("ANPAEXAMPLE000000005"),
			Arn:              aws.String("arn:aws:iam::aws:policy/AmazonRedshiftAllCommandsFullAccess"),
			AttachmentCount:  aws.Int32(1),
			Path:             aws.String("/"),
			CreateDate:       aws.Time(time.Date(2016, 3, 21, 0, 0, 0, 0, time.UTC)),
			DefaultVersionId: aws.String("v1"),
		},
	}

	// Issue: AttachmentCount=0 AND IsAttachable=true → Warning (orphaned
	// policy — never attached to any entity, but still attachable so an
	// operator could accidentally wire it up).
	policies = append(policies, iamtypes.Policy{
		PolicyName:       aws.String("orphan-unattached-policy"),
		PolicyId:         aws.String("ANPAEXAMPLE555555555"),
		Arn:              aws.String("arn:aws:iam::123456789012:policy/orphan-unattached-policy"),
		AttachmentCount:  aws.Int32(0),
		IsAttachable:     true,
		Path:             aws.String("/"),
		CreateDate:       aws.Time(time.Date(2024, 5, 1, 11, 0, 0, 0, time.UTC)),
		DefaultVersionId: aws.String("v1"),
		Description:      aws.String("Legacy policy never attached — leftover from a decommissioned project"),
	})

	// Issue: wildcard Action+Resource → Broken (overly permissive policy)
	policies = append(policies, iamtypes.Policy{
		PolicyName:       aws.String("wildcard-allow-policy"),
		PolicyId:         aws.String("ANPAEXAMPLE666666666"),
		Arn:              aws.String("arn:aws:iam::123456789012:policy/wildcard-allow-policy"),
		AttachmentCount:  aws.Int32(1),
		Path:             aws.String("/"),
		CreateDate:       aws.Time(time.Date(2024, 3, 20, 9, 0, 0, 0, time.UTC)),
		DefaultVersionId: aws.String("v1"),
		Description:      aws.String("Legacy admin policy with wildcard actions — should be replaced with least-privilege"),
	})

	// Generate 18 more policies
	policyNames := []string{
		"acme-ec2-describe", "acme-rds-connect", "acme-sqs-send", "acme-sns-publish",
		"acme-dynamodb-access", "acme-ecr-pull", "acme-eks-describe", "acme-lambda-invoke",
		"acme-kms-decrypt", "acme-ssm-read", "acme-cloudtrail-read", "acme-config-read",
		"acme-athena-query", "acme-glue-access", "acme-sfn-execute", "acme-eventbridge-put",
		"acme-kinesis-read", "acme-backup-access",
	}
	for i, name := range policyNames {
		policyID := fmt.Sprintf("ANPAEXAMPLE%09d", 500+i)
		attachCount := int32(1 + i%6)
		createDate := time.Date(2025, time.Month(1+i%12), 1+i%28, 9+i%10, 0, 0, 0, time.UTC)
		policies = append(policies, iamtypes.Policy{
			PolicyName:      aws.String(name),
			PolicyId:        aws.String(policyID),
			Arn:             aws.String("arn:aws:iam::123456789012:policy/" + name),
			AttachmentCount: aws.Int32(attachCount),
			Path:            aws.String("/"),
			CreateDate:      aws.Time(createDate),
		})
	}
	return policies
}

func buildIAMUsers() []iamtypes.User {
	return []iamtypes.User{
		{
			UserName:         aws.String("alice.johnson"),
			UserId:           aws.String("AIDAEXAMPLE111111111"),
			Arn:              aws.String("arn:aws:iam::123456789012:user/alice.johnson"),
			Path:             aws.String("/"),
			CreateDate:       aws.Time(time.Date(2024, 6, 15, 9, 0, 0, 0, time.UTC)),
			PasswordLastUsed: aws.Time(time.Date(2026, 3, 20, 14, 22, 0, 0, time.UTC)),
		},
		{
			UserName:         aws.String("bob.smith"),
			UserId:           aws.String("AIDAEXAMPLE222222222"),
			Arn:              aws.String("arn:aws:iam::123456789012:user/bob.smith"),
			Path:             aws.String("/"),
			CreateDate:       aws.Time(time.Date(2024, 9, 1, 10, 30, 0, 0, time.UTC)),
			PasswordLastUsed: aws.Time(time.Date(2026, 3, 19, 8, 55, 0, 0, time.UTC)),
		},
		{
			UserName:   aws.String("ci-service-account"),
			UserId:     aws.String("AIDAEXAMPLE333333333"),
			Arn:        aws.String("arn:aws:iam::123456789012:user/service-accounts/ci-service-account"),
			Path:       aws.String("/service-accounts/"),
			CreateDate: aws.Time(time.Date(2025, 1, 10, 12, 0, 0, 0, time.UTC)),
		},
		// CT-event cross-reference users
		{
			UserName:         aws.String("bob"),
			UserId:           aws.String("AIDAEXAMPLE444444444"),
			Arn:              aws.String("arn:aws:iam::333333333333:user/bob"),
			Path:             aws.String("/"),
			CreateDate:       aws.Time(time.Date(2025, 11, 1, 9, 0, 0, 0, time.UTC)),
			PasswordLastUsed: aws.Time(time.Date(2026, 4, 7, 16, 0, 0, 0, time.UTC)),
		},
		{
			UserName:   aws.String("charlie"),
			UserId:     aws.String("AIDAEXAMPLE555555555"),
			Arn:        aws.String("arn:aws:iam::123456789012:user/charlie"),
			Path:       aws.String("/"),
			CreateDate: aws.Time(time.Date(2026, 4, 7, 15, 10, 5, 0, time.UTC)),
		},
		// Issue: PasswordLastUsed >365d ago → Warning (dormant user, possible stale credential)
		{
			UserName:         aws.String("former-contractor"),
			UserId:           aws.String("AIDAEXAMPLE666666666"),
			Arn:              aws.String("arn:aws:iam::123456789012:user/former-contractor"),
			Path:             aws.String("/"),
			CreateDate:       aws.Time(time.Date(2023, 1, 15, 9, 0, 0, 0, time.UTC)),
			PasswordLastUsed: aws.Time(time.Date(2024, 6, 1, 14, 0, 0, 0, time.UTC)),
		},
		// Witness: console password created long ago and never used.
		{
			UserName:   aws.String(IAMUserConsoleNeverUsed),
			UserId:     aws.String("AIDAEXAMPLECONSNEVER"),
			Arn:        aws.String("arn:aws:iam::123456789012:user/" + IAMUserConsoleNeverUsed),
			Path:       aws.String("/"),
			CreateDate: aws.Time(time.Date(2024, 2, 1, 9, 0, 0, 0, time.UTC)),
		},
		// Witness: one active access key, old and never used.
		{
			UserName:   aws.String(IAMUserKeyUnused),
			UserId:     aws.String("AIDAEXAMPLESTALEKEY1"),
			Arn:        aws.String("arn:aws:iam::123456789012:user/service-accounts/" + IAMUserKeyUnused),
			Path:       aws.String("/service-accounts/"),
			CreateDate: aws.Time(time.Date(2024, 5, 12, 9, 0, 0, 0, time.UTC)),
		},
		// Witness: both access-key slots active at once.
		{
			UserName:   aws.String(IAMUserTwoKeys),
			UserId:     aws.String("AIDAEXAMPLEDUALKEY01"),
			Arn:        aws.String("arn:aws:iam::123456789012:user/service-accounts/" + IAMUserTwoKeys),
			Path:       aws.String("/service-accounts/"),
			CreateDate: aws.Time(time.Date(2025, 11, 3, 9, 0, 0, 0, time.UTC)),
		},
		// Witness: AdministratorAccess attached directly to a user.
		{
			UserName:   aws.String(IAMUserAdminAttached),
			UserId:     aws.String("AIDAEXAMPLEADMINUSR1"),
			Arn:        aws.String("arn:aws:iam::123456789012:user/" + IAMUserAdminAttached),
			Path:       aws.String("/"),
			CreateDate: aws.Time(time.Date(2024, 10, 4, 9, 0, 0, 0, time.UTC)),
		},
		// Issue: PasswordLastUsed=nil (never logged in) with old CreateDate → Warning (unused console access)
		{
			UserName:   aws.String("legacy-service-user"),
			UserId:     aws.String("AIDAEXAMPLE777777777"),
			Arn:        aws.String("arn:aws:iam::123456789012:user/service-accounts/legacy-service-user"),
			Path:       aws.String("/service-accounts/"),
			CreateDate: aws.Time(time.Date(2022, 11, 10, 8, 0, 0, 0, time.UTC)),
			// PasswordLastUsed omitted — user never logged in via console
		},
	}
}

func buildIAMGroups() []iamtypes.Group {
	return []iamtypes.Group{
		{
			GroupName:  aws.String("admins"),
			GroupId:    aws.String("AGPAEXAMPLE111111111"),
			Arn:        aws.String("arn:aws:iam::123456789012:group/admins"),
			Path:       aws.String("/"),
			CreateDate: aws.Time(time.Date(2024, 3, 1, 8, 0, 0, 0, time.UTC)),
		},
		{
			GroupName:  aws.String("developers"),
			GroupId:    aws.String("AGPAEXAMPLE222222222"),
			Arn:        aws.String("arn:aws:iam::123456789012:group/developers"),
			Path:       aws.String("/"),
			CreateDate: aws.Time(time.Date(2024, 3, 1, 8, 5, 0, 0, time.UTC)),
		},
		{
			GroupName:  aws.String("readonly"),
			GroupId:    aws.String("AGPAEXAMPLE333333333"),
			Arn:        aws.String("arn:aws:iam::123456789012:group/readonly"),
			Path:       aws.String("/"),
			CreateDate: aws.Time(time.Date(2024, 3, 1, 8, 10, 0, 0, time.UTC)),
		},
		{
			GroupName:  aws.String(IAMGroupPowerUser),
			GroupId:    aws.String("AGPAEXAMPLE555555555"),
			Arn:        aws.String("arn:aws:iam::123456789012:group/" + IAMGroupPowerUser),
			Path:       aws.String("/"),
			CreateDate: aws.Time(time.Date(2024, 5, 6, 9, 0, 0, 0, time.UTC)),
		},
		// Issue: empty group (no users) → Warning (unused IAM group, potential access confusion)
		{
			GroupName:  aws.String("empty-group"),
			GroupId:    aws.String("AGPAEXAMPLE444444444"),
			Arn:        aws.String("arn:aws:iam::123456789012:group/empty-group"),
			Path:       aws.String("/"),
			CreateDate: aws.Time(time.Date(2024, 8, 15, 10, 0, 0, 0, time.UTC)),
		},
	}
}

func buildIAMRelations(f *IAMFixtures) {
	// Attached role policies
	f.AttachedRolePolicies["acme-eks-node-role"] = []iamtypes.AttachedPolicy{
		{PolicyName: aws.String("AmazonEKSWorkerNodePolicy"), PolicyArn: aws.String("arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy")},
		{PolicyName: aws.String("AmazonEC2ContainerRegistryReadOnly"), PolicyArn: aws.String("arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly")},
	}
	f.AttachedRolePolicies["acme-lambda-execution"] = []iamtypes.AttachedPolicy{
		{PolicyName: aws.String("acme-cloudwatch-logs"), PolicyArn: aws.String("arn:aws:iam::123456789012:policy/acme-cloudwatch-logs")},
		{PolicyName: aws.String("acme-s3-read-only"), PolicyArn: aws.String("arn:aws:iam::123456789012:policy/acme-s3-read-only")},
	}
	f.InlineRolePolicies["acme-eks-node-role"] = []string{"trust-policy"}
	f.InlineRolePolicies["acme-lambda-execution"] = []string{"logging-policy"}
	// s3→role pivot: the demo role has an inline policy whose Statement[].
	// Resource mentions HealthyBucketARN. Enriched via ListRolePolicies +
	// GetRolePolicy during the IAM roles fetch.
	f.InlineRolePolicies["a9s-demo-s3-access-role"] = []string{"s3-bucket-access"}
	// Graph-root roles that must list non-empty policies so the
	// role→role_policies drill lands on content.
	f.InlineRolePolicies["AcmeBackupRoleProd"] = []string{"backup-access"}
	f.AttachedRolePolicies["AcmeBackupRoleProd"] = []iamtypes.AttachedPolicy{
		{PolicyName: aws.String("AWSBackupServiceRolePolicyForBackup"), PolicyArn: aws.String("arn:aws:iam::aws:policy/service-role/AWSBackupServiceRolePolicyForBackup")},
	}
	f.InlineRolePolicies["redshift-reporting-copy-role"] = []string{"s3-audit-copy"}
	f.AttachedRolePolicies["redshift-reporting-copy-role"] = []iamtypes.AttachedPolicy{
		{PolicyName: aws.String("AmazonRedshiftAllCommandsFullAccess"), PolicyArn: aws.String("arn:aws:iam::aws:policy/AmazonRedshiftAllCommandsFullAccess")},
	}

	// Attached user policies
	f.AttachedUserPolicies["alice.johnson"] = []iamtypes.AttachedPolicy{
		{PolicyName: aws.String("acme-s3-read-only"), PolicyArn: aws.String("arn:aws:iam::123456789012:policy/acme-s3-read-only")},
	}
	f.AttachedUserPolicies[IAMUserAdminAttached] = []iamtypes.AttachedPolicy{
		{PolicyName: aws.String("AdministratorAccess"), PolicyArn: aws.String("arn:aws:iam::aws:policy/AdministratorAccess")},
	}

	// Witness attachments and inline documents for the escalation findings.
	f.AttachedRolePolicies[RoleAdminAttached] = []iamtypes.AttachedPolicy{
		{PolicyName: aws.String("AdministratorAccess"), PolicyArn: aws.String("arn:aws:iam::aws:policy/AdministratorAccess")},
	}
	f.InlineRolePolicies[RoleInlinePrivEsc] = []string{"pipeline-deploy"}
	f.InlinePolicyDocuments[RoleInlinePrivEsc+"/pipeline-deploy"] = url.PathEscape(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["iam:PassRole","lambda:CreateFunction","lambda:InvokeFunction"],"Resource":"*"}]}`)
	f.PolicyDocuments["arn:aws:iam::123456789012:policy/"+PolicyPrivEsc] = url.PathEscape(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["iam:PassRole","lambda:CreateFunction","lambda:InvokeFunction"],"Resource":"*"}]}`)

	// Attached group policies
	f.AttachedGroupPolicies["admins"] = []iamtypes.AttachedPolicy{
		{PolicyName: aws.String("AdministratorAccess"), PolicyArn: aws.String("arn:aws:iam::aws:policy/AdministratorAccess")},
	}
	f.AttachedGroupPolicies[IAMGroupPowerUser] = []iamtypes.AttachedPolicy{
		{PolicyName: aws.String("PowerUserAccess"), PolicyArn: aws.String("arn:aws:iam::aws:policy/PowerUserAccess")},
	}
	f.AttachedGroupPolicies["developers"] = []iamtypes.AttachedPolicy{
		{PolicyName: aws.String("acme-s3-read-only"), PolicyArn: aws.String("arn:aws:iam::123456789012:policy/acme-s3-read-only")},
		{PolicyName: aws.String("acme-deploy-policy"), PolicyArn: aws.String("arn:aws:iam::123456789012:policy/acme-deploy-policy")},
	}

	// Group users
	f.GroupUsers["admins"] = []iamtypes.User{
		{UserName: aws.String("alice.johnson"), UserId: aws.String("AIDAEXAMPLE111111111"), Arn: aws.String("arn:aws:iam::123456789012:user/alice.johnson"), Path: aws.String("/"), CreateDate: aws.Time(time.Date(2024, 6, 15, 9, 0, 0, 0, time.UTC))},
	}
	f.GroupUsers["developers"] = []iamtypes.User{
		{UserName: aws.String("alice.johnson"), UserId: aws.String("AIDAEXAMPLE111111111"), Arn: aws.String("arn:aws:iam::123456789012:user/alice.johnson"), Path: aws.String("/"), CreateDate: aws.Time(time.Date(2024, 6, 15, 9, 0, 0, 0, time.UTC))},
		{UserName: aws.String("bob.smith"), UserId: aws.String("AIDAEXAMPLE222222222"), Arn: aws.String("arn:aws:iam::123456789012:user/bob.smith"), Path: aws.String("/"), CreateDate: aws.Time(time.Date(2024, 9, 1, 10, 30, 0, 0, time.UTC))},
	}

	f.GroupUsers[IAMGroupPowerUser] = []iamtypes.User{
		{UserName: aws.String("bob.smith"), UserId: aws.String("AIDAEXAMPLE222222222"), Arn: aws.String("arn:aws:iam::123456789012:user/bob.smith"), Path: aws.String("/"), CreateDate: aws.Time(time.Date(2024, 9, 1, 10, 30, 0, 0, time.UTC))},
	}

	// Groups for user
	f.GroupsForUser["alice.johnson"] = []iamtypes.Group{
		{GroupName: aws.String("admins"), GroupId: aws.String("AGPAEXAMPLE111111111"), Arn: aws.String("arn:aws:iam::123456789012:group/admins"), Path: aws.String("/"), CreateDate: aws.Time(time.Date(2024, 3, 1, 8, 0, 0, 0, time.UTC))},
		{GroupName: aws.String("developers"), GroupId: aws.String("AGPAEXAMPLE222222222"), Arn: aws.String("arn:aws:iam::123456789012:group/developers"), Path: aws.String("/"), CreateDate: aws.Time(time.Date(2024, 3, 1, 8, 5, 0, 0, time.UTC))},
	}

	// Entities for policy
	f.EntitiesForPolicy["arn:aws:iam::123456789012:policy/acme-s3-read-only"] = &PolicyEntities{
		Roles: []iamtypes.PolicyRole{
			{RoleName: aws.String("acme-lambda-execution"), RoleId: aws.String("AROAEXAMPLE222222222")},
		},
		Users: []iamtypes.PolicyUser{
			{UserName: aws.String("alice.johnson"), UserId: aws.String("AIDAEXAMPLE111111111")},
		},
		Groups: []iamtypes.PolicyGroup{
			{GroupName: aws.String("developers"), GroupId: aws.String("AGPAEXAMPLE222222222")},
		},
	}
	f.EntitiesForPolicy["arn:aws:iam::aws:policy/PowerUserAccess"] = &PolicyEntities{
		Groups: []iamtypes.PolicyGroup{
			{GroupName: aws.String(IAMGroupPowerUser), GroupId: aws.String("AGPAEXAMPLE555555555")},
		},
	}
	f.EntitiesForPolicy["arn:aws:iam::aws:policy/AdministratorAccess"] = &PolicyEntities{
		Groups: []iamtypes.PolicyGroup{
			{GroupName: aws.String("admins"), GroupId: aws.String("AGPAEXAMPLE111111111")},
		},
	}

	// Policy documents (URL-encoded JSON)
	f.PolicyDocuments["arn:aws:iam::123456789012:policy/acme-cloudwatch-logs"] = url.PathEscape(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["logs:CreateLogGroup","logs:CreateLogStream","logs:PutLogEvents"],"Resource":"arn:aws:logs:*:123456789012:*"}]}`)

	f.PolicyDocuments["arn:aws:iam::123456789012:policy/acme-s3-read-only"] = url.PathEscape(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["s3:GetObject","s3:ListBucket"],"Resource":["arn:aws:s3:::acme-data-*","arn:aws:s3:::acme-data-*/*"]}]}`)

	f.PolicyDocuments["arn:aws:iam::123456789012:policy/acme-deploy-policy"] = url.PathEscape(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["codedeploy:GetApplication","codedeploy:ListDeployments","s3:GetObject","s3:PutObject"],"Resource":"*"}]}`)

	f.PolicyDocuments["arn:aws:iam::123456789012:policy/acme-secrets-access"] = url.PathEscape(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["secretsmanager:GetSecretValue","secretsmanager:DescribeSecret"],"Resource":"arn:aws:secretsmanager:us-east-1:123456789012:secret:acme/*"}]}`)

	f.PolicyDocuments["arn:aws:iam::aws:policy/PowerUserAccess"] = url.PathEscape(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","NotAction":["iam:*","organizations:*","account:*"],"Resource":"*"}]}`)

	f.PolicyDocuments["arn:aws:iam::aws:policy/AdministratorAccess"] = url.PathEscape(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"*","Resource":"*"}]}`)

	// AWS-managed policies used in role_policies
	f.PolicyDocuments["arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy"] = url.PathEscape(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["ec2:DescribeInstances","ec2:DescribeRouteTables","ec2:DescribeSecurityGroups","ec2:DescribeSubnets","ec2:DescribeVolumes","ec2:DescribeVpcs","eks:DescribeCluster"],"Resource":"*"}]}`)

	f.PolicyDocuments["arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"] = url.PathEscape(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["ecr:GetAuthorizationToken","ecr:BatchCheckLayerAvailability","ecr:GetDownloadUrlForLayer","ecr:BatchGetImage"],"Resource":"*"}]}`)

	// Inline policy documents
	f.InlinePolicyDocuments["acme-eks-node-role/trust-policy"] = url.PathEscape(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"ec2.amazonaws.com"},"Action":"sts:AssumeRole"}]}`)

	f.InlinePolicyDocuments["acme-lambda-execution/logging-policy"] = url.PathEscape(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["logs:CreateLogStream","logs:PutLogEvents"],"Resource":"arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/*"}]}`)
	f.InlinePolicyDocuments["a9s-demo-s3-access-role/s3-bucket-access"] = url.PathEscape(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["s3:GetObject","s3:ListBucket"],"Resource":["` + HealthyBucketARN + `","` + HealthyBucketARN + `/*"]}]}`)

	// Issue role: wildcard trust — no external-id condition
	f.PolicyDocuments["arn:aws:iam::123456789012:policy/orphan-unattached-policy"] = url.PathEscape(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["s3:GetObject"],"Resource":"arn:aws:s3:::acme-old-project-*/*"}]}`)

	f.PolicyDocuments["arn:aws:iam::123456789012:policy/wildcard-allow-policy"] = url.PathEscape(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"*","Resource":"*"}]}`)
}
