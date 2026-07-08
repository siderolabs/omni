<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Resource } from '@/api/grpc'
import type { MachineStatusLinkSpec } from '@/api/omni/specs/ephemeral.pb'
import TIcon from '@/components/Icon/TIcon.vue'
import { useDerivedMachineStage } from '@/methods/useDerivedMachineStage'
import { cn } from '@/methods/utils'

const { machine } = defineProps<{
  machine: Resource<MachineStatusLinkSpec>
}>()

const { status } = useDerivedMachineStage(() => machine.spec.snapshot)
</script>

<template>
  <span
    v-if="status"
    :class="
      cn(
        'inline-flex items-center gap-1 text-xs',
        status.class,
        { 'opacity-50': machine.spec.tearing_down },
        $attrs.class,
      )
    "
  >
    <TIcon :icon="status.icon" class="size-4" aria-hidden="true" />
    {{ status.name }}
  </span>
</template>
