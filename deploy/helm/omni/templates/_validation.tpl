{{/*
Compile all validation checks here
*/}}
{{- define "omni.validateValues" -}}

{{/* Legacy Chart Upgrade Check */}}
{{/* Detect upgrade attempts from the old Omni Helm chart (ghcr.io/siderolabs/charts/omni, version 0.0.x) */}}
  {{- if .Release.IsUpgrade -}}
    {{- $ns := include "omni.namespace" . -}}
    {{- $deployments := (lookup "apps/v1" "Deployment" $ns "").items | default list -}}
    {{- range $deployments -}}
      {{- $labels := .metadata.labels | default dict -}}
      {{- $chartLabel := get $labels "helm.sh/chart" -}}
      {{- if and $chartLabel (hasPrefix "omni-0." $chartLabel) -}}
        {{- fail "Upgrading from the legacy Omni Helm chart (ghcr.io/siderolabs/charts/omni) is not supported. The chart has been rewritten from scratch with a completely different values schema. Please uninstall the old release first and perform a fresh install with the new chart." -}}
      {{- end -}}
    {{- end -}}
  {{- end -}}

{{/* Etcd Encryption Key Check */}}
  {{- if not .Values.etcdEncryptionKey.existingSecret -}}
    {{- if empty .Values.etcdEncryptionKey.omniAsc -}}
      {{- fail "You must provide an etcd encryption key in etcdEncryptionKey.omniAsc or use an existing secret via etcdEncryptionKey.existingSecret" -}}
    {{- end -}}
  {{- end -}}

{{/* Version Check Logic */}}
  {{- if not .Values.skipVersionCheck -}}
    {{/* source the tag from the helper */}}
    {{- $tag := include "omni.imageTag" . -}}
    {{- $semverTag := $tag | trimPrefix "v" -}}

    {{- if not (semverCompare ">= 1.5.0-0" $semverTag) -}}
      {{- fail (printf "Omni version [%s] is not supported by this chart due to known config merging issues. You should use v1.5.0 or above. Set 'skipVersionCheck: true' to bypass this check at your own risk." $tag) -}}
    {{- end -}}
  {{- end -}}

  {{- if empty .Values.additionalConfigSources -}}
    {{- if empty .Values.config.account.id -}}
      {{- fail "You must provide a unique UUID value for config.account.id (or provide it via additionalConfigSources)!" -}}
    {{- end -}}

    {{- $wg := .Values.config.services.siderolink.wireGuard | default dict -}}
    {{- if empty $wg.advertisedEndpoint -}}
      {{- fail "WireGuard advertised endpoint must be provided in config.services.siderolink.wireGuard.advertisedEndpoint in the IP:PORT format (or via additionalConfigSources)" -}}
    {{- end -}}

    {{- if empty .Values.config.auth.initialUsers -}}
      {{- fail "At least one initial user must be provided in config.auth.initialUsers (or via additionalConfigSources)" -}}
    {{- end -}}

    {{/* Auth Provider Check - at least one must be enabled */}}
    {{- $auth0Enabled := and .Values.config.auth.auth0 .Values.config.auth.auth0.enabled -}}
    {{- $oidcEnabled := and .Values.config.auth.oidc .Values.config.auth.oidc.enabled -}}
    {{- $samlEnabled := and .Values.config.auth.saml .Values.config.auth.saml.enabled -}}
    {{- if not (or $auth0Enabled $oidcEnabled $samlEnabled) -}}
      {{- fail "At least one authentication provider must be enabled (config.auth.auth0.enabled, config.auth.oidc.enabled, or config.auth.saml.enabled) or configured via additionalConfigSources" -}}
    {{- end -}}

    {{/* Example.com Check - prevent users from deploying with placeholder values */}}
    {{- if contains "example.com" .Values.config.services.api.advertisedURL -}}
      {{- fail "config.services.api.advertisedURL contains 'example.com'. Please set your actual domain (or use additionalConfigSources)." -}}
    {{- end -}}
    {{- if contains "example.com" .Values.config.services.kubernetesProxy.advertisedURL -}}
      {{- fail "config.services.kubernetesProxy.advertisedURL contains 'example.com'. Please set your actual domain (or use additionalConfigSources)." -}}
    {{- end -}}
    {{- if contains "example.com" .Values.config.services.machineAPI.advertisedURL -}}
      {{- fail "config.services.machineAPI.advertisedURL contains 'example.com'. Please set your actual domain (or use additionalConfigSources)." -}}
    {{- end -}}

    {{- include "omni.validateIngress" (dict "Ingress" .Values.ingress.main "Url" .Values.config.services.api.advertisedURL "Name" "main" "TargetCfg" "config.services.api.advertisedURL") -}}

    {{- include "omni.validateIngress" (dict "Ingress" .Values.ingress.kubernetesProxy "Url" .Values.config.services.kubernetesProxy.advertisedURL "Name" "kubernetesProxy" "TargetCfg" "config.services.kubernetesProxy.advertisedURL") -}}

    {{- include "omni.validateIngress" (dict "Ingress" .Values.ingress.siderolinkApi "Url" .Values.config.services.machineAPI.advertisedURL "Name" "siderolinkApi" "TargetCfg" "config.services.machineAPI.advertisedURL") -}}
  {{- end -}}

{{- end -}}

{{/*
Helper Template: Validate a single Ingress against a Config URL.
Requires a dict with:
- .Ingress: The ingress object (e.g. .Values.ingress.main)
- .Url: The advertised URL string
- .Name: Human readable name for error messages
- .TargetCfg: The config path name for error messages
*/}}
{{- define "omni.validateIngress" -}}
  {{- if and .Ingress.enabled (not .Ingress.skipConfigCheck) -}}

    {{/* 1. Check Protocol Consistency (TLS vs http/https) */}}
    {{- $expectHttps := not (empty .Ingress.tls) -}}
    {{- $isHttps := hasPrefix "https://" .Url -}}
    {{- $isHttp := hasPrefix "http://" .Url -}}

    {{- if and $expectHttps (not $isHttps) -}}
      {{- fail (printf "Validation Failed: Ingress '%s' has TLS enabled, but %s [%s] does not start with 'https://'. Set 'ingress.%s.skipConfigCheck: true' to bypass." .Name .TargetCfg .Url .Name) -}}
    {{- end -}}

    {{- if and (not $expectHttps) (not $isHttp) -}}
       {{/* If TLS is disabled, we expect http://. If the user provided https://, that's a mismatch. */}}
       {{- fail (printf "Validation Failed: Ingress '%s' has TLS disabled, but %s [%s] starts with 'https://' (expected 'http://'). Set 'ingress.%s.skipConfigCheck: true' to bypass." .Name .TargetCfg .Url .Name) -}}
    {{- end -}}

    {{/* 2. Check Host Consistency */}}
    {{- $parsed := urlParse .Url -}}
    {{- $configHost := (split ":" $parsed.host)._0 -}}
    {{- if ne .Ingress.host $configHost -}}
      {{- fail (printf "Validation Failed: Ingress '%s' host [%s] does not match host in %s [%s]. Set 'ingress.%s.skipConfigCheck: true' to bypass." .Name .Ingress.host .TargetCfg .Url .Name) -}}
    {{- end -}}

  {{- end -}}
{{- end -}}
