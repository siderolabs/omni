/* eslint-disable */
// @ts-nocheck
/*
* This file is a generated Typescript file for GRPC Gateway, DO NOT MODIFY
*/

import * as GoogleProtobufAny from "../google/protobuf/any.pb"
import * as GoogleRpcStatus from "../google/rpc/status.pb"

export enum Code {
  FATAL = 0,
  LOCKED = 1,
  CANCELED = 2,
}

export enum ContainerDriver {
  CONTAINERD = 0,
  CRI = 1,
}

export enum ContainerdNamespace {
  NS_UNKNOWN = 0,
  NS_SYSTEM = 1,
  NS_CRI = 2,
}

export type Error = {
  code?: Code
  message?: string
  details?: GoogleProtobufAny.Any[]
}

export type Metadata = {
  hostname?: string
  error?: string
  status?: GoogleRpcStatus.Status
}

export type Data = {
  metadata?: Metadata
  bytes?: Uint8Array
}

export type DataResponse = {
  messages?: Data[]
}

export type Empty = {
  metadata?: Metadata
}

export type EmptyResponse = {
  messages?: Empty[]
}

export type ContainerdInstance = {
  driver?: ContainerDriver
  namespace?: ContainerdNamespace
}

export type URL = {
  full_path?: string
}

export type PEMEncodedCertificateAndKey = {
  crt?: Uint8Array
  key?: Uint8Array
}

export type PEMEncodedKey = {
  key?: Uint8Array
}

export type PEMEncodedCertificate = {
  crt?: Uint8Array
}

export type NetIP = {
  ip?: Uint8Array
}

export type NetIPPort = {
  ip?: Uint8Array
  port?: number
}

export type NetIPPrefix = {
  ip?: Uint8Array
  prefix_length?: number
}