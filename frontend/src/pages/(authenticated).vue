<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { Runtime } from '@/api/common/omni.pb'
import type { SysVersionSpec } from '@/api/omni/specs/system.pb'
import { EphemeralNamespace, SysVersionID, SysVersionType } from '@/api/resources'
import { usePostHog } from '@/methods/posthog'
import { useTitle } from '@/methods/title'
import { useResourceGet } from '@/methods/useResourceGet'
import { useUserpilot } from '@/methods/userpilot'

definePage({
  meta: { guard: 'keys' },
})

const { data: sysVersion } = useResourceGet<SysVersionSpec>({
  runtime: Runtime.Omni,
  resource: {
    namespace: EphemeralNamespace,
    type: SysVersionType,
    id: SysVersionID,
  },
})

useTitle(() => sysVersion.value?.spec.instance_name)
useUserpilot()
usePostHog()
</script>

<template>
  <RouterView />
</template>
