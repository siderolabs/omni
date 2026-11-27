<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useIntervalFn, useNow } from '@vueuse/core'
import { dump } from 'js-yaml'
import { computed } from 'vue'
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import {
  TalosContainerHealthStatusType,
  TalosContainerImageStatusType,
  TalosContainerInstanceStatusType,
  TalosContainerMountStatusType,
  TalosContainerRunStatusType,
  TalosContainersNamespace,
  TalosContainerSpecType,
} from '@/api/resources'
import type {
  ContainerHealthStatusSpec,
  ContainerImageStatusSpec,
  ContainerInstanceStatusSpec,
  ContainerMountStatusSpec,
  ContainerRunStatusSpec,
  ContainerSpecSpec,
} from '@/api/talos/containers.pb'
import TButton from '@/components/Button/TButton.vue'
import CodeBlock from '@/components/CodeBlock/CodeBlock.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import StatusGlyph from '@/components/Status/StatusGlyph.vue'
import StatusPill from '@/components/Status/StatusPill.vue'
import ResponsiveTable from '@/components/Table/ResponsiveTable.vue'
import TableCell from '@/components/Table/TableCell.vue'
import TableRoot from '@/components/Table/TableRoot.vue'
import TableRow from '@/components/Table/TableRow.vue'
import TAlert from '@/components/TAlert.vue'
import { formatFullDateTime } from '@/methods/time'
import { useResourceWatch } from '@/methods/useResourceWatch'
import ContainerImage from '@/views/Containers/components/ContainerImage.vue'
import DetailCard from '@/views/Containers/components/DetailCard.vue'
import DetailRow from '@/views/Containers/components/DetailRow.vue'
import {
  dependsOnList,
  healthCheckTarget,
  limitsText,
  mountSource,
  restartText,
  runAsText,
  runSummary,
  runTime,
} from '@/views/Containers/lib/containerDetail'
import { containerLogsRoute, containersRoute } from '@/views/Containers/lib/containerRoutes'
import { containerRows, imageRef, instanceId } from '@/views/Containers/lib/containerRows'
import {
  formatSince,
  healthAppearance,
  healthName,
  stateAppearance,
  stateName,
} from '@/views/Containers/lib/containerStatus'
import { injectMachineContainers } from '@/views/Containers/lib/machineContainers'

const { clusterId, machineId, containerId } = defineProps<{
  clusterId: string
  machineId: string
  containerId: string
}>()

// The statuses come from the page, which already watches them for the tab
const { data: statuses, loading, err, supported } = injectMachineContainers()

// The list's search and sort came along in the query, to go back to
const route = useRoute()

const status = computed(() => statuses.value.find((s) => s.metadata.id === containerId))

/** Watches one of this container's resources by ID, on this machine. */
const watchOne = <T,>(type: string, id: () => string | undefined) =>
  useResourceWatch<T>(() => ({
    runtime: Runtime.Talos,
    resource: { namespace: TalosContainersNamespace, type, id: id() ?? '' },
    context: { node: machineId },
    // Nothing to watch until the container is known to exist
    skip: !supported.value || !status.value || !id(),
  }))

const { data: spec } = watchOne<ContainerSpecSpec>(TalosContainerSpecType, () => containerId)
const { data: image } = watchOne<ContainerImageStatusSpec>(
  TalosContainerImageStatusType,
  () => containerId,
)
const { data: runStatus } = watchOne<ContainerRunStatusSpec>(
  TalosContainerRunStatusType,
  () => containerId,
)
const { data: mountStatus } = watchOne<ContainerMountStatusSpec>(
  TalosContainerMountStatusType,
  () => containerId,
)

// A health status belongs to one instance, so it follows the current generation
const { data: health } = watchOne<ContainerHealthStatusSpec>(TalosContainerHealthStatusType, () =>
  status.value && spec.value?.spec.healthCheck?.kind
    ? instanceId(containerId, status.value.spec.generation)
    : undefined,
)

const { data: allInstances } = useResourceWatch<ContainerInstanceStatusSpec>(() => ({
  runtime: Runtime.Talos,
  resource: { namespace: TalosContainersNamespace, type: TalosContainerInstanceStatusType },
  context: { node: machineId },
  skip: !supported.value,
}))

