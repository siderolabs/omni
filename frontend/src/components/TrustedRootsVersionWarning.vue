<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { lt, valid } from 'semver'
import { computed } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { SiderolinkAPIConfigSpec } from '@/api/omni/specs/siderolink.pb'
import { APIConfigType, ConfigID, DefaultNamespace } from '@/api/resources'
import TAlert from '@/components/TAlert.vue'
import { useResourceGet } from '@/methods/useResourceGet'

const {
  talosVersion,
  message = "Talos below 1.12 cannot embed it in boot media. If Omni's machine API uses a certificate signed by this CA, machines booting this media cannot join Omni.",
} = defineProps<{ talosVersion?: string; message?: string }>()

const { data: apiConfig } = useResourceGet<SiderolinkAPIConfigSpec>({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: APIConfigType,
    id: ConfigID,
  },
})

// Embedded machine configuration needs Talos 1.12, a version that does not parse counts as the latest one.
const show = computed(
  () =>
    !!apiConfig.value?.spec.trusted_roots &&
    !!talosVersion &&
    !!valid(talosVersion) &&
    lt(talosVersion, '1.12.0'),
)
</script>

<template>
  <TAlert v-if="show" type="warn" title="This Talos version cannot carry Omni's custom CA">
    {{ message }}
  </TAlert>
</template>
