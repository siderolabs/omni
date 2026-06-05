<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref, useTemplateRef } from 'vue'
import { useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type { ClusterMachineIdentitySpec } from '@/api/omni/specs/omni.pb'
import {
  ClusterMachineIdentityType,
  DefaultNamespace,
  LabelCluster,
  TalosKubeSpanNamespace,
  TalosKubeSpanPeerStatusType,
} from '@/api/resources'
import type { PeerStatusSpec } from '@/api/talos/kubespan.pb'
import IconButton from '@/components/Button/IconButton.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import StatsItem from '@/components/Stats/StatsItem.vue'
import TInput from '@/components/TInput/TInput.vue'
import { formatBytes } from '@/methods'
import { useResourceWatch } from '@/methods/useResourceWatch'
import KubeSpanCanvas from '@/views/KubeSpanStatus/components/KubeSpanCanvas.vue'

const { clusterId, machineId } = defineProps<{
  clusterId: string
  machineId: string
}>()

const searchQuery = ref('')
const canvasRef = useTemplateRef('canvasRef')
const router = useRouter()

const { data: machines } = useResourceWatch<ClusterMachineIdentitySpec>(() => ({
  resource: {
    namespace: DefaultNamespace,
    type: ClusterMachineIdentityType,
  },
  runtime: Runtime.Omni,
  selectors: [`${LabelCluster}=${clusterId}`],
}))

const { data: peersUnsorted } = useResourceWatch<PeerStatusSpec>(() => ({
  resource: {
    namespace: TalosKubeSpanNamespace,
    type: TalosKubeSpanPeerStatusType,
  },
  runtime: Runtime.Talos,
  context: {
    cluster: clusterId,
    node: machineId,
  },
}))

const peers = computed(() =>
  peersUnsorted.value.toSorted((a, b) => a.spec.label!.localeCompare(b.spec.label!)),
)

const machineIdMap = computed(() => new Map(machines.value.map((m) => [m.metadata.id!, m])))
const machineNodenameMap = computed(() => new Map(machines.value.map((m) => [m.spec.nodename!, m])))
const selectedMachine = computed(() => machineIdMap.value.get(machineId))

const onlineCount = computed(() => peers.value.filter((p) => p.spec.state === 'up').length)
const offlineCount = computed(() => peers.value.length - onlineCount.value)

const peerMatches = computed(() => {
  const q = searchQuery.value.toLowerCase()
  const matches = !q
    ? peers.value
    : peers.value.filter(
        (peer) =>
          peer.spec.label?.toLowerCase().includes(q) ||
          peer.spec.lastUsedEndpoint?.toLowerCase().includes(q) ||
          peer.spec.endpoint?.toLowerCase().includes(q),
      )

  return new Set(matches.map((m) => m.metadata.id!))
})

function isOnline(peer: Resource<PeerStatusSpec>) {
  return peer.spec.state === 'up'
}

function onPeerClick(peer: Resource<PeerStatusSpec>) {
  const target = machineNodenameMap.value.get(peer.spec.label!)

  if (target?.metadata.id) router.push({ params: { machine: target.metadata.id } })

  canvasRef.value?.resetView()
}
</script>

<template>
  <PageContainer class="@container flex h-full flex-col gap-4">
    <div class="flex flex-wrap gap-6">
      <h1 class="shrink-0 text-xl font-medium text-naturals-n14">KubeSpan status</h1>
      <div class="flex flex-wrap gap-6">
        <StatsItem title="Total Nodes" :value="peers.length" icon="server-stack" />
        <StatsItem title="Online" :value="onlineCount" icon="check-in-circle-classic" />
        <StatsItem title="Offline" :value="offlineCount" icon="error" />
      </div>
    </div>

    <div class="flex grow flex-col gap-2 @3xl:flex-row">
      <div class="flex min-w-0 grow flex-col gap-2">
        <div
          class="flex flex-wrap items-center justify-between gap-4 rounded-lg bg-naturals-n2 p-2"
        >
          <div class="flex items-center gap-4 text-xs text-naturals-n11">
            <div class="flex items-center gap-1.5">
              <div class="h-0.5 w-5 rounded bg-green-g1"></div>
              <span>Online</span>
            </div>

            <div class="flex items-center gap-1.5">
              <div class="h-0.5 w-5 rounded bg-red-r1"></div>
              <span>Offline</span>
            </div>

            <div class="flex items-center gap-1.5">
              <div class="flex items-center gap-0.5">
                <div class="h-0 w-2 rounded-[1px] border-t-2 border-green-g1"></div>
                <div class="h-0 w-2 rounded-[1px] border-t-4 border-green-g1"></div>
                <div class="h-0 w-2 rounded-[1px] border-t-8 border-green-g1"></div>
              </div>
              <span>Traffic Volume</span>
            </div>
          </div>

          <div class="text-xs text-naturals-n10/55">
            Drag to pan · drag card to move · scroll to zoom
          </div>

          <div class="flex overflow-hidden rounded border border-naturals-n4 bg-naturals-n2">
            <IconButton
              icon="plus"
              aria-label="zoom in"
              class="rounded-none"
              @click="canvasRef?.zoomIn"
            />
            <IconButton
              icon="minus"
              aria-label="zoom out"
              class="rounded-none"
              @click="canvasRef?.zoomOut"
            />
            <IconButton
              icon="fullscreen"
              aria-label="reset view"
              class="rounded-none"
              @click="canvasRef?.resetView"
            />
          </div>
        </div>

        <TInput v-model="searchQuery" placeholder="Search..." icon="search" />

        <KubeSpanCanvas
          v-if="selectedMachine"
          ref="canvasRef"
          :selected-machine
          :machine-nodename-map
          :peers
          :peer-matches
          @peer-click="onPeerClick"
        />
      </div>

      <div class="flex shrink-0 flex-col rounded-lg bg-naturals-n2 px-2 @3xl:w-64">
        <div class="px-2 py-3">
          <h3 class="text-sm font-medium text-naturals-n14">Cluster nodes</h3>
        </div>

        <div class="flex items-center justify-between px-2 py-2">
          <span class="text-xs font-medium tracking-wide text-naturals-n14 uppercase">Name</span>
          <span class="text-xs font-medium tracking-wide text-naturals-n14 uppercase">Status</span>
        </div>

        <div class="flex-1 overflow-y-auto">
          <div
            v-for="peer in peers"
            :key="peer.metadata.id"
            class="flex cursor-pointer flex-col gap-1 border-naturals-n6 px-2 py-3 transition-opacity not-last-of-type:border-b hover:bg-naturals-n3"
            :class="!peerMatches.has(peer.metadata.id!) && 'opacity-30'"
            @click="onPeerClick(peer)"
          >
            <div class="flex items-center justify-between gap-1">
              <span class="truncate text-sm text-naturals-n13">{{ peer.spec.label }}</span>
              <span
                class="rounded bg-current/20 px-1.5 py-0.5 text-xs uppercase"
                :class="isOnline(peer) ? 'text-green-g1' : 'text-red-r1'"
              >
                {{ isOnline(peer) ? 'Online' : 'Offline' }}
              </span>
            </div>

            <div class="flex justify-between gap-3 text-[0.625rem] text-naturals-n11">
              <span class="flex items-center gap-1">
                <span class="inline-flex items-center gap-0.5 text-naturals-n14">
                  <TIcon icon="long-arrow-down" class="size-3" />
                  <span class="font-medium tracking-wide uppercase">RX</span>
                </span>

                {{ formatBytes(peer.spec.receiveBytes) }}
              </span>

              <span class="flex items-center gap-1">
                <span class="inline-flex items-center gap-0.5 text-naturals-n14">
                  <span class="font-medium tracking-wide uppercase">TX</span>
                  <TIcon icon="long-arrow-top" class="size-3" />
                </span>

                {{ formatBytes(peer.spec.transmitBytes) }}
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </PageContainer>
</template>