// Newest run first
const instances = computed(() =>
  allInstances.value
    .filter((i) => i.spec.containerID === containerId)
    .sort((a, b) => (b.spec.generation ?? 0) - (a.spec.generation ?? 0)),
)

// The exit code of the newest run that has finished
const lastExitCode = computed(
  () => instances.value.find((i) => i.spec.phase === 'terminated')?.spec.exitCode,
)

// The same row the list shows, for the image and the time in state
const row = computed(() =>
  status.value
    ? containerRows([status.value], image.value ? [image.value] : [], instances.value)[0]
    : undefined,
)

const now = useNow({ scheduler: (cb) => useIntervalFn(cb, 1000) })

const security = computed(() => spec.value?.spec.security)
const dependsOn = computed(() => dependsOnList(spec.value?.spec.dependsOn))
const healthCheck = computed(() => spec.value?.spec.healthCheck)

const yesNo = (value?: boolean) => (value ? 'Yes' : 'No')

// Talos marks a resource it removed fields from; the spec is always read redacted
const redacted = computed(() => !!spec.value?.metadata.annotations?.['talos.dev/redacted'])

const yaml = computed(() =>
  [status.value, spec.value, image.value, runStatus.value, mountStatus.value, health.value]
    .flatMap((r) => (r ? [dump({ metadata: r.metadata, spec: r.spec }, { lineWidth: -1 })] : []))
    .join('---\n'),
)
</script>

