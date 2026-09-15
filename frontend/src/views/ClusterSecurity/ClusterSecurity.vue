<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { ClusterStatusSpec } from '@/api/omni/specs/omni.pb'
import { PlatformConfigSpecArch } from '@/api/omni/specs/virtual.pb'
import { ClusterStatusType, DefaultNamespace, LabelEnterprise } from '@/api/resources'
import TIcon from '@/components/Icon/TIcon.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import { getDocsLink } from '@/methods'
import { useIsEnterprise } from '@/methods/features'
import { useResourceWatch } from '@/methods/useResourceWatch'
import ClusterSecurityTarget from '@/views/ClusterSecurity/components/ClusterSecurityTarget.vue'
import type { Match } from '@/views/ClusterSecurity/util/ReportTypes'
import { useClusterArtifactTargets } from '@/views/ClusterSecurity/util/securityReports'
import ScanDetailsModal from '@/views/InstallationMedia/vulnerabilities/ScanDetailsModal.vue'

const { clusterId } = defineProps<{ clusterId: string }>()

const isEnterprise = useIsEnterprise()

const { data: clusterStatus, err: clusterStatusError } = useResourceWatch<ClusterStatusSpec>(
  () => ({
    skip: !isEnterprise.value,
    runtime: Runtime.Omni,
    resource: {
      namespace: DefaultNamespace,
      type: ClusterStatusType,
      id: clusterId,
    },
  }),
)

const isEnterpriseCluster = computed(
  () => clusterStatus.value?.metadata.labels?.[LabelEnterprise] !== undefined,
)

const {
  data: artifactTargets,
  loading: targetsLoading,
  err: targetsError,
} = useClusterArtifactTargets(() => ({ clusterId, skip: !isEnterpriseCluster.value }))

const currentVersion = computed(() => artifactTargets.value?.current_talos_version)

const detailsModal = ref<{
  open: boolean
  schematicId: string
  arch: PlatformConfigSpecArch
  version: string
  matches: Match[]
}>()
</script>

<template>
  <section class="flex flex-col gap-6">
    <header class="flex flex-col items-start">
      <h1 class="text-lg text-naturals-n14">
        Vulnerabilities for {{ clusterId }}

        <span v-if="currentVersion" class="resource-label label-red inline-flex items-center gap-1">
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
          target="_blank"
          rel="noopener noreferrer"
        >
          docs
        </a>
        page.
      </p>
    </header>

    <TAlert v-if="!isEnterprise" type="info" title="Vulnerability scanning unavailable">
      Vulnerability scanning requires the enterprise image factory.
    </TAlert>

    <TAlert v-else-if="clusterStatusError" type="error" title="Failed to load cluster information">
      {{ clusterStatusError }}
    </TAlert>

    <p
      v-else-if="!clusterStatus || targetsLoading"
      class="flex items-center gap-1.5 text-sm text-naturals-n11"
    >
      <TSpinner class="size-4" />
      Loading cluster information…
    </p>

    <TAlert v-else-if="!isEnterpriseCluster" type="info" title="Vulnerability scanning unavailable">
      Vulnerability scanning requires every machine in the cluster to run an image from the
      enterprise image factory.
    </TAlert>

    <TAlert v-else-if="targetsError" type="error" title="Failed to load cluster information">
      {{ targetsError.message }}
    </TAlert>

    <TAlert v-else-if="!currentVersion" type="error" title="Unknown Talos version">
      Could not determine the cluster's running Talos version.
    </TAlert>

    <TAlert v-else-if="!artifactTargets?.targets?.length" type="info" title="No machines to scan">
      This cluster has no machines with a known schematic and architecture yet.
    </TAlert>

    <template v-else>
      <ClusterSecurityTarget
        v-for="artifactTarget in artifactTargets?.targets"
        :key="`${artifactTarget.schematic_id}-${artifactTarget.arch}`"
        :current-version="currentVersion"
        :upgrade-versions="artifactTargets.upgrade_target_versions"
        :artifact-target
        @open-details="
          (schematicId, arch, version, matches) =>
            (detailsModal = { open: true, schematicId, arch, version, matches })
        "
      />
    </template>

    <ScanDetailsModal
      v-if="detailsModal"
      v-model:open="detailsModal.open"
      :matches="detailsModal.matches"
      :schematic-id="detailsModal.schematicId"
      :talos-version="detailsModal.version"
      :arch="detailsModal.arch"
    />
  </section>
</template>
