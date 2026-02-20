<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { NodeSpec as V1NodeSpec, NodeStatus as V1NodeStatus } from 'kubernetes-types/core/v1'
import { DateTime } from 'luxon'
import { computed, onBeforeUnmount, ref, useId } from 'vue'
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import { subscribe } from '@/api/grpc'
import type { MachineStatusLinkSpec } from '@/api/omni/specs/ephemeral.pb'
import type { ClusterMachineStatusSpec } from '@/api/omni/specs/omni.pb'
import { ConfigApplyStatus } from '@/api/omni/specs/omni.pb'
import { withAbortController, withContext, withRuntime } from '@/api/options'
import {
  ClusterMachineStatusType,
  DefaultNamespace,
  kubernetes,
  MachineStatusLinkType,
  MetricsNamespace,
  TalosAddressRoutedNoK8s,
  TalosK8sNamespace,
  TalosMachineStatusID,
  TalosMachineStatusType,
  TalosNetworkNamespace,
  TalosNodeAddressType,
  TalosNodenameID,
  TalosNodenameType,
  TalosRuntimeNamespace,
} from '@/api/resources'
import type { ServiceEvent } from '@/api/talos/machine/machine.pb'
import { MachineService } from '@/api/talos/machine/machine.pb'
import TGroupAnimation from '@/components/common/Animation/TGroupAnimation.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import TListItem from '@/components/common/List/TListItem.vue'
import TStatus from '@/components/common/Status/TStatus.vue'
import Tag from '@/components/common/Tag/Tag.vue'
import TAlert from '@/components/TAlert.vue'
import { TCommonStatuses } from '@/constants'
import { getContext } from '@/context'
import { formatBytes, getStatus } from '@/methods'
import { addMachineLabels, removeMachineLabels } from '@/methods/machine'
import { useResourceWatch } from '@/methods/useResourceWatch'
import ClusterMachinePhase from '@/views/cluster/ClusterMachines/ClusterMachinePhase.vue'
import NodeConditions from '@/views/cluster/Nodes/components/NodeConditions.vue'
import NodeDiagnosticWarnings from '@/views/cluster/Nodes/NodeDiagnosticWarnings.vue'
import NodeServiceEvents from '@/views/cluster/Nodes/NodeServiceEvents.vue'
import ItemLabels from '@/views/omni/ItemLabels/ItemLabels.vue'

const route = useRoute()
const context = getContext()

const services = ref<
  {
    name?: string
    state?: string
    status: TCommonStatuses
    events?: ServiceEvent[]
  }[]
>([])

let abortController: AbortController | undefined

const fetchServices = async () => {
  if (abortController) {
    abortController.abort()
  }

  abortController = new AbortController()

  const res = await MachineService.ServiceList(
    {},
    withContext(context),
    withRuntime(Runtime.Talos),
    withAbortController(abortController),
  )

  abortController = undefined

  services.value = []

  res.messages?.forEach((message) =>
    message.services?.forEach((service) =>
      services.value.push({
        name: service.id,
        state: service.state,
        status: service.health?.unknown
          ? TCommonStatuses.HEALTH_UNKNOWN
          : service.health?.healthy
            ? TCommonStatuses.HEALTHY
            : TCommonStatuses.UNHEALTHY,
        events: service.events?.events,
      }),
    ),
  )
}

fetchServices()

const stream = subscribe(
  MachineService.Events,
  {},
  (event) => {
    // For some reason @type is not typed on Any
    const data = event.data as (typeof event.data & { ['@type']?: string }) | undefined

    if (data?.['@type']?.includes('machine.ServiceStateEvent')) {
      fetchServices()
    }
  },
  [withRuntime(Runtime.Talos), withContext(context)],
)

onBeforeUnmount(() => {
  stream.shutdown()
})

const configApplyStatusToConfigApplyStatusName = (status?: ConfigApplyStatus): string => {
  switch (status) {
    case ConfigApplyStatus.APPLIED:
      return 'Applied'
    case ConfigApplyStatus.FAILED:
      return 'Failed'
    case ConfigApplyStatus.PENDING:
      return 'Pending'
    default:
      return 'Unknown'
  }
}

