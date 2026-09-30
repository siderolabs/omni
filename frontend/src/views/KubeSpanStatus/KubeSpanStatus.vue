<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useRouteQuery } from '@vueuse/router'
import prettyBytes from 'pretty-bytes'
import { computed, useTemplateRef } from 'vue'
import { useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type { ClusterMachineIdentitySpec } from '@/api/omni/specs/omni.pb'
import {
  ClusterMachineIdentityType,
  DefaultNamespace,
  LabelCluster,
  TalosKubeSpanLinkName,
  TalosKubeSpanNamespace,
  TalosKubeSpanPeerStatusType,
  TalosLinkStatusType,
  TalosNetworkNamespace,
} from '@/api/resources'
import type { PeerStatusSpec } from '@/api/talos/kubespan.pb'
import type { LinkStatusSpec } from '@/api/talos/network.pb'
import IconButton from '@/components/Button/IconButton.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import StatsItem from '@/components/Stats/StatsItem.vue'
import StatusPill from '@/components/Status/StatusPill.vue'
import TAlert from '@/components/TAlert.vue'
import TInput from '@/components/TInput/TInput.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'
import KubeSpanCanvas from '@/views/KubeSpanStatus/components/KubeSpanCanvas.vue'
import KubeSpanStatusQuickStart from '@/views/KubeSpanStatus/KubeSpanStatusQuickStart.vue'

const { clusterId, machineId } = defineProps<{
  clusterId: string
  machineId: string
}>()

const searchQuery = useRouteQuery('q', '')
const canvasRef = useTemplateRef('canvasRef')
const router = useRouter()

const {
  data: kubeSpanLink,
  loading: kubeSpanLinkLoading,
  err: kubeSpanLinkErr,
} = useResourceWatch<LinkStatusSpec>(() => ({
  runtime: Runtime.Talos,
  resource: {
    namespace: TalosNetworkNamespace,
    type: TalosLinkStatusType,
    id: TalosKubeSpanLinkName,
  },
  context: {
    cluster: clusterId,
    machine: machineId,
  },
}))

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
}
</script>

<template>
  <PageContainer v-if="kubeSpanLink" class="@container flex h-full flex-col gap-4">
    <div class="flex flex-wrap gap-6">
      <h1 class="shrink-0 text-xl font-medium text-content-emphasis">KubeSpan status</h1>
      <div class="flex flex-wrap gap-6">
        <StatsItem title="Total Nodes" :value="peers.length" icon="server-stack" />
        <StatsItem title="Online" :value="onlineCount" icon="check-in-circle-classic" />
        <StatsItem title="Offline" :value="offlineCount" icon="error" />
      </div>
    </div>

    <div class="flex grow flex-col gap-2 @3xl:flex-row">
      <div class="flex min-w-0 grow flex-col gap-2">
        <div
          class="flex flex-wrap items-center justify-between gap-4 rounded-lg bg-surface-card p-2"
        >
          <div class="flex items-center gap-4 text-xs text-content-secondary">
            <div class="flex items-center gap-1.5">
              <div class="h-0 w-5 border-t-2 border-status-success-default"></div>
              <span>Online</span>
            </div>

            <div class="flex items-center gap-1.5">
              <div class="h-0 w-5 border-t-2 border-dashed border-status-danger-default"></div>
              <span>Offline</span>
            </div>

            <div class="flex items-center gap-1.5">
              <div class="flex items-center gap-0.5">
                <div class="h-0 w-2 rounded-[1px] border-t-2 border-status-success-default"></div>
                <div class="h-0 w-2 rounded-[1px] border-t-4 border-status-success-default"></div>
                <div class="h-0 w-2 rounded-[1px] border-t-8 border-status-success-default"></div>
              </div>
              <span>Traffic Volume</span>
            </div>
          </div>

          <div class="text-xs text-content-muted/55">Drag to pan · scroll to zoom</div>

          <div class="flex overflow-hidden rounded border border-border-default bg-surface-chrome">
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
              aria-label="fit view"
              class="rounded-none"
              @click="canvasRef?.fitView"
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

      <div class="flex shrink-0 flex-col rounded-lg bg-surface-card px-2 @3xl:w-64">
        <div class="px-2 py-3">
          <h3 class="text-sm font-medium text-content-emphasis">Cluster nodes</h3>
        </div>

        <div class="flex items-center justify-between px-2 py-2">
          <span class="text-xs font-medium tracking-wide text-content-emphasis uppercase">
            Name
          </span>
          <span class="text-xs font-medium tracking-wide text-content-emphasis uppercase">
            Status
          </span>
        </div>

        <div class="flex-1 overflow-y-auto">
          <div
            v-for="peer in peers"
            :key="peer.metadata.id"
            class="flex cursor-pointer flex-col gap-1 border-border-strong px-2 py-3 transition-opacity not-last-of-type:border-b hover:bg-surface-raised"
            :class="!peerMatches.has(peer.metadata.id!) && 'opacity-30'"
            @click="onPeerClick(peer)"
          >
            <div class="flex items-center justify-between gap-1">
              <span class="truncate text-sm text-content-default">{{ peer.spec.label }}</span>
              <StatusPill :tone="isOnline(peer) ? 'success' : 'danger'">
                {{ isOnline(peer) ? 'Online' : 'Offline' }}
              </StatusPill>
            </div>

            <div class="flex justify-between gap-3 text-[0.625rem] text-content-secondary">
              <span class="flex items-center gap-1">
                <span class="inline-flex items-center gap-0.5 text-content-emphasis">
                  <TIcon icon="long-arrow-down" class="size-3" />
                  <span class="font-medium tracking-wide uppercase">RX</span>
                </span>

                {{ prettyBytes(peer.spec.receiveBytes ?? 0) }}
              </span>

              <span class="flex items-center gap-1">
                <span class="inline-flex items-center gap-0.5 text-content-emphasis">
                  <span class="font-medium tracking-wide uppercase">TX</span>
                  <TIcon icon="long-arrow-top" class="size-3" />
                </span>

                {{ prettyBytes(peer.spec.transmitBytes ?? 0) }}
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </PageContainer>

  <PageContainer
    v-else-if="kubeSpanLinkLoading"
    class="h-full place-content-center place-items-center"
  >
    <TSpinner class="size-6" />
  </PageContainer>

  <PageContainer v-else-if="kubeSpanLinkErr">
    <TAlert type="error" title="Error">{{ kubeSpanLinkErr }}</TAlert>
  </PageContainer>

  <KubeSpanStatusQuickStart v-else :cluster-id="clusterId" />
</template>
