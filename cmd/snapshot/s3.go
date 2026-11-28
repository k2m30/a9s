package main

import (
	"cmp"
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/s3control"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// s3Data is the normalized, raw view of S3 buckets the checklist generator reads. It records
// AWS facts only (never a9s's classification): the checklist generator applies the
// docs/resources/s3.md rules to derive expected colors/glyphs/status.
type s3Data struct {
	Buckets []s3Bucket `json:"buckets"`
	// AccountPAB is the account-level block (S3 Control GetPublicAccessBlock),
	// read when a bucket's own block leaves a flag off, as the app reads it.
	AccountPAB *s3PAB `json:"account_pab,omitempty"`
}

type s3Bucket struct {
	Name          string          `json:"name"`
	Region        string          `json:"region,omitempty"`
	CreationDate  string          `json:"creation_date,omitempty"`
	Location      s3Location      `json:"location"`
	Notifications s3Notifications `json:"notifications"`
	PAB           s3PAB           `json:"pab"`
	PolicyStatus  s3PolicyStatus  `json:"policy_status"`
	ACL           s3ACL           `json:"acl"`
	Versioning    s3Versioning    `json:"versioning"`
	Logging       s3Logging       `json:"logging"`
	Lifecycle     s3Lifecycle     `json:"lifecycle"`
	ObjectLock    s3ObjectLock    `json:"object_lock"`
}

// GetBucketLocation names us-east-1 by a null constraint and eu-west-1 by EU.
type s3Location struct {
	readOutcome
	Region string `json:"region,omitempty"`
}

// s3PAB is the GetPublicAccessBlock answer: Outcome "configured" when a block
// was returned, and only then are the four flags read.
type s3PAB struct {
	Outcome               string `json:"outcome"`
	ErrorCode             string `json:"error_code,omitempty"`
	BlockPublicAcls       bool   `json:"block_public_acls,omitempty"`
	IgnorePublicAcls      bool   `json:"ignore_public_acls,omitempty"`
	BlockPublicPolicy     bool   `json:"block_public_policy,omitempty"`
	RestrictPublicBuckets bool   `json:"restrict_public_buckets,omitempty"`
}

type s3PolicyStatus struct {
	readOutcome
	IsPublic bool `json:"is_public,omitempty"`
}

type s3ACL struct {
	readOutcome
	GroupGrants []s3GroupGrant `json:"group_grants,omitempty"`
}

type s3GroupGrant struct {
	URI        string `json:"uri"`
	Permission string `json:"permission"`
}

type s3Versioning struct {
	readOutcome
	Status    string `json:"status,omitempty"`
	MFADelete string `json:"mfa_delete,omitempty"`
}

type s3Logging struct {
	readOutcome
	Enabled      bool   `json:"enabled,omitempty"`
	TargetBucket string `json:"target_bucket,omitempty"`
}

type s3Lifecycle struct {
	readOutcome
	EnabledRules int `json:"enabled_rules,omitempty"`
}

type s3ObjectLock struct {
	readOutcome
	Enabled bool `json:"enabled,omitempty"`
}

// captureS3 lists every bucket (ListBuckets, all pages) and reads each one's
// posture from the bucket's own region: S3 answers a per-bucket read sent to
// another region with PermanentRedirect or IllegalLocationConstraintException.
// All buckets are captured; display caps belong to the checklist generator.
func captureS3(ctx context.Context, cfg aws.Config) (any, error) {
	client := s3.NewFromConfig(cfg)

	var buckets []s3Bucket
	pager := s3.NewListBucketsPaginator(client, &s3.ListBucketsInput{})
	for pager.HasMorePages() {
		out, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, b := range out.Buckets {
			buckets = append(buckets, s3Bucket{
				Name:         aws.ToString(b.Name),
				Region:       aws.ToString(b.BucketRegion),
				CreationDate: snapFormatTime(b.CreationDate),
			})
		}
	}

	clients := map[string]*s3.Client{cfg.Region: client}
	in := func(region string) *s3.Client {
		if clients[region] == nil {
			regional := cfg.Copy()
			regional.Region = region
			clients[region] = s3.NewFromConfig(regional)
		}
		return clients[region]
	}
	data := s3Data{Buckets: buckets}
	for i := range buckets {
		b := &buckets[i]
		b.Location = captureS3Location(ctx, in(cmp.Or(b.Region, cfg.Region)), b.Name)
		captureS3Posture(ctx, in(cmp.Or(b.Location.Region, b.Region, cfg.Region)), b)
		if data.AccountPAB == nil && !b.PAB.closesAll() {
			account := captureS3AccountPAB(ctx, cfg)
			data.AccountPAB = &account
		}
	}

	return data, nil
}

// closesAll reports whether the bucket's own block was read and sets all four
// flags; otherwise the account's block decides part of the bucket's.
func (p s3PAB) closesAll() bool {
	return p.Outcome == "configured" && p.BlockPublicAcls && p.IgnorePublicAcls && p.BlockPublicPolicy && p.RestrictPublicBuckets
}

func captureS3AccountPAB(ctx context.Context, cfg aws.Config) s3PAB {
	id, err := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		o, code := outcomeOf(err)
		return s3PAB{Outcome: o, ErrorCode: code}
	}
	out, err := s3control.NewFromConfig(cfg).GetPublicAccessBlock(ctx, &s3control.GetPublicAccessBlockInput{AccountId: id.Account})
	if err != nil {
		o, code := outcomeOf(err)
		return s3PAB{Outcome: o, ErrorCode: code}
	}
	c := out.PublicAccessBlockConfiguration
	if c == nil {
		return s3PAB{Outcome: outcomeNone}
	}
	return s3PAB{
		Outcome:               "configured",
		BlockPublicAcls:       aws.ToBool(c.BlockPublicAcls),
		IgnorePublicAcls:      aws.ToBool(c.IgnorePublicAcls),
		BlockPublicPolicy:     aws.ToBool(c.BlockPublicPolicy),
		RestrictPublicBuckets: aws.ToBool(c.RestrictPublicBuckets),
	}
}