const alertDismissed = ref(false)

const { data: nodename } = useResourceWatch<{ nodename: string }>({
  runtime: Runtime.Talos,
  resource: {
    id: TalosNodenameID,
    type: TalosNodenameType,
    namespace: TalosK8sNamespace,
  },
  context,
})

const { data: node } = useResourceWatch<V1NodeSpec, V1NodeStatus>(() => {
  const id = nodename.value?.spec.nodename

  return {
    skip: !id,
    runtime: Runtime.Kubernetes,
    resource: {
      id: id!,
      type: kubernetes.node,
    },
    context,
  }
})

const { data: nodeaddress } = useResourceWatch<{ addresses: string[] }>({
  runtime: Runtime.Talos,
  resource: {
    id: TalosAddressRoutedNoK8s,
    type: TalosNodeAddressType,
    namespace: TalosNetworkNamespace,
  },
  context,
})

const { data: talosMachineStatus } = useResourceWatch<{
  stage: string
  status: {
    ready: boolean
    unmetConditions: {
      name: string
      reason: string
    }[]
  }
}>({
  runtime: Runtime.Talos,
  resource: {
    id: TalosMachineStatusID,
    type: TalosMachineStatusType,
    namespace: TalosRuntimeNamespace,
  },
  context,
})

const { data: machineStatus } = useResourceWatch<MachineStatusLinkSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    id: route.params.machine as string,
    type: MachineStatusLinkType,
    namespace: MetricsNamespace,
  },
}))

const { data: clusterMachineStatus } = useResourceWatch<ClusterMachineStatusSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    type: ClusterMachineStatusType,
    namespace: DefaultNamespace,
    id: route.params.machine as string,
  },
}))

const roles = computed(() =>
  Object.keys(node.value?.metadata.labels ?? {})
    .filter((label) => label.includes('node-role.kubernetes.io/'))
    .map((label) => label.split('/')[1]),
)

const status = computed(() => getStatus(node.value!))

const timeGetter = (time: string | undefined) => {
  if (time) {
    const dt: DateTime = /^\d+$/.exec(time)
      ? DateTime.fromSeconds(parseInt(time))
      : DateTime.fromISO(time)

    return dt.setLocale('en').toLocaleString(DateTime.DATETIME_FULL_WITH_SECONDS)
  }

  return 'Never'
}

const getCPUInfo = () => {
  const processors = machineStatus.value?.spec.message_status?.hardware?.processors
  if (!processors) {
    return 'Not Detected'
  }

  const cpus: Record<string, number> = {}

  for (const processor of processors) {
    const id = `${processor.frequency! / 1000} GHz ${processor.manufacturer}, ${processor.core_count} Cores`

    cpus[id] = (cpus[id] ?? 0) + 1
  }

  const parts: string[] = []

  for (const id in cpus) {
    if (cpus[id] > 1) {
      parts.push(`${cpus[id]} x ${id}`)
    }

    parts.push(`${id}`)
  }

  return parts.join(', ')
}

const getMemInfo = () => {
  const total = machineStatus.value?.spec.message_status?.hardware?.memory_modules?.reduce(
    (prev, current) => {
      return prev + current.size_mb!
    },
    0,
  )

  return formatBytes(total! * 1024 * 1024)
}

const getSecureBootStatus = () => {
  const securityState = machineStatus?.value?.spec.message_status?.security_state
  if (!securityState) {
    return TCommonStatuses.UNKNOWN
  }

  if (securityState.secure_boot) {
    return TCommonStatuses.ENABLED
  }

  return TCommonStatuses.DISABLED
}

const servicesSectionHeadingId = useId()
</script>

