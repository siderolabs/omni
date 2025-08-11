<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import pluralize from 'pluralize'
import { computed, onBeforeMount, ref } from 'vue'
import { useRouter } from 'vue-router'
import { copyText } from 'vue3-clipboard'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type { MachineStatusMetricsSpec } from '@/api/omni/specs/omni.pb'
import {
  APIConfigType,
  ClusterStatusType,
  ConfigID,
  DefaultJoinTokenID,
  DefaultJoinTokenType,
  DefaultNamespace,
  EphemeralNamespace,
  MachineStatusMetricsID,
  MachineStatusMetricsType,
  RoleNone,
  SysVersionID,
  SysVersionType,
} from '@/api/resources'
import { itemID } from '@/api/watch'
import TButton from '@/components/common/Button/TButton.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import Watch from '@/components/common/Watch/Watch.vue'
import {
  copyKernelArgs,
  downloadAuditLog,
  downloadMachineJoinConfig,
  downloadOmniconfig,
  downloadTalosconfig,
} from '@/methods'
import { canCreateClusters, canReadAuditLog, canReadClusters, currentUser } from '@/methods/auth'
import { auditLogEnabled } from '@/methods/features'
import OverviewCircleChartItem from '@/views/cluster/Overview/components/OverviewCircleChart/OverviewCircleChartItem.vue'
import ClusterStatus from '@/views/omni/Clusters/ClusterStatus.vue'

const hasRoleNone = computed(() => {
  const role = currentUser.value?.spec?.role

  return !role || role === RoleNone
})

const router = useRouter()

const showJoinToken = ref(false)
const resource = {
  namespace: DefaultNamespace,
  type: ClusterStatusType,
}

const machineStatusMetricsResource = {
  namespace: EphemeralNamespace,
  type: MachineStatusMetricsType,
  id: MachineStatusMetricsID,
}

const sysVersionResource = {
  type: SysVersionType,
  namespace: EphemeralNamespace,
  id: SysVersionID,
}

const openClusterCreate = () => {
  router.push({ name: 'ClusterCreate' })
}

const openClusters = () => {
  router.push({ name: 'Clusters' })
}

const computePercentOfMachinesAssignedToClusters = (
  items: Resource<MachineStatusMetricsSpec>[],
): string => {
  return (
    (100 / Math.max(items[0]?.spec.registered_machines_count ?? 0, 1)) *
    (items[0]?.spec.allocated_machines_count ?? 0)
  ).toFixed(0)
}

const copyValue = (value) => {
  return copyText(value, undefined, () => {})
}

const openDownloadIso = () => {
  router.push({
    query: { modal: 'downloadInstallationMedia' },
  })
}

const downloadOmnictl = () => {
  router.push({
    query: { modal: 'downloadOmnictlBinaries' },
  })
}

const downloadTalosctl = () => {
  router.push({
    query: { modal: 'downloadTalosctlBinaries' },
  })
}

const auditLogAvailable = ref(false)

onBeforeMount(async () => {
  auditLogAvailable.value = await auditLogEnabled()
})
</script>

