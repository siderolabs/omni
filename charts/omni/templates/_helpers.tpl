{{/*
Expand the name of the chart.
*/}}
{{- define "omni.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Generate basic labels
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
Chart label
*/}}
{{- define "omni.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a fully qualified image name with optional digest
*/}}
{{- define "omni.image" -}}
{{- $repository := .Values.omni.image.repository -}}
{{- $tag := .Values.omni.image.tag -}}
{{- $digest := .Values.omni.image.digest | default "" -}}
{{- if ne $digest "" -}}
{{- printf "%s@%s" $repository $digest -}}
{{- else -}}
{{- printf "%s:%s" $repository $tag -}}
{{- end -}}
{{- end }}
