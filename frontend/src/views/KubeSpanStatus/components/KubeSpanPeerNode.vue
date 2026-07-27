<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script lang="ts">
import type { Resource } from '@/api/grpc'
import type { PeerStatusSpec } from '@/api/talos/kubespan.pb'

export interface KubeSpanPeerNodeData {
  peer: Resource<PeerStatusSpec>
  isOnline: boolean
  dimmed: boolean
  label: string
}
</script>

<script setup lang="ts">
import { Handle, type NodeProps, Position } from '@vue-flow/core'

const { data } = defineProps<NodeProps<KubeSpanPeerNodeData>>()
</script>

<template>
  <div
    class="flex size-full items-center gap-2 rounded-md border border-naturals-n6 bg-naturals-n2 px-2.5 shadow-lg/40 transition-opacity"
    :class="data.dimmed ? 'opacity-30' : 'opacity-100'"
  >
    <Handle id="left" type="target" :position="Position.Left" class="min-h-0! min-w-0!" />

    <div
      class="size-2 rounded-xs border border-current"
      :class="data.isOnline ? 'bg-current text-green-g1' : 'text-red-r1'"
    ></div>

    <div class="flex min-w-0 grow flex-col gap-1">
      <div class="truncate text-xs/tight font-medium text-naturals-n12">
        {{ data.peer.spec.label }}
      </div>

      <span class="w-max rounded bg-current/20 px-1 py-0.5 text-[0.625rem]/none text-naturals-n11">
        {{ data.label }}
      </span>

      <div class="truncate font-mono text-[0.625rem]/tight text-naturals-n10">
        {{ data.peer.spec.lastUsedEndpoint || data.peer.spec.endpoint || '—' }}
      </div>
    </div>
  </div>
</template>
