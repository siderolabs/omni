<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import pluralize from 'pluralize'
import { computed, ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type {
  ClusterMachineConfigStatusSpec,
  ClusterStatusSpec,
  MachineStatusSpec,
  TalosVersionSpec,
} from '@/api/omni/specs/omni.pb'
import { PlatformConfigSpecArch } from '@/api/omni/specs/virtual.pb'
import {
  ClusterMachineConfigStatusType,
  ClusterStatusType,
  DefaultNamespace,
  LabelCluster,
  LabelControlPlaneRole,
  MachineStatusType,
  TalosVersionType,
} from '@/api/resources'
import TButton from '@/components/Button/TButton.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import Tooltip from '@/components/Tooltip/Tooltip.vue'
import { getDocsLink } from '@/methods'
import { useIsEnterprise } from '@/methods/features'
import { useResourceList } from '@/methods/useResourceList'
import { useResourceWatch } from '@/methods/useResourceWatch'
import SeverityBadges from '@/views/ClusterSecurity/components/SeverityBadges.vue'
import UpgradePathCard from '@/views/ClusterSecurity/components/UpgradePathCard.vue'
import {
  scanKey,
  type ScanRequest,
  useClusterVulnerabilityScans,
} from '@/views/ClusterSecurity/util/useClusterVulnerabilityScans'
import { computeUpgradeTargets, diffMatches } from '@/views/ClusterSecurity/util/vulnerabilityDiff'
import type { Match } from '@/views/InstallationMedia/vulnerabilities/ReportTypes'
import ScanDetailsModal from '@/views/InstallationMedia/vulnerabilities/ScanDetailsModal.vue'

const { clusterId } = defineProps<{ clusterId: string }>()

const isEnterpriseFactory = useIsEnterprise()

const { data: clusterStatus, loading: statusLoading } = useResourceWatch<ClusterStatusSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: ClusterStatusType,
    id: clusterId,
  },
}))

const { data: configStatuses, loading: configLoading } =
  useResourceWatch<ClusterMachineConfigStatusSpec>(() => ({
    skip: !isEnterpriseFactory.value,
    runtime: Runtime.Omni,
    resource: {
      namespace: DefaultNamespace,
      type: ClusterMachineConfigStatusType,
    },
    selectors: [`${LabelCluster}=${clusterId}`],
  }))

const { data: machineStatuses } = useResourceWatch<MachineStatusSpec>(() => ({
  skip: !isEnterpriseFactory.value,
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: MachineStatusType,
  },
  selectors: [`${LabelCluster}=${clusterId}`],
}))

const { data: talosVersions } = useResourceList<TalosVersionSpec>(() => ({
  skip: !isEnterpriseFactory.value,
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: TalosVersionType,
  },
}))

const currentVersion = computed(() => clusterStatus.value?.spec.talos_version)

const archByMachine = computed(
  () => new Map(machineStatuses.value.map((m) => [m.metadata.id!, m.spec.hardware?.arch])),
)

interface Combo {
  key: string
  schematicId: string
  arch: string
  machineCount: number
  includesControlPlane: boolean
}

// Unique (schematic, arch) combinations across the cluster's machines.
const combos = computed(() => {
  const map = new Map<string, Combo>()

  for (const cfg of configStatuses.value) {
    const schematicId = cfg.spec.schematic_id
    const arch = archByMachine.value.get(cfg.metadata.id!)

    if (!schematicId || !arch) continue

    const key = `${schematicId}|${arch}`
    const existing = map.get(key)
    const isControlPlane = LabelControlPlaneRole in (cfg.metadata.labels ?? {})

    if (existing) {
      existing.machineCount += 1
      existing.includesControlPlane ||= isControlPlane
    } else {
      map.set(key, {
        key,
        schematicId,
        arch,
        machineCount: 1,
        includesControlPlane: isControlPlane,
      })
    }
  }

  return map.values().toArray()
})

const upgradeTargets = computed(() =>
  currentVersion.value
    ? computeUpgradeTargets(
        currentVersion.value,
        talosVersions.value
          .filter((v) => !v.spec.deprecated && !v.spec.unsupported)
          .map((v) => v.spec.version!),
      )
    : [],
)

// Every (combo, version) pair that needs a scan.
const scanRequests = computed<ScanRequest[]>(() => {
  if (!currentVersion.value) return []

  const versions = [
    ...new Set([currentVersion.value, ...upgradeTargets.value.map((t) => t.version)]),
  ]

  return combos.value.flatMap((combo) =>
    versions.map((version) => ({ schematicId: combo.schematicId, arch: combo.arch, version })),
  )
})

const { results } = useClusterVulnerabilityScans(scanRequests)

function scanFor(combo: Combo, version: string) {
  return results.value.get(scanKey({ schematicId: combo.schematicId, arch: combo.arch, version }))
}

