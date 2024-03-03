{{- /* Define a named template "omni.params" that outputs the command-line parameters for the Omni application based on values set in values.yaml */ -}}
{{- define "omni.params" -}}
{{- $omni := .Values.omni -}}
- --account-id="${OMNI_ACCOUNT_UUID}"
- --advertised-api-url="{{ printf "https://%s:%s/" $omni.hostName (toString $omni.service.api.targetPort) }}"
- --advertised-kubernetes-proxy-url="{{ printf "https://%s:%s/" $omni.hostName (toString $omni.service.k8sProxy.targetPort) }}"
{{- if $omni.auth.auth0.enabled }}
- --auth-auth0-enabled=true
- --auth-auth0-client-id="{{ $omni.auth.auth0.clientId }}"
- --auth-auth0-domain="{{ $omni.auth.auth0.domain }}"
{{- end }}
{{- if $omni.auth.saml.enabled }}
- --auth-saml-enabled=true
{{- if $omni.auth.saml.url }}
- --auth-saml-url="{{ $omni.auth.saml.url }}"
{{- else if $omni.auth.saml.metadata }}
- --auth-saml-metadata={{ $omni.auth.saml.metadata }}
{{- end }}
{{- if $omni.auth.saml.labelRules }}
- --auth-saml-label-rules='{{ toJson $omni.auth.saml.labelRules }}'
{{- end }}
{{- end }}
{{- if $omni.auth.webauthn.enabled }}
- --auth-webauthn-enabled=true
{{- if $omni.auth.webauthn.required }}
- --auth-webauthn-required=true
{{- end }}
{{- end }}
- --bind-addr="0.0.0.0:{{ .Values.omni.service.api.targetPort }}"
{{- if $omni.tls.secretName }}
- --cert="/etc/omni/tls/tls.crt"
- --key="/etc/omni/tls/tls.key"
{{- end }}
{{- if $omni.debug }}
- --debug=true
{{- end }}
{{- if $omni.disableControllerRuntimeCache }}
- --disable-controller-runtime-cache=true
{{- end }}
{{- if $omni.enableTalosPreReleaseVersions }}
- --enable-talos-pre-release-versions=true
{{- end }}
{{- if and $omni.etcd.backup.enabled (not $omni.etcd.embedded) }}
{{- if $omni.etcd.backup.localPath }}
- --etcd-backup-local-path="{{ $omni.etcd.backup.localPath }}"
{{- end }}
{{- if $omni.etcd.backup.maxInterval }}
- --etcd-backup-max-interval={{ $omni.etcd.backup.maxInterval }}
{{- end }}
{{- if $omni.etcd.backup.minInterval }}
- --etcd-backup-min-interval={{ $omni.etcd.backup.minInterval }}
{{- end }}
{{- if $omni.etcd.backup.s3 }}
- --etcd-backup-s3=true
{{- end }}
{{- if index $omni "etcd" "backup" "tickInterval" }}
- --etcd-backup-tick-interval="{{ $omni.etcd.backup.tickInterval }}"
{{- end }}
{{- end }}
{{- if $omni.etcd.embedded }}
- --etcd-embedded=true
{{- else if index $omni "etcd" "tls" "existingSecret" }}
- --etcd-ca-path="/etc/etcd/tls/ca.crt"
- --etcd-client-cert-path="/etc/etcd/tls/tls.crt"
- --etcd-client-key-path="/etc/etcd/tls/tls.key"
{{- end }}
{{- if and $omni.etcd.endpoints (gt (len $omni.etcd.endpoints) 0) }}
- --etcd-endpoints={{ join "," $omni.etcd.endpoints }}
{{- end }}
- --event-sink-port={{ $omni.service.eventSink.targetPort }}
{{- if $omni.frontend.bind }}
- --frontend-bind="{{ $omni.frontend.bind }}"
{{- end }}
{{- if $omni.frontend.dst }}
- --frontend-dst="{{ $omni.frontend.dst }}"
{{- end }}
- --image-factory-address="{{ $omni.imageFactory.address }}"
{{- if $omni.imageFactory.pxeAddress }}
- --image-factory-pxe-address="{{ $omni.imageFactory.pxeAddress }}"
{{- end }}
{{- if and $omni.initialUsers (gt (len $omni.initialUsers) 0) }}
- --initial-users={{ join "," $omni.initialUsers }}
{{- end }}
- --k8s-proxy-bind-addr="{{ printf "0.0.0.0:%d" (int $omni.service.k8sProxy.targetPort) }}"
- --kubernetes-registry="{{ $omni.kubernetesRegistry }}"
{{- if $omni.lbMaxPort }}
- --lb-max-port={{ $omni.lbMaxPort }}
{{- end }}
{{- if $omni.lbMinPort }}
- --lb-min-port={{ $omni.lbMinPort }}
{{- end }}
- --local-resource-server-port={{ $omni.localResourceServerPort }}
- --log-resource-updates-log-level="{{ $omni.logResourceUpdatesLogLevel }}"
{{- if and $omni.logResourceUpdatesTypes (gt (len $omni.logResourceUpdatesTypes) 0) }}
- --log-resource-updates-types="{{ join "," $omni.logResourceUpdatesTypes }}""
{{- end }}
- --log-server-port={{ $omni.service.logServer.targetPort }}
- --log-storage-enabled={{ if $omni.logStorage.enabled }}true{{ else }}false{{ end }}
- --log-storage-flush-period={{ $omni.logStorage.flushPeriod }}
- --log-storage-path="{{ $omni.logStorage.path }}"
- --machine-api-bind-addr="{{ $omni.advertisedIP }}:{{ $omni.service.api.targetPort }}"
{{- if index $omni "machineApi" "tls" "existingSecret" }}
- --machine-api-cert="/etc/machine-api/tls/tls.crt"
- --machine-api-key="/etc/machine-api/tls/tls.key"
{{- end }}
- --metrics-bind-addr="{{ $omni.metricsBindAddr }}"
- --name="{{ $omni.name }}"
{{- if $omni.pprofBindAddr }}
- --pprof-bind-addr="{{ $omni.pprofBindAddr }}"
{{- end }}
{{- if $omni.privateKeySource }}
- --private-key-source="{{ $omni.privateKeySource }}"
{{- end }}
{{- if and $omni.publicKeyFiles (gt (len $omni.publicKeyFiles) 0) }}
- --public-key-files="{{ join "," $omni.publicKeyFiles }}"
{{- end }}
- --public-key-pruning-interval="{{ $omni.publicKeyPruningInterval }}"
{{- if and $omni.registryMirror (gt (len $omni.registryMirror) 0) }}
- --registry-mirror="{{ join "," $omni.registryMirror }}"
{{- end }}
- --secondary-storage-path="{{ $omni.secondaryStoragePath }}"
- --siderolink-api-advertised-url="{{ printf "grpc://%s:%s/" $omni.hostName (toString $omni.service.siderolink.api.targetPort) }}"
{{- if $omni.siderolink.disableLastEndpoint }}
- --siderolink-disable-last-endpoint=true
{{- end }}
- --siderolink-wireguard-advertised-addr="{{ $omni.advertisedIP }}:{{ $omni.service.siderolink.wireguard.targetPort }}"
- --siderolink-wireguard-bind-addr="0.0.0.0:{{ $omni.service.siderolink.wireguard.targetPort }}"
- --storage-kind="{{ $omni.storageKind }}"
{{- if $omni.suspended }}
- --suspended
{{- end }}
- --talos-installer-registry="{{ $omni.talosInstallerRegistry }}"
- --workload-proxying-enabled={{ if $omni.workloadProxyingEnabled }}true{{ else }}false{{ end }}
{{- end }}
