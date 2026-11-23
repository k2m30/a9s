package main

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"
)

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
	AccessKeysRead   readOutcome    `json:"access_keys_read"`
	AccessKeys       []iamAccessKey `json:"access_keys"`
	// LoginProfile is "present", or outcomeNone when the user has no console
	// password.
	LoginProfile   readOutcome  `json:"login_profile"`
	MFADevices     *readOutcome `json:"mfa_devices,omitempty"`
	MFADeviceCount int          `json:"mfa_device_count,omitempty"`
}

// iamAccessKey captures ListAccessKeys + GetAccessKeyLastUsed per key.
type iamAccessKey struct {
	AccessKeyId  string      `json:"access_key_id"`
	Status       string      `json:"status"`
	CreateDate   string      `json:"create_date,omitempty"`
	LastUsed     readOutcome `json:"last_used"`
	LastUsedDate string      `json:"last_used_date,omitempty"`
}

// captureIAMUser lists every IAM user (ListUsers, all pages) and captures the
// access-key ages and MFA/login-profile posture.
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
				CreateDate:       snapFormatTime(u.CreateDate),
				PasswordLastUsed: snapFormatTime(u.PasswordLastUsed),
			})
		}
	}

	for i := range users {
		users[i].AccessKeys, users[i].AccessKeysRead = captureIAMUserAccessKeys(ctx, client, users[i].UserName)
		users[i].LoginProfile = captureIAMLoginProfile(ctx, client, users[i].UserName)
		if users[i].LoginProfile.Outcome == "present" {
			devices, err := pageAll(ctx, &iam.ListMFADevicesInput{UserName: aws.String(users[i].UserName)},
				func(ctx context.Context, in *iam.ListMFADevicesInput) (*iam.ListMFADevicesOutput, error) {
					return client.ListMFADevices(ctx, in)
				},
				func(out *iam.ListMFADevicesOutput) []iamtypes.MFADevice { return out.MFADevices }, allPages)
			read := readOf(err)
			users[i].MFADevices = &read
			users[i].MFADeviceCount = len(devices)
		}
	}

	return iamUserData{Users: users}, nil
}

func captureIAMUserAccessKeys(ctx context.Context, client *iam.Client, userName string) ([]iamAccessKey, readOutcome) {
	metas, err := pageAll(ctx, &iam.ListAccessKeysInput{UserName: aws.String(userName)},
		func(ctx context.Context, in *iam.ListAccessKeysInput) (*iam.ListAccessKeysOutput, error) {
			return client.ListAccessKeys(ctx, in)
		},
		func(out *iam.ListAccessKeysOutput) []iamtypes.AccessKeyMetadata { return out.AccessKeyMetadata }, allPages)
	keys := make([]iamAccessKey, 0, len(metas))
	for _, meta := range metas {
		key := iamAccessKey{
			AccessKeyId: aws.ToString(meta.AccessKeyId),
			Status:      string(meta.Status),
			CreateDate:  snapFormatTime(meta.CreateDate),
		}
		lastUsedOut, lastUsedErr := client.GetAccessKeyLastUsed(ctx, &iam.GetAccessKeyLastUsedInput{AccessKeyId: meta.AccessKeyId})
		key.LastUsed = readOf(lastUsedErr)
		if lastUsedOut != nil && lastUsedOut.AccessKeyLastUsed != nil {
			key.LastUsedDate = snapFormatTime(lastUsedOut.AccessKeyLastUsed.LastUsedDate)
		}
		keys = append(keys, key)
	}
	return keys, readOf(err)
}

func captureIAMLoginProfile(ctx context.Context, client *iam.Client, userName string) readOutcome {
	_, err := client.GetLoginProfile(ctx, &iam.GetLoginProfileInput{UserName: aws.String(userName)})
	if err != nil {
		return readOf(err)
	}
	return readOutcome{Outcome: "present"}
}

// iamGroupData is the raw view of IAM groups the checklist generator reads.
type iamGroupData struct {
	Groups []iamGroup `json:"groups"`
}

type iamGroup struct {
	GroupName  string      `json:"group_name"`
	Arn        string      `json:"arn"`
	CreateDate string      `json:"create_date,omitempty"`
	Members    readOutcome `json:"members"`
	UserCount  int         `json:"user_count,omitempty"`
}

