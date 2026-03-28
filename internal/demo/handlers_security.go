package demo

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"
)

// registerSecurityHandlers registers IAM (users, groups, policies), WAF, and ACM handlers.
func registerSecurityHandlers(t *Transport) {
	registerIAMExtHandlers(t)
	registerWAFHandlers(t)
	registerACMHandlers(t)
}

// ---------------------------------------------------------------------------
// IAM extended handlers (awsquery — XML)
// ---------------------------------------------------------------------------

func registerIAMExtHandlers(t *Transport) {
	t.Handle("iam", "ListUsers", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["iam-user"]()
		users := ExtractSDK[iamtypes.User](resources)

		var sb strings.Builder
		sb.WriteString(`<IsTruncated>false</IsTruncated>`)
		sb.WriteString(`<Users>`)
		for _, u := range users {
			userName := aws.ToString(u.UserName)
			userID := aws.ToString(u.UserId)
			arn := aws.ToString(u.Arn)
			path := aws.ToString(u.Path)
			createDate := ""
			if u.CreateDate != nil {
				createDate = u.CreateDate.UTC().Format(time.RFC3339)
			}
			passLastUsed := ""
			if u.PasswordLastUsed != nil {
				passLastUsed = u.PasswordLastUsed.UTC().Format(time.RFC3339)
			}

			fmt.Fprintf(&sb, `<member>`)
			fmt.Fprintf(&sb, `<UserName>%s</UserName>`, xmlEscape(userName))
			fmt.Fprintf(&sb, `<UserId>%s</UserId>`, xmlEscape(userID))
			fmt.Fprintf(&sb, `<Arn>%s</Arn>`, xmlEscape(arn))
			fmt.Fprintf(&sb, `<Path>%s</Path>`, xmlEscape(path))
			if createDate != "" {
				fmt.Fprintf(&sb, `<CreateDate>%s</CreateDate>`, createDate)
			}
			if passLastUsed != "" {
				fmt.Fprintf(&sb, `<PasswordLastUsed>%s</PasswordLastUsed>`, passLastUsed)
			}
			fmt.Fprintf(&sb, `</member>`)
		}
		sb.WriteString(`</Users>`)

		body := awsQueryXML("ListUsers", "https://iam.amazonaws.com/doc/2010-05-08/", sb.String())
		return XMLResponse(body), nil
	})

	t.Handle("iam", "ListGroups", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["iam-group"]()
		groups := ExtractSDK[iamtypes.Group](resources)

		var sb strings.Builder
		sb.WriteString(`<IsTruncated>false</IsTruncated>`)
		sb.WriteString(`<Groups>`)
		for _, g := range groups {
			groupName := aws.ToString(g.GroupName)
			groupID := aws.ToString(g.GroupId)
			arn := aws.ToString(g.Arn)
			path := aws.ToString(g.Path)
			createDate := ""
			if g.CreateDate != nil {
				createDate = g.CreateDate.UTC().Format(time.RFC3339)
			}

			fmt.Fprintf(&sb, `<member>`)
			fmt.Fprintf(&sb, `<GroupName>%s</GroupName>`, xmlEscape(groupName))
			fmt.Fprintf(&sb, `<GroupId>%s</GroupId>`, xmlEscape(groupID))
			fmt.Fprintf(&sb, `<Arn>%s</Arn>`, xmlEscape(arn))
			fmt.Fprintf(&sb, `<Path>%s</Path>`, xmlEscape(path))
			if createDate != "" {
				fmt.Fprintf(&sb, `<CreateDate>%s</CreateDate>`, createDate)
			}
			fmt.Fprintf(&sb, `</member>`)
		}
		sb.WriteString(`</Groups>`)

		body := awsQueryXML("ListGroups", "https://iam.amazonaws.com/doc/2010-05-08/", sb.String())
		return XMLResponse(body), nil
	})

	t.Handle("iam", "ListPolicies", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["policy"]()
		policies := ExtractSDK[iamtypes.Policy](resources)

		var sb strings.Builder
		sb.WriteString(`<IsTruncated>false</IsTruncated>`)
		sb.WriteString(`<Policies>`)
		for _, p := range policies {
			policyName := aws.ToString(p.PolicyName)
			policyID := aws.ToString(p.PolicyId)
			arn := aws.ToString(p.Arn)
			path := aws.ToString(p.Path)
			attachCount := int32(0)
			if p.AttachmentCount != nil {
				attachCount = *p.AttachmentCount
			}
			createDate := ""
			if p.CreateDate != nil {
				createDate = p.CreateDate.UTC().Format(time.RFC3339)
			}

			fmt.Fprintf(&sb, `<member>`)
			fmt.Fprintf(&sb, `<PolicyName>%s</PolicyName>`, xmlEscape(policyName))
			fmt.Fprintf(&sb, `<PolicyId>%s</PolicyId>`, xmlEscape(policyID))
			fmt.Fprintf(&sb, `<Arn>%s</Arn>`, xmlEscape(arn))
			fmt.Fprintf(&sb, `<Path>%s</Path>`, xmlEscape(path))
			fmt.Fprintf(&sb, `<AttachmentCount>%d</AttachmentCount>`, attachCount)
			if createDate != "" {
				fmt.Fprintf(&sb, `<CreateDate>%s</CreateDate>`, createDate)
			}
			fmt.Fprintf(&sb, `</member>`)
		}
		sb.WriteString(`</Policies>`)

		body := awsQueryXML("ListPolicies", "https://iam.amazonaws.com/doc/2010-05-08/", sb.String())
		return XMLResponse(body), nil
	})

	t.Handle("iam", "ListAccountAliases", func(_ *http.Request) (*http.Response, error) {
		sb := `<IsTruncated>false</IsTruncated><AccountAliases><member>demo-account</member></AccountAliases>`
		body := awsQueryXML("ListAccountAliases", "https://iam.amazonaws.com/doc/2010-05-08/", sb)
		return XMLResponse(body), nil
	})
}

// ---------------------------------------------------------------------------
// WAF (awsjson11 — X-Amz-Target routing)
// ---------------------------------------------------------------------------

func registerWAFHandlers(t *Transport) {
	t.Handle("wafv2", "ListWebACLs", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["waf"]()
		acls := ExtractSDK[wafv2types.WebACLSummary](resources)

		out := &wafv2.ListWebACLsOutput{
			WebACLs: acls,
		}
		return JSONResponse(out)
	})
}

// ---------------------------------------------------------------------------
// ACM (awsjson11 — X-Amz-Target routing)
// ---------------------------------------------------------------------------

func registerACMHandlers(t *Transport) {
	t.Handle("acm", "ListCertificates", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["acm"]()
		certs := ExtractSDK[acmtypes.CertificateSummary](resources)

		out := &acm.ListCertificatesOutput{
			CertificateSummaryList: certs,
		}
		return JSONResponse(out)
	})
}
