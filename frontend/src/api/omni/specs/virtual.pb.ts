/* eslint-disable */
// @ts-nocheck
/*
* This file is a generated Typescript file for GRPC Gateway, DO NOT MODIFY
*/
export type CurrentUserSpec = {
  identity?: string
  role?: string
}

export type PermissionsSpec = {
  can_read_clusters?: boolean
  can_create_clusters?: boolean
  can_manage_users?: boolean
  can_read_machines?: boolean
  can_remove_machines?: boolean
  can_read_machine_logs?: boolean
  can_read_machine_config_patches?: boolean
  can_manage_machine_config_patches?: boolean
  can_manage_backup_store?: boolean
  can_access_maintenance_nodes?: boolean
}

export type ClusterPermissionsSpec = {
  can_add_machines?: boolean
  can_remove_machines?: boolean
  can_reboot_machines?: boolean
  can_update_kubernetes?: boolean
  can_download_kubeconfig?: boolean
  can_sync_kubernetes_manifests?: boolean
  can_update_talos?: boolean
  can_download_talosconfig?: boolean
  can_read_config_patches?: boolean
  can_manage_config_patches?: boolean
  can_manage_cluster_features?: boolean
  can_download_support_bundle?: boolean
}

export type LabelsCompletionSpecValues = {
  items?: string[]
}

export type LabelsCompletionSpec = {
  items?: {[key: string]: LabelsCompletionSpecValues}
}