<template>
  <div class="overview py-4">
    <TAlert
      v-if="clusterMachineStatus?.spec?.last_config_error && !alertDismissed"
      :dismiss="{ name: 'Dismiss', action: () => (alertDismissed = true) }"
      class="mb-4"
      title="Configuration Error"
      type="error"
    >
      {{ clusterMachineStatus?.spec?.last_config_error }}
    </TAlert>
    <ul class="overview-data-list">
      <li class="overview-data-item">
        <h4 class="overview-data-heading">Kubernetes</h4>
        <div class="overview-data-row">
          <p class="overview-data-name">Hostname</p>
          <p class="overview-data">{{ nodename?.spec?.nodename }}</p>
        </div>
        <div class="overview-data-row">
          <p class="overview-data-name">Kubelet Version</p>
          <p class="overview-data">
            {{ node?.status?.nodeInfo?.kubeletVersion }}
          </p>
        </div>
        <div class="overview-data-row">
          <p class="overview-data-name">OS</p>
          <p class="overview-data">
            {{ node?.status?.nodeInfo?.operatingSystem }}
          </p>
        </div>
        <div class="overview-data-row">
          <p class="overview-data-name">Roles</p>
          <div class="overview-data-roles">
            <Tag v-for="role in roles" :key="role" class="overview-data-role">
              {{ role }}
            </Tag>
          </div>
        </div>
        <div class="overview-data-row">
          <p class="overview-data-name">Kubernetes Node Status</p>
          <div class="overview-data">
            <TStatus :title="status" />
          </div>
        </div>
      </li>
      <li class="overview-data-item">
        <h4 class="overview-data-heading">Talos</h4>
        <div class="overview-data-row">
          <p class="overview-data-name">Addresses</p>
          <p class="overview-data">{{ nodeaddress?.spec.addresses.join(', ') }}</p>
        </div>
        <div class="overview-data-row">
          <p class="overview-data-name">OS</p>
          <p class="overview-data">{{ node?.status?.nodeInfo?.osImage }}</p>
        </div>
        <div class="overview-data-row">
          <p class="overview-data-name">Architecture</p>
          <p class="overview-data">
            {{ node?.status?.nodeInfo?.architecture }}
          </p>
        </div>
        <div class="overview-data-row">
          <p class="overview-data-name">Kernel Version</p>
          <p class="overview-data">
            {{ node?.status?.nodeInfo?.kernelVersion }}
          </p>
        </div>
        <div class="overview-data-row">
          <p class="overview-data-name">Container Runtime Version</p>
          <p class="overview-data">
            {{ node?.status?.nodeInfo?.containerRuntimeVersion }}
          </p>
        </div>
        <div class="overview-data-row">
          <p class="overview-data-name">Config Status</p>
          <TStatus
            :title="
              configApplyStatusToConfigApplyStatusName(
                clusterMachineStatus?.spec?.config_apply_status,
              )
            "
          />
        </div>
      </li>
      <li class="overview-data-item">
        <h4 class="overview-data-heading">Machine Status</h4>
        <div class="overview-data-row">
          <p class="overview-data-name">Stage</p>
          <ClusterMachinePhase
            v-if="clusterMachineStatus"
            :machine="clusterMachineStatus"
            class="text-xs"
          />
        </div>
        <div v-if="talosMachineStatus" class="overview-data-row">
          <p class="overview-data-name">Unmet Conditions</p>
          <NodeConditions
            v-if="talosMachineStatus.spec.status?.unmetConditions.length > 0"
            :conditions="talosMachineStatus.spec.status?.unmetConditions"
          />
          <div v-else class="flex items-center gap-1 text-xs text-green-g1">
            <TIcon icon="check-in-circle-classic" class="h-4" />
            None
          </div>
        </div>
        <template v-if="machineStatus">
          <div class="overview-data-row">
            <p class="overview-data-name">Last Active</p>
            <p class="overview-data">
              {{ timeGetter(machineStatus.spec.siderolink_counter?.last_alive) }}
            </p>
          </div>
          <div class="overview-data-row">
            <p class="overview-data-name">Secure Boot</p>
            <p class="overview-data">
              <TStatus class="overview-status" :title="getSecureBootStatus()" />
            </p>
          </div>
          <div class="overview-data-row">
            <p class="overview-data-name">CPU</p>
            <p class="overview-data">
              {{ getCPUInfo() }}
            </p>
          </div>
          <div class="overview-data-row">
            <p class="overview-data-name">Memory</p>
            <p class="overview-data">
              {{ getMemInfo() }}
            </p>
          </div>
        </template>
      </li>
    </ul>

    <template v-if="machineStatus">
      <ul v-if="machineStatus.spec.message_status?.diagnostics" class="overview-data-list">
        <li class="flex w-full flex-col rounded bg-naturals-n2">
          <h4 class="overview-data-heading">Diagnostic Warnings</h4>
          <NodeDiagnosticWarnings :diagnostics="machineStatus?.spec?.message_status?.diagnostics" />
        </li>
      </ul>

      <ul class="overview-data-list">
        <li class="flex w-full flex-col rounded bg-naturals-n2">
          <h4 class="overview-data-heading">Labels</h4>
          <div class="overview-data-row">
            <ItemLabels
              :resource="machineStatus"
              :add-label-func="addMachineLabels"
              :remove-label-func="removeMachineLabels"
            />
          </div>
        </li>
      </ul>
    </template>

    <section class="overview-services" :aria-labelledby="servicesSectionHeadingId">
      <header class="overview-services-heading">
        <h2 :id="servicesSectionHeadingId" class="overview-services-title">Services</h2>
        <span class="overview-services-amount">{{ services.length }}</span>
      </header>
      <ul class="overview-table-header grid grid-cols-4">
        <li class="overview-table-name col-span-2">ID</li>
        <li class="overview-table-name">Running</li>
        <li class="overview-table-name">Health</li>
      </ul>
      <TGroupAnimation>
        <TListItem v-for="service in services" :key="service.name">
          <template #default>
            <div class="grid grid-cols-4 p-1">
              <RouterLink
                class="list-item-link col-span-2"
                :to="{
                  name: 'NodeLogs',
                  query: {
                    cluster: $route.query.cluster,
                    namespace: $route.query.namespace,
                    uid: $route.query.uid,
                  },
                  params: {
                    machine: $route.params.machine,
                    service: service.name,
                  },
                }"
              >
                {{ service.name }}
              </RouterLink>
              <TStatus class="overview-status" :title="service.state" />
              <TStatus class="overview-status" :title="service.status" />
            </div>
          </template>
          <template #details>
            <NodeServiceEvents :events="service.events" />
          </template>
        </TListItem>
      </TGroupAnimation>
    </section>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.overview {
  @apply flex w-full flex-col gap-6;
}