func captureS3Location(ctx context.Context, client *s3.Client, bucket string) s3Location {
	out, err := client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{Bucket: aws.String(bucket)})
	loc := s3Location{readOutcome: readOf(err)}
	if out == nil {
		return loc
	}
	switch loc.Region = string(out.LocationConstraint); loc.Region {
	case "":
		loc.Region = "us-east-1"
	case string(s3types.BucketLocationConstraintEu):
		loc.Region = "eu-west-1"
	}
	return loc
}

func captureS3Posture(ctx context.Context, client *s3.Client, b *s3Bucket) {
	bucket := aws.String(b.Name)
	b.Notifications = captureS3Notifications(ctx, client, b.Name)
	b.PAB = capturePAB(ctx, client, b.Name)

	status, err := client.GetBucketPolicyStatus(ctx, &s3.GetBucketPolicyStatusInput{Bucket: bucket})
	b.PolicyStatus.readOutcome = readOf(err)
	if status != nil && status.PolicyStatus != nil {
		b.PolicyStatus.IsPublic = aws.ToBool(status.PolicyStatus.IsPublic)
	}

	acl, err := client.GetBucketAcl(ctx, &s3.GetBucketAclInput{Bucket: bucket})
	b.ACL.readOutcome = readOf(err)
	if acl != nil {
		for _, g := range acl.Grants {
			if g.Grantee != nil && g.Grantee.Type == s3types.TypeGroup {
				b.ACL.GroupGrants = append(b.ACL.GroupGrants, s3GroupGrant{URI: aws.ToString(g.Grantee.URI), Permission: string(g.Permission)})
			}
		}
	}

	versioning, err := client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{Bucket: bucket})
	b.Versioning.readOutcome = readOf(err)
	if versioning != nil {
		b.Versioning.Status = string(versioning.Status)
		b.Versioning.MFADelete = string(versioning.MFADelete)
	}

	logging, err := client.GetBucketLogging(ctx, &s3.GetBucketLoggingInput{Bucket: bucket})
	b.Logging.readOutcome = readOf(err)
	if logging != nil && logging.LoggingEnabled != nil {
		b.Logging.Enabled = true
		b.Logging.TargetBucket = aws.ToString(logging.LoggingEnabled.TargetBucket)
	}

	lifecycle, err := client.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{Bucket: bucket})
	b.Lifecycle.readOutcome = readOf(err)
	if lifecycle != nil {
		for _, rule := range lifecycle.Rules {
			if rule.Status == s3types.ExpirationStatusEnabled {
				b.Lifecycle.EnabledRules++
			}
		}
	}

	lock, err := client.GetObjectLockConfiguration(ctx, &s3.GetObjectLockConfigurationInput{Bucket: bucket})
	b.ObjectLock.readOutcome = readOf(err)
	b.ObjectLock.Enabled = lock != nil && lock.ObjectLockConfiguration != nil &&
		lock.ObjectLockConfiguration.ObjectLockEnabled == s3types.ObjectLockEnabledEnabled
}

type s3Notifications struct {
	readOutcome
	LambdaFunctionArns []string `json:"lambda_function_arns,omitempty"`
	QueueArns          []string `json:"queue_arns,omitempty"`
	TopicArns          []string `json:"topic_arns,omitempty"`
}

func captureS3Notifications(ctx context.Context, client *s3.Client, bucket string) s3Notifications {
	out, err := client.GetBucketNotificationConfiguration(ctx, &s3.GetBucketNotificationConfigurationInput{Bucket: aws.String(bucket)})
	n := s3Notifications{readOutcome: readOf(err)}
	if out == nil {
		return n
	}
	for _, c := range out.LambdaFunctionConfigurations {
		n.LambdaFunctionArns = append(n.LambdaFunctionArns, aws.ToString(c.LambdaFunctionArn))
	}
	for _, c := range out.QueueConfigurations {
		n.QueueArns = append(n.QueueArns, aws.ToString(c.QueueArn))
	}
	for _, c := range out.TopicConfigurations {
		n.TopicArns = append(n.TopicArns, aws.ToString(c.TopicArn))
	}
	return n
}

func capturePAB(ctx context.Context, client *s3.Client, bucket string) s3PAB {
	out, err := client.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{Bucket: aws.String(bucket)})
	if err != nil {
		o, code := outcomeOf(err)
		return s3PAB{Outcome: o, ErrorCode: code}
	}
	cfg := out.PublicAccessBlockConfiguration
	if cfg == nil {
		return s3PAB{Outcome: outcomeNone}
	}
	return s3PAB{
		Outcome:               "configured",
		BlockPublicAcls:       aws.ToBool(cfg.BlockPublicAcls),
		IgnorePublicAcls:      aws.ToBool(cfg.IgnorePublicAcls),
		BlockPublicPolicy:     aws.ToBool(cfg.BlockPublicPolicy),
		RestrictPublicBuckets: aws.ToBool(cfg.RestrictPublicBuckets),
	}
}

// snapFormatTime is the single formatTime implementation shared by every
// capture file in this package (each AWS SDK service response uses the same
// *time.Time -> "2006-01-02 15:04" convention).
func snapFormatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02 15:04")
}
