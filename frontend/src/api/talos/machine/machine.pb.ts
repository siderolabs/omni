/* eslint-disable */
// @ts-nocheck
/*
* This file is a generated Typescript file for GRPC Gateway, DO NOT MODIFY
*/

import * as CommonCommon from "../../common/common.pb"
import * as fm from "../../fetch.pb"
import * as GoogleProtobufAny from "../../google/protobuf/any.pb"
import * as GoogleProtobufDuration from "../../google/protobuf/duration.pb"
import * as GoogleProtobufEmpty from "../../google/protobuf/empty.pb"
import * as GoogleProtobufTimestamp from "../../google/protobuf/timestamp.pb"

export enum ApplyConfigurationRequestMode {
  REBOOT = 0,
  AUTO = 1,
  NO_REBOOT = 2,
  STAGED = 3,
  TRY = 4,
}

export enum RebootRequestMode {
  DEFAULT = 0,
  POWERCYCLE = 1,
}

export enum SequenceEventAction {
  NOOP = 0,
  START = 1,
  STOP = 2,
}

export enum PhaseEventAction {
  START = 0,
  STOP = 1,
}

export enum TaskEventAction {
  START = 0,
  STOP = 1,
}

export enum ServiceStateEventAction {
  INITIALIZED = 0,
  PREPARING = 1,
  WAITING = 2,
  RUNNING = 3,
  STOPPING = 4,
  FINISHED = 5,
  FAILED = 6,
  SKIPPED = 7,
  STARTING = 8,
}

export enum MachineStatusEventMachineStage {
  UNKNOWN = 0,
  BOOTING = 1,
  INSTALLING = 2,
  MAINTENANCE = 3,
  RUNNING = 4,
  REBOOTING = 5,
  SHUTTING_DOWN = 6,
  RESETTING = 7,
  UPGRADING = 8,
}

export enum ResetRequestWipeMode {
  ALL = 0,
  SYSTEM_DISK = 1,
  USER_DISKS = 2,
}

export enum UpgradeRequestRebootMode {
  DEFAULT = 0,
  POWERCYCLE = 1,
}

export enum ListRequestType {
  REGULAR = 0,
  DIRECTORY = 1,
  SYMLINK = 2,
}

export enum EtcdMemberAlarmAlarmType {
  NONE = 0,
  NOSPACE = 1,
  CORRUPT = 2,
}

export enum MachineConfigMachineType {
  TYPE_UNKNOWN = 0,
  TYPE_INIT = 1,
  TYPE_CONTROL_PLANE = 2,
  TYPE_WORKER = 3,
}

export enum NetstatRequestFilter {
  ALL = 0,
  CONNECTED = 1,
  LISTENING = 2,
}

export enum ConnectRecordState {
  RESERVED = 0,
  ESTABLISHED = 1,
  SYN_SENT = 2,
  SYN_RECV = 3,
  FIN_WAIT1 = 4,
  FIN_WAIT2 = 5,
  TIME_WAIT = 6,
  CLOSE = 7,
  CLOSEWAIT = 8,
  LASTACK = 9,
  LISTEN = 10,
  CLOSING = 11,
}

export enum ConnectRecordTimerActive {
  OFF = 0,
  ON = 1,
  KEEPALIVE = 2,
  TIMEWAIT = 3,
  PROBE = 4,
}

export type ApplyConfigurationRequest = {
  data?: Uint8Array
  mode?: ApplyConfigurationRequestMode
  dry_run?: boolean
  try_mode_timeout?: GoogleProtobufDuration.Duration
}

export type ApplyConfiguration = {
  metadata?: CommonCommon.Metadata
  warnings?: string[]
  mode?: ApplyConfigurationRequestMode
  mode_details?: string
}

export type ApplyConfigurationResponse = {
  messages?: ApplyConfiguration[]
}

export type RebootRequest = {
  mode?: RebootRequestMode
}

export type Reboot = {
  metadata?: CommonCommon.Metadata
  actor_id?: string
}

export type RebootResponse = {
  messages?: Reboot[]
}

export type BootstrapRequest = {
  recover_etcd?: boolean
  recover_skip_hash_check?: boolean
}

export type Bootstrap = {
  metadata?: CommonCommon.Metadata
}

export type BootstrapResponse = {
  messages?: Bootstrap[]
}

export type SequenceEvent = {
  sequence?: string
  action?: SequenceEventAction
  error?: CommonCommon.Error
}

export type PhaseEvent = {
  phase?: string
  action?: PhaseEventAction
}

export type TaskEvent = {
  task?: string
  action?: TaskEventAction
}

export type ServiceStateEvent = {
  service?: string
  action?: ServiceStateEventAction
  message?: string
  health?: ServiceHealth
}

export type RestartEvent = {
  cmd?: string
}

export type ConfigLoadErrorEvent = {
  error?: string
}

export type ConfigValidationErrorEvent = {
  error?: string
}

export type AddressEvent = {
  hostname?: string
  addresses?: string[]
}

export type MachineStatusEventMachineStatusUnmetCondition = {
  name?: string
  reason?: string
}

export type MachineStatusEventMachineStatus = {
  ready?: boolean
  unmet_conditions?: MachineStatusEventMachineStatusUnmetCondition[]
}

export type MachineStatusEvent = {
  stage?: MachineStatusEventMachineStage
  status?: MachineStatusEventMachineStatus
}

