{{- define "omni.args" -}}
- --account-id="{{ .Values.accountUuid }}"
- --advertised-api-url={{ printf "https://%s/" .Values.domainName }} 
- --advertised-kubernetes-proxy-url={{ printf "https://%s/" .Values.service.k8sProxy.domainName }}
{{- if .Values.auth.auth0.enabled }}
- --auth-auth0-enabled=true
- --auth-auth0-client-id={{ .Values.auth.auth0.clientId | toString}}
- --auth-auth0-domain={{ .Values.auth.auth0.domain }}
{{- end }}
{{- if .Values.auth.saml.enabled }}
- --auth-saml-enabled=true
{{- if .Values.auth.saml.url }}
- --auth-saml-url="{{ .Values.auth.saml.url }}"
{{- end }}
{{- end }}
{{- if .Values.volumes.tls.secretName }}
- --cert=/etc/omni/tls/tls.crt
- --key=/etc/omni/tls/tls.key
{{- end }}
{{- if .Values.debug }}
- --debug=true
{{- end }}
{{- if .Values.etcd.embedded }}
- --etcd-embedded=true
{{- end }}
{{- if .Values.imageFactory.pxeAddress }}
- --image-factory-address="{{ .Values.imageFactory.address }}"
{{- end }}
{{- if .Values.imageFactory.pxeAddress }}
- --image-factory-pxe-address="{{ .Values.imageFactory.pxeAddress }}"
{{- end }}
{{- if and .Values.initialUsers (gt (len .Values.initialUsers) 0) }}
- --initial-users={{ join "," .Values.initialUsers }}
{{- end }}
{{- if .Values.name }}
- --name={{ .Values.name}}
{{- end }}
{{- if .Values.privateKeySource }}
- --private-key-source={{ .Values.privateKeySource }}
{{- end }}
- --siderolink-api-advertised-url={{ printf "https://%s" .Values.service.siderolink.domainName }}
- --siderolink-wireguard-advertised-addr={{ .Values.service.siderolink.wireguard.address }}:{{ .Values.service.siderolink.wireguard.port }}
{{- end }}
