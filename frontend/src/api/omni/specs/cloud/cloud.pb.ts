/* eslint-disable */
// @ts-nocheck
/*
* This file is a generated Typescript file for GRPC Gateway, DO NOT MODIFY
*/

export enum MachineRequestStatusSpecStage {
  UNKNOWN = 0,
  PROVISIONING = 1,
  PROVISIONED = 2,
  FAILED = 3,
}

export type MachineRequestSpec = {
  talos_version?: string
  schematic_id?: string
  provider_data?: string
}

export type MachineRequestStatusSpec = {
  id?: string
  stage?: MachineRequestStatusSpecStage
}