export type EventsRequest = {
  tail_events?: number
  tail_id?: string
  tail_seconds?: number
  with_actor_id?: string
}

export type Event = {
  metadata?: CommonCommon.Metadata
  data?: GoogleProtobufAny.Any
  id?: string
  actor_id?: string
}

export type ResetPartitionSpec = {
  label?: string
  wipe?: boolean
}

export type ResetRequest = {
  graceful?: boolean
  reboot?: boolean
  system_partitions_to_wipe?: ResetPartitionSpec[]
  user_disks_to_wipe?: string[]
  mode?: ResetRequestWipeMode
}

export type Reset = {
  metadata?: CommonCommon.Metadata
  actor_id?: string
}

export type ResetResponse = {
  messages?: Reset[]
}

export type Shutdown = {
  metadata?: CommonCommon.Metadata
  actor_id?: string
}

export type ShutdownRequest = {
  force?: boolean
}

export type ShutdownResponse = {
  messages?: Shutdown[]
}

export type UpgradeRequest = {
  image?: string
  preserve?: boolean
  stage?: boolean
  force?: boolean
  reboot_mode?: UpgradeRequestRebootMode
}

export type Upgrade = {
  metadata?: CommonCommon.Metadata
  ack?: string
  actor_id?: string
}

export type UpgradeResponse = {
  messages?: Upgrade[]
}

export type ServiceList = {
  metadata?: CommonCommon.Metadata
  services?: ServiceInfo[]
}

export type ServiceListResponse = {
  messages?: ServiceList[]
}

export type ServiceInfo = {
  id?: string
  state?: string
  events?: ServiceEvents
  health?: ServiceHealth
}

export type ServiceEvents = {
  events?: ServiceEvent[]
}

export type ServiceEvent = {
  msg?: string
  state?: string
  ts?: GoogleProtobufTimestamp.Timestamp
}

export type ServiceHealth = {
  unknown?: boolean
  healthy?: boolean
  last_message?: string
  last_change?: GoogleProtobufTimestamp.Timestamp
}

export type ServiceStartRequest = {
  id?: string
}

export type ServiceStart = {
  metadata?: CommonCommon.Metadata
  resp?: string
}

export type ServiceStartResponse = {
  messages?: ServiceStart[]
}

export type ServiceStopRequest = {
  id?: string
}

export type ServiceStop = {
  metadata?: CommonCommon.Metadata
  resp?: string
}

export type ServiceStopResponse = {
  messages?: ServiceStop[]
}

export type ServiceRestartRequest = {
  id?: string
}

export type ServiceRestart = {
  metadata?: CommonCommon.Metadata
  resp?: string
}

export type ServiceRestartResponse = {
  messages?: ServiceRestart[]
}

export type CopyRequest = {
  root_path?: string
}

export type ListRequest = {
  root?: string
  recurse?: boolean
  recursion_depth?: number
  types?: ListRequestType[]
  report_xattrs?: boolean
}

export type DiskUsageRequest = {
  recursion_depth?: number
  all?: boolean
  threshold?: string
  paths?: string[]
}

export type FileInfo = {
  metadata?: CommonCommon.Metadata
  name?: string
  size?: string
  mode?: number
  modified?: string
  is_dir?: boolean
  error?: string
  link?: string
  relative_name?: string
  uid?: number
  gid?: number
  xattrs?: Xattr[]
}

export type Xattr = {
  name?: string
  data?: Uint8Array
}

export type DiskUsageInfo = {
  metadata?: CommonCommon.Metadata
  name?: string
  size?: string
  error?: string
  relative_name?: string
}

export type Mounts = {
  metadata?: CommonCommon.Metadata
  stats?: MountStat[]
}

export type MountsResponse = {
  messages?: Mounts[]
}

export type MountStat = {
  filesystem?: string
  size?: string
  available?: string
  mounted_on?: string
}

export type Version = {
  metadata?: CommonCommon.Metadata
  version?: VersionInfo
  platform?: PlatformInfo
  features?: FeaturesInfo
}

export type VersionResponse = {
  messages?: Version[]
}

export type VersionInfo = {
  tag?: string
  sha?: string
  built?: string
  go_version?: string
  os?: string
  arch?: string
}

export type PlatformInfo = {
  name?: string
  mode?: string
}

export type FeaturesInfo = {
  rbac?: boolean
}

export type LogsRequest = {
  namespace?: string
  id?: string
  driver?: CommonCommon.ContainerDriver
  follow?: boolean
  tail_lines?: number
}

export type ReadRequest = {
  path?: string
}

export type LogsContainer = {
  metadata?: CommonCommon.Metadata
  ids?: string[]
}

export type LogsContainersResponse = {
  messages?: LogsContainer[]
}

export type RollbackRequest = {
}

export type Rollback = {
  metadata?: CommonCommon.Metadata
}

export type RollbackResponse = {
  messages?: Rollback[]
}

export type ContainersRequest = {
  namespace?: string
  driver?: CommonCommon.ContainerDriver
}

export type ContainerInfo = {
  namespace?: string
  id?: string
  uid?: string
  internal_id?: string
  image?: string
  pid?: number
  status?: string
  pod_id?: string
  name?: string
  network_namespace?: string
}

export type Container = {
  metadata?: CommonCommon.Metadata
  containers?: ContainerInfo[]
}

export type ContainersResponse = {
  messages?: Container[]
}

export type DmesgRequest = {
  follow?: boolean
  tail?: boolean
}

