{{- define "omni.args" -}}
- --account-id="{{ .Values.accountUuid }}"
- --advertised-api-url="{{ printf "https://%s:%d/" .Values.domainName .Values.service.api.targetPort }}"
- --advertised-kubernetes-proxy-url="{{ printf "https://%s:%s/" .Values.domainName (toString .Values.service.k8sProxy.targetPort) }}"
{{- if .Values.auth.auth0.enabled }}
- --auth-auth0-enabled=true
- --auth-auth0-client-id="{{ .Values.auth.auth0.clientId }}"
- --auth-auth0-domain="{{ .Values.auth.auth0.domain }}"
{{- end }}
{{- if .Values.auth.saml.enabled }}
- --auth-saml-enabled=true
{{- if .Values.auth.saml.url }}
- --auth-saml-url="{{ .Values.auth.saml.url }}"
{{- end }}
{{- end }}
- --bind-addr="0.0.0.0:{{ .Values.service.api.targetPort }}"
{{- if .Values.volumes.tls.secretName }}
- --cert="/etc/omni/tls/tls.crt"
- --key="/etc/omni/tls/tls.key"
{{- end }}
{{- if .Values.debug }}
- --debug=true
{{- end }}
{{- if .Values.etcd.embedded }}
- --etcd-embedded=true
{{- end }}
- --event-sink-port={{ .Values.service.eventSink.port }}
- --image-factory-address="{{ .Values.imageFactory.address }}"
{{- if .Values.imageFactory.pxeAddress }}
- --image-factory-pxe-address="{{ .Values.imageFactory.pxeAddress }}"
{{- end }}
{{- if and .Values.initialUsers (gt (len .Values.initialUsers) 0) }}
- --initial-users={{ join "," .Values.initialUsers }}
{{- end }}
- --k8s-proxy-bind-addr="{{ printf "0.0.0.0:%d" .Values.service.k8sProxy.port }}"
- --machine-api-bind-addr="{{ .Values.service.siderolink.api.bindAddress }}:{{ .Values.service.siderolink.api.port }}"
{{- if .Values.privateKeySource }}
- --private-key-source="{{ .Values.privateKeySource }}"
{{- end }}
- --siderolink-api-advertised-url="{{ printf "grpc://%s:%d/" .Values.domainName .Values.service.siderolink.api.targetPort }}"
- --siderolink-wireguard-advertised-addr="{{ .Values.service.siderolink.wireguard.address }}:{{ .Values.service.siderolink.wireguard.targetPort }}"
{{- end }}
