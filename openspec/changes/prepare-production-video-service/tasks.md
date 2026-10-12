## 1. Release prerequisites

- [ ] 1.1 Reconcile and close the existing staging change using actual merged-source review and verification evidence; validate and archive it without inventing the missing final review.
  - Pending: prior staging change still lacks final merged-source review and complete browser/automatic-recovery evidence; no archive.
- [ ] 1.2 Confirm all four integration changes are human-merged and deployed to staging with matching contracts; verify their revisions, disabled production settings and synthetic evidence.
  - Local producer/consumer drill passed, but human merges and deployed staging revisions remain pending, including mpd-api PR 211.
- [ ] 1.3 Capture the production account/VPC, identity references, operational alert destination, cost estimate and proposed recovery objectives for operator review; verify no live deployment is implied.
  - Configuration requires explicit references and the runbook records proposed RPO/RTO. The draft in `docs/production/operator-review.md` records the production account/VPC, identity references, cost estimate, recovery objectives and alert plan from read-only AWS reads on 2026-10-12. Production Cognito pool and SendRec client IDs, the alert destination and operator review remain pending.

## 2. Infrastructure and release controls

- [x] 2.1 Extend Java CDK with isolated production configuration, private encrypted resources and exact origins; verify synthesis and automated infrastructure assertions.
  - Verified locally: six Java assertions pass and synthetic production synthesis shows private encrypted resources, exact CloudFront origins, separate VPC/bucket, zero application tasks and disabled integration flags.
- [ ] 2.2 Add immutable-image promotion, a single migration task, staging checks and a manual production approval; verify failed readiness triggers a safe rollback rehearsal.
  - Implemented digest-bound build/promotion, one migration task and production-environment approval gate. Controlled tests prove failed-readiness rollback without database reversal. Actual staging promotion, reviewer configuration and ECS rollback rehearsal remain pending.
- [ ] 2.3 Configure proposed multi-task and Multi-AZ operation; inventory every startup worker and use atomic claims or single-leader leases, or explicitly disable unsupported optional workers. Test media, stuck recovery, cleanup, retention, digest/onboarding email and legacy webhooks with controlled sinks and interrupted leaders before scaling.
  - Worker controls and disabled optional sends are inventoried in the runbook. Local concurrency/recovery checks exist; final full-worker evidence and two-task staging interruption checks remain pending before scaling.
- [ ] 2.4 Add health/readiness and backlog/processing/backup alarms with redacted logs; inject synthetic faults and verify the configured operator actually receives alerts.
  - Implemented dependency readiness, numeric probes and nine alarms. Local fault tests pass. Actual SNS destination and synthetic alarm receipt/recovery remain pending.

## 3. Recovery and client verification

- [ ] 3.1 Configure reviewed production backup/versioning retention without automatic source-media deletion; verify managed user/workspace retention changes are refused and stale nonzero retention_days plus old warnings cannot cause worker deletion or later S3 purge. Keep staging and retained legacy resources untouched.
  - Synthesis verifies 35-day production backups, versioned media with no expiry, and unchanged seven-day staging backups. Application retention guards are implemented; final stale-setting cleanup evidence and reviewed production policy remain pending.
- [ ] 3.2 Restore a synthetic database and media set into isolation; measure RPO/RTO and verify decoding, passwords, captions, identities and pending-event recovery against the proposed objectives.
  - Local isolated PostgreSQL/media restore passed, including two decoded versions, passwords, captions, identity and pending-event claim. Measured 0.191s snapshot age and 1.013s restoration do not establish AWS RPO/RTO; isolated RDS/S3 restore and controlled event delivery remain pending.
- [ ] 3.3 Complete the Chrome/Safari/Firefox recording/upload/watch/embed matrix with actual browser/OS versions, denied-device and interrupted-network cases; leave unavailable checks explicitly incomplete.
  - No Chrome, Safari or Firefox browser matrix was run for this implementation. All requested browser/OS and denied-device/interrupted-network evidence remains pending.
- [ ] 3.4 Exercise the synthetic future-client contract, offboarding, service-token rotation and safe deployment rollback; verify no Staff OS UI or live sending integration is required.
  - Real Go producer/HTTP adapter and Python consumer/DB integration passed locally, including lost-response retry, publication, playback and deletion. Actual staff offboarding, service-token rotation across deployed services and ECS rollback remain pending.

## 4. Production handoff

- [ ] 4.1 Publish the measured evidence, immutable revisions, configuration diff and operating runbook; verify secret scans and strict OpenSpec validation pass.
  - Runbook and local measurements are recorded. Final immutable revisions, reviewed configuration diff, secret scan and strict validation evidence remain pending at this reconciliation.
- [ ] 4.2 Open the readiness implementation PR and complete review/CI; present the concrete production rollout for explicit operator approval before deployment.
  - Implementation review/CI and the concrete rollout approval request remain pending. This preparation does not authorize deployment.
- [ ] 4.3 After separate rollout approval, deploy the empty production service and verify approved pilot checks; record the exact live state and preserve Vimeo service and retained resources.
  - Not authorized or run. Production remains unactivated; no pilot or retained-resource changes occurred.

Delivery note: source, local evidence and the runbook are committed and pushed. Staged secret scans and all three strict validations passed. The operator restored GitHub workflow access and [implementation PR 4](https://github.com/My-Performance-Doctor/sendrec/pull/4) is open. Application, Docker build and infrastructure CI passed; the review helper has no completed round after three failed attempts. Recovered findings have fixes; final-revision checks are tracked in the PR. Browser E2E and preview deployment skipped under the PR workflow. Task 4.2 also requires a concrete rollout approval request after the remaining evidence. No production deployment occurred.
