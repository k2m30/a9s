package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	CodeIGWStateAttaching domain.FindingCode = "igw.state.attaching"
	CodeIGWStateDetaching domain.FindingCode = "igw.state.detaching"
	CodeIGWNoAttachments  domain.FindingCode = "igw.no-attachments"
)
