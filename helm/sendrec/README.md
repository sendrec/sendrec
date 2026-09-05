# SendRec Helm chart

Kubernetes deployment for [SendRec](https://github.com/sendrec/sendrec) - open-source async video messaging.

## What the chart deploys

| Resource | Condition |
| --- | --- |
| `Deployment` | always |
| `ConfigMap` (`sendrec-configmap`) | always - every non-secret env var |
| `Secret` (`sendrec-secret`) | unless `sendrec.existingSecret` is set |
| `Service` | always |
| `Ingress` | `sendrec.ingress.enabled` |
| `NetworkPolicy` | `sendrec.networkPolicy.enabled` |
| `PersistentVolumeClaim` | local transcription with `sendrec.transcription.type: volume` |
| anything in `sendrec.extraResources` | when non-empty |

The app container gets its configuration through `envFrom`, pulling in the whole ConfigMap and the whole Secret.

## Requirements

- Kubernetes cluster with an ingress controller
- Helm
- PostgreSQL database (migrations run automatically on startup)
- S3-compatible object storage (Garage, MinIO, AWS S3, R2, Wasabi, Backblaze, ...)

## Quick start

Verify your configuration in `values.yaml`:

```yaml
sendrec:
  env:
    baseUrl: "https://sendrec.example.com"
    s3Endpoint: "https://s3.eu-central-1.amazonaws.com"
    s3PublicEndpoint: "https://s3.eu-central-1.amazonaws.com"
    s3Bucket: "recordings"
    s3Region: "eu-central-1"

  ...

  secrets:
    databaseUrl: "postgres://sendrec:secret@postgres:5432/sendrec?sslmode=require"
    jwtSecret: "<openssl rand -hex 32>"
    s3AccessKey: "..."
    s3SecretKey: "..."

  ...

  ingress:
    hosts:
      - name: sendrec.example.com
        path: /
    tls:
      - hosts:
          - sendrec.example.com
        secretName: sendrec-tls
```

Deploy the application:

```bash
helm install sendrec ./helm/sendrec -n sendrec --create-namespace -f values.yaml
```

Upgrade the application on changes:

```bash
helm upgrade sendrec ./helm/sendrec -n sendrec -f values.yaml
```

### Required values

Rendering fails with an explicit message if any of these are empty:

`sendrec.env.baseUrl`, `sendrec.env.s3Endpoint`, `sendrec.env.s3Bucket`, and - unless `sendrec.existingSecret` is set - `sendrec.secrets.databaseUrl`, `sendrec.secrets.jwtSecret`, `sendrec.secrets.s3AccessKey`, `sendrec.secrets.s3SecretKey`.

### Using an existing Secret

Set `sendrec.existingSecret: my-secret` to skip the chart-managed Secret (useful with External Secrets, Sealed Secrets, SOPS, Infisical, …). The referenced Secret must contain the same keys the chart's Secret would have - see [Secrets](#secrets). The deployment then annotates `checksum/secret: "external-secret"`, so rotating that Secret does **not** trigger a rollout on its own.

## Environment variables

Every key under `sendrec.env` renders into the ConfigMap unconditionally. **An empty string behaves exactly like "unset"** - the app falls back to its own default - so leaving a key `""` is always safe.

The **Default** column below is what this chart ships in `values.yaml`, which is not always what the app would do on its own. Where the two differ, the app's own default follows in parentheses.

### Core

| Values key | Env var | Meaning | Default |
| --- | --- | --- | --- |
| `env.baseUrl` | `BASE_URL` | Public URL of the app. Used for CORS, cookies, share links and OAuth redirects. No trailing slash | *required* |
| `port` | `PORT` | Container listen port; also drives the Service and probes | `8080` |

### Storage (S3-compatible)

| Values key | Env var | Meaning | Default |
| --- | --- | --- | --- |
| `env.s3Endpoint` | `S3_ENDPOINT` | API endpoint the **server** talks to; may be a cluster-internal address | *required* |
| `env.s3PublicEndpoint` | `S3_PUBLIC_ENDPOINT` | Endpoint used to sign presigned URLs the **browser** must reach. Falls back to `S3_ENDPOINT`, which breaks whenever the internal address isn't publicly resolvable | `""` |
| `env.s3Bucket` | `S3_BUCKET` | Bucket holding videos, thumbnails and transcripts | *required* |
| `env.s3Region` | `S3_REGION` | Must match the region configured on the storage side | `""` (app: `eu-central-1`) |
| `env.awsRequestChecksumCalculation` | `AWS_REQUEST_CHECKSUM_CALCULATION` | Keep `when_required` for non-AWS providers; the AWS SDK default sends checksums many S3 clones reject | `when_required` |
| `env.awsResponseChecksumValidation` | `AWS_RESPONSE_CHECKSUM_VALIDATION` | Same, for responses | `when_required` |

Video bytes never pass through the server - the browser uploads straight to storage with presigned URLs.

### Limits

Set any of these to `"0"` for unlimited. The chart ships `"0"` for all three, so a default install enforces no per-user quota; leave them `""` to fall back to the app's free-plan limits instead.

| Values key | Env var | Meaning | Default |
| --- | --- | --- | --- |
| `env.maxUploadBytes` | `MAX_UPLOAD_BYTES` | Max size per recording/upload. Keep the ingress body-size limits in sync | `524288000` (500 MB) |
| `env.maxVideosPerMonth` | `MAX_VIDEOS_PER_MONTH` | Videos a user may create per month | `0` (app: free plan, `25`) |
| `env.maxVideoDurationSeconds` | `MAX_VIDEO_DURATION_SECONDS` | Max recording length | `0` (app: free plan, `300`) |
| `env.maxPlaylists` | `MAX_PLAYLISTS` | Playlists a free-tier user may create | `0` (app: free plan, `3`) |

### Features

| Values key | Env var | Meaning | Default |
| --- | --- | --- | --- |
| `env.apiDocsEnabled` | `API_DOCS_ENABLED` | Serve the interactive API reference at `/api/docs` | `"true"` (app: `"false"`) |
| `env.brandingEnabled` | `BRANDING_ENABLED` | Let users customise watch-page logo, colours, footer and custom CSS | `"true"` (app: `"false"`) |
| `env.registrationEnabled` | `REGISTRATION_ENABLED` | Allow self-signup. `"false"` hides the form *and* disables the API endpoint | `"false"` (app: `"true"`) |
| `env.planBadgeEnabled` | `PLAN_BADGE_ENABLED` | Show the Free/Pro/Business badge next to the logo. Only useful with billing configured | `"false"` |
| `env.analyticsScript` | `ANALYTICS_SCRIPT` | A full `<script>` tag injected into every watch page (Umami, Plausible, Matomo, …). The CSP nonce is added automatically | `""` |
| `env.allowedFrameAncestors` | `ALLOWED_FRAME_ANCESTORS` | **Extra** space-separated CSP `frame-ancestors` origins; the app always prepends `'self'`. Widen it to embed SendRec in Nextcloud, a wiki, etc. | `""` |

### Security

| Values key | Env var | Meaning | Default |
| --- | --- | --- | --- |
| `env.trustedProxy` | `TRUSTED_PROXY` | Whether `X-Forwarded-For` may be believed when identifying a client | `"true"` (app default: `"false"`) |
| `env.rateLimitEnabled` | `RATE_LIMIT_ENABLED` | Master switch for all per-IP rate limiting | `"true"` |
| `env.webhookAllowPrivateTargets` | `WEBHOOK_ALLOW_PRIVATE_TARGETS` | Whether user-configured webhooks may resolve to private addresses | `"false"` |

**`trustedProxy` is the one to think about, and the chart deliberately differs from the app here.** The app defaults it to `"false"`; the chart ships `"true"`, because the chart's own defaults (`ingress.enabled: true`, `service.type: ClusterIP`) describe a pod that is only reachable through a proxy. The client address keys both rate limiting and unique-viewer dedup. Left `"false"` behind an ingress controller, every request appears to come from the controller's pod IP, so all users share a single rate-limit bucket and one busy client throttles everyone. Set it to `"true"` when the only path to the pod is a proxy you control that always sets `X-Forwarded-For` (Traefik, nginx and Caddy all do) - the app then trusts the *last* entry in that header. Do **not** set it if pods are reachable directly, because then clients can forge the header and evade limiting entirely. Pairing it with a `NetworkPolicy` that admits only your ingress namespace makes the assumption enforceable.

If you expose the Service directly (`service.type: LoadBalancer` or `NodePort`), set `trustedProxy: "false"`. The chart prints a warning on install and upgrade when it detects that combination.

Rate limits are fixed per route group (token bucket, requests/second + burst): auth and watch-page auth `0.5/5`, video writes `2/10`, comment writes `0.2/3`, comment reads and watch pages `5/20`. Rejected requests get `429` with `Retry-After: 10`. Only disable rate limiting in test/e2e stacks where the whole suite shares one source IP.

`webhookAllowPrivateTargets` guards an SSRF surface: users configure their own webhook URLs. Left `"false"`, targets resolving to loopback, RFC1918, link-local (including the `169.254.169.254` cloud metadata endpoint), multicast or unspecified addresses are rejected, and plaintext `http://` targets are refused. Only turn it on for local development.

### Transcription

| Values key | Env var | Meaning | Default |
| --- | --- | --- | --- |
| `env.transcriptionEnabled` | `TRANSCRIPTION_ENABLED` | Master switch. When off, uploads simply skip transcription | `"false"` |
| `env.transcriptionProvider` | `TRANSCRIPTION_PROVIDER` | `local` (whisper.cpp in the container), `openai` (any OpenAI-compatible API) or `deepgram` | `local` |
| `env.transcriptionApiUrl` | `TRANSCRIPTION_API_URL` | Base URL for OpenAI-compatible providers, e.g. `https://api.groq.com/openai`. Empty means `https://api.openai.com` | `""` |
| `env.transcriptionModel` | `TRANSCRIPTION_MODEL` | Model override. Empty uses `whisper-1` (openai) or `nova-3` (deepgram) | `""` |
| `env.transcriptionTimeoutSeconds` | `TRANSCRIPTION_TIMEOUT_SECONDS` | HTTP timeout for cloud providers | `""` (300) |
| `env.whisperModelPath` | `WHISPER_MODEL_PATH` | Model file path inside the container, `local` only | `/models/ggml-small.bin` |
| `secrets.transcriptionApiKey` | `TRANSCRIPTION_API_KEY` | API key for `openai`/`deepgram` | `""` |

`local` is CPU-bound and runs on the app pod - size `resources` accordingly. The cloud providers need no extra storage.

#### Local model storage

When `transcriptionEnabled: "true"` **and** the provider is `local` (or empty), the chart adds a `download-transcription-model` init container that fetches the model unless it is already present, plus a `/models` volume shared with the app container:

| Values key | Meaning | Default |
| --- | --- | --- |
| `transcription.type` | `emptyDir` (re-downloaded on every pod start) or `volume` (creates a `ReadWriteOnce` PVC that survives restarts) | `emptyDir` |
| `transcription.size` | `emptyDir` size limit, or PVC storage request | `1Gi` |
| `transcription.initImage` | Image used for the download | `alpine/curl:8.17.0` |
| `transcription.modelUrl` | Where to fetch the model from | ggml-small.bin on Hugging Face |
| `transcription.modelPath` | Download target; keep it equal to `env.whisperModelPath` | `/models/ggml-small.bin` |

The default model is ~466 MB, so `emptyDir` means re-downloading it on every restart - prefer `volume` in production.

### AI summaries

| Values key | Env var | Meaning | Default |
| --- | --- | --- | --- |
| `env.aiEnabled` | `AI_ENABLED` | Generate summaries and chapter markers after transcription | `"false"` |
| `env.aiBaseUrl` | `AI_BASE_URL` | Any OpenAI-compatible endpoint (Mistral, OpenAI, Ollama, …) | `""` |
| `env.aiModel` | `AI_MODEL` | Model name | `mistral-small-latest` |
| `env.aiTimeout` | `AI_TIMEOUT` | Request timeout in Go duration format (`60s`, `5m`). Raise it for slow local endpoints | `60s` |
| `secrets.aiApiKey` | `AI_API_KEY` | Provider API key; leave empty for local Ollama | `""` |

Only transcribed videos are summarised, so AI requires transcription.

### Audio post-processing

| Values key | Env var | Meaning | Default |
| --- | --- | --- | --- |
| `env.noiseReductionFilter` | `NOISE_REDUCTION_FILTER` | ffmpeg audio filter appended as `-af` during transcode/normalize | `""` |

Two gates apply: the filter runs only if this value is non-empty **and** the individual user enabled noise reduction in their settings. A non-empty value is also what surfaces the toggle in the UI, so `""` hides the feature entirely. Useful values: `arnndn=m=/app/models/std.rnnn` (RNNoise; the model ships in the image, better quality, more CPU) or `afftdn=nr=12:nf=-50` (cheaper FFT denoiser). Either one adds work to every transcode job.

### Social login / SSO

| Values key | Env var | Meaning |
| --- | --- | --- |
| `secrets.googleClientId` / `googleClientSecret` | `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` | Google OAuth 2.0 credentials |
| `secrets.microsoftClientId` / `microsoftClientSecret` | `MICROSOFT_CLIENT_ID` / `MICROSOFT_CLIENT_SECRET` | Microsoft (Entra ID) credentials |
| `secrets.githubSsoClientId` / `githubSsoClientSecret` | `GITHUB_SSO_CLIENT_ID` / `GITHUB_SSO_CLIENT_SECRET` | GitHub OAuth App credentials |
| `env.googleAuthAllowedDomains` | `GOOGLE_AUTH_ALLOWED_DOMAINS` | Comma-separated email domains allowed to sign in with Google. Exact match - `example.com` does not cover `mail.example.com`. Empty allows any Google account |

Providers without credentials are not advertised on the login screen. Register the redirect URI `<baseUrl>/api/auth/sso/{google,microsoft,github}/callback` in each provider's console. Workspace-level SAML and per-organisation OIDC are configured in-app, not here.

### Email

The app picks a backend in this order: Listmonk (if `listmonkBaseUrl` is set) → SMTP (if `smtpHost` is set) → sendmail (only if `emailUseSendmail: "true"`). With none configured, signup skips email confirmation and new users can log in immediately.

| Values key | Env var | Meaning | Default |
| --- | --- | --- | --- |
| `env.listmonkBaseUrl` | `LISTMONK_URL` | Listmonk instance URL | `""` |
| `secrets.listmonkUsername` / `listmonkPassword` | `LISTMONK_USER` / `LISTMONK_PASSWORD` | Listmonk API credentials | `""` |
| `env.listmonkTemplateId` | `LISTMONK_TEMPLATE_ID` | Share-link emails | `""` |
| `env.listmonkCommentTemplateId` | `LISTMONK_COMMENT_TEMPLATE_ID` | New-comment notifications | `""` |
| `env.listmonkViewTemplateId` | `LISTMONK_VIEW_TEMPLATE_ID` | View notifications and digest | `""` |
| `env.listmonkConfirmTemplateId` | `LISTMONK_CONFIRM_TEMPLATE_ID` | Signup confirmation | `""` |
| `env.listmonkWelcomeTemplateId` | `LISTMONK_WELCOME_TEMPLATE_ID` | Welcome email after confirmation | `""` |
| `env.listmonkOnboardingDay2TemplateId` | `LISTMONK_ONBOARDING_DAY2_TEMPLATE_ID` | Day-2 onboarding | `""` |
| `env.listmonkOnboardingDay7TemplateId` | `LISTMONK_ONBOARDING_DAY7_TEMPLATE_ID` | Day-7 onboarding | `""` |
| `env.listmonkOrgInviteTemplateId` | `LISTMONK_ORG_INVITE_TEMPLATE_ID` | Workspace invitations | `""` |
| `env.listmonkRetentionWarningTemplateId` | `LISTMONK_RETENTION_WARNING_TEMPLATE_ID` | Retention/auto-delete warnings | `""` |
| `env.smtpHost` | `SMTP_HOST` | SMTP relay hostname | `""` |
| `env.smtpPort` | `SMTP_PORT` | Relay port | `""` (587) |
| `env.smtpTls` | `SMTP_TLS` | `starttls`, `tls` (implicit, port 465), `auto` or `none`. Unrecognised values are coerced to `starttls` with a warning so a typo cannot silently downgrade to plaintext | `""` (starttls) |
| `secrets.smtpUsername` / `smtpPassword` | `SMTP_USERNAME` / `SMTP_PASSWORD` | Relay credentials; omit both for unauthenticated relays | `""` |
| `env.emailUseSendmail` | `EMAIL_USE_SENDMAIL` | `"true"` to use the local `sendmail(8)` binary, also as a fallback when Listmonk fails | `""` |
| `env.emailFromAddress` | `EMAIL_FROM_ADDRESS` | `From:` address for both backends | `""` (`noreply@sendrec.eu`) |
| `env.emailAllowlist` | `EMAIL_ALLOWLIST` | Comma-separated allowed recipient domains (`@example.com`) and addresses. Confirmation, welcome, onboarding, invite and retention mail bypass it. Handy for staging | `""` |
| `env.developerEmail` | `DEVELOPER_EMAIL` | Redirects **every** outgoing email to this address and bypasses the allowlist. Staging only - in production it silently swallows all user mail | `""` |

Template IDs are optional: without one, a plain HTML fallback is sent.

### Billing (Creem)

Billing only initialises when `secrets.creemApiKey` is set; otherwise every user is on the free tier and no billing UI appears.

| Values key | Env var | Meaning |
| --- | --- | --- |
| `secrets.creemApiKey` | `CREEM_API_KEY` | Creem API key. `creem_test_` keys auto-route to the test API |
| `secrets.creemWebhookSecret` | `CREEM_WEBHOOK_SECRET` | Webhook signing secret (HMAC-SHA256) |
| `env.creemProProductId` | `CREEM_PRO_PRODUCT_ID` | Personal Pro plan product |
| `env.creemBusinessProductId` | `CREEM_BUSINESS_PRODUCT_ID` | Personal Business plan product |
| `env.creemOrgProProductId` | `CREEM_ORG_PRO_PRODUCT_ID` | Workspace Pro plan; falls back to the personal Pro product when empty |
| `env.creemOrgBusinessProductId` | `CREEM_ORG_BUSINESS_PRODUCT_ID` | Workspace Business plan; falls back to the personal Business product when empty |

All product IDs must be recurring/subscription products. They are also used in reverse to map incoming webhook events back to a plan. Point Creem at `<baseUrl>/api/webhooks/creem` and subscribe to the `subscription.*` events.

> **The app refuses to start** when `CREEM_API_KEY` is set without `CREEM_WEBHOOK_SECRET` - an empty secret would make every billing webhook forgeable.

### Secrets

Rendered into `sendrec-secret` unless `existingSecret` is set: `DATABASE_URL`, `JWT_SECRET`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `MICROSOFT_CLIENT_ID`, `MICROSOFT_CLIENT_SECRET`, `GITHUB_SSO_CLIENT_ID`, `GITHUB_SSO_CLIENT_SECRET`, `AI_API_KEY`, `TRANSCRIPTION_API_KEY`, `LISTMONK_USER`, `LISTMONK_PASSWORD`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `CREEM_API_KEY`, `CREEM_WEBHOOK_SECRET`.

`jwtSecret` signs auth tokens: generate it with `openssl rand -hex 32`, and note that changing it invalidates every session. The app refuses to start without one, regardless of `BASE_URL`.

## Workload configuration

| Values key | Meaning | Default |
| --- | --- | --- |
| `replicas` | Pod count. The app is stateless (state lives in Postgres and S3), but transcode/transcription jobs run in-process | `1` |
| `image.repository` / `image.tag` / `image.pullPolicy` | Container image. An empty tag resolves to `v<appVersion>` from `Chart.yaml` | `ghcr.io/sendrec/sendrec` / `""` / `Always` |
| `deploymentStrategy` | Passed through to the Deployment | `RollingUpdate` 25%/25% |
| `resources` | Requests/limits. See [Sizing the pod](#sizing-the-pod) | `200m` / `512Mi` requests, no limit |
| `livenessProbe` / `readinessProbe` | Passed through as-is; both hit `/api/health` | see `values.yaml` |
| `deployment.extraInitContainers` | Extra init containers, appended after the model downloader | `[]` |
| `deployment.extraContainers` | Sidecars | `[]` |
| `deployment.extraVolumes` / `deployment.extraVolumeMounts` | Extra pod volumes and mounts on the app container | `[]` |
| `extraResources` | Arbitrary manifests rendered verbatim (CronJob, ConfigMap, ServiceMonitor, …) | `[]` |

The Deployment carries `checksum/configmap` and `checksum/secret` annotations, so config changes roll pods automatically.

### Sizing the pod

**Editing has a memory floor, and it is well above what an HTTP server needs.** Transcode, normalize, trim, remove-segments and composite all run ffmpeg in this pod. Measured peak RSS for a single 1080p30 x264 encode, ffmpeg 7.1:

| | peak RSS | wall |
| --- | --- | --- |
| ffmpeg defaults | 524 MB | 34s |
| the bounded encoder settings the app now passes | 342 MB | 23s |
| stream copy, no encode | 35 MB | - |

Reproduce with `hack/encoder-memory/run.sh`. That figure is the ffmpeg process alone, not the pod: the Go server sits alongside it, and nothing bounds how many encodes run at once.

The chart requests `512Mi` and sets no memory limit. The request is what gets the pod scheduled somewhere it can actually finish an encode; the absent limit is deliberate, because the app does not bound how many encodes run at once, so any default limit would be a hard kill threshold guessed without knowing your inputs. Set `resources.limits.memory` once you have measured your own workload.

`512Mi` is not enough for everything:

- **4K or high-DPI sources** scale roughly with pixel count. The app scales down to 1080p during transcode, but the decode side still holds full-resolution frames.
- **Local transcription** runs whisper.cpp in the same pod and is both CPU and memory hungry.
- **Noise reduction** adds an ffmpeg filter to every transcode.
- **Concurrent jobs.** Nothing bounds how many edits run at once, so two simultaneous 1080p edits want roughly twice the memory.

Under-provisioning does not fail gracefully: the OOM killer takes the whole pod, so an edit triggered by one user drops every in-flight request. If you cannot give the pod headroom, keep `replicas: 1` and expect edits on large recordings to fail.

## Networking

| Values key | Meaning | Default |
| --- | --- | --- |
| `service.type` / `service.port` | Service exposure | `ClusterIP` / `8080` |
| `ingress.enabled` | Render an Ingress | `true` |
| `ingress.className` | Ingress class | `traefik` |
| `ingress.annotations` | Controller annotations. The defaults raise Traefik's request/response buffering to 512 MB to match `maxUploadBytes` | see `values.yaml` |
| `ingress.hosts` | List of `{name, path}` | `sendrec.yourdomain.com` |
| `ingress.pathType` | Path matching | `ImplementationSpecific` |
| `ingress.tls` | Standard Ingress TLS blocks | `[]` |

If you raise `maxUploadBytes`, raise the matching ingress limits too - otherwise the controller rejects large uploads before the app sees them. On non-Traefik controllers, replace the buffering annotations with the equivalent (for example `nginx.ingress.kubernetes.io/proxy-body-size`).

### NetworkPolicy

| Values key | Meaning | Default |
| --- | --- | --- |
| `networkPolicy.enabled` | Render the policy | `false` |
| `networkPolicy.policyTypes` | Policy types | `[Ingress]` |
| `networkPolicy.ingress` | Rendered verbatim into `spec.ingress`; empty means deny-all for that direction | `{}` |
| `networkPolicy.egress` | Rendered verbatim into `spec.egress` | `{}` |

Restricting ingress to your ingress-controller namespace is what makes `trustedProxy: "true"` safe:

```yaml
sendrec:
  networkPolicy:
    enabled: true
    policyTypes:
      - Ingress
    ingress:
      - from:
          - namespaceSelector:
              matchLabels:
                kubernetes.io/metadata.name: traefik
```

Note that enabling `Egress` without rules blocks everything outbound, including DNS, Postgres and S3.

## Uninstall

```bash
helm -n sendrec uninstall sendrec
```

This deletes everything the release owns, including the whisper model PVC - add `helm.sh/resource-policy: keep` to it beforehand if you want to keep the downloaded model. Postgres and object storage are external and untouched.

## See also

- [SELF-HOSTING.md](../../SELF-HOSTING.md) - provider notes and Docker Compose setup. It predates several of the variables above (`RATE_LIMIT_ENABLED`, `WEBHOOK_ALLOW_PRIVATE_TARGETS`, `NOISE_REDUCTION_FILTER`, `DEVELOPER_EMAIL`, the workspace Creem product IDs), so treat this page as the current reference for anything under `sendrec.env`
- [Releases](https://github.com/sendrec/sendrec/releases) - changelog
