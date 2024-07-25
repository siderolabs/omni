/* eslint-disable */
// @ts-nocheck
/*
* This file is a generated Typescript file for GRPC Gateway, DO NOT MODIFY
*/

import * as GoogleProtobufDuration from "../../google/protobuf/duration.pb"
import * as GoogleProtobufTimestamp from "../../google/protobuf/timestamp.pb"
import * as MachineMachine from "../../talos/machine/machine.pb"

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
}

export enum ConditionType {
  UnknownCondition = 0,
  Etcd = 1,
  WireguardConnection = 2,
}

export enum MachineStatusSpecRole {
  NONE = 0,
  CONTROL_PLANE = 1,
  WORKER = 2,
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

export enum MachineSetSpecMachineClassAllocationType {
  Static = 0,
  Unlimited = 1,
}

export enum TalosUpgradeStatusSpecPhase {
  Unknown = 0,
  Upgrading = 1,
  Done = 2,
  Failed = 3,
  Reverting = 4,
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

export enum ExtensionsConfigurationStatusSpecPhase {
  Unknown = 0,
  Ready = 1,
  Failed = 2,
}

export enum MachineExtensionsStatusSpecItemPhase {
  Installed = 0,
  Installing = 1,
  Removing = 2,
}

export type MachineSpec = {
  management_address?: string
  connected?: boolean
}

export type SecureBootStatus = {
  enabled?: boolean
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

export type MachineStatusSpecSchematicOverlay = {
  name?: string
  image?: string
}

export type MachineStatusSpecSchematicMetaValue = {
  key?: number
  value?: string
}

export type MachineStatusSpecSchematic = {
  id?: string
  invalid?: boolean
  extensions?: string[]
  initial_schematic?: string
  overlay?: MachineStatusSpecSchematicOverlay
  kernel_args?: string[]
  meta_values?: MachineStatusSpecSchematicMetaValue[]
  full_id?: string
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
  secure_boot_status?: SecureBootStatus
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
  install_image?: string
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
}

export type ClusterMachineTalosVersionSpec = {
  talos_version?: string
  schematic_id?: string
}

export type ClusterMachineConfigSpec = {
  data?: Uint8Array
  cluster_machine_version?: string
  generation_error?: string
}

export type RedactedClusterMachineConfigSpec = {
  data?: string
}

export type ClusterMachineIdentitySpec = {
  node_identity?: string
  etcd_member_id?: string
  nodename?: string
  node_ips?: string[]
}

export type ClusterMachineTemplateSpec = {
  install_image?: string
  kubernetes_version?: string
  install_disk?: string
  patch?: string
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
  cluster_machine_version?: string
  cluster_machine_config_sha256?: string
  last_config_error?: string
  talos_version?: string
  schematic_id?: string
}

export type ClusterBootstrapStatusSpec = {
  bootstrapped?: boolean
}

export type ClusterSecretsSpec = {
  data?: Uint8Array
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
}

export type ConfigPatchSpec = {
  data?: string
}

export type MachineSetSpecMachineClass = {
  name?: string
  machine_count?: number
  allocation_type?: MachineSetSpecMachineClassAllocationType
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
  machine_class?: MachineSetSpecMachineClass
  bootstrap_spec?: MachineSetSpecBootstrapSpec
  delete_strategy?: MachineSetSpecUpdateStrategy
  update_strategy_config?: MachineSetSpecUpdateStrategyConfig
  delete_strategy_config?: MachineSetSpecUpdateStrategyConfig
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
  machine_class?: MachineSetSpecMachineClass
  locked_updates?: number
}

export type MachineSetRequiredMachinesSpec = {
  required_additional_machines?: number
}

export type MachineSetNodeSpec = {
}

export type MachineLabelsSpec = {
}

export type MachineStatusSnapshotSpec = {
  machine_status?: MachineMachine.MachineStatusEvent
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
}

export type OngoingTaskSpec = BaseOngoingTaskSpec
  & OneOf<{ talos_upgrade: TalosUpgradeStatusSpec; kubernetes_upgrade: KubernetesUpgradeStatusSpec; destroy: DestroyStatusSpec }>

export type ClusterMachineEncryptionKeySpec = {
  data?: Uint8Array
}

export type ExposedServiceSpec = {
  port?: number
  label?: string
  icon_base64?: string
  url?: string
}

export type ClusterWorkloadProxyStatusSpec = {
  num_exposed_services?: number
}

export type FeaturesConfigSpec = {
  enable_workload_proxying?: boolean
  etcd_backup_settings?: EtcdBackupSettings
  embedded_discovery_service?: boolean
}

export type EtcdBackupSettings = {
  tick_interval?: GoogleProtobufDuration.Duration
  min_interval?: GoogleProtobufDuration.Duration
  max_interval?: GoogleProtobufDuration.Duration
}

export type MachineClassSpec = {
  match_labels?: string[]
}

export type MachineClassStatusSpec = {
  required_additional_machines?: number
}

export type MachineConfigGenOptionsSpecInstallImage = {
  talos_version?: string
  schematic_id?: string
  schematic_initialized?: boolean
  schematic_invalid?: boolean
  secure_boot_status?: SecureBootStatus
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
}

export type ExtensionsConfigurationSpec = {
  extensions?: string[]
}

export type ExtensionsConfigurationStatusSpec = {
  phase?: ExtensionsConfigurationStatusSpecPhase
  error?: string
  extensions?: string[]
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