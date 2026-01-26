/* eslint-disable */
// @ts-nocheck
/*
* This file is a generated Typescript file for GRPC Gateway, DO NOT MODIFY
*/

import * as GoogleProtobufDuration from "../../google/protobuf/duration.pb"
import * as GoogleProtobufTimestamp from "../../google/protobuf/timestamp.pb"
import * as MachineMachine from "../../talos/machine/machine.pb"
import * as ManagementManagement from "../management/management.pb"
import * as SpecsVirtual from "./virtual.pb"

type Absent<T, K extends keyof T> = { [k in Exclude<keyof T, K>]?: undefined };
type OneOf<T> =
  | { [k in keyof T]?: undefined }
  | (
    keyof T extends infer K ?
      (K extends string & keyof T ? { [k in K]: T[K] } & Absent<T, K>
        : never)
    : never);

export enum ConfigApplyStatus {
  UNKNOWN = 0,
  PENDING = 1,
  APPLIED = 2,
  FAILED = 3,
}

export enum MachineSetPhase {
  Unknown = 0,
  ScalingUp = 1,
  ScalingDown = 2,
  Running = 3,
  Destroying = 4,
  Failed = 5,
  Reconfiguring = 6,
  Upgrading = 7,
}

export enum ConditionType {
  UnknownCondition = 0,
  Etcd = 1,
  WireguardConnection = 2,
}

export enum GrpcTunnelMode {
  UNSET = 0,
  ENABLED = 1,
  DISABLED = 2,
}

export enum MachineStatusSpecRole {
  NONE = 0,
  CONTROL_PLANE = 1,
  WORKER = 2,
}

export enum MachineStatusSpecPowerState {
  POWER_STATE_UNKNOWN = 0,
  POWER_STATE_UNSUPPORTED = 1,
  POWER_STATE_ON = 2,
  POWER_STATE_OFF = 3,
}

export enum EtcdBackupStatusSpecStatus {
  Unknown = 0,
  Ok = 1,
  Error = 2,
  Running = 3,
}

export enum ClusterMachineStatusSpecStage {
  UNKNOWN = 0,
  BOOTING = 1,
  INSTALLING = 2,
  UPGRADING = 6,
  CONFIGURING = 3,
  RUNNING = 4,
  REBOOTING = 7,
  SHUTTING_DOWN = 8,
  BEFORE_DESTROY = 9,
  DESTROYING = 5,
  POWERING_ON = 10,
  POWERED_OFF = 11,
}

export enum ClusterStatusSpecPhase {
  UNKNOWN = 0,
  SCALING_UP = 1,
  SCALING_DOWN = 2,
  RUNNING = 3,
  DESTROYING = 4,
}

export enum MachineSetSpecUpdateStrategy {
  Unset = 0,
  Rolling = 1,
}

export enum MachineSetSpecMachineClassType {
  Static = 0,
  Unlimited = 1,
}

export enum MachineSetSpecMachineAllocationType {
  Static = 0,
  Unlimited = 1,
}

export enum TalosUpgradeStatusSpecPhase {
  Unknown = 0,
  Upgrading = 1,
  Done = 2,
  Failed = 3,
  Reverting = 4,
  UpdatingMachineSchematics = 5,
}

export enum MachineStatusSnapshotSpecPowerStage {
  POWER_STAGE_NONE = 0,
  POWER_STAGE_POWERED_OFF = 1,
  POWER_STAGE_POWERING_ON = 2,
}

export enum ControlPlaneStatusSpecConditionStatus {
  Unknown = 0,
  Ready = 1,
  NotReady = 2,
}

export enum ControlPlaneStatusSpecConditionSeverity {
  Info = 0,
  Warning = 1,
  Error = 2,
}

export enum KubernetesUpgradeStatusSpecPhase {
  Unknown = 0,
  Upgrading = 1,
  Done = 2,
  Failed = 3,
  Reverting = 4,
}

export enum MachineUpgradeStatusSpecPhase {
  Unknown = 0,
  Pending = 1,
  Upgrading = 2,
  UpToDate = 3,
}

export enum MachineExtensionsStatusSpecItemPhase {
  Installed = 0,
  Installing = 1,
  Removing = 2,
}

export enum ClusterMachineRequestStatusSpecStage {
  UNKNOWN = 0,
  PENDING = 1,
  PROVISIONING = 2,
  PROVISIONED = 3,
  DEPROVISIONING = 4,
  FAILED = 5,
}