// captureIAMGroup lists every IAM group (ListGroups, all pages) and captures
// GetGroup's Users[] count (empty-group-orphan signal).
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
				CreateDate: snapFormatTime(g.CreateDate),
			})
		}
	}

	for i := range groups {
		users, err := pageAll(ctx, &iam.GetGroupInput{GroupName: aws.String(groups[i].GroupName)},
			func(ctx context.Context, in *iam.GetGroupInput) (*iam.GetGroupOutput, error) {
				return client.GetGroup(ctx, in)
			},
			func(out *iam.GetGroupOutput) []iamtypes.User { return out.Users }, allPages)
		groups[i].Members = readOf(err)
		groups[i].UserCount = len(users)
	}

	return iamGroupData{Groups: groups}, nil
}

// roleData is the raw view of IAM roles the checklist generator reads.
type roleData struct {
	Roles []roleEntry `json:"roles"`
}

type roleEntry struct {
	RoleName                 string             `json:"role_name"`
	Arn                      string             `json:"arn"`
	CreateDate               string             `json:"create_date,omitempty"`
	AssumeRolePolicyDocument string             `json:"assume_role_policy_document,omitempty"`
	RoleLastUsed             readOutcome        `json:"role_last_used"`
	RoleLastUsedDate         string             `json:"role_last_used_date,omitempty"`
	InlinePoliciesRead       readOutcome        `json:"inline_policies_read"`
	InlinePolicies           []roleInlinePolicy `json:"inline_policies,omitempty"`
}

type roleInlinePolicy struct {
	PolicyName string `json:"policy_name"`
	readOutcome
	// Document is URL-encoded, as IAM returns it.
	Document string `json:"document,omitempty"`
}

func captureRoleInlinePolicies(ctx context.Context, client *iam.Client, roleName string) ([]roleInlinePolicy, readOutcome) {
	var policies []roleInlinePolicy
	paginator := iam.NewListRolePoliciesPaginator(client, &iam.ListRolePoliciesInput{RoleName: aws.String(roleName)})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return policies, readOf(err)
		}
		for _, name := range out.PolicyNames {
			got, getErr := client.GetRolePolicy(ctx, &iam.GetRolePolicyInput{RoleName: aws.String(roleName), PolicyName: aws.String(name)})
			p := roleInlinePolicy{PolicyName: name, readOutcome: readOf(getErr)}
			if got != nil {
				p.Document = aws.ToString(got.PolicyDocument)
			}
			policies = append(policies, p)
		}
	}
	return policies, readOf(nil)
}

// captureRole lists every IAM role (ListRoles, all pages) and captures
// RoleLastUsed via GetRole.
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
				CreateDate:               snapFormatTime(r.CreateDate),
				AssumeRolePolicyDocument: aws.ToString(r.AssumeRolePolicyDocument),
			})
		}
	}

	for i := range roles {
		roles[i].InlinePolicies, roles[i].InlinePoliciesRead = captureRoleInlinePolicies(ctx, client, roles[i].RoleName)
		role, read := readOne(func() (*iamtypes.Role, error) {
			out, err := client.GetRole(ctx, &iam.GetRoleInput{RoleName: aws.String(roles[i].RoleName)})
			if err != nil {
				return nil, err
			}
			return out.Role, nil
		})
		roles[i].RoleLastUsed = read
		if role != nil && role.RoleLastUsed != nil {
			roles[i].RoleLastUsedDate = snapFormatTime(role.RoleLastUsed.LastUsedDate)
		}
	}

	return roleData{Roles: roles}, nil
}

// policyData is the raw view of customer-managed IAM policies the checklist generator reads.
type policyData struct {
	Policies []policyEntry `json:"policies"`
	// GroupInlinePolicies are the policies embedded in groups, which the
	// policy list shows as rows of their own.
	GroupsRead          readOutcome           `json:"groups_read"`
	GroupInlinePolicies []groupInlinePolicies `json:"group_inline_policies"`
}

type policyEntry struct {
	PolicyName       string      `json:"policy_name"`
	Arn              string      `json:"arn"`
	AttachmentCount  int32       `json:"attachment_count"`
	DefaultVersionId string      `json:"default_version_id,omitempty"`
	CreateDate       string      `json:"create_date,omitempty"`
	UpdateDate       string      `json:"update_date,omitempty"`
	DocumentRead     readOutcome `json:"document_read"`
	Document         string      `json:"document,omitempty"`
}

type groupInlinePolicies struct {
	GroupName string `json:"group_name"`
	readOutcome
	PolicyNames []string `json:"policy_names,omitempty"`
}