// Per-combo view model: current scan plus a diff per upgrade target.
const comboReports = computed(() =>
  combos.value.map((combo) => {
    const current = currentVersion.value ? scanFor(combo, currentVersion.value) : undefined

    return {
      combo,
      current,
      upgrades: upgradeTargets.value.map((target) => {
        const scan = scanFor(combo, target.version)
        const diff =
          current?.matches && scan?.matches ? diffMatches(current.matches, scan.matches) : undefined

        return { target, diff, loading: scan?.loading ?? true, error: scan?.error }
      }),
    }
  }),
)

const initialLoading = computed(() => statusLoading.value || configLoading.value)

function archEnum(arch: string) {
  return arch === 'arm64' ? PlatformConfigSpecArch.ARM64 : PlatformConfigSpecArch.AMD64
}

const detailsModal = ref<{
  open: boolean
  schematicId: string
  arch: string
  version: string
  matches: Match[]
}>()

function openDetails(schematicId: string, arch: string, version: string, matches: Match[]) {
  detailsModal.value = { open: true, schematicId, arch, version, matches }
}
</script>

<template>
  <section class="flex flex-col gap-6">
    <header class="flex flex-col items-start">
      <h1 class="text-lg text-naturals-n14">
        Vulnerabilities for {{ clusterId }}

        <span class="resource-label label-red inline-flex items-center gap-1">
          <TIcon class="size-3.5 shrink-0" icon="talos" aria-label="Talos version" />
          {{ currentVersion }}
        </span>
      </h1>

      <p class="text-sm text-naturals-n11">
        Vulnerabilities detected for the cluster's running Talos version, and how upgrading would
        change them. More information available on our
        <a
          class="link-primary"
          :href="getDocsLink('talos', '/advanced-guides/SBOM', { talosVersion: currentVersion })"
        >
          docs
        </a>
        page.
      </p>
    </header>

    <TAlert v-if="!isEnterpriseFactory" type="info" title="Vulnerability scanning unavailable">
      Vulnerability scanning requires the enterprise image factory.
    </TAlert>

    <p v-else-if="initialLoading" class="flex items-center gap-1.5 text-sm text-naturals-n11">
      <TSpinner class="size-4" />
      Loading cluster information…
    </p>

    <TAlert v-else-if="!currentVersion" type="error" title="Unknown Talos version">
      Could not determine the cluster's running Talos version.
    </TAlert>

    <TAlert v-else-if="!comboReports.length" type="info" title="No machines to scan">
      This cluster has no machines with a known schematic and architecture yet.
    </TAlert>

    <template v-else>
      <article
        v-for="{ combo, current, upgrades } in comboReports"
        :key="combo.key"
        class="flex flex-col gap-4 rounded border border-naturals-n5 p-4"
      >
        <header class="flex flex-col gap-1">
          <h2 class="flex items-center gap-1 text-sm text-naturals-n13">
            <TIcon aria-hidden="true" icon="document-text" class="size-4" />
            Schematic
            <Tooltip :description="combo.schematicId">
              <span class="font-mono">{{ combo.schematicId.slice(0, 12) }}…</span>
            </Tooltip>
          </h2>

          <div class="flex flex-wrap items-center gap-2">
            <span class="resource-label label-orange">{{ combo.arch }}</span>
            <span class="resource-label label-blue">
              {{ combo.includesControlPlane ? 'control plane' : 'worker' }}
            </span>
            <span class="text-xs text-naturals-n11">
              applies to {{ combo.machineCount }}
              {{ pluralize('machine', combo.machineCount) }}
            </span>
          </div>
        </header>

        <div class="flex flex-col gap-3">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="flex flex-col gap-1.5">
              <div
                v-if="current?.loading"
                class="flex items-center gap-1.5 text-xs text-naturals-n11"
              >
                <TSpinner class="size-4" />
                Running scan…
              </div>
              <TAlert v-else-if="current?.error" type="error" title="Scan failed">
                {{ current.error }}
              </TAlert>
              <SeverityBadges v-else-if="current?.matches" :matches="current.matches" />
            </div>

            <TButton
              v-if="current?.matches?.length"
              size="sm"
              icon="document-text"
              icon-position="left"
              @click="openDetails(combo.schematicId, combo.arch, currentVersion!, current.matches)"
            >
              View report
            </TButton>
          </div>

          <div v-if="upgrades.length" class="flex flex-col gap-3">
            <h3 class="text-sm text-naturals-n11">Upgrade paths</h3>
            <UpgradePathCard
              v-for="upgrade in upgrades"
              :key="upgrade.target.version"
              :target="upgrade.target"
              :diff="upgrade.diff"
              :loading="upgrade.loading"
              :error="upgrade.error"
            />
          </div>
          <p v-else class="flex items-center gap-1.5 text-xs text-green-g1">
            <TIcon icon="check-in-circle" class="size-4 shrink-0" aria-hidden="true" />
            Already on the latest available Talos version.
          </p>
        </div>
      </article>
    </template>

    <ScanDetailsModal
      v-if="detailsModal"
      v-model:open="detailsModal.open"
      :matches="detailsModal.matches"
      :schematic-id="detailsModal.schematicId"
      :talos-version="detailsModal.version"
      :arch="archEnum(detailsModal.arch)"
    />
  </section>
</template>
