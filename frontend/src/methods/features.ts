// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { useLocalStorage } from '@vueuse/core'
import { Userpilot } from 'userpilot'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import { ResourceService } from '@/api/grpc'
import type { FeaturesConfigSpec } from '@/api/omni/specs/omni.pb'
import type { CurrentUserSpec } from '@/api/omni/specs/virtual.pb'
import { withAbortController, withRuntime } from '@/api/options'
import { DefaultNamespace, FeaturesConfigID, FeaturesConfigType } from '@/api/resources'
import { useWatch } from '@/components/common/Watch/useWatch'

const resource = {
  type: FeaturesConfigType,
  namespace: DefaultNamespace,
  id: FeaturesConfigID,
}

export function useFeatures() {
  return useWatch<FeaturesConfigSpec>({
    resource,
    runtime: Runtime.Omni,
  })
}

let userPilotInitialized = false
let userPilotInitializeAbortController: AbortController | null = null

export const trackingState = useLocalStorage<boolean>('tracking', null)

export const getUserPilotToken = async () => {
  userPilotInitializeAbortController?.abort()

  userPilotInitializeAbortController = new AbortController()

  const featuresConfig = await ResourceService.Get<Resource<FeaturesConfigSpec>>(
    {
      type: FeaturesConfigType,
      namespace: DefaultNamespace,
      id: FeaturesConfigID,
    },
    withRuntime(Runtime.Omni),
    withAbortController(userPilotInitializeAbortController),
  )

  return featuresConfig.spec?.user_pilot_settings?.app_token
}

export const initializeUserPilot = async (user: Resource<CurrentUserSpec>) => {
  if (!trackingState.value) {
    return
  }

  if (!userPilotInitialized) {
    const token = await getUserPilotToken()
    if (!token) {
      userPilotInitialized = true

      return
    }

    Userpilot.initialize(token)

    userPilotInitialized = true
  }

  Userpilot.identify(user.spec.user_id!, {
    role: user.spec.role!,
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

export const getImageFactoryBaseURL = async (): Promise<string> => {
  const featuresConfig = await getFeaturesConfig()

  if (!featuresConfig.spec?.image_factory_base_url) {
    throw new Error('image_factory_base_url is not set in features config')
  }

  return featuresConfig.spec?.image_factory_base_url
}

const getFeaturesConfig = async (): Promise<Resource<FeaturesConfigSpec>> => {
  if (!cachedFeaturesConfig) {
    cachedFeaturesConfig = await ResourceService.Get(resource, withRuntime(Runtime.Omni))
  }

  return cachedFeaturesConfig
}