export type ProcessesResponse = {
  messages?: Process[]
}

export type Process = {
  metadata?: CommonCommon.Metadata
  processes?: ProcessInfo[]
}

export type ProcessInfo = {
  pid?: number
  ppid?: number
  state?: string
  threads?: number
  cpu_time?: number
  virtual_memory?: string
  resident_memory?: string
  command?: string
  executable?: string
  args?: string
  label?: string
}

export type RestartRequest = {
  namespace?: string
  id?: string
  driver?: CommonCommon.ContainerDriver
}

export type Restart = {
  metadata?: CommonCommon.Metadata
}

export type RestartResponse = {
  messages?: Restart[]
}

export type StatsRequest = {
  namespace?: string
  driver?: CommonCommon.ContainerDriver
}

export type Stats = {
  metadata?: CommonCommon.Metadata
  stats?: Stat[]
}

export type StatsResponse = {
  messages?: Stats[]
}

export type Stat = {
  namespace?: string
  id?: string
  memory_usage?: string
  cpu_usage?: string
  pod_id?: string
  name?: string
}

export type Memory = {
  metadata?: CommonCommon.Metadata
  meminfo?: MemInfo
}

export type MemoryResponse = {
  messages?: Memory[]
}

export type MemInfo = {
  memtotal?: string
  memfree?: string
  memavailable?: string
  buffers?: string
  cached?: string
  swapcached?: string
  active?: string
  inactive?: string
  activeanon?: string
  inactiveanon?: string
  activefile?: string
  inactivefile?: string
  unevictable?: string
  mlocked?: string
  swaptotal?: string
  swapfree?: string
  dirty?: string
  writeback?: string
  anonpages?: string
  mapped?: string
  shmem?: string
  slab?: string
  sreclaimable?: string
  sunreclaim?: string
  kernelstack?: string
  pagetables?: string
  nfsunstable?: string
  bounce?: string
  writebacktmp?: string
  commitlimit?: string
  committedas?: string
  vmalloctotal?: string
  vmallocused?: string
  vmallocchunk?: string
  hardwarecorrupted?: string
  anonhugepages?: string
  shmemhugepages?: string
  shmempmdmapped?: string
  cmatotal?: string
  cmafree?: string
  hugepagestotal?: string
  hugepagesfree?: string
  hugepagesrsvd?: string
  hugepagessurp?: string
  hugepagesize?: string
  directmap4k?: string
  directmap2m?: string
  directmap1g?: string
}

export type HostnameResponse = {
  messages?: Hostname[]
}

export type Hostname = {
  metadata?: CommonCommon.Metadata
  hostname?: string
}

export type LoadAvgResponse = {
  messages?: LoadAvg[]
}

export type LoadAvg = {
  metadata?: CommonCommon.Metadata
  load1?: number
  load5?: number
  load15?: number
}

export type SystemStatResponse = {
  messages?: SystemStat[]
}

export type SystemStat = {
  metadata?: CommonCommon.Metadata
  boot_time?: string
  cpu_total?: CPUStat
  cpu?: CPUStat[]
  irq_total?: string
  irq?: string[]
  context_switches?: string
  process_created?: string
  process_running?: string
  process_blocked?: string
  soft_irq_total?: string
  soft_irq?: SoftIRQStat
}

export type CPUStat = {
  user?: number
  nice?: number
  system?: number
  idle?: number
  iowait?: number
  irq?: number
  soft_irq?: number
  steal?: number
  guest?: number
  guest_nice?: number
}

export type SoftIRQStat = {
  hi?: string
  timer?: string
  net_tx?: string
  net_rx?: string
  block?: string
  block_io_poll?: string
  tasklet?: string
  sched?: string
  hrtimer?: string
  rcu?: string
}

export type CPUFreqStatsResponse = {
  messages?: CPUsFreqStats[]
}

export type CPUsFreqStats = {
  metadata?: CommonCommon.Metadata
  cpu_freq_stats?: CPUFreqStats[]
}

export type CPUFreqStats = {
  current_frequency?: string
  minimum_frequency?: string
  maximum_frequency?: string
  governor?: string
}

export type CPUInfoResponse = {
  messages?: CPUsInfo[]
}

export type CPUsInfo = {
  metadata?: CommonCommon.Metadata
  cpu_info?: CPUInfo[]
}

export type CPUInfo = {
  processor?: number
  vendor_id?: string
  cpu_family?: string
  model?: string
  model_name?: string
  stepping?: string
  microcode?: string
  cpu_mhz?: number
  cache_size?: string
  physical_id?: string
  siblings?: number
  core_id?: string
  cpu_cores?: number
  apic_id?: string
  initial_apic_id?: string
  fpu?: string
  fpu_exception?: string
  cpu_id_level?: number
  wp?: string
  flags?: string[]
  bugs?: string[]
  bogo_mips?: number
  cl_flush_size?: number
  cache_alignment?: number
  address_sizes?: string
  power_management?: string
}

export type NetworkDeviceStatsResponse = {
  messages?: NetworkDeviceStats[]
}

export type NetworkDeviceStats = {
  metadata?: CommonCommon.Metadata
  total?: NetDev
  devices?: NetDev[]
}

export type NetDev = {
  name?: string
  rx_bytes?: string
  rx_packets?: string
  rx_errors?: string
  rx_dropped?: string
  rx_fifo?: string
  rx_frame?: string
  rx_compressed?: string
  rx_multicast?: string
  tx_bytes?: string
  tx_packets?: string
  tx_errors?: string
  tx_dropped?: string
  tx_fifo?: string
  tx_collisions?: string
  tx_carrier?: string
  tx_compressed?: string
}

