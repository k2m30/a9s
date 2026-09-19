package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

func TestFetchCallerIdentity_AssumedRole(t *testing.T) {
	stsMock := &mockSTSGetCallerIdentityClient{
		output: &sts.GetCallerIdentityOutput{
			Arn:     aws.String("arn:aws:sts::123456789012:assumed-role/admin-role/session-name"),
			Account: aws.String("123456789012"),
			UserId:  aws.String("AROAWXZQ4F6JSPY5RBZ7F:session-name"),
		},
	}
	iamMock := &mockIAMListAccountAliasesClient{
		output: &iam.ListAccountAliasesOutput{
			AccountAliases: []string{"acme-prod"},
		},
	}

	result, err := awsclient.FetchCallerIdentity(context.Background(), stsMock, iamMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.AccountID != "123456789012" {
		t.Errorf("AccountID: expected %q, got %q", "123456789012", result.AccountID)
	}
	if result.AccountAlias != "acme-prod" {
		t.Errorf("AccountAlias: expected %q, got %q", "acme-prod", result.AccountAlias)
	}
	if result.RoleName != "admin-role" {
		t.Errorf("RoleName: expected %q, got %q", "admin-role", result.RoleName)
	}
	if result.SessionName != "session-name" {
		t.Errorf("SessionName: expected %q, got %q", "session-name", result.SessionName)
	}
	if !result.IsAssumedRole {
		t.Error("IsAssumedRole: expected true, got false")
	}
	if result.IdentityName != "admin-role" {
		t.Errorf("IdentityName: expected %q, got %q", "admin-role", result.IdentityName)
	}
}

func TestFetchCallerIdentity_IAMUser(t *testing.T) {
	stsMock := &mockSTSGetCallerIdentityClient{
		output: &sts.GetCallerIdentityOutput{
			Arn:     aws.String("arn:aws:iam::111222333444:user/deploy-bot@example.com"),
			Account: aws.String("111222333444"),
			UserId:  aws.String("AIDAWXZQ4F6JSPY5RBZ7F"),
		},
	}
	iamMock := &mockIAMListAccountAliasesClient{
		output: &iam.ListAccountAliasesOutput{
			AccountAliases: []string{},
		},
	}

	result, err := awsclient.FetchCallerIdentity(context.Background(), stsMock, iamMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.UserName != "deploy-bot@example.com" {
		t.Errorf("UserName: expected %q, got %q", "deploy-bot@example.com", result.UserName)
	}
	if result.IsAssumedRole {
		t.Error("IsAssumedRole: expected false, got true")
	}
	if result.IdentityName != "deploy-bot@example.com" {
		t.Errorf("IdentityName: expected %q, got %q", "deploy-bot@example.com", result.IdentityName)
	}
	if result.AccountAlias != "" {
		t.Errorf("AccountAlias: expected empty, got %q", result.AccountAlias)
	}
}

func TestFetchCallerIdentity_WithAlias(t *testing.T) {
	stsMock := &mockSTSGetCallerIdentityClient{
		output: &sts.GetCallerIdentityOutput{
			Arn:     aws.String("arn:aws:sts::123456789012:assumed-role/dev-role/mysession"),
			Account: aws.String("123456789012"),
			UserId:  aws.String("AROAWXZQ4F6JSPY5RBZ7F:mysession"),
		},
	}
	iamMock := &mockIAMListAccountAliasesClient{
		output: &iam.ListAccountAliasesOutput{
			AccountAliases: []string{"acme-prod"},
		},
	}

	result, err := awsclient.FetchCallerIdentity(context.Background(), stsMock, iamMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.AccountAlias != "acme-prod" {
		t.Errorf("AccountAlias: expected %q, got %q", "acme-prod", result.AccountAlias)
	}
}

func TestFetchCallerIdentity_NoAlias(t *testing.T) {
	stsMock := &mockSTSGetCallerIdentityClient{
		output: &sts.GetCallerIdentityOutput{
			Arn:     aws.String("arn:aws:sts::123456789012:assumed-role/dev-role/mysession"),
			Account: aws.String("123456789012"),
			UserId:  aws.String("AROAWXZQ4F6JSPY5RBZ7F:mysession"),
		},
	}
	iamMock := &mockIAMListAccountAliasesClient{
		output: &iam.ListAccountAliasesOutput{
			AccountAliases: []string{},
		},
	}

	result, err := awsclient.FetchCallerIdentity(context.Background(), stsMock, iamMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.AccountAlias != "" {
		t.Errorf("AccountAlias: expected empty, got %q", result.AccountAlias)
	}
}

// An IAM failure still returns the STS data and no error.

func TestFetchCallerIdentity_IAMAccessDenied(t *testing.T) {
	stsMock := &mockSTSGetCallerIdentityClient{
		output: &sts.GetCallerIdentityOutput{
			Arn:     aws.String("arn:aws:sts::123456789012:assumed-role/readonly-role/session"),
			Account: aws.String("123456789012"),
			UserId:  aws.String("AROAWXZQ4F6JSPY5RBZ7F:session"),
		},
	}
	iamMock := &mockIAMListAccountAliasesClient{
		err: fmt.Errorf("AccessDenied: User is not authorized to perform: iam:ListAccountAliases"),
	}

	result, err := awsclient.FetchCallerIdentity(context.Background(), stsMock, iamMock)
	if err != nil {
		t.Fatalf("expected no error when IAM fails, got %v", err)
	}

	if result.AccountID != "123456789012" {
		t.Errorf("AccountID should still be populated, got %q", result.AccountID)
	}
	if result.AccountAlias != "" {
		t.Errorf("AccountAlias should be empty when IAM fails, got %q", result.AccountAlias)
	}
	if result.RoleName != "readonly-role" {
		t.Errorf("RoleName should still be parsed, got %q", result.RoleName)
	}
}

func TestFetchCallerIdentity_STSError(t *testing.T) {
	stsMock := &mockSTSGetCallerIdentityClient{
		err: fmt.Errorf("ExpiredToken: security token has expired"),
	}
	iamMock := &mockIAMListAccountAliasesClient{
		output: &iam.ListAccountAliasesOutput{
			AccountAliases: []string{"acme-prod"},
		},
	}

	result, err := awsclient.FetchCallerIdentity(context.Background(), stsMock, iamMock)
	if err == nil {
		t.Fatal("expected error when STS fails, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result when STS fails, got %+v", result)
	}
}

func TestFetchCallerIdentity_FederatedUser(t *testing.T) {
	stsMock := &mockSTSGetCallerIdentityClient{
		output: &sts.GetCallerIdentityOutput{
			Arn:     aws.String("arn:aws:sts::123456789012:federated-user/admin"),
			Account: aws.String("123456789012"),
			UserId:  aws.String("123456789012:admin"),
		},
	}
	iamMock := &mockIAMListAccountAliasesClient{
		output: &iam.ListAccountAliasesOutput{
			AccountAliases: []string{},
		},
	}

	result, err := awsclient.FetchCallerIdentity(context.Background(), stsMock, iamMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.UserName != "admin" {
		t.Errorf("UserName: expected %q, got %q", "admin", result.UserName)
	}
	if result.IsAssumedRole {
		t.Error("IsAssumedRole: expected false for federated-user, got true")
	}
}
