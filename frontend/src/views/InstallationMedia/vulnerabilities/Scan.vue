<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import pluralize from 'pluralize'
import { ref } from 'vue'

import { PlatformConfigSpecArch } from '@/api/omni/specs/virtual.pb'
import TButton from '@/components/Button/TButton.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import ScanDetailsModal from '@/views/InstallationMedia/vulnerabilities/ScanDetailsModal.vue'
import { useVulnerabilityReport } from '@/views/InstallationMedia/vulnerabilities/useVulnerabilityReport'

const { schematicId, talosVersion, arch } = defineProps<{
  schematicId: string
  talosVersion: string
  arch: PlatformConfigSpecArch
}>()

const scanDetailsModalOpen = ref(false)

const {
  data: matches,
  loading,
  err,
} = useVulnerabilityReport(() => ({
  arch,
  schematicId,
  talosVersion,
}))
</script>

<template>
  <section class="flex flex-col gap-3">
    <h3 class="text-sm text-naturals-n14">Vulnerability Scan</h3>

    <p v-if="loading" class="flex items-center gap-1.5">
      <TSpinner class="size-4" />
      Running vulnerability scan...
    </p>
    <TAlert v-else-if="err" title="Error" type="error">{{ err }}</TAlert>

    <template v-if="matches">
      <p v-if="matches.length === 0" class="flex items-center gap-1.5 text-xs text-green-g1">
        <TIcon icon="check-in-circle" class="size-4 shrink-0" aria-hidden="true" />
        No vulnerabilities detected
      </p>

      <div v-else class="flex flex-col items-start gap-2">
        <p class="flex items-center gap-1.5 text-xs text-yellow-y1">
          <TIcon icon="warning" class="size-4 shrink-0" aria-hidden="true" />
          <span>
            <span class="font-medium">{{ matches.length }}</span>
            {{ pluralize('vulnerability', matches.length) }} after VEX suppression
          </span>
        </p>

        <TButton
          size="sm"
          icon="document-text"
          icon-position="left"
          @click="scanDetailsModalOpen = true"
        >
          View report
        </TButton>
      </div>
    </template>
  </section>

  <ScanDetailsModal
    v-if="matches"
    v-model:open="scanDetailsModalOpen"
    :matches="matches"
    :schematic-id
    :talos-version
    :arch
  />
</template>
