/* eslint-disable */
// @ts-nocheck
/*
* This file is a generated Typescript file for GRPC Gateway, DO NOT MODIFY
*/

import * as CommonCommon from "../../common/common.pb"
import * as fm from "../../fetch.pb"
import * as GoogleProtobufDuration from "../../google/protobuf/duration.pb"
import * as GoogleProtobufEmpty from "../../google/protobuf/empty.pb"
import * as GoogleProtobufTimestamp from "../../google/protobuf/timestamp.pb"

export enum KubernetesSyncManifestResponseResponseType {
  UNKNOWN = 0,
  MANIFEST = 1,
  ROLLOUT = 2,
}

export type KubeconfigResponse = {
  kubeconfig?: Uint8Array
}

export type TalosconfigResponse = {
  talosconfig?: Uint8Array
}

export type OmniconfigResponse = {
  omniconfig?: Uint8Array
}

export type MachineLogsRequest = {
  machine_id?: string
  follow?: boolean
  tail_lines?: number
}

export type ValidateConfigRequest = {
  config?: string
}

export type TalosconfigRequest = {
  raw?: boolean
  break_glass?: boolean
}

export type CreateServiceAccountRequest = {
  armored_pgp_public_key?: string
  use_user_role?: boolean
  role?: string
  name?: string
}

export type CreateServiceAccountResponse = {
  public_key_id?: string
}

export type RenewServiceAccountRequest = {
  name?: string
  armored_pgp_public_key?: string
}

export type RenewServiceAccountResponse = {
  public_key_id?: string
}

export type DestroyServiceAccountRequest = {
  name?: string
}

export type ListServiceAccountsResponseServiceAccountPgpPublicKey = {
  id?: string
  armored?: string
  expiration?: GoogleProtobufTimestamp.Timestamp
}

export type ListServiceAccountsResponseServiceAccount = {
  name?: string
  pgp_public_keys?: ListServiceAccountsResponseServiceAccountPgpPublicKey[]
  role?: string
}

export type ListServiceAccountsResponse = {
  service_accounts?: ListServiceAccountsResponseServiceAccount[]
}

export type KubeconfigRequest = {
  service_account?: boolean
  service_account_ttl?: GoogleProtobufDuration.Duration
  service_account_user?: string
  service_account_groups?: string[]
  grant_type?: string
  break_glass?: boolean
}

export type KubernetesUpgradePreChecksRequest = {
  new_version?: string
}

export type KubernetesUpgradePreChecksResponse = {
  ok?: boolean
  reason?: string
}

export type KubernetesSyncManifestRequest = {
  dry_run?: boolean
}

export type KubernetesSyncManifestResponse = {
  response_type?: KubernetesSyncManifestResponseResponseType
  path?: string
  object?: Uint8Array
  diff?: string
  skipped?: boolean
}

export type CreateSchematicRequest = {
  extensions?: string[]
  extra_kernel_args?: string[]
  meta_values?: {[key: number]: string}
  talos_version?: string
  media_id?: string
  secure_boot?: boolean
}

export type CreateSchematicResponse = {
  schematic_id?: string
  pxe_url?: string
}

export type GetSupportBundleRequest = {
  cluster?: string
}

export type GetSupportBundleResponseProgress = {
  source?: string
  error?: string
  state?: string
  total?: number
  value?: number
}

export type GetSupportBundleResponse = {
  progress?: GetSupportBundleResponseProgress
  bundle_data?: Uint8Array
}

