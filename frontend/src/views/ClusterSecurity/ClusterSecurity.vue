<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { TalosVersionSpec } from '@/api/omni/specs/omni.pb'
import { PlatformConfigSpecArch } from '@/api/omni/specs/virtual.pb'
import { DefaultNamespace, TalosVersionType } from '@/api/resources'
import TIcon from '@/components/Icon/TIcon.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import { getDocsLink } from '@/methods'
import { useIsEnterprise } from '@/methods/features'
import { useResourceGet } from '@/methods/useResourceGet'
import ClusterSecurityTarget from '@/views/ClusterSecurity/components/ClusterSecurityTarget.vue'
import type { Match } from '@/views/ClusterSecurity/util/ReportTypes'
import { useClusterArtifactTargets } from '@/views/ClusterSecurity/util/securityReports'
import ScanDetailsModal from '@/views/InstallationMedia/vulnerabilities/ScanDetailsModal.vue'

const { clusterId } = defineProps<{ clusterId: string }>()

const isEnterpriseFactory = useIsEnterprise()

const {
  data: artifactTargets,
  loading: targetsLoading,
  err: targetsError,
} = useClusterArtifactTargets(() => ({ clusterId, skip: !isEnterpriseFactory.value }))

const currentVersion = computed(() => artifactTargets.value?.current_talos_version)

const { data: talosVersion } = useResourceGet<TalosVersionSpec>(() => ({
  skip: !isEnterpriseFactory.value || !currentVersion.value,
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: TalosVersionType,
    id: currentVersion.value ?? '',
  },
}))

const isTalosVersionEnterpriseFactory = computed(() => talosVersion.value?.spec.is_enterprise)

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

    <TAlert
      v-else-if="!isTalosVersionEnterpriseFactory"
      type="info"
      title="Vulnerability scanning unavailable"
    >
      Vulnerability scanning requires using a talos version from the enterprise image factory.
    </TAlert>

    <p v-else-if="targetsLoading" class="flex items-center gap-1.5 text-sm text-naturals-n11">
      <TSpinner class="size-4" />
      Loading cluster information…
    </p>

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
