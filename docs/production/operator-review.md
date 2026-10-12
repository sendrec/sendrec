# Production inputs for operator review

Draft for OpenSpec task 1.3 of `prepare-production-video-service`. Nothing here is deployed or authorized. Every proposal below is proposed, awaiting operator approval. Production stays unactivated until a separate rollout approval, as the [production runbook](README.md) describes.

Evidence was read on 2026-10-12 between 06:10 and 06:30 UTC with read-only AWS calls through the `mpd-staging` profile, role DeveloperWorkloadAccess, and the `mpd-prod` profile, role ProductionWorkloadAccess. No AWS resource was created, changed or deleted. Each number is marked **measured**, with its source, or **estimated**, with its assumptions. Prices are USD on-demand list prices for ap-southeast-2, before GST, free tier, credits or Savings Plans.

## No live deployment

- Measured, `mpd-prod`: the production account has no S3 bucket whose name contains `sendrec`, `video` or `record` other than `mpd-vimeo-videos`. It has no ECS cluster or ECR repository for SendRec. The ECS clusters present are mpd-api, mpd-ai, machine learning, bridge, data mart and business analytics.
- Not verifiable with this role: `ProductionWorkloadAccess` cannot call `cloudformation:ListStacks`, so the absence of a `SendRecProduction` stack is inferred from the missing cluster and bucket, not read directly.
- The CDK production path still synthesizes zero application tasks and refuses bootstrap or registration settings.

## 1. Account, network and identity

### Account and VPC

| Item | Value | Status |
|---|---|---|
| Account | Production `891612549531`, ap-southeast-2 | Measured with `sts get-caller-identity` |
| VPC | Reuse `vpc-0093eafc43a8ad516`, Base-VPC, 10.1.0.0/16, owned by `base-infrastructure` | Measured. Proposed, awaiting operator approval |
| Same VPC as mpd-api production | Yes. mpd-api `ApplicationConfig` names this VPC for production | Read from mpd-api at `05d26eab` |
| Fargate tasks | Private subnets 10.1.4.0/24 in 2a and 10.1.5.0/24 in 2b | Measured subnet layout. Proposed placement |
| Internal ALB and RDS | Isolated subnets 10.1.0.0/24 in 2a and 10.1.1.0/24 in 2b | Measured subnet layout. Proposed placement |
| NAT | One gateway, `nat-0caf0075fa9409596` in 2a. Both private route tables use it | Measured |
| Peering | `pcx-0b9fb2da66864e474` to Deployment 10.2.0.0/16, routed from private and isolated subnets | Measured |
| VPC endpoints | None in the production VPC | Measured |
| CloudFront origin prefix list | `pl-b8a742d1` | Measured in both accounts |
| Recordings bucket | `mpd-video-recordings-production` | Proposed, awaiting operator approval. `head-bucket` returned 404 on 2026-10-12, so the name was free then. It is not reserved |

A new VPC is not proposed. The account boundary already separates staging and production, and a second VPC would need its own NAT and peering. `ProductionConfig` only requires that the VPC differ from staging's, which this one does.

The `mpd-aws` skill says production has two NAT gateways, one per AZ. The live account has one. With one NAT in 2a, an outage in 2a removes outbound access for tasks in both AZs. Tasks would lose S3, Cognito, mpd-api and Secrets Manager at task start. Multi-AZ tasks and Multi-AZ RDS do not cover that case.

No peering to mpd-api is needed. The services share a VPC, and staging reaches mpd-api through its public HTTPS hostname via NAT. The proposal keeps that pattern for production.

### Identity references

Production Cognito values are unknown. `ProductionWorkloadAccess` cannot call `cognito-idp:ListUserPools`, and mpd-api production has not created a SendRec client yet.