func captureGroupInlinePolicies(ctx context.Context, client *iam.Client) (readOutcome, []groupInlinePolicies) {
	groups, err := pageAll(ctx, &iam.ListGroupsInput{},
		func(ctx context.Context, in *iam.ListGroupsInput) (*iam.ListGroupsOutput, error) {
			return client.ListGroups(ctx, in)
		},
		func(out *iam.ListGroupsOutput) []iamtypes.Group { return out.Groups }, allPages)
	names := make([]string, 0, len(groups))
	for _, g := range groups {
		names = append(names, aws.ToString(g.GroupName))
	}
	rows := describeListed(names, 1, func(batch []string) (map[string][]string, []string, error) {
		policies, listErr := pageAll(ctx, &iam.ListGroupPoliciesInput{GroupName: aws.String(batch[0])},
			func(ctx context.Context, in *iam.ListGroupPoliciesInput) (*iam.ListGroupPoliciesOutput, error) {
				return client.ListGroupPolicies(ctx, in)
			},
			func(out *iam.ListGroupPoliciesOutput) []string { return out.PolicyNames }, allPages)
		if listErr != nil {
			return nil, nil, listErr
		}
		return map[string][]string{batch[0]: policies}, nil, nil
	})
	out := make([]groupInlinePolicies, 0, len(rows))
	for _, row := range rows {
		g := groupInlinePolicies{GroupName: row.id, readOutcome: row.read}
		if row.item != nil {
			g.PolicyNames = *row.item
		}
		out = append(out, g)
	}
	return readOf(err), out
}

// capturePolicy lists every customer-managed policy (ListPolicies with
// Scope=Local, all pages) and captures the default version's Document
// (wildcard-admin detection).
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
				CreateDate:       snapFormatTime(p.CreateDate),
				UpdateDate:       snapFormatTime(p.UpdateDate),
			})
		}
	}

	for i := range policies {
		if policies[i].DefaultVersionId == "" {
			continue
		}
		version, read := readOne(func() (*iamtypes.PolicyVersion, error) {
			out, err := client.GetPolicyVersion(ctx, &iam.GetPolicyVersionInput{
				PolicyArn: aws.String(policies[i].Arn),
				VersionId: aws.String(policies[i].DefaultVersionId),
			})
			if err != nil {
				return nil, err
			}
			return out.PolicyVersion, nil
		})
		policies[i].DocumentRead = read
		if version != nil {
			policies[i].Document = aws.ToString(version.Document)
		}
	}

	data := policyData{Policies: policies}
	data.GroupsRead, data.GroupInlinePolicies = captureGroupInlinePolicies(ctx, client)
	return data, nil
}

// kmsData is the raw view of KMS keys the checklist generator reads.
type kmsData struct {
	Keys []kmsKey `json:"keys"`
	// Aliases is the account's ListAliases answer the key list names its keys
	// by.
	Aliases readOutcome `json:"aliases"`
}

type kmsKey struct {
	KeyId              string       `json:"key_id"`
	KeyArn             string       `json:"key_arn"`
	AliasNames         []string     `json:"alias_names,omitempty"`
	Describe           readOutcome  `json:"describe"`
	KeyState           string       `json:"key_state,omitempty"`
	KeyManager         string       `json:"key_manager,omitempty"`
	KeySpec            string       `json:"key_spec,omitempty"`
	Origin             string       `json:"origin,omitempty"`
	CustomKeyStoreId   string       `json:"custom_key_store_id,omitempty"`
	Rotation           *readOutcome `json:"rotation,omitempty"`
	KeyRotationEnabled bool         `json:"key_rotation_enabled,omitempty"`
}

// ListKeys returns ids only: the state and manager come from DescribeKey, the
// names from the account's aliases.
func captureKMS(ctx context.Context, cfg aws.Config) (any, error) {
	client := kms.NewFromConfig(cfg)

	var ids []string
	arns := map[string]string{}
	paginator := kms.NewListKeysPaginator(client, &kms.ListKeysInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, k := range out.Keys {
			ids = append(ids, aws.ToString(k.KeyId))
			arns[aws.ToString(k.KeyId)] = aws.ToString(k.KeyArn)
		}
	}

	aliases, aliasesRead := captureKMSAliases(ctx, client)
	rows := describeListed(ids, 1, func(batch []string) (map[string]kmstypes.KeyMetadata, []string, error) {
		out, err := client.DescribeKey(ctx, &kms.DescribeKeyInput{KeyId: aws.String(batch[0])})
		if err != nil {
			return nil, nil, err
		}
		return describedAs(batch[0], out.KeyMetadata), nil, nil
	})
	keys := make([]kmsKey, 0, len(rows))
	for _, row := range rows {
		key := kmsKey{KeyId: row.id, KeyArn: arns[row.id], AliasNames: aliases[row.id], Describe: row.read}
		if m := row.item; m != nil {
			key.KeyState = string(m.KeyState)
			key.KeyManager = string(m.KeyManager)
			key.KeySpec = string(m.KeySpec)
			key.Origin = string(m.Origin)
			key.CustomKeyStoreId = aws.ToString(m.CustomKeyStoreId)
			if kmsKeyRotates(m) {
				rotationOut, err := client.GetKeyRotationStatus(ctx, &kms.GetKeyRotationStatusInput{KeyId: aws.String(row.id)})
				read := readOf(err)
				key.Rotation = &read
				if rotationOut != nil {
					key.KeyRotationEnabled = rotationOut.KeyRotationEnabled
				}
			}
		}
		keys = append(keys, key)
	}

	return kmsData{Keys: keys, Aliases: aliasesRead}, nil
}