<template>
  <div v-if="hasRoleNone" class="flex flex-col items-center justify-center gap-2">
    <div class="relative mb-6 h-16 w-16">
      <div
        class="absolute left-0 top-0 h-full w-full -translate-y-1.5 translate-x-1.5 rotate-12 rounded-lg bg-naturals-N2"
      />
      <div
        class="absolute left-0 top-0 flex h-full w-full items-center justify-center rounded-lg bg-naturals-N3"
      >
        <TIcon icon="warning" class="h-6 w-6 text-naturals-N11" />
      </div>
    </div>
    <p class="text-lg text-naturals-N14">You don't have access to Omni Web</p>
    <p class="text-xs text-naturals-N10">At least Reader role is required</p>
    <div class="mt-3 flex gap-3">
      <TButton
        type="primary"
        icon="talos-config"
        icon-position="left"
        @click="() => downloadOmniconfig()"
      >
        Download <code>omniconfig</code></TButton
      >
      <TButton
        type="primary"
        icon="talos-config"
        icon-position="left"
        @click="() => downloadOmnictl()"
      >
        Download omnictl</TButton
      >
    </div>
  </div>
  <div v-else class="flex flex-col">
    <PageHeader title="Home" />
    <Watch
      :opts="{
        resource: {
          type: APIConfigType,
          namespace: DefaultNamespace,
          id: ConfigID,
        },
        runtime: Runtime.Omni,
      }"
      errors-alert
      spinner
    >
      <template #default="{ items }">
        <div class="flex gap-6">
          <div class="w-full space-y-6">
            <div class="overview-card flex flex-1 flex-wrap gap-3 p-6">
              <div class="flex flex-1 flex-col gap-3" style="min-width: 200px">
                <div class="text-sm text-naturals-N14">General Information</div>
                <div class="overview-general-info-grid">
                  <div>Backend Version</div>
                  <Watch
                    :opts="{ resource: sysVersionResource, runtime: Runtime.Omni }"
                    class="text-right"
                  >
                    <template #default="{ items }">
                      {{ items[0]?.spec?.backend_version }}
                    </template>
                  </Watch>
                  <div>API Endpoint</div>
                  <div>
                    {{ items[0]?.spec?.machine_api_advertised_url }}
                    <TIcon
                      icon="copy"
                      class="overview-copy-icon"
                      @click="() => copyValue(items[0]?.spec?.machine_api_advertised_url)"
                    />
                  </div>
                  <div>WireGuard Endpoint</div>
                  <div>
                    {{ items[0]?.spec?.wireguard_advertised_endpoint }}
                    <TIcon
                      icon="copy"
                      class="overview-copy-icon"
                      @click="() => copyValue(items[0]?.spec?.wireguard_advertised_endpoint)"
                    />
                  </div>
                  <div>Default Join Token</div>
                  <Watch
                    :opts="{
                      resource: {
                        type: DefaultJoinTokenType,
                        namespace: DefaultNamespace,
                        id: DefaultJoinTokenID,
                      },
                      runtime: Runtime.Omni,
                    }"
                  >
                    <template #default="tokens">
                      <div class="flex items-center gap-1">
                        <div
                          class="token flex-1 cursor-pointer select-none truncate text-right"
                          @click="() => (showJoinToken = !showJoinToken)"
                        >
                          {{
                            showJoinToken
                              ? tokens.items[0]?.spec?.token_id
                              : tokens.items[0]?.spec?.token_id.replace(/./g, '•')
                          }}
                        </div>
                        <TIcon
                          icon="copy"
                          class="overview-copy-icon"
                          @click="() => copyValue(tokens.items[0]?.spec?.token_id)"
                        />
                      </div>
                    </template>
                  </Watch>
                </div>
              </div>
              <div v-if="canReadClusters" class="flex px-12">
                <Watch :opts="{ resource: machineStatusMetricsResource, runtime: Runtime.Omni }">
                  <template #default="{ items }">
                    <OverviewCircleChartItem
                      class="text-sm text-naturals-N14"
                      :chart-fill-percents="computePercentOfMachinesAssignedToClusters(items)"
                      name="Machines"
                      :usage-name="(items[0]?.spec.allocated_machines_count ?? 0) + ' Used'"
                      :usage-percents="computePercentOfMachinesAssignedToClusters(items)"
                      :usage-total="items[0]?.spec.registered_machines_count ?? 0"
                    />
                  </template>
                </Watch>
              </div>
            </div>
            <div v-if="canReadClusters" class="overview-card flex-1">
              <div class="flex flex-auto gap-6 p-6">
                <div class="text-sm text-naturals-N14">Recent Clusters</div>
                <div class="grow" />
                <TButton
                  :disabled="!canCreateClusters"
                  icon-position="left"
                  icon="plus"
                  type="subtle"
                  @click="openClusterCreate"
                  >Create Cluster
                </TButton>
                <TButton icon-position="left" icon="clusters" type="subtle" @click="openClusters"
                  >View All</TButton
                >
              </div>
              <Watch
                :opts="{
                  resource: resource,
                  runtime: Runtime.Omni,
                  sortByField: 'created',
                  sortDescending: true,
                }"
              >
                <template #norecords>
                  <div class="px-6 pb-6">No clusters</div>
                </template>
                <template #default="{ items }">
                  <div
                    v-for="item in items.slice(0, 5)"
                    :key="itemID(item)"
                    class="recent-clusters-row"
                  >
                    <div class="grid flex-1 grid-cols-3">
                      <router-link
                        :to="{ name: 'ClusterOverview', params: { cluster: item.metadata.id } }"
                        class="list-item-link"
                      >
                        {{ item.metadata.id }}
                      </router-link>
                      <div>
                        {{ item.spec.machines.total }}
                        {{ pluralize('Node', item.spec.machines.total) }}
                      </div>
                      <ClusterStatus :cluster="item" />
                    </div>
                  </div>
                </template>
              </Watch>
            </div>
          </div>
          <div class="flex-col space-y-6">
            <div
              class="overview-card flex flex-col place-items-stretch gap-5 p-6"
              style="width: 350px"
            >
              <div class="text-sm text-naturals-N14">Add Machines</div>
              <TButton icon="long-arrow-down" icon-position="left" @click="openDownloadIso"
                >Download Installation Media</TButton
              >
              <TButton
                icon="long-arrow-down"
                icon-position="left"
                @click="() => downloadMachineJoinConfig()"
                >Download Machine Join Config</TButton
              >
              <TButton icon="copy" icon-position="left" @click="() => copyKernelArgs()"
                >Copy Kernel Parameters</TButton
              >
            </div>
            <div
              class="overview-card flex flex-col place-items-stretch gap-5 p-6"
              style="width: 350px"
            >
              <div class="text-sm text-naturals-N14">CLI</div>
              <TButton
                type="primary"
                icon="document"
                icon-position="left"
                @click="() => downloadTalosconfig()"
              >
                Download <code>talosconfig</code></TButton
              >
              <TButton
                type="primary"
                icon="talos-config"
                icon-position="left"
                @click="() => downloadTalosctl()"
              >
                Download talosctl</TButton
              >
              <TButton
                type="primary"
                icon="document"
                icon-position="left"
                @click="() => downloadOmniconfig()"
              >
                Download <code>omniconfig</code></TButton
              >
              <TButton
                type="primary"
                icon="talos-config"
                icon-position="left"
                @click="() => downloadOmnictl()"
              >
                Download omnictl</TButton
              >
            </div>
            <div
              v-if="canReadAuditLog && auditLogAvailable"
              class="overview-card flex flex-col place-items-stretch gap-5 p-6"
              style="width: 350px"
            >
              <div class="text-sm text-naturals-N14">Tools</div>
              <TButton
                type="primary"
                icon="document"
                icon-position="left"
                @click="() => downloadAuditLog()"
              >
                Get audit logs</TButton
              >
            </div>
          </div>
        </div>
      </template>
    </Watch>
  </div>
</template>

<style scoped>
.overview-card {
  font-size: 12px;
  @apply rounded bg-naturals-N2;
}

.overview-copy-icon {
  @apply h-5 w-5 cursor-pointer rounded p-0.5 text-primary-P2;
}

.overview-copy-icon:hover {
  @apply bg-naturals-N5 text-primary-P1;
}

.overview-general-info-grid {
  @apply grid grid-cols-3 gap-4 text-xs;
}

.overview-general-info-grid > *:nth-child(even) {
  @apply col-span-2 flex items-center justify-end gap-1;
}

.recent-clusters-row {
  @apply flex w-full place-content-between px-6 py-4;
}

.recent-clusters-row:not(:first-of-type) {
  @apply border-t border-naturals-N4;
}

.token {
  font-family: 'Roboto Mono', 'consolas', monospace, sans-serif;
}
</style>