| Context or setting | Production value | Source and status |
|---|---|---|
| `MPD_COGNITO_ISSUER` | `https://cognito-idp.ap-southeast-2.amazonaws.com/<production mpd-staff pool ID>` | Unknown. mpd-api `ApplicationStack` creates the `mpd-staff` pool in each account |
| `MPD_COGNITO_CLIENT_ID` | Production `mpd-staff-sendrec` client ID | Does not exist. mpd-api creates this client only when both `sendrecCallbackUrl` and `aiClientDomainPrefix` are set. Both are empty for production at `05d26eab` |
| `MPD_ALLOWED_CLIENT_IDS` | Production Staff OS or EHR client IDs allowed to exchange tokens | Unknown |
| `MPD_ALLOWED_ORIGINS` | Exact production Staff OS origin | Unknown. Staging uses the placeholder `https://staff.example.test` |
| `MPD_ACCESS_URL` | `https://api.myperformancedoctor.com/api/v1/staff/video-access` | Proposed, awaiting operator approval. Host from mpd-api production config, path from staging |
| `MPD_EVENT_RECEIVER_URL` | `https://api.myperformancedoctor.com/api/video-reports/sendrec/webhook` | Proposed, awaiting operator approval. Same derivation |
| `MPD_EVENT_KEY_ID` | `sendrec-production-1` | Proposed, awaiting operator approval |
| `MPD_WORKSPACE_ID`, `MPD_TENANT_ID` | Production managed workspace and tenant | Unknown. The workspace is provisioned through the [operator identity runbook](../mpd/operator-identity.md) |

Ordering follows from the callback. The SendRec callback URL contains the production CloudFront hostname, and that hostname exists only after the production stack is created. The sequence is:

1. Deploy the empty production stack.
2. Set `sendrecCallbackUrl` and a production hosted sign-in domain in mpd-api, then deploy mpd-api.
3. Put the new client ID and secret into the SendRec integration secret.
4. Activate the integration.

Staging differs from this plan. Measured on `sendrec-staging:5`, registered 2026-10-12 05:30 UTC: all three integration flags are `true`. `MPD_COGNITO_CLIENT_ID` equals the `mpd-staff-ehr` client of the `mpd-staff` pool, `ap-southeast-2_JjrPuaJf3`. The staging pool lists no `mpd-staff-sendrec` client. The operator should confirm whether staging deliberately reuses the EHR client. Its callback URLs were not read, because the call that returns them also returns the client secret.

### DNS and domain

Proposed, awaiting operator approval: start on the managed `*.cloudfront.net` hostname, as staging does and as design decision 2 allows.

One caution. Patient watch links carry `BASE_URL`, so patients would see a `cloudfront.net` address. A custom hostname such as `video.myperformancedoctor.com` needs an ACM certificate in us-east-1, a DNS record and new callback and CORS values. The distribution's original hostname keeps working after an alias is added, but decide the hostname before any patient receives a link. This review did not check which provider hosts DNS for `myperformancedoctor.com`.

### Secrets

These are names and fields only. No values were read.

| Secret | Created by | Fields |
|---|---|---|
| `sendrec-production/rds-credentials` | Stack, generated | host, port, username, password |
| `sendrec-production/jwt-secret` | Stack, generated | session signing key |
| `sendrec-production/mpd-integration` | Operator, before activation. Proposed name, awaiting operator approval | `MPD_COGNITO_CLIENT_SECRET`, `MPD_SESSION_ENCRYPTION_KEY`, `MPD_SERVICE_TOKEN_HASHES`, `MPD_EVENT_SIGNING_KEY` |
| mpd-api production SendRec integration secret | mpd-api operator, in `891612549531` | Counterpart of staging `mpd-api/sendrec-integration`. Production ARN is empty in mpd-api config |

Production creates no synthetic webhook verifier and no SMTP secret, because optional email workers stay disabled. All stack secrets are retained on stack deletion.

### Image and release

`ProductionConfig` requires a same-account `891612549531` ECR digest. Measured: production has no SendRec ECR repository. Staging has only the CDK asset repository, so the `MPD_ECR_REPOSITORY` used by the candidate build workflow does not exist yet either. Staging's current image is 63 MB compressed. The Whisper model, about 488 MB, downloads from Hugging Face at each task start.

Releases run from GitHub Actions roles, not from a Deployment-account pipeline as other MPD application repos do. That difference is already in the runbook. It needs the GitHub OIDC trust and release roles that the README lists as not configured.

## 2. Monthly cost estimate

### Unit prices used

