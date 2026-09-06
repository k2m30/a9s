# Attention Signals — Golden Contract

Per registered AWS resource type, this doc defines the read-only signals
that a production SRE cares about: degraded, failing, stopped, stale,
misconfigured-dangerously, cost-bleeding.

Signals are organized by the cost of observing them:

- **Wave 1** — available from the *list* API response (or pure
  computation over it, or cross-reference against already-loaded
  sibling-type lists). Zero extra API calls. **Per-resource `Describe*`
  is Wave 2, not Wave 1.**
- **Wave 2** — requires an extra read-only API call, bounded: one
  account-wide call + a small constant number of calls per resource.
- **Wave 3** — exceeds Wave 2's budget: CloudWatch metrics per resource,
  many calls per resource, async polling, external systems.

State-value buckets: **Healthy**, **Warning**, **Broken**, **Dim**
(terminal/admin-off/informational).

## Visualization Surfaces

Every signal in this contract must land on one or more of these five
existing surfaces. No other UI is allowed. This table is the master
definition; `docs/resources/<shortName>.md` §4 transcribes it verbatim.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count + list frame title `!N` suffix | Aggregated count of `!`-severity findings. `~` findings do not bump. The list frame title appends a space-separated `!N` after the count parentheses when the current list has N > 0 issues (`s3(50+) !5`, `ec2(17) !1`), or `!N+` when N is a truncated lower bound; N uses the same aggregation as the menu badge (Wave 1 issue-colored rows + Wave 2 `!`-severity findings). No suffix when N = 0, and omitted in attention-only mode (`ctrl+z`) — the filtered count already is the issue count, so `name(5 of 50+) [!]` stays as-is. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing" — e.g. maintenance scheduled, certificate expiring soon. `!` = important background concern, `~` = informational. **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause (e.g. `stopping: Server.SpotInstanceShutdown`, `expires in 7d`). **Healthy rows render blank** — no `OK` / `available` / `ACTIVE` / `running`. Empty means "nothing to see." |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. No ceremonial header. |

### S1 — list frame title issue count

S1 has two surfaces: the main-menu `issues:N` badge and the
resource-list frame title. The frame-title rules:

- The title appends a space-separated `!N` after the count parentheses
  when the current list has N > 0 issues: `s3(50+) !5`, `ec2(17) !1`.
- When N is a truncated lower bound, the suffix renders `!N+` —
  mirroring the menu badge's truncated `N+` form.
- N uses the same aggregation as the menu badge: Wave 1 issue-colored
  rows plus Wave 2 `!`-severity findings for the resources in the list.
  `~` findings do not bump.
- Attention-only mode (`ctrl+z`): the existing `[!]` title suffix
  stays and `!N` is omitted — the filtered count already is the issue
  count; `name(5 of 50+) [!]` renders exactly as today.
- Healthy list (N = 0): no suffix — the title is unchanged.

## Signals

Every registered finding, by category, generated from the catalog by
`cmd/catalogen` — run `go run ./cmd/catalogen` after any `FindingDef` change
and `make check-catalogen` will confirm it. The API each signal reads is cited
per type in `docs/resources/<shortName>.md` §3, which is where a batch adds its
citation.

<!-- BEGIN GENERATED: signals -->

### Compute