export type DiskStatsResponse = {
  messages?: DiskStats[]
}

export type DiskStats = {
  metadata?: CommonCommon.Metadata
  total?: DiskStat
  devices?: DiskStat[]
}

export type DiskStat = {
  name?: string
  read_completed?: string
  read_merged?: string
  read_sectors?: string
  read_time_ms?: string
  write_completed?: string
  write_merged?: string
  write_sectors?: string
  write_time_ms?: string
  io_in_progress?: string
  io_time_ms?: string
  io_time_weighted_ms?: string
  discard_completed?: string
  discard_merged?: string
  discard_sectors?: string
  discard_time_ms?: string
}

export type EtcdLeaveClusterRequest = {
}

export type EtcdLeaveCluster = {
  metadata?: CommonCommon.Metadata
}

export type EtcdLeaveClusterResponse = {
  messages?: EtcdLeaveCluster[]
}

export type EtcdRemoveMemberRequest = {
  member?: string
}

export type EtcdRemoveMember = {
  metadata?: CommonCommon.Metadata
}

export type EtcdRemoveMemberResponse = {
  messages?: EtcdRemoveMember[]
}

export type EtcdRemoveMemberByIDRequest = {
  member_id?: string
}

export type EtcdRemoveMemberByID = {
  metadata?: CommonCommon.Metadata
}

export type EtcdRemoveMemberByIDResponse = {
  messages?: EtcdRemoveMemberByID[]
}

export type EtcdForfeitLeadershipRequest = {
}

export type EtcdForfeitLeadership = {
  metadata?: CommonCommon.Metadata
  member?: string
}

export type EtcdForfeitLeadershipResponse = {
  messages?: EtcdForfeitLeadership[]
}

export type EtcdMemberListRequest = {
  query_local?: boolean
}

export type EtcdMember = {
  id?: string
  hostname?: string
  peer_urls?: string[]
  client_urls?: string[]
  is_learner?: boolean
}

export type EtcdMembers = {
  metadata?: CommonCommon.Metadata
  legacy_members?: string[]
  members?: EtcdMember[]
}

export type EtcdMemberListResponse = {
  messages?: EtcdMembers[]
}

export type EtcdSnapshotRequest = {
}

export type EtcdRecover = {
  metadata?: CommonCommon.Metadata
}

export type EtcdRecoverResponse = {
  messages?: EtcdRecover[]
}

export type EtcdAlarmListResponse = {
  messages?: EtcdAlarm[]
}

export type EtcdAlarm = {
  metadata?: CommonCommon.Metadata
  member_alarms?: EtcdMemberAlarm[]
}

export type EtcdMemberAlarm = {
  member_id?: string
  alarm?: EtcdMemberAlarmAlarmType
}

export type EtcdAlarmDisarmResponse = {
  messages?: EtcdAlarmDisarm[]
}

export type EtcdAlarmDisarm = {
  metadata?: CommonCommon.Metadata
  member_alarms?: EtcdMemberAlarm[]
}

export type EtcdDefragmentResponse = {
  messages?: EtcdDefragment[]
}

export type EtcdDefragment = {
  metadata?: CommonCommon.Metadata
}

export type EtcdStatusResponse = {
  messages?: EtcdStatus[]
}

export type EtcdStatus = {
  metadata?: CommonCommon.Metadata
  member_status?: EtcdMemberStatus
}

export type EtcdMemberStatus = {
  storage_version?: string
  member_id?: string
  protocol_version?: string
  db_size?: string
  db_size_in_use?: string
  leader?: string
  raft_index?: string
  raft_term?: string
  raft_applied_index?: string
  errors?: string[]
  is_learner?: boolean
}

export type EtcdDowngradeValidateRequest = {
  version?: string
}

export type EtcdDowngradeValidateResponse = {
  messages?: EtcdDowngradeValidate[]
}

export type EtcdDowngradeValidate = {
  metadata?: CommonCommon.Metadata
  cluster_downgrade?: EtcdClusterDowngrade
}

export type EtcdDowngradeEnableRequest = {
  version?: string
}

export type EtcdDowngradeEnableResponse = {
  messages?: EtcdDowngradeEnable[]
}

export type EtcdDowngradeEnable = {
  metadata?: CommonCommon.Metadata
  cluster_downgrade?: EtcdClusterDowngrade
}

export type EtcdDowngradeCancelResponse = {
  messages?: EtcdDowngradeCancel[]
}

export type EtcdDowngradeCancel = {
  metadata?: CommonCommon.Metadata
  cluster_downgrade?: EtcdClusterDowngrade
}

export type EtcdClusterDowngrade = {
  cluster_version?: string
}

export type RouteConfig = {
  network?: string
  gateway?: string
  metric?: number
}

export type DHCPOptionsConfig = {
  route_metric?: number
}

export type NetworkDeviceConfig = {
  interface?: string
  cidr?: string
  mtu?: number
  dhcp?: boolean
  ignore?: boolean
  dhcp_options?: DHCPOptionsConfig
  routes?: RouteConfig[]
}

export type NetworkConfig = {
  hostname?: string
  interfaces?: NetworkDeviceConfig[]
}

export type InstallConfig = {
  install_disk?: string
  install_image?: string
}

