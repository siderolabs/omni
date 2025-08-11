/* eslint-disable */
// @ts-nocheck
/*
* This file is a generated Typescript file for GRPC Gateway, DO NOT MODIFY
*/

import * as GoogleProtobufTimestamp from "../../google/protobuf/timestamp.pb"

export enum NodeUniqueTokenStatusSpecState {
  UNKNOWN = 0,
  PERSISTENT = 1,
  EPHEMERAL = 2,
  NONE = 3,
  UNSUPPORTED = 4,
}

export enum JoinTokenStatusSpecState {
  UNKNOWN = 0,
  ACTIVE = 1,
  REVOKED = 2,
  EXPIRED = 3,
}

export type SiderolinkConfigSpec = {
  private_key?: string
  public_key?: string
  wireguard_endpoint?: string
  subnet?: string
  server_address?: string
  initial_join_token?: string
  advertised_endpoint?: string
}

export type SiderolinkSpec = {
  node_subnet?: string
  node_public_key?: string
  last_endpoint?: string
  connected?: boolean
  virtual_addrport?: string
  remote_addr?: string
  node_unique_token?: string
}

export type LinkStatusSpec = {
  node_subnet?: string
  node_public_key?: string
  virtual_addrport?: string
  link_id?: string
}

export type SiderolinkCounterSpec = {
  bytes_received?: string
  bytes_sent?: string
  last_alive?: GoogleProtobufTimestamp.Timestamp
}

export type ConnectionParamsSpec = {
  args?: string
  api_endpoint?: string
  wireguard_endpoint?: string
  join_token?: string
  use_grpc_tunnel?: boolean
  events_port?: number
  logs_port?: number
}

export type PendingMachineStatusSpec = {
  token?: string
  talos_installed?: boolean
}

export type JoinConfig = {
  kernel_args?: string[]
  config?: string
}

export type SiderolinkAPIConfigSpec = {
  machine_api_advertised_url?: string
  wireguard_advertised_endpoint?: string
  enforce_grpc_tunnel?: boolean
  events_port?: number
  logs_port?: number
}

export type ProviderJoinConfigSpec = {
  config?: JoinConfig
  join_token?: string
}

export type MachineJoinConfigSpec = {
  config?: JoinConfig
}

export type NodeUniqueTokenSpec = {
  token?: string
}

export type NodeUniqueTokenStatusSpec = {
  state?: NodeUniqueTokenStatusSpecState
}

export type JoinTokenSpec = {
  expiration_time?: GoogleProtobufTimestamp.Timestamp
  revoked?: boolean
  name?: string
}

export type JoinTokenStatusSpecWarning = {
  machine?: string
  message?: string
}

export type JoinTokenStatusSpec = {
  state?: JoinTokenStatusSpecState
  is_default?: boolean
  use_count?: string
  expiration_time?: GoogleProtobufTimestamp.Timestamp
  name?: string
  warnings?: JoinTokenStatusSpecWarning[]
}

export type JoinTokenUsageSpec = {
  token_id?: string
}

export type DefaultJoinTokenSpec = {
  token_id?: string
}