export enum InfraMachineConfigSpecAcceptanceStatus {
  PENDING = 0,
  ACCEPTED = 1,
  REJECTED = 2,
}

export enum InfraMachineConfigSpecMachinePowerState {
  POWER_STATE_DEFAULT = 0,
  POWER_STATE_OFF = 1,
  POWER_STATE_ON = 2,
}

export enum SecretRotationSpecStatus {
  IDLE = 0,
  IN_PROGRESS = 1,
}

export enum SecretRotationSpecPhase {
  OK = 0,
  PRE_ROTATE = 1,
  ROTATE = 2,
  POST_ROTATE = 3,
}

export enum SecretRotationSpecComponent {
  NONE = 0,
  TALOS_CA = 1,
}

export type MachineSpec = {
  management_address?: string
  connected?: boolean
  use_grpc_tunnel?: boolean
}

export type SecurityState = {
  secure_boot?: boolean
  booted_with_uki?: boolean
}

export type Overlay = {
  name?: string
  image?: string
}

export type MetaValue = {
  key?: number
  value?: string
}

export type MachineStatusSpecHardwareStatusProcessor = {
  core_count?: number
  thread_count?: number
  frequency?: number
  description?: string
  manufacturer?: string
}

export type MachineStatusSpecHardwareStatusMemoryModule = {
  size_mb?: number
  description?: string
}

export type MachineStatusSpecHardwareStatusBlockDevice = {
  size?: string
  model?: string
  linux_name?: string
  name?: string
  serial?: string
  uuid?: string
  wwid?: string
  type?: string
  bus_path?: string
  system_disk?: boolean
  readonly?: boolean
  transport?: string
}

export type MachineStatusSpecHardwareStatus = {
  processors?: MachineStatusSpecHardwareStatusProcessor[]
  memory_modules?: MachineStatusSpecHardwareStatusMemoryModule[]
  blockdevices?: MachineStatusSpecHardwareStatusBlockDevice[]
  arch?: string
}

export type MachineStatusSpecNetworkStatusNetworkLinkStatus = {
  linux_name?: string
  hardware_address?: string
  speed_mbps?: number
  link_up?: boolean
  description?: string
}

export type MachineStatusSpecNetworkStatus = {
  hostname?: string
  domainname?: string
  addresses?: string[]
  default_gateways?: string[]
  network_links?: MachineStatusSpecNetworkStatusNetworkLinkStatus[]
}

export type MachineStatusSpecPlatformMetadata = {
  platform?: string
  hostname?: string
  region?: string
  zone?: string
  instance_type?: string
  instance_id?: string
  provider_id?: string
  spot?: boolean
}

export type MachineStatusSpecSchematicInitialState = {
  extensions?: string[]
}

export type MachineStatusSpecSchematic = {
  id?: string
  invalid?: boolean
  extensions?: string[]
  initial_schematic?: string
  overlay?: Overlay
  kernel_args?: string[]
  meta_values?: MetaValue[]
  full_id?: string
  in_agent_mode?: boolean
  raw?: string
  initial_state?: MachineStatusSpecSchematicInitialState
}

export type MachineStatusSpecDiagnostic = {
  id?: string
  message?: string
  details?: string[]
}

export type MachineStatusSpec = {
  talos_version?: string
  hardware?: MachineStatusSpecHardwareStatus
  network?: MachineStatusSpecNetworkStatus
  last_error?: string
  management_address?: string
  connected?: boolean
  maintenance?: boolean
  cluster?: string
  role?: MachineStatusSpecRole
  platform_metadata?: MachineStatusSpecPlatformMetadata
  image_labels?: {[key: string]: string}
  schematic?: MachineStatusSpecSchematic
  initial_talos_version?: string
  diagnostics?: MachineStatusSpecDiagnostic[]
  power_state?: MachineStatusSpecPowerState
  security_state?: SecurityState
  kernel_cmdline?: string
}

export type TalosConfigSpec = {
  ca?: string
  crt?: string
  key?: string
}

export type ClusterSpecFeatures = {
  enable_workload_proxy?: boolean
  disk_encryption?: boolean
  use_embedded_discovery_service?: boolean
}

export type ClusterSpec = {
  kubernetes_version?: string
  talos_version?: string
  features?: ClusterSpecFeatures
  backup_configuration?: EtcdBackupConf
}

export type ClusterTaintSpec = {
}

export type EtcdBackupConf = {
  interval?: GoogleProtobufDuration.Duration
  enabled?: boolean
}

export type EtcdBackupEncryptionSpec = {
  encryption_key?: Uint8Array
}