export type MachineConfig = {
  type?: MachineConfigMachineType
  install_config?: InstallConfig
  network_config?: NetworkConfig
  kubernetes_version?: string
}

export type ControlPlaneConfig = {
  endpoint?: string
}

export type CNIConfig = {
  name?: string
  urls?: string[]
}

export type ClusterNetworkConfig = {
  dns_domain?: string
  cni_config?: CNIConfig
}

export type ClusterConfig = {
  name?: string
  control_plane?: ControlPlaneConfig
  cluster_network?: ClusterNetworkConfig
  allow_scheduling_on_control_planes?: boolean
}

export type GenerateConfigurationRequest = {
  config_version?: string
  cluster_config?: ClusterConfig
  machine_config?: MachineConfig
  override_time?: GoogleProtobufTimestamp.Timestamp
}

export type GenerateConfiguration = {
  metadata?: CommonCommon.Metadata
  data?: Uint8Array[]
  talosconfig?: Uint8Array
}

export type GenerateConfigurationResponse = {
  messages?: GenerateConfiguration[]
}

export type GenerateClientConfigurationRequest = {
  roles?: string[]
  crt_ttl?: GoogleProtobufDuration.Duration
}

export type GenerateClientConfiguration = {
  metadata?: CommonCommon.Metadata
  ca?: Uint8Array
  crt?: Uint8Array
  key?: Uint8Array
  talosconfig?: Uint8Array
}

export type GenerateClientConfigurationResponse = {
  messages?: GenerateClientConfiguration[]
}

export type PacketCaptureRequest = {
  interface?: string
  promiscuous?: boolean
  snap_len?: number
  bpf_filter?: BPFInstruction[]
}

export type BPFInstruction = {
  op?: number
  jt?: number
  jf?: number
  k?: number
}

export type NetstatRequestFeature = {
  pid?: boolean
}

export type NetstatRequestL4proto = {
  tcp?: boolean
  tcp6?: boolean
  udp?: boolean
  udp6?: boolean
  udplite?: boolean
  udplite6?: boolean
  raw?: boolean
  raw6?: boolean
}

export type NetstatRequestNetNS = {
  hostnetwork?: boolean
  netns?: string[]
  allnetns?: boolean
}

export type NetstatRequest = {
  filter?: NetstatRequestFilter
  feature?: NetstatRequestFeature
  l4proto?: NetstatRequestL4proto
  netns?: NetstatRequestNetNS
}

export type ConnectRecordProcess = {
  pid?: number
  name?: string
}

export type ConnectRecord = {
  l4proto?: string
  localip?: string
  localport?: number
  remoteip?: string
  remoteport?: number
  state?: ConnectRecordState
  txqueue?: string
  rxqueue?: string
  tr?: ConnectRecordTimerActive
  timerwhen?: string
  retrnsmt?: string
  uid?: number
  timeout?: string
  inode?: string
  ref?: string
  pointer?: string
  process?: ConnectRecordProcess
  netns?: string
}

export type Netstat = {
  metadata?: CommonCommon.Metadata
  connectrecord?: ConnectRecord[]
}

export type NetstatResponse = {
  messages?: Netstat[]
}

export type MetaWriteRequest = {
  key?: number
  value?: Uint8Array
}

export type MetaWrite = {
  metadata?: CommonCommon.Metadata
}

export type MetaWriteResponse = {
  messages?: MetaWrite[]
}

export type MetaDeleteRequest = {
  key?: number
}

export type MetaDelete = {
  metadata?: CommonCommon.Metadata
}

export type MetaDeleteResponse = {
  messages?: MetaDelete[]
}

export type ImageListRequest = {
  namespace?: CommonCommon.ContainerdNamespace
}

export type ImageListResponse = {
  metadata?: CommonCommon.Metadata
  name?: string
  digest?: string
  size?: string
  created_at?: GoogleProtobufTimestamp.Timestamp
}

export type ImagePullRequest = {
  namespace?: CommonCommon.ContainerdNamespace
  reference?: string
}

export type ImagePull = {
  metadata?: CommonCommon.Metadata
}

export type ImagePullResponse = {
  messages?: ImagePull[]
}

