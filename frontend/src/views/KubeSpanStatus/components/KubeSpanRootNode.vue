<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script lang="ts">
import type { Resource } from '@/api/grpc'
import type { ClusterMachineIdentitySpec } from '@/api/omni/specs/omni.pb'

export interface KubeSpanRootNodeData {
  machine: Resource<ClusterMachineIdentitySpec>
  traffic: string
}
</script>

<script setup lang="ts">
import { Handle, type NodeProps, Position } from '@vue-flow/core'

import { LabelControlPlaneRole } from '@/api/resources'

const { dimensions, data } = defineProps<NodeProps<KubeSpanRootNodeData>>()
</script>

<template>
  <div class="w-full rounded-lg border border-border-strong bg-surface-card shadow-lg/40">
    <div
      class="flex items-center gap-1 rounded-[7px] border border-border-accent bg-surface-card px-3"
      :style="{ height: `${dimensions.height}px` }"
    >
      <Handle id="right" type="source" :position="Position.Right" class="min-h-0! min-w-0!" />

      <div class="size-2 shrink-0 rounded-xs bg-status-success-default"></div>
      <span v-if="data.machine" class="truncate text-sm font-medium text-content-emphasis">
        {{ data.machine.spec.nodename }}
      </span>
    </div>

    <div class="flex flex-col gap-2 px-4 py-2 text-xs">
      <div class="flex flex-col gap-1 text-xs">
        <span class="text-content-secondary">Role</span>
        <span class="text-content-emphasis">
          {{
            typeof data.machine.metadata.labels?.[LabelControlPlaneRole] === 'string'
              ? 'Control Plane'
              : 'Worker'
          }}
        </span>
      </div>

      <div class="flex flex-col gap-1 text-xs">
        <span class="text-content-secondary">Incoming / Outgoing traffic</span>
        <span class="text-content-emphasis">
          {{ data.traffic }}
        </span>
      </div>
    </div>
  </div>
</template>