.overview-data-list {
  @apply flex flex-col items-stretch justify-start gap-y-6 xl:flex-row xl:items-start xl:justify-between xl:gap-x-6;
}

.overview-data-item {
  @apply w-full rounded bg-naturals-n2;
  align-self: stretch;
  max-width: 100%;
}

.overview-data-heading {
  @apply w-full border-b border-naturals-n4 px-4 py-3 text-xs text-naturals-n13;
  font-size: 13px;
}
.overview-data-row {
  @apply flex w-full items-center justify-between gap-2;
  padding: 14px 16px 10px 16px;
}
.overview-data-row:last-of-type {
  padding-bottom: 12px;
}
.overview-data-name {
  @apply text-xs text-naturals-n11;
}
.overview-data {
  @apply text-xs text-naturals-n13;
}
.overview-data-roles {
  @apply flex;
}
.overview-data-role:not(:last-of-type) {
  margin-right: 6px;
}
.overview-services-heading {
  @apply mb-4 flex items-center;
}
.overview-services-title {
  @apply mr-2 text-base text-naturals-n13;
}
.overview-services-amount {
  @apply bg-naturals-n5 text-xs text-naturals-n12;
  border-radius: 30px;
  padding: 3px 7px;
}
.overview-table-header {
  @apply rounded-sm bg-naturals-n2 px-4 py-2 pl-11;
}
.overview-table-name {
  @apply w-full text-xs text-naturals-n13;
}
</style>
