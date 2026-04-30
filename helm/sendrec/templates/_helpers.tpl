{{- define "sendrec.validateRequired" -}}
{{- $_ := required "sendrec.env.baseUrl is required (maps to BASE_URL)" .Values.sendrec.env.baseUrl -}}
{{- $_ := required "sendrec.env.s3Endpoint is required (maps to S3_ENDPOINT)" .Values.sendrec.env.s3Endpoint -}}
{{- $_ := required "sendrec.env.s3PublicEndpoint is required (maps to S3_PUBLIC_ENDPOINT)" .Values.sendrec.env.s3PublicEndpoint -}}
{{- $_ := required "sendrec.env.s3Bucket is required (maps to S3_BUCKET)" .Values.sendrec.env.s3Bucket -}}
{{- if not .Values.sendrec.existingSecret -}}
{{- $_ := required "sendrec.secrets.databaseUrl is required unless sendrec.existingSecret is set" .Values.sendrec.secrets.databaseUrl -}}
{{- $_ := required "sendrec.secrets.jwtSecret is required unless sendrec.existingSecret is set" .Values.sendrec.secrets.jwtSecret -}}
{{- $_ := required "sendrec.secrets.s3AccessKey is required unless sendrec.existingSecret is set" .Values.sendrec.secrets.s3AccessKey -}}
{{- $_ := required "sendrec.secrets.s3SecretKey is required unless sendrec.existingSecret is set" .Values.sendrec.secrets.s3SecretKey -}}
{{- end -}}
{{- end -}}
