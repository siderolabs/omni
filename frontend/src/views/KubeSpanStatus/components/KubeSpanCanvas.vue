<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useElementBounding } from '@vueuse/core'
import prettyBytes from 'pretty-bytes'
import { computed, ref, useTemplateRef, watch } from 'vue'

import type { Resource } from '@/api/grpc'
import type { ClusterMachineIdentitySpec } from '@/api/omni/specs/omni.pb'
import { LabelControlPlaneRole } from '@/api/resources'
import type { PeerStatusSpec } from '@/api/talos/kubespan.pb'

const { selectedMachine, machineNodenameMap, peers, peerMatches } = defineProps<{
  selectedMachine: Resource<ClusterMachineIdentitySpec>
  machineNodenameMap: Map<string, Resource<ClusterMachineIdentitySpec>>
  peers: Resource<PeerStatusSpec>[]
  peerMatches: Set<string>
}>()

const emit = defineEmits<{
  peerClick: [Resource<PeerStatusSpec>]
}>()

defineExpose({
  zoomIn,
  zoomOut,
  resetView,
})

const cardWidth = 200,
  cardHeight = 32,
  cardX = 0,
  peerWidth = 200,
  peerHeight = 66,
  peerX = 360,
  peerSpacing = 16

const canvasEl = useTemplateRef('canvasEl')
const canvasBounding = useElementBounding(canvasEl)
const viewport = ref({ x: 0, y: 0, k: 1 })
const nodeOverrides = ref<Record<string, { x: number; y: number }>>({})
const panState = ref<{ sx: number; sy: number; vx: number; vy: number } | null>(null)
const isPanning = ref(false)
const cardDrag = ref<{
  key: string
  sx: number
  sy: number
  ox: number
  oy: number
  moved: boolean
} | null>(null)
const draggingKey = ref<string | null>(null)
const suppressClick = ref(false)

const totalTrafficOut = computed(() =>
  peers.reduce((prev, p) => prev + (p.spec.transmitBytes ?? 0), 0),
)
const totalTrafficIn = computed(() =>
  peers.reduce((prev, p) => prev + (p.spec.receiveBytes ?? 0), 0),
)
const totalTraffic = computed(() => totalTrafficIn.value + totalTrafficOut.value)

const edgePaths = computed(() =>
  peers.map((peer, i) => {
    const key = peer.metadata.id!
    const pos = getPeerPos(key, i)

    return {
      key,
      peer,
      d: bezierPath(cardX + cardWidth, 0, pos.x, pos.y + peerHeight / 2),
      stroke: peerColor(peer),
      sw: edgeStrokeWidth(peer),
      online: isOnline(peer),
      dimmed: !peerMatches.has(peer.metadata.id!),
    }
  }),
)

const peerCards = computed(() =>
  peers.map((peer, i) => {
    const key = peer.metadata.id!

    return {
      key,
      peer,
      pos: getPeerPos(key, i),
      basePos: peerBasePos(i),
      dimmed: !peerMatches.has(peer.metadata.id!),
    }
  }),
)

watch([canvasBounding.width, canvasBounding.height], centerViewport, { once: true })

function centerViewport() {
  const { width, height } = canvasBounding
  if (width.value <= 0) return

  viewport.value = {
    x: Math.round((width.value - (peerX + peerWidth)) / 2),
    y: Math.round(height.value / 2),
    k: 1,
  }
}

function isOnline(peer: Resource<PeerStatusSpec>) {
  return peer.spec.state === 'up'
}

function peerColor(peer: Resource<PeerStatusSpec>) {
  return isOnline(peer) ? 'var(--color-green-g1)' : 'var(--color-red-r1)'
}

function edgeStrokeWidth(peer: Resource<PeerStatusSpec>) {
  const traffic = (peer.spec.receiveBytes ?? 0) + (peer.spec.transmitBytes ?? 0)
  const trafficPct = totalTraffic.value ? traffic / totalTraffic.value : 0

  return 1.5 + trafficPct * 3
}

function peerBasePos(index: number) {
  const n = peers.length
  const totalH = n * peerHeight + (n - 1) * peerSpacing
  return { x: peerX, y: -totalH / 2 + index * (peerHeight + peerSpacing) }
}

function bezierPath(x1: number, y1: number, x2: number, y2: number) {
  const dx = Math.max(50, (x2 - x1) * 0.55)
  return `M ${x1} ${y1} C ${x1 + dx} ${y1} ${x2 - dx} ${y2} ${x2} ${y2}`
}

function getPeerPos(key: string, index: number) {
  return nodeOverrides.value[key] ?? peerBasePos(index)
}

function onPeerClick(peer: Resource<PeerStatusSpec>) {
  if (suppressClick.value) return

  emit('peerClick', peer)
}

