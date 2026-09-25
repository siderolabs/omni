<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { Runtime } from '@/api/common/omni.pb'
import { type Resource, ResourceService } from '@/api/grpc'
import type { PermissionsSpec } from '@/api/omni/specs/virtual.pb'
import { withRuntime } from '@/api/options'
import { PermissionsID, PermissionsType, VirtualNamespace } from '@/api/resources'
import { useTitle } from '@/methods/title'

definePage({
  name: 'Internals',
  redirect: {
    name: 'InternalsOmniResources',
  },
  // These pages expose raw resources, including sensitive ones, so access is decided by the backend.
  beforeEnter: async () => {
    try {
      const permissions: Resource<PermissionsSpec> = await ResourceService.Get(
        { namespace: VirtualNamespace, type: PermissionsType, id: PermissionsID },
        withRuntime(Runtime.Omni),
      )

      if (permissions.spec.can_access_internals) return
    } catch {
      // Fall through to forbidden
    }

    return { path: '/forbidden' }
  },
})

useTitle('Internals')
</script>

<template>
  <RouterView />
</template>
