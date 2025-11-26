// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { useLocalStorage } from '@vueuse/core'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import { ResourceService } from '@/api/grpc'
import type { FeaturesConfigSpec } from '@/api/omni/specs/omni.pb'
import { withRuntime } from '@/api/options'
import { DefaultNamespace, FeaturesConfigID, FeaturesConfigType } from '@/api/resources'
import { useResourceWatch } from '@/methods/useResourceWatch'

const resource = {
  type: FeaturesConfigType,
  namespace: DefaultNamespace,
  id: FeaturesConfigID,
}

export function useFeatures() {
  return useResourceWatch<FeaturesConfigSpec>({
    resource,
    runtime: Runtime.Omni,
  })
}

let cachedFeaturesConfig: Resource<FeaturesConfigSpec> | undefined

export const embeddedDiscoveryServiceFeatureAvailable = async (): Promise<boolean> => {
  const featuresConfig = await getFeaturesConfig()

  return featuresConfig.spec?.embedded_discovery_service ?? false
}

export const auditLogEnabled = async (): Promise<boolean> => {
  const featuresConfig = await getFeaturesConfig()

  return featuresConfig.spec?.audit_log_enabled ?? false
}

const getFeaturesConfig = async (): Promise<Resource<FeaturesConfigSpec>> => {
  if (!cachedFeaturesConfig) {
    cachedFeaturesConfig = await ResourceService.Get(resource, withRuntime(Runtime.Omni))
  }

  return cachedFeaturesConfig
}

export function useInstallationMediaEnabled() {
  return useLocalStorage('_installation_media_enabled', false)
}