<template>
  <PageContainer class="flex flex-col gap-base">
    <RouterLink
      :to="containersRoute(clusterId, machineId, route.query)"
      class="flex w-fit items-center gap-micro text-xs text-content-secondary hover:text-content-emphasis"
    >
      <TIcon icon="chevron-left" class="size-3" />
      All containers
    </RouterLink>

    <TAlert v-if="!supported" type="info" title="No containers">
      Containers need Talos 1.15 or later on this machine.
    </TAlert>

    <TSpinner v-else-if="loading" class="size-6 self-center" />

    <TAlert v-else-if="err" type="error" title="Failed to fetch containers">{{ err }}</TAlert>

    <TAlert v-else-if="!status || !row" type="info" title="Container not found">
      This machine's configuration doesn't declare a container named {{ containerId }}.
    </TAlert>

    <template v-else>
      <header class="flex flex-wrap items-center gap-compact">
        <h2 class="text-base text-content-emphasis">{{ containerId }}</h2>
        <StatusPill v-bind="stateAppearance(status.spec.state, status.spec.health)">
          {{ stateName(status.spec.state) }}
        </StatusPill>
        <ContainerImage class="text-xs" :image="imageRef(row)" :digest="image?.spec.digest" />

        <TButton
          is="router-link"
          variant="secondary"
          size="sm"
          class="ml-auto"
          :to="containerLogsRoute(clusterId, machineId, containerId)"
        >
          Logs
        </TButton>
      </header>

      <div class="grid grid-cols-1 items-start gap-base xl:grid-cols-3">
        <DetailCard title="Runtime">
          <DetailRow label="Health">
            <span class="inline-flex items-center gap-micro">
              <StatusGlyph v-bind="healthAppearance(status.spec.health)" aria-hidden="true" />
              {{ healthName(status.spec.health) }}
            </span>
          </DetailRow>
          <DetailRow label="PID">
            <span class="font-mono">{{ status.spec.pid || '—' }}</span>
          </DetailRow>
          <DetailRow label="In state since">
            <template v-if="row.since">
              {{ formatFullDateTime(row.since.toISOString()) }} ({{ formatSince(row.since, now) }})
            </template>
            <template v-else>—</template>
          </DetailRow>
          <DetailRow label="Restarts">
            <span class="font-mono">{{ status.spec.restartCount ?? 0 }}</span>
          </DetailRow>
          <DetailRow label="Last exit code">
            <span class="font-mono">{{ lastExitCode ?? '—' }}</span>
          </DetailRow>
          <DetailRow v-if="status.spec.error" label="Error">
            <span class="font-mono">{{ status.spec.error }}</span>
          </DetailRow>
          <DetailRow v-if="image?.spec.error" label="Image error">
            <span class="font-mono">{{ image.spec.error }}</span>
          </DetailRow>
        </DetailCard>

        <DetailCard title="Runs as">
          <DetailRow label="User">
            <span class="font-mono">{{ runAsText(spec?.spec.runAs) }}</span>
          </DetailRow>
          <DetailRow label="Network">
            {{ spec?.spec.network?.hostNetwork ? 'Host' : 'Isolated' }}
          </DetailRow>
          <DetailRow label="Privileged">{{ yesNo(security?.privileged) }}</DetailRow>
          <DetailRow v-if="security?.capabilitiesAdd?.length" label="Capabilities added">
            <span class="font-mono">{{ security.capabilitiesAdd.join(', ') }}</span>
          </DetailRow>
          <DetailRow v-if="security?.capabilitiesDrop?.length" label="Capabilities dropped">
            <span class="font-mono">{{ security.capabilitiesDrop.join(', ') }}</span>
          </DetailRow>
          <DetailRow label="Seccomp">
            {{
              security?.seccompUnconfined
                ? 'Unconfined'
                : security?.seccompLocalhostProfile
                  ? security.seccompLocalhostProfile
                  : 'Default'
            }}
          </DetailRow>
          <DetailRow label="Read-only root filesystem">
            {{ yesNo(security?.readOnlyRootFS) }}
          </DetailRow>
          <DetailRow label="machined access">{{ yesNo(security?.machinedAccess) }}</DetailRow>
          <DetailRow label="Limits">{{ limitsText(spec?.spec.resources) }}</DetailRow>
          <DetailRow v-if="spec?.spec.workingDir" label="Working directory">
            <span class="font-mono">{{ spec.spec.workingDir }}</span>
          </DetailRow>
        </DetailCard>

        <DetailCard title="Lifecycle">
          <DetailRow label="Restart">{{ restartText(spec?.spec.restart) }}</DetailRow>
          <DetailRow v-if="runStatus?.spec.backoffStep" label="Backoff step">
            <span class="font-mono">{{ runStatus.spec.backoffStep }}</span>
          </DetailRow>
          <DetailRow v-if="spec?.spec.stopTimeout" label="Stop timeout">
            <span class="font-mono">{{ spec.spec.stopTimeout }}</span>
          </DetailRow>
          <DetailRow label="Health check">
            <span class="font-mono">
              {{ healthCheck ? healthCheckTarget(healthCheck) : 'None' }}
            </span>
          </DetailRow>
          <template v-if="healthCheck?.kind">
            <DetailRow label="Every">
              <span class="font-mono">
                {{ healthCheck.interval || '—' }}, timeout {{ healthCheck.timeout || '—' }}
              </span>
            </DetailRow>
            <DetailRow label="Unhealthy after">
              {{ healthCheck.failureThreshold ?? '—' }} failures
            </DetailRow>
            <DetailRow label="Last result">
              {{ health ? (health.spec.healthy ? 'Healthy' : 'Unhealthy') : 'Not checked yet' }}
            </DetailRow>
            <DetailRow v-if="health?.spec.lastChange" label="Since">
              {{ formatFullDateTime(health.spec.lastChange) }}
            </DetailRow>
            <DetailRow v-if="health?.spec.message" label="Failure">
              <span class="font-mono">{{ health.spec.message }}</span>
            </DetailRow>
          </template>
        </DetailCard>
      </div>

      <div
        v-if="status.spec.waitingFor?.length || dependsOn.length"
        class="grid grid-cols-1 items-start gap-base xl:grid-cols-2"
      >
        <DetailCard v-if="status.spec.waitingFor?.length" title="Waiting for">
          <DetailRow v-for="gate in status.spec.waitingFor" :key="gate" label="Unmet">
            <span class="font-mono">{{ gate }}</span>
          </DetailRow>
        </DetailCard>

        <DetailCard v-if="dependsOn.length" title="Depends on">
          <DetailRow v-for="gate in dependsOn" :key="gate" label="Gate">
            <span class="font-mono">{{ gate }}</span>
          </DetailRow>
        </DetailCard>
      </div>

      <section v-if="spec?.spec.mounts?.length" class="flex flex-col gap-tight">
        <h3 class="text-sm text-content-emphasis">Mounts</h3>
        <TAlert v-if="mountStatus?.spec.error" type="error" title="Mounts failed">
          {{ mountStatus.spec.error }}
        </TAlert>
        <TableRoot class="w-full">
          <template #head>
            <TableRow>
              <TableCell th>Destination</TableCell>
              <TableCell th>Source</TableCell>
              <TableCell th>Options</TableCell>
            </TableRow>
          </template>
          <template #body>
            <TableRow v-for="mount in spec.spec.mounts" :key="mount.destination">
              <TableCell class="font-mono wrap-anywhere">{{ mount.destination }}</TableCell>
              <TableCell class="font-mono wrap-anywhere">{{ mountSource(mount) }}</TableCell>
              <TableCell class="font-mono wrap-anywhere">{{ mount.options?.join(', ') }}</TableCell>
            </TableRow>
          </template>
        </TableRoot>
      </section>

      <section v-if="spec?.spec.devices?.length" class="flex flex-col gap-tight">
        <h3 class="text-sm text-content-emphasis">Devices</h3>
        <TableRoot class="w-full">
          <template #head>
            <TableRow>
              <TableCell th>Destination</TableCell>
              <TableCell th>Volume</TableCell>
            </TableRow>
          </template>
          <template #body>
            <TableRow v-for="device in spec.spec.devices" :key="device.destination">
              <TableCell class="font-mono wrap-anywhere">{{ device.destination }}</TableCell>
              <TableCell class="font-mono wrap-anywhere">{{ device.volumeName }}</TableCell>
            </TableRow>
          </template>
        </TableRoot>
      </section>

      <section class="flex flex-col gap-tight">
        <h3 class="text-sm text-content-emphasis">Run history</h3>
        <p v-if="!instances.length" class="text-xs text-content-secondary">
          The container hasn't run since the machine booted.
        </p>
        <!-- 65rem: the fixed columns take 49rem, two of them full dates, which leaves Error 16rem -->
        <ResponsiveTable
          v-else
          :rows="instances"
          :row-key="(instance) => instance.metadata.id!"
          :min-width-rem="65"
        >
          <template #head>
            <TableCell th class="w-16">Run</TableCell>
            <TableCell th class="w-28">Phase</TableCell>
            <TableCell th class="w-20">PID</TableCell>
            <TableCell th class="w-20">Exit code</TableCell>
            <TableCell th class="w-56">Started</TableCell>
            <TableCell th class="w-56">Finished</TableCell>
            <TableCell th>Error</TableCell>
          </template>

          <template #row="{ row: instance }">
            <TableCell class="font-mono">{{ instance.spec.generation ?? 0 }}</TableCell>
            <TableCell>{{ instance.spec.phase }}</TableCell>
            <TableCell class="font-mono">{{ instance.spec.pid || '' }}</TableCell>
            <TableCell class="font-mono">{{ instance.spec.exitCode ?? '' }}</TableCell>
            <TableCell>{{ runTime(instance.spec.startedAt) }}</TableCell>
            <TableCell>{{ runTime(instance.spec.finishedAt) }}</TableCell>
            <TableCell class="font-mono wrap-anywhere">{{ instance.spec.error }}</TableCell>
          </template>

          <!-- Stacked: the run and its phase, the error if there is one, then the numbers as one
          phrase -->
          <template #stacked="{ row: instance }">
            <div class="flex flex-col gap-tight">
              <div class="flex items-start justify-between gap-tight">
                <span class="text-content-emphasis">Run {{ instance.spec.generation ?? 0 }}</span>
                <span class="shrink-0">{{ instance.spec.phase }}</span>
              </div>

              <p v-if="instance.spec.error" class="font-mono wrap-anywhere">
                {{ instance.spec.error }}
              </p>

              <p class="text-content-secondary">{{ runSummary(instance.spec) }}</p>
            </div>
          </template>
        </ResponsiveTable>
      </section>

      <details class="flex flex-col gap-tight">
        <summary class="cursor-pointer text-sm text-content-emphasis">Raw YAML</summary>
        <p v-if="redacted" class="py-tight text-xs text-content-secondary">
          Talos redacts sensitive fields, such as the environment, entrypoint and arguments.
        </p>
        <CodeBlock :code="yaml" lang="yaml" />
      </details>
    </template>
  </PageContainer>
</template>
