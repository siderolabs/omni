/* eslint-disable */
// @ts-nocheck
/*
* This file is a generated Typescript file for GRPC Gateway, DO NOT MODIFY
*/

import * as GoogleProtobufTimestamp from "../../google/protobuf/timestamp.pb"
import * as SpecsOmni from "./omni.pb"

export enum MachineRequestStatusSpecStage {
  UNKNOWN = 0,
  PROVISIONING = 1,
  PROVISIONED = 2,
  FAILED = 3,
}

export enum InfraMachineSpecMachinePowerState {
  POWER_STATE_OFF = 0,
  POWER_STATE_ON = 1,
}

export enum InfraMachineStatusSpecMachinePowerState {
  POWER_STATE_UNKNOWN = 0,
  POWER_STATE_OFF = 1,
  POWER_STATE_ON = 2,
}

export type MachineRequestSpec = {
  talos_version?: string
  overlay?: SpecsOmni.Overlay
  extensions?: string[]
  kernel_args?: string[]
  meta_values?: SpecsOmni.MetaValue[]
  provider_data?: string
  grpc_tunnel?: SpecsOmni.GrpcTunnelMode
}

export type MachineRequestStatusSpec = {
  id?: string
  stage?: MachineRequestStatusSpecStage
  error?: string
  status?: string
}

export type InfraMachineSpec = {
  preferred_power_state?: InfraMachineSpecMachinePowerState
  acceptance_status?: SpecsOmni.InfraMachineConfigSpecAcceptanceStatus
  cluster_talos_version?: string
  extensions?: string[]
  wipe_id?: string
  extra_kernel_args?: string
  requested_reboot_id?: string
  cordoned?: boolean
  install_event_id?: string
}

export type InfraMachineStateSpec = {
  installed?: boolean
}

export type InfraMachineStatusSpec = {
  power_state?: InfraMachineStatusSpecMachinePowerState
  ready_to_use?: boolean
  last_reboot_id?: string
  last_reboot_timestamp?: GoogleProtobufTimestamp.Timestamp
  installed?: boolean
}

export type InfraProviderStatusSpec = {
  schema?: string
  name?: string
  description?: string
  icon?: string
}

export type InfraProviderHealthStatusSpec = {
  last_heartbeat_timestamp?: GoogleProtobufTimestamp.Timestamp
  error?: string
}