// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { RequestError } from '@/api/fetch.pb'
import { Code } from '@/api/google/rpc/code.pb'
import type { Resource } from '@/api/grpc'
import { ResourceService } from '@/api/grpc'
import type { EulaAcceptanceSpec } from '@/api/omni/specs/auth.pb'
import { withRuntime, withTimeout } from '@/api/options'
import { DefaultNamespace, EulaAcceptanceID, EulaAcceptanceType } from '@/api/resources'

// set by the router once the acceptance is confirmed
export const eulaAccepted = ref(false)

// isEulaAccepted reads the acceptance as the signed in user, so it only works after sign in.
export const isEulaAccepted = async (): Promise<boolean> => {
  try {
    await ResourceService.Get<Resource<EulaAcceptanceSpec>>(
      {
        namespace: DefaultNamespace,
        type: EulaAcceptanceType,
        id: EulaAcceptanceID,
      },
      withRuntime(Runtime.Omni),
      withTimeout(10_000),
    )

    return true
  } catch (e) {
    // only an admin can read it, and an unreadable acceptance is nobody else's business
    return (e as RequestError)?.code !== Code.NOT_FOUND
  }
}