// kmsKeyRotates reports whether a key's rotation is the operator's to set:
// automatic rotation exists only for a symmetric encryption key whose key
// material KMS generated, outside a custom key store, and KMS rotates
// AWS-managed keys itself.
func kmsKeyRotates(m *kmstypes.KeyMetadata) bool {
	return m.KeyManager == kmstypes.KeyManagerTypeCustomer && m.KeySpec == kmstypes.KeySpecSymmetricDefault &&
		m.Origin == kmstypes.OriginTypeAwsKms && aws.ToString(m.CustomKeyStoreId) == ""
}

// captureKMSAliases returns the alias names of every key, in ListAliases
// order.
func captureKMSAliases(ctx context.Context, client *kms.Client) (map[string][]string, readOutcome) {
	byKey := map[string][]string{}
	paginator := kms.NewListAliasesPaginator(client, &kms.ListAliasesInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return byKey, readOf(err)
		}
		for _, a := range out.Aliases {
			if id := aws.ToString(a.TargetKeyId); id != "" {
				byKey[id] = append(byKey[id], aws.ToString(a.AliasName))
			}
		}
	}
	return byKey, readOf(nil)
}

// secretsData is the raw view of Secrets Manager secrets the checklist generator reads.
type secretsData struct {
	Secrets []secretEntry `json:"secrets"`
}

type secretEntry struct {
	Name                   string `json:"name"`
	Arn                    string `json:"arn"`
	KmsKeyId               string `json:"kms_key_id,omitempty"`
	RotationEnabled        bool   `json:"rotation_enabled"`
	RotationLambdaARN      string `json:"rotation_lambda_arn,omitempty"`
	LastRotatedDate        string `json:"last_rotated_date,omitempty"`
	NextRotationDate       string `json:"next_rotation_date,omitempty"`
	LastAccessedDate       string `json:"last_accessed_date,omitempty"`
	DeletedDate            string `json:"deleted_date,omitempty"`
	AutomaticallyAfterDays int64  `json:"automatically_after_days,omitempty"`
}

// captureSecrets lists every secret (ListSecrets, all pages, with the secrets
// scheduled for deletion); every secret signal reads the list entry.
func captureSecrets(ctx context.Context, cfg aws.Config) (any, error) {
	client := secretsmanager.NewFromConfig(cfg)

	var secrets []secretEntry
	paginator := secretsmanager.NewListSecretsPaginator(client, &secretsmanager.ListSecretsInput{IncludePlannedDeletion: aws.Bool(true)})
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
				LastRotatedDate:   snapFormatTime(s.LastRotatedDate),
				NextRotationDate:  snapFormatTime(s.NextRotationDate),
				LastAccessedDate:  snapFormatTime(s.LastAccessedDate),
				DeletedDate:       snapFormatTime(s.DeletedDate),
			}
			if s.RotationRules != nil {
				entry.AutomaticallyAfterDays = aws.ToInt64(s.RotationRules.AutomaticallyAfterDays)
			}
			secrets = append(secrets, entry)
		}
	}

	return secretsData{Secrets: secrets}, nil
}

// acmData is the raw view of ACM certificates the checklist generator reads.
type acmData struct {
	Certificates []acmCertificate `json:"certificates"`
}

type acmCertificate struct {
	CertificateArn string `json:"certificate_arn"`
	DomainName     string `json:"domain_name,omitempty"`
	Status         string `json:"status,omitempty"`
	NotBefore      string `json:"not_before,omitempty"`
	NotAfter       string `json:"not_after,omitempty"`
	InUse          bool   `json:"in_use"`
	KeyAlgorithm   string `json:"key_algorithm,omitempty"`
}

