/* eslint-disable */
// @ts-nocheck
/*
* This file is a generated Typescript file for GRPC Gateway, DO NOT MODIFY
*/

import * as fm from "../../fetch.pb"
export type StorageServiceStatfsRequest = {
  path?: string
}

export type StorageServiceStatfsResponse = {
  size?: string
  used?: string
  available?: string
  inodes?: string
  inodes_used?: string
  inodes_free?: string
}

export class StorageService {
  static Statfs(req: StorageServiceStatfsRequest, ...options: fm.fetchOption[]): Promise<StorageServiceStatfsResponse> {
    return fm.fetchReq<StorageServiceStatfsRequest, StorageServiceStatfsResponse>("POST", `/machine.StorageService/Statfs`, req, ...options)
  }
}