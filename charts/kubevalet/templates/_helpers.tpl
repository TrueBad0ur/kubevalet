{{/*
Expand the name of the chart.
*/}}
{{- define "kubevalet.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a fully qualified app name.
*/}}
{{- define "kubevalet.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Chart label.
*/}}
{{- define "kubevalet.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels.
*/}}
{{- define "kubevalet.labels" -}}
helm.sh/chart: {{ include "kubevalet.chart" . }}
{{ include "kubevalet.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels.
*/}}
{{- define "kubevalet.selectorLabels" -}}
app.kubernetes.io/name: {{ include "kubevalet.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
ServiceAccount name.
*/}}
{{- define "kubevalet.serviceAccountName" -}}
{{- if .Values.serviceAccount.name }}
{{- .Values.serviceAccount.name }}
{{- else }}
{{- include "kubevalet.fullname" . }}
{{- end }}
{{- end }}

{{/*
PostgreSQL service hostname (internal).
*/}}
{{- define "kubevalet.postgresHost" -}}
{{- printf "%s-postgres" (include "kubevalet.fullname" .) }}
{{- end }}

{{/*
Build the POSTGRES_DSN value.
*/}}
{{- define "kubevalet.postgresDSN" -}}
{{- if .Values.postgres.enabled }}
{{- printf "postgres://%s:%s@%s:5432/%s?sslmode=disable"
    .Values.postgres.user
    .Values.postgres.password
    (include "kubevalet.postgresHost" .)
    .Values.postgres.database }}
{{- else }}
{{- required "postgres.external.dsn is required when postgres.enabled is false" .Values.postgres.external.dsn }}
{{- end }}
{{- end }}
