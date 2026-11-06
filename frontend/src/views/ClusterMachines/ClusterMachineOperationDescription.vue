<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useIntervalFn, useNow } from '@vueuse/core'
import { computed } from 'vue'

import type { ClusterMachineStatusSpecOperation } from '@/api/omni/specs/omni.pb'
import { machineOperationDescription } from '@/methods/machineOperation'

const { operation } = defineProps<{
  operation: ClusterMachineStatusSpecOperation
}>()

const now = useNow({ scheduler: (cb) => useIntervalFn(cb, 1000) })

const description = computed(() => machineOperationDescription(operation, now.value))
</script>

<template>
  <p class="max-w-md wrap-break-word whitespace-pre-wrap">{{ description }}</p>
</template>
