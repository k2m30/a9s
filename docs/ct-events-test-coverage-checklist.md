# ct-events Test Coverage Checklist

| Event ID | EventName | Column | Related Resource | Expected Resolves To | Covered? |
|---|---|---|------------------|--|---|
| evt-0a1b2c3d4e5f60001 | CreateBucket | left | user             | arn:aws:iam::123456789012:root | ❌ |
| evt-0a1b2c3d4e5f60001 | CreateBucket | left | bucket           | webapp-assets-prod | ❌ |
| evt-0a1b2c3d4e5f60001 | CreateBucket | right | role             | - | ❌ |
| evt-0a1b2c3d4e5f60001 | CreateBucket | right | iam-user         | arn:aws:iam::123456789012:root | ❌ |
| evt-0a1b2c3d4e5f60001 | CreateBucket | right | ec2              | - | ❌ |
| evt-0a1b2c3d4e5f60001 | CreateBucket | right | s3               | webapp-assets-prod | ❌ |
| evt-0a1b2c3d4e5f60001 | CreateBucket | right | s3_objects       | - | ❌ |
| evt-0a1b2c3d4e5f60001 | CreateBucket | right | lambda           | - | ❌ |
| evt-0a1b2c3d4e5f60001 | CreateBucket | right | rds              | - | ❌ |
| evt-0a1b2c3d4e5f60001 | CreateBucket | right | kms              | - | ❌ |
| evt-0a1b2c3d4e5f60001 | CreateBucket | right | secrets          | - | ❌ |
| evt-0a1b2c3d4e5f60001 | CreateBucket | right | vpce             | - | ❌ |
| evt-0a1b2c3d4e5f60001 | CreateBucket | right | sg               | - | ❌ |
| evt-0a1b2c3d4e5f60001 | CreateBucket | right | ddb              | - | ❌ |
| evt-0a1b2c3d4e5f60001 | CreateBucket | right | cfn              | - | ❌ |
| evt-0a1b2c3d4e5f60001 | CreateBucket | pivot | AccessKeyId      | - | ❌ |
| evt-0a1b2c3d4e5f60001 | CreateBucket | pivot | Username         | - | ❌ |
| evt-0a1b2c3d4e5f60001 | CreateBucket | pivot | EventName        | self (CreateBucket) | ❌ |
| evt-0a1b2c3d4e5f60001 | CreateBucket | pivot | SharedEventId    | - | ❌ |
| evt-0a1b2c3d4e5f60002 | DeleteBucket | left | user             | bob.smith | ❌ |
| evt-0a1b2c3d4e5f60002 | DeleteBucket | left | role_name        | - | ❌ |
| evt-0a1b2c3d4e5f60002 | DeleteBucket | right | role             | - | ❌ |
| evt-0a1b2c3d4e5f60002 | DeleteBucket | right | iam-user         | bob.smith | ❌ |
| evt-0a1b2c3d4e5f60002 | DeleteBucket | right | ec2              | - | ❌ |
| evt-0a1b2c3d4e5f60002 | DeleteBucket | right | s3               | webapp-assets-prod | ❌ |
| evt-0a1b2c3d4e5f60002 | DeleteBucket | right | s3_objects       | - | ❌ |
| evt-0a1b2c3d4e5f60002 | DeleteBucket | right | lambda           | - | ❌ |
| evt-0a1b2c3d4e5f60002 | DeleteBucket | right | rds              | - | ❌ |
| evt-0a1b2c3d4e5f60002 | DeleteBucket | right | kms              | - | ❌ |
| evt-0a1b2c3d4e5f60002 | DeleteBucket | right | secrets          | - | ❌ |
| evt-0a1b2c3d4e5f60002 | DeleteBucket | right | vpce             | - | ❌ |
| evt-0a1b2c3d4e5f60002 | DeleteBucket | right | sg               | - | ❌ |
| evt-0a1b2c3d4e5f60002 | DeleteBucket | right | ddb              | - | ❌ |
| evt-0a1b2c3d4e5f60002 | DeleteBucket | right | cfn              | - | ❌ |
| evt-0a1b2c3d4e5f60002 | DeleteBucket | pivot | AccessKeyId      | - | ❌ |
| evt-0a1b2c3d4e5f60002 | DeleteBucket | pivot | Username         | self (bob.smith) | ❌ |
| evt-0a1b2c3d4e5f60002 | DeleteBucket | pivot | EventName        | self (DeleteBucket) | ❌ |
| evt-0a1b2c3d4e5f60002 | DeleteBucket | pivot | SharedEventId    | - | ❌ |
| evt-0a1b2c3d4e5f60003 | DescribeInstances | left | user             | - | ❌ |
| evt-0a1b2c3d4e5f60003 | DescribeInstances | left | role_name        | acme-eks-node-role | ❌ |
| evt-0a1b2c3d4e5f60003 | DescribeInstances | right | role             | acme-eks-node-role | ❌ |
| evt-0a1b2c3d4e5f60003 | DescribeInstances | right | iam-user         | - | ❌ |
| evt-0a1b2c3d4e5f60003 | DescribeInstances | right | ec2              | - | ❌ |
| evt-0a1b2c3d4e5f60003 | DescribeInstances | right | s3               | - | ❌ |
| evt-0a1b2c3d4e5f60003 | DescribeInstances | right | s3_objects       | - | ❌ |
| evt-0a1b2c3d4e5f60003 | DescribeInstances | right | lambda           | - | ❌ |
| evt-0a1b2c3d4e5f60003 | DescribeInstances | right | rds              | - | ❌ |
| evt-0a1b2c3d4e5f60003 | DescribeInstances | right | kms              | - | ❌ |
| evt-0a1b2c3d4e5f60003 | DescribeInstances | right | secrets          | - | ❌ |
| evt-0a1b2c3d4e5f60003 | DescribeInstances | right | vpce             | - | ❌ |
| evt-0a1b2c3d4e5f60003 | DescribeInstances | right | sg               | - | ❌ |
| evt-0a1b2c3d4e5f60003 | DescribeInstances | right | ddb              | - | ❌ |
| evt-0a1b2c3d4e5f60003 | DescribeInstances | right | cfn              | - | ❌ |
| evt-0a1b2c3d4e5f60003 | DescribeInstances | pivot | AccessKeyId      | - | ❌ |
| evt-0a1b2c3d4e5f60003 | DescribeInstances | pivot | Username         | self (acme-eks-node-role) | ❌ |
| evt-0a1b2c3d4e5f60003 | DescribeInstances | pivot | EventName        | self (DescribeInstances) | ❌ |
| evt-0a1b2c3d4e5f60003 | DescribeInstances | pivot | SharedEventId    | - | ❌ |
| evt-0a1b2c3d4e5f60004 | TerminateInstanceInAutoScalingGroup | left | user             | - | ❌ |
| evt-0a1b2c3d4e5f60004 | TerminateInstanceInAutoScalingGroup | left | role_name        | - | ❌ |
| evt-0a1b2c3d4e5f60004 | TerminateInstanceInAutoScalingGroup | right | role             | - | ❌ |
| evt-0a1b2c3d4e5f60004 | TerminateInstanceInAutoScalingGroup | right | iam-user         | - | ❌ |
| evt-0a1b2c3d4e5f60004 | TerminateInstanceInAutoScalingGroup | right | ec2              | i-0a1b2c3d4e5f60001 | ❌ |
| evt-0a1b2c3d4e5f60004 | TerminateInstanceInAutoScalingGroup | right | s3               | - | ❌ |
| evt-0a1b2c3d4e5f60004 | TerminateInstanceInAutoScalingGroup | right | s3_objects       | - | ❌ |
| evt-0a1b2c3d4e5f60004 | TerminateInstanceInAutoScalingGroup | right | lambda           | - | ❌ |
| evt-0a1b2c3d4e5f60004 | TerminateInstanceInAutoScalingGroup | right | rds              | - | ❌ |
| evt-0a1b2c3d4e5f60004 | TerminateInstanceInAutoScalingGroup | right | kms              | - | ❌ |
| evt-0a1b2c3d4e5f60004 | TerminateInstanceInAutoScalingGroup | right | secrets          | - | ❌ |
| evt-0a1b2c3d4e5f60004 | TerminateInstanceInAutoScalingGroup | right | vpce             | - | ❌ |
| evt-0a1b2c3d4e5f60004 | TerminateInstanceInAutoScalingGroup | right | sg               | - | ❌ |
| evt-0a1b2c3d4e5f60004 | TerminateInstanceInAutoScalingGroup | right | ddb              | - | ❌ |
| evt-0a1b2c3d4e5f60004 | TerminateInstanceInAutoScalingGroup | right | cfn              | - | ❌ |
| evt-0a1b2c3d4e5f60004 | TerminateInstanceInAutoScalingGroup | pivot | AccessKeyId      | - | ❌ |
| evt-0a1b2c3d4e5f60004 | TerminateInstanceInAutoScalingGroup | pivot | Username         | - | ❌ |
| evt-0a1b2c3d4e5f60004 | TerminateInstanceInAutoScalingGroup | pivot | EventName        | self (TerminateInstanceInAutoScalingGroup) | ❌ |
| evt-0a1b2c3d4e5f60004 | TerminateInstanceInAutoScalingGroup | pivot | SharedEventId    | - | ❌ |
| evt-0a1b2c3d4e5f60005 | ApiCallRateInsight | left | user             | bob.smith | ❌ |
| evt-0a1b2c3d4e5f60005 | ApiCallRateInsight | left | role_name        | - | ❌ |
| evt-0a1b2c3d4e5f60005 | ApiCallRateInsight | right | role             | - | ❌ |
| evt-0a1b2c3d4e5f60005 | ApiCallRateInsight | right | iam-user         | bob.smith | ❌ |
| evt-0a1b2c3d4e5f60005 | ApiCallRateInsight | right | ec2              | - | ❌ |
| evt-0a1b2c3d4e5f60005 | ApiCallRateInsight | right | s3               | - | ❌ |
| evt-0a1b2c3d4e5f60005 | ApiCallRateInsight | right | s3_objects       | - | ❌ |
| evt-0a1b2c3d4e5f60005 | ApiCallRateInsight | right | lambda           | - | ❌ |
| evt-0a1b2c3d4e5f60005 | ApiCallRateInsight | right | rds              | - | ❌ |
| evt-0a1b2c3d4e5f60005 | ApiCallRateInsight | right | kms              | - | ❌ |
| evt-0a1b2c3d4e5f60005 | ApiCallRateInsight | right | secrets          | - | ❌ |
| evt-0a1b2c3d4e5f60005 | ApiCallRateInsight | right | vpce             | - | ❌ |
| evt-0a1b2c3d4e5f60005 | ApiCallRateInsight | right | sg               | - | ❌ |
| evt-0a1b2c3d4e5f60005 | ApiCallRateInsight | right | ddb              | - | ❌ |
| evt-0a1b2c3d4e5f60005 | ApiCallRateInsight | right | cfn              | - | ❌ |
| evt-0a1b2c3d4e5f60005 | ApiCallRateInsight | pivot | AccessKeyId      | - | ❌ |
| evt-0a1b2c3d4e5f60005 | ApiCallRateInsight | pivot | Username         | self (bob.smith) | ❌ |
| evt-0a1b2c3d4e5f60005 | ApiCallRateInsight | pivot | EventName        | self (ApiCallRateInsight) | ❌ |
| evt-0a1b2c3d4e5f60005 | ApiCallRateInsight | pivot | SharedEventId    | - | ❌ |
| evt-0a1b2c3d4e5f60006 | VpcEndpointAccess | left | user             | - | ❌ |
| evt-0a1b2c3d4e5f60006 | VpcEndpointAccess | left | role_name        | ci-runner | ❌ |
| evt-0a1b2c3d4e5f60006 | VpcEndpointAccess | right | role             | ci-runner | ❌ |
| evt-0a1b2c3d4e5f60006 | VpcEndpointAccess | right | iam-user         | - | ❌ |
| evt-0a1b2c3d4e5f60006 | VpcEndpointAccess | right | ec2              | - | ❌ |
| evt-0a1b2c3d4e5f60006 | VpcEndpointAccess | right | s3               | - | ❌ |
| evt-0a1b2c3d4e5f60006 | VpcEndpointAccess | right | s3_objects       | - | ❌ |
| evt-0a1b2c3d4e5f60006 | VpcEndpointAccess | right | lambda           | - | ❌ |
| evt-0a1b2c3d4e5f60006 | VpcEndpointAccess | right | rds              | - | ❌ |
| evt-0a1b2c3d4e5f60006 | VpcEndpointAccess | right | kms              | - | ❌ |
| evt-0a1b2c3d4e5f60006 | VpcEndpointAccess | right | secrets          | - | ❌ |
| evt-0a1b2c3d4e5f60006 | VpcEndpointAccess | right | vpce             | vpce-0abc123 | ❌ |
| evt-0a1b2c3d4e5f60006 | VpcEndpointAccess | right | sg               | - | ❌ |
| evt-0a1b2c3d4e5f60006 | VpcEndpointAccess | right | ddb              | - | ❌ |
| evt-0a1b2c3d4e5f60006 | VpcEndpointAccess | right | cfn              | - | ❌ |
| evt-0a1b2c3d4e5f60006 | VpcEndpointAccess | pivot | AccessKeyId      | - | ❌ |
| evt-0a1b2c3d4e5f60006 | VpcEndpointAccess | pivot | Username         | self (ci-runner) | ❌ |
| evt-0a1b2c3d4e5f60006 | VpcEndpointAccess | pivot | EventName        | self (VpcEndpointAccess) | ❌ |
| evt-0a1b2c3d4e5f60006 | VpcEndpointAccess | pivot | SharedEventId    | - | ❌ |
| e-a1b2c3d4 | DescribeInstances | left | user             | - | ❌ |
| e-a1b2c3d4 | DescribeInstances | left | role_name        | KarpenterNodeRole | ✅ |
| e-a1b2c3d4 | DescribeInstances | right | role             | KarpenterNodeRole | ✅ |
| e-a1b2c3d4 | DescribeInstances | right | iam-user         | - | ❌ |
| e-a1b2c3d4 | DescribeInstances | right | ec2              | - | ❌ |
| e-a1b2c3d4 | DescribeInstances | right | s3               | - | ❌ |
| e-a1b2c3d4 | DescribeInstances | right | s3_objects       | - | ❌ |
| e-a1b2c3d4 | DescribeInstances | right | lambda           | - | ❌ |
| e-a1b2c3d4 | DescribeInstances | right | rds              | - | ❌ |
| e-a1b2c3d4 | DescribeInstances | right | kms              | - | ❌ |
| e-a1b2c3d4 | DescribeInstances | right | secrets          | - | ❌ |
| e-a1b2c3d4 | DescribeInstances | right | vpce             | - | ❌ |
| e-a1b2c3d4 | DescribeInstances | right | sg               | - | ❌ |
| e-a1b2c3d4 | DescribeInstances | right | ddb              | - | ❌ |
| e-a1b2c3d4 | DescribeInstances | right | cfn              | - | ❌ |
| e-a1b2c3d4 | DescribeInstances | pivot | AccessKeyId      | self (ASIAY44QH8DCKARPEXMP) | ❌ |
| e-a1b2c3d4 | DescribeInstances | pivot | Username         | self (KarpenterNodeRole) | ❌ |
| e-a1b2c3d4 | DescribeInstances | pivot | EventName        | self (DescribeInstances) | ❌ |
| e-a1b2c3d4 | DescribeInstances | pivot | SharedEventId    | - | ❌ |
| e-b2c3d4e5 | TerminateInstances | left | user             | - | ❌ |
| e-b2c3d4e5 | TerminateInstances | left | role_name        | AWSReservedSSO_AdminAccess_3c4d5e6f7a8b9c0d | ✅ |
| e-b2c3d4e5 | TerminateInstances | right | role             | AWSReservedSSO_AdminAccess_3c4d5e6f7a8b9c0d | ✅ |
| e-b2c3d4e5 | TerminateInstances | right | iam-user         | - | ❌ |
| e-b2c3d4e5 | TerminateInstances | right | ec2              | i-0a1b2c3d4e5f60001, i-0a1b2c3d4e5f60002 | ✅ |
| e-b2c3d4e5 | TerminateInstances | right | s3               | - | ❌ |
| e-b2c3d4e5 | TerminateInstances | right | s3_objects       | - | ❌ |
| e-b2c3d4e5 | TerminateInstances | right | lambda           | - | ❌ |
| e-b2c3d4e5 | TerminateInstances | right | rds              | - | ❌ |
| e-b2c3d4e5 | TerminateInstances | right | kms              | - | ❌ |
| e-b2c3d4e5 | TerminateInstances | right | secrets          | - | ❌ |
| e-b2c3d4e5 | TerminateInstances | right | vpce             | - | ❌ |
| e-b2c3d4e5 | TerminateInstances | right | sg               | - | ❌ |
| e-b2c3d4e5 | TerminateInstances | right | ddb              | - | ❌ |
| e-b2c3d4e5 | TerminateInstances | right | cfn              | - | ❌ |
| e-b2c3d4e5 | TerminateInstances | pivot | AccessKeyId      | self (ASIAZK7L9PQRSSOXEXMP) | ❌ |
| e-b2c3d4e5 | TerminateInstances | pivot | Username         | self (AWSReservedSSO_AdminAccess_3c4d5e6f7a8b9c0d) | ❌ |
| e-b2c3d4e5 | TerminateInstances | pivot | EventName        | self (TerminateInstances) | ❌ |
| e-b2c3d4e5 | TerminateInstances | pivot | SharedEventId    | - | ❌ |
| e-c3d4e5f6 | PutObject | left | user             | bob.smith | ✅ |
| e-c3d4e5f6 | PutObject | left | role_name        | - | ❌ |
| e-c3d4e5f6 | PutObject | right | role             | - | ❌ |
| e-c3d4e5f6 | PutObject | right | iam-user         | bob | ✅ |
| e-c3d4e5f6 | PutObject | right | ec2              | - | ❌ |
| e-c3d4e5f6 | PutObject | right | s3               | webapp-assets-prod | ✅ |
| e-c3d4e5f6 | PutObject | right | s3_objects       | webapp-assets-prod/webapp-assets-prod/2026/04/07/app.log | ✅ |
| e-c3d4e5f6 | PutObject | right | lambda           | - | ❌ |
| e-c3d4e5f6 | PutObject | right | rds              | - | ❌ |
| e-c3d4e5f6 | PutObject | right | kms              | - | ❌ |
| e-c3d4e5f6 | PutObject | right | secrets          | - | ❌ |
| e-c3d4e5f6 | PutObject | right | vpce             | - | ❌ |
| e-c3d4e5f6 | PutObject | right | sg               | - | ❌ |
| e-c3d4e5f6 | PutObject | right | ddb              | - | ❌ |
| e-c3d4e5f6 | PutObject | right | cfn              | - | ❌ |
| e-c3d4e5f6 | PutObject | pivot | AccessKeyId      | self (AKIAIOSFODNN7BOB1XMP) | ❌ |
| e-c3d4e5f6 | PutObject | pivot | Username         | self (bob) | ❌ |
| e-c3d4e5f6 | PutObject | pivot | EventName        | self (PutObject) | ❌ |
| e-c3d4e5f6 | PutObject | pivot | SharedEventId    | - | ❌ |
| e-d4e5f6a7 | RotateKey | left | user             | - | ❌ |
| e-d4e5f6a7 | RotateKey | left | role_name        | - | ❌ |
| e-d4e5f6a7 | RotateKey | right | role             | - | ❌ |
| e-d4e5f6a7 | RotateKey | right | iam-user         | - | ❌ |
| e-d4e5f6a7 | RotateKey | right | ec2              | - | ❌ |
| e-d4e5f6a7 | RotateKey | right | s3               | - | ❌ |
| e-d4e5f6a7 | RotateKey | right | s3_objects       | - | ❌ |
| e-d4e5f6a7 | RotateKey | right | lambda           | - | ❌ |
| e-d4e5f6a7 | RotateKey | right | rds              | - | ❌ |
| e-d4e5f6a7 | RotateKey | right | kms              | arn:aws:kms:us-east-1:444444444444:key/2f7e9a5b-8c1d-4e3f-9a0b-1c2d3e4f5a6b | ✅ |
| e-d4e5f6a7 | RotateKey | right | secrets          | - | ❌ |
| e-d4e5f6a7 | RotateKey | right | vpce             | - | ❌ |
| e-d4e5f6a7 | RotateKey | right | sg               | - | ❌ |
| e-d4e5f6a7 | RotateKey | right | ddb              | - | ❌ |
| e-d4e5f6a7 | RotateKey | right | cfn              | - | ❌ |
| e-d4e5f6a7 | RotateKey | pivot | AccessKeyId      | - | ❌ |
| e-d4e5f6a7 | RotateKey | pivot | Username         | - | ❌ |
| e-d4e5f6a7 | RotateKey | pivot | EventName        | self (RotateKey) | ❌ |
| e-d4e5f6a7 | RotateKey | pivot | SharedEventId    | - | ❌ |
| e-e5f6a7b8 | PutBucketPolicy | left | user             | - | ❌ |
| e-e5f6a7b8 | PutBucketPolicy | left | role_name        | - | ❌ |
| e-e5f6a7b8 | PutBucketPolicy | right | role             | - | ❌ |
| e-e5f6a7b8 | PutBucketPolicy | right | iam-user         | - | ❌ |
| e-e5f6a7b8 | PutBucketPolicy | right | ec2              | - | ❌ |
| e-e5f6a7b8 | PutBucketPolicy | right | s3               | prod-artifacts | ✅ |
| e-e5f6a7b8 | PutBucketPolicy | right | s3_objects       | - | ❌ |
| e-e5f6a7b8 | PutBucketPolicy | right | lambda           | - | ❌ |
| e-e5f6a7b8 | PutBucketPolicy | right | rds              | - | ❌ |
| e-e5f6a7b8 | PutBucketPolicy | right | kms              | - | ❌ |
| e-e5f6a7b8 | PutBucketPolicy | right | secrets          | - | ❌ |
| e-e5f6a7b8 | PutBucketPolicy | right | vpce             | - | ❌ |
| e-e5f6a7b8 | PutBucketPolicy | right | sg               | - | ❌ |
| e-e5f6a7b8 | PutBucketPolicy | right | ddb              | - | ❌ |
| e-e5f6a7b8 | PutBucketPolicy | right | cfn              | - | ❌ |
| e-e5f6a7b8 | PutBucketPolicy | pivot | AccessKeyId      | - | ❌ |
| e-e5f6a7b8 | PutBucketPolicy | pivot | Username         | - | ❌ |
| e-e5f6a7b8 | PutBucketPolicy | pivot | EventName        | self (PutBucketPolicy) | ❌ |
| e-e5f6a7b8 | PutBucketPolicy | pivot | SharedEventId    | - | ❌ |
| e-f6a7b8c9 | GetObject | left | user             | - | ❌ |
| e-f6a7b8c9 | GetObject | left | role_name        | eks-checkout-svc-sa | ✅ |
| e-f6a7b8c9 | GetObject | right | role             | eks-checkout-svc-sa | ✅ |
| e-f6a7b8c9 | GetObject | right | iam-user         | - | ❌ |
| e-f6a7b8c9 | GetObject | right | ec2              | - | ❌ |
| e-f6a7b8c9 | GetObject | right | s3               | checkout-config | ✅ |
| e-f6a7b8c9 | GetObject | right | s3_objects       | checkout-config/checkout-config/prod/config.json | ✅ |
| e-f6a7b8c9 | GetObject | right | lambda           | - | ❌ |
| e-f6a7b8c9 | GetObject | right | rds              | - | ❌ |
| e-f6a7b8c9 | GetObject | right | kms              | - | ❌ |
| e-f6a7b8c9 | GetObject | right | secrets          | - | ❌ |
| e-f6a7b8c9 | GetObject | right | vpce             | vpce-0abc123def456 | ✅ |
| e-f6a7b8c9 | GetObject | right | sg               | - | ❌ |
| e-f6a7b8c9 | GetObject | right | ddb              | - | ❌ |
| e-f6a7b8c9 | GetObject | right | cfn              | - | ❌ |
| e-f6a7b8c9 | GetObject | pivot | AccessKeyId      | - | ❌ |
| e-f6a7b8c9 | GetObject | pivot | Username         | self (eks-checkout-svc-sa) | ❌ |
| e-f6a7b8c9 | GetObject | pivot | EventName        | self (GetObject) | ❌ |
| e-f6a7b8c9 | GetObject | pivot | SharedEventId    | - | ❌ |
| e-a7b8c9d0 | PutObject | left | user             | - | ❌ |
| e-a7b8c9d0 | PutObject | left | role_name        | CiBuildRole | ✅ |
| e-a7b8c9d0 | PutObject | right | role             | CiBuildRole | ✅ |
| e-a7b8c9d0 | PutObject | right | iam-user         | - | ❌ |
| e-a7b8c9d0 | PutObject | right | ec2              | - | ❌ |
| e-a7b8c9d0 | PutObject | right | s3               | shared-artifacts | ✅ |
| e-a7b8c9d0 | PutObject | right | s3_objects       | shared-artifacts/shared-artifacts/build-4821.tar.gz | ✅ |
| e-a7b8c9d0 | PutObject | right | lambda           | - | ❌ |
| e-a7b8c9d0 | PutObject | right | rds              | - | ❌ |
| e-a7b8c9d0 | PutObject | right | kms              | - | ❌ |
| e-a7b8c9d0 | PutObject | right | secrets          | - | ❌ |
| e-a7b8c9d0 | PutObject | right | vpce             | - | ❌ |
| e-a7b8c9d0 | PutObject | right | sg               | - | ❌ |
| e-a7b8c9d0 | PutObject | right | ddb              | - | ❌ |
| e-a7b8c9d0 | PutObject | right | cfn              | - | ❌ |
| e-a7b8c9d0 | PutObject | pivot | AccessKeyId      | self (ASIAQF3M2N8KCIB1XMPL) | ❌ |
| e-a7b8c9d0 | PutObject | pivot | Username         | self (CiBuildRole) | ❌ |
| e-a7b8c9d0 | PutObject | pivot | EventName        | self (PutObject) | ❌ |
| e-a7b8c9d0 | PutObject | pivot | SharedEventId    | - | ❌ |
| e-b8c9d0e1 | RunInstances | left | user             | - | ❌ |
| e-b8c9d0e1 | RunInstances | left | role_name        | - | ❌ |
| e-b8c9d0e1 | RunInstances | right | role             | - | ❌ |
| e-b8c9d0e1 | RunInstances | right | iam-user         | - | ❌ |
| e-b8c9d0e1 | RunInstances | right | ec2              | - | ❌ |
| e-b8c9d0e1 | RunInstances | right | s3               | - | ❌ |
| e-b8c9d0e1 | RunInstances | right | s3_objects       | - | ❌ |
| e-b8c9d0e1 | RunInstances | right | lambda           | - | ❌ |
| e-b8c9d0e1 | RunInstances | right | rds              | - | ❌ |
| e-b8c9d0e1 | RunInstances | right | kms              | - | ❌ |
| e-b8c9d0e1 | RunInstances | right | secrets          | - | ❌ |
| e-b8c9d0e1 | RunInstances | right | vpce             | - | ❌ |
| e-b8c9d0e1 | RunInstances | right | sg               | - | ❌ |
| e-b8c9d0e1 | RunInstances | right | ddb              | - | ❌ |
| e-b8c9d0e1 | RunInstances | right | cfn              | - | ❌ |
| e-b8c9d0e1 | RunInstances | pivot | AccessKeyId      | - | ❌ |
| e-b8c9d0e1 | RunInstances | pivot | Username         | - | ❌ |
| e-b8c9d0e1 | RunInstances | pivot | EventName        | self (RunInstances) | ❌ |
| e-b8c9d0e1 | RunInstances | pivot | SharedEventId    | - | ❌ |
| e-c9d0e1f2 | PutObject | left | user             | - | ❌ |
| e-c9d0e1f2 | PutObject | left | role_name        | DataPipelineRole | ✅ |
| e-c9d0e1f2 | PutObject | right | role             | DataPipelineRole | ✅ |
| e-c9d0e1f2 | PutObject | right | iam-user         | - | ❌ |
| e-c9d0e1f2 | PutObject | right | ec2              | - | ❌ |
| e-c9d0e1f2 | PutObject | right | s3               | prod-lake | ✅ |
| e-c9d0e1f2 | PutObject | right | s3_objects       | prod-lake/prod-lake/landing/2026/04/07/batch-0719.parquet | ✅ |
| e-c9d0e1f2 | PutObject | right | lambda           | - | ❌ |
| e-c9d0e1f2 | PutObject | right | rds              | - | ❌ |
| e-c9d0e1f2 | PutObject | right | kms              | - | ❌ |
| e-c9d0e1f2 | PutObject | right | secrets          | - | ❌ |
| e-c9d0e1f2 | PutObject | right | vpce             | vpce-0ff11223344556677 | ✅ |
| e-c9d0e1f2 | PutObject | right | sg               | - | ❌ |
| e-c9d0e1f2 | PutObject | right | ddb              | - | ❌ |
| e-c9d0e1f2 | PutObject | right | cfn              | - | ❌ |
| e-c9d0e1f2 | PutObject | pivot | AccessKeyId      | - | ❌ |
| e-c9d0e1f2 | PutObject | pivot | Username         | self (DataPipelineRole) | ❌ |
| e-c9d0e1f2 | PutObject | pivot | EventName        | self (PutObject) | ❌ |
| e-c9d0e1f2 | PutObject | pivot | SharedEventId    | - | ❌ |
| e-d0e1f2a3 | CreateUser | left | user             | alice.johnson | ❌ |
| e-d0e1f2a3 | CreateUser | left | role_name        | - | ❌ |
| e-d0e1f2a3 | CreateUser | right | role             | - | ❌ |
| e-d0e1f2a3 | CreateUser | right | iam-user         | alice.johnson, charlie | ❌ |
| e-d0e1f2a3 | CreateUser | right | ec2              | - | ❌ |
| e-d0e1f2a3 | CreateUser | right | s3               | - | ❌ |
| e-d0e1f2a3 | CreateUser | right | s3_objects       | - | ❌ |
| e-d0e1f2a3 | CreateUser | right | lambda           | - | ❌ |
| e-d0e1f2a3 | CreateUser | right | rds              | - | ❌ |
| e-d0e1f2a3 | CreateUser | right | kms              | - | ❌ |
| e-d0e1f2a3 | CreateUser | right | secrets          | - | ❌ |
| e-d0e1f2a3 | CreateUser | right | vpce             | - | ❌ |
| e-d0e1f2a3 | CreateUser | right | sg               | - | ❌ |
| e-d0e1f2a3 | CreateUser | right | ddb              | - | ❌ |
| e-d0e1f2a3 | CreateUser | right | cfn              | - | ❌ |
| e-d0e1f2a3 | CreateUser | pivot | AccessKeyId      | - | ❌ |
| e-d0e1f2a3 | CreateUser | pivot | Username         | self (alice.johnson) | ❌ |
| e-d0e1f2a3 | CreateUser | pivot | EventName        | self (CreateUser) | ❌ |
| e-d0e1f2a3 | CreateUser | pivot | SharedEventId    | - | ❌ |
| e-e1f2a3b4 | AttachUserPolicy | left | user             | alice.johnson | ❌ |
| e-e1f2a3b4 | AttachUserPolicy | left | role_name        | - | ❌ |
| e-e1f2a3b4 | AttachUserPolicy | right | role             | - | ❌ |
| e-e1f2a3b4 | AttachUserPolicy | right | iam-user         | alice.johnson, bob | ❌ |
| e-e1f2a3b4 | AttachUserPolicy | right | ec2              | - | ❌ |
| e-e1f2a3b4 | AttachUserPolicy | right | s3               | - | ❌ |
| e-e1f2a3b4 | AttachUserPolicy | right | s3_objects       | - | ❌ |
| e-e1f2a3b4 | AttachUserPolicy | right | lambda           | - | ❌ |
| e-e1f2a3b4 | AttachUserPolicy | right | rds              | - | ❌ |
| e-e1f2a3b4 | AttachUserPolicy | right | kms              | - | ❌ |
| e-e1f2a3b4 | AttachUserPolicy | right | secrets          | - | ❌ |
| e-e1f2a3b4 | AttachUserPolicy | right | vpce             | - | ❌ |
| e-e1f2a3b4 | AttachUserPolicy | right | sg               | - | ❌ |
| e-e1f2a3b4 | AttachUserPolicy | right | ddb              | - | ❌ |
| e-e1f2a3b4 | AttachUserPolicy | right | cfn              | - | ❌ |
| e-e1f2a3b4 | AttachUserPolicy | pivot | AccessKeyId      | - | ❌ |
| e-e1f2a3b4 | AttachUserPolicy | pivot | Username         | self (alice.johnson) | ❌ |
| e-e1f2a3b4 | AttachUserPolicy | pivot | EventName        | self (AttachUserPolicy) | ❌ |
| e-e1f2a3b4 | AttachUserPolicy | pivot | SharedEventId    | - | ❌ |
| e-f2a3b4c5 | CreateAccessKey | left | user             | alice.johnson | ❌ |
| e-f2a3b4c5 | CreateAccessKey | left | role_name        | - | ❌ |
| e-f2a3b4c5 | CreateAccessKey | right | role             | - | ❌ |
| e-f2a3b4c5 | CreateAccessKey | right | iam-user         | alice.johnson, bob | ❌ |
| e-f2a3b4c5 | CreateAccessKey | right | ec2              | - | ❌ |
| e-f2a3b4c5 | CreateAccessKey | right | s3               | - | ❌ |
| e-f2a3b4c5 | CreateAccessKey | right | s3_objects       | - | ❌ |
| e-f2a3b4c5 | CreateAccessKey | right | lambda           | - | ❌ |
| e-f2a3b4c5 | CreateAccessKey | right | rds              | - | ❌ |
| e-f2a3b4c5 | CreateAccessKey | right | kms              | - | ❌ |
| e-f2a3b4c5 | CreateAccessKey | right | secrets          | - | ❌ |
| e-f2a3b4c5 | CreateAccessKey | right | vpce             | - | ❌ |
| e-f2a3b4c5 | CreateAccessKey | right | sg               | - | ❌ |
| e-f2a3b4c5 | CreateAccessKey | right | ddb              | - | ❌ |
| e-f2a3b4c5 | CreateAccessKey | right | cfn              | - | ❌ |
| e-f2a3b4c5 | CreateAccessKey | pivot | AccessKeyId      | - | ❌ |
| e-f2a3b4c5 | CreateAccessKey | pivot | Username         | self (alice.johnson) | ❌ |
| e-f2a3b4c5 | CreateAccessKey | pivot | EventName        | self (CreateAccessKey) | ❌ |
| e-f2a3b4c5 | CreateAccessKey | pivot | SharedEventId    | - | ❌ |

