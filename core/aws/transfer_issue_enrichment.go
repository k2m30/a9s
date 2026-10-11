// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// transfer_issue_enrichment.go — Wave 2 issue enrichment for the transfer
// resource type.
package aws

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/transfer"
	transfertypes "github.com/aws/aws-sdk-go-v2/service/transfer/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// EnrichTransferSecurityPolicy judges each server's security policy by what
// DescribeSecurityPolicy says it allows, never by its name: a policy is weak
// when it offers a SHA-1 SSH key exchange, a SHA-1 or 64-bit-tag SSH MAC, or
// a TLS cipher suite without forward secrecy (transferPolicyWeakRows). Each
// policy name is read once per run, when the first server naming it is
// judged. A failed read leaves the servers naming that policy not inspected;
// a client without the call reads nothing.
func EnrichTransferSecurityPolicy(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]string),
	}
	if clients == nil || clients.Transfer == nil {
		return result, nil
	}
	api, ok := clients.Transfer.(TransferDescribeSecurityPolicyAPI)
	if !ok {
		return result, nil
	}
	var readsMu sync.Mutex
	reads := map[string]func() (*transfertypes.DescribedSecurityPolicy, error){}
	policy := func(name string) (*transfertypes.DescribedSecurityPolicy, error) {
		readsMu.Lock()
		read, ok := reads[name]
		if !ok {
			read = sync.OnceValues(func() (*transfertypes.DescribedSecurityPolicy, error) {
				out, err := api.DescribeSecurityPolicy(ctx, &transfer.DescribeSecurityPolicyInput{SecurityPolicyName: aws.String(name)})
				if err != nil {
					return nil, err
				}
				if out.SecurityPolicy == nil {
					return nil, UnusableAnswerErr{Call: "DescribeSecurityPolicy", Field: "security policy"}
				}
				return out.SecurityPolicy, nil
			})
			reads[name] = read
		}
		readsMu.Unlock()
		return read()
	}
	resources = capAtEnrichmentCap(&result, resources, nil, resourceIDsOf)
	var failures []Failure
	var mu sync.Mutex
	loopErr := ForEachRow(ctx, &result, resourceIDs(resources), EnrichmentParallelism, func(i int) {
		r := resources[i]
		server, ok := assertStruct[transfertypes.DescribedServer](r.RawStruct)
		name := aws.ToString(server.SecurityPolicyName)
		if !ok || name == "" {
			return
		}
		described, err := policy(name)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			MarkSkipped(&result, r.ID, &failures, err)
			return
		}
		if rows := transferPolicyWeakRows(described); len(rows) > 0 {
			setWave2Finding(&result, r.ID, transferCodeLegacyPolicy, append([]domain.DetailRow{{Label: "Security policy", Value: name, Tier: tierOf(transferCodeLegacyPolicy)}}, rows...))
		}
	})
	return result, errors.Join(loopErr, AggregateFailures("DescribeSecurityPolicy", failures, len(resources)))
}

// transferPolicyWeakRows names what a security policy allows that is weak: a
// SHA-1 SSH key exchange (diffie-hellman-group14-sha1), a SHA-1 SSH MAC or one
// with a 64-bit tag (hmac-sha1, umac-64), and a TLS cipher suite with RSA key
// exchange, which has no forward secrecy. It returns nil for a policy that
// allows none of them.
func transferPolicyWeakRows(p *transfertypes.DescribedSecurityPolicy) []domain.DetailRow {
	tier := tierOf(transferCodeLegacyPolicy)
	var rows []domain.DetailRow
	add := func(label string, offered []string, weak func(string) bool) {
		var hits []string
		for _, a := range offered {
			if weak(a) {
				hits = append(hits, a)
			}
		}
		if len(hits) > 0 {
			rows = append(rows, domain.DetailRow{Label: label, Value: strings.Join(hits, ", "), Tier: tier})
		}
	}
	add("Weak key exchange", p.SshKexs, func(k string) bool { return strings.HasSuffix(k, "-sha1") })
	add("Weak message checks", p.SshMacs, func(m string) bool {
		return strings.HasPrefix(m, "hmac-sha1") || strings.HasPrefix(m, "umac-64")
	})
	add("Ciphers without forward secrecy", p.TlsCiphers, func(c string) bool { return strings.HasPrefix(c, "TLS_RSA_") })
	return rows
}
