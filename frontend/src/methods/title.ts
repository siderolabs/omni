// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { watchEffect } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { SysVersionSpec } from '@/api/omni/specs/system.pb'
import { EphemeralNamespace, SysVersionID, SysVersionType } from '@/api/resources'
import { useResourceGet } from '@/methods/useResourceGet'

export async function useDocumentTitle() {
  const { data: sysVersion } = useResourceGet<SysVersionSpec>({
    runtime: Runtime.Omni,
    resource: {
      namespace: EphemeralNamespace,
      type: SysVersionType,
      id: SysVersionID,
    },
  })

  watchEffect(() => {
    const instanceName = sysVersion.value?.spec.instance_name

    document.title = instanceName ? `Omni - ${instanceName}` : 'Omni'
  })
}