export class MachineService {
  static ApplyConfiguration(req: ApplyConfigurationRequest, ...options: fm.fetchOption[]): Promise<ApplyConfigurationResponse> {
    return fm.fetchReq<ApplyConfigurationRequest, ApplyConfigurationResponse>("POST", `/machine.MachineService/ApplyConfiguration`, req, ...options)
  }
  static Bootstrap(req: BootstrapRequest, ...options: fm.fetchOption[]): Promise<BootstrapResponse> {
    return fm.fetchReq<BootstrapRequest, BootstrapResponse>("POST", `/machine.MachineService/Bootstrap`, req, ...options)
  }
  static Containers(req: ContainersRequest, ...options: fm.fetchOption[]): Promise<ContainersResponse> {
    return fm.fetchReq<ContainersRequest, ContainersResponse>("POST", `/machine.MachineService/Containers`, req, ...options)
  }
  static Copy(req: CopyRequest, entityNotifier?: fm.NotifyStreamEntityArrival<CommonCommon.Data>, ...options: fm.fetchOption[]): Promise<void> {
    return fm.fetchStreamingRequest<CopyRequest, CommonCommon.Data>("POST", `/machine.MachineService/Copy`, req, entityNotifier, ...options)
  }
  static CPUFreqStats(req: GoogleProtobufEmpty.Empty, ...options: fm.fetchOption[]): Promise<CPUFreqStatsResponse> {
    return fm.fetchReq<GoogleProtobufEmpty.Empty, CPUFreqStatsResponse>("POST", `/machine.MachineService/CPUFreqStats`, req, ...options)
  }
  static CPUInfo(req: GoogleProtobufEmpty.Empty, ...options: fm.fetchOption[]): Promise<CPUInfoResponse> {
    return fm.fetchReq<GoogleProtobufEmpty.Empty, CPUInfoResponse>("POST", `/machine.MachineService/CPUInfo`, req, ...options)
  }
  static DiskStats(req: GoogleProtobufEmpty.Empty, ...options: fm.fetchOption[]): Promise<DiskStatsResponse> {
    return fm.fetchReq<GoogleProtobufEmpty.Empty, DiskStatsResponse>("POST", `/machine.MachineService/DiskStats`, req, ...options)
  }
  static Dmesg(req: DmesgRequest, entityNotifier?: fm.NotifyStreamEntityArrival<CommonCommon.Data>, ...options: fm.fetchOption[]): Promise<void> {
    return fm.fetchStreamingRequest<DmesgRequest, CommonCommon.Data>("POST", `/machine.MachineService/Dmesg`, req, entityNotifier, ...options)
  }
  static Events(req: EventsRequest, entityNotifier?: fm.NotifyStreamEntityArrival<Event>, ...options: fm.fetchOption[]): Promise<void> {
    return fm.fetchStreamingRequest<EventsRequest, Event>("POST", `/machine.MachineService/Events`, req, entityNotifier, ...options)
  }
  static EtcdMemberList(req: EtcdMemberListRequest, ...options: fm.fetchOption[]): Promise<EtcdMemberListResponse> {
    return fm.fetchReq<EtcdMemberListRequest, EtcdMemberListResponse>("POST", `/machine.MachineService/EtcdMemberList`, req, ...options)
  }
  static EtcdRemoveMemberByID(req: EtcdRemoveMemberByIDRequest, ...options: fm.fetchOption[]): Promise<EtcdRemoveMemberByIDResponse> {
    return fm.fetchReq<EtcdRemoveMemberByIDRequest, EtcdRemoveMemberByIDResponse>("POST", `/machine.MachineService/EtcdRemoveMemberByID`, req, ...options)
  }
  static EtcdLeaveCluster(req: EtcdLeaveClusterRequest, ...options: fm.fetchOption[]): Promise<EtcdLeaveClusterResponse> {
    return fm.fetchReq<EtcdLeaveClusterRequest, EtcdLeaveClusterResponse>("POST", `/machine.MachineService/EtcdLeaveCluster`, req, ...options)
  }
  static EtcdForfeitLeadership(req: EtcdForfeitLeadershipRequest, ...options: fm.fetchOption[]): Promise<EtcdForfeitLeadershipResponse> {
    return fm.fetchReq<EtcdForfeitLeadershipRequest, EtcdForfeitLeadershipResponse>("POST", `/machine.MachineService/EtcdForfeitLeadership`, req, ...options)
  }
  static EtcdSnapshot(req: EtcdSnapshotRequest, entityNotifier?: fm.NotifyStreamEntityArrival<CommonCommon.Data>, ...options: fm.fetchOption[]): Promise<void> {
    return fm.fetchStreamingRequest<EtcdSnapshotRequest, CommonCommon.Data>("POST", `/machine.MachineService/EtcdSnapshot`, req, entityNotifier, ...options)
  }
  static EtcdAlarmList(req: GoogleProtobufEmpty.Empty, ...options: fm.fetchOption[]): Promise<EtcdAlarmListResponse> {
    return fm.fetchReq<GoogleProtobufEmpty.Empty, EtcdAlarmListResponse>("POST", `/machine.MachineService/EtcdAlarmList`, req, ...options)
  }
  static EtcdAlarmDisarm(req: GoogleProtobufEmpty.Empty, ...options: fm.fetchOption[]): Promise<EtcdAlarmDisarmResponse> {
    return fm.fetchReq<GoogleProtobufEmpty.Empty, EtcdAlarmDisarmResponse>("POST", `/machine.MachineService/EtcdAlarmDisarm`, req, ...options)
  }
  static EtcdDefragment(req: GoogleProtobufEmpty.Empty, ...options: fm.fetchOption[]): Promise<EtcdDefragmentResponse> {
    return fm.fetchReq<GoogleProtobufEmpty.Empty, EtcdDefragmentResponse>("POST", `/machine.MachineService/EtcdDefragment`, req, ...options)
  }
  static EtcdStatus(req: GoogleProtobufEmpty.Empty, ...options: fm.fetchOption[]): Promise<EtcdStatusResponse> {
    return fm.fetchReq<GoogleProtobufEmpty.Empty, EtcdStatusResponse>("POST", `/machine.MachineService/EtcdStatus`, req, ...options)
  }
  static EtcdDowngradeValidate(req: EtcdDowngradeValidateRequest, ...options: fm.fetchOption[]): Promise<EtcdDowngradeValidateResponse> {
    return fm.fetchReq<EtcdDowngradeValidateRequest, EtcdDowngradeValidateResponse>("POST", `/machine.MachineService/EtcdDowngradeValidate`, req, ...options)
  }
  static EtcdDowngradeEnable(req: EtcdDowngradeEnableRequest, ...options: fm.fetchOption[]): Promise<EtcdDowngradeEnableResponse> {
    return fm.fetchReq<EtcdDowngradeEnableRequest, EtcdDowngradeEnableResponse>("POST", `/machine.MachineService/EtcdDowngradeEnable`, req, ...options)
  }
  static EtcdDowngradeCancel(req: GoogleProtobufEmpty.Empty, ...options: fm.fetchOption[]): Promise<EtcdDowngradeCancelResponse> {
    return fm.fetchReq<GoogleProtobufEmpty.Empty, EtcdDowngradeCancelResponse>("POST", `/machine.MachineService/EtcdDowngradeCancel`, req, ...options)
  }
  static GenerateConfiguration(req: GenerateConfigurationRequest, ...options: fm.fetchOption[]): Promise<GenerateConfigurationResponse> {
    return fm.fetchReq<GenerateConfigurationRequest, GenerateConfigurationResponse>("POST", `/machine.MachineService/GenerateConfiguration`, req, ...options)
  }
  static Hostname(req: GoogleProtobufEmpty.Empty, ...options: fm.fetchOption[]): Promise<HostnameResponse> {
    return fm.fetchReq<GoogleProtobufEmpty.Empty, HostnameResponse>("POST", `/machine.MachineService/Hostname`, req, ...options)
  }
  static Kubeconfig(req: GoogleProtobufEmpty.Empty, entityNotifier?: fm.NotifyStreamEntityArrival<CommonCommon.Data>, ...options: fm.fetchOption[]): Promise<void> {
    return fm.fetchStreamingRequest<GoogleProtobufEmpty.Empty, CommonCommon.Data>("POST", `/machine.MachineService/Kubeconfig`, req, entityNotifier, ...options)
  }
  static List(req: ListRequest, entityNotifier?: fm.NotifyStreamEntityArrival<FileInfo>, ...options: fm.fetchOption[]): Promise<void> {
    return fm.fetchStreamingRequest<ListRequest, FileInfo>("POST", `/machine.MachineService/List`, req, entityNotifier, ...options)
  }
  static DiskUsage(req: DiskUsageRequest, entityNotifier?: fm.NotifyStreamEntityArrival<DiskUsageInfo>, ...options: fm.fetchOption[]): Promise<void> {
    return fm.fetchStreamingRequest<DiskUsageRequest, DiskUsageInfo>("POST", `/machine.MachineService/DiskUsage`, req, entityNotifier, ...options)
  }
  static LoadAvg(req: GoogleProtobufEmpty.Empty, ...options: fm.fetchOption[]): Promise<LoadAvgResponse> {
    return fm.fetchReq<GoogleProtobufEmpty.Empty, LoadAvgResponse>("POST", `/machine.MachineService/LoadAvg`, req, ...options)
  }
  static Logs(req: LogsRequest, entityNotifier?: fm.NotifyStreamEntityArrival<CommonCommon.Data>, ...options: fm.fetchOption[]): Promise<void> {
    return fm.fetchStreamingRequest<LogsRequest, CommonCommon.Data>("POST", `/machine.MachineService/Logs`, req, entityNotifier, ...options)
  }
  static LogsContainers(req: GoogleProtobufEmpty.Empty, ...options: fm.fetchOption[]): Promise<LogsContainersResponse> {
    return fm.fetchReq<GoogleProtobufEmpty.Empty, LogsContainersResponse>("POST", `/machine.MachineService/LogsContainers`, req, ...options)
  }
  static Memory(req: GoogleProtobufEmpty.Empty, ...options: fm.fetchOption[]): Promise<MemoryResponse> {
    return fm.fetchReq<GoogleProtobufEmpty.Empty, MemoryResponse>("POST", `/machine.MachineService/Memory`, req, ...options)
  }
  static Mounts(req: GoogleProtobufEmpty.Empty, ...options: fm.fetchOption[]): Promise<MountsResponse> {
    return fm.fetchReq<GoogleProtobufEmpty.Empty, MountsResponse>("POST", `/machine.MachineService/Mounts`, req, ...options)
  }
  static NetworkDeviceStats(req: GoogleProtobufEmpty.Empty, ...options: fm.fetchOption[]): Promise<NetworkDeviceStatsResponse> {
    return fm.fetchReq<GoogleProtobufEmpty.Empty, NetworkDeviceStatsResponse>("POST", `/machine.MachineService/NetworkDeviceStats`, req, ...options)
  }
  static Processes(req: GoogleProtobufEmpty.Empty, ...options: fm.fetchOption[]): Promise<ProcessesResponse> {
    return fm.fetchReq<GoogleProtobufEmpty.Empty, ProcessesResponse>("POST", `/machine.MachineService/Processes`, req, ...options)
  }
  static Read(req: ReadRequest, entityNotifier?: fm.NotifyStreamEntityArrival<CommonCommon.Data>, ...options: fm.fetchOption[]): Promise<void> {
    return fm.fetchStreamingRequest<ReadRequest, CommonCommon.Data>("POST", `/machine.MachineService/Read`, req, entityNotifier, ...options)
  }
  static Reboot(req: RebootRequest, ...options: fm.fetchOption[]): Promise<RebootResponse> {
    return fm.fetchReq<RebootRequest, RebootResponse>("POST", `/machine.MachineService/Reboot`, req, ...options)
  }
  static Restart(req: RestartRequest, ...options: fm.fetchOption[]): Promise<RestartResponse> {
    return fm.fetchReq<RestartRequest, RestartResponse>("POST", `/machine.MachineService/Restart`, req, ...options)
  }
  static Rollback(req: RollbackRequest, ...options: fm.fetchOption[]): Promise<RollbackResponse> {
    return fm.fetchReq<RollbackRequest, RollbackResponse>("POST", `/machine.MachineService/Rollback`, req, ...options)
  }
  static Reset(req: ResetRequest, ...options: fm.fetchOption[]): Promise<ResetResponse> {
    return fm.fetchReq<ResetRequest, ResetResponse>("POST", `/machine.MachineService/Reset`, req, ...options)
  }
  static ServiceList(req: GoogleProtobufEmpty.Empty, ...options: fm.fetchOption[]): Promise<ServiceListResponse> {
    return fm.fetchReq<GoogleProtobufEmpty.Empty, ServiceListResponse>("POST", `/machine.MachineService/ServiceList`, req, ...options)
  }
  static ServiceRestart(req: ServiceRestartRequest, ...options: fm.fetchOption[]): Promise<ServiceRestartResponse> {
    return fm.fetchReq<ServiceRestartRequest, ServiceRestartResponse>("POST", `/machine.MachineService/ServiceRestart`, req, ...options)
  }
  static ServiceStart(req: ServiceStartRequest, ...options: fm.fetchOption[]): Promise<ServiceStartResponse> {
    return fm.fetchReq<ServiceStartRequest, ServiceStartResponse>("POST", `/machine.MachineService/ServiceStart`, req, ...options)
  }
  static ServiceStop(req: ServiceStopRequest, ...options: fm.fetchOption[]): Promise<ServiceStopResponse> {
    return fm.fetchReq<ServiceStopRequest, ServiceStopResponse>("POST", `/machine.MachineService/ServiceStop`, req, ...options)
  }
  static Shutdown(req: ShutdownRequest, ...options: fm.fetchOption[]): Promise<ShutdownResponse> {
    return fm.fetchReq<ShutdownRequest, ShutdownResponse>("POST", `/machine.MachineService/Shutdown`, req, ...options)
  }
  static Stats(req: StatsRequest, ...options: fm.fetchOption[]): Promise<StatsResponse> {
    return fm.fetchReq<StatsRequest, StatsResponse>("POST", `/machine.MachineService/Stats`, req, ...options)
  }
  static SystemStat(req: GoogleProtobufEmpty.Empty, ...options: fm.fetchOption[]): Promise<SystemStatResponse> {
    return fm.fetchReq<GoogleProtobufEmpty.Empty, SystemStatResponse>("POST", `/machine.MachineService/SystemStat`, req, ...options)
  }
  static Upgrade(req: UpgradeRequest, ...options: fm.fetchOption[]): Promise<UpgradeResponse> {
    return fm.fetchReq<UpgradeRequest, UpgradeResponse>("POST", `/machine.MachineService/Upgrade`, req, ...options)
  }
  static Version(req: GoogleProtobufEmpty.Empty, ...options: fm.fetchOption[]): Promise<VersionResponse> {
    return fm.fetchReq<GoogleProtobufEmpty.Empty, VersionResponse>("POST", `/machine.MachineService/Version`, req, ...options)
  }
  static GenerateClientConfiguration(req: GenerateClientConfigurationRequest, ...options: fm.fetchOption[]): Promise<GenerateClientConfigurationResponse> {
    return fm.fetchReq<GenerateClientConfigurationRequest, GenerateClientConfigurationResponse>("POST", `/machine.MachineService/GenerateClientConfiguration`, req, ...options)
  }
  static PacketCapture(req: PacketCaptureRequest, entityNotifier?: fm.NotifyStreamEntityArrival<CommonCommon.Data>, ...options: fm.fetchOption[]): Promise<void> {
    return fm.fetchStreamingRequest<PacketCaptureRequest, CommonCommon.Data>("POST", `/machine.MachineService/PacketCapture`, req, entityNotifier, ...options)
  }
  static Netstat(req: NetstatRequest, ...options: fm.fetchOption[]): Promise<NetstatResponse> {
    return fm.fetchReq<NetstatRequest, NetstatResponse>("POST", `/machine.MachineService/Netstat`, req, ...options)
  }
  static MetaWrite(req: MetaWriteRequest, ...options: fm.fetchOption[]): Promise<MetaWriteResponse> {
    return fm.fetchReq<MetaWriteRequest, MetaWriteResponse>("POST", `/machine.MachineService/MetaWrite`, req, ...options)
  }
  static MetaDelete(req: MetaDeleteRequest, ...options: fm.fetchOption[]): Promise<MetaDeleteResponse> {
    return fm.fetchReq<MetaDeleteRequest, MetaDeleteResponse>("POST", `/machine.MachineService/MetaDelete`, req, ...options)
  }
  static ImageList(req: ImageListRequest, entityNotifier?: fm.NotifyStreamEntityArrival<ImageListResponse>, ...options: fm.fetchOption[]): Promise<void> {
    return fm.fetchStreamingRequest<ImageListRequest, ImageListResponse>("POST", `/machine.MachineService/ImageList`, req, entityNotifier, ...options)
  }
  static ImagePull(req: ImagePullRequest, ...options: fm.fetchOption[]): Promise<ImagePullResponse> {
    return fm.fetchReq<ImagePullRequest, ImagePullResponse>("POST", `/machine.MachineService/ImagePull`, req, ...options)
  }
}