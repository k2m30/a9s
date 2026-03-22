package unit

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// ===========================================================================
// YAML fixture builders — return []resource.Resource with Fields map populated
// ===========================================================================

func fixtureVPCs() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "vpc-0abc1234def56789a",
			Name:   "prod-vpc",
			Status: "available",
			Fields: map[string]string{
				"vpc_id":     "vpc-0abc1234def56789a",
				"cidr_block": "10.0.0.0/16",
				"state":      "available",
				"is_default": "No",
				"owner_id":   "123456789012",
			},
		},
	}
}

func fixtureSGs() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "sg-0abc1234def56789a",
			Name:   "web-sg",
			Status: "",
			Fields: map[string]string{
				"group_id":    "sg-0abc1234def56789a",
				"group_name":  "web-sg",
				"vpc_id":      "vpc-0abc1234",
				"description": "Web server security group",
				"owner_id":    "123456789012",
			},
		},
	}
}

func fixtureNGs() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "prod-ng-01",
			Name:   "prod-ng-01",
			Status: "ACTIVE",
			Fields: map[string]string{
				"nodegroup_name": "prod-ng-01",
				"cluster_name":   "prod-cluster",
				"status":         "ACTIVE",
				"instance_types": "t3.large,t3.xlarge",
				"desired_size":   "3",
			},
		},
	}
}

func fixtureSubnets() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "subnet-0abc1234def56789a",
			Name:   "public-subnet-1a",
			Status: "available",
			Fields: map[string]string{
				"subnet_id":         "subnet-0abc1234def56789a",
				"vpc_id":            "vpc-0abc1234",
				"cidr_block":        "10.0.1.0/24",
				"availability_zone": "us-east-1a",
				"state":             "available",
				"available_ips":     "251",
			},
		},
	}
}

func fixtureRTBs() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "rtb-0abc1234def56789a",
			Name:   "public-rtb",
			Status: "",
			Fields: map[string]string{
				"route_table_id": "rtb-0abc1234def56789a",
				"vpc_id":         "vpc-0abc1234",
				"routes_count":   "2",
			},
		},
	}
}

func fixtureNATs() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "nat-0abc1234def56789a",
			Name:   "prod-nat",
			Status: "available",
			Fields: map[string]string{
				"nat_gateway_id":    "nat-0abc1234def56789a",
				"vpc_id":            "vpc-0abc1234",
				"subnet_id":         "subnet-0abc1234",
				"state":             "available",
				"connectivity_type": "public",
				"public_ip":         "54.123.45.67",
			},
		},
	}
}

func fixtureIGWs() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "igw-0abc1234def56789a",
			Name:   "prod-igw",
			Status: "",
			Fields: map[string]string{
				"internet_gateway_id": "igw-0abc1234def56789a",
				"vpc_id":              "vpc-0abc1234",
				"state":               "attached",
				"owner_id":            "123456789012",
			},
		},
	}
}

func fixtureEIPs() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "eipalloc-0abc1234def56789a",
			Name:   "prod-eip",
			Status: "",
			Fields: map[string]string{
				"allocation_id":    "eipalloc-0abc1234def56789a",
				"public_ip":        "54.123.45.67",
				"association_id":   "eipassoc-0abc1234",
				"instance_id":      "i-0abc1234",
				"domain":           "vpc",
				"private_ip":       "10.0.1.42",
			},
		},
	}
}

func fixtureTGWs() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "tgw-0abc1234def56789a",
			Name:   "prod-tgw",
			Status: "available",
			Fields: map[string]string{
				"transit_gateway_id": "tgw-0abc1234def56789a",
				"state":              "available",
				"owner_id":           "123456789012",
				"description":        "Production transit gateway",
			},
		},
	}
}

func fixtureVPCEs() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "vpce-0abc1234def56789a",
			Name:   "s3-endpoint",
			Status: "available",
			Fields: map[string]string{
				"vpc_endpoint_id":   "vpce-0abc1234def56789a",
				"service_name":      "com.amazonaws.us-east-1.s3",
				"vpc_endpoint_type": "Gateway",
				"state":             "available",
				"vpc_id":            "vpc-0abc1234",
			},
		},
	}
}