export type EtcdBackupHeader = {
  version?: string
}

export type EtcdBackupSpec = {
  created_at?: GoogleProtobufTimestamp.Timestamp
  snapshot?: string
  size?: string
}

export type BackupDataSpec = {
  interval?: GoogleProtobufDuration.Duration
  cluster_uuid?: string
  encryption_key?: Uint8Array
  aes_cbc_encryption_secret?: string
  secretbox_encryption_secret?: string
}

export type EtcdBackupS3ConfSpec = {
  bucket?: string
  region?: string
  endpoint?: string
  access_key_id?: string
  secret_access_key?: string
  session_token?: string
}

export type EtcdBackupStatusSpec = {
  status?: EtcdBackupStatusSpecStatus
  error?: string
  last_backup_time?: GoogleProtobufTimestamp.Timestamp
  last_backup_attempt?: GoogleProtobufTimestamp.Timestamp
}

export type EtcdManualBackupSpec = {
  backup_at?: GoogleProtobufTimestamp.Timestamp
}

export type EtcdBackupStoreStatusSpec = {
  configuration_name?: string
  configuration_error?: string
}

export type EtcdBackupOverallStatusSpec = {
  configuration_name?: string
  configuration_error?: string
  last_backup_status?: EtcdBackupStatusSpec
}

export type ClusterMachineSpec = {
  kubernetes_version?: string
}

export type ClusterMachineConfigPatchesSpec = {
  patches?: string[]
  compressed_patches?: Uint8Array[]
}

export type ClusterMachineTalosVersionSpec = {
  talos_version?: string
  schematic_id?: string
}

export type ClusterMachineConfigSpec = {
  data?: Uint8Array
  generation_error?: string
  compressed_data?: Uint8Array
  without_comments?: boolean
  grub_use_uki_cmdline?: boolean
}

export type RedactedClusterMachineConfigSpec = {
  data?: string
  compressed_data?: Uint8Array
}

export type ClusterMachineIdentitySpec = {
  node_identity?: string
  etcd_member_id?: string
  nodename?: string
  node_ips?: string[]
  discovery_service_endpoint?: string
}

export type ClusterMachineStatusSpecProvisionStatus = {
  provider_id?: string
  request_id?: string
}

export type ClusterMachineStatusSpec = {
  ready?: boolean
  stage?: ClusterMachineStatusSpecStage
  apid_available?: boolean
  config_up_to_date?: boolean
  last_config_error?: string
  management_address?: string
  config_apply_status?: ConfigApplyStatus
  is_removed?: boolean
  provision_status?: ClusterMachineStatusSpecProvisionStatus
}

export type Machines = {
  total?: number
  healthy?: number
  connected?: number
  requested?: number
}

export type ClusterStatusSpec = {
  available?: boolean
  machines?: Machines
  phase?: ClusterStatusSpecPhase
  ready?: boolean
  kubernetesAPIReady?: boolean
  controlplaneReady?: boolean
  has_connected_control_planes?: boolean
  use_embedded_discovery_service?: boolean
}

export type ClusterUUID = {
  uuid?: string
}

export type ClusterConfigVersionSpec = {
  version?: string
}

export type ClusterMachineConfigStatusSpec = {
  cluster_machine_config_version?: string
  cluster_machine_config_sha256?: string
  last_config_error?: string
  talos_version?: string
  schematic_id?: string
  redacted_current_machine_config?: string
  compressed_redacted_machine_config?: Uint8Array
}

export type MachinePendingUpdatesSpecUpgrade = {
  from_schematic?: string
  to_schematic?: string
  from_version?: string
  to_version?: string
}

export type MachinePendingUpdatesSpec = {
  upgrade?: MachinePendingUpdatesSpecUpgrade
  config_diff?: string
}

export type ClusterBootstrapStatusSpec = {
  bootstrapped?: boolean
}

export type ClusterSecretsSpecCertsCA = {
  crt?: Uint8Array
  key?: Uint8Array
}

export type ClusterSecretsSpecCerts = {
  os?: ClusterSecretsSpecCertsCA
}

export type ClusterSecretsSpec = {
  data?: Uint8Array
  imported?: boolean
  extra_certs?: ClusterSecretsSpecCerts
}

export type ImportedClusterSecretsSpec = {
  data?: string
}

export type LoadBalancerConfigSpec = {
  bind_port?: string
  siderolink_endpoint?: string
  endpoints?: string[]
}