function onCanvasMouseDown(e: MouseEvent) {
  if ((e.target as HTMLElement).closest('[data-card]')) return
  e.preventDefault()
  panState.value = { sx: e.clientX, sy: e.clientY, vx: viewport.value.x, vy: viewport.value.y }
  isPanning.value = true
}

function onCanvasMouseMove(e: MouseEvent) {
  if (cardDrag.value) {
    const d = cardDrag.value
    const dx = e.clientX - d.sx
    const dy = e.clientY - d.sy
    if (!d.moved && Math.hypot(dx, dy) > 5) {
      d.moved = true
      draggingKey.value = d.key
    }
    if (!d.moved) return
    nodeOverrides.value = {
      ...nodeOverrides.value,
      [d.key]: { x: d.ox + dx / viewport.value.k, y: d.oy + dy / viewport.value.k },
    }
    return
  }
  if (!panState.value) return
  viewport.value = {
    ...viewport.value,
    x: panState.value.vx + (e.clientX - panState.value.sx),
    y: panState.value.vy + (e.clientY - panState.value.sy),
  }
}

function resolveCollisions(anchorKey: string) {
  const GUARD = 12
  const positions = peerCards.value.map((c) => ({
    key: c.key,
    x: nodeOverrides.value[c.key]?.x ?? c.basePos.x,
    y: nodeOverrides.value[c.key]?.y ?? c.basePos.y,
  }))

  let collided = true
  let safety = 0
  while (collided && safety < 80) {
    collided = false
    safety++
    for (let i = 0; i < positions.length; i++) {
      for (let j = i + 1; j < positions.length; j++) {
        const a = positions[i]
        const b = positions[j]
        const dx = b.x + peerWidth / 2 - (a.x + peerWidth / 2)
        const dy = b.y + peerHeight / 2 - (a.y + peerHeight / 2)
        const ovX = peerWidth + GUARD - Math.abs(dx)
        const ovY = peerHeight + GUARD - Math.abs(dy)
        if (ovX > 0 && ovY > 0) {
          collided = true
          const aShare = a.key === anchorKey ? 0 : b.key === anchorKey ? 1 : 0.5
          const bShare = 1 - aShare
          if (ovY <= ovX) {
            const sign = dy === 0 ? (Math.random() < 0.5 ? -1 : 1) : Math.sign(dy)
            a.y -= sign * ovY * aShare
            b.y += sign * ovY * bShare
          } else {
            const sign = dx === 0 ? (Math.random() < 0.5 ? -1 : 1) : Math.sign(dx)
            a.x -= sign * ovX * aShare
            b.x += sign * ovX * bShare
          }
        }
      }
    }
  }

  const next = { ...nodeOverrides.value }
  for (const p of positions) next[p.key] = { x: p.x, y: p.y }
  nodeOverrides.value = next
}

function onCanvasMouseUp() {
  if (cardDrag.value?.moved) {
    resolveCollisions(cardDrag.value.key)
    suppressClick.value = true
    setTimeout(() => {
      suppressClick.value = false
    }, 0)
  }
  panState.value = null
  cardDrag.value = null
  isPanning.value = false
  draggingKey.value = null
}

function onCanvasWheel(e: WheelEvent) {
  e.preventDefault()

  const factor = Math.pow(1.0015, -e.deltaY)
  const x = e.clientX - canvasBounding.left.value
  const y = e.clientY - canvasBounding.top.value

  zoomAround(factor, x, y)
}

function onCardMouseDown(e: MouseEvent, key: string, baseX: number, baseY: number) {
  e.stopPropagation()
  e.preventDefault()

  const o = nodeOverrides.value[key]
  cardDrag.value = {
    key,
    sx: e.clientX,
    sy: e.clientY,
    ox: o ? o.x : baseX,
    oy: o ? o.y : baseY,
    moved: false,
  }
}

function zoomAround(factor: number, x: number, y: number) {
  const k = Math.max(0.2, Math.min(3, viewport.value.k * factor))

  viewport.value = {
    x: x - (x - viewport.value.x) * (k / viewport.value.k),
    y: y - (y - viewport.value.y) * (k / viewport.value.k),
    k,
  }
}

function zoomIn() {
  const x = canvasBounding.width.value / 2
  const y = canvasBounding.height.value / 2

  zoomAround(1.25, x, y)
}

function zoomOut() {
  const x = canvasBounding.width.value / 2
  const y = canvasBounding.height.value / 2

  zoomAround(0.75, x, y)
}

function resetView() {
  nodeOverrides.value = {}
  centerViewport()
}
</script>

<template>
  <div
    ref="canvasEl"
    class="relative min-h-100 flex-1 overflow-hidden rounded-sm border border-naturals-n4 bg-naturals-n0 bg-[radial-gradient(var(--color-naturals-n4)_1px,transparent_1px)] select-none"
    :class="isPanning ? 'cursor-grabbing' : 'cursor-grab'"
    :style="{
      backgroundSize: `${22 * viewport.k}px ${22 * viewport.k}px`,
      backgroundPosition: `${viewport.x}px ${viewport.y}px`,
    }"
    @wheel="onCanvasWheel"
    @mousedown="onCanvasMouseDown"
    @mousemove="onCanvasMouseMove"
    @mouseup="onCanvasMouseUp"
    @mouseleave="onCanvasMouseUp"
  >
    <svg
      v-for="e in edgePaths"
      :key="e.key"
      width="100%"
      height="100%"
      class="pointer-events-none absolute inset-0"
      :class="draggingKey === e.key ? 'z-10' : ''"
    >
      <g :transform="`translate(${viewport.x},${viewport.y}) scale(${viewport.k})`">
        <path
          :d="e.d"
          :stroke="e.stroke"
          :stroke-width="e.sw"
          fill="none"
          stroke-linecap="round"
          :opacity="e.dimmed ? 0.35 : 0.7"
        />
      </g>
    </svg>

    <div
      v-if="peers.length"
      class="pointer-events-none absolute inset-0 origin-top-left"
      :style="{
        transform: `translate(${viewport.x}px,${viewport.y}px) scale(${viewport.k})`,
      }"
    >
      <div
        class="absolute rounded-lg border border-naturals-n6 bg-naturals-n2 shadow-lg/40"
        :style="{
          left: `${cardX}px`,
          top: `${-cardHeight / 2}px`,
          width: `${cardWidth}px`,
        }"
      >
        <div
          data-card
          class="flex items-center gap-1 rounded-[7px] border border-primary-p3 bg-naturals-n2 px-3"
          :style="{
            height: `${cardHeight}px`,
          }"
        >
          <div class="size-2 shrink-0 rounded-xs bg-green-g1"></div>
          <span v-if="selectedMachine" class="truncate text-sm font-medium text-naturals-n14">
            {{ selectedMachine.spec.nodename }}
          </span>
        </div>

        <div class="flex flex-col gap-2 px-4 py-2 text-xs">
          <div class="flex flex-col gap-1 text-xs">
            <span class="text-naturals-n11">Role</span>
            <span class="text-naturals-n14">
              {{
                typeof selectedMachine.metadata.labels?.[LabelControlPlaneRole] === 'string'
                  ? 'Control Plane'
                  : 'Worker'
              }}
            </span>
          </div>

          <div class="flex flex-col gap-1 text-xs">
            <span class="text-naturals-n11">Incoming / Outgoing traffic</span>
            <span class="text-naturals-n14">
              {{ `${prettyBytes(totalTrafficIn)} / ${prettyBytes(totalTrafficOut)}` }}
            </span>
          </div>
        </div>
      </div>

      <div
        v-for="c in peerCards"
        :key="`card-${c.key}`"
        data-card
        class="pointer-events-auto absolute flex items-center gap-2 rounded-md border border-naturals-n6 bg-naturals-n2 px-2.5"
        :class="[
          draggingKey === c.key
            ? 'z-10 cursor-grabbing shadow-lg/80'
            : 'cursor-grab shadow-lg/40 transition-all',
          c.dimmed ? 'opacity-30' : 'opacity-100',
        ]"
        :style="{
          left: `${c.pos.x}px`,
          top: `${c.pos.y}px`,
          width: `${peerWidth}px`,
          height: `${peerHeight}px`,
        }"
        @mousedown.stop.prevent="onCardMouseDown($event, c.key, c.basePos.x, c.basePos.y)"
        @click="onPeerClick(c.peer)"
      >
        <div class="size-2 rounded-xs" :style="{ backgroundColor: peerColor(c.peer) }"></div>

        <div class="flex min-w-0 grow flex-col gap-1">
          <div class="truncate text-xs/tight font-medium text-naturals-n12">
            {{ c.peer.spec.label }}
          </div>

          <span
            class="w-max rounded bg-current/20 px-1 py-0.5 text-[0.625rem]/none text-naturals-n11"
          >
            {{
              typeof machineNodenameMap.get(c.peer.spec.label!)?.metadata.labels?.[
                LabelControlPlaneRole
              ] === 'string'
                ? 'Control Plane'
                : 'Worker'
            }}
          </span>

          <div class="truncate font-mono text-[0.625rem]/tight text-naturals-n10">
            {{ c.peer.spec.lastUsedEndpoint || c.peer.spec.endpoint || '—' }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
