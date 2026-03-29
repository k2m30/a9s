# SSM Parameters (ssm) — Related Resources

## Real-World Use Cases

**1. "What references this parameter?"** SSM parameters store configuration values used by CloudFormation templates (`{{resolve:ssm:...}}`), ECS task definitions (as injected secrets), and Lambda functions (via code or environment variables). Before changing a parameter, you need to know the blast radius.

**2. "Who changed this parameter and when?"** SSM has built-in parameter history (`ssm:GetParameterHistory`), separate from CloudTrail. The history shows value changes, versions, and the actor.

**3. "Is this parameter encrypted?"** SecureString parameters are encrypted with KMS. You need the KMS key to verify who can decrypt the value.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| ECS Task Definitions (not in a9s) | Search task definitions for `containerDefinitions[].secrets[].valueFrom` matching this parameter's ARN or name (SSM parameters can be referenced by name with `arn:aws:ssm:{region}:{account}:parameter/{name}` or by name prefix). | "Which ECS tasks inject this parameter?" Breaking change if the parameter is deleted. | P1 |
| CloudFormation Stacks (cfn) | CFN templates use `{{resolve:ssm:{parameter-name}}}` or `{{resolve:ssm-secure:{parameter-name}}}`. Check CFN stacks for parameters referencing this SSM parameter. Also: SSM parameters are often used as CFN stack parameters via `AWS::SSM::Parameter::Value<>` type. Heuristic — requires parsing templates. | "Which CloudFormation stacks use this parameter?" CFN resolves SSM parameters at deploy time — a changed value affects the next deploy. | P1 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| KMS Key (kms) | For SecureString parameters, the parameter has `KeyId` in its metadata — FORWARD. Default is `alias/aws/ssm` (AWS-managed key), but custom keys are common. | "Who can decrypt this parameter?" The KMS key policy determines access to SecureString values. | P1 |
| Parameter History | `ssm:GetParameterHistory` with `Name` — shows all versions, who changed each, and when. Not a resource, but critical operational data. SSM stores up to 100 versions. | "When was this parameter last changed and by whom?" Built-in audit trail without needing CloudTrail. | P1 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| PutParameter | "Who changed this parameter's value?" Parameter changes can break applications that read them at startup or periodically. Shows the actor and parameter type (String, SecureString, StringList). |
| DeleteParameter | "Who deleted this parameter?" Applications that reference it will fail on next read. |
| GetParameter / GetParameters | "Who accessed this parameter?" For SecureString parameters, this is a security audit event similar to Secrets Manager's GetSecretValue. |