## Notes

- Covered? marks ✅ only for (event, resource) pairs asserted in `tests/unit/ctdetail_demo_rightcol_nav_test.go` (right-column groups listed per case), and for left-column `user`/`role_name` pairs on Cases A–I that also have a non-empty value and are exercised as Principal rows in `tests/unit/ctdetail_demo_nav_test.go`. Pivot rows are never asserted in any of the three matrix tests → all ❌.
- Initial 6 fixtures (`evt-0a1b2c3d4e5f60001`–`evt-0a1b2c3d4e5f60006`) and Cases J/K/L (`e-d0e1f2a3`, `e-e1f2a3b4`, `e-f2a3b4c5`) are NOT covered by any of the three matrix test files.
- Case H (`e-b8c9d0e1`) is asserted to have zero actionable right-column rows; this is a negative assertion, so rows are left ❌.
- `s3_objects` values follow the task rule `<bucket>/<key>` verbatim using the raw `key` field from `requestParameters`. In the current fixtures the `key` values already begin with the bucket name, producing the doubled-bucket display (e.g. `webapp-assets-prod/webapp-assets-prod/2026/04/07/app.log`). This is taken from the source JSON and not inferred.
- `pivot AccessKeyId` is derived strictly from `userIdentity.accessKeyId`. Case L (`CreateAccessKey`) has an `accessKeyId` only inside `responseElements.accessKey`, not in `userIdentity`, so it is `-`.
- No `sharedEventID` is present in any of the 18 fixture JSON blobs → every `pivot SharedEventId` row resolves to `-`.
