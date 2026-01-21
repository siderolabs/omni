// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { Runtime } from '@/api/common/omni.pb'
import { Code } from '@/api/google/rpc/code.pb'
import type { Resource } from '@/api/grpc'
import { ResourceService } from '@/api/grpc'
import type { SysVersionSpec } from '@/api/omni/specs/system.pb'
import { withRuntime } from '@/api/options'
import { EphemeralNamespace, SysVersionID, SysVersionType } from '@/api/resources'

let title: string | undefined = undefined

export const refreshTitle = async () => {
  if (title) {
    document.title = title
    return
  }

  try {
    const sysVersion: Resource<SysVersionSpec> = await ResourceService.Get(
      {
        namespace: EphemeralNamespace,
        type: SysVersionType,
        id: SysVersionID,
      },
      withRuntime(Runtime.Omni),
    )

    if (sysVersion?.spec?.instance_name) {
      title = 'Omni - ' + sysVersion?.spec?.instance_name

      document.title = title
    }
  } catch (e) {
    if (e?.code !== Code.UNAUTHENTICATED) {
      console.log('failed to get sysversion resource', e)
    }
  }
}
