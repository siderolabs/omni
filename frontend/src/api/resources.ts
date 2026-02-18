// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// THIS FILE WAS AUTOMATICALLY GENERATED, PLEASE DO NOT EDIT.

export const RoleNone = "None";
export const RoleInfraProvider = "InfraProvider";
export const RoleReader = "Reader";
export const RoleOperator = "Operator";
export const RoleAdmin = "Admin";
export const RedirectQueryParam = "redirect";
export const AuthFlowQueryParam = "flow";
export const CLIAuthFlow = "cli";
export const FrontendAuthFlow = "frontend";
export const WorkloadProxyAuthFlow = "workload-proxy";
export const SignatureHeaderKey = "x-sidero-signature";
export const TimestampHeaderKey = "x-sidero-timestamp";
export const PayloadHeaderKey = "x-sidero-payload";
export const authHeader = "authorization";
export const authBearerHeaderPrefix = "Bearer ";
export const SignatureVersionV1 = "siderov1";
export const samlSessionHeader = "saml-session";
export const SignedRedirect = "v1:";
export const workloadProxyPublicKeyIdCookie = "publicKeyId";
export const workloadProxyPublicKeyIdSignatureBase64Cookie = "publicKeyIdSignatureBase64";
export const DefaultKubernetesVersion = "1.35.1";
export const installDiskMinSize = 5e+09;
export const MaxJoinTokenNameLength = 16;
export const authPublicKeyIDQueryParam = "public-key-id";
export const SecureBoot = "secureboot";
export const DefaultTalosVersion = "1.12.4";
export const MinTalosVersion = "1.8.0";
export const PatchWeightInstallDisk = 0;
export const PatchBaseWeightCluster = 200;
export const PatchBaseWeightMachineSet = 400;
export const PatchBaseWeightClusterMachine = 400;
export const ExposedServiceLabelAnnotationKey = "omni-kube-service-exposer.sidero.dev/label";
export const ExposedServicePortAnnotationKey = "omni-kube-service-exposer.sidero.dev/port";
export const ExposedServiceIconAnnotationKey = "omni-kube-service-exposer.sidero.dev/icon";
export const ExposedServicePrefixAnnotationKey = "omni-kube-service-exposer.sidero.dev/prefix";
export const PlatformMetalID = "metal";
export const TalosServiceType = "Services.v1alpha1.talos.dev";
export const TalosCPUType = "CPUStats.perf.talos.dev";
export const TalosDiscoveredVolumeType = "DiscoveredVolumes.block.talos.dev";
export const TalosDiskType = "Disks.block.talos.dev";
export const TalosMemoryType = "MemoryStats.perf.talos.dev";
export const TalosNodenameType = "Nodenames.kubernetes.talos.dev";
export const TalosMemberType = "Members.cluster.talos.dev";
export const TalosPCIDeviceType = "PCIDevices.hardware.talos.dev";
export const TalosNodeAddressType = "NodeAddresses.net.talos.dev";
export const TalosMountStatusType = "MountStatuses.runtime.talos.dev";
export const TalosMachineStatusType = "MachineStatuses.runtime.talos.dev";
export const TalosNodenameID = "nodename";
export const TalosAddressRoutedNoK8s = "routed-no-k8s";
export const TalosCPUID = "latest";
export const TalosMemoryID = "latest";
export const TalosMachineStatusID = "machine";
export const TalosPerfNamespace = "perf";
export const TalosClusterNamespace = "cluster";
export const TalosHardwareNamespace = "hardware";
export const TalosRuntimeNamespace = "runtime";
export const TalosK8sNamespace = "k8s";
export const TalosNetworkNamespace = "network";
export const ServiceAccountDomain = "serviceaccount.omni.sidero.dev";
export const InfraProviderServiceAccountDomain = "infra-provider.serviceaccount.omni.sidero.dev";
export const DefaultNamespace = "default";
export const EphemeralNamespace = "ephemeral";
export const MetricsNamespace = "metrics";
export const VirtualNamespace = "virtual";
export const ExternalNamespace = "external";
export const InfraProviderNamespace = "infra-provider";
export const InfraProviderEphemeralNamespace = "infra-provider-ephemeral";
export const MachineLocked = "omni.sidero.dev/locked";
export const UpdateLocked = "omni.sidero.dev/locked-update";
export const ResourceManagedByClusterTemplates = "omni.sidero.dev/managed-by-cluster-templates";
export const ConfigPatchName = "name";
export const ConfigPatchDescription = "description";
export const ClusterLocked = "omni.sidero.dev/cluster-locked";
export const ClusterImportIsInProgress = "omni.sidero.dev/cluster-import-is-in-progress";
export const KernelArgsInitialized = "omni.sidero.dev/kernel-args-initialized";
export const EtcdBackupS3ConfID = "etcd-backup-s3-conf";
export const EtcdBackupS3ConfType = "EtcdBackupS3Configs.omni.sidero.dev";
export const BackupDataType = "BackupDatas.omni.sidero.dev";
export const ClusterType = "Clusters.omni.sidero.dev";
export const ClusterBootstrapStatusType = "ClusterBootstrapStatuses.omni.sidero.dev";
export const ClusterConfigVersionType = "ClusterConfigVersions.omni.sidero.dev";
export const ClusterDestroyStatusType = "ClusterDestroyStatuses.omni.sidero.dev";
export const ClusterDiagnosticsType = "ClusterDiagnostics.omni.sidero.dev";
export const ClusterEndpointType = "ClusterEndpoints.omni.sidero.dev";
export const ClusterMachineType = "ClusterMachines.omni.sidero.dev";
export const ClusterMachineConfigType = "ClusterMachineConfigs.omni.sidero.dev";
export const ClusterMachineConfigPatchesType = "ClusterMachineConfigPatches.omni.sidero.dev";
export const ClusterMachineConfigStatusType = "ClusterMachineConfigStatuses.omni.sidero.dev";
export const ClusterMachineEncryptionKeyType = "ClusterMachineEncryptionKeys.omni.sidero.dev";
export const ClusterMachineIdentityType = "ClusterMachineIdentities.omni.sidero.dev";
export const ClusterMachineRequestStatusType = "ClusterMachineRequestStatuses.omni.sidero.dev";
export const ClusterMachineStatusType = "ClusterMachineStatuses.omni.sidero.dev";
export const ClusterMachineTalosVersionType = "ClusterMachineTalosVersions.omni.sidero.dev";
export const ClusterMetricsType = "ClusterMetrics.omni.sidero.dev";
export const ClusterMetricsID = "metrics";
export const ClusterStatusType = "ClusterStatuses.omni.sidero.dev";
export const ClusterStatusMetricsType = "ClusterStatusMetrics.omni.sidero.dev";
export const ClusterStatusMetricsID = "metrics";
export const ClusterTaintType = "ClusterTaints.omni.sidero.dev";
export const ClusterUUIDType = "ClusterUUIDs.omni.sidero.dev";
export const ClusterWorkloadProxyStatusType = "ClusterWorkloadProxyStatuses.omni.sidero.dev";
export const ConfigPatchType = "ConfigPatches.omni.sidero.dev";
export const ControlPlaneStatusType = "ControlPlaneStatuses.omni.sidero.dev";
export const DiscoveryAffiliateDeleteTaskType = "DiscoveryAffiliateDeleteTasks.omni.sidero.dev";
export const EtcdBackupType = "EtcdBackups.omni.sidero.dev";
export const EtcdBackupStoreStatusID = "etcdbackup-store-status";
export const EtcdBackupStoreStatusType = "EtcdBackupStoreStatuses.omni.sidero.dev";
export const EtcdBackupEncryptionType = "EtcdBackupEncryptions.omni.sidero.dev";
export const EtcdBackupOverallStatusID = "etcdbackup-overall-status";
export const EtcdBackupOverallStatusType = "EtcdBackupOverallStatuses.omni.sidero.dev";
export const EtcdBackupStatusType = "EtcdBackupStatuses.omni.sidero.dev";
export const EtcdManualBackupType = "EtcdManualBackups.omni.sidero.dev";
export const ExposedServiceType = "ExposedServices.omni.sidero.dev";
export const ExtensionsConfigurationType = "ExtensionsConfigurations.omni.sidero.dev";
export const FeaturesConfigID = "features-config";
export const FeaturesConfigType = "FeaturesConfigs.omni.sidero.dev";
export const ImagePullRequestType = "ImagePullRequests.omni.sidero.dev";
export const ImagePullStatusType = "ImagePullStatuses.omni.sidero.dev";
export const ImportedClusterSecretsType = "ImportedClusterSecrets.omni.sidero.dev";
export const InfraMachineBMCConfigType = "InfraMachineBMCConfigs.omni.sidero.dev";
export const InfraMachineConfigType = "InfraMachineConfigs.omni.sidero.dev";
export const InfraProviderCombinedStatusType = "InfraProviderCombinedStatuses.omni.sidero.dev";
export const InstallationMediaType = "InstallationMedias.omni.sidero.dev";
export const InstallationMediaConfigType = "InstallationMediaConfigs.omni.sidero.dev";
export const KernelArgsType = "KernelArgs.omni.sidero.dev";
export const KernelArgsStatusType = "KernelArgsStatuses.omni.sidero.dev";
export const KubernetesStatusType = "KubernetesStatuses.omni.sidero.dev";
export const KubernetesUpgradeManifestStatusType = "KubernetesUpgradeManifestStatuses.omni.sidero.dev";
export const KubernetesUpgradeStatusType = "KubernetesUpgradeStatuses.omni.sidero.dev";
export const KubernetesVersionType = "KubernetesVersions.omni.sidero.dev";
export const SystemLabelPrefix = "omni.sidero.dev/";
export const LabelControlPlaneRole = "omni.sidero.dev/role-controlplane";
export const LabelWorkerRole = "omni.sidero.dev/role-worker";
export const LabelCluster = "omni.sidero.dev/cluster";
export const LabelClusterUUID = "omni.sidero.dev/cluster-uuid";
export const LabelHostname = "omni.sidero.dev/hostname";
export const LabelMachineSet = "omni.sidero.dev/machine-set";
export const LabelClusterMachine = "omni.sidero.dev/cluster-machine";
export const LabelMachine = "omni.sidero.dev/machine";
export const LabelSystemPatch = "omni.sidero.dev/system-patch";
export const LabelExposedServiceAlias = "omni.sidero.dev/exposed-service-alias";
export const LabelHasGeneratedExposedServiceAlias = "omni.sidero.dev/has-generated-exposed-service-alias";
export const LabelInfraProviderID = "omni.sidero.dev/infra-provider-id";
export const LabelIsStaticInfraProvider = "omni.sidero.dev/is-static-infra-provider";
export const LabelMachineRequest = "omni.sidero.dev/machine-request";
export const LabelMachineRequestSet = "omni.sidero.dev/machine-request-set";
export const LabelNoManualAllocation = "omni.sidero.dev/no-manual-allocation";
export const LabelManagedByMachineSetNodeController = "omni.sidero.dev/managed-by-machine-set-node-controller";
export const LabelIsManagedByStaticInfraProvider = "omni.sidero.dev/is-managed-by-static-infra-provider";
export const LabelMachinePendingAccept = "omni.sidero.dev/accept-pending";
export const InfraProviderLabelPrefix = "omni.sidero.dev/infra-provider";
export const LabelClusterTaintedByImporting = "omni.sidero.dev/tainted-by-importing";
export const LabelClusterTaintedByExporting = "omni.sidero.dev/tainted-by-exporting";
export const LabelClusterTaintedByBreakGlass = "omni.sidero.dev/tainted-by-break-glass";
export const MachineStatusLabelConnected = "omni.sidero.dev/connected";
export const MachineStatusLabelReadyToUse = "omni.sidero.dev/ready-to-use";
export const MachineStatusLabelDisconnected = "omni.sidero.dev/disconnected";
export const MachineStatusLabelInvalidState = "omni.sidero.dev/invalid-state";
export const MachineStatusLabelReportingEvents = "omni.sidero.dev/reporting-events";
export const MachineStatusLabelAvailable = "omni.sidero.dev/available";
export const MachineStatusLabelArch = "omni.sidero.dev/arch";
export const MachineStatusLabelCPU = "omni.sidero.dev/cpu";
export const MachineStatusLabelCores = "omni.sidero.dev/cores";
export const MachineStatusLabelMem = "omni.sidero.dev/mem";
export const MachineStatusLabelStorage = "omni.sidero.dev/storage";
export const MachineStatusLabelNet = "omni.sidero.dev/net";
export const MachineStatusLabelPlatform = "omni.sidero.dev/platform";
export const MachineStatusLabelRegion = "omni.sidero.dev/region";
export const MachineStatusLabelZone = "omni.sidero.dev/zone";
export const MachineStatusLabelInstance = "omni.sidero.dev/instance";
export const MachineStatusLabelTalosVersion = "omni.sidero.dev/talos-version";
export const MachineStatusLabelInstalled = "omni.sidero.dev/installed";
export const ClusterMachineStatusLabelNodeName = "omni.sidero.dev/node-name";
export const ExtensionsConfigurationLabel = "omni.sidero.dev/root-configuration";
export const LabelMachineRequestInUse = "omni.sidero.dev/machine-request-in-use";
export const MachineType = "Machines.omni.sidero.dev";
export const MachineClassType = "MachineClasses.omni.sidero.dev";
export const MachineConfigDiffType = "MachineConfigDiffs.omni.sidero.dev";
export const MachineConfigGenOptionsType = "MachineConfigGenOptions.omni.sidero.dev";
export const MachineExtensionsType = "MachineExtensions.omni.sidero.dev";
export const MachineExtensionsStatusType = "MachineExtensionsStatuses.omni.sidero.dev";
export const MachineLabelsType = "MachineLabels.omni.sidero.dev";
export const MachinePendingUpdatesType = "MachinePendingUpdates.omni.sidero.dev";
export const MachineRequestSetType = "MachineRequestSets.omni.sidero.dev";
export const MachineRequestSetStatusType = "MachineRequestSetStatuses.omni.sidero.dev";
export const ClusterMachineSecretsType = "ClusterMachineSecrets.omni.sidero.dev";
export const ControlPlanesIDSuffix = "control-planes";
export const DefaultWorkersIDSuffix = "workers";
export const MachineSetType = "MachineSets.omni.sidero.dev";
export const MachineSetConfigStatusType = "MachineSetConfigStatuses.omni.sidero.dev";
export const MachineSetDestroyStatusType = "MachineSetDestroyStatuses.omni.sidero.dev";
export const MachineSetNodeType = "MachineSetNodes.omni.sidero.dev";
export const MachineSetStatusType = "MachineSetStatuses.omni.sidero.dev";
export const MachineStatusType = "MachineStatuses.omni.sidero.dev";
export const MachineStatusLinkType = "MachineStatusLinks.omni.sidero.dev";
export const MachineStatusMetricsType = "MachineStatusMetrics.omni.sidero.dev";
export const MachineStatusMetricsID = "metrics";
export const MachineStatusSnapshotType = "MachineStatusSnapshots.omni.sidero.dev";
export const MachineUpgradeStatusType = "MachineUpgradeStatuses.omni.sidero.dev";
export const MaintenanceConfigStatusType = "MaintenanceConfigStatuses.omni.sidero.dev";
export const NodeForceDestroyRequestType = "NodeForceDestroyRequests.omni.sidero.dev";
export const OngoingTaskType = "OngoingTasks.omni.sidero.dev";
export const RedactedClusterMachineConfigType = "RedactedClusterMachineConfigs.omni.sidero.dev";
export const RotateKubernetesCAType = "RotateKubernetesCAs.omni.sidero.dev";
export const RotateTalosCAType = "RotateTalosCAs.omni.sidero.dev";
export const SchematicType = "Schematics.omni.sidero.dev";
export const SchematicConfigurationType = "SchematicConfigurations.omni.sidero.dev";
export const SecretRotationType = "SecretRotations.omni.sidero.dev";
export const ClusterSecretsType = "ClusterSecrets.omni.sidero.dev";
export const ClusterSecretsRotationStatusType = "ClusterSecretsRotationStatuses.omni.sidero.dev";
export const TalosExtensionsType = "TalosExtensions.omni.sidero.dev";
export const TalosUpgradeStatusType = "TalosUpgradeStatuses.omni.sidero.dev";
export const TalosVersionType = "TalosVersions.omni.sidero.dev";
export const AccessPolicyType = "AccessPolicies.omni.sidero.dev";
export const AuthConfigID = "auth-config";
export const AuthConfigType = "AuthConfigs.omni.sidero.dev";
export const IdentityType = "Identities.omni.sidero.dev";
export const SAMLLabelPrefix = "saml.omni.sidero.dev/";
export const LabelPublicKeyUserID = "user-id";
export const LabelIdentityUserID = "user-id";
export const LabelIdentityTypeServiceAccount = "type-service-account";
export const PublicKeyType = "PublicKeys.omni.sidero.dev";
export const SAMLLabelRuleType = "SAMLLabelRules.omni.sidero.dev";
export const ServiceAccountStatusType = "ServiceAccountStatuses.omni.sidero.dev";
export const UserType = "Users.omni.sidero.dev";
export const BMCConfigType = "BMCConfigs.omni.sidero.dev";
export const ConfigPatchRequestType = "ConfigPatchRequests.omni.sidero.dev";
export const InfraMachineType = "InfraMachines.omni.sidero.dev";
export const MachineRegistrationType = "InfraMachineRegistrations.omni.sidero.dev";
export const MachineRequestType = "MachineRequests.omni.sidero.dev";
export const MachineRequestStatusType = "MachineRequestStatuses.omni.sidero.dev";
export const InfraMachineStatusType = "InfraMachineStatuses.omni.sidero.dev";
export const ProviderType = "InfraProviders.omni.sidero.dev";
export const InfraProviderHealthStatusType = "InfraProviderHealthStatuses.omni.sidero.dev";
export const InfraProviderStatusType = "InfraProviderStatuses.omni.sidero.dev";
export const KubernetesResourceType = "KubernetesResources.omni.sidero.dev";
export const ConfigType = "Configs.omni.sidero.dev";
export const ConfigID = "siderolink-config";
export const ConnectionParamsType = "ConnectionParams.omni.sidero.dev";
export const DefaultJoinTokenID = "default";
export const DefaultJoinTokenType = "DefaultJoinTokens.omni.sidero.dev";
export const GRPCTunnelConfigType = "GRPCTunnelConfigs.omni.sidero.dev";
export const JoinTokenType = "JoinTokens.omni.sidero.dev";
export const JoinTokenStatusType = "JoinTokenStatuses.omni.sidero.dev";
export const JoinTokenUsageType = "JoinTokenUsages.omni.sidero.dev";
export const SiderolinkResourceType = "Links.omni.sidero.dev";
export const LinkStatusType = "LinkStatuses.omni.sidero.dev";
export const MachineJoinConfigType = "MachineJoinConfigs.omni.sidero.dev";
export const PendingMachineType = "PendingMachines.omni.sidero.dev";
export const PendingMachineStatusType = "PendingMachineStatuses.omni.sidero.dev";
export const ProviderJoinConfigType = "ProviderJoinConfigs.omni.sidero.dev";
export const SiderolinkCounterNamespace = "metrics";
export const APIConfigType = "SiderolinkAPIConfigs.omni.sidero.dev";
export const SysVersionType = "SysVersions.system.sidero.dev";
export const SysVersionID = "current";
export const AdvertisedEndpointsID = "current";
export const AdvertisedEndpointsType = "AdvertisedEndpoints.omni.sidero.dev";
export const CloudPlatformConfigType = "CloudPlatformConfigs.omni.sidero.dev";
export const ClusterPermissionsType = "ClusterPermissions.omni.sidero.dev";
export const CurrentUserID = "current";
export const CurrentUserType = "CurrentUsers.omni.sidero.dev";
export const KubernetesUsageType = "KubernetesUsages.omni.sidero.dev";
export const LabelsCompletionType = "LabelsCompletions.omni.sidero.dev";
export const MetalPlatformConfigType = "MetalPlatformConfigs.omni.sidero.dev";
export const PermissionsID = "permissions";
export const PermissionsType = "Permissions.omni.sidero.dev";
export const SBCConfigType = "SBCConfigs.omni.sidero.dev";
export const MetalNetworkPlatformConfig = 10;
export const LabelsMeta = 12;
export const NamespaceType = "Namespaces.meta.cosi.dev";
export const ResourceDefinitionType = "ResourceDefinitions.meta.cosi.dev";
export const MetaNamespace = "meta";

export const kubernetes = {
  service: "services.v1",
  pod: "pods.v1",
  node: "nodes.v1",
  cluster: `clusters`,
  machine: `machines`,
  sideroServers: "servers",
  crd: "customresourcedefinitions.v1.apiextensions.k8s.io",
};

export const talos = {
  // resources
  service: "Services.v1alpha1.talos.dev",
  cpu: "CPUStats.perf.talos.dev",
  mem: "MemoryStats.perf.talos.dev",
  nodename: "Nodenames.kubernetes.talos.dev",
  member: "Members.cluster.talos.dev",
  nodeaddress: "NodeAddresses.net.talos.dev",

  // well known resource IDs
  defaultNodeNameID: "nodename",

  // namespaces
  perfNamespace: "perf",
  clusterNamespace: "cluster",
  runtimeNamespace: "runtime",
  k8sNamespace: "k8s",
  networkNamespace: "network",
};
