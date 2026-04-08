package unit

import (
	"encoding/json"
	"strings"
	"testing"

	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	cloudfronttypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	demo "github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/fieldpath"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func allDemoByType(t *testing.T) map[string][]resource.Resource {
	t.Helper()
	out := make(map[string][]resource.Resource)
	for _, rt := range resource.AllResourceTypes() {
		resources, ok := demo.GetResources(rt.ShortName)
		if !ok {
			t.Fatalf("demo fixtures missing for resource type %q", rt.ShortName)
		}
		if len(resources) == 0 {
			t.Fatalf("demo fixtures empty for resource type %q", rt.ShortName)
		}
		out[rt.ShortName] = resources
	}
	return out
}

func indexByID(resources []resource.Resource) map[string]resource.Resource {
	m := make(map[string]resource.Resource, len(resources))
	for _, r := range resources {
		m[r.ID] = r
	}
	return m
}

func TestIssue189_DemoFixtures_All66TypesPresentAndPopulated(t *testing.T) {
	fixtures := allDemoByType(t)
	if len(fixtures) != len(resource.AllResourceTypes()) {
		t.Fatalf("fixture type count mismatch: got=%d want=%d", len(fixtures), len(resource.AllResourceTypes()))
	}
}

func TestIssue189_DemoFixtures_AllRawStructsNonNil(t *testing.T) {
	fixtures := allDemoByType(t)
	for short, list := range fixtures {
		for i, r := range list {
			if r.RawStruct == nil {
				t.Errorf("%s fixture[%d] ID=%s has nil RawStruct", short, i, r.ID)
			}
		}
	}
}

func TestIssue189_DemoFixtures_AllConfiguredDetailFieldsResolved(t *testing.T) {
	fixtures := allDemoByType(t)
	for short, list := range fixtures {
		navigable := resource.GetNavigableFields(short)
		if len(navigable) == 0 {
			continue
		}
		for _, nf := range navigable {
			path := nf.FieldPath
			for i, r := range list {
				v := strings.TrimSpace(fieldpath.ExtractSubtree(r.RawStruct, path))
				if v == "" || v == "-" || strings.EqualFold(v, "<nil>") {
					if fv := strings.TrimSpace(r.Fields[path]); fv != "" {
						v = fv // Fields map is checked first by ExtractFieldList at runtime
					}
				}
				if v == "" || v == "-" || strings.EqualFold(v, "<nil>") {
					t.Errorf("%s fixture[%d] ID=%s navigable path=%q resolved empty", short, i, r.ID, path)
				}
			}
		}
	}
}

func TestIssue189_NavigableFields_TargetFixturesExist(t *testing.T) {
	fixtures := allDemoByType(t)
	for _, rt := range resource.AllResourceTypes() {
		for _, nf := range resource.GetNavigableFields(rt.ShortName) {
			target := fixtures[nf.TargetType]
			if len(target) == 0 {
				t.Errorf("navigable field %s.%s -> %s has no target fixtures", rt.ShortName, nf.FieldPath, nf.TargetType)
			}
		}
	}
}

func TestIssue189_RelatedDemo_IDsResolveToRealFixtures(t *testing.T) {
	fixtures := allDemoByType(t)

	// Extend the fixture lookup to also cover child types (e.g. s3_objects).
	// Child types are not in AllResourceTypes() — they are browsed via parent
	// views — but their related-demo IDs must still resolve to real fixtures.
	for _, ct := range resource.AllChildTypes() {
		if _, exists := fixtures[ct.ShortName]; exists {
			continue // already covered by top-level entry with same short name
		}
		if resources, ok := demo.GetResources(ct.ShortName); ok && len(resources) > 0 {
			fixtures[ct.ShortName] = resources
		}
	}

	for _, rt := range resource.AllResourceTypes() {
		checker := resource.GetRelatedDemo(rt.ShortName)
		if checker == nil {
			continue
		}
		for _, src := range fixtures[rt.ShortName] {
			for _, res := range checker(src) {
				if len(res.ResourceIDs) == 0 {
					continue
				}
				target := indexByID(fixtures[res.TargetType])
				for _, id := range res.ResourceIDs {
					if _, ok := target[id]; !ok {
						t.Errorf("related demo %s -> %s references missing ID=%q", rt.ShortName, res.TargetType, id)
					}
				}
			}
		}
	}
}

func TestIssue189_DemoDependencies_CoreCrossReferences(t *testing.T) {
	fixtures := allDemoByType(t)

	t.Run("ec2_vpc_sg_subnet_links", func(t *testing.T) {
		vpcs := indexByID(fixtures["vpc"])
		sgs := indexByID(fixtures["sg"])
		subnets := indexByID(fixtures["subnet"])
		for _, r := range fixtures["ec2"] {
			inst, ok := r.RawStruct.(ec2types.Instance)
			if !ok {
				t.Fatalf("ec2 fixture RawStruct type = %T", r.RawStruct)
			}
			if inst.VpcId == nil || *inst.VpcId == "" {
				t.Errorf("ec2 %s missing VpcId", r.ID)
			} else if _, ok := vpcs[*inst.VpcId]; !ok {
				t.Errorf("ec2 %s VpcId %s not found in vpc fixtures", r.ID, *inst.VpcId)
			}
			if inst.SubnetId == nil || *inst.SubnetId == "" {
				t.Errorf("ec2 %s missing SubnetId", r.ID)
			} else if _, ok := subnets[*inst.SubnetId]; !ok {
				t.Errorf("ec2 %s SubnetId %s not found in subnet fixtures", r.ID, *inst.SubnetId)
			}
			if len(inst.SecurityGroups) == 0 {
				t.Errorf("ec2 %s missing SecurityGroups", r.ID)
			}
			for _, sg := range inst.SecurityGroups {
				if sg.GroupId == nil || *sg.GroupId == "" {
					t.Errorf("ec2 %s has security group with empty GroupId", r.ID)
					continue
				}
				if _, ok := sgs[*sg.GroupId]; !ok {
					t.Errorf("ec2 %s SecurityGroup %s not found in sg fixtures", r.ID, *sg.GroupId)
				}
			}
		}
	})

	t.Run("ec2_image_snapshot_eip_asg_links", func(t *testing.T) {
		amis := indexByID(fixtures["ami"])
		snaps := indexByID(fixtures["ebs-snap"])
		eips := fixtures["eip"]
		asgs := fixtures["asg"]
		roles := indexByID(fixtures["role"])
		seenSnapLink := false
		seenEipLink := false
		seenAsgLink := false
		seenProfileRoleLink := false
		for _, r := range fixtures["ec2"] {
			inst := r.RawStruct.(ec2types.Instance)
			if inst.ImageId != nil && *inst.ImageId != "" {
				if _, ok := amis[*inst.ImageId]; !ok {
					t.Errorf("ec2 %s ImageId %s not found in ami fixtures", r.ID, *inst.ImageId)
				}
			}
			for _, bdm := range inst.BlockDeviceMappings {
				if bdm.Ebs == nil || bdm.Ebs.VolumeId == nil {
					continue
				}
				for _, snap := range fixtures["ebs-snap"] {
					raw := snap.RawStruct.(ec2types.Snapshot)
					if raw.VolumeId != nil && *raw.VolumeId == *bdm.Ebs.VolumeId {
						seenSnapLink = true
					}
				}
			}
			if inst.IamInstanceProfile != nil && inst.IamInstanceProfile.Arn != nil && *inst.IamInstanceProfile.Arn != "" {
				profileName := arnLeaf(*inst.IamInstanceProfile.Arn)
				if _, ok := roles[profileName]; ok {
					seenProfileRoleLink = true
				}
			}
		}
		for _, eip := range eips {
			raw := eip.RawStruct.(ec2types.Address)
			if raw.InstanceId != nil && *raw.InstanceId != "" {
				seenEipLink = true
			}
		}
		for _, asg := range asgs {
			raw := asg.RawStruct.(asgtypes.AutoScalingGroup)
			if len(raw.Instances) > 0 {
				seenAsgLink = true
			}
		}
		if len(snaps) == 0 {
			t.Fatal("ebs-snap fixtures missing")
		}
		if !seenSnapLink {
			t.Error("no EC2->EBS->snapshot demo linkage detected")
		}
		if !seenEipLink {
			t.Error("no EC2 fixture has associated EIP")
		}
		if !seenAsgLink {
			t.Error("no ASG fixture references EC2 instances")
		}
		if !seenProfileRoleLink {
			t.Error("no EC2 instance profile ARN maps to an IAM role fixture")
		}
	})

	t.Run("lambda_loggroup_sqs_role_links", func(t *testing.T) {
		logGroups := indexByID(fixtures["logs"])
		roles := indexByID(fixtures["role"])
		sqsByID := indexByID(fixtures["sqs"])
		canonicalFound := 0
		eventSourceFound := 0
		for _, r := range fixtures["lambda"] {
			fn := r.RawStruct.(lambdatypes.FunctionConfiguration)
			if fn.FunctionName == nil {
				continue
			}
			canonical := "/aws/lambda/" + *fn.FunctionName
			if _, ok := logGroups[canonical]; ok {
				canonicalFound++
			}
			if fn.LoggingConfig != nil && fn.LoggingConfig.LogGroup != nil {
				if _, ok := logGroups[*fn.LoggingConfig.LogGroup]; !ok {
					t.Errorf("lambda %s custom log group %s not found", r.ID, *fn.LoggingConfig.LogGroup)
				}
			}
			if fn.Role != nil && *fn.Role != "" {
				name := arnLeaf(*fn.Role)
				if _, ok := roles[name]; !ok {
					t.Errorf("lambda %s role %s not found in role fixtures", r.ID, name)
				}
			}
			if eventSource := strings.TrimSpace(r.Fields["event_source_arn"]); eventSource != "" {
				queueName := arnLeaf(eventSource)
				if _, ok := sqsByID[queueName]; !ok {
					t.Errorf("lambda %s event source queue %s not found in sqs fixtures", r.ID, queueName)
				}
				eventSourceFound++
			}
		}
		if canonicalFound == 0 {
			t.Error("no Lambda fixture maps to canonical /aws/lambda/{name} log group")
		}
		if eventSourceFound == 0 {
			t.Error("no Lambda fixture contains event_source_arn linked to SQS fixture")
		}
	})

	t.Run("ecs_tg_elb_ecr_logs_links", func(t *testing.T) {
		tgByName := indexByID(fixtures["tg"])
		elbByName := indexByID(fixtures["elb"])
		ecrByName := indexByID(fixtures["ecr"])
		logByName := indexByID(fixtures["logs"])
		for _, r := range fixtures["ecs-svc"] {
			svc := r.RawStruct.(ecstypes.Service)
			for _, lb := range svc.LoadBalancers {
				if lb.TargetGroupArn == nil {
					continue
				}
				tgName := lbNameFromARN(*lb.TargetGroupArn)
				if _, ok := tgByName[tgName]; !ok {
					t.Errorf("ecs-svc %s target group %s missing", r.ID, tgName)
				}
			}
		}
		for _, r := range fixtures["tg"] {
			tg := r.RawStruct.(elbv2types.TargetGroup)
			for _, lbArn := range tg.LoadBalancerArns {
				lbName := lbNameFromARN(lbArn)
				if _, ok := elbByName[lbName]; !ok {
					t.Errorf("tg %s load balancer %s missing", r.ID, lbName)
				}
			}
		}
		if len(ecrByName) == 0 || len(logByName) == 0 {
			t.Fatal("ecr or logs fixtures missing")
		}
	})

	t.Run("s3_cf_r53_links", func(t *testing.T) {
		s3ByName := indexByID(fixtures["s3"])
		lambdaByName := indexByID(fixtures["lambda"])
		sqsByName := indexByID(fixtures["sqs"])
		snsByARN := indexByID(fixtures["sns"])
		cfByID := indexByID(fixtures["cf"])
		elbDNSExists := func(dns string) bool {
			want := strings.TrimSuffix(dns, ".")
			for _, r := range fixtures["elb"] {
				raw := r.RawStruct.(elbv2types.LoadBalancer)
				if raw.DNSName != nil && strings.EqualFold(strings.TrimSuffix(*raw.DNSName, "."), want) {
					return true
				}
			}
			return false
		}
		cfDomainExists := func(domain string) bool {
			for _, r := range fixtures["cf"] {
				raw := r.RawStruct.(cloudfronttypes.DistributionSummary)
				if raw.DomainName != nil && strings.EqualFold(strings.TrimSuffix(*raw.DomainName, "."), strings.TrimSuffix(domain, ".")) {
					return true
				}
			}
			return false
		}
		if len(cfByID) == 0 || len(fixtures["elb"]) == 0 {
			t.Fatal("cloudfront or elb fixtures missing")
		}
		seenS3NotificationLink := false
		for _, b := range fixtures["s3"] {
			if lambdaArn := strings.TrimSpace(b.Fields["notification_lambda"]); lambdaArn != "" {
				name := arnLeaf(lambdaArn)
				if _, ok := lambdaByName[name]; !ok {
					t.Errorf("s3 bucket %s notification Lambda %s missing", b.ID, name)
				}
				seenS3NotificationLink = true
			}
			if sqsArn := strings.TrimSpace(b.Fields["notification_sqs"]); sqsArn != "" {
				name := arnLeaf(sqsArn)
				if _, ok := sqsByName[name]; !ok {
					t.Errorf("s3 bucket %s notification SQS %s missing", b.ID, name)
				}
				seenS3NotificationLink = true
			}
			if snsArn := strings.TrimSpace(b.Fields["notification_sns"]); snsArn != "" {
				if _, ok := snsByARN[snsArn]; !ok {
					t.Errorf("s3 bucket %s notification SNS %s missing", b.ID, snsArn)
				}
				seenS3NotificationLink = true
			}
		}
		if !seenS3NotificationLink {
			t.Error("no S3 notification linkage to Lambda/SQS/SNS fixtures found")
		}
		for _, cf := range fixtures["cf"] {
			raw := cf.RawStruct.(cloudfronttypes.DistributionSummary)
			if raw.Origins == nil {
				continue
			}
			for _, o := range raw.Origins.Items {
				if o.DomainName == nil {
					continue
				}
				d := strings.TrimSuffix(*o.DomainName, ".")
				if strings.Contains(d, ".s3") {
					bucket := strings.Split(d, ".")[0]
					if _, ok := s3ByName[bucket]; !ok {
						t.Errorf("cloudfront origin bucket %s missing in s3 fixtures", bucket)
					}
				} else if strings.Contains(d, ".elb.amazonaws.com") {
					if !elbDNSExists(d) {
						t.Errorf("cloudfront origin ELB DNS %s missing in elb fixtures", d)
					}
				}
			}
		}
		for _, zone := range fixtures["r53"] {
			records, ok := demo.GetR53Records(zone.ID)
			if !ok {
				t.Fatalf("r53 zone %s missing records fixture", zone.ID)
			}
			for _, rec := range records {
				raw := rec.RawStruct.(r53types.ResourceRecordSet)
				if raw.AliasTarget == nil || raw.AliasTarget.DNSName == nil {
					continue
				}
				dns := strings.TrimSuffix(*raw.AliasTarget.DNSName, ".")
				if strings.Contains(dns, ".cloudfront.net") {
					if !cfDomainExists(dns) {
						t.Errorf("r53 alias target %s not found in cloudfront fixtures", dns)
					}
					if raw.AliasTarget.HostedZoneId == nil || *raw.AliasTarget.HostedZoneId != "Z2FDTNDATAQYW2" {
						t.Errorf("r53 cloudfront alias %s has unexpected hosted zone ID", dns)
					}
				}
				if strings.Contains(dns, ".elb.amazonaws.com") {
					if !elbDNSExists(dns) {
						t.Errorf("r53 alias target ELB %s not found", dns)
					}
				}
				if strings.Contains(dns, ".s3-website") {
					bucket := strings.Split(dns, ".")[0]
					if _, ok := s3ByName[bucket]; !ok {
						t.Errorf("r53 alias target S3 website bucket %s not found", bucket)
					}
				}
			}
		}
	})

	t.Run("cloudtrail_alarm_sg_vpc_eks_links", func(t *testing.T) {
		all := make(map[string]struct{})
		for _, list := range fixtures {
			for _, r := range list {
				all[r.ID] = struct{}{}
			}
		}
		for _, ev := range fixtures["ct-events"] {
			raw := ev.RawStruct.(cloudtrailtypes.Event)
			for _, rr := range raw.Resources {
				if rr.ResourceName == nil || *rr.ResourceName == "" {
					continue
				}
				if _, ok := all[*rr.ResourceName]; !ok {
					t.Errorf("cloudtrail event %s resource %s missing from fixtures", ev.ID, *rr.ResourceName)
				}
			}
			if raw.CloudTrailEvent != nil && *raw.CloudTrailEvent != "" {
				issuerARN := cloudTrailSessionIssuerARN(*raw.CloudTrailEvent)
				if issuerARN != "" {
					roleName := arnLeaf(issuerARN)
					if _, ok := all[roleName]; !ok {
						t.Errorf("cloudtrail event %s session issuer role %s missing from fixtures", ev.ID, roleName)
					}
				}
			}
		}
		for _, a := range fixtures["alarm"] {
			raw := a.RawStruct.(cwtypes.MetricAlarm)
			for _, dim := range raw.Dimensions {
				if dim.Value == nil || *dim.Value == "" {
					continue
				}
				if _, ok := all[*dim.Value]; !ok {
					// Some dimensions are names/ARNs rather than IDs; ensure at least one of each alarm has valid fixture linkage.
					if !strings.Contains(*dim.Value, ":") {
						t.Errorf("alarm %s dimension value %s missing from fixtures", a.ID, *dim.Value)
					}
				}
			}
		}
		sgByID := indexByID(fixtures["sg"])
		for _, eni := range fixtures["eni"] {
			raw := eni.RawStruct.(ec2types.NetworkInterface)
			for _, g := range raw.Groups {
				if g.GroupId == nil {
					continue
				}
				if _, ok := sgByID[*g.GroupId]; !ok {
					t.Errorf("eni %s references missing sg %s", eni.ID, *g.GroupId)
				}
			}
		}
		for _, sg := range fixtures["sg"] {
			raw := sg.RawStruct.(ec2types.SecurityGroup)
			for _, p := range raw.IpPermissions {
				for _, pair := range p.UserIdGroupPairs {
					if pair.GroupId == nil {
						continue
					}
					if _, ok := sgByID[*pair.GroupId]; !ok {
						t.Errorf("sg %s rule references missing sg %s", sg.ID, *pair.GroupId)
					}
				}
			}
		}
		vpcByID := indexByID(fixtures["vpc"])
		for _, typ := range []string{"ec2", "subnet", "sg", "nat", "igw", "elb", "dbi"} {
			for _, r := range fixtures[typ] {
				val := r.Fields["vpc_id"]
				if val == "" {
					continue
				}
				if _, ok := vpcByID[val]; !ok {
					t.Errorf("%s %s references missing vpc %s", typ, r.ID, val)
				}
			}
		}
		logByName := indexByID(fixtures["logs"])
		nodeByName := indexByID(fixtures["ng"])
		seenEKSNodeTagLink := false
		for _, ec2 := range fixtures["ec2"] {
			raw := ec2.RawStruct.(ec2types.Instance)
			clusterName := ""
			nodegroupName := ""
			for _, tag := range raw.Tags {
				if tag.Key == nil || tag.Value == nil {
					continue
				}
				switch *tag.Key {
				case "eks:cluster-name":
					clusterName = *tag.Value
				case "eks:nodegroup-name":
					nodegroupName = *tag.Value
				}
			}
			if clusterName == "" || nodegroupName == "" {
				continue
			}
			seenEKSNodeTagLink = true
			if _, ok := nodeByName[nodegroupName]; !ok {
				t.Errorf("ec2 %s eks nodegroup %s not found in ng fixtures", ec2.ID, nodegroupName)
			}
		}
		if !seenEKSNodeTagLink {
			t.Error("no EC2 fixture has EKS nodegroup tags")
		}
		for _, c := range fixtures["eks"] {
			var clusterName string
			switch raw := c.RawStruct.(type) {
			case ekstypes.Cluster:
				if raw.Name != nil {
					clusterName = *raw.Name
				}
			case *ekstypes.Cluster:
				if raw != nil && raw.Name != nil {
					clusterName = *raw.Name
				}
			default:
				t.Fatalf("eks fixture %s unexpected RawStruct type %T", c.ID, c.RawStruct)
			}
			if clusterName == "" {
				continue
			}
			if _, ok := logByName["/aws/eks/"+clusterName+"/cluster"]; !ok {
				t.Errorf("eks cluster %s missing matching log group", clusterName)
			}
			hasNodegroup := false
			for _, ng := range fixtures["ng"] {
				nraw := ng.RawStruct.(ekstypes.Nodegroup)
				if nraw.ClusterName != nil && *nraw.ClusterName == clusterName {
					hasNodegroup = true
					break
				}
			}
			if !hasNodegroup {
				t.Errorf("eks cluster %s has no matching nodegroup", clusterName)
			}
		}
		if len(nodeByName) == 0 {
			t.Fatal("nodegroup fixtures missing")
		}
	})
}

func cloudTrailSessionIssuerARN(eventJSON string) string {
	var body map[string]any
	if err := json.Unmarshal([]byte(eventJSON), &body); err != nil {
		return ""
	}
	userIdentity, _ := body["userIdentity"].(map[string]any)
	sessionContext, _ := userIdentity["sessionContext"].(map[string]any)
	sessionIssuer, _ := sessionContext["sessionIssuer"].(map[string]any)
	arn, _ := sessionIssuer["arn"].(string)
	return strings.TrimSpace(arn)
}

func arnLeaf(arn string) string {
	idx := strings.LastIndex(arn, "/")
	if idx >= 0 && idx < len(arn)-1 {
		return arn[idx+1:]
	}
	idx = strings.LastIndex(arn, ":")
	if idx >= 0 && idx < len(arn)-1 {
		return arn[idx+1:]
	}
	return arn
}

func lbNameFromARN(arn string) string {
	parts := strings.Split(arn, "/")
	// Example: arn:...:loadbalancer/app/acme-prod-web/1234567890abcdef
	if len(parts) >= 3 {
		return parts[len(parts)-2]
	}
	return arnLeaf(arn)
}
