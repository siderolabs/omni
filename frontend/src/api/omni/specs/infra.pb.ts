/* eslint-disable */
// @ts-nocheck
/*
* This file is a generated Typescript file for GRPC Gateway, DO NOT MODIFY
*/

import * as SpecsOmni from "./omni.pb"

export enum MachineRequestStatusSpecStage {
  UNKNOWN = 0,
  PROVISIONING = 1,
  PROVISIONED = 2,
  FAILED = 3,
}

export type MachineRequestSpec = {
  talos_version?: string
  overlay?: SpecsOmni.Overlay
  extensions?: string[]
  kernel_args?: string[]
  meta_values?: SpecsOmni.MetaValue[]
  provider_data?: string
}

export type MachineRequestStatusSpec = {
  id?: string
  stage?: MachineRequestStatusSpecStage
  error?: string
}

export type InfraProviderStatusSpec = {
  schema?: string
  name?: string
  description?: string
  icon?: string
}