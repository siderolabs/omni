/* eslint-disable */
// @ts-nocheck
/*
* This file is a generated Typescript file for GRPC Gateway, DO NOT MODIFY
*/

import * as GoogleProtobufTimestamp from "../../google/protobuf/timestamp.pb"
export type SiderolinkConfigSpec = {
  private_key?: string
  public_key?: string
  wireguard_endpoint?: string
  subnet?: string
  server_address?: string
  join_token?: string
  advertised_endpoint?: string
}

export type SiderolinkSpec = {
  node_subnet?: string
  node_public_key?: string
  last_endpoint?: string
  connected?: boolean
  virtual_addrport?: string
  remote_addr?: string
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
}