func fixtureENIs() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "eni-0abc1234def56789a",
			Name:   "prod-eni",
			Status: "in-use",
			Fields: map[string]string{
				"network_interface_id": "eni-0abc1234def56789a",
				"status":              "in-use",
				"interface_type":      "interface",
				"vpc_id":              "vpc-0abc1234",
				"subnet_id":           "subnet-0abc1234",
				"private_ip":          "10.0.1.42",
				"mac_address":         "02:ab:cd:ef:12:34",
			},
		},
	}
}

func fixtureRDSSnaps() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "rds-snap-prod-20250615",
			Name:   "rds-snap-prod-20250615",
			Status: "available",
			Fields: map[string]string{
				"snapshot_id":   "rds-snap-prod-20250615",
				"db_instance":   "prod-db-01",
				"status":        "available",
				"engine":        "mysql",
				"engine_version": "8.0.35",
				"snapshot_type": "automated",
				"storage":       "100",
			},
		},
	}
}

func fixtureDocDBSnaps() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "docdb-snap-prod-20250615",
			Name:   "docdb-snap-prod-20250615",
			Status: "available",
			Fields: map[string]string{
				"snapshot_id":   "docdb-snap-prod-20250615",
				"cluster_id":    "docdb-prod-cluster",
				"status":        "available",
				"engine":        "docdb",
				"snapshot_type": "automated",
			},
		},
	}
}

func fixtureSNSSubs() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "arn:aws:sns:us-east-1:123456789012:alerts:a1b2c3d4",
			Name:   "alerts-sub",
			Status: "",
			Fields: map[string]string{
				"subscription_arn": "arn:aws:sns:us-east-1:123456789012:alerts:a1b2c3d4",
				"topic_arn":        "arn:aws:sns:us-east-1:123456789012:alerts",
				"protocol":         "email",
				"endpoint":         "user@example.com",
				"owner":            "123456789012",
			},
		},
	}
}

func fixturePolicies() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "ANPAI1234567890EXAMPLE",
			Name:   "ReadOnlyAccess",
			Status: "",
			Fields: map[string]string{
				"policy_name":      "ReadOnlyAccess",
				"policy_id":        "ANPAI1234567890EXAMPLE",
				"arn":              "arn:aws:iam::123456789012:policy/ReadOnlyAccess",
				"path":             "/",
				"attachment_count": "5",
				"description":      "Provides read-only access",
			},
		},
	}
}

// ===========================================================================
// 1. VPC — YAML Tests
// ===========================================================================

func TestQA_YAML_VPC_ViewContainsFields(t *testing.T) {
	vpcs := fixtureVPCs()
	for _, v := range vpcs {
		out := yamlView(t, v, 120, 40)
		for k, val := range v.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("VPC YAML for %q missing key %q", v.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("VPC YAML for %q missing value %q", v.ID, val)
			}
		}
	}
}