export class ManagementService {
  static Kubeconfig(req: KubeconfigRequest, ...options: fm.fetchOption[]): Promise<KubeconfigResponse> {
    return fm.fetchReq<KubeconfigRequest, KubeconfigResponse>("POST", `/management.ManagementService/Kubeconfig`, req, ...options)
  }
  static Talosconfig(req: TalosconfigRequest, ...options: fm.fetchOption[]): Promise<TalosconfigResponse> {
    return fm.fetchReq<TalosconfigRequest, TalosconfigResponse>("POST", `/management.ManagementService/Talosconfig`, req, ...options)
  }
  static Omniconfig(req: GoogleProtobufEmpty.Empty, ...options: fm.fetchOption[]): Promise<OmniconfigResponse> {
    return fm.fetchReq<GoogleProtobufEmpty.Empty, OmniconfigResponse>("POST", `/management.ManagementService/Omniconfig`, req, ...options)
  }
  static MachineLogs(req: MachineLogsRequest, entityNotifier?: fm.NotifyStreamEntityArrival<CommonCommon.Data>, ...options: fm.fetchOption[]): Promise<void> {
    return fm.fetchStreamingRequest<MachineLogsRequest, CommonCommon.Data>("POST", `/management.ManagementService/MachineLogs`, req, entityNotifier, ...options)
  }
  static ValidateConfig(req: ValidateConfigRequest, ...options: fm.fetchOption[]): Promise<GoogleProtobufEmpty.Empty> {
    return fm.fetchReq<ValidateConfigRequest, GoogleProtobufEmpty.Empty>("POST", `/management.ManagementService/ValidateConfig`, req, ...options)
  }
  static CreateServiceAccount(req: CreateServiceAccountRequest, ...options: fm.fetchOption[]): Promise<CreateServiceAccountResponse> {
    return fm.fetchReq<CreateServiceAccountRequest, CreateServiceAccountResponse>("POST", `/management.ManagementService/CreateServiceAccount`, req, ...options)
  }
  static RenewServiceAccount(req: RenewServiceAccountRequest, ...options: fm.fetchOption[]): Promise<RenewServiceAccountResponse> {
    return fm.fetchReq<RenewServiceAccountRequest, RenewServiceAccountResponse>("POST", `/management.ManagementService/RenewServiceAccount`, req, ...options)
  }
  static ListServiceAccounts(req: GoogleProtobufEmpty.Empty, ...options: fm.fetchOption[]): Promise<ListServiceAccountsResponse> {
    return fm.fetchReq<GoogleProtobufEmpty.Empty, ListServiceAccountsResponse>("POST", `/management.ManagementService/ListServiceAccounts`, req, ...options)
  }
  static DestroyServiceAccount(req: DestroyServiceAccountRequest, ...options: fm.fetchOption[]): Promise<GoogleProtobufEmpty.Empty> {
    return fm.fetchReq<DestroyServiceAccountRequest, GoogleProtobufEmpty.Empty>("POST", `/management.ManagementService/DestroyServiceAccount`, req, ...options)
  }
  static KubernetesUpgradePreChecks(req: KubernetesUpgradePreChecksRequest, ...options: fm.fetchOption[]): Promise<KubernetesUpgradePreChecksResponse> {
    return fm.fetchReq<KubernetesUpgradePreChecksRequest, KubernetesUpgradePreChecksResponse>("POST", `/management.ManagementService/KubernetesUpgradePreChecks`, req, ...options)
  }
  static KubernetesSyncManifests(req: KubernetesSyncManifestRequest, entityNotifier?: fm.NotifyStreamEntityArrival<KubernetesSyncManifestResponse>, ...options: fm.fetchOption[]): Promise<void> {
    return fm.fetchStreamingRequest<KubernetesSyncManifestRequest, KubernetesSyncManifestResponse>("POST", `/management.ManagementService/KubernetesSyncManifests`, req, entityNotifier, ...options)
  }
  static CreateSchematic(req: CreateSchematicRequest, ...options: fm.fetchOption[]): Promise<CreateSchematicResponse> {
    return fm.fetchReq<CreateSchematicRequest, CreateSchematicResponse>("POST", `/management.ManagementService/CreateSchematic`, req, ...options)
  }
  static GetSupportBundle(req: GetSupportBundleRequest, entityNotifier?: fm.NotifyStreamEntityArrival<GetSupportBundleResponse>, ...options: fm.fetchOption[]): Promise<void> {
    return fm.fetchStreamingRequest<GetSupportBundleRequest, GetSupportBundleResponse>("POST", `/management.ManagementService/GetSupportBundle`, req, entityNotifier, ...options)
  }
}