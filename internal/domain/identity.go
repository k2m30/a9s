// Package domain — see contracts.go for the package overview.
//
// identity.go owns the platform-agnostic mirror of the AWS caller-identity
// record. The on-the-wire fetch type lives in internal/aws (which Bubble
// Tea cannot depend on after Phase-05 5a-extract); the runtime converts
// *awsclient.CallerIdentity → *domain.CallerIdentity at the runtime/aws
// boundary and emits the mirror via SetIdentityIntent so the adapter has a
// renderer-shaped value to apply to views without importing internal/aws.
package domain

// CallerIdentity is the domain mirror of the AWS caller identity. Pure data;
// the field set is the subset of awsclient.CallerIdentity that adapters
// render (account badge, header role, identity panel rows). UserID is
// intentionally omitted — it is only used by aws/identity.parseARN and
// never read at the renderer boundary.
type CallerIdentity struct {
	AccountID     string
	AccountAlias  string
	Arn           string
	RoleName      string
	UserName      string
	SessionName   string
	IdentityName  string
	IsAssumedRole bool
}
