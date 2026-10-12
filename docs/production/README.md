# Production preparation

Production is not activated. Jeff is reviewing mpd-api PR 211, and the matching integration has not run on staging. The production stack creates zero application tasks. Its configuration requires an explicit Sydney account, separate VPC and bucket, immutable ECR image and operational SNS topic. The existing staging backup retention stays at seven days.

The production account, network, identity references, cost estimate, recovery objectives and alert destination are drafted for operator review in [operator-review.md](operator-review.md). That draft is a proposal, not an approval or a deployment.

## Release

`MPD immutable candidate` builds a main-branch image into an ECR repository that enforces immutable tags. It records the source revision and digest without deploying. The inherited upstream Docker Hub/Coolify release workflow only runs in the upstream repository.

The operator prepares a reviewed manifest with these fields:

```json
{
  "revision": "<full-source-commit>",
  "image": "<account>.dkr.ecr.ap-southeast-2.amazonaws.com/sendrec@sha256:<digest>",
  "infrastructureSha256": "<sha256-of-canonical-template-array>",
  "staging": {
    "stack": "<staging-stack-name>",
    "templatePath": "<reviewed-staging-template-path>",
    "cluster": "<staging-cluster-arn>",
    "service": "<staging-service-arn>",
    "applicationTaskDefinition": "<full-application-task-definition-arn>",
    "migrationTaskDefinition": "<full-migration-task-definition-arn>",
    "subnets": ["<private-subnet-id>"],
    "securityGroups": ["<service-security-group-id>"],
    "readinessUrl": "https://<staging-cloudfront-host>/api/ready",
    "desiredCount": 1
  },
  "production": {
    "stack": "<production-stack-name>",
    "templatePath": "<reviewed-production-template-path>",
    "cluster": "<production-cluster-arn>",
    "service": "<production-service-arn>",
    "applicationTaskDefinition": "<full-application-task-definition-arn>",
    "migrationTaskDefinition": "<full-migration-task-definition-arn>",
    "subnets": ["<private-subnet-id>"],
    "securityGroups": ["<service-security-group-id>"],
    "readinessUrl": "https://<production-cloudfront-host>/api/ready",
    "desiredCount": 2
  }
}
```

A manifest contains references, never credentials. Record the reviewed template hash and cost estimate in the activation PR. Hash the JSON array `[stagingTemplate, productionTemplate]` with sorted keys and compact separators. The promotion script checks that digest and compares the target live CloudFormation template with its reviewed file before migration. Both environments currently consume the same-account digest. A different production account needs reviewed ECR replication and release changes first.

The promotion workflow runs only from main. It checks that the source revision is an ancestor of its workflow revision and that the immutable image tag binds that revision to the requested digest. Configure the staging build/release roles and production release role through GitHub environment variables. Give the release roles only the relevant ECS, ECR read, CloudFormation GetTemplate and PassRole permissions. Their trust policies must match this repository and the corresponding GitHub environment.

Before dispatch, configure the production environment with named reviewers and prevent self-review. The workflow checks that protection exists, deploys staging, stores its digest-bound readiness result, then waits at the production environment approval. These repository settings and roles are not configured by this change.

`infrastructure/promote-release.py` checks registered task images and migration modes before starting exactly one migration task. The migration definition gets database secrets only. Application tasks use `MIGRATIONS_MODE=skip`. After a successful migration, ECS deploys the candidate and the script checks the selected task definition, task count and dependency readiness. A failed candidate restores the previous application definition and count, checks the rollback, and retains the forward migration, recordings and events. A failed migration leaves the service untouched. The release workflow concurrency group prevents overlapping promotions through this workflow; operators must not start a competing release manually.

Infrastructure provisioning remains a separate reviewed operation. Synthesize production with `targetEnvironment=production` plus `productionAccount`, `productionVpcId`, `productionCloudFrontPrefixListId`, `productionBucket`, `productionImage` and `productionAlertTopicArn`. To rehearse the controlled migration path on staging, pass the same digest as `releaseImage`. The task-definition outputs supply the manifest references. Do not deploy this preparation stack or dispatch production without separate rollout approval.

## Integration configuration

`MPD_IDENTITY_ENABLED`, `MPD_SERVICE_ENABLED` and `MPD_EVENTS_ENABLED` default to false in both environments. The stack injects no integration credentials while disabled. A separate activation supplies `activateMpdIntegration=true`, an existing target-account `mpdIntegrationSecretArn`, and these non-secret contexts:

- `MPD_COGNITO_ISSUER`, `MPD_COGNITO_CLIENT_ID`, `MPD_ACCESS_URL`.
- `MPD_WORKSPACE_ID`, `MPD_TENANT_ID`.
- `MPD_ALLOWED_CLIENT_IDS` and `MPD_ALLOWED_ORIGINS`, exact comma-separated allowlists.
- `MPD_EVENT_RECEIVER_URL` and `MPD_EVENT_KEY_ID`.

The referenced Secrets Manager JSON object supplies `MPD_COGNITO_CLIENT_SECRET`, `MPD_SESSION_ENCRYPTION_KEY`, `MPD_SERVICE_TOKEN_HASHES` and `MPD_EVENT_SIGNING_KEY`. Create or rotate those through the approved secret workflow. Do not pass values as CDK contexts. The service-token field contains SHA256 verifiers, not plaintext tokens. Keep production and staging client, workspace, token and signing-key references separate. Provision the approved managed workspace before enabling login. The application identity runbook covers the identity binding and explicit account links.

`MPD_REQUIRE_PASSWORD=true` remains mandatory for the initial pilot. Controlled release tasks set `MPD_MANAGED_MODE=true`, `OPTIONAL_WORKERS_ENABLED=false`, `LEGACY_WEBHOOKS_ENABLED=false` and `AI_ENABLED=false`. Registrations and API documentation stay disabled.

