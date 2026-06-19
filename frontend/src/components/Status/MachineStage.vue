<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Resource } from '@/api/grpc'
import type { MachineStatusLinkSpec } from '@/api/omni/specs/ephemeral.pb'
import TIcon from '@/components/Icon/TIcon.vue'
import Tooltip from '@/components/Tooltip/Tooltip.vue'
import { useDerivedMachineStage } from '@/methods/useDerivedMachineStage'

defineOptions({ inheritAttrs: false })

const { machine, iconOnly } = defineProps<{
  machine: Resource<MachineStatusLinkSpec>
  iconOnly?: boolean
}>()

const { status } = useDerivedMachineStage(() => machine.spec.snapshot)
</script>

<template>
  <Tooltip
    v-if="status"
    :disabled="!iconOnly"
    :description="status.name"
    :class="{ 'opacity-50': machine.spec.tearing_down }"
  >
    <div class="flex items-center gap-1" v-bind="$attrs" :class="status.class">
      <!-- Wrapper to prevent tooltip bounce when icon is animated e.g. loading -->
      <span class="size-4 shrink-0">
        <TIcon
          :icon="status.icon"
          class="size-full"
          :aria-label="`status: ${status.name.toLowerCase()}`"
        />
      </span>

      <span v-if="!iconOnly" class="text-xs">{{ status.name }}</span>
    </div>
  </Tooltip>
</template>
