---
title: "Resource Types"
---

a9s supports **62 AWS resource types** across **12 service categories**. All API calls are read-only.

## Compute

| Resource | Short Name |
|----------|-----------|
| EC2 Instances | `ec2` |
| ECS Services | `ecs-svc` |
| ECS Clusters | `ecs` |
| ECS Tasks | `ecs-task` |
| Lambda Functions | `lambda` |
| Auto Scaling Groups | `asg` |
| Elastic Beanstalk | `eb` |

## Containers

| Resource | Short Name |
|----------|-----------|
| EKS Clusters | `eks` |
| EKS Node Groups | `ng` |

## Networking

| Resource | Short Name |
|----------|-----------|
| Load Balancers | `elb` |
| Target Groups | `tg` |
| Security Groups | `sg` |
| VPCs | `vpc` |
| Subnets | `subnet` |
| Route Tables | `rtb` |
| NAT Gateways | `nat` |
| Internet Gateways | `igw` |
| Elastic IPs | `eip` |
| VPC Endpoints | `vpce` |
| Transit Gateways | `tgw` |
| Network Interfaces | `eni` |

## Databases & Storage

| Resource | Short Name |
|----------|-----------|
| DB Instances | `dbi` |
| S3 Buckets | `s3` |
| ElastiCache Redis | `redis` |
| DB Clusters | `dbc` |
| DynamoDB Tables | `ddb` |
| OpenSearch Domains | `opensearch` |
| Redshift Clusters | `redshift` |
| EFS File Systems | `efs` |
| RDS Snapshots | `rds-snap` |
| DocDB Snapshots | `docdb-snap` |

## Monitoring

| Resource | Short Name |
|----------|-----------|
| CloudWatch Alarms | `alarm` |
| CloudWatch Log Groups | `logs` |
| CloudTrail Trails | `trail` |

## Messaging

| Resource | Short Name |
|----------|-----------|
| SQS Queues | `sqs` |
| SNS Topics | `sns` |
| SNS Subscriptions | `sns-sub` |
| EventBridge Rules | `eb-rule` |
| Kinesis Streams | `kinesis` |
| MSK Clusters | `msk` |
| Step Functions | `sfn` |

## Secrets & Config

| Resource | Short Name |
|----------|-----------|
| Secrets Manager | `secrets` |
| SSM Parameters | `ssm` |
| KMS Keys | `kms` |

## DNS & CDN

| Resource | Short Name |
|----------|-----------|
| Route 53 Hosted Zones | `r53` |
| CloudFront Distributions | `cf` |
| ACM Certificates | `acm` |
| API Gateways | `apigw` |

## Security & IAM

| Resource | Short Name |
|----------|-----------|
| IAM Roles | `role` |
| IAM Policies | `policy` |
| IAM Users | `iam-user` |
| IAM Groups | `iam-group` |
| WAF Web ACLs | `waf` |

## CI/CD

| Resource | Short Name |
|----------|-----------|
| CloudFormation Stacks | `cfn` |
| CodePipelines | `pipeline` |
| CodeBuild Projects | `cb` |
| ECR Repositories | `ecr` |
| CodeArtifact Repos | `codeartifact` |

## Data & Analytics

| Resource | Short Name |
|----------|-----------|
| Glue Jobs | `glue` |
| Athena Workgroups | `athena` |

## Backup

| Resource | Short Name |
|----------|-----------|
| Backup Plans | `backup` |
| SES Identities | `ses` |