| shortName | Name | Wave | Code | Phrase | Severity | Detail |
| --- | --- | --- | --- | --- | --- | --- |
| `ec2` | EC2 Instances | wave1 | `ec2.state.pending` | pending | warn | — |
| `ec2` | EC2 Instances | wave1 | `ec2.state.shutting-down` | shutting down | warn | — |
| `ec2` | EC2 Instances | wave1 | `ec2.state.stopping` | stopping | warn | — |
| `ec2` | EC2 Instances | wave1 | `ec2.state.stopped` | stopped | warn | — |
| `ec2` | EC2 Instances | wave1 | `ec2.state.stopped.server` | stopped | broken | — |
| `ec2` | EC2 Instances | wave1 | `ec2.state.terminated` | terminated | dim | — |
| `ec2` | EC2 Instances | wave2 | `ec2.instance-status-impaired` | impaired: system checks failing | broken | AWS reports this instance is impaired — system or instance status checks are failing. |
| `ec2` | EC2 Instances | wave2 | `ec2.instance-status.initializing` | initializing: checks in progress | warn | Instance status checks have not yet passed since start. |
| `ec2` | EC2 Instances | wave2 | `ec2.instance-status.insufficient-data` | status unknown: AWS insufficient-data | warn | AWS cannot determine status — insufficient data from the hypervisor. |
| `ec2` | EC2 Instances | wave2 | `ec2.scheduled-event` | scheduled event: <code> at <date> | warn | — |
| `ec2` | EC2 Instances | wave1 | `ec2.imdsv1-allowed` | IMDSv1 allowed | warn | Instance metadata answers requests without a session token, so an SSRF bug on this host can read the attached IAM role's credentials. Require session tokens for instance metadata. |
| `ec2` | EC2 Instances | wave1 | `ec2.public-ip` | public address | warn | The instance holds a routable public address, so every port its security groups leave open is reachable from the internet. Put it behind a NAT gateway or load balancer unless it must be addressed directly. |
| `ec2` | EC2 Instances | wave2 | `ec2.internet-exposed` | port(s) <list> reachable from the internet | broken | Sensitive ports on this instance answer from any address on the internet, so the services behind them are exposed to untargeted scanning. Narrow the security group's ingress rules to known CIDRs or reach the host through a bastion. |
| `ec2` | EC2 Instances | wave2 | `ec2.user-data-secret` | credential in user data | broken | A credential is stored in this instance's user data, which every principal holding ec2:DescribeInstanceAttribute can read. Move the value into Secrets Manager or Systems Manager Parameter Store and rotate it. |
| `ecs-svc` | ECS Services | wave1 | `ecs-svc.state.inactive` | inactive | broken | — |
| `ecs-svc` | ECS Services | wave1 | `ecs-svc.state.draining` | draining | warn | — |
| `ecs-svc` | ECS Services | wave1 | `ecs-svc.tasks.none-running` | no tasks running | broken | The service is asking for tasks and none of them are running, so it is serving nothing. Read the service's events and the stopped tasks' reasons — an image pull failure, a failing health check or no capacity in the cluster are the usual causes. |
| `ecs-svc` | ECS Services | wave1 | `ecs-svc.tasks.below-desired` | running below desired count | warn | Fewer tasks are running than the service asks for, so it is carrying its traffic on reduced capacity. Read the service's events for placement failures and check the cluster has room for the missing tasks. |
| `ecs-svc` | ECS Services | wave2 | `ecs-svc.deployment-failed` | deployment failed | broken | This service is not running the tasks it was asked to run: a deployment failed, the tasks cannot be placed, or the load balancer is failing their health checks. Read the service's events and task-stopped reasons to find which, then fix the task definition, the capacity, or the health check that is rejecting them. |
| `ecs-svc` | ECS Services | wave2 | `ecs-svc.public-ip` | tasks get public IPs | warn | Every task this service launches gets its own routable public address, so each one is reachable from the internet on whatever its security groups leave open. Turn off public address assignment on the service and reach the tasks through a load balancer or NAT gateway. |
| `ecs` | ECS Clusters | wave1 | `ecs.state.provisioning` | provisioning | warn | — |
| `ecs` | ECS Clusters | wave1 | `ecs.state.deprovisioning` | deprovisioning | warn | — |
| `ecs` | ECS Clusters | wave1 | `ecs.state.failed` | failed | broken | — |
| `ecs` | ECS Clusters | wave1 | `ecs.state.inactive` | inactive | broken | — |
| `ecs` | ECS Clusters | wave2 | `ecs.cluster-issue` | <N> pending tasks | warn | — |
| `ecs-task` | ECS Tasks | wave1 | `ecs-task.state.provisioning` | provisioning | warn | — |
| `ecs-task` | ECS Tasks | wave1 | `ecs-task.state.pending` | pending | warn | — |
| `ecs-task` | ECS Tasks | wave1 | `ecs-task.state.activating` | activating | warn | — |
| `ecs-task` | ECS Tasks | wave1 | `ecs-task.state.deactivating` | deactivating | warn | — |
| `ecs-task` | ECS Tasks | wave1 | `ecs-task.state.stopping` | stopping | warn | — |
| `ecs-task` | ECS Tasks | wave1 | `ecs-task.state.deprovisioning` | deprovisioning | warn | — |
| `ecs-task` | ECS Tasks | wave1 | `ecs-task.state.stopped` | stopped | dim | — |
| `ecs-task` | ECS Tasks | wave1 | `ecs-task.stop-code.failed` | stopped: <stop code> | broken | — |
| `ecs-task` | ECS Tasks | wave1 | `ecs-task.health.unhealthy` | unhealthy | broken | — |
| `ecs-task` | ECS Tasks | wave2 | `ecs-task.task-failed` | <stop code or container> failed | broken | — |
| `ecs-task` | ECS Tasks | wave2 | `ecs-task.privileged` | privileged container | broken | A container in this task runs privileged, so it holds the host's full device and kernel-capability set and a container escape becomes a host compromise. Drop the privileged flag and grant only the specific Linux capabilities the workload needs. |
| `ecs-task` | ECS Tasks | wave2 | `ecs-task.host-namespace` | shares the host network or process namespace | warn | This task shares the host's network or process namespace, so its containers can see and reach every other process and loopback service on that instance. Switch the task definition to the awsvpc network mode and leave the process-namespace setting unset. |
| `ecs-task` | ECS Tasks | wave2 | `ecs-task.writable-root` | writable root filesystem | warn | A container in this task can write to its own root filesystem, so anything that lands code on it persists for the life of the task. Make the container's root filesystem read-only and mount a volume for the paths it genuinely writes. |
| `ecs-task` | ECS Tasks | wave2 | `ecs-task.no-logging` | container without log driver | warn | A container in this task has no log driver, so its stdout and stderr are discarded and nothing survives the task stopping. Give the container a log driver pointing at awslogs or your log router. |
| `ecs-task` | ECS Tasks | wave2 | `ecs-task.env-secret` | credential in container environment | broken | A credential is stored as a plaintext environment variable in this task definition, readable by anyone who can call ecs:DescribeTaskDefinition. Move the value to Secrets Manager or Systems Manager Parameter Store and reference it through the container's \`secrets\` block. |
| `lambda` | Lambda Functions | wave1 | `lambda.last-update.failed` | last update failed to apply | broken | — |
| `lambda` | Lambda Functions | wave1 | `lambda.runtime.deprecated` | runtime is end-of-life | broken | — |
| `lambda` | Lambda Functions | wave1 | `lambda.state.pending` | pending | warn | — |
| `lambda` | Lambda Functions | wave1 | `lambda.state.failed` | failed | broken | — |
| `lambda` | Lambda Functions | wave1 | `lambda.state.inactive` | inactive, evicted after extended idle time | dim | — |
| `lambda` | Lambda Functions | wave1 | `lambda.dlq.missing` | no dead-letter queue configured | warn | — |
| `lambda` | Lambda Functions | wave1 | `lambda.env-secret` | credential in environment variables | broken | A credential is stored as a plaintext environment variable on this function, readable by anyone who can call lambda:GetFunctionConfiguration. Move the value to Secrets Manager or Systems Manager Parameter Store, read it at cold start, and rotate the exposed one. |
| `lambda` | Lambda Functions | wave2 | `lambda.public-policy` | invokable by anyone | broken | The function's resource policy allows a wildcard principal, so any AWS caller can invoke it and whatever it does downstream runs on your account's bill and permissions. Replace the \`\*\` principal with the specific account, service, or ARN that should be allowed to call it. |
| `lambda` | Lambda Functions | wave2 | `lambda.function-url-public` | function endpoint open without authentication | broken | The function has a web endpoint that requires no authentication, so anyone on the internet who learns the address can invoke it without credentials. Set the endpoint to require signed requests, or put an authorizing layer in front of it. |
| `asg` | Auto Scaling Groups | wave1 | `asg.state.deleting` | delete in progress | warn | — |
| `asg` | Auto Scaling Groups | wave1 | `asg.instances.underprovisioned` | <N> of <M> instances in service | broken | — |
| `asg` | Auto Scaling Groups | wave1 | `asg.instances.unhealthy` | <N> unhealthy instance(s) | warn | — |
| `asg` | Auto Scaling Groups | wave1 | `asg.scaling.suspended` | scaling suspended | warn | — |
| `asg` | Auto Scaling Groups | wave2 | `asg.scaling-activity-failed` | latest scaling activity failed | broken | — |
| `asg` | Auto Scaling Groups | wave1 | `asg.launch-config.legacy` | uses a launch configuration | warn | The group launches from a launch configuration, an immutable legacy resource AWS no longer develops — it cannot carry IMDSv2 defaults, newer instance types, or versioned edits. Copy it to a launch template and point the group at that. |
| `asg` | Auto Scaling Groups | wave1 | `asg.single-az` | single availability zone | warn | Every instance in this group sits in one availability zone, so a single zone failure takes the whole group down. Add subnets from at least one more zone to the group. |
| `asg` | Auto Scaling Groups | wave1 | `asg.no-elb-health-check` | no load balancer health check | warn | The group is behind a load balancer but only watches EC2 status checks, so an instance whose application has stopped answering stays in service. Set the group's health check type to the load balancer's. |
| `asg` | Auto Scaling Groups | wave2 | `asg.launch-config.imdsv1` | launch configuration allows IMDSv1 | warn | Instances this group launches answer metadata requests without a session token, so an SSRF bug on any of them leaks the attached role's credentials. Launch configurations cannot be edited — copy this one to a launch template that requires session tokens and repoint the group. |
| `asg` | Auto Scaling Groups | wave2 | `asg.launch-config.public-ip` | launch configuration assigns public IPs | warn | Every instance this group launches gets a routable public address, so each new instance is reachable from the internet on whatever its security groups leave open. Copy the launch configuration to a launch template with public address assignment off. |
| `asg` | Auto Scaling Groups | wave2 | `asg.launch-config.secret` | credential in launch configuration user data | broken | A credential is pasted into the launch configuration's user data, so it is readable by anyone who can call autoscaling:DescribeLaunchConfigurations and lands on every instance the group starts. Move the value to Secrets Manager or Systems Manager Parameter Store and rotate it. |
| `ebs` | EBS Volumes | wave1 | `ebs.state.creating` | creating | warn | — |
| `ebs` | EBS Volumes | wave1 | `ebs.state.error` | error | broken | — |
| `ebs` | EBS Volumes | wave1 | `ebs.orphan-unattached` | orphan: unattached Nd | warn | The volume has been unattached since it was created, so it is billed hourly for no workload; the age is in the status. Snapshot it if the data matters, then delete it. |
| `ebs` | EBS Volumes | wave1 | `ebs.encryption.disabled` | unencrypted | warn | Volume is not encrypted at rest — re-create from encrypted snapshot. |
| `ebs` | EBS Volumes | wave2 | `ebs.volume-io-degraded` | volume I/O degraded | broken | — |
| `ebs` | EBS Volumes | wave2 | `ebs.not-in-backup-plan` | not covered by a backup plan | warn | No backup plan selects this volume, so nothing is scheduled to copy it and a deletion is final. Add it to a plan by ARN, or give it a tag one of your plans already selects on. |
| `ebs` | EBS Volumes | wave2 | `ebs.no-snapshot` | no snapshot exists | warn | This volume is attached and in use, and no snapshot of it exists, so there is no point to restore from. Take one, or put the volume in a backup plan that will. |
| `ebs-snap` | EBS Snapshots | wave1 | `ebs-snap.state.pending` | pending | warn | — |
| `ebs-snap` | EBS Snapshots | wave1 | `ebs-snap.state.error` | error | broken | — |
| `ebs-snap` | EBS Snapshots | wave1 | `ebs-snap.encryption.disabled` | unencrypted | warn | Snapshot is not encrypted at rest — re-create from an encrypted volume. |
| `ebs-snap` | EBS Snapshots | wave1 | `ebs-snap.aged-automated` | automated, <N>d old | warn | This automated snapshot is old and no retention policy prunes it, so it is billed indefinitely; the age is in the status. Add a lifecycle policy, or delete it. |
| `ebs-snap` | EBS Snapshots | wave2 | `ebs-snap.orphan` | orphan: source volume deleted | warn | — |
| `ebs-snap` | EBS Snapshots | wave2 | `ebs-snap.public` | shared with all AWS accounts | broken | This snapshot is shared with every AWS account, so anyone can restore a volume from it and read whatever the source disk held. Stop sharing the snapshot with the \`all\` group. |
| `ami` | AMIs | wave1 | `ami.state.pending` | pending | warn | — |
| `ami` | AMIs | wave1 | `ami.state.failed` | failed | broken | — |
| `ami` | AMIs | wave1 | `ami.state.dim` | deregistered | dim | — |
| `ami` | AMIs | wave1 | `ami.deprecated` | deprecated | warn | The deprecation date has passed — AWS no longer recommends this AMI for new launches. |
| `ami` | AMIs | wave1 | `ami.public` | shared with all AWS accounts | broken | This image is shared with every AWS account, so anyone can launch it and read whatever the snapshot behind it contains. Remove the \`all\` group from the image's launch permission. |
| `lt` | Launch Templates | wave1 | `lt.warn.imdsv1` | IMDSv1 allowed | warn | Instance metadata does not require session tokens; IMDSv1 credentials are exposed to SSRF. |
| `lt` | Launch Templates | wave1 | `lt.warn.unencrypted` | EBS encryption disabled | warn | A block device explicitly sets Encrypted=false; launched instances get unencrypted volumes. |
| `lt` | Launch Templates | wave2 | `lt.warn.deprecated_ami` | deprecated AMI | warn | The default version references an AMI past its deprecation time. |
| `lt` | Launch Templates | wave2 | `lt.user-data-secret` | credential in user data | broken | A credential is pasted into the default version's user data, so it is readable by anyone who can call ec2:DescribeLaunchTemplateVersions and lands on every instance launched from this template. Move the value to Secrets Manager or Systems Manager Parameter Store and rotate it. |
| `lt` | Launch Templates | wave1 | `lt.warn.details_denied` | details denied | warn | Access to the default version was denied; only the listed fields are visible. |
| `lt` | Launch Templates | wave1 | `lt.warn.details_unavailable` | details unavailable | warn | Details could not be retrieved; only the name is visible. |

### Containers

| shortName | Name | Wave | Code | Phrase | Severity | Detail |
| --- | --- | --- | --- | --- | --- | --- |
| `eks` | EKS Clusters | wave1 | `eks.state.creating` | creating | warn | — |
| `eks` | EKS Clusters | wave1 | `eks.state.updating` | updating | warn | — |
| `eks` | EKS Clusters | wave1 | `eks.state.failed` | failed | broken | The cluster is in a failed state and will not recover on its own; when AWS reports health issues, the first is the phrase and any others follow as rows. Open a support case or recreate the cluster. |
| `eks` | EKS Clusters | wave1 | `eks.health-issue` | issue: <health issue code> | warn | The control plane reports at least one health issue; the first code is the phrase, any others follow as rows, and the EKS console carries the message. Add-ons and nodes may misbehave until it clears. |
| `eks` | EKS Clusters | wave1 | `eks.public-endpoint` | cluster endpoint reachable from the internet | broken | The cluster's Kubernetes endpoint answers from the public internet, so its authentication is the only thing between the control plane and every scanner on the network. Turn off public endpoint access and reach the cluster over the VPC, or at minimum restrict public access to the office and build ranges. |
| `eks` | EKS Clusters | wave1 | `eks.control-plane-logging-off` | control plane logging incomplete | warn | Some control-plane log types are not being sent to CloudWatch, so an authentication attempt or an admission decision made during an incident leaves no record to investigate. Enable all five control-plane log types on the cluster. |
| `eks` | EKS Clusters | wave1 | `eks.secrets-not-kms` | secrets not encrypted with KMS | warn | Kubernetes secrets in this cluster are stored in etcd with only the AWS-managed default protection and no envelope encryption of their own. Attach a KMS key to the cluster's secrets encryption configuration so a copy of etcd is useless without that key. |
| `eks` | EKS Clusters | wave1 | `eks.version-unsupported` | Kubernetes <version> is out of standard support | broken | This Kubernetes minor is past standard support, so it no longer receives the full patch stream and AWS will upgrade it on its own schedule if you do not. Plan an upgrade to a version in standard support before the automatic one lands during business hours. |
| `eks` | EKS Clusters | wave1 | `eks.warn.details_denied` | details denied | warn | Access to resource details was denied; only the name is visible. |
| `eks` | EKS Clusters | wave1 | `eks.warn.details_unavailable` | details unavailable | warn | Details could not be retrieved; only the name is visible. |
| `ng` | EKS Node Groups | wave1 | `ng.state.creating` | creating | warn | — |
| `ng` | EKS Node Groups | wave1 | `ng.state.updating` | updating | warn | — |
| `ng` | EKS Node Groups | wave1 | `ng.state.deleting` | deleting | warn | — |
| `ng` | EKS Node Groups | wave1 | `ng.state.create-failed` | create failed | broken | — |
| `ng` | EKS Node Groups | wave1 | `ng.state.delete-failed` | delete failed | broken | — |
| `ng` | EKS Node Groups | wave1 | `ng.state.degraded` | degraded | broken | The node group is degraded, so some nodes are failing or not joining; when AWS reports health issues, the first is the phrase and any others follow as rows. Fix the cause, usually IAM, subnet capacity or the launch template, and let the group reconcile. |
| `ng` | EKS Node Groups | wave1 | `ng.warn.details_denied` | details denied | warn | Access to resource details was denied; only the name is visible. |
| `ng` | EKS Node Groups | wave1 | `ng.warn.details_unavailable` | details unavailable | warn | Details could not be retrieved; only the name is visible. |

### Networking

| shortName | Name | Wave | Code | Phrase | Severity | Detail |
| --- | --- | --- | --- | --- | --- | --- |
| `elb` | Load Balancers | wave1 | `elb.state.provisioning` | provisioning | warn | The load balancer is still being built and is not yet accepting traffic. This normally clears in a few minutes; if it does not, its subnets are usually out of free IP addresses. |
| `elb` | Load Balancers | wave1 | `elb.state.active_impaired` | active impaired | warn | The load balancer is serving traffic but could not set up or scale in at least one availability zone, so capacity there is degraded. Check that every attached subnet has spare IP addresses. |
| `elb` | Load Balancers | wave1 | `elb.state.failed` | failed | broken | The load balancer could not be created and will not recover on its own. It has to be deleted and recreated; nothing routes through it in the meantime. |
| `elb` | Load Balancers | wave2 | `elb.misconfigured` | deletion protection disabled | warn | — |
| `elb` | Load Balancers | wave2 | `elb.desync-mitigation-off` | HTTP desync mitigation off | warn | The load balancer forwards requests it knows are ambiguous instead of rejecting them, so a crafted request can be interpreted one way by the balancer and another by the target. Set the desync mitigation mode to defensive or strictest. |
| `elb` | Load Balancers | wave2 | `elb.invalid-headers-kept` | invalid HTTP headers not dropped | warn | Headers that are not valid HTTP are passed through to the targets instead of being dropped, which is how request smuggling reaches an application. Turn on dropping of invalid header fields. |
| `elb` | Load Balancers | wave2 | `elb.plain-http-listener` | <ports> in the clear | warn | A listener on this load balancer carries traffic in the clear, so credentials and session cookies cross the network readable by anyone on the path; the ports are listed below. Terminate TLS on the listener, or redirect it to an HTTPS listener. |
| `elb` | Load Balancers | wave2 | `elb.weak-tls-policy` | weak TLS policy on <ports> | warn | A listener's security policy still negotiates older protocol versions or ciphers without forward secrecy, so a client can be steered onto a breakable connection; the ports are listed below. Move the listener to one of the modern security policies that require version 1.2 or later. |
| `tg` | Target Groups | wave2 | `tg.unhealthy-targets` | unhealthy targets: <N>/<M> | broken | — |
| `sg` | Security Groups | wave1 | `sg.ingress.wide-open` | all ports open to 0.0.0.0/0 | broken | One ingress rule opens every port and protocol to the whole internet, so nothing this group protects is reachable only from where you intended. Replace it with rules naming the ports each workload actually serves and the addresses allowed to reach them. |
| `sg` | Security Groups | wave1 | `sg.ingress.dangerous-ports` | ports <list> open to 0.0.0.0/0 | broken | An administrative or database port on this group accepts connections from any address on the internet, which is how credential-stuffing and direct database access start. Narrow the rule to the addresses that need it, or move the access behind a bastion or private link. |
| `sg` | Security Groups | wave1 | `sg.default-with-rules` | default group allows traffic | warn | The VPC's default security group still carries rules, and AWS attaches it to any resource launched without an explicit group. Remove every ingress rule and every egress rule other than the AWS-created allow-all, and give each workload its own group. |
| `sg` | Security Groups | wave2 | `sg.unused` | not attached to anything | warn | No network interface in this account references this group, so its rules protect nothing and its name still gets picked from the console list. Delete it, or attach it to the workload it was written for. |
| `vpc` | VPCs | wave1 | `vpc.state.pending` | pending | warn | — |
| `vpc` | VPCs | wave2 | `vpc.no-flow-logs` | no active VPC flow logs | warn | — |
| `subnet` | Subnets | wave1 | `subnet.state.pending` | pending | warn | — |
| `subnet` | Subnets | wave1 | `subnet.state.unavailable` | unavailable | broken | — |
| `subnet` | Subnets | wave1 | `subnet.state.failed` | failed | broken | — |
| `subnet` | Subnets | wave1 | `subnet.state.failed-insufficient-capacity` | failed-insufficient-capacity | broken | — |
| `subnet` | Subnets | wave1 | `subnet.auto-public-ip` | auto-assigns public IPs | warn | Every instance launched into this subnet is given a public address by default, so a workload reaches the internet whether or not its owner intended it to. Turn the subnet's auto-assign public address setting off and attach an elastic address to the instances that genuinely need one. |
| `rtb` | Route Tables | wave1 | `rtb.route.blackhole` | blackhole route (target deleted) | broken | — |
| `rtb` | Route Tables | wave1 | `rtb.orphan-unassociated` | no subnet associations | warn | — |
| `nat` | NAT Gateways | wave1 | `nat.state.pending` | pending | warn | — |
| `nat` | NAT Gateways | wave1 | `nat.state.deleting` | deleting | warn | — |
| `nat` | NAT Gateways | wave1 | `nat.state.failed` | failed | broken | — |
| `nat` | NAT Gateways | wave1 | `nat.state.deleted` | deleted | dim | — |
| `igw` | Internet Gateways | wave1 | `igw.state.attaching` | attaching | warn | — |
| `igw` | Internet Gateways | wave1 | `igw.state.detaching` | detaching | warn | — |
| `igw` | Internet Gateways | wave1 | `igw.no-attachments` | no VPC attachments | warn | — |
| `eip` | Elastic IPs | wave1 | `eip.unassociated` | unassociated | warn | — |
| `vpce` | VPC Endpoints | wave1 | `vpce.state.pending_acceptance` | pending acceptance | warn | — |
| `vpce` | VPC Endpoints | wave1 | `vpce.state.pending` | pending | warn | — |
| `vpce` | VPC Endpoints | wave1 | `vpce.state.deleting` | deleting | warn | — |
| `vpce` | VPC Endpoints | wave1 | `vpce.state.failed` | failed | broken | — |
| `vpce` | VPC Endpoints | wave1 | `vpce.state.rejected` | rejected | broken | — |
| `vpce` | VPC Endpoints | wave1 | `vpce.state.expired` | expired | broken | — |
| `vpce` | VPC Endpoints | wave1 | `vpce.state.partial` | partial | broken | — |
| `vpce` | VPC Endpoints | wave1 | `vpce.state.deleted` | deleted | dim | — |
| `vpce` | VPC Endpoints | wave1 | `vpce.policy-open` | endpoint policy open to anyone | warn | The endpoint policy grants every action to every principal, so any identity that can reach this endpoint can use it to talk to resources in other accounts. Replace it with a policy naming the principals and resources this VPC is allowed to reach. |
| `tgw` | Transit Gateways | wave1 | `tgw.state.pending` | pending | warn | The gateway is still being created and does not route yet. Attachments created now stay pending until it comes up. |
| `tgw` | Transit Gateways | wave1 | `tgw.state.modifying` | modifying | warn | A configuration change is being applied. Routing across the gateway can be inconsistent until it settles. |
| `tgw` | Transit Gateways | wave1 | `tgw.state.deleting` | deleting | warn | The gateway is being torn down. Every attachment on it goes away and any traffic still routed through it will stop. |
| `tgw` | Transit Gateways | wave1 | `tgw.state.failed` | failed | broken | The gateway could not be created and will not recover. It has to be recreated, and anything routed through it has no path. |
| `tgw` | Transit Gateways | wave1 | `tgw.state.deleted` | deleted | dim | This gateway is gone. AWS keeps returning it for a while after deletion, so route tables that still point at it are dead references worth cleaning up. |
| `tgw` | Transit Gateways | wave2 | `tgw.attachment-failed` | attachment <id> failed | broken | The network behind this attachment has no path across the gateway. Failed attachments do not retry; delete and recreate the attachment. |
| `tgw` | Transit Gateways | wave2 | `tgw.attachment-transitional` | attachment <id> <state> | warn | The attachment is between states — being modified, rolled back, or waiting for the gateway owner to accept it — and traffic across it is not reliable until it settles. Pending acceptance is the one state that needs a person: the owning account has to approve it. |
| `tgw` | Transit Gateways | wave1 | `tgw.auto-accept-attachments` | auto-accepts shared attachments | warn | Any account this gateway is shared with can attach a VPC to it without review, putting that VPC on your routed network the moment it asks. Turn auto-accept off and approve each attachment explicitly. |
| `eni` | Network Interfaces | wave1 | `eni.state.attaching` | attaching | warn | — |
| `eni` | Network Interfaces | wave1 | `eni.state.detaching` | detaching | warn | — |
| `eni` | Network Interfaces | wave1 | `eni.state.available` | available | warn | — |
| `transfer` | Transfer Family | wave1 | `transfer.warn.offline` | offline: not accepting transfers | warn | Server is offline; partners cannot connect until it is started. |
| `transfer` | Transfer Family | wave1 | `transfer.warn.starting` | starting | warn | Server is starting; not yet fully able to respond. |
| `transfer` | Transfer Family | wave1 | `transfer.warn.stopping` | stopping | warn | Server is stopping; transfers are draining. |
| `transfer` | Transfer Family | wave1 | `transfer.broken.start_failed` | start failed | broken | Server failed to come online; partner transfers are down. |
| `transfer` | Transfer Family | wave1 | `transfer.warn.stop_failed` | stop failed | warn | Stop failed; the server may still be serving transfers. |
| `transfer` | Transfer Family | wave1 | `transfer.warn.legacy_policy` | legacy security policy | warn | The server's security policy still allows weak ciphers and old TLS versions, so a client can be steered onto a breakable connection. Move the server to a current security policy. |
| `transfer` | Transfer Family | wave1 | `transfer.warn.no_logging` | no activity logging | warn | Neither a logging role nor structured log destinations are configured. |
| `transfer` | Transfer Family | wave1 | `transfer.warn.details_denied` | details denied | warn | Access to server details was denied; only the listed fields are visible. |
| `transfer` | Transfer Family | wave1 | `transfer.warn.details_unavailable` | details unavailable | warn | Details could not be retrieved; only the name is visible. |
| `vpc-peer` | VPC Peering | wave1 | `vpc-peer.warn.provisioning` | provisioning | warn | Peering connection is being provisioned. |
| `vpc-peer` | VPC Peering | wave1 | `vpc-peer.warn.initiating` | initiating | warn | Peering request is being initiated. |
| `vpc-peer` | VPC Peering | wave1 | `vpc-peer.warn.pending_acceptance` | pending acceptance: expires in <N>d | warn | The peer has not accepted this request yet, and AWS expires it a week after creation; the countdown is in the status and the date is listed below. Ask the accepter to approve it. |
| `vpc-peer` | VPC Peering | wave1 | `vpc-peer.warn.expired` | expired: never accepted | warn | The peering request expired unaccepted; recreate it if still needed. |
| `vpc-peer` | VPC Peering | wave1 | `vpc-peer.broken.rejected` | rejected | broken | The accepter rejected this peering request, so nothing will ever route across it; AWS keeps the record listed for a while. Delete it and request again once the other side agrees. |
| `vpc-peer` | VPC Peering | wave1 | `vpc-peer.broken.failed` | failed | broken | The peering connection failed to establish and will not recover on its own; the status message is listed below. Delete it and request a new one. |
| `vpc-peer` | VPC Peering | wave1 | `vpc-peer.warn.deleting` | deleting | warn | Peering connection is being deleted. |
| `vpc-peer` | VPC Peering | wave1 | `vpc-peer.dim.deleted` | deleted | dim | AWS keeps deleted connections listed for a window. |
| `vpc-peer` | VPC Peering | wave1 | `vpc-peer.warn.cidr_overlap` | CIDR overlap with peer | warn | The requester and accepter VPCs have overlapping address ranges, so routes into the overlap are blackholed; the range is listed below. Re-address one side, or peer a VPC that does not overlap. |
| `vpc-peer` | VPC Peering | wave2 | `vpc-peer.warn.no_local_route` | no local route to peer | warn | No loaded route table routes to this peering connection. |
| `vpc-peer` | VPC Peering | wave2 | `vpc-peer.warn.route_blackholed` | route to peer blackholed | warn | A route references this connection but its state is blackhole. |

### Databases & storage

| shortName | Name | Wave | Code | Phrase | Severity | Detail |
| --- | --- | --- | --- | --- | --- | --- |
| `dbi` | DB Instances | wave1 | `dbi.broken.failed` | failed | broken | — |
| `dbi` | DB Instances | wave1 | `dbi.broken.storage_full` | storage-full | broken | — |
| `dbi` | DB Instances | wave1 | `dbi.broken.incompatible_network` | incompatible-network | broken | — |
| `dbi` | DB Instances | wave1 | `dbi.broken.incompatible_option_group` | incompatible-option-group | broken | — |
| `dbi` | DB Instances | wave1 | `dbi.broken.incompatible_parameters` | incompatible-parameters | broken | — |
| `dbi` | DB Instances | wave1 | `dbi.broken.incompatible_restore` | incompatible-restore | broken | — |
| `dbi` | DB Instances | wave1 | `dbi.broken.restore_error` | restore-error | broken | — |
| `dbi` | DB Instances | wave1 | `dbi.broken.encryption_key_unavailable` | encryption key unavailable | broken | — |
| `dbi` | DB Instances | wave1 | `dbi.broken.stopped` | stopped | broken | — |
| `dbi` | DB Instances | wave1 | `dbi.warn.transitional` | <status>: <pending field> | warn | — |
| `dbi` | DB Instances | wave1 | `dbi.warn.no_automated_backups` | no automated backups | warn | — |
| `dbi` | DB Instances | wave1 | `dbi.warn.publicly_accessible` | publicly accessible | warn | — |
| `dbi` | DB Instances | wave1 | `dbi.warn.unencrypted_storage` | unencrypted storage | warn | — |
| `dbi` | DB Instances | wave1 | `dbi.warn.deletion_protection_off` | deletion protection off | warn | — |
| `dbi` | DB Instances | wave2 | `dbi.pending-maintenance` | maintenance scheduled | warn | AWS has a maintenance action pending for this instance and will apply it in a maintenance window of its choosing once the target date passes; the action, apply method and earliest date are listed below. Apply it yourself in a window that suits you. |
| `dbi` | DB Instances | wave1 | `dbi.single-az` | single-AZ | warn | The instance runs in one Availability Zone, so an AZ failure takes the database down until you restore it. Enable Multi-AZ to keep a synchronous standby in a second AZ. |
| `dbi` | DB Instances | wave1 | `dbi.minor-upgrade-off` | auto minor version upgrade off | warn | Minor engine patches — including security fixes — are never applied automatically. Enable auto minor version upgrade, or schedule the patching yourself. |
| `dbi` | DB Instances | wave2 | `dbi.not-in-backup-plan` | not covered by a backup plan | warn | No backup plan selects this database, so its retention is whatever the instance's own automated backups happen to be. Add it to a plan by ARN, or give it a tag one of your plans already selects on. |
| `dbi` | DB Instances | wave1 | `dbi.iam-auth-off` | IAM database authentication off | warn | Connections authenticate with long-lived database passwords only. Enable IAM database authentication so credentials become short-lived tokens tied to IAM identities. |
| `dbi` | DB Instances | wave1 | `dbi.default-master-user` | default master username | warn | The administrative account uses the vendor default name, so an attacker only has to guess the password. Create a differently-named administrative user and retire this one. |
| `dbi` | DB Instances | wave1 | `dbi.ca-cert-expiring` | server certificate expires in <N> days | warn | The server certificate expires soon; clients that verify the connection will refuse to talk to it once it does. Rotate the instance onto the current certificate authority during a maintenance window. |
| `dbi` | DB Instances | wave2 | `dbi.engine-deprecated` | engine version deprecated | broken | AWS no longer supports this engine version, so it stops receiving security patches and will be force-upgraded on AWS's schedule. Upgrade to a supported version during a maintenance window of your choosing. |
| `s3` | S3 Buckets | wave2 | `s3.public-access-block-incomplete` | public access block incomplete | warn | Bucket-level public access block is missing or partial — account-level PAB may still apply. |
| `s3` | S3 Buckets | wave2 | `s3.public` | publicly accessible | broken | AWS reports this bucket's policy as public, so anyone on the internet can reach its objects. Remove the wildcard-principal statements from the bucket policy, or block them with a public access block. |
| `s3` | S3 Buckets | wave2 | `s3.versioning-off` | versioning off | warn | Overwritten and deleted objects are gone for good — there is no previous version to restore. Enable versioning on the bucket. |
| `s3` | S3 Buckets | wave2 | `s3.mfa-delete-off` | MFA delete off | warn | Versioning is on, but anyone holding the delete permission can still remove versions permanently. Enable MFA delete so destroying a version needs a second factor. |
| `s3` | S3 Buckets | wave2 | `s3.access-logging-off` | access logging off | warn | Nothing records who read or wrote objects here, so an incident leaves no trail to follow. Point server access logging at a log destination bucket. |
| `s3` | S3 Buckets | wave2 | `s3.no-lifecycle` | no lifecycle rules | warn | No lifecycle rule expires or transitions objects, so data and cost accumulate indefinitely. Add a lifecycle rule matching the bucket's retention policy. |
| `s3` | S3 Buckets | wave2 | `s3.no-object-lock` | object lock off | warn | Objects can be overwritten or deleted by anyone with write access — nothing enforces retention. Enable object lock on a new bucket and migrate if the data is compliance-relevant. |
| `redis` | ElastiCache Redis | wave1 | `redis.broken.create_failed` | create failed — see events | broken | — |
| `redis` | ElastiCache Redis | wave1 | `redis.warn.creating` | creating — new group | warn | — |
| `redis` | ElastiCache Redis | wave1 | `redis.warn.deleting` | deleting — teardown | warn | — |
| `redis` | ElastiCache Redis | wave1 | `redis.warn.modifying` | modifying — config change | warn | — |
| `redis` | ElastiCache Redis | wave1 | `redis.warn.snapshotting` | snapshotting — backup running | warn | — |
| `redis` | ElastiCache Redis | wave1 | `redis.warn.shard_issue` | shard <NodeGroupId>: <status> | warn | — |
| `redis` | ElastiCache Redis | wave1 | `redis.warn.multiaz_without_auto_failover` | multi-AZ without auto-failover | warn | — |
| `redis` | ElastiCache Redis | wave1 | `redis.encryption-at-rest-off` | encryption at rest off | warn | Cached data is written to disk and to backups unencrypted. Encryption at rest can only be turned on at creation time — recreate the replication group with it enabled and migrate. |
| `redis` | ElastiCache Redis | wave1 | `redis.encryption-in-transit-off` | encryption in transit off | warn | Client traffic to this group crosses the network in cleartext, so anyone with VPC access can read the cached data. Enable in-transit encryption on the replication group. |
| `redis` | ElastiCache Redis | wave1 | `redis.no-auth` | no authentication token | broken | The group accepts any client that can reach it — encryption in transit is on but no authentication token is required. Set one, so a network-level reachability mistake is not immediately a data breach. |
| `redis` | ElastiCache Redis | wave1 | `redis.no-backup` | automatic backups off | warn | Automatic backups are off, so a failed replication group takes its data with it. Set a snapshot retention limit of at least one day. |
| `dbc` | DB Clusters | wave1 | `dbc.broken.failed` | failed: cluster operation | broken | — |
| `dbc` | DB Clusters | wave1 | `dbc.broken.encryption_key_unreachable` | encryption key unreachable | broken | — |
| `dbc` | DB Clusters | wave1 | `dbc.broken.incompatible_parameters` | parameter group incompatible | broken | — |
| `dbc` | DB Clusters | wave1 | `dbc.broken.no_writer` | no writer: reads only | broken | — |
| `dbc` | DB Clusters | wave1 | `dbc.warn.transitional` | <status>: in progress | warn | — |
| `dbc` | DB Clusters | wave1 | `dbc.warn.deletion_protection_off` | delete-protection off | warn | — |
| `dbc` | DB Clusters | wave1 | `dbc.warn.not_encrypted_at_rest` | not encrypted at rest | warn | — |
| `dbc` | DB Clusters | wave1 | `dbc.warn.no_automated_backups` | no automated backups | warn | — |
| `dbc` | DB Clusters | wave2 | `dbc.maintenance-overdue` | maintenance overdue | broken | — |
| `dbc` | DB Clusters | wave1 | `dbc.single-az` | single-AZ | warn | The cluster has no instance in a second Availability Zone, so an AZ failure takes it down until you restore it. Add a replica in another AZ. |
| `dbc` | DB Clusters | wave1 | `dbc.minor-upgrade-off` | auto minor version upgrade off | warn | Minor engine patches — including security fixes — are never applied automatically. Enable auto minor version upgrade, or schedule the patching yourself. |
| `dbc` | DB Clusters | wave1 | `dbc.iam-auth-off` | IAM database authentication off | warn | Connections authenticate with long-lived database passwords only. Enable IAM database authentication so credentials become short-lived tokens tied to IAM identities. |
| `dbc` | DB Clusters | wave1 | `dbc.default-master-user` | default master username | warn | The administrative account uses the vendor default name, so an attacker only has to guess the password. Create a differently-named administrative user and retire this one. |
| `dbc` | DB Clusters | wave2 | `dbc.not-in-backup-plan` | not covered by a backup plan | warn | No backup plan selects this cluster, so its retention is whatever the cluster's own automated backups happen to be. Add it to a plan by ARN, or give it a tag one of your plans already selects on. |
| `ddb` | DynamoDB Tables | wave1 | `ddb.broken.kms_key_inaccessible` | kms key inaccessible | broken | — |
| `ddb` | DynamoDB Tables | wave1 | `ddb.broken.archived_kms_lost` | archived: kms key lost | broken | — |
| `ddb` | DynamoDB Tables | wave1 | `ddb.warn.creating` | creating | warn | — |
| `ddb` | DynamoDB Tables | wave1 | `ddb.warn.updating` | updating | warn | — |
| `ddb` | DynamoDB Tables | wave1 | `ddb.warn.deleting` | deleting | warn | — |
| `ddb` | DynamoDB Tables | wave1 | `ddb.warn.archiving` | archiving | warn | — |
| `ddb` | DynamoDB Tables | wave2 | `ddb.pitr-off` | point-in-time recovery disabled | warn | — |
| `ddb` | DynamoDB Tables | wave1 | `ddb.deletion-protection-off` | deletion protection off | warn | A single delete call (DeleteTable) destroys this table and its data. Turn on deletion protection so removing it takes a deliberate second step. |
| `ddb` | DynamoDB Tables | wave2 | `ddb.cross-account-policy` | resource policy grants another account | warn | The table's resource policy grants access to an AWS account outside this one. Confirm each account belongs to a partner you meant to share with, and remove the rest. |
| `ddb` | DynamoDB Tables | wave2 | `ddb.public-policy` | resource policy open to anyone | broken | The table's resource policy allows any AWS principal, so anyone with an AWS account can reach it. Replace the wildcard principal with the specific roles that need access. |
| `ddb` | DynamoDB Tables | wave2 | `ddb.not-in-backup-plan` | not covered by a backup plan | warn | No backup plan selects this table, so nothing is scheduled to copy it and point-in-time recovery alone will not survive the table being deleted. Add it to a plan by ARN, or give it a tag one of your plans already selects on. |
| `ddb` | DynamoDB Tables | wave1 | `ddb.warn.details_denied` | details denied | warn | Access to resource details was denied; only the name is visible. |
| `ddb` | DynamoDB Tables | wave1 | `ddb.warn.details_unavailable` | details unavailable | warn | Details could not be retrieved; only the name is visible. |
| `opensearch` | OpenSearch Domains | wave1 | `opensearch.dim.deleting` | deleting: removal in progress | dim | — |
| `opensearch` | OpenSearch Domains | wave1 | `opensearch.broken.isolated` | isolated: quarantined by AWS | broken | — |
| `opensearch` | OpenSearch Domains | wave1 | `opensearch.warn.processing` | processing: config change in flight | warn | — |
| `opensearch` | OpenSearch Domains | wave1 | `opensearch.update-forced` | software update forced soon | warn | AWS will apply this update automatically once the scheduled date passes; upgrade on your own schedule before then to control the maintenance window. |
| `opensearch` | OpenSearch Domains | wave1 | `opensearch.encryption-off` | encryption at rest off | warn | Data at rest is stored unencrypted. Enabling encryption at rest requires creating a new domain and migrating data — it cannot be turned on in place. |
| `opensearch` | OpenSearch Domains | wave1 | `opensearch.public` | reachable outside a VPC | broken | The domain sits outside a VPC and its access policy allows any principal, so the search endpoint is reachable from the internet. Move the domain into a VPC, or scope the access policy to named principals. |
| `opensearch` | OpenSearch Domains | wave1 | `opensearch.https-not-enforced` | HTTPS not enforced | warn | The domain accepts plaintext HTTP, so queries and results can be read off the wire. Turn on Require HTTPS in the domain's endpoint options. |
| `opensearch` | OpenSearch Domains | wave1 | `opensearch.node-to-node-tls-off` | node-to-node encryption off | warn | Traffic between the domain's own nodes is unencrypted. Node-to-node encryption can only be enabled on a domain that already has it configured at creation — recreate the domain if this data is sensitive. |
| `opensearch` | OpenSearch Domains | wave1 | `opensearch.warn.details_denied` | details denied | warn | — |
| `opensearch` | OpenSearch Domains | wave1 | `opensearch.warn.details_unavailable` | details unavailable | warn | Details could not be retrieved; only the name is visible. |
| `redshift` | Redshift Clusters | wave1 | `redshift.broken.incompatible_hsm` | incompatible-hsm | broken | — |
| `redshift` | Redshift Clusters | wave1 | `redshift.broken.incompatible_network` | incompatible-network | broken | — |
| `redshift` | Redshift Clusters | wave1 | `redshift.broken.incompatible_parameters` | incompatible-parameters | broken | — |
| `redshift` | Redshift Clusters | wave1 | `redshift.broken.incompatible_restore` | incompatible-restore | broken | — |
| `redshift` | Redshift Clusters | wave1 | `redshift.broken.hardware_failure` | hardware-failure | broken | — |
| `redshift` | Redshift Clusters | wave1 | `redshift.broken.storage_full` | storage-full | broken | — |
| `redshift` | Redshift Clusters | wave1 | `redshift.broken.unavailable` | unavailable | broken | — |
| `redshift` | Redshift Clusters | wave1 | `redshift.broken.failed` | failed | broken | — |
| `redshift` | Redshift Clusters | wave1 | `redshift.warn.creating` | creating | warn | — |
| `redshift` | Redshift Clusters | wave1 | `redshift.warn.modifying` | modifying | warn | — |
| `redshift` | Redshift Clusters | wave1 | `redshift.warn.resizing` | resizing | warn | — |
| `redshift` | Redshift Clusters | wave1 | `redshift.warn.rebooting` | rebooting | warn | — |
| `redshift` | Redshift Clusters | wave1 | `redshift.warn.renaming` | renaming | warn | — |
| `redshift` | Redshift Clusters | wave1 | `redshift.warn.deleting` | deleting | warn | — |
| `redshift` | Redshift Clusters | wave1 | `redshift.warn.maintenance` | maintenance | warn | — |
| `redshift` | Redshift Clusters | wave1 | `redshift.warn.availability_modifying` | modifying | warn | — |
| `redshift` | Redshift Clusters | wave1 | `redshift.warn.pending_change` | pending change queued | warn | — |
| `redshift` | Redshift Clusters | wave1 | `redshift.warn.maintenance_deferred` | maintenance deferred | warn | — |
| `redshift` | Redshift Clusters | wave1 | `redshift.warn.publicly_accessible` | publicly accessible | warn | — |
| `redshift` | Redshift Clusters | wave1 | `redshift.warn.unencrypted_at_rest` | unencrypted at rest | warn | — |
| `redshift` | Redshift Clusters | wave2 | `redshift.audit-logging-off` | audit logging off | warn | Nothing records connections and queries against this cluster, so an incident leaves no trail to follow. Enable audit logging to an S3 bucket or a CloudWatch log group. |
| `redshift` | Redshift Clusters | wave2 | `redshift.require-ssl-off` | SSL not required | warn | The cluster accepts unencrypted client connections, so credentials and query results can be read off the wire. Set the parameter group's require-SSL parameter (require\_ssl) to true and reboot. |
| `efs` | EFS File Systems | wave1 | `efs.broken.error` | error | broken | — |
| `efs` | EFS File Systems | wave1 | `efs.broken.no_mount_targets` | no mount targets | broken | — |
| `efs` | EFS File Systems | wave1 | `efs.warn.creating` | creating | warn | — |
| `efs` | EFS File Systems | wave1 | `efs.warn.updating` | updating | warn | — |
| `efs` | EFS File Systems | wave1 | `efs.warn.deleting` | deleting | warn | — |
| `efs` | EFS File Systems | wave2 | `efs.mount-target-down` | mount target down | broken | — |
| `efs` | EFS File Systems | wave1 | `efs.unencrypted` | not encrypted | warn | File data is stored unencrypted at rest. Encryption can only be set when the file system is created — create an encrypted file system and copy the data across. |
| `efs` | EFS File Systems | wave2 | `efs.public-policy` | file system policy open to anyone | broken | The file system policy allows any AWS principal, so anyone who can reach a mount target can read and write the data. Replace the wildcard principal with the specific roles that need access. |
| `efs` | EFS File Systems | wave2 | `efs.no-backup-policy` | automatic backups off | warn | AWS Backup is not taking daily backups of this file system, so a deletion or corruption is unrecoverable. Turn the automatic backup policy on. |
| `dbi-snap` | DB Instance Snapshots | wave1 | `dbi-snap.broken.failed` | failed | broken | — |
| `dbi-snap` | DB Instance Snapshots | wave1 | `dbi-snap.broken.incompatible` | `<incompatible-* status>` | broken | — |
| `dbi-snap` | DB Instance Snapshots | wave1 | `dbi-snap.warn.creating` | creating: <pct>% | warn | — |
| `dbi-snap` | DB Instance Snapshots | wave1 | `dbi-snap.warn.transitional` | <status> | warn | — |
| `dbi-snap` | DB Instance Snapshots | wave1 | `dbi-snap.warn.unencrypted` | unencrypted | warn | — |
| `dbi-snap` | DB Instance Snapshots | wave2 | `dbi-snap.orphan` | orphan: source DB deleted | broken | — |
| `dbi-snap` | DB Instance Snapshots | wave2 | `dbi-snap.past-retention` | automated, <N>d past retention | broken | — |
| `dbi-snap` | DB Instance Snapshots | wave2 | `dbi-snap.public` | shared with all AWS accounts | broken | The snapshot is shared with every AWS account, so anyone can restore it and read the database it came from. Remove \`all\` from the snapshot's restore attribute. |
| `dbc-snap` | DB Cluster Snapshots | wave1 | `dbc-snap.broken.failed` | failed | broken | — |
| `dbc-snap` | DB Cluster Snapshots | wave1 | `dbc-snap.broken.incompatible` | `<incompatible-* status>` | broken | — |
| `dbc-snap` | DB Cluster Snapshots | wave1 | `dbc-snap.warn.creating` | creating | warn | — |
| `dbc-snap` | DB Cluster Snapshots | wave1 | `dbc-snap.warn.transitional` | <status> | warn | — |
| `dbc-snap` | DB Cluster Snapshots | wave1 | `dbc-snap.warn.manual_unused` | manual, unused <N>d | warn | — |
| `dbc-snap` | DB Cluster Snapshots | wave1 | `dbc-snap.warn.unencrypted` | unencrypted | warn | — |
| `dbc-snap` | DB Cluster Snapshots | wave2 | `dbc-snap.orphan` | orphan: source cluster deleted | broken | — |
| `dbc-snap` | DB Cluster Snapshots | wave2 | `dbc-snap.past-retention` | automated, <N>d past retention | broken | — |
| `dbc-snap` | DB Cluster Snapshots | wave2 | `dbc-snap.public` | shared with all AWS accounts | broken | The snapshot is shared with every AWS account, so anyone can restore it and read the cluster it came from. Remove \`all\` from the snapshot's restore attribute. |

### Monitoring

| shortName | Name | Wave | Code | Phrase | Severity | Detail |
| --- | --- | --- | --- | --- | --- | --- |
| `alarm` | CloudWatch Alarms | wave1 | `alarm.state.alarm` | alarm triggered | broken | — |
| `alarm` | CloudWatch Alarms | wave1 | `alarm.state.insufficient_data` | insufficient data | warn | — |
| `alarm` | CloudWatch Alarms | wave1 | `alarm.no_actions` | no actions | warn | — |
| `alarm` | CloudWatch Alarms | wave1 | `alarm.actions-disabled` | actions disabled | warn | The alarm still changes state but runs none of its actions, so nobody is notified when it triggers. Switch actions back on for this alarm. |
| `logs` | CloudWatch Log Groups | wave1 | `logs.retention-never-expire` | retention: never expire | warn | No retention policy set — events kept forever, billed indefinitely. |
| `logs` | CloudWatch Log Groups | wave1 | `logs.stale-empty` | empty, created over 90 days ago | warn | — |
| `logs` | CloudWatch Log Groups | wave2 | `logs.missing-metric-filters` | audit log group missing metric filters | warn | — |
| `logs` | CloudWatch Log Groups | wave1 | `logs.no-kms` | not encrypted with KMS | warn | Log events are encrypted with the CloudWatch Logs service key, so anyone with read access to the log group can read them and you cannot revoke that access with a key policy. Associate a KMS key with this log group. |
| `trail` | CloudTrail Trails | wave1 | `trail.log-file-validation.disabled` | log file validation disabled | warn | — |
| `trail` | CloudTrail Trails | wave2 | `trail.not-logging` | not logging | broken | — |
| `trail` | CloudTrail Trails | wave2 | `trail.delivery-error` | delivery error: <LatestDeliveryError> | broken | — |
| `trail` | CloudTrail Trails | wave2 | `trail.delivery-stale` | delivery stale since <LatestDeliveryTime> | broken | — |
| `trail` | CloudTrail Trails | wave1 | `trail.no-cloudwatch-logs` | not delivering to CloudWatch Logs | warn | Events are delivered to the bucket only, so no metric filter or alarm can watch them and nobody is paged on suspicious account activity. Attach a log group to this trail. |
| `trail` | CloudTrail Trails | wave1 | `trail.no-kms` | log files not KMS-encrypted | warn | Delivered log files use S3-managed encryption, so anyone who can read the bucket can read the audit trail. Set a KMS key on the trail so log files are encrypted with a key you control. |
| `trail` | CloudTrail Trails | wave2 | `trail.log-bucket-public` | log bucket is publicly accessible | broken | The bucket holding this trail's log files is publicly accessible, so the account's audit history can be read by anyone. Remove the public grant from that bucket's policy and access control list. |
| `trail` | CloudTrail Trails | wave2 | `trail.log-bucket-no-access-logging` | log bucket has no access logging | warn | The bucket holding this trail's log files records no access logging, so reads of the audit history leave no trace. Enable server access logging on that bucket. |
| `ct-events` | CloudTrail Events | wave1 | `ct_event.severity.danger` | destructive call | broken | CloudTrail recorded a call that either failed or was destructive; the event name and error code in this row say which. Verify it was expected and, if not, find out who made it. |
| `ct-events` | CloudTrail Events | wave1 | `ct_event.severity.attention` | root account activity | warn | CloudTrail recorded a call worth a look: a modifying call, root-account activity, cross-account access, or a read of sensitive data. The status names which; verify the caller was expected. |
| `ct-events` | CloudTrail Events | wave1 | `ct_event.severity.info` | routine event | dim | — |

### Messaging

| shortName | Name | Wave | Code | Phrase | Severity | Detail |
| --- | --- | --- | --- | --- | --- | --- |
| `sqs` | SQS Queues | wave2 | `sqs.missing-dlq` | no DLQ configured | warn | Messages this queue's consumers keep failing on are retried until they expire and are then thrown away, so a poison message is lost with no record of it. Set a redrive policy pointing at a dead-letter queue. |
| `sqs` | SQS Queues | wave2 | `sqs.no-kms` | not encrypted with KMS | warn | Messages sit unencrypted in the queue, so anyone who reaches the backing storage reads their contents. Set a KMS key on the queue so AWS encrypts each message at rest. |
| `sqs` | SQS Queues | wave2 | `sqs.public-policy` | queue policy open to anyone | broken | The queue's access policy grants send or receive to every AWS principal, so anyone can drain the messages or flood the workers reading them. Scope the policy's Principal to the accounts and roles that actually use the queue. |
| `sns` | SNS Topics | wave2 | `sns.no-subscribers` | topic has no subscribers | warn | Nothing is subscribed to this topic, so every message published to it is discarded on arrival. Either subscribe the endpoint that was meant to receive them, or delete the topic and whatever still publishes to it. |
| `sns` | SNS Topics | wave2 | `sns.all-pending-confirmation` | all pending confirmation | warn | Every subscription on this topic is still waiting for its endpoint to confirm, so no message is being delivered to anyone. Confirm the subscriptions from their endpoints, or remove the ones that were never wanted. |
| `sns` | SNS Topics | wave2 | `sns.public-policy` | topic policy open to anyone | broken | The topic's access policy grants publish or subscribe to every AWS principal, so anyone can read what this topic broadcasts or inject messages its subscribers will trust. Scope the policy's Principal to the accounts and roles that actually use the topic. |
| `sns` | SNS Topics | wave2 | `sns.no-kms` | not encrypted with KMS | warn | Messages sit unencrypted in the topic, so anyone who reaches the backing storage or a raw log of it reads their contents. Set a KMS key on the topic so AWS encrypts each message at rest. |
| `sns-sub` | SNS Subscriptions | wave1 | `sns-sub.state.pending-confirmation` | endpoint has not confirmed the subscription | warn | — |
| `sns-sub` | SNS Subscriptions | wave1 | `sns-sub.state.deleted` | endpoint deleted | dim | — |
| `sns-sub` | SNS Subscriptions | wave1 | `sns-sub.plain-http` | delivers over plain HTTP | warn | The subscription delivers over plain HTTP, so every message crosses the network in the clear and anyone on the path can read or alter it before the endpoint sees it. Point the subscription at an HTTPS endpoint. |
| `eb` | Elastic Beanstalk | wave1 | `eb.health.red` | health: red | broken | — |
| `eb` | Elastic Beanstalk | wave1 | `eb.health.yellow` | health: yellow | warn | — |
| `eb` | Elastic Beanstalk | wave1 | `eb.health.grey` | health: grey | warn | — |
| `eb` | Elastic Beanstalk | wave1 | `eb.status.terminated` | terminated | dim | — |
| `eb` | Elastic Beanstalk | wave1 | `eb.status.launching` | launching | warn | — |
| `eb` | Elastic Beanstalk | wave1 | `eb.status.terminating` | terminating | dim | — |
| `eb` | Elastic Beanstalk | wave2 | `eb.environment-causes` | EB causes: <first cause> | warn | — |
| `eb` | Elastic Beanstalk | wave2 | `eb.managed-updates-off` | managed platform updates off | warn | The environment never takes platform patches on its own, so it stays on whatever version it was launched with until someone updates it by hand. Turn managed platform updates on and pick a weekly maintenance window. |
| `eb` | Elastic Beanstalk | wave2 | `eb.enhanced-health-off` | enhanced health reporting off | warn | Health is reported from basic checks only, so the environment cannot tell you which instance or which request is failing, or why. Switch health reporting to enhanced. |
| `eb` | Elastic Beanstalk | wave2 | `eb.cloudwatch-logs-off` | log streaming to CloudWatch off | warn | Instance logs stay on the instances and disappear when those instances are replaced, so there is nothing left to read after a failure. Turn on log streaming to CloudWatch Logs. |
| `eb-rule` | EventBridge Rules | wave1 | `eb-rule.state.disabled` | disabled | dim | — |
| `eb-rule` | EventBridge Rules | wave2 | `eb-rule.target-issue` | enabled rule has no targets (rule matches but goes nowhere) | broken | — |
| `kinesis` | Kinesis Streams | wave1 | `kinesis.warn.creating` | creating | warn | — |
| `kinesis` | Kinesis Streams | wave1 | `kinesis.warn.updating` | updating | warn | — |
| `kinesis` | Kinesis Streams | wave1 | `kinesis.warn.deleting` | deleting | warn | — |
| `kinesis` | Kinesis Streams | wave2 | `kinesis.unencrypted` | not encrypted at rest | warn | Records sit unencrypted at rest, so anyone who reaches the backing storage reads whatever the stream carries. Turn on server-side encryption and point the stream at a KMS key. |
| `kinesis` | Kinesis Streams | wave2 | `kinesis.min-retention` | 24h retention | warn | The stream keeps only the default 24 hours of records, so a consumer that falls behind for a day, or an outage longer than one, loses data with no way to replay it. Raise the retention period to cover the longest replay you expect to need. |
| `msk` | MSK Clusters | wave1 | `msk.warn.creating` | creating | warn | — |
| `msk` | MSK Clusters | wave1 | `msk.warn.updating` | updating | warn | — |
| `msk` | MSK Clusters | wave1 | `msk.warn.maintenance` | maintenance | warn | — |
| `msk` | MSK Clusters | wave1 | `msk.warn.rebooting_broker` | rebooting broker | warn | — |
| `msk` | MSK Clusters | wave1 | `msk.warn.healing` | healing | warn | — |
| `msk` | MSK Clusters | wave1 | `msk.warn.deleting` | deleting | warn | — |
| `msk` | MSK Clusters | wave1 | `msk.broken.failed` | failed | broken | — |
| `msk` | MSK Clusters | wave2 | `msk.broker-outdated` | broker software outdated | warn | — |
| `msk` | MSK Clusters | wave2 | `msk.encryption-not-tls` | encryption in transit not enforced | warn | — |
| `msk` | MSK Clusters | wave2 | `msk.public-access` | brokers reachable from the internet | broken | Kafka brokers are published to the internet with their own public addresses, so the cluster is reachable from anywhere its security groups allow rather than only from inside the VPC. Turn public access off and reach the brokers from within the VPC or over a peered network. |
| `msk` | MSK Clusters | wave2 | `msk.unauthenticated` | unauthenticated access allowed | broken | The cluster accepts Kafka clients that present no credentials at all, so anyone who can reach a broker can read and write every topic. Turn unauthenticated access off and require one of the cluster's authentication methods. |
| `sfn` | Step Functions | wave2 | `sfn.latest-execution-failed` | latest execution <STATUS> | broken | — |
| `sfn` | Step Functions | wave2 | `sfn.logging-off` | execution logging off | warn | The state machine records nothing about its executions, so a failed run leaves no trace of which state failed or what it was handed. Turn on execution logging to a CloudWatch log group. |
| `sfn` | Step Functions | wave2 | `sfn.no-cmk` | not encrypted with a customer key | warn | Execution history and state data are encrypted with an AWS-owned key you cannot audit, rotate, or revoke. Point the state machine at a customer managed KMS key. |
| `sfn` | Step Functions | wave2 | `sfn.definition-secret` | credential in state machine definition | broken | A credential is written into the state machine's definition, so it is readable by anyone who can call states:DescribeStateMachine and it travels with every export of the workflow. Move the value to Secrets Manager and reference it at run time, then rotate it. |
| `ses` | SES Identities | wave1 | `ses.verification.failed` | verification failed | broken | — |
| `ses` | SES Identities | wave1 | `ses.verification.temp_failure` | verify: temp failure | broken | — |
| `ses` | SES Identities | wave1 | `ses.verification.not_started` | verification not started | broken | — |
| `ses` | SES Identities | wave1 | `ses.verification.pending` | pending verification | warn | — |
| `ses` | SES Identities | wave1 | `ses.sending.disabled` | sending disabled | warn | — |
| `ses` | SES Identities | wave2 | `ses.account-shutdown` | sending paused by AWS (shutdown) | broken | — |
| `ses` | SES Identities | wave2 | `ses.account-probation` | account under review (probation) | broken | — |
| `ses` | SES Identities | wave2 | `ses.quota-high` | quota 80%+ used | warn | — |
| `ses` | SES Identities | wave2 | `ses.dkim-off` | DKIM not enabled | warn | Outbound mail from this domain is not signed, so receivers cannot tell genuine mail from a forgery and are more likely to reject it or file it as spam. Enable DKIM signing for the identity and publish the records AWS gives you. |

### Secrets & config

| shortName | Name | Wave | Code | Phrase | Severity | Detail |
| --- | --- | --- | --- | --- | --- | --- |
| `secrets` | Secrets Manager | wave1 | `secrets.state.deleted` | deleted | broken | — |
| `secrets` | Secrets Manager | wave1 | `secrets.state.rotation_overdue` | rotation overdue | warn | — |
| `secrets` | Secrets Manager | wave1 | `secrets.state.dormant` | dormant | warn | — |
| `secrets` | Secrets Manager | wave1 | `secrets.rotation.disabled` | rotation not enabled | warn | — |
| `secrets` | Secrets Manager | wave1 | `secrets.value.stale` | value unchanged in over 365 days | warn | — |
| `secrets` | Secrets Manager | wave2 | `secrets.public-policy` | resource policy open to anyone | broken | The secret's resource policy allows a wildcard principal, so any AWS account can read the credential this secret holds. Remove the "\*" principal from the resource policy, or scope it with a condition naming the accounts that need it. |
| `secrets` | Secrets Manager | wave2 | `secrets.cross-account-policy` | resource policy grants another account | warn | The secret's resource policy names a principal in another AWS account, so that account can read the credential. Confirm the grant is intended and still needed, and remove the account from the resource policy otherwise. |
| `ssm` | SSM Parameters | wave1 | `ssm.value.plaintext-sensitive` | plaintext value looks like a credential | broken | — |
| `ssm` | SSM Parameters | wave1 | `ssm.value.stale` | not modified in over 365 days | warn | — |
| `kms` | KMS Keys | wave1 | `kms.state.pending_deletion` | pending deletion | broken | — |
| `kms` | KMS Keys | wave1 | `kms.state.disabled` | disabled | warn | — |
| `kms` | KMS Keys | wave1 | `kms.state.unavailable` | <key state> | broken | — |
| `kms` | KMS Keys | wave1 | `kms.access-denied` | access denied (kms:DescribeKey) | broken | — |
| `kms` | KMS Keys | wave2 | `kms.rotation-disabled` | key rotation disabled | warn | This customer-managed key never rotates its backing material, so every ciphertext ever written under it depends on one key that has been in use since creation. Enable automatic key rotation on the key. |
| `kms` | KMS Keys | wave2 | `kms.public-policy` | key policy open to anyone | broken | The key policy allows a wildcard principal, so any AWS account can use this key to decrypt data encrypted with it. Replace the "\*" principal with the specific accounts or roles that need the key, or add a condition scoping the grant. |

### Dns & cdn

| shortName | Name | Wave | Code | Phrase | Severity | Detail |
| --- | --- | --- | --- | --- | --- | --- |
| `r53` | Route 53 Hosted Zones | wave1 | `r53.zone.unused` | only default NS/SOA records remain | warn | — |
| `r53` | Route 53 Hosted Zones | wave2 | `r53.orphan-private-zone` | private zone with no VPC associations (orphan) | warn | — |
| `r53` | Route 53 Hosted Zones | wave2 | `r53.query-logging-off` | query logging off | warn | Nothing records who resolves names in this public zone, so a subdomain being probed or abused leaves no evidence. Create a query logging configuration for the zone. |
| `r53` | Route 53 Hosted Zones | wave2 | `r53.dangling-record` | record points at a released address | broken | The record still answers with an address the account no longer holds, so whoever claims that address next receives traffic for this name. Delete the record or repoint it at an address you own. |
| `cf` | CloudFront Distributions | wave2 | `cf.insecure-protocol` | no HTTPS redirect (insecure); origin without TLS | warn | — |
| `cf` | CloudFront Distributions | wave2 | `cf.origin-bucket-missing` | S3 origin bucket does not exist | broken | The distribution forwards requests to a bucket that no longer exists, so those paths fail and anyone who creates a bucket with that name starts serving your traffic. Repoint the origin at a bucket you own, or remove it. |
| `cf` | CloudFront Distributions | wave2 | `cf.deprecated-tls` | minimum TLS below 1.2 | warn | Viewers may negotiate a protocol version with known weaknesses, which modern browsers already refuse. Raise the distribution's minimum protocol version to TLS 1.2 or later. |
| `cf` | CloudFront Distributions | wave2 | `cf.logging-off` | access logging off | warn | The distribution records no request logs, so an attack or abuse pattern at the edge leaves nothing to investigate. Turn on standard logging and give it a destination. |
| `cf` | CloudFront Distributions | wave2 | `cf.no-default-root-object` | no default root object | warn | A request for the distribution root returns whatever the origin serves there, which can expose object names you did not mean to publish. Set a default root object such as index.html. |
| `cf` | CloudFront Distributions | wave2 | `cf.s3-origin-no-oac` | S3 origin without origin access control | warn | The bucket behind this origin must be open to reach it through CloudFront, so viewers can bypass the distribution and read from the bucket directly. Attach an origin access control and restrict the bucket policy to it. |
| `cf` | CloudFront Distributions | wave2 | `cf.default-certificate` | uses the default CloudFront certificate | warn | The distribution serves custom domains with the default CloudFront certificate, so viewers reaching those names get a certificate mismatch warning. Attach a certificate that covers the aliases. |
| `cf` | CloudFront Distributions | wave2 | `cf.no-geo-restriction` | no geo restriction | warn | Content is served to every country, including any the account is not meant to serve. Add a geographic restriction if the distribution should be limited. |
| `acm` | ACM Certificates | wave1 | `acm.expires-critical` | expires in <N> days | broken | — |
| `acm` | ACM Certificates | wave1 | `acm.expires-soon` | expires in <N> days | warn | — |
| `acm` | ACM Certificates | wave1 | `acm.orphan` | certificate not in use (orphan) | warn | — |
| `acm` | ACM Certificates | wave1 | `acm.weak-key` | weak key algorithm | warn | The certificate's key is short enough to be worth attacking, and browsers are withdrawing trust from keys this size. Reissue the certificate with a key of 2048 bits or more, or an elliptic-curve key. |
| `apigw` | API Gateways | wave2 | `apigw.no-deployed-stages` | no deployed stages | warn | — |
| `apigw` | API Gateways | wave2 | `apigw.stage-config-issues` | no throttling configured (DoS risk); access logs disabled | warn | — |
| `apigw` | API Gateways | wave2 | `apigw.no-authorizer-public` | internet-facing with no authorizer | broken | Anyone on the internet can call every route this gateway exposes, because nothing checks the caller's identity. Attach an authorizer, or scope the resource policy to the callers that should reach it. |
| `apigw` | API Gateways | wave2 | `apigw.no-authorizer` | no authorizer | warn | Nothing checks the caller's identity, so any client that can reach the network this gateway sits on can call every route. Attach an authorizer. |
| `apigw` | API Gateways | wave2 | `apigw.no-access-logs` | no access logs | warn | The stage records no access logs, so a burst of abusive or failing requests leaves nothing to investigate. Point the stage's access logging at a log group. |
| `apigw` | API Gateways | wave2 | `apigw.tracing-off` | X-Ray tracing off | warn | Requests through this stage are not traced, so a slow or failing integration cannot be followed to its cause. Turn on X-Ray tracing for the stage. |
| `apigw` | API Gateways | wave2 | `apigw.stage-variable-secret` | credential in stage variables | broken | A stage variable holds what looks like a credential, and stage variables are readable by anyone who can read the gateway's configuration. Move the value into Secrets Manager and reference it from the integration. |

### Security & iam

| shortName | Name | Wave | Code | Phrase | Severity | Detail |
| --- | --- | --- | --- | --- | --- | --- |
| `role` | IAM Roles | wave1 | `role.trust.wildcard-principal` | anyone can assume this role | broken | Any AWS account can call sts:AssumeRole on this role and obtain its permissions. Replace the "\*" principal in the trust policy with the specific account or role ARNs, or add an sts:ExternalId condition. |
| `role` | IAM Roles | wave1 | `role.trust.confused-deputy` | service can assume without source scoping | warn | An AWS service principal can assume this role on behalf of any caller, so another customer's resource can trick the service into using your role. Add an aws:SourceAccount or aws:SourceArn condition to the trust statement. |
| `role` | IAM Roles | wave1 | `role.inline-privilege-escalation` | inline policy allows privilege escalation: <combo> | broken | An inline policy on this role grants a combination of actions that lets its holder grant itself full administrator. Split or scope the inline policy so the escalation actions are not all available together. |
| `role` | IAM Roles | wave2 | `iam-role.dormant` | dormant role (>90d) | warn | Nothing has assumed this role in over 90 days, so its trust policy and permissions are live but unexercised. Confirm the workload that used it is gone, then delete the role. |
| `role` | IAM Roles | wave2 | `role.admin-attached` | has an administrator policy | warn | This principal is attached to an AWS-managed policy that grants administrator-equivalent access, so anything it can be used for it can be used for everything. Replace the managed policy with a scoped policy covering only the actions this principal needs. |
| `policy` | IAM Policies | wave1 | `iam-policy.orphan-unattached` | unattached, no roles/users/groups use it | warn | — |
| `policy` | IAM Policies | wave2 | `iam-policy.admin-star` | `admin star (allows * on *)` | broken | This policy allows every action on every resource, so anyone holding it is an account administrator. Replace the "\*" action and resource with the specific ones its holders need. |
| `policy` | IAM Policies | wave2 | `policy.privilege-escalation` | allows privilege escalation: <combo> | broken | This policy grants a combination of actions that lets its holder grant itself full administrator, even though no single action looks privileged. Split the combination across separate policies or remove the escalation actions. |
| `iam-user` | IAM Users | wave2 | `iam-user.no-mfa` | console user without MFA | broken | This user signs in to the console with a password alone, so a leaked or guessed password is a full takeover. Register an MFA device for the user, or remove the console password if the user only needs programmatic access. |
| `iam-user` | IAM Users | wave2 | `iam-user.old-key` | key <keyID> >90d (rotation) | warn | This access key has been valid for more than 90 days, so a copy taken at any point since it was created still works. Create a replacement key, move callers onto it, then deactivate and delete the old one. |
| `iam-user` | IAM Users | wave2 | `iam-user.admin-attached` | has an administrator policy | warn | This principal is attached to an AWS-managed policy that grants administrator-equivalent access, so anything it can be used for it can be used for everything. Replace the managed policy with a scoped policy covering only the actions this principal needs. |
| `iam-user` | IAM Users | wave2 | `iam-user.console-never-used` | console password never used | warn | This user has a console password that has never been used since the account was created, so it is an unguarded sign-in path nobody is watching. Delete the login profile and leave the user with programmatic access only. |
| `iam-user` | IAM Users | wave2 | `iam-user.console-dormant` | console sign-in unused for 90 days | warn | Nobody has signed in to this console login for over 90 days. Confirm the person still needs it and delete the login profile if they do not. |
| `iam-user` | IAM Users | wave2 | `iam-user.access-key-unused` | access key unused for <N> days | warn | This access key is active but has not signed a request in over 90 days, so it is a live credential with no owner watching it. Deactivate the key, confirm nothing breaks, then delete it. |
| `iam-user` | IAM Users | wave2 | `iam-user.two-active-keys` | two active access keys | warn | This user has both of its access-key slots active at once, which doubles the exposure and means a rotation cannot be completed. Deactivate and delete the key that is no longer in use. |
| `iam-group` | IAM Groups | wave2 | `iam-group.orphan-or-noop` | group has no members (orphan) | warn | This group grants nothing to nobody: it either has no members or carries no policies, so it only adds noise to access reviews. Delete it, or attach the policy and members it was created for. |
| `iam-group` | IAM Groups | wave2 | `iam-group.admin-attached` | has an administrator policy | warn | This principal is attached to an AWS-managed policy that grants administrator-equivalent access, so anything it can be used for it can be used for everything. Replace the managed policy with a scoped policy covering only the actions this principal needs. |
| `waf` | WAF Web ACLs | wave2 | `waf.no-logging` | no logging configuration | warn | This web ACL is not writing request logs anywhere, so a blocked or allowed request leaves no trace to investigate an incident with. Attach a logging configuration pointing at a Kinesis Firehose stream, S3 bucket, or CloudWatch log group. |
| `waf` | WAF Web ACLs | wave2 | `waf.no-rules` | web ACL has no rules | warn | This web ACL contains no rules, so every request reaches the protected resource and the ACL provides no protection at all. Add rule groups or custom rules, or remove the ACL so it does not read as coverage it is not providing. |

### Ci/cd

| shortName | Name | Wave | Code | Phrase | Severity | Detail |
| --- | --- | --- | --- | --- | --- | --- |
| `cfn` | CloudFormation Stacks | wave1 | `cfn.stack.failed` | <status, in words> | broken | — |
| `cfn` | CloudFormation Stacks | wave1 | `cfn.stack.rollback` | <status, in words> | broken | — |
| `cfn` | CloudFormation Stacks | wave1 | `cfn.stack.in_progress` | <status, in words> | warn | — |
| `cfn` | CloudFormation Stacks | wave1 | `cfn.stack.deleted` | delete complete | dim | — |
| `cfn` | CloudFormation Stacks | wave2 | `cfn.recent-resource-failure` | recent resource failure: <ResourceType/LogicalResourceId> | broken | — |
| `cfn` | CloudFormation Stacks | wave2 | `cfn.stack-drifted` | stack drifted from template | warn | — |
| `cfn` | CloudFormation Stacks | wave1 | `cfn.termination-protection-off` | termination protection off | warn | A single delete call removes this stack and every resource it owns, with no second step to stop an accidental or scripted deletion. Turn on termination protection so the stack must be unprotected deliberately before it can be deleted. |
| `cfn` | CloudFormation Stacks | wave1 | `cfn.output-secret` | credential in stack outputs | broken | A stack output holds what looks like a credential, and outputs are readable by anyone who can describe the stack and importable by any other stack in the account. Move the value into Secrets Manager, export only its name, and rotate the exposed credential. |
| `pipeline` | CodePipelines | wave2 | `pipeline.stage-failed` | stage <stage> failed | broken | — |
| `cb` | CodeBuild Projects | wave2 | `cb.latest-build-failed` | latest build <status> (<date>) | broken | — |
| `cb` | CodeBuild Projects | wave1 | `cb.public-builds` | build results publicly visible | broken | Build logs, environment variables and artifacts for this project are readable by anyone on the internet without an AWS account, so any credential or internal hostname a build prints is public. Set the project's visibility back to private and rotate anything the logs have already exposed. |
| `cb` | CodeBuild Projects | wave1 | `cb.buildspec-from-source` | buildspec taken from the source repository | warn | The build instructions come from a file in the source repository, so anyone who can open a pull request can change what runs inside the build role. Move the buildspec inline into the project definition, or restrict who can trigger builds from unmerged branches. |
| `cb` | CodeBuild Projects | wave1 | `cb.source-url-credential` | credential in the source repository address | broken | The source repository address embeds a username and password or token, which is stored in the project definition and printed in build logs in clear text. Move the credential into a CodeBuild source credential or Secrets Manager entry and rotate it, because it must be assumed leaked. |
| `cb` | CodeBuild Projects | wave1 | `cb.env-secret` | credential in environment variables | broken | A plaintext environment variable on this project holds what looks like a credential; every build log and anyone who can read the project definition sees its value. Move it to Secrets Manager or Parameter Store, reference it by type, and rotate the exposed value. |
| `ecr` | ECR Repositories | wave2 | `ecr.vulnerabilities` | <N> critical, <M> high vulnerabilities | broken | — |
| `ecr` | ECR Repositories | wave2 | `ecr.public-policy` | repository policy open to anyone | broken | The repository policy grants a wildcard principal, so any AWS account can pull the images this repository holds and read whatever is baked into their layers. Replace the wildcard principal with the accounts or roles that need the images, or scope the grant with a condition. |
| `ecr` | ECR Repositories | wave2 | `ecr.no-lifecycle-policy` | no lifecycle policy | warn | No lifecycle policy is set, so every image ever pushed is kept forever: storage cost grows without limit and long-superseded, vulnerable images stay pullable by tag or digest. Add a lifecycle policy that expires untagged images and caps how many versions of each tag are retained. |
| `ecr` | ECR Repositories | wave1 | `ecr.scan-on-push-off` | scan on push off | warn | Images pushed to this repository are never scanned, so a known vulnerability in a base layer reaches production without anyone being told. Turn on scan on push for the repository so every new image is checked as it arrives. |
| `ecr` | ECR Repositories | wave1 | `ecr.mutable-tags` | tags are mutable | warn | An existing tag in this repository can be moved to different image content, so the digest behind a deployed tag can change without any deployment. Set the repository to immutable tags so a tag always names the image it was built from. |
| `codeartifact` | CodeArtifact Repos | wave2 | `codeartifact.no-permissions-policy` | no permissions policy | warn | — |
| `codeartifact` | CodeArtifact Repos | wave2 | `codeartifact.public-access-policy` | public access policy | broken | The repository's resource policy grants a wildcard principal, so any AWS account can read the packages it holds and, depending on the actions allowed, publish into it. Replace the "\*" principal with the accounts or roles that need the repository, or scope the grant with a condition. |

### Data & analytics

| shortName | Name | Wave | Code | Phrase | Severity | Detail |
| --- | --- | --- | --- | --- | --- | --- |
| `glue` | Glue Jobs | wave2 | `glue.latest-run-failed` | latest run <STATUS> | broken | The job's most recent run did not finish, so whatever it feeds has been stale since then. Check the run's error message and CloudWatch logs for the cause, then rerun the job. |
| `glue` | Glue Jobs | wave1 | `glue.no-security-configuration` | no security configuration | warn | This job names no security configuration, so its S3 output, its CloudWatch log stream and its job bookmarks are all written without encryption at rest. Create a security configuration with a KMS key and attach it to the job. |
| `glue` | Glue Jobs | wave1 | `glue.continuous-logging-off` | continuous logging off | warn | Continuous logging is off, so driver and executor output only appears after the run ends and is lost entirely when a run is killed, leaving failures with no diagnostics. Add the continuous CloudWatch logging argument to the job's default arguments. |
| `glue` | Glue Jobs | wave1 | `glue.argument-secret` | credential in job arguments | broken | A default argument on this job holds what looks like a credential, and default arguments are readable by anyone who can describe the job and are echoed into run history. Move the value into Secrets Manager, pass its name instead, and rotate the exposed credential. |
| `athena` | Athena Workgroups | wave1 | `athena.workgroup-disabled` | disabled | warn | Workgroup is administratively disabled — queries submitted against it are rejected until re-enabled. |
| `athena` | Athena Workgroups | wave2 | `athena.settings-not-enforced` | settings can be overridden per query | warn | Every query submitted to this workgroup may override the settings it defines, so the result location and encryption configured here are advisory rather than binding. Turn on the workgroup's configuration enforcement so its settings apply to every query. |
| `athena` | Athena Workgroups | wave2 | `athena.results-unencrypted` | query results stored unencrypted | warn | Query results are written to S3 with no encryption configured, so whatever a query returns is readable by anyone who can read the results bucket. Set an encryption option on the workgroup's result configuration. |
| `mwaa` | Managed Airflow | wave1 | `mwaa.warn.creating` | creating | warn | Environment is being provisioned; Airflow is not yet reachable. |
| `mwaa` | Managed Airflow | wave1 | `mwaa.warn.creating_snapshot` | creating snapshot | warn | The environment is snapshotting its metadata database before an update or upgrade. |
| `mwaa` | Managed Airflow | wave1 | `mwaa.warn.pending` | pending: awaiting VPC endpoints | warn | Creation is paused until the required VPC endpoints exist in your VPC. |
| `mwaa` | Managed Airflow | wave1 | `mwaa.warn.updating` | updating | warn | Environment update in progress; workers may be replaced. |
| `mwaa` | Managed Airflow | wave1 | `mwaa.warn.rolling_back` | rolling back: update failed | warn | Update or upgrade failed; the environment is restoring the latest metadata snapshot. |
| `mwaa` | Managed Airflow | wave1 | `mwaa.warn.maintenance` | maintenance in progress | warn | Scheduled maintenance is running; the environment may be briefly unavailable. |
| `mwaa` | Managed Airflow | wave1 | `mwaa.broken.create_failed` | create failed | broken | Environment creation failed and the environment was not created. |
| `mwaa` | Managed Airflow | wave1 | `mwaa.broken.update_failed` | update failed: rolled back | broken | Update failed; environment was restored to its previous state and is usable. |
| `mwaa` | Managed Airflow | wave1 | `mwaa.broken.unavailable` | unavailable: not stable | broken | Environment failed and did not return to a stable state; contact AWS support. |
| `mwaa` | Managed Airflow | wave1 | `mwaa.dim.deleting` | deleting | dim | Environment is being deleted. |
| `mwaa` | Managed Airflow | wave1 | `mwaa.dim.deleted` | deleted | dim | Environment has been deleted. |
| `mwaa` | Managed Airflow | wave1 | `mwaa.warn.last_update_failed` | last update failed | warn | The last update to this environment failed, so it is still running its previous configuration; the error code and message are listed below. Fix the cause and update again. |
| `mwaa` | Managed Airflow | wave1 | `mwaa.warn.webserver_public` | webserver public | warn | The Airflow web server answers from the public internet, so its login page is reachable by anyone; the access mode is listed below. Switch the environment to private-only access from your VPC. |
| `mwaa` | Managed Airflow | wave1 | `mwaa.warn.details_denied` | details denied | warn | Access to environment details was denied; only the name is visible. |
| `mwaa` | Managed Airflow | wave1 | `mwaa.warn.details_unavailable` | details unavailable | warn | Details could not be retrieved; only the name is visible. |

### Backup

| shortName | Name | Wave | Code | Phrase | Severity | Detail |
| --- | --- | --- | --- | --- | --- | --- |
| `backup` | Backup Plans | wave2 | `backup.job-failed` | <N> jobs failed in last 24h | broken | — |
| `backup` | Backup Plans | wave2 | `backup.job-partial` | partial: <N> of <M> resources skipped | warn | — |

### Other

| shortName | Name | Wave | Code | Phrase | Severity | Detail |
| --- | --- | --- | --- | --- | --- | --- |
| `alarm\_history` | Alarm History | wave1 | `alarm-history.broken.alarm` | alarm | broken | — |
| `alarm\_history` | Alarm History | wave1 | `alarm-history.warn.insufficient_data` | insufficient data | warn | — |
| `asg\_activities` | Scaling Activities | wave1 | `asg-activity.broken.failed` | failed | broken | — |
| `asg\_activities` | Scaling Activities | wave1 | `asg-activity.warn.cancelled` | cancelled | warn | — |
| `cb\_builds` | CodeBuild Builds | wave1 | `cb-build.broken.failed` | failed | broken | — |
| `cb\_builds` | CodeBuild Builds | wave1 | `cb-build.broken.fault` | fault | broken | — |
| `cb\_builds` | CodeBuild Builds | wave1 | `cb-build.broken.timed_out` | timed out | broken | — |
| `cb\_builds` | CodeBuild Builds | wave1 | `cb-build.warn.in_progress` | in progress | warn | — |
| `cb\_builds` | CodeBuild Builds | wave1 | `cb-build.dim.stopped` | stopped | dim | — |
| `cfn\_events` | Stack Events | wave1 | `cfn-event.broken.failed` | <status, lowercased> | broken | — |
| `cfn\_events` | Stack Events | wave1 | `cfn-event.warn.in_progress` | <status, lowercased> | warn | — |
| `cfn\_events` | Stack Events | wave1 | `cfn-event.dim.deleted` | deleted | dim | — |
| `cfn\_resources` | Stack Resources | wave1 | `cfn-resource.broken.failed` | <status, lowercased> | broken | — |
| `cfn\_resources` | Stack Resources | wave1 | `cfn-resource.warn.in_progress` | <status, lowercased> | warn | — |
| `cfn\_resources` | Stack Resources | wave1 | `cfn-resource.dim.deleted` | deleted | dim | — |
| `dbi\_events` | RDS Events | wave1 | `dbi_events.broken.failure` | failure | broken | — |
| `dbi\_events` | RDS Events | wave1 | `dbi_events.broken.low_storage` | low storage | broken | — |
| `dbi\_events` | RDS Events | wave1 | `dbi_events.warn.failover` | failover | warn | — |
| `dbi\_events` | RDS Events | wave1 | `dbi_events.warn.recovery` | recovery | warn | — |
| `eb\_rule\_targets` | EB Rule Targets | wave1 | `eb_rule_targets.warn.no_dlq` | no DLQ configured | warn | — |
| `ecr\_images` | ECR Images | wave1 | `ecr_images.broken.scan_failed` | scan failed | broken | — |
| `ecr\_images` | ECR Images | wave1 | `ecr_images.broken.critical` | <N> critical vulnerabilities | broken | — |
| `ecr\_images` | ECR Images | wave1 | `ecr_images.warn.high` | <N> high vulnerabilities | warn | — |
| `ecr\_images` | ECR Images | wave1 | `ecr_images.dim.untagged` | untagged | dim | — |
| `ecs\_svc\_logs` | Service Logs | wave1 | `log-event.broken.error` | error | broken | — |
| `ecs\_svc\_logs` | Service Logs | wave1 | `log-event.warn.warning` | warning | warn | — |
| `ecs\_tasks` | Service Tasks | wave1 | `ecs-task.health.unhealthy` | unhealthy | broken | — |
| `ecs\_tasks` | Service Tasks | wave1 | `ecs-task.stop-code.failed` | stopped: <stop code> | broken | — |
| `ecs\_tasks` | Service Tasks | wave1 | `ecs-task.state.stopped` | stopped | dim | — |
| `ecs\_tasks` | Service Tasks | wave1 | `ecs-task.state.provisioning` | provisioning | warn | — |
| `ecs\_tasks` | Service Tasks | wave1 | `ecs-task.state.pending` | pending | warn | — |
| `ecs\_tasks` | Service Tasks | wave1 | `ecs-task.state.activating` | activating | warn | — |
| `ecs\_tasks` | Service Tasks | wave1 | `ecs-task.state.deactivating` | deactivating | warn | — |
| `ecs\_tasks` | Service Tasks | wave1 | `ecs-task.state.stopping` | stopping | warn | — |
| `ecs\_tasks` | Service Tasks | wave1 | `ecs-task.state.deprovisioning` | deprovisioning | warn | — |
| `elb\_listeners` | ELB Listeners | wave1 | `elb_listeners.broken.no_certificate` | no certificate configured | broken | — |
| `glue\_runs` | Job Runs | wave1 | `glue-run.broken.failed` | failed | broken | — |
| `glue\_runs` | Job Runs | wave1 | `glue-run.broken.timeout` | timeout | broken | — |
| `glue\_runs` | Job Runs | wave1 | `glue-run.broken.error` | error | broken | — |
| `glue\_runs` | Job Runs | wave1 | `glue-run.broken.expired` | expired | broken | — |
| `glue\_runs` | Job Runs | wave1 | `glue-run.warn.running` | running | warn | — |
| `glue\_runs` | Job Runs | wave1 | `glue-run.warn.starting` | starting | warn | — |
| `glue\_runs` | Job Runs | wave1 | `glue-run.warn.stopping` | stopping | warn | — |
| `glue\_runs` | Job Runs | wave1 | `glue-run.warn.waiting` | waiting | warn | — |
| `glue\_runs` | Job Runs | wave1 | `glue-run.dim.stopped` | stopped | dim | — |
| `lambda\_invocations` | Lambda Invocations | wave1 | `lambda-invocation.broken.timeout` | timed out | broken | — |
| `log\_events` | Log Events | wave1 | `log-event.broken.error` | error | broken | — |
| `log\_events` | Log Events | wave1 | `log-event.warn.warning` | warning | warn | — |
| `pipeline\_stages` | Pipeline Stages | wave1 | `pipeline-stage.broken.failed` | failed | broken | — |
| `role\_policies` | Role Policies | wave1 | `role-policy.broken.over_privileged` | over-privileged | broken | — |
| `role\_policies` | Role Policies | wave1 | `role-policy.dim.inline` | inline | dim | — |
| `sfn\_execution\_history` | SFN Execution History | wave1 | `sfn-execution-history.broken.event_failed` | task failed | broken | — |
| `sfn\_executions` | SFN Executions | wave1 | `sfn-execution.broken.failed` | failed | broken | — |
| `sfn\_executions` | SFN Executions | wave1 | `sfn-execution.broken.timed_out` | timed out | broken | — |
| `sfn\_executions` | SFN Executions | wave1 | `sfn-execution.broken.aborted` | aborted | broken | — |
| `sns\_subscriptions` | SNS Subscriptions | wave1 | `sns-sub.state.pending-confirmation` | endpoint has not confirmed the subscription | warn | — |
| `sns\_subscriptions` | SNS Subscriptions | wave1 | `sns-sub.state.deleted` | endpoint deleted | dim | — |
| `sns\_subscriptions` | SNS Subscriptions | wave1 | `sns-sub.plain-http` | delivers over plain HTTP | warn | The subscription delivers over plain HTTP, so every message crosses the network in the clear and anyone on the path can read or alter it before the endpoint sees it. Point the subscription at an HTTPS endpoint. |
| `transfer\_agreements` | Agreements | wave1 | `transfer.warn.agreement_inactive` | inactive: partner traffic rejected | warn | Agreement is inactive; partner traffic is rejected. |
| `transfer\_agreements` | Agreements | wave1 | `transfer.broken.cert_expired` | expired | broken | The certificate has expired, so partner connections that present or verify it now fail. Import a renewed certificate and point the profile at it. |
| `transfer\_agreements` | Agreements | wave1 | `transfer.warn.cert_expiring` | expires in <N>d | warn | The certificate expires soon; once it does, partner connections that present or verify it will fail. Import a renewed certificate before the inactive date. |

<!-- END GENERATED: signals -->

## Not yet implemented

Signals the per-type designs describe and no code emits. They are not
`FindingDef`s, so the generated table above cannot carry them; each is one line
here until it lands there.

Wave 3 — the deeper reads, mostly CloudWatch, that no type performs yet:

- `ec2` — CloudWatch `StatusCheckFailed`.
- `ecs-svc` — CloudWatch `CPUUtilization`/`MemoryUtilization` p99 per service.
- `ecs` — CloudWatch `CPUReservation`/`MemoryReservation`; `DescribeContainerInstances` per cluster for agent-disconnect.
- `ecs-task` — cross-cluster outlier detection.
- `lambda` — CloudWatch `Errors`/`Invocations` ratio, `Throttles`, `Duration` p99 vs `Timeout`; `GetFunctionConcurrency` per function.
- `asg` — CloudWatch `GroupDesiredCapacity` vs `GroupInServiceInstances` delta sustained.
- `eb` — `DescribeConfigurationSettings` platform-EOL check; `DescribeEvents` severity filter.
- `ebs` — CloudWatch `VolumeQueueLength`; `BurstBalance` on gp2.
- `ebs-snap` — snapshot-lineage cost attribution.
- `ami` — sharing-history audit.
- `lt` — version diffing (`$Default` vs `$Latest`); `IamInstanceProfile` role resolution; `ssm:GetParameter` AMI resolution; `DescribeInstances`-by-tag; spot/CPU/placement config.
- `eks` — addon health.
- `ng` — AMI release drift; `ListUpdates` per node group.
- `elb` — CloudWatch `HTTPCode_ELB_5XX_Count`.
- `tg` — CloudWatch `UnHealthyHostCount`/`HealthyHostCount` ratios.
- `vpc` — `DescribeVpcAttribute(EnableDnsSupport)` per VPC.
- `nat` — CloudWatch `ErrorPortAllocation`, `PacketsDropCount`, `BytesOutToDestination==0` cost waste.
- `eip` — `DescribeAddressesAttribute` per address (reverse DNS).
- `vpce` — endpoint policy analysis beyond the wildcard-principal case.
- `tgw` — CloudWatch `PacketDropCountBlackhole`/`PacketDropCountNoRoute`.
- `transfer` — CloudWatch `FilesIn`/`FilesOut`/`BytesIn`/`BytesOut` and AS2 `InboundMessage`/`OutboundMessage`; `ListExecutions` workflow runs; per-user home directories and SSH keys; AS2 MDN history.
- `vpc-peer` — security-group cross-referencing; IPv6 overlap; DNS effective state; flow-log traffic verification; remote-side health (invisible by construction).
- `sg` — security group referencing a deleted security group.
- `dbi` — CloudWatch `FreeStorageSpace`, `CPUUtilization`, `ReplicaLag`, `DatabaseConnections`.
- `dbc` — CloudWatch `DBInstanceReplicaLag`, `DatabaseConnections`.
- `redis` — CloudWatch `DatabaseMemoryUsagePercentage`, `Evictions`, `ReplicationLag`, `EngineCPUUtilization`.
- `ddb` — CloudWatch `ReadThrottleEvents`, `WriteThrottleEvents`, `SystemErrors`.
- `opensearch` — cluster health is CloudWatch-only (`AWS/ES` `ClusterStatus.red`/`yellow`), plus `FreeStorageSpace` and `JVMMemoryPressure`.
- `redshift` — CloudWatch `PercentageDiskSpaceUsed`, `HealthStatus`.
- `efs` — CloudWatch `PercentIOLimit`, `BurstCreditBalance`.
- `s3` — `GetBucketEncryption` per bucket.
- `sqs` — CloudWatch `NumberOfMessagesSent`/`NumberOfMessagesReceived` trend.
- `sns` — CloudWatch `NumberOfNotificationsFailed`.
- `sns-sub` — `GetSubscriptionAttributes` per subscription (dead-letter queue); CloudWatch `NumberOfNotificationsFailed` per endpoint.
- `eb-rule` — CloudWatch `FailedInvocations`/`ThrottledRules` per rule.
- `kinesis` — CloudWatch `GetRecords.IteratorAgeMilliseconds` consumer lag, `WriteProvisionedThroughputExceeded`, `ReadProvisionedThroughputExceeded`.
- `msk` — CloudWatch `ActiveControllerCount`, `OfflinePartitionsCount`, `UnderReplicatedPartitions`, `KafkaDataLogsDiskUsed`.
- `sfn` — CloudWatch `ExecutionsFailed`/`ExecutionsTimedOut`/`ExecutionThrottled` trend.
- `ses` — reputation dashboard (`BounceRate`/`ComplaintRate`) via CloudWatch.
- `secrets` — `DescribeSecret` per secret for `VersionIdsToStages` stuck on `AWSPENDING`.
- `ssm` — `GetParameterHistory` per parameter for the true access age.
- `kms` — grant-level analysis per key (`ListGrants`).
- `role` — `GenerateServiceLastAccessedDetails` permission-usage audit.
- `policy` — IAM Access Advisor unused-permission analysis.
- `iam-user` — credential report (`GenerateCredentialReport` + `GetCredentialReport`), one account-wide pull.
- `iam-group` — blast-radius analysis across the group's members.
- `waf` — CloudWatch `BlockedRequests` spike; managed-rule-group version drift.
- `r53` — registrar nameserver lookup (external); health-check status aggregation per zone.
- `cf` — CloudWatch `5xxErrorRate`/`TotalErrorRate`; origin-deleted cross-check.
- `apigw` — CloudWatch `5XXError`/`4XXError`; `GetUsagePlans` quota-breach detection.
- `trail` — `LookupEvents` absence detection.
- `ct-events` — absence-of-expected-events alerting.
- `cfn` — `DetectStackDrift` + `DescribeStackDriftDetectionStatus` for fresh drift detection.
- `pipeline` — `ListPipelineExecutions` trend; dormant-pipeline detection.
- `cb` — stale project (>90d); cache configuration and performance signals.
- `ecr` — `DescribeImageScanFindings` per image.
- `codeartifact` — `DescribeRepository` encryption check.
- `glue` — DPU-hours trend; bookmark-stuck detection.
- `athena` — `ListQueryExecutions` + `BatchGetQueryExecution` failure rate per workgroup.
- `mwaa` — CloudWatch `AWS/MWAA` `SchedulerHeartbeat`, DAG `ImportErrors`/`TotalParseTime`, `QueuedTasks`/`RunningTasks`, worker and scheduler CPU and memory; Airflow-version EOL check (no stable API source).
- `backup` — "newest completed older than rule cadence × 2", which needs `GetBackupPlan` per plan for the cadence.

Wave 1 and Wave 2 conditions the designs specify and no fetcher or enricher emits:

- `ami` — backing snapshot missing (cross-ref `ebs-snap`, owner-scoped only).
- `vpc` — no subnets (empty VPC).
- `subnet` — free-address ratio below 10% (Warning) or 2% (Broken); public subnet without a `0.0.0.0/0` route to an internet gateway.
- `igw` — a `detached` attachment; gateway attached to a VPC with no `0.0.0.0/0` route through it.
- `eip` — attached to a stopped instance (zombie billing).
- `tgw` — attachment `rejected`/`rejecting`.
- `eni` — requester-managed interface whose description references a deleted service.
- `sqs` — queue depth over threshold, depth rising unbounded, oldest message older than `VisibilityTimeout` × 5, dead-letter queue holding messages.
- `secrets` — rotation overdue by more than `AutomaticallyAfterDays` × 2 (rotation failing).
- `ssm` — `Advanced` tier parameter unmodified for 90 days (cost).
- `waf` — `DefaultAction==Allow` with zero rules.
- `r53` — DNSSEC `SIGNING` with an inactive key-signing key on a public zone.
- `acm` — `RenewalSummary.RenewalStatus==FAILED`; `DomainValidationOptions[].ValidationStatus==FAILED`.
- `alarm` — `INSUFFICIENT_DATA` older than 2 × `Period` (dead metric pipeline); dimensions naming a resource absent from a loaded sibling list (zombie alarm).
- `logs` — referenced KMS key in `PendingDeletion`; last event older than the expected write cadence (silent service).
- `cfn` — stack `IN_PROGRESS` for over an hour (stuck).
- `pipeline` — execution `Stopped`/`Cancelled`; stage `InProgress` for over two hours.
- `codeartifact` — empty repository older than 30 days (unused).
