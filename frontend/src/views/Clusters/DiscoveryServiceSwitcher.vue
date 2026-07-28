<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import TButtonGroup from '@/components/Button/TButtonGroup.vue'
import Tooltip from '@/components/Tooltip/Tooltip.vue'

type Props = {
  useEmbedded: boolean
  disablePublic: boolean
  // embeddedAvailable is false when the embedded discovery service is not enabled on this Omni instance
  // or the cluster's Talos version does not support connecting to it (< v1.5.0). The Embedded and Both
  // options are disabled then. It is required so an omitted prop can't silently offer an unavailable option.
  embeddedAvailable: boolean
  // publicConfigurable is true only for clusters created with Talos 1.14+ (the multi-doc DiscoveryServiceConfig).
  // Only there can the embedded service run alongside the public one, so the "Both" option is offered exclusively
  // for such clusters.
  publicConfigurable?: boolean
  disabled?: boolean
}

const { useEmbedded, disablePublic, embeddedAvailable, publicConfigurable, disabled } =
  defineProps<Props>()

// Both booleans are reported together in a single event rather than as two models: a switch between Public and
// Embedded changes both at once, and two separate updates would momentarily describe a state the backend rejects.
const emit = defineEmits<{
  change: [value: { useEmbedded: boolean; disablePublic: boolean }]
}>()

const publicTooltip = 'Use the public discovery service (discovery.talos.dev).'

// The two booleans encode a tri-state. Disabling the public service without the embedded one (no discovery at all)
// is rejected by validation, so it is not reachable from this control.
const mode = computed<string>({
  get: () => {
    if (!useEmbedded) {
      return 'public'
    }

    // On Talos 1.14+ the embedded service is added alongside the public one unless the public one is turned off.
    // On older clusters enabling the embedded service always replaces the public one (disablePublic is unused there).
    if (disablePublic || !publicConfigurable) {
      return 'embedded'
    }

    return 'both'
  },
  set: (value) => {
    switch (value) {
      case 'public':
        emit('change', { useEmbedded: false, disablePublic: false })

        break
      case 'embedded':
        // Turning the public service off is only meaningful on Talos 1.14+; older clusters replace it implicitly.
        emit('change', { useEmbedded: true, disablePublic: publicConfigurable ?? false })

        break
      case 'both':
        emit('change', { useEmbedded: true, disablePublic: false })

        break
    }
  },
})

const options = computed(() => {
  // The availability requirement is only worth showing when the option is disabled, to explain why; when the
  // embedded service is available the requirement is already met. The Talos >= v1.5.0 clause is dropped for
  // Talos 1.14+ clusters (publicConfigurable), which always satisfy it.
  // Tooltips render with whitespace-pre (no auto-wrap), so the longer lines are broken by hand.
  const embeddedRequirement = publicConfigurable
    ? 'Available only when the embedded discovery service is\nenabled on this Omni instance.'
    : 'Available only when the embedded discovery service is\n' +
      "enabled on this Omni instance and the cluster's Talos\n" +
      'version supports connecting to it (>= v1.5.0).'

  const embeddedDescription = "Use Omni's embedded discovery service instead of the public one."

  const bothDescription =
    'Run the embedded discovery service alongside the public one, so\n' +
    'discovery keeps working if one of them becomes unavailable.'

  const embeddedTooltip = embeddedAvailable
    ? embeddedDescription
    : embeddedDescription + '\n\n' + embeddedRequirement

  const bothTooltip = embeddedAvailable
    ? bothDescription
    : bothDescription + '\n\n' + embeddedRequirement

  const opts = [
    {
      label: 'Public',
      value: 'public',
      disabled,
      tooltip: publicTooltip,
    },
    {
      label: 'Embedded',
      value: 'embedded',
      disabled: disabled || !embeddedAvailable,
      tooltip: embeddedTooltip,
    },
  ]

  if (publicConfigurable) {
    opts.push({
      label: 'Both',
      value: 'both',
      disabled: disabled || !embeddedAvailable,
      tooltip: bothTooltip,
    })
  }

  return opts
})
</script>

<template>
  <div class="flex items-center gap-1">
    <Tooltip placement="bottom">
      <template #description>
        <div class="flex max-w-xs flex-col gap-1 p-2">
          <p>
            The discovery service lets the machines in a cluster find one another. Each node
            registers its identity and addresses with the service and uses it to locate the other
            cluster members.
          </p>
          <p>
            It is required for KubeSpan to function and makes cluster membership discovery and
            bootstrapping easier.
          </p>
        </div>
      </template>
      <span class="block flex-1 truncate text-xs text-naturals-n11 select-none">
        Discovery Service
      </span>
    </Tooltip>
    <TButtonGroup v-model="mode" :options="options" />
  </div>
</template>
