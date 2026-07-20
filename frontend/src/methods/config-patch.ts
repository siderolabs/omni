// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { Runtime } from '@/api/common/omni.pb'
import { type Resource, ResourceService } from '@/api/grpc'
import type { ConfigPatchSpec } from '@/api/omni/specs/omni.pb'
import { withRuntime } from '@/api/options'
import { ConfigPatchType, DefaultNamespace, LabelDisabled } from '@/api/resources'

export function isConfigPatchDisabled(labels?: Record<string, string>) {
  return !!labels && LabelDisabled in labels
}

export async function setConfigPatchDisabled(id: string, disabled: boolean) {
  const patch = await ResourceService.Get<Resource<ConfigPatchSpec>>(
    {
      namespace: DefaultNamespace,
      type: ConfigPatchType,
      id,
    },
    withRuntime(Runtime.Omni),
  )

  patch.metadata.labels ||= {}

  if (disabled) {
    patch.metadata.labels[LabelDisabled] = ''
  } else {
    delete patch.metadata.labels[LabelDisabled]
  }

  await ResourceService.Update(patch, undefined, withRuntime(Runtime.Omni))
}