export type LoadBalancerStatusSpec = {
  healthy?: boolean
  stopped?: boolean
}

export type KubernetesVersionSpec = {
  version?: string
}

export type TalosVersionSpec = {
  version?: string
  compatible_kubernetes_versions?: string[]
  deprecated?: boolean
}

export type InstallationMediaSpec = {
  name?: string
  architecture?: string
  profile?: string
  contentType?: string
  src_file_prefix?: string
  dest_file_prefix?: string
  extension?: string
  no_secure_boot?: boolean
  overlay?: string
  min_talos_version?: string
}

export type ConfigPatchSpec = {
  data?: string
  compressed_data?: Uint8Array
}

export type MachineSetSpecMachineClass = {
  name?: string
  machine_count?: number
  allocation_type?: MachineSetSpecMachineClassType
}

export type MachineSetSpecMachineAllocation = {
  name?: string
  machine_count?: number
  allocation_type?: MachineSetSpecMachineAllocationType
}

export type MachineSetSpecBootstrapSpec = {
  cluster_uuid?: string
  snapshot?: string
}

export type MachineSetSpecRollingUpdateStrategyConfig = {
  max_parallelism?: number
}

export type MachineSetSpecUpdateStrategyConfig = {
  rolling?: MachineSetSpecRollingUpdateStrategyConfig
}

export type MachineSetSpec = {
  update_strategy?: MachineSetSpecUpdateStrategy
  machine_class?: MachineSetSpecMachineAllocation
  bootstrap_spec?: MachineSetSpecBootstrapSpec
  delete_strategy?: MachineSetSpecUpdateStrategy
  update_strategy_config?: MachineSetSpecUpdateStrategyConfig
  delete_strategy_config?: MachineSetSpecUpdateStrategyConfig
  machine_allocation?: MachineSetSpecMachineAllocation
}

export type TalosUpgradeStatusSpec = {
  phase?: TalosUpgradeStatusSpecPhase
  error?: string
  step?: string
  status?: string
  last_upgrade_version?: string
  current_upgrade_version?: string
  upgrade_versions?: string[]
}

export type MachineSetStatusSpec = {
  phase?: MachineSetPhase
  ready?: boolean
  error?: string
  machines?: Machines
  config_hash?: string
  machine_allocation?: MachineSetSpecMachineAllocation
  locked_updates?: number
  config_updates_allowed?: boolean
  update_strategy?: MachineSetSpecUpdateStrategy
  update_strategy_config?: MachineSetSpecUpdateStrategyConfig
}

export type MachineSetNodeSpec = {
}

export type MachineLabelsSpec = {
}

export type MachineStatusSnapshotSpec = {
  machine_status?: MachineMachine.MachineStatusEvent
  power_stage?: MachineStatusSnapshotSpecPowerStage
}

export type ControlPlaneStatusSpecCondition = {
  type?: ConditionType
  reason?: string
  status?: ControlPlaneStatusSpecConditionStatus
  severity?: ControlPlaneStatusSpecConditionSeverity
}

export type ControlPlaneStatusSpec = {
  conditions?: ControlPlaneStatusSpecCondition[]
}

export type ClusterEndpointSpec = {
  management_addresses?: string[]
}

export type KubernetesStatusSpecNodeStatus = {
  nodename?: string
  kubelet_version?: string
  ready?: boolean
}

export type KubernetesStatusSpecStaticPodStatus = {
  app?: string
  version?: string
  ready?: boolean
}

export type KubernetesStatusSpecNodeStaticPods = {
  nodename?: string
  static_pods?: KubernetesStatusSpecStaticPodStatus[]
}

export type KubernetesStatusSpec = {
  nodes?: KubernetesStatusSpecNodeStatus[]
  static_pods?: KubernetesStatusSpecNodeStaticPods[]
}

export type KubernetesUpgradeStatusSpec = {
  phase?: KubernetesUpgradeStatusSpecPhase
  error?: string
  step?: string
  status?: string
  last_upgrade_version?: string
  current_upgrade_version?: string
  upgrade_versions?: string[]
}

export type KubernetesUpgradeManifestStatusSpec = {
  out_of_sync?: number
  last_fatal_error?: string
}

export type DestroyStatusSpec = {
  phase?: string
}


type BaseOngoingTaskSpec = {
  title?: string
  resource_id?: string
}

export type OngoingTaskSpec = BaseOngoingTaskSpec
  & OneOf<{ talos_upgrade: TalosUpgradeStatusSpec; kubernetes_upgrade: KubernetesUpgradeStatusSpec; destroy: DestroyStatusSpec; machine_upgrade: MachineUpgradeStatusSpec; secrets_rotation: ClusterSecretsRotationStatusSpec }>

