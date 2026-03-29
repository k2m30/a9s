# Elastic Beanstalk (eb) — Related Resources

## Real-World Use Cases

**1. "What's actually running inside this Beanstalk environment?"** EB is an abstraction over EC2, ASG, ELB, and SGs. When something goes wrong, you need to break through the abstraction and find the underlying resources — the ASG that manages instances, the ELB that routes traffic, the SGs that control access.

**2. "Why is the deployment stuck?"** EB deployments are CloudFormation stacks under the hood. When a deployment hangs, the answer is usually in the CFN stack events, not the EB console.

**3. "Is this EB environment the reason our bill spiked?"** You need to find the EC2 instances, their instance types, and the ELB — EB hides these details behind its own abstraction.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| CloudFormation Stacks (cfn) | EB creates a CFN stack named `awseb-{environment-id}-stack` for each environment. Search CFN stacks matching this pattern, or look for `elasticbeanstalk:environment-id` tag. | "Why is the deployment stuck?" CFN stack events show exactly which resource is blocking. | P0 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| EC2 Instances (ec2) | `elasticbeanstalk:DescribeEnvironmentResources` with `EnvironmentId` → returns `Instances[]` with instance IDs. Or search EC2 instances for `elasticbeanstalk:environment-name` tag. | "Which instances run this environment?" For SSH/SSM access, or to check instance health. | P0 |
| Auto Scaling Group (asg) | Same `DescribeEnvironmentResources` → returns `AutoScalingGroups[]`. Or search ASGs for `elasticbeanstalk:environment-name` tag. | "What's the scaling config? Why did it scale?" Navigate to the ASG for scaling activities and policies. | P1 |
| Load Balancer (elb) | Same `DescribeEnvironmentResources` → returns `LoadBalancers[]`. Or search ELBs for `elasticbeanstalk:environment-name` tag. | "What's the ELB health? Why are requests failing?" | P1 |
| Security Groups (sg) | Via the EC2 instances or ELB. EB-created SGs typically have `elasticbeanstalk` in the description or name. | "What ports are open?" Security audit of the EB environment. | P2 |
| CloudWatch Log Groups (logs) | EB-managed log groups follow the pattern `/aws/elasticbeanstalk/{environment-name}/{log-type}`. Common log types: `var/log/web.stdout.log`, `var/log/eb-engine.log`. | "Where are the application logs?" | P1 |
| S3 Bucket (s3) | EB stores application versions in an S3 bucket named `elasticbeanstalk-{region}-{account-id}`. Environment's `VersionLabel` references a key in this bucket. | "Where is the deployed artifact?" Useful for verifying which code version is actually deployed. | P2 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| UpdateEnvironment | "Who deployed or changed environment config?" Shows configuration changes, platform updates, and application version deployments. |
| TerminateEnvironment | "Who tore down this environment?" |
| RebuildEnvironment | "Who triggered a full rebuild?" Rebuilds replace all underlying resources. |
