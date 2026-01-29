{{/*
Expand the name of the chart.
*/}}
{{- define "omni.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "omni.fullname" -}}
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
Create chart name and version as used by the chart label.
*/}}
{{- define "omni.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Expand the namespace of the release.
Allows overriding it for multi-namespace deployments in combined charts.
*/}}
{{- define "omni.namespace" -}}
{{- default .Release.Namespace .Values.namespaceOverride | trunc 63 | trimSuffix "-" -}}
{{- end }}

{{/*
Common labels
*/}}
{{- define "omni.labels" -}}
helm.sh/chart: {{ include "omni.chart" . }}
{{ include "omni.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "omni.selectorLabels" -}}
app.kubernetes.io/name: {{ include "omni.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "omni.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "omni.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Return the name of the etcd encryption secret to use.
If 'existingSecret' is set, use it.
Otherwise, use the chart's fullname suffix.
*/}}
{{- define "omni.etcdSecretName" -}}
{{- if .Values.etcdEncryptionKey.existingSecret -}}
    {{- .Values.etcdEncryptionKey.existingSecret -}}
{{- else -}}
    {{- include "omni.fullname" . -}}-etcd-key
{{- end -}}
{{- end -}}

{{/*
Return the image tag to use.
*/}}
{{- define "omni.imageTag" -}}
{{- .Values.image.tag | default .Chart.AppVersion -}}
{{- end -}}
