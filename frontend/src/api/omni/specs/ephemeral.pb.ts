/* eslint-disable */
// @ts-nocheck
/*
* This file is a generated Typescript file for GRPC Gateway, DO NOT MODIFY
*/

import * as SpecsOmni from "./omni.pb"
import * as SpecsSiderolink from "./siderolink.pb"
export type MachineStatusLinkSpec = {
  message_status?: SpecsOmni.MachineStatusSpec
  siderolink_counter?: SpecsSiderolink.SiderolinkCounterSpec
  machine_created_at?: string
  tearing_down?: boolean
}