func TestQA_YAML_VPC_FrameTitle(t *testing.T) {
	vpcs := fixtureVPCs()
	m := yamlModel(vpcs[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("VPC FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_VPC_RawContentUncolored(t *testing.T) {
	vpcs := fixtureVPCs()
	m := yamlModel(vpcs[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("VPC RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 2. SG — YAML Tests
// ===========================================================================

func TestQA_YAML_SG_ViewContainsFields(t *testing.T) {
	sgs := fixtureSGs()
	for _, s := range sgs {
		out := yamlView(t, s, 120, 40)
		for k, val := range s.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("SG YAML for %q missing key %q", s.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("SG YAML for %q missing value %q", s.ID, val)
			}
		}
	}
}

func TestQA_YAML_SG_FrameTitle(t *testing.T) {
	sgs := fixtureSGs()
	m := yamlModel(sgs[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("SG FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_SG_RawContentUncolored(t *testing.T) {
	sgs := fixtureSGs()
	m := yamlModel(sgs[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("SG RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 3. NG — YAML Tests
// ===========================================================================

func TestQA_YAML_NG_ViewContainsFields(t *testing.T) {
	ngs := fixtureNGs()
	for _, n := range ngs {
		out := yamlView(t, n, 120, 40)
		for k, val := range n.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("NG YAML for %q missing key %q", n.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("NG YAML for %q missing value %q", n.ID, val)
			}
		}
	}
}

func TestQA_YAML_NG_FrameTitle(t *testing.T) {
	ngs := fixtureNGs()
	m := yamlModel(ngs[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("NG FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_NG_RawContentUncolored(t *testing.T) {
	ngs := fixtureNGs()
	m := yamlModel(ngs[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("NG RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 4. Subnet — YAML Tests
// ===========================================================================

func TestQA_YAML_Subnet_ViewContainsFields(t *testing.T) {
	subnets := fixtureSubnets()
	for _, s := range subnets {
		out := yamlView(t, s, 120, 40)
		for k, val := range s.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("Subnet YAML for %q missing key %q", s.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("Subnet YAML for %q missing value %q", s.ID, val)
			}
		}
	}
}

func TestQA_YAML_Subnet_FrameTitle(t *testing.T) {
	subnets := fixtureSubnets()
	m := yamlModel(subnets[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("Subnet FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_Subnet_RawContentUncolored(t *testing.T) {
	subnets := fixtureSubnets()
	m := yamlModel(subnets[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("Subnet RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 5. RTB — YAML Tests
// ===========================================================================

func TestQA_YAML_RTB_ViewContainsFields(t *testing.T) {
	rtbs := fixtureRTBs()
	for _, r := range rtbs {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("RTB YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("RTB YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_RTB_FrameTitle(t *testing.T) {
	rtbs := fixtureRTBs()
	m := yamlModel(rtbs[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("RTB FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_RTB_RawContentUncolored(t *testing.T) {
	rtbs := fixtureRTBs()
	m := yamlModel(rtbs[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("RTB RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 6. NAT — YAML Tests
// ===========================================================================

func TestQA_YAML_NAT_ViewContainsFields(t *testing.T) {
	nats := fixtureNATs()
	for _, n := range nats {
		out := yamlView(t, n, 120, 40)
		for k, val := range n.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("NAT YAML for %q missing key %q", n.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("NAT YAML for %q missing value %q", n.ID, val)
			}
		}
	}
}

func TestQA_YAML_NAT_FrameTitle(t *testing.T) {
	nats := fixtureNATs()
	m := yamlModel(nats[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("NAT FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_NAT_RawContentUncolored(t *testing.T) {
	nats := fixtureNATs()
	m := yamlModel(nats[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("NAT RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 7. IGW — YAML Tests
// ===========================================================================

func TestQA_YAML_IGW_ViewContainsFields(t *testing.T) {
	igws := fixtureIGWs()
	for _, i := range igws {
		out := yamlView(t, i, 120, 40)
		for k, val := range i.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("IGW YAML for %q missing key %q", i.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("IGW YAML for %q missing value %q", i.ID, val)
			}
		}
	}
}

func TestQA_YAML_IGW_FrameTitle(t *testing.T) {
	igws := fixtureIGWs()
	m := yamlModel(igws[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("IGW FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_IGW_RawContentUncolored(t *testing.T) {
	igws := fixtureIGWs()
	m := yamlModel(igws[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("IGW RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 8. EIP — YAML Tests
// ===========================================================================

func TestQA_YAML_EIP_ViewContainsFields(t *testing.T) {
	eips := fixtureEIPs()
	for _, e := range eips {
		out := yamlView(t, e, 120, 40)
		for k, val := range e.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("EIP YAML for %q missing key %q", e.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("EIP YAML for %q missing value %q", e.ID, val)
			}
		}
	}
}

func TestQA_YAML_EIP_FrameTitle(t *testing.T) {
	eips := fixtureEIPs()
	m := yamlModel(eips[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("EIP FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_EIP_RawContentUncolored(t *testing.T) {
	eips := fixtureEIPs()
	m := yamlModel(eips[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("EIP RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 9. TGW — YAML Tests
// ===========================================================================

func TestQA_YAML_TGW_ViewContainsFields(t *testing.T) {
	tgws := fixtureTGWs()
	for _, tg := range tgws {
		out := yamlView(t, tg, 120, 40)
		for k, val := range tg.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("TGW YAML for %q missing key %q", tg.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("TGW YAML for %q missing value %q", tg.ID, val)
			}
		}
	}
}

func TestQA_YAML_TGW_FrameTitle(t *testing.T) {
	tgws := fixtureTGWs()
	m := yamlModel(tgws[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("TGW FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_TGW_RawContentUncolored(t *testing.T) {
	tgws := fixtureTGWs()
	m := yamlModel(tgws[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("TGW RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 10. VPCE — YAML Tests
// ===========================================================================

func TestQA_YAML_VPCE_ViewContainsFields(t *testing.T) {
	vpces := fixtureVPCEs()
	for _, v := range vpces {
		out := yamlView(t, v, 120, 40)
		for k, val := range v.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("VPCE YAML for %q missing key %q", v.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("VPCE YAML for %q missing value %q", v.ID, val)
			}
		}
	}
}

func TestQA_YAML_VPCE_FrameTitle(t *testing.T) {
	vpces := fixtureVPCEs()
	m := yamlModel(vpces[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("VPCE FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_VPCE_RawContentUncolored(t *testing.T) {
	vpces := fixtureVPCEs()
	m := yamlModel(vpces[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("VPCE RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 11. ENI — YAML Tests
// ===========================================================================

func TestQA_YAML_ENI_ViewContainsFields(t *testing.T) {
	enis := fixtureENIs()
	for _, e := range enis {
		out := yamlView(t, e, 120, 40)
		for k, val := range e.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("ENI YAML for %q missing key %q", e.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("ENI YAML for %q missing value %q", e.ID, val)
			}
		}
	}
}

func TestQA_YAML_ENI_FrameTitle(t *testing.T) {
	enis := fixtureENIs()
	m := yamlModel(enis[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("ENI FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_ENI_RawContentUncolored(t *testing.T) {
	enis := fixtureENIs()
	m := yamlModel(enis[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("ENI RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 12. RDS Snapshot — YAML Tests
// ===========================================================================

func TestQA_YAML_RDSSnap_ViewContainsFields(t *testing.T) {
	snaps := fixtureRDSSnaps()
	for _, s := range snaps {
		out := yamlView(t, s, 120, 40)
		for k, val := range s.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("RDSSnap YAML for %q missing key %q", s.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("RDSSnap YAML for %q missing value %q", s.ID, val)
			}
		}
	}
}

func TestQA_YAML_RDSSnap_FrameTitle(t *testing.T) {
	snaps := fixtureRDSSnaps()
	m := yamlModel(snaps[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("RDSSnap FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_RDSSnap_RawContentUncolored(t *testing.T) {
	snaps := fixtureRDSSnaps()
	m := yamlModel(snaps[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("RDSSnap RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 13. DocDB Snapshot — YAML Tests
// ===========================================================================

func TestQA_YAML_DocDBSnap_ViewContainsFields(t *testing.T) {
	snaps := fixtureDocDBSnaps()
	for _, s := range snaps {
		out := yamlView(t, s, 120, 40)
		for k, val := range s.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("DocDBSnap YAML for %q missing key %q", s.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("DocDBSnap YAML for %q missing value %q", s.ID, val)
			}
		}
	}
}

func TestQA_YAML_DocDBSnap_FrameTitle(t *testing.T) {
	snaps := fixtureDocDBSnaps()
	m := yamlModel(snaps[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("DocDBSnap FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_DocDBSnap_RawContentUncolored(t *testing.T) {
	snaps := fixtureDocDBSnaps()
	m := yamlModel(snaps[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("DocDBSnap RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 14. SNS Subscription — YAML Tests
// ===========================================================================

func TestQA_YAML_SNSSub_ViewContainsFields(t *testing.T) {
	subs := fixtureSNSSubs()
	for _, s := range subs {
		out := yamlView(t, s, 120, 40)
		for k, val := range s.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("SNSSub YAML for %q missing key %q", s.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("SNSSub YAML for %q missing value %q", s.ID, val)
			}
		}
	}
}

func TestQA_YAML_SNSSub_FrameTitle(t *testing.T) {
	subs := fixtureSNSSubs()
	m := yamlModel(subs[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("SNSSub FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_SNSSub_RawContentUncolored(t *testing.T) {
	subs := fixtureSNSSubs()
	m := yamlModel(subs[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("SNSSub RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// 15. Policy — YAML Tests
// ===========================================================================

func TestQA_YAML_Policy_ViewContainsFields(t *testing.T) {
	policies := fixturePolicies()
	for _, p := range policies {
		out := yamlView(t, p, 120, 40)
		for k, val := range p.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("Policy YAML for %q missing key %q", p.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("Policy YAML for %q missing value %q", p.ID, val)
			}
		}
	}
}

func TestQA_YAML_Policy_FrameTitle(t *testing.T) {
	policies := fixturePolicies()
	m := yamlModel(policies[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("Policy FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_Policy_RawContentUncolored(t *testing.T) {
	policies := fixturePolicies()
	m := yamlModel(policies[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("Policy RawContent() contains ANSI codes, expected plain YAML")
	}
}