| Unit | Price | Source |
|---|---|---|
| Fargate x86 vCPU-hour | $0.04856 | Measured: staging Cost Explorer, 2026-10-10, cost divided by usage |
| Fargate GB-hour | $0.00532 | Measured: same |
| ALB hour, LCU-hour | $0.0252, $0.008 | Measured from Cost Explorer; LCU from the Pricing API |
| RDS PostgreSQL db.t4g.micro Multi-AZ, db.t4g.small Multi-AZ | $0.051, $0.102 per hour | Pricing API |
| RDS gp3 Multi-AZ storage, backup beyond the free allowance | $0.276, $0.095 per GB-month | Pricing API |
| S3 Standard storage | $0.025 per GB-month, first 50 TB | Pricing API |
| Data transfer out to the internet | $0.114 per GB, first 10 TB | Pricing API, region and CloudFront Australia agree |
| NAT data processing | $0.059 per GB | Measured: staging Cost Explorer |
| CloudWatch alarm metric, custom metric, log ingestion | $0.10, $0.30 per month, $0.67 per GB | Pricing API |
| Secrets Manager | $0.40 per secret-month | Measured: staging Cost Explorer |

### Staging shape used as the baseline

Measured on staging, 2026-10-05 to 2026-10-12:

- One Fargate task, 2 vCPU and 4 GB, x86. ECS CPU peaked at 99.9% on 2026-10-07 during synthetic transcription and stayed under 0.2% on idle days. Memory peaked at 21%.
- RDS `sendrec-staging-encrypted-db`: db.t4g.micro, single-AZ, 20 GB gp3, PostgreSQL 18.6. Weekly CPU averaged 4%. Freeable memory fell as low as 74 MB of 1 GiB with 2 to 4 connections.
- Recordings bucket: 11 objects, 3.06 MB, versioning never enabled, no lifecycle rules.
- Log group `/ecs/sendrec-staging`: 30-day retention, under 300 bytes a day on idle days and 107 KB on the busy test day.
- CloudFront `E1SOB3QVORZR22`, price class All.

Production, as synthesized, changes this shape in four ways: two tasks of the same size across two AZs, Multi-AZ RDS of the same class, 35-day backups and S3 versioning. The CDK still pins db.t4g.micro for production.

### Fixed monthly cost, any volume

| Item | Monthly | Basis |
|---|---|---|
| Fargate, 2 tasks of 2 vCPU and 4 GB, 730 hours | $172.86 | Estimated: measured rates multiplied by the proposed size |
| RDS db.t4g.micro Multi-AZ | $37.23 | Estimated: list price, 730 hours |
| RDS storage, 20 GB gp3 Multi-AZ | $5.52 | Estimated: list price |
| RDS backup storage beyond the free allowance | $1.00 | Estimated: database under 5 GB, so 35 days of logs mostly fit inside the free 20 GB |
| ALB hours | $18.40 | Estimated: measured rate, 730 hours |
| ALB LCUs | $3.00 | Estimated: about 0.5 LCU on average for staff and patient page traffic |
| CloudFront, application pages and API | $1.00 | Estimated: media bypasses CloudFront, so only HTML, JS and API responses pass through it |
| CloudWatch alarms and custom metrics | $3.70 | Estimated: 16 alarm metrics, because seven alarms use two-metric math, plus seven log metric filters |
| CloudWatch logs and probe Lambda | $1.00 | Estimated: minute-level monitor lines from two tasks plus the five-minute backup probe |
| Secrets Manager, 3 secrets | $1.20 | Estimated: measured rate |
| Cross-AZ traffic and ECR storage | $1.15 | Estimated: tasks in two AZs talking to one database writer; about 20 images of 63 MB |
| NAT gateway hours | $0.00 | Measured: the existing shared NAT is already paid by base infrastructure |
| **Fixed subtotal** | **about $246** | Estimated |

### Volume-driven cost

There is no measured Vimeo volume in this repository, mpd-api docs or the shared MPD knowledge base. The production `mpd-vimeo-videos` bucket holds one object of 38.8 KB, measured from CloudWatch S3 storage metrics for July to September 2026, so Vimeo media is not in AWS today. Client notes cite roughly 200 to 245 active patients at different dates in 2026, and each patient receives doctor results videos.

Assumptions, all estimated:

- 2.5 Mbps average stored bitrate, so a 10-minute recording is about 0.19 GB.
- Each recording is stored twice: the current object plus a retained noncurrent version, because versioning has no expiry. This is a ceiling.
- Each recording is played in full three times, by the patient, a family member and a staff preview. Playback uses presigned S3 URLs, so egress is S3 transfer out, not CloudFront.
- Without an S3 gateway endpoint, tasks move each recording about three times through NAT: read for transcode, read for transcription and write back.