## Workers and monitoring

| Work | Control |
|---|---|
| Transcription | Atomic `SKIP LOCKED` claims, generation increment on recovery and current-media/generation publication checks |
| Transcode/normalize | Unique object keys, file-key compare-and-set and retired-object transaction fence |
| Stuck processing | Atomic status update; reserved media version rejects late edit results |
| Cleanup and thumbnails | Per-key row locks for retired objects, idempotent tombstone purge and thumbnail row leases with media checks |
| MPD outbox | Durable row claims, lease token and retry state |
| AI summary/document | Disabled by `AI_ENABLED=false` |
| Digest/onboarding/retention email | Disabled by `OPTIONAL_WORKERS_ENABLED=false` |
| Legacy user webhooks | Disabled by `LEGACY_WEBHOOKS_ENABLED=false` |

Verify these controls with synthetic overlapping workers before scaling to two tasks. Optional workers remain disabled until their send recovery and deduplication can be demonstrated. Managed source retention is indefinite. The application guard rejects stale retention settings; S3 has no expiry lifecycle.

`/api/ready` checks the database, storage and worker monitoring with a three-second deadline. It returns only check names and booleans. The monitor emits numeric aggregates each minute: failed transcript records updated within the last 15 minutes, abandoned uploads older than an hour, processing or transcription work older than 32 minutes, the oldest retained pending event, and dead or blocked events. `UploadFailures` counts abandoned upload records rather than individual HTTP rejections. Backlog and failure counts trigger alarms without removing healthy API targets from the load balancer. Readiness fails when the monitoring query fails or its last successful sample becomes stale. An idle queue cannot prove a processing loop is alive, so this does not replace worker supervision or recovery tests.

Production alarms cover unhealthy targets, target 5xx, database CPU, abandoned uploads, failed transcripts, old processing, outbox age/dead events and RDS recovery lag. A five-minute probe checks 35-day retention, `LatestRestorableTime` and the ECS service desired count. Application alarms stay quiet while the desired count is zero. When tasks are desired, missing application probes breach; idle target 5xx data does not. Database and backup alarms remain active independently. Unknown service activity is treated as active, and missing backup-probe data triggers its alarm. Every alarm uses the explicitly supplied SNS topic. No alert delivery has been exercised. The activation check must inject a synthetic fault, verify actual receipt and then verify recovery at that destination.

## Recovery evidence

Run the local rehearsal from the repository root:

```bash
python3 infrastructure/restore-drill.py --output /tmp/sendrec-local-restore-evidence.json
```

It creates its own PostgreSQL 18 container with no network or published ports, applies the migrations, and restores a synthetic database dump and two local media versions into separate destinations. It decodes both videos, checks caption cues, verifies password acceptance and rejection, restores the staff identity binding, and claims a retained pending event. It removes only the disposable container and temporary files it created. It does not contact AWS or send an event.

The 2026-10-08 local rehearsal passed. It measured 0.191 seconds from snapshot start to recovery start and 1.013 seconds to restore and validate. Those numbers do not establish production RPO or RTO. The proposed RPO of 15 minutes and RTO of four hours still require operator review and an isolated AWS drill restoring an RDS point in time plus matching S3 object versions. Verify actual protected playback, transcript retrieval, fresh identity authorization and event delivery to a controlled sink after that restore. Record source revisions, digests and measurements without credentials, signed URLs, captions or patient data.

## Local integration evidence

The 2026-10-08 cross-service drill passed in 67.45 seconds. It exercised the real Go producer's SQL events and HMAC delivery through the mpd-api FastAPI webhook into its PostgreSQL inbox. The real Go HTTP adapter supplied metadata, captions, password changes and publication to the consumer's report workflow. The claim route used a supplied synthetic staff principal, so this check does not establish browser login or Cognito authorization.

The receiver intentionally lost the response after its first durable commit. Seven delivery attempts across the recording lifecycle retained one report, one timeline item and one task intent. Staff preview produced no watched mark. Anonymous playback and its duplicate produced one watched result, a second transcript revision updated the existing report, and deletion removed URLs while preserving the clinical timeline. Media bytes and transcript enrichment were synthetic. No external task delivery occurred.

The readiness checks also passed 15 Python tests, six Java infrastructure tests and the bootstrap access positive/negative controls. The actual synthesized production template contains zero application tasks, Multi-AZ RDS, 35-day backups and nine alarms. These results verify local implementation. They do not close the deployment, browser or alert-receipt tasks in [the readiness checklist](../../openspec/changes/prepare-production-video-service/tasks.md).

## Unverified checks

| Check | Current evidence |
|---|---|
| Chrome screen/camera, camera-only, MP4/WebM, denied devices and interrupted network | No browser run for this implementation |
| Safari equivalent recording and protected watch/embed checks | No supported Safari host exercised |
| Firefox equivalent recording and protected watch/embed checks | No browser run for this implementation |
| Signed direct upload, captions and retranscription | Local tests only until staged integration runs |
| Fresh staff offboarding within 60 seconds | Cross-service staging check pending mpd-api merge |
| Production alert receipt | SNS destination and synthetic receipt check pending |
| AWS RDS/S3 restoration and measured recovery objectives | Local rehearsal only |
| Actual ECS candidate failure and rollback | Controlled script tests only |
| Production account/VPC inputs, cost estimate and pilot approval | Drafted in [operator-review.md](operator-review.md); operator review and pilot approval pending |
| Prior staging change closure | Existing missing final review and incomplete browser/automatic recovery evidence remain open |

Keep the OpenSpec tasks and prior staging change open until the corresponding evidence exists. No retained resources or Vimeo content move as part of this change.
