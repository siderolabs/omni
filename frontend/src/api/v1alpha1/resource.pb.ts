/* eslint-disable */
// @ts-nocheck
/*
* This file is a generated Typescript file for GRPC Gateway, DO NOT MODIFY
*/

import * as GoogleProtobufTimestamp from "../google/protobuf/timestamp.pb"

export enum LabelTermOperation {
  EXISTS = 0,
  EQUAL = 1,
  NOT_EXISTS = 2,
  IN = 3,
  LT = 4,
  LTE = 5,
  LT_NUMERIC = 6,
  LTE_NUMERIC = 7,
}

export type Metadata = {
  namespace?: string
  type?: string
  id?: string
  version?: string
  owner?: string
  phase?: string
  created?: GoogleProtobufTimestamp.Timestamp
  updated?: GoogleProtobufTimestamp.Timestamp
  finalizers?: string[]
  annotations?: {[key: string]: string}
  labels?: {[key: string]: string}
}

export type Spec = {
  proto_spec?: Uint8Array
  yaml_spec?: string
}

export type Resource = {
  metadata?: Metadata
  spec?: Spec
}

export type LabelTerm = {
  key?: string
  op?: LabelTermOperation
  value?: string[]
  invert?: boolean
}

export type LabelQuery = {
  terms?: LabelTerm[]
}

export type IDQuery = {
  regexp?: string
}