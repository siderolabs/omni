/* eslint-disable */
// @ts-nocheck
/*
* This file is a generated Typescript file for GRPC Gateway, DO NOT MODIFY
*/

import * as fm from "../../fetch.pb"
export type Context = {
  name?: string
  cluster?: string
}

export type ListContextsRequest = {
}

export type ListContextsResponse = {
  current?: string
  contexts?: Context[]
}

export class ContextService {
  static List(req: ListContextsRequest, ...options: fm.fetchOption[]): Promise<ListContextsResponse> {
    return fm.fetchReq<ListContextsRequest, ListContextsResponse>("POST", `/config.ContextService/List`, req, ...options)
  }
}