// captureACM lists every certificate (ListCertificates, all pages); every
// certificate signal reads the list summary.
func captureACM(ctx context.Context, cfg aws.Config) (any, error) {
	client := acm.NewFromConfig(cfg)

	var certs []acmCertificate
	// The fetcher's filters: unfiltered, ListCertificates leaves out most key
	// types and every ACME-issued certificate.
	paginator := acm.NewListCertificatesPaginator(client, &acm.ListCertificatesInput{
		Includes:                  &acmtypes.Filters{KeyTypes: acmtypes.KeyAlgorithm("").Values()},
		CertificateKeyPairOrigins: acmtypes.CertificateKeyPairOrigin("").Values(),
	})
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
				NotBefore:      snapFormatTime(c.NotBefore),
				NotAfter:       snapFormatTime(c.NotAfter),
				InUse:          aws.ToBool(c.InUse),
				KeyAlgorithm:   string(c.KeyAlgorithm),
			})
		}
	}

	return acmData{Certificates: certs}, nil
}

// wafData is the raw view of WAFv2 Web ACLs the checklist generator reads, across both the
// REGIONAL and CLOUDFRONT scopes.
type wafData struct {
	WebACLs []wafWebACL `json:"web_acls"`
	partialRead
}

type wafWebACL struct {
	Name                 string      `json:"name"`
	Id                   string      `json:"id"`
	ARN                  string      `json:"arn"`
	Scope                string      `json:"scope"`
	Describe             readOutcome `json:"describe"`
	RuleCount            int         `json:"rule_count,omitempty"`
	DefaultActionIsAllow bool        `json:"default_action_is_allow,omitempty"`
}

// wafClientFor returns the client that serves a scope. AWS serves every
// CLOUDFRONT-scope operation from the us-east-1 endpoint alone, whatever
// region the profile names, so that scope is read through a client of its
// own.
func wafClientFor(cfg aws.Config, scope wafv2types.Scope) *wafv2.Client {
	if scope == wafv2types.ScopeCloudfront {
		edgeCfg := cfg.Copy()
		edgeCfg.Region = "us-east-1"
		return wafv2.NewFromConfig(edgeCfg)
	}
	return wafv2.NewFromConfig(cfg)
}

func wafScopePart(scope wafv2types.Scope) part[wafData] {
	return part[wafData]{name: string(scope), run: func(ctx context.Context, cfg aws.Config, d *wafData) error {
		client := wafClientFor(cfg, scope)
		summaries, err := pageAll(ctx, &wafv2.ListWebACLsInput{Scope: scope},
			func(ctx context.Context, in *wafv2.ListWebACLsInput) (*wafv2.ListWebACLsOutput, error) {
				return client.ListWebACLs(ctx, in)
			},
			func(out *wafv2.ListWebACLsOutput) []wafv2types.WebACLSummary { return out.WebACLs }, allPages)
		for _, w := range summaries {
			d.WebACLs = append(d.WebACLs, wafWebACL{
				Name:  aws.ToString(w.Name),
				Id:    aws.ToString(w.Id),
				ARN:   aws.ToString(w.ARN),
				Scope: string(scope),
			})
		}
		return err
	}}
}

func wafDescribe(ctx context.Context, cfg aws.Config, d *wafData) error {
	acls := d.WebACLs
	for i := range acls {
		scope := wafv2types.ScopeRegional
		if acls[i].Scope == string(wafv2types.ScopeCloudfront) {
			scope = wafv2types.ScopeCloudfront
		}
		acl, read := readOne(func() (*wafv2types.WebACL, error) {
			out, err := wafClientFor(cfg, scope).GetWebACL(ctx, &wafv2.GetWebACLInput{
				Name:  aws.String(acls[i].Name),
				Id:    aws.String(acls[i].Id),
				Scope: scope,
			})
			if err != nil {
				return nil, err
			}
			return out.WebACL, nil
		})
		acls[i].Describe = read
		if acl != nil {
			acls[i].RuleCount = len(acl.Rules)
			acls[i].DefaultActionIsAllow = acl.DefaultAction != nil && acl.DefaultAction.Allow != nil
		}
	}

	return nil
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

// captureSSM lists every parameter (DescribeParameters, all pages). Every
// signal derives from ParameterMetadata on the list response.
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
				LastModifiedDate: snapFormatTime(p.LastModifiedDate),
			})
		}
	}

	return ssmData{Parameters: params}, nil
}