| Scenario | Recordings a month | Stored growth a month | S3 storage, month 1 / month 12 | Egress | NAT processing | Month 1 total | Month 12 total |
|---|---|---|---|---|---|---|---|
| Pilot | 50 of 10 min | 19 GB | $0.47 / $5.62 | $3.21 | $1.66 | about $251 | about $257 |
| Base, Vimeo replacement | 300 of 10 min | 113 GB | $2.81 / $33.75 | $19.24 | $9.96 | about $278 | about $309 |
| High | 1,000 of 15 min | 563 GB | $14.06 / $168.75 | $96.19 | $49.78 | about $407 | about $562 |

S3 request charges stay under $1.10 a month in every scenario. By month 12 the base case holds about 1.35 TB, and the high case about 6.75 TB.

### Main drivers and options

- Fargate is about 62% of the base month-1 total. Two always-on 2 vCPU tasks exist for transcription headroom and AZ redundancy, not for request load. A one-year Compute Savings Plan would lower this, but none is assumed.
- Egress and storage grow with volume and become the largest variable items in the high case.
- Proposed, awaiting operator approval: db.t4g.small Multi-AZ instead of micro, adding $37.23 a month. Staging's 74 MB freeable-memory low on one task suggests two tasks will strain 1 GiB.
- Proposed, awaiting operator approval: an S3 gateway endpoint in the production VPC. It has no hourly charge and removes the NAT processing line, $9.96 a month in the base case. It is a `base-infrastructure` change and also keeps task S3 traffic working during a NAT outage.
- Not proposed: a second NAT gateway, $43.07 a month at list price. It belongs to `base-infrastructure` and would serve every production workload, so it is listed as an open decision.

## 3. Proposed recovery objectives

Proposed, awaiting operator approval:

| Objective | Proposal |
|---|---|
| RPO, database | 15 minutes |
| RPO, media | Zero for acknowledged uploads. Clients write directly to versioned S3 with a signed `If-None-Match: *` condition |
| RTO | 4 hours from alarm to verified playback on restored state |
| RDS automated backups | 35 days, the RDS maximum, with point-in-time recovery |
| S3 | Versioning on, no expiry of current or noncurrent versions, no lifecycle deletion, until a clinical retention policy is approved |
| Retention workers | Automatic source deletion stays disabled in workers and settings, as the runbook records |
| Staging | Seven-day backups unchanged |

### Basis for the numbers

- RDS point-in-time recovery usually trails by about five minutes. Measured once on staging at 06:16:22 UTC, `LatestRestorableTime` was 3 minutes 51 seconds behind. A single sample does not establish production RPO.
- The `BackupAgeSeconds` alarm fires at 900 seconds, so a lag beyond the 15-minute RPO alerts within about ten minutes, two five-minute periods.
- The RTO budget, all estimated and unmeasured:
  - About 10 minutes for alarm detection.
  - 30 minutes for the decision.
  - 20 to 40 minutes for a point-in-time restore of a database under 5 GB.
  - 15 minutes for cutover and task replacement, including the 600-second health grace period and the model download.
  - 30 minutes for verification.

  That totals about 2 hours, inside the 4-hour proposal.

### Gaps the AWS drill in task 3.2 must close

- No documented cutover. A point-in-time restore creates a new instance. Tasks read `DB_HOST` from `sendrec-production/rds-credentials`, so one possible cutover updates that secret's host and forces a new deployment. The next stack update would then rewrite the host through the secret attachment. The drill must prove a cutover and record it.
- Database and media consistency. Restoring the database to time T leaves S3 objects written after T as unreferenced orphans, which is harmless. Objects deleted after T survive only as noncurrent versions, and the drill must restore the versions the restored rows reference.
- Regional loss. Sydney residency rules out cross-region copies without a separate decision, so a regional outage has no RTO.
- Account compromise. Backups live in the same account as the database. Cross-account snapshot copies or an AWS Backup vault lock are not configured.

### What the existing scripts verify

`infrastructure/backup-probe.py` runs every five minutes as a Lambda. It reports `BackupAgeSeconds` and `ServiceDesiredCount`:

- `BackupAgeSeconds` is the gap from `LatestRestorableTime` to now. It reports 86,400 when retention is below 35 days or the read fails.
- `ServiceDesiredCount` reports 1 when the ECS read fails, so an unknown state counts as active.

The probe does not test restorability, snapshot presence, S3 versioning or object integrity.

`infrastructure/restore-drill.py` runs locally only. It never contacts AWS. Inside a network-isolated PostgreSQL 18 container it applies migrations, inserts synthetic state, then dumps and restores it. It verifies:

- matching hashes and ffmpeg decoding for two media versions
- caption cues
- correct and incorrect password checks
- the external identity row
- the current media version
- at least three pending outbox events, one of which it claims

Its output marks production RPO and RTO as unverified. The 2026-10-08 run measured 0.191 seconds of snapshot age and 1.013 seconds to restore. Those are local container numbers, not AWS evidence.

## 4. Alerts plan

### Alarms in the synthesized production stack

Read from `SendRecStack.addProductionAlarms`. Each alarm evaluates two five-minute periods with greater-than-or-equal comparison, so it fires after about ten minutes. Application alarms are gated on `ServiceDesiredCount > 0`, so they stay quiet while production has zero tasks.

| Alarm | Signal | Threshold | Missing data |
|---|---|---|---|
| UnhealthyTargets | ALB unhealthy host count | 1 | Breaching when tasks are desired |
| Target5xx | ALB target 5xx count | 1 | Not breaching |
| DatabaseCPU | RDS CPU percent | 85 | Breaching |
| UploadFailures | Abandoned upload records older than an hour | 1 | Breaching when tasks are desired |
| ProcessingFailures | Failed transcript records in the last 15 minutes | 1 | Breaching when tasks are desired |
| OldProcessingJobs | Processing or transcription work older than 32 minutes | 1 | Breaching when tasks are desired |
| OutboxOldestSeconds | Age of the oldest pending MPD event | 300 seconds | Breaching when tasks are desired |
| OutboxDeadLetters | Dead or blocked MPD events | 1 | Breaching when tasks are desired |
| BackupAgeSeconds | Recovery lag from the backup probe | 900 seconds | Breaching |

If every task stops, the ALB reports zero unhealthy hosts. The stopped monitor then produces no metrics, and with tasks desired the missing data breaches, so the application alarms catch the outage.

### Gaps, proposed, awaiting operator approval

- Alarms have alarm actions only. No OK action is set, so the destination never hears that an alarm cleared. Design decision 3 asks that recovery be verified at the destination. Adding OK actions is a small CDK change for task 2.4.
- No alarm watches RDS free storage. The instance has 20 GB and no storage autoscaling. Proposed: alarm below 4 GB free.
- No alarm watches RDS freeable memory. Proposed: alarm below 64 MB, given the staging low of 74 MB.
- Target5xx at a single error over two periods may be noisy at pilot volume. Proposed: keep it for the pilot and review after two weeks of data.

### Operational destination, an open decision

`ProductionConfig` requires an existing SNS topic ARN in `891612549531`, so the topic must exist before synthesis.

- Measured: the account's existing topics are `cpc-lambda-alerts`, `failed_lambda_functions`, `pipedrive-reconciler-alerts` and `pipedrive-stripe-writeback-alerts`. All belong to other workloads.
- Proposed, awaiting operator approval: a dedicated topic named `sendrec-production-alerts`. Subscribe a shared operations mailbox and, if wanted, a Slack channel through AWS Chatbot.

The operator still needs to name the recipients, decide whether out-of-hours response is expected and approve creating the topic. Task 2.4 then injects a synthetic fault and confirms receipt and recovery at that destination.

## 5. Open decisions for the operator

1. Approve or change the account, VPC reuse, subnet placement and bucket name in section 1.
2. Choose the alert destination: topic name, recipients and out-of-hours expectations. Approve creating the topic.
3. Choose the hostname: launch on `cloudfront.net` or a custom domain before patients receive links.
4. Decide the image path into production: cross-account ECR replication from staging or a production build repository. Create the ECR repository either way.
5. Approve the mpd-api production changes the identity chain needs: a staff-pool hosted sign-in domain, the SendRec callback and the production integration secret. Confirm whether staging's reuse of the `mpd-staff-ehr` client is intended.
6. Choose the RDS class: db.t4g.micro as synthesized, or db.t4g.small for $37.23 a month more.
7. Decide whether to ask `base-infrastructure` for an S3 gateway endpoint, and whether to add a second NAT gateway.
8. Approve or change the recovery objectives: RPO 15 minutes, RTO 4 hours, 35-day backups, indefinite versioned media. Decide whether backups also need cross-account protection.
9. Accept the base cost estimate of about $278 in month 1 and about $309 by month 12. Better still, supply real monthly Vimeo recording counts and durations so the volume rows can be replaced with measured figures.