export type ClusterMachineEncryptionKeySpec = {
  data?: Uint8Array
}

export type ExposedServiceSpec = {
  port?: number
  label?: string
  icon_base64?: string
  url?: string
  error?: string
  has_explicit_alias?: boolean
}

export type ClusterWorkloadProxyStatusSpec = {
  num_exposed_services?: number
}

export type FeaturesConfigSpec = {
  enable_workload_proxying?: boolean
  etcd_backup_settings?: EtcdBackupSettings
  embedded_discovery_service?: boolean
  audit_log_enabled?: boolean
  image_factory_base_url?: string
  user_pilot_settings?: UserPilotSettings
  stripe_settings?: StripeSettings
  talos_pre_release_versions_enabled?: boolean
  image_factory_pxe_base_url?: string
  account?: Account
}

export type UserPilotSettings = {
  app_token?: string
}

export type StripeSettings = {
  enabled?: boolean
  min_commit?: number
}

export type Account = {
  id?: string
  name?: string
}

export type EtcdBackupSettings = {
  tick_interval?: GoogleProtobufDuration.Duration
  min_interval?: GoogleProtobufDuration.Duration
  max_interval?: GoogleProtobufDuration.Duration
}

export type MachineClassSpecProvision = {
  provider_id?: string
  kernel_args?: string[]
  meta_values?: MetaValue[]
  provider_data?: string
  grpc_tunnel?: GrpcTunnelMode
}

export type MachineClassSpec = {
  match_labels?: string[]
  auto_provision?: MachineClassSpecProvision
}

export type MachineConfigGenOptionsSpecInstallImage = {
  talos_version?: string
  schematic_id?: string
  schematic_initialized?: boolean
  schematic_invalid?: boolean
  platform?: string
  security_state?: SecurityState
}

export type MachineConfigGenOptionsSpec = {
  install_disk?: string
  install_image?: MachineConfigGenOptionsSpecInstallImage
}

export type EtcdAuditResultSpec = {
  etcd_member_ids?: string[]
}

export type KubeconfigSpec = {
  data?: Uint8Array
}

export type KubernetesUsageSpecQuantity = {
  requests?: number
  limits?: number
  capacity?: number
}

export type KubernetesUsageSpecPod = {
  count?: number
  capacity?: number
}

export type KubernetesUsageSpec = {
  cpu?: KubernetesUsageSpecQuantity
  mem?: KubernetesUsageSpecQuantity
  storage?: KubernetesUsageSpecQuantity
  pods?: KubernetesUsageSpecPod
}

export type ImagePullRequestSpecNodeImageList = {
  node?: string
  images?: string[]
}

export type ImagePullRequestSpec = {
  node_image_list?: ImagePullRequestSpecNodeImageList[]
}

export type ImagePullStatusSpec = {
  last_processed_node?: string
  last_processed_image?: string
  last_processed_error?: string
  processed_count?: number
  total_count?: number
  request_version?: string
}

export type SchematicSpec = {
}

export type TalosExtensionsSpecInfo = {
  name?: string
  author?: string
  version?: string
  description?: string
  ref?: string
  digest?: string
}

export type TalosExtensionsSpec = {
  items?: TalosExtensionsSpecInfo[]
}

export type SchematicConfigurationSpec = {
  schematic_id?: string
  talos_version?: string
  kernel_args?: string[]
}

export type ExtensionsConfigurationSpec = {
  extensions?: string[]
}

export type KernelArgsSpec = {
  args?: string[]
}

export type KernelArgsStatusSpec = {
  args?: string[]
  current_args?: string[]
  unmet_conditions?: string[]
  current_cmdline?: string
}

export type MachineUpgradeStatusSpec = {
  schematic_id?: string
  talos_version?: string
  current_schematic_id?: string
  current_talos_version?: string
  phase?: MachineUpgradeStatusSpecPhase
  status?: string
  error?: string
  is_maintenance?: boolean
}

export type MachineExtensionsSpec = {
  extensions?: string[]
}

export type MachineExtensionsStatusSpecItem = {
  name?: string
  immutable?: boolean
  phase?: MachineExtensionsStatusSpecItemPhase
}

export type MachineExtensionsStatusSpec = {
  extensions?: MachineExtensionsStatusSpecItem[]
  talos_version?: string
}

