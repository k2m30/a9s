// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fixtures

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

// ELBSSLPolicies is the predefined security policy list
// elasticloadbalancingv2:DescribeSSLPolicies answers, with the protocols AWS
// states for each policy. Ciphers are omitted: no demo surface reads them.
var ELBSSLPolicies = []elbv2types.SslPolicy{
	{Name: aws.String("ELBSecurityPolicy-2015-05"), SslProtocols: []string{"TLSv1", "TLSv1.1", "TLSv1.2"}},
	{Name: aws.String("ELBSecurityPolicy-2016-08"), SslProtocols: []string{"TLSv1", "TLSv1.1", "TLSv1.2"}},
	{Name: aws.String("ELBSecurityPolicy-FS-1-1-2019-08"), SslProtocols: []string{"TLSv1.1", "TLSv1.2"}},
	{Name: aws.String("ELBSecurityPolicy-FS-1-2-2019-08"), SslProtocols: []string{"TLSv1.2"}},
	{Name: aws.String("ELBSecurityPolicy-FS-1-2-Res-2019-08"), SslProtocols: []string{"TLSv1.2"}},
	{Name: aws.String("ELBSecurityPolicy-FS-1-2-Res-2020-10"), SslProtocols: []string{"TLSv1.2"}},
	{Name: aws.String("ELBSecurityPolicy-FS-2018-06"), SslProtocols: []string{"TLSv1", "TLSv1.1", "TLSv1.2"}},
	{Name: aws.String("ELBSecurityPolicy-TLS-1-0-2015-04"), SslProtocols: []string{"TLSv1", "TLSv1.1", "TLSv1.2"}},
	{Name: aws.String("ELBSecurityPolicy-TLS-1-1-2017-01"), SslProtocols: []string{"TLSv1.1", "TLSv1.2"}},
	{Name: aws.String("ELBSecurityPolicy-TLS-1-2-2017-01"), SslProtocols: []string{"TLSv1.2"}},
	{Name: aws.String("ELBSecurityPolicy-TLS-1-2-Ext-2018-06"), SslProtocols: []string{"TLSv1.2"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-0-2021-06"), SslProtocols: []string{"TLSv1", "TLSv1.1", "TLSv1.2", "TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-0-FIPS-2023-04"), SslProtocols: []string{"TLSv1", "TLSv1.1", "TLSv1.2", "TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-0-FIPS-PQ-2025-09"), SslProtocols: []string{"TLSv1", "TLSv1.1", "TLSv1.2", "TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-0-PQ-2025-09"), SslProtocols: []string{"TLSv1", "TLSv1.1", "TLSv1.2", "TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-1-2021-06"), SslProtocols: []string{"TLSv1.1", "TLSv1.2", "TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-1-FIPS-2023-04"), SslProtocols: []string{"TLSv1.1", "TLSv1.2", "TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-2-2021-06"), SslProtocols: []string{"TLSv1.2", "TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-2-Ext0-FIPS-2023-04"), SslProtocols: []string{"TLSv1.2", "TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-2-Ext0-FIPS-PQ-2025-09"), SslProtocols: []string{"TLSv1.2", "TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-2-Ext0-RFC9151-FIPS-2023-07"), SslProtocols: []string{"TLSv1.2", "TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-2-Ext1-2021-06"), SslProtocols: []string{"TLSv1.2", "TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-2-Ext1-FIPS-2023-04"), SslProtocols: []string{"TLSv1.2", "TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-2-Ext1-FIPS-PQ-2025-09"), SslProtocols: []string{"TLSv1.2", "TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-2-Ext1-PQ-2025-09"), SslProtocols: []string{"TLSv1.2", "TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-2-Ext2-2021-06"), SslProtocols: []string{"TLSv1.2", "TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-2-Ext2-FIPS-2023-04"), SslProtocols: []string{"TLSv1.2", "TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-2-Ext2-FIPS-PQ-2025-09"), SslProtocols: []string{"TLSv1.2", "TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-2-Ext2-PQ-2025-09"), SslProtocols: []string{"TLSv1.2", "TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-2-FIPS-2023-04"), SslProtocols: []string{"TLSv1.2", "TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-2-FIPS-PQ-2025-09"), SslProtocols: []string{"TLSv1.2", "TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-2-PQ-2025-09"), SslProtocols: []string{"TLSv1.2", "TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-2-RFC9151-FIPS-2023-07"), SslProtocols: []string{"TLSv1.2", "TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-2-RFC9151-INTEROP1-FIPS-2023-07"), SslProtocols: []string{"TLSv1.2", "TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-2-RFC9151-INTEROP2-FIPS-2023-07"), SslProtocols: []string{"TLSv1.2", "TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-2-RFC9151-INTEROP3-FIPS-2023-07"), SslProtocols: []string{"TLSv1.2", "TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-2-RFC9151-INTEROP4-FIPS-2023-07"), SslProtocols: []string{"TLSv1.2", "TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-2-Res-2021-06"), SslProtocols: []string{"TLSv1.2", "TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-2-Res-FIPS-2023-04"), SslProtocols: []string{"TLSv1.2", "TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-2-Res-FIPS-PQ-2025-09"), SslProtocols: []string{"TLSv1.2", "TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-2-Res-PQ-2025-09"), SslProtocols: []string{"TLSv1.2", "TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-3-2021-06"), SslProtocols: []string{"TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-3-FIPS-2023-04"), SslProtocols: []string{"TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-3-FIPS-PQ-2025-09"), SslProtocols: []string{"TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-3-PQ-2025-09"), SslProtocols: []string{"TLSv1.3"}},
	{Name: aws.String("ELBSecurityPolicy-TLS13-1-3-RFC9151-FIPS-2023-07"), SslProtocols: []string{"TLSv1.3"}},
}
