/* eslint-disable */
// @ts-nocheck
/*
* This file is a generated Typescript file for GRPC Gateway, DO NOT MODIFY
*/

import * as fm from "../../fetch.pb"
import * as CosiResourceResource from "../../v1alpha1/resource.pb"

export enum EventType {
  UNKNOWN = 0,
  CREATED = 1,
  UPDATED = 2,
  DESTROYED = 3,
  BOOTSTRAPPED = 4,
}

export enum DependencyGraphResponseNodeType {
  UNKNOWN = 0,
  CONTROLLER = 1,
  RESOURCE = 2,
}

export type Resource = {
  metadata?: CosiResourceResource.Metadata
  spec?: string
}

export type GetRequest = {
  namespace?: string
  type?: string
  id?: string
}

export type GetResponse = {
  body?: string
}

export type ListRequest = {
  namespace?: string
  type?: string
  offset?: number
  limit?: number
  sort_by_field?: string
  sort_descending?: boolean
  search_for?: string[]
}

export type ListResponse = {
  items?: string[]
  total?: number
}

export type Event = {
  resource?: string
  old?: string
  event_type?: EventType
}

export type WatchRequest = {
  namespace?: string
  type?: string
  id?: string
  tail_events?: number
  offset?: number
  limit?: number
  sort_by_field?: string
  sort_descending?: boolean
  search_for?: string[]
}

export type WatchResponse = {
  event?: Event
  total?: number
  sort_field_data?: string
  sort_descending?: boolean
}

export type CreateRequest = {
  resource?: Resource
}

export type CreateResponse = {
}

export type UpdateRequest = {
  currentVersion?: string
  resource?: Resource
}

export type UpdateResponse = {
}

export type DeleteRequest = {
  namespace?: string
  type?: string
  id?: string
}

export type DeleteResponse = {
}

export type ControllersRequest = {
}

export type ControllersResponse = {
  controllers?: string[]
}

export type DependencyGraphRequest = {
  controllers?: string[]
  show_destroy_ready?: boolean
  resources?: string[]
}

export type DependencyGraphResponseNode = {
  id?: string
  label?: string
  type?: DependencyGraphResponseNodeType
  labels?: string[]
  fields?: string[]
}

export type DependencyGraphResponseEdge = {
  id?: string
  source?: string
  target?: string
  edge_type?: number
}

export type DependencyGraphResponse = {
  nodes?: DependencyGraphResponseNode[]
  edges?: DependencyGraphResponseEdge[]
}

export class ResourceService {
  static Get(req: GetRequest, ...options: fm.fetchOption[]): Promise<GetResponse> {
    return fm.fetchReq<GetRequest, GetResponse>("POST", `/omni.resources.ResourceService/Get`, req, ...options)
  }
  static List(req: ListRequest, ...options: fm.fetchOption[]): Promise<ListResponse> {
    return fm.fetchReq<ListRequest, ListResponse>("POST", `/omni.resources.ResourceService/List`, req, ...options)
  }
  static Create(req: CreateRequest, ...options: fm.fetchOption[]): Promise<CreateResponse> {
    return fm.fetchReq<CreateRequest, CreateResponse>("POST", `/omni.resources.ResourceService/Create`, req, ...options)
  }
  static Update(req: UpdateRequest, ...options: fm.fetchOption[]): Promise<UpdateResponse> {
    return fm.fetchReq<UpdateRequest, UpdateResponse>("POST", `/omni.resources.ResourceService/Update`, req, ...options)
  }
  static Delete(req: DeleteRequest, ...options: fm.fetchOption[]): Promise<DeleteResponse> {
    return fm.fetchReq<DeleteRequest, DeleteResponse>("POST", `/omni.resources.ResourceService/Delete`, req, ...options)
  }
  static Teardown(req: DeleteRequest, ...options: fm.fetchOption[]): Promise<DeleteResponse> {
    return fm.fetchReq<DeleteRequest, DeleteResponse>("POST", `/omni.resources.ResourceService/Teardown`, req, ...options)
  }
  static Watch(req: WatchRequest, entityNotifier?: fm.NotifyStreamEntityArrival<WatchResponse>, ...options: fm.fetchOption[]): Promise<void> {
    return fm.fetchStreamingRequest<WatchRequest, WatchResponse>("POST", `/omni.resources.ResourceService/Watch`, req, entityNotifier, ...options)
  }
  static Controllers(req: ControllersRequest, ...options: fm.fetchOption[]): Promise<ControllersResponse> {
    return fm.fetchReq<ControllersRequest, ControllersResponse>("POST", `/omni.resources.ResourceService/Controllers`, req, ...options)
  }
  static DependencyGraph(req: DependencyGraphRequest, ...options: fm.fetchOption[]): Promise<DependencyGraphResponse> {
    return fm.fetchReq<DependencyGraphRequest, DependencyGraphResponse>("POST", `/omni.resources.ResourceService/DependencyGraph`, req, ...options)
  }
}