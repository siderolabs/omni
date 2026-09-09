<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import pluralize from 'pluralize'
import coerce from 'semver/functions/coerce'
import { computed } from 'vue'

import type { ArtifactTarget } from '@/api/omni/imagefactory/imagefactory.pb'
import { PlatformConfigSpecArch } from '@/api/omni/specs/virtual.pb'
import TButton from '@/components/Button/TButton.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import Tooltip from '@/components/Tooltip/Tooltip.vue'
import SeverityBadges from '@/views/ClusterSecurity/components/SeverityBadges.vue'
import UpgradePathCard from '@/views/ClusterSecurity/components/UpgradePathCard.vue'
import type { Match } from '@/views/ClusterSecurity/util/ReportTypes'
import { archToString, configArchFromArch } from '@/views/ClusterSecurity/util/securityReports'
import {
  scanKey,
  type ScanRequest,
  useClusterVulnerabilityScans,
} from '@/views/ClusterSecurity/util/useClusterVulnerabilityScans'

const {
  artifactTarget,
  currentVersion,
  upgradeVersions = [],
} = defineProps<{
  artifactTarget: ArtifactTarget
  currentVersion: string
  upgradeVersions?: string[]
}>()

defineEmits<{
  openDetails: [
    schematicId: string,
    arch: PlatformConfigSpecArch,
    version: string,
    matches: Match[],
  ]
}>()

const arch = computed(() => configArchFromArch(artifactTarget.arch!))

const { results: scanResults } = useClusterVulnerabilityScans(() =>
  [currentVersion, ...upgradeVersions].map<ScanRequest>((version) => ({
    schematicId: artifactTarget.schematic_id!,
    arch: arch.value,
    version,
  })),
)

const currentVersionScan = computed(() =>
  scanResults.value.get(
    scanKey({
      schematicId: artifactTarget.schematic_id!,
      arch: arch.value,
      version: currentVersion,
    }),
  ),
)

const upgradeVersionScans = computed(() => {
  const cur = coerce(currentVersion, { includePrerelease: true })

  return upgradeVersions
    .map((version) => {
      const v = coerce(version, { includePrerelease: true })

      return {
        version,
        isPatch: v?.major === cur?.major && v?.minor === cur?.minor,
        scan: scanResults.value.get(
          scanKey({
            schematicId: artifactTarget.schematic_id!,
            arch: arch.value,
            version,
          }),
        ),
      }
    })
    .filter((s): s is typeof s & { scan: NonNullable<typeof s.scan> } => !!s.scan)
})
</script>

<template>
  <article class="flex flex-col gap-4 rounded border border-naturals-n5 p-4">
    <header class="flex flex-col gap-1">
      <h2 class="flex items-center gap-1 text-sm text-naturals-n13">
        <TIcon aria-hidden="true" icon="document-text" class="size-4" />
        Schematic
        <Tooltip :description="artifactTarget.schematic_id">
          <span class="font-mono">{{ artifactTarget.schematic_id?.slice(0, 12) }}…</span>
        </Tooltip>
      </h2>

      <div class="flex flex-wrap items-center gap-2">
        <span class="resource-label label-orange">
          {{ archToString(arch) }}
        </span>
        <span class="resource-label label-blue">
          {{ artifactTarget.includes_control_plane ? 'control plane' : 'worker' }}
        </span>
        <span class="text-xs text-naturals-n11">
          applies to {{ artifactTarget.machine_count }}
          {{ pluralize('machine', artifactTarget.machine_count) }}
        </span>
      </div>
    </header>

    <div class="flex flex-col gap-3">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="flex flex-col gap-1.5">
          <div
            v-if="currentVersionScan?.loading"
            class="flex items-center gap-1.5 text-xs text-naturals-n11"
          >
            <TSpinner class="size-4" />
            Running scan…
          </div>
          <TAlert v-else-if="currentVersionScan?.error" type="error" title="Scan failed">
            {{ currentVersionScan.error }}
          </TAlert>
          <SeverityBadges
            v-else-if="currentVersionScan?.matches"
            :matches="currentVersionScan.matches"
          />
        </div>

        <TButton
          v-if="currentVersionScan?.matches?.length"
          size="sm"
          icon="document-text"
          icon-position="left"
          @click="
            $emit(
              'openDetails',
              artifactTarget.schematic_id ?? '',
              arch,
              currentVersion!,
              currentVersionScan.matches,
            )
          "
        >
          View report
        </TButton>
      </div>

      <div v-if="currentVersionScan && upgradeVersionScans.length" class="flex flex-col gap-3">
        <h3 class="text-sm text-naturals-n11">Upgrade paths</h3>
        <UpgradePathCard
          v-for="{ scan, version, isPatch } in upgradeVersionScans"
          :key="version"
          :current-version-scan="currentVersionScan"
          :scan
          :version
          :is-patch
        />
      </div>
      <p v-else class="flex items-center gap-1.5 text-xs text-green-g1">
        <TIcon icon="check-in-circle" class="size-4 shrink-0" aria-hidden="true" />
        Already on the latest available Talos version.
      </p>
    </div>
  </article>
</template>
