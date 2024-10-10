// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

export enum TPodsViewFilterOptions {
  ALL = "All",
  PENDING = "Pending",
  RUNNING = "Running",
  FAILED = "Failed",
  SUCCEEDED = "Succeeded",
  UNKNOWN = "Unknown"
}

export enum NodesViewFilterOptions {
  ALL = "All",
  READY = "Ready",
  UNKNOWN = "Unknown",
  NOT_READY = "Not Ready",
}

export enum TServersServersFilterOptions {
  ALL = "All",
  ON = "On",
  OFF = "Off",
}

export enum TServersStatusesFilterOptions {
  ALL = "All",
  READY = "Ready",
  NOT_ACCEPTED = "Not Accepted",
  FAILED = "Failed",
}

export enum TCommonStatuses {
  ALL = "All",
  READY = "Ready",
  NOT_READY = "Not Ready",
  PENDING = "Pending",
  RUNNING = "Running",
  SUCCEEDED = "Succeeded",
  COMPLETED = "Completed",
  FAILED = "Failed",
  ERROR = 'Error',
  UNKNOWN = "Unknown",
  HEALTH_UNKNOWN = "Health Unknown",
  UNHEALTHY = "Unhealthy",
  HEALTHY = "Healthy",
  ON = 'On',
  OFF = 'Off',
  TRUE = 'True',
  FALSE = 'False',
  UP_TO_DATE = 'Up-to-Date',
  OUTDATED = 'Outdated',
  APPLIED = 'Applied',
  LOADING = "Loading...",
  WAITING = "Waiting",
  STOPPING = "Stopping",
  FINISHED = "Finished",
  ENABLED = "Enabled",
  DISABLED = "Disabled",
  PROVISIONING = "Provisioning",
  PROVISION_FAILED = "Provision Failed",
  PROVISIONED = "Provisioned",
  DEPROVISIONING = "Deprovisioning"
}
