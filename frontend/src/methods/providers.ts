// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import { ResourceService } from '@/api/grpc'
import type { InfraProviderSpec } from '@/api/omni/specs/infra.pb'
import { withRuntime } from '@/api/options'
import { InfraProviderNamespace, ProviderType, RoleInfraProvider } from '@/api/resources'

import { createServiceAccount } from './user'

export const setupInfraProvider = async (name: string) => {
  await ResourceService.Create<Resource<InfraProviderSpec>>(
    {
      metadata: {
        id: name,
        namespace: InfraProviderNamespace,
        type: ProviderType,
      },
      spec: {},
    },
    withRuntime(Runtime.Omni),
  )

  return await createServiceAccount(name, RoleInfraProvider)
}