export type MachineStatusMetricsSpec = {
  registered_machines_count?: number
  connected_machines_count?: number
  allocated_machines_count?: number
  pending_machines_count?: number
  platforms?: {[key: string]: number}
  secure_boot_status?: {[key: string]: number}
  uki_status?: {[key: string]: number}
}

export type ClusterMetricsSpec = {
  features?: {[key: string]: number}
}

export type ClusterStatusMetricsSpec = {
  not_ready_count?: number
  phases?: {[key: number]: number}
}

export type ClusterKubernetesNodesSpec = {
  nodes?: string[]
}

export type KubernetesNodeAuditResultSpec = {
  deleted_nodes?: string[]
}

export type MachineRequestSetSpec = {
  provider_id?: string
  machine_count?: number
  talos_version?: string
  extensions?: string[]
  kernel_args?: string[]
  meta_values?: MetaValue[]
  provider_data?: string
  grpc_tunnel?: GrpcTunnelMode
}

export type MachineRequestSetStatusSpec = {
}

export type ClusterDiagnosticsSpecNode = {
  id?: string
  num_diagnostics?: number
}

export type ClusterDiagnosticsSpec = {
  nodes?: ClusterDiagnosticsSpecNode[]
}

export type MachineRequestSetPressureSpec = {
  required_machines?: number
}

export type ClusterMachineRequestStatusSpec = {
  status?: string
  machine_uuid?: string
  provider_id?: string
  stage?: ClusterMachineRequestStatusSpecStage
}

export type InfraMachineConfigSpec = {
  power_state?: InfraMachineConfigSpecMachinePowerState
  acceptance_status?: InfraMachineConfigSpecAcceptanceStatus
  extra_kernel_args?: string
  requested_reboot_id?: string
  cordoned?: boolean
}

export type InfraMachineBMCConfigSpecIPMI = {
  address?: string
  port?: number
  username?: string
  password?: string
}

export type InfraMachineBMCConfigSpecAPI = {
  address?: string
}

export type InfraMachineBMCConfigSpec = {
  ipmi?: InfraMachineBMCConfigSpecIPMI
  api?: InfraMachineBMCConfigSpecAPI
}

export type MaintenanceConfigStatusSpec = {
  public_key_at_last_apply?: string
}

export type NodeForceDestroyRequestSpec = {
}

export type DiscoveryAffiliateDeleteTaskSpec = {
  cluster_id?: string
  discovery_service_endpoint?: string
}

export type InfraProviderCombinedStatusSpecHealth = {
  connected?: boolean
  error?: string
  initialized?: boolean
}

export type InfraProviderCombinedStatusSpec = {
  name?: string
  description?: string
  icon?: string
  health?: InfraProviderCombinedStatusSpecHealth
}

export type MachineConfigDiffSpec = {
  diff?: string
}

export type InstallationMediaConfigSpecCloud = {
  platform?: string
}

export type InstallationMediaConfigSpecSBC = {
  overlay?: string
  overlay_options?: string
}

export type InstallationMediaConfigSpec = {
  talos_version?: string
  architecture?: SpecsVirtual.PlatformConfigSpecArch
  install_extensions?: string[]
  kernel_args?: string
  cloud?: InstallationMediaConfigSpecCloud
  sbc?: InstallationMediaConfigSpecSBC
  join_token?: string
  secure_boot?: boolean
  grpc_tunnel?: GrpcTunnelMode
  machine_labels?: {[key: string]: string}
  bootloader?: ManagementManagement.SchematicBootloader
}

export type RotateTalosCASpec = {
}

export type SecretRotationSpec = {
  status?: SecretRotationSpecStatus
  phase?: SecretRotationSpecPhase
  component?: SecretRotationSpecComponent
  certs?: ClusterSecretsSpecCerts
  extra_certs?: ClusterSecretsSpecCerts
  backup_certs_os?: ClusterSecretsSpecCertsCA[]
}

export type ClusterSecretsRotationStatusSpec = {
  phase?: SecretRotationSpecPhase
  component?: SecretRotationSpecComponent
  error?: string
  step?: string
  status?: string
}

export type ClusterMachineSecretsSpecRotation = {
  status?: SecretRotationSpecStatus
  phase?: SecretRotationSpecPhase
  component?: SecretRotationSpecComponent
  extra_certs?: ClusterSecretsSpecCerts
  secret_rotation_version?: string
}

export type ClusterMachineSecretsSpec = {
  data?: Uint8Array
  rotation?: ClusterMachineSecretsSpecRotation
}