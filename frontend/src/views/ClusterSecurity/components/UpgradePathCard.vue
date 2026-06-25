<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref } from 'vue'

import TButton from '@/components/Button/TButton.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import VulnerabilityList from '@/views/ClusterSecurity/components/VulnerabilityList.vue'
import type { UpgradeDiff, UpgradeTarget } from '@/views/ClusterSecurity/util/vulnerabilityDiff'

const { target, diff, loading, error } = defineProps<{
  target: UpgradeTarget
  diff?: UpgradeDiff
  loading: boolean
  error?: string
}>()

const expanded = ref(false)

const kindLabel = computed(() => (target.kind === 'patch' ? 'Latest patch' : 'Next minor version'))

const canExpand = computed(() => !!diff && (diff.resolved.length > 0 || diff.introduced.length > 0))
</script>

<template>
  <div class="rounded border border-naturals-n5 bg-naturals-n2">
    <div class="flex flex-wrap items-center gap-3 px-4 py-3">
      <TIcon icon="upgrade" class="size-5 shrink-0 text-naturals-n11" aria-hidden="true" />

      <div class="flex flex-1 flex-col">
        <span class="text-sm font-medium text-naturals-n14">Upgrade to {{ target.version }}</span>
        <span class="text-xs text-naturals-n11">{{ kindLabel }}</span>
      </div>

      <TSpinner v-if="loading" class="size-4" />

      <template v-else-if="diff">
        <ul class="flex flex-wrap items-center gap-1.5 text-xs">
          <li
            v-if="diff.resolved.length"
            class="rounded-sm bg-green-700 px-2 py-1 font-medium text-white"
          >
            {{ diff.resolved.length }} fixed
          </li>
          <li class="rounded-sm bg-naturals-n4 px-2 py-1 text-naturals-n11">
            {{ diff.remaining.length }} remaining
          </li>
          <li
            v-if="diff.introduced.length"
            class="rounded-sm bg-red-700 px-2 py-1 font-medium text-white"
          >
            {{ diff.introduced.length }} new
          </li>
          <li
            v-if="!diff.resolved.length && !diff.introduced.length"
            class="rounded-sm bg-naturals-n4 px-2 py-1 text-naturals-n11"
          >
            No change
          </li>
        </ul>

        <TButton
          v-if="canExpand"
          size="sm"
          variant="secondary"
          :icon="expanded ? 'arrow-up' : 'arrow-down'"
          icon-position="right"
          @click="expanded = !expanded"
        >
          Details
        </TButton>
      </template>
    </div>

    <TAlert v-if="error" type="error" title="Scan failed" class="mx-4 mb-3">{{ error }}</TAlert>

    <div
      v-else-if="expanded && diff"
      class="flex flex-col gap-4 border-t border-naturals-n5 px-4 py-3"
    >
      <section v-if="diff.resolved.length" class="flex flex-col gap-2">
        <h4 class="flex items-center gap-1.5 text-xs font-medium text-green-g1">
          <TIcon icon="check-in-circle" class="size-4 shrink-0" aria-hidden="true" />
          Fixed by this upgrade ({{ diff.resolved.length }})
        </h4>
        <VulnerabilityList :matches="diff.resolved" />
      </section>

      <section v-if="diff.introduced.length" class="flex flex-col gap-2">
        <h4 class="flex items-center gap-1.5 text-xs font-medium text-red-r1">
          <TIcon icon="warning" class="size-4 shrink-0" aria-hidden="true" />
          Introduced by this upgrade ({{ diff.introduced.length }})
        </h4>
        <VulnerabilityList :matches="diff.introduced" />
      </section>
    </div>
  </div